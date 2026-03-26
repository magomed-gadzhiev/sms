import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Counter, Trend } from 'k6/metrics';
import { SharedArray } from 'k6/data';

// =============================================================================
// Фикстуры: загружаются один раз и разделяются между VU
// =============================================================================

const destinations = new SharedArray('destinations', function () {
    const data = JSON.parse(open('../test/load/fixtures/destinations.json'));
    const all = [];
    for (const group of data.domestic) {
        all.push(...group.numbers);
    }
    for (const group of data.international) {
        all.push(...group.numbers);
    }
    return all;
});

const sources = new SharedArray('sources', function () {
    const data = JSON.parse(open('../test/load/fixtures/sources.json'));
    return data.sources.map(s => s.name);
});

const messageTemplates = new SharedArray('messages', function () {
    const data = JSON.parse(open('../test/load/fixtures/messages.json'));
    const all = [];
    for (const cat of data.templates) {
        all.push(...cat.texts);
    }
    return all;
});

// =============================================================================
// Кастомные метрики
// =============================================================================

const errorRate = new Rate('errors');
const sendCounter = new Counter('sms_sent');
const batchCounter = new Counter('sms_batch_sent');
const statusCounter = new Counter('status_checked');
const historyCounter = new Counter('history_checked');
const sendLatency = new Trend('send_latency');

// =============================================================================
// Конфигурация
// =============================================================================

