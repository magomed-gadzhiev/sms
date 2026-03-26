import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Counter, Trend } from 'k6/metrics';
import { SharedArray } from 'k6/data';

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

const errorRate = new Rate('errors');
const sendCounter = new Counter('sms_sent');
const batchCounter = new Counter('sms_batch_sent');
const sendLatency = new Trend('send_latency');

export const options = {
    stages: [
        { duration: '20s', target: 100 },
        { duration: '40s', target: 300 },
        { duration: '1m',  target: 500 },
        { duration: '40s', target: 300 },
        { duration: '20s', target: 0 },
    ],
    thresholds: {
        http_req_duration: ['p(95)<2000', 'p(99)<5000'],
        http_req_failed: ['rate<0.05'],
        errors: ['rate<0.05'],
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://host.docker.internal:8080';
const API_KEY = __ENV.API_KEY || 'test-api-key';

function randomItem(arr) { return arr[Math.floor(Math.random() * arr.length)]; }

function randomDestination() {
    if (Math.random() < 0.7) return randomItem(destinations);
    const prefixes = ['7910', '7903', '7920', '7900', '7999'];
    return randomItem(prefixes) + Math.floor(Math.random() * 10000000).toString().padStart(7, '0');
}

function randomText() {
    let text = randomItem(messageTemplates);
    text = text.replace('{code}', Math.floor(100000 + Math.random() * 900000));
    text = text.replace('{order_id}', Math.floor(10000 + Math.random() * 90000));
    text = text.replace('{amount}', (Math.random() * 10000).toFixed(2));
    text = text.replace('{balance}', (Math.random() * 100000).toFixed(2));
    return text;
}

const headers = { 'Content-Type': 'application/json', 'X-API-Key': API_KEY };

function sendSMS() {
    const payload = JSON.stringify({
        source: randomItem(sources),
        destination: randomDestination(),
        text: randomText(),
        external_id: `k6-${__VU}-${__ITER}-${Date.now()}`,
    });
    const res = http.post(`${BASE_URL}/api/v1/sms/send`, payload, { headers, tags: { name: 'SendSMS' } });
    sendLatency.add(res.timings.duration);
    sendCounter.add(1);
    return check(res, {
        'send: status 200': (r) => r.status === 200,
        'send: has message_id': (r) => { try { return JSON.parse(r.body).message_id !== undefined; } catch { return false; } },
    });
}

function sendBatch() {
    const batchSize = 5 + Math.floor(Math.random() * 6);
    const messages = [];
    for (let i = 0; i < batchSize; i++) {
        messages.push({ source: randomItem(sources), destination: randomDestination(), text: randomText() });
    }
    const res = http.post(`${BASE_URL}/api/v1/sms/batch`, JSON.stringify({ messages }), { headers, tags: { name: 'SendBatch' } });
    batchCounter.add(batchSize);
    return check(res, { 'batch: status 200': (r) => r.status === 200 });
}

function healthCheck() {
    const res = http.get(`${BASE_URL}/health`, { tags: { name: 'Health' } });
    return check(res, { 'health: status 200': (r) => r.status === 200 });
}

export default function () {
    const roll = Math.random() * 100;
    let success;
    if (roll < 75) success = sendSMS();
    else if (roll < 90) success = sendBatch();
    else success = healthCheck();
    errorRate.add(!success);
    sleep(0.05);
}

export function handleSummary(data) {
    let s = '\n' + '='.repeat(60) + '\n';
    s += '  SMS Gateway Quick Load Test Results\n';
    s += '='.repeat(60) + '\n\n';
    if (data.metrics.http_reqs) {
        const r = data.metrics.http_reqs.values;
        s += `Throughput:  ${(r.rate || 0).toFixed(2)} req/s  (total: ${r.count || 0})\n`;
    }
    if (data.metrics.http_req_duration) {
        const d = data.metrics.http_req_duration.values;
        s += `Latency:    avg=${(d.avg || 0).toFixed(1)}ms  p95=${(d['p(95)'] || 0).toFixed(1)}ms  p99=${(d['p(99)'] || 0).toFixed(1)}ms\n`;
    }
    if (data.metrics.http_req_failed) {
        s += `Error rate: ${((data.metrics.http_req_failed.values.rate || 0) * 100).toFixed(2)}%\n`;
    }
    if (data.metrics.sms_sent) s += `SMS sent:   ${data.metrics.sms_sent.values.count || 0}\n`;
    if (data.metrics.sms_batch_sent) s += `Batch SMS:  ${data.metrics.sms_batch_sent.values.count || 0}\n`;
    s += '\n' + '='.repeat(60) + '\n';
    return { 'stdout': s, 'quick-load-test-summary.json': JSON.stringify(data, null, 2) };
}
