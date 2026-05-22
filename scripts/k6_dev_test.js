import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Counter } from 'k6/metrics';
import { SharedArray } from 'k6/data';

// Облегчённый нагрузочный тест для разработки
// ~1 минута, 10-30 VU, ~500-1500 сообщений
// Использование: docker run --rm --network deployments_smpp-network \
//   -v $(pwd)/scripts:/scripts:ro -v $(pwd)/test:/test:ro \
//   -e BASE_URL=http://client-gateway-1:8080 -e API_KEY=lt-high-volume-key-001 \
//   grafana/k6:latest run /scripts/k6_dev_test.js

const destinations = new SharedArray('destinations', function () {
    const data = JSON.parse(open('../test/load/fixtures/destinations.json'));
    const all = [];
    for (const group of data.domestic) all.push(...group.numbers);
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

export const options = {
    stages: [
        { duration: '10s', target: 10 },
        { duration: '30s', target: 30 },
        { duration: '15s', target: 10 },
        { duration: '5s',  target: 0 },
    ],
    thresholds: {
        http_req_failed: ['rate<0.10'],
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://host.docker.internal:8080';
const API_KEY = __ENV.API_KEY || 'test-api-key';

function randomItem(arr) { return arr[Math.floor(Math.random() * arr.length)]; }

function randomText() {
    let text = randomItem(messageTemplates);
    text = text.replace('{code}', Math.floor(100000 + Math.random() * 900000));
    text = text.replace('{order_id}', Math.floor(10000 + Math.random() * 90000));
    text = text.replace('{amount}', (Math.random() * 10000).toFixed(2));
    text = text.replace('{balance}', (Math.random() * 100000).toFixed(2));
    return text;
}

const headers = { 'Content-Type': 'application/json', 'X-API-Key': API_KEY };

export default function () {
    const payload = JSON.stringify({
        source: randomItem(sources),
        destination: randomItem(destinations),
        text: randomText(),
        external_id: `dev-${__VU}-${__ITER}-${Date.now()}`,
    });
    const res = http.post(`${BASE_URL}/api/v1/sms/send`, payload, { headers });
    sendCounter.add(1);
    const success = check(res, {
        'status 200': (r) => r.status === 200,
    });
    errorRate.add(!success);
    sleep(0.1);
}

export function handleSummary(data) {
    let s = '\n  Dev Load Test: ';
    if (data.metrics.sms_sent) s += `${data.metrics.sms_sent.values.count} SMS`;
    if (data.metrics.http_reqs) s += `, ${(data.metrics.http_reqs.values.rate || 0).toFixed(0)} req/s`;
    if (data.metrics.http_req_duration) s += `, avg=${(data.metrics.http_req_duration.values.avg || 0).toFixed(0)}ms`;
    if (data.metrics.http_req_failed) s += `, err=${((data.metrics.http_req_failed.values.rate || 0) * 100).toFixed(1)}%`;
    s += '\n';
    return { 'stdout': s };
}
