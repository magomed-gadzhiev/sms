import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Counter, Trend } from 'k6/metrics';
import { SharedArray } from 'k6/data';

// ─── Fixtures ──────────────────────────────────────────────────────────────

const destinations = new SharedArray('destinations', function () {
    const data = JSON.parse(open('../test/load/fixtures/destinations.json'));
    const all = [];
    for (const group of data.domestic) all.push(...group.numbers);
    for (const group of data.international) all.push(...group.numbers);
    return all;
});

const sources = new SharedArray('sources', function () {
    const data = JSON.parse(open('../test/load/fixtures/sources.json'));
    return data.sources.map(s => s.name);
});

const messageTemplates = new SharedArray('messages', function () {
    const data = JSON.parse(open('../test/load/fixtures/messages.json'));
    const all = [];
    for (const cat of data.templates) all.push(...cat.texts);
    return all;
});

// ─── Custom Metrics ────────────────────────────────────────────────────────

const sendLatency    = new Trend('sms_send_latency',    true); // milliseconds
const e2eLatency     = new Trend('sms_e2e_latency',     true);
const pipelineErrors = new Rate('sms_pipeline_errors');
const sentTotal      = new Counter('sms_sent_total');
const batchTotal     = new Counter('sms_batch_total');

// ─── Config from env ───────────────────────────────────────────────────────

const SCENARIO = __ENV.SCENARIO  || 'pipeline';
const BASE_URL = __ENV.BASE_URL  || 'http://localhost:8080';
const API_KEY  = __ENV.API_KEY   || 'test-api-key';
const RUN_ID   = __ENV.RUN_ID   || 'unknown';
const RUN_TYPE = __ENV.RUN_TYPE  || 'regression';
const GIT_SHA  = __ENV.GIT_SHA   || 'unknown';

// ─── Options per scenario ──────────────────────────────────────────────────

const commonTags = { run_id: RUN_ID, run_type: RUN_TYPE, git_sha: GIT_SHA };

const smokeOptions = {
    scenarios: {
        smoke: {
            executor: 'constant-vus',
            vus: 10,
            duration: '2m',
            exec: 'smokeDefault',
            tags: { scenario: 'smoke' },
        },
    },
    thresholds: {
        'http_req_failed':   ['rate<0.01'],
        'http_req_duration': ['p(95)<1000'],
    },
    tags: commonTags,
};

const pipelineOptions = {
    scenarios: {
        pipeline_baseline: {
            executor: 'ramping-vus',
            startVUs: 0,
            stages: [
                { duration: '2m', target: 200 },
                { duration: '3m', target: 500 },
                { duration: '5m', target: 500 },
                { duration: '2m', target: 0 },
            ],
            exec: 'pipelineDefault',
            gracefulRampDown: '30s',
            tags: { scenario: 'pipeline' },
        },
    },
    thresholds: {
        'http_req_duration{name:SendSMS}': ['p(95)<500', 'p(99)<1000'],
        'http_req_failed':                 ['rate<0.01'],
        'sms_pipeline_errors':             ['rate<0.02'],
    },
    tags: commonTags,
};

const soakOptions = {
    scenarios: {
        soak: {
            executor: 'constant-vus',
            vus: 200,
            duration: '30m',
            exec: 'pipelineDefault',
            tags: { scenario: 'soak' },
        },
    },
    thresholds: {
        'http_req_duration{name:SendSMS}': ['p(95)<500'],
        'http_req_failed':                 ['rate<0.01'],
        'sms_pipeline_errors':             ['rate<0.02'],
    },
    tags: commonTags,
};

export const options =
    SCENARIO === 'smoke' ? smokeOptions :
    SCENARIO === 'soak'  ? soakOptions  :
                           pipelineOptions;

// ─── Helpers ───────────────────────────────────────────────────────────────

const headers = { 'Content-Type': 'application/json', 'X-API-Key': API_KEY };

function pick(arr) { return arr[Math.floor(Math.random() * arr.length)]; }

function destination() {
    if (Math.random() < 0.7) return pick(destinations);
    const prefixes = ['7910', '7903', '7920', '7900', '7999', '7985', '7925'];
    return pick(prefixes) + Math.floor(Math.random() * 10000000).toString().padStart(7, '0');
}

function messageText() {
    return pick(messageTemplates)
        .replace('{code}',       Math.floor(100000 + Math.random() * 900000))
        .replace('{order_id}',   Math.floor(10000  + Math.random() * 90000))
        .replace('{amount}',     (Math.random() * 10000).toFixed(2))
        .replace('{balance}',    (Math.random() * 100000).toFixed(2))
        .replace('{receipt_id}', Math.floor(1000000 + Math.random() * 9000000))
        .replace('{tracking}',   'LT' + Math.floor(1000000000 + Math.random() * 9000000000))
        .replace('{discount}',   Math.floor(5 + Math.random() * 75))
        .replace('{promo}',      Math.floor(1000 + Math.random() * 9000))
        .replace('{time}',       `${Math.floor(8 + Math.random() * 12)}:${Math.floor(Math.random() * 60).toString().padStart(2, '0')}`)
        .replace('{ip}',         `${Math.floor(1 + Math.random() * 254)}.${Math.floor(Math.random() * 256)}.${Math.floor(Math.random() * 256)}.1`)
        .replace('{rate}',       Math.floor(100 + Math.random() * 9900))
        .replace('{tx_id}',      Math.floor(10000000 + Math.random() * 90000000));
}

// VU-local queue of sent messages for e2e tracking
// Each VU has its own copy; no shared mutable state between VUs.
const pendingMessages = []; // { id: string, sentAt: number }
const E2E_TIMEOUT_MS = 30000;

// ─── Request functions ─────────────────────────────────────────────────────

