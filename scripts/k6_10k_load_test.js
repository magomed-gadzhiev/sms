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
const requestCounter = new Counter('total_requests');
const latencyTrend = new Trend('request_latency');

// =============================================================================
// Конфигурация для достижения 10K msg/s
// =============================================================================

export const options = {
    stages: [
        // Warm-up
        { duration: '30s', target: 100 },
        { duration: '1m',  target: 500 },
        { duration: '1m',  target: 1000 },
        // Основная нагрузка: 2000 VUs * ~5 req/s = 10K req/s
        { duration: '5m',  target: 2000 },
        // Пиковая нагрузка
        { duration: '2m',  target: 2500 },
        // Устойчивая нагрузка
        { duration: '10m', target: 2000 },
        // Снижение
        { duration: '2m',  target: 1000 },
        { duration: '1m',  target: 500 },
        { duration: '30s', target: 0 },
    ],
    thresholds: {
        http_reqs: ['rate>=10000'],
        http_req_duration: ['p(95)<500', 'p(99)<1000'],
        http_req_failed: ['rate<0.01'],
        errors: ['rate<0.01'],
        request_latency: ['p(95)<500', 'p(99)<1000'],
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_KEY = __ENV.API_KEY || 'lt-high-volume-key-001';

// =============================================================================
// Генераторы данных
// =============================================================================

function randomDestination() {
    // 60% — из фикстур, 40% — динамические
    if (Math.random() < 0.6) {
        return destinations[Math.floor(Math.random() * destinations.length)];
    }
    const prefixes = ['7910', '7903', '7920', '7900', '7999', '7916', '7925', '7985', '7906', '7901'];
    const prefix = prefixes[Math.floor(Math.random() * prefixes.length)];
    return prefix + Math.floor(Math.random() * 10000000).toString().padStart(7, '0');
}

function randomSource() {
    return sources[Math.floor(Math.random() * sources.length)];
}

function randomText() {
    let text = messageTemplates[Math.floor(Math.random() * messageTemplates.length)];
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
// Основная функция
// =============================================================================

export default function () {
    const payload = JSON.stringify({
        source: randomSource(),
        destination: randomDestination(),
        text: randomText(),
        external_id: `10k-${__VU}-${__ITER}-${Date.now()}`,
        registered_delivery: Math.random() > 0.7,
    });

    const start = Date.now();
    const res = http.post(`${BASE_URL}/api/v1/sms/send`, payload, {
        headers: {
            'Content-Type': 'application/json',
            'X-API-Key': API_KEY,
        },
        tags: { name: 'SendSMS' },
        timeout: '30s',
    });
    const latency = Date.now() - start;

    requestCounter.add(1);
    latencyTrend.add(latency);

    const success = check(res, {
        'status is 200 or 202': (r) => r.status === 200 || r.status === 202,
        'has message_id': (r) => {
            try { return JSON.parse(r.body).message_id !== undefined; }
            catch { return false; }
        },
        'latency < 1000ms': (r) => r.timings.duration < 1000,
    });

    errorRate.add(!success);

    // 200ms задержка: 2000 VUs * 5 req/s = 10K req/s
    sleep(0.2);
}

// =============================================================================
// Отчёт
// =============================================================================

export function handleSummary(data) {
    const summary = generateSummary(data);
    return {
        'stdout': summary,
        '10k-load-test-summary.json': JSON.stringify(data, null, 2),
    };
}

function generateSummary(data) {
    let s = '\n';
    s += '='.repeat(60) + '\n';
    s += '  10K Messages/Second Load Test Results\n';
    s += '='.repeat(60) + '\n\n';

    const rate = data.metrics.http_reqs?.values?.rate || 0;
    const total = data.metrics.http_reqs?.values?.count || 0;
    const targetMet = rate >= 10000;

    s += `Throughput:  ${rate.toFixed(2)} req/s  (total: ${total})\n`;
    s += `Target:     >= 10,000 req/s  ${targetMet ? '[PASS]' : '[FAIL ' + (rate / 100).toFixed(1) + '%]'}\n\n`;

    if (data.metrics.http_req_duration) {
        const d = data.metrics.http_req_duration.values;
        s += `Latency:\n`;
        s += `  min:  ${(d.min || 0).toFixed(1)} ms\n`;
        s += `  avg:  ${(d.avg || 0).toFixed(1)} ms\n`;
        s += `  p95:  ${(d['p(95)'] || 0).toFixed(1)} ms  (target: < 500ms)  ${(d['p(95)'] || 0) < 500 ? '[PASS]' : '[FAIL]'}\n`;
        s += `  p99:  ${(d['p(99)'] || 0).toFixed(1)} ms  (target: < 1000ms) ${(d['p(99)'] || 0) < 1000 ? '[PASS]' : '[FAIL]'}\n`;
        s += `  max:  ${(d.max || 0).toFixed(1)} ms\n\n`;
    }

    const failRate = (data.metrics.http_req_failed?.values?.rate || 0) * 100;
    s += `Error rate: ${failRate.toFixed(2)}%  (target: < 1%)  ${failRate < 1 ? '[PASS]' : '[FAIL]'}\n\n`;

    if (data.metrics.vus) {
        s += `VUs:        max=${data.metrics.vus.values.max || 0}\n\n`;
    }

    s += '='.repeat(60) + '\n';
    const allPass = targetMet &&
        failRate < 1 &&
        (data.metrics.http_req_duration?.values?.['p(95)'] || 1000) < 500;
    s += allPass ? '  ALL TARGETS MET\n' : '  SOME TARGETS NOT MET\n';
    s += '='.repeat(60) + '\n';

    return s;
}