export const options = {
    stages: [
        { duration: '1m',  target: 500 },
        { duration: '2m',  target: 1000 },
        { duration: '3m',  target: 2000 },
        { duration: '5m',  target: 3000 },
        { duration: '3m',  target: 5000 },
        { duration: '2m',  target: 3000 },
        { duration: '1m',  target: 0 },
    ],
    thresholds: {
        http_req_duration: ['p(95)<2000', 'p(99)<5000'],
        http_req_failed: ['rate<0.05'],
        errors: ['rate<0.05'],
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_KEY = __ENV.API_KEY || 'test-api-key';

// =============================================================================
// Генераторы данных из фикстур
// =============================================================================

function randomDestination() {
    return destinations[Math.floor(Math.random() * destinations.length)];
}

function randomDynamicDestination() {
    // 70% — из фикстур (кешированные маршруты), 30% — случайные (новые маршруты)
    if (Math.random() < 0.7) {
        return randomDestination();
    }
    const prefixes = ['7910', '7903', '7920', '7900', '7999', '7985', '7925'];
    const prefix = prefixes[Math.floor(Math.random() * prefixes.length)];
    return prefix + Math.floor(Math.random() * 10000000).toString().padStart(7, '0');
}

function randomSource() {
    return sources[Math.floor(Math.random() * sources.length)];
}

function randomText() {
    let text = messageTemplates[Math.floor(Math.random() * messageTemplates.length)];
    // Подставляем плейсхолдеры
    text = text.replace('{code}', Math.floor(100000 + Math.random() * 900000));
    text = text.replace('{order_id}', Math.floor(10000 + Math.random() * 90000));
    text = text.replace('{amount}', (Math.random() * 10000).toFixed(2));
    text = text.replace('{balance}', (Math.random() * 100000).toFixed(2));
    text = text.replace('{receipt_id}', Math.floor(1000000 + Math.random() * 9000000));
    text = text.replace('{tracking}', 'LT' + Math.floor(1000000000 + Math.random() * 9000000000));
    text = text.replace('{discount}', Math.floor(5 + Math.random() * 75));
    text = text.replace('{promo}', Math.floor(1000 + Math.random() * 9000));
    text = text.replace('{time}', `${Math.floor(8 + Math.random() * 12)}:${Math.floor(Math.random() * 60).toString().padStart(2, '0')}`);
    text = text.replace('{ip}', `${Math.floor(1 + Math.random() * 254)}.${Math.floor(Math.random() * 256)}.${Math.floor(Math.random() * 256)}.${Math.floor(1 + Math.random() * 254)}`);
    text = text.replace('{server}', `srv-${Math.floor(1 + Math.random() * 50)}.prod`);
    text = text.replace('{rate}', Math.floor(100 + Math.random() * 9900));
    text = text.replace('{flight}', Math.floor(100 + Math.random() * 900));
    text = text.replace('{dep_time}', `${Math.floor(Math.random() * 24).toString().padStart(2, '0')}:${Math.floor(Math.random() * 60).toString().padStart(2, '0')}`);
    text = text.replace('{arr_time}', `${Math.floor(Math.random() * 24).toString().padStart(2, '0')}:${Math.floor(Math.random() * 60).toString().padStart(2, '0')}`);
    text = text.replace('{gate}', `${Math.floor(1 + Math.random() * 30)}${['A', 'B', 'C'][Math.floor(Math.random() * 3)]}`);
    text = text.replace('{tx_id}', Math.floor(10000000 + Math.random() * 90000000));
    return text;
}

// =============================================================================
// Эндпоинты
// =============================================================================

const headers = {
    'Content-Type': 'application/json',
    'X-API-Key': API_KEY,
};

function sendSMS() {
    const payload = JSON.stringify({
        source: randomSource(),
        destination: randomDynamicDestination(),
        text: randomText(),
        external_id: `k6-${__VU}-${__ITER}-${Date.now()}`,
        registered_delivery: Math.random() > 0.5,
    });

    const res = http.post(`${BASE_URL}/api/v1/sms/send`, payload, {
        headers,
        tags: { name: 'SendSMS' },
    });

    sendLatency.add(res.timings.duration);
    sendCounter.add(1);

    return check(res, {
        'send: status 200': (r) => r.status === 200,
        'send: has message_id': (r) => {
            try { return JSON.parse(r.body).message_id !== undefined; }
            catch { return false; }
        },
        'send: latency < 500ms': (r) => r.timings.duration < 500,
    });
}

function sendBatch() {
    const batchSize = 5 + Math.floor(Math.random() * 16); // 5-20 сообщений
    const messages = [];
    for (let i = 0; i < batchSize; i++) {
        messages.push({
            source: randomSource(),
            destination: randomDynamicDestination(),
            text: randomText(),
            external_id: `k6-batch-${__VU}-${__ITER}-${i}`,
        });
    }

    const res = http.post(`${BASE_URL}/api/v1/sms/batch`, JSON.stringify({ messages }), {
        headers,
        tags: { name: 'SendBatch' },
    });

    batchCounter.add(batchSize);

    return check(res, {
        'batch: status 200': (r) => r.status === 200,
        'batch: has results': (r) => {
            try { return JSON.parse(r.body).results !== undefined; }
            catch { return false; }
        },
    });
}

function getHistory() {
    const statuses = ['', 'pending', 'queued', 'sent', 'delivered', 'failed'];
    const status = statuses[Math.floor(Math.random() * statuses.length)];
    const limit = [10, 20, 50, 100][Math.floor(Math.random() * 4)];
    let url = `${BASE_URL}/api/v1/sms/history?limit=${limit}`;
    if (status) url += `&status=${status}`;

    const res = http.get(url, {
        headers: { 'X-API-Key': API_KEY },
        tags: { name: 'GetHistory' },
    });

    historyCounter.add(1);

    return check(res, {
        'history: status 200': (r) => r.status === 200,
    });
}

function healthCheck() {
    const res = http.get(`${BASE_URL}/health`, {
        tags: { name: 'Health' },
    });

    return check(res, {
        'health: status 200': (r) => r.status === 200,
    });
}

// =============================================================================
// Основная функция — mixed load
// =============================================================================

export default function () {
    const roll = Math.random() * 100;
    let success;

    if (roll < 70) {
        success = sendSMS();
    } else if (roll < 85) {
        success = sendBatch();
    } else if (roll < 93) {
        success = getHistory();
    } else {
        success = healthCheck();
    }

    errorRate.add(!success);
    sleep(0.1);
}

// =============================================================================
// Отчёт
// =============================================================================

export function handleSummary(data) {
    const summary = generateSummary(data);
    return {
        'stdout': summary,
        'load-test-summary.json': JSON.stringify(data, null, 2),
    };
}

function generateSummary(data) {
    let s = '\n';
    s += '='.repeat(60) + '\n';
    s += '  SMS Gateway Load Test Results (with fixtures)\n';
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
        const f = data.metrics.http_req_failed.values;
        s += `Error rate: ${((f.rate || 0) * 100).toFixed(2)}%\n`;
    }
    if (data.metrics.sms_sent) {
        s += `SMS sent:   ${data.metrics.sms_sent.values.count || 0}\n`;
    }
    if (data.metrics.sms_batch_sent) {
        s += `Batch SMS:  ${data.metrics.sms_batch_sent.values.count || 0}\n`;
    }

    s += '\n' + '='.repeat(60) + '\n';
    return s;
}