function sendSMS() {
    const payload = JSON.stringify({
        source:              pick(sources),
        destination:         destination(),
        text:                messageText(),
        external_id:         `k6-${__VU}-${__ITER}-${Date.now()}`,
        registered_delivery: true,
    });

    const res = http.post(`${BASE_URL}/api/v1/sms/send`, payload, {
        headers,
        tags: { name: 'SendSMS' },
    });

    sendLatency.add(res.timings.duration);
    sentTotal.add(1);

    const ok = check(res, {
        'SendSMS status 200': r => r.status === 200,
        'SendSMS has message_id': r => {
            try { return !!JSON.parse(r.body).message_id; } catch { return false; }
        },
    });

    if (ok) {
        try {
            const body = JSON.parse(res.body);
            if (body.message_id) {
                pendingMessages.push({ id: body.message_id, sentAt: Date.now() });
                // Cap queue at 20 to prevent unbounded growth per VU
                if (pendingMessages.length > 20) pendingMessages.shift();
            }
        } catch { /* ignore parse errors */ }
    }

    return ok;
}

function sendBatch() {
    const batchSize = 5 + Math.floor(Math.random() * 16); // 5-20 messages
    const messages = Array.from({ length: batchSize }, (_, i) => ({
        source:      pick(sources),
        destination: destination(),
        text:        messageText(),
        external_id: `k6-batch-${__VU}-${__ITER}-${i}-${Date.now()}`,
    }));

    const res = http.post(`${BASE_URL}/api/v1/sms/batch`, JSON.stringify({ messages }), {
        headers,
        tags: { name: 'SendBatch' },
    });

    batchTotal.add(batchSize);

    return check(res, {
        'SendBatch status 200': r => r.status === 200,
        'SendBatch has results': r => {
            try { return Array.isArray(JSON.parse(r.body).results); } catch { return false; }
        },
    });
}

function pollStatus() {
    // If no pending messages, fall back to sendSMS
    if (pendingMessages.length === 0) return sendSMS();

    const msg = pendingMessages[0];
    const elapsed = Date.now() - msg.sentAt;

    // Timeout — message never reached delivered
    if (elapsed > E2E_TIMEOUT_MS) {
        pendingMessages.shift();
        pipelineErrors.add(1);
        return false;
    }

    const res = http.get(`${BASE_URL}/api/v1/sms/status/${msg.id}`, {
        headers: { 'X-API-Key': API_KEY },
        tags: { name: 'PollStatus' },
    });

    if (!check(res, { 'PollStatus status 200': r => r.status === 200 })) {
        return false;
    }

    try {
        const status = JSON.parse(res.body).status;
        if (status === 'delivered' || status === 'sent') {
            e2eLatency.add(Date.now() - msg.sentAt);
            pendingMessages.shift();
        } else if (status === 'failed' || status === 'rejected') {
            pipelineErrors.add(1);
            pendingMessages.shift();
        }
        // 'pending', 'queued' etc → stay in queue, poll again next iteration
    } catch { /* ignore */ }

    return true;
}

function healthCheck() {
    const res = http.get(`${BASE_URL}/health`, { tags: { name: 'Health' } });
    return check(res, { 'Health status 200': r => r.status === 200 });
}

// ─── Scenario executors ────────────────────────────────────────────────────

export function smokeDefault() {
    const ok = Math.random() < 0.8 ? sendSMS() : healthCheck();
    sleep(0.1);
    return ok;
}

export function pipelineDefault() {
    const roll = Math.random() * 100;
    let ok;
    if      (roll < 70) ok = sendSMS();
    else if (roll < 85) ok = sendBatch();
    else                ok = pollStatus();
    sleep(0.05);
    return ok;
}

// Required by k6 when no scenario `exec` is set; also used for local dev runs
export default function () {
    if (SCENARIO === 'smoke') return smokeDefault();
    return pipelineDefault();
}

// ─── Summary ───────────────────────────────────────────────────────────────

export function handleSummary(data) {
    const m = data.metrics;
    const fmt = (v, unit) => (v !== undefined ? `${v.toFixed(1)}${unit}` : 'N/A');

    let s = '\n' + '='.repeat(65) + '\n';
    s += `  SMS Gateway ${SCENARIO.toUpperCase()} — run_id: ${RUN_ID}  git: ${GIT_SHA}\n`;
    s += '='.repeat(65) + '\n\n';

    if (m.http_reqs) {
        const r = m.http_reqs.values;
        s += `Throughput:      ${fmt(r.rate, ' req/s')}  (total: ${r.count})\n`;
    }
    if (m.http_req_duration) {
        const d = m.http_req_duration.values;
        s += `HTTP Latency:    avg=${fmt(d.avg, 'ms')}  p95=${fmt(d['p(95)'], 'ms')}  p99=${fmt(d['p(99)'], 'ms')}\n`;
    }
    if (m.sms_e2e_latency) {
        const d = m.sms_e2e_latency.values;
        s += `E2E Latency:     avg=${fmt(d.avg, 'ms')}  p95=${fmt(d['p(95)'], 'ms')}  p99=${fmt(d['p(99)'], 'ms')}\n`;
    }
    if (m.http_req_failed) {
        s += `Error rate:      ${fmt(m.http_req_failed.values.rate * 100, '%')}\n`;
    }
    if (m.sms_pipeline_errors) {
        s += `Pipeline errors: ${fmt(m.sms_pipeline_errors.values.rate * 100, '%')}\n`;
    }
    if (m.sms_sent_total) {
        s += `SMS sent:        ${m.sms_sent_total.values.count}\n`;
    }

    s += '\n' + '='.repeat(65) + '\n';

    return {
        stdout: s,
        [`baseline-${SCENARIO}-${RUN_ID}.json`]: JSON.stringify(data, null, 2),
    };
}
