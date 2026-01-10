import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';
import { Counter } from 'k6/metrics';
import { Trend } from 'k6/metrics';

// Кастомные метрики
const errorRate = new Rate('errors');
const requestCounter = new Counter('total_requests');
const latencyTrend = new Trend('request_latency');

// Конфигурация для достижения 10K сообщений/сек
export const options = {
    stages: [
        // Warm-up: постепенное увеличение нагрузки
        { duration: '30s', target: 100 },   // 100 VUs
        { duration: '1m', target: 500 },    // 500 VUs
        { duration: '1m', target: 1000 },   // 1000 VUs
        
        // Основная нагрузка: поддержание 10K msg/s
        // При 10 запросов на VU в секунду нужно ~1000 VUs
        // Но для надежности используем больше VUs с меньшим RPS на каждого
        { duration: '5m', target: 2000 },   // 2000 VUs (~5 req/s на VU = 10K req/s)
        
        // Пиковая нагрузка
        { duration: '2m', target: 2500 },   // 2500 VUs
        
        // Устойчивая нагрузка
        { duration: '10m', target: 2000 },  // 10 минут устойчивой нагрузки
        
        // Снижение нагрузки
        { duration: '2m', target: 1000 },
        { duration: '1m', target: 500 },
        { duration: '30s', target: 0 },
    ],
    thresholds: {
        // Целевые метрики для 10K msg/s
        http_reqs: ['rate>=10000'],           // Минимум 10K запросов в секунду
        http_req_duration: ['p(95)<500', 'p(99)<1000'],  // p95 < 500ms, p99 < 1s
        http_req_failed: ['rate<0.01'],       // Меньше 1% ошибок
        errors: ['rate<0.01'],                 // Меньше 1% ошибок
        request_latency: ['p(95)<500', 'p(99)<1000'],
    },
};

// Базовый URL API Gateway
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_KEY = __ENV.API_KEY || 'test-api-key';

// Пул HTTP соединений для переиспользования
const httpOptions = {
    timeout: '30s',
    tags: { name: 'SendSMS' },
};

// Функция для генерации случайного номера телефона
function randomPhoneNumber() {
    const prefix = '7900';
    const suffix = Math.floor(Math.random() * 10000000).toString().padStart(7, '0');
    return prefix + suffix;
}

// Функция для генерации случайного источника
function randomSource() {
    const sources = ['12345', '67890', 'SMS', 'API', 'TEST'];
    return sources[Math.floor(Math.random() * sources.length)];
}

// Основная функция теста
export default function () {
    // Подготовка данных запроса
    const payload = JSON.stringify({
        source: randomSource(),
        destination: randomPhoneNumber(),
        text: `10K load test message #${__VU}-${__ITER} at ${Date.now()}`,
    });

    const params = {
        ...httpOptions,
        headers: {
            'Content-Type': 'application/json',
            'X-API-Key': API_KEY,
        },
    };

    // Отправка запроса
    const start = Date.now();
    const res = http.post(`${BASE_URL}/api/v1/sms/send`, payload, params);
    const latency = Date.now() - start;

    // Регистрация метрик
    requestCounter.add(1);
    latencyTrend.add(latency);

    // Проверка результата
    const success = check(res, {
        'status is 200 or 202': (r) => r.status === 200 || r.status === 202,
        'response has message_id': (r) => {
            try {
                const body = JSON.parse(r.body);
                return body.message_id !== undefined;
            } catch (e) {
                return false;
            }
        },
        'response time < 1000ms': (r) => r.timings.duration < 1000,
    });

    // Регистрация ошибок
    errorRate.add(!success);

    // Минимальная задержка (10 запросов в секунду на VU = 100ms задержка)
    // Но для достижения 10K msg/s с 2000 VUs нужно ~5 req/s на VU = 200ms
    sleep(0.2);
}

// Функция для обработки результатов
export function handleSummary(data) {
    const summary = generateTextSummary(data);
    return {
        'stdout': summary,
        'summary.json': JSON.stringify(data, null, 2),
    };
}

// Генерация текстового summary
function generateTextSummary(data) {
    let summary = '\n';
    summary += '═══════════════════════════════════════════════════════\n';
    summary += '  10K Messages/Second Load Test Results\n';
    summary += '═══════════════════════════════════════════════════════\n\n';

    // HTTP метрики
    if (data.metrics.http_reqs) {
        const reqs = data.metrics.http_reqs;
        summary += 'HTTP Requests:\n';
        summary += `  Total: ${reqs.values.count || 0}\n`;
        summary += `  Rate: ${(reqs.values.rate || 0).toFixed(2)} req/s\n`;
        summary += `  Target: >= 10000 req/s\n`;
        if (reqs.values.rate >= 10000) {
            summary += '  ✅ Target achieved!\n';
        } else {
            summary += `  ⚠️  Target not achieved (${((reqs.values.rate / 10000) * 100).toFixed(1)}%)\n`;
        }
        summary += '\n';
    }

    // Latency метрики
    if (data.metrics.http_req_duration) {
        const duration = data.metrics.http_req_duration;
        summary += 'HTTP Request Duration:\n';
        summary += `  min: ${(duration.values.min || 0).toFixed(2)} ms\n`;
        summary += `  avg: ${(duration.values.avg || 0).toFixed(2)} ms\n`;
        summary += `  max: ${(duration.values.max || 0).toFixed(2)} ms\n`;
        if (duration.values['p(95)'] !== undefined) {
            summary += `  p95: ${duration.values['p(95)'].toFixed(2)} ms\n`;
        }
        if (duration.values['p(99)'] !== undefined) {
            summary += `  p99: ${duration.values['p(99)'].toFixed(2)} ms\n`;
        }
        summary += `  Target: p95 < 500ms, p99 < 1000ms\n`;
        const p95Ok = duration.values['p(95)'] < 500;
        const p99Ok = duration.values['p(99)'] < 1000;
        if (p95Ok && p99Ok) {
            summary += '  ✅ Latency targets met!\n';
        } else {
            summary += `  ⚠️  Latency targets not met (p95: ${p95Ok ? 'OK' : 'FAIL'}, p99: ${p99Ok ? 'OK' : 'FAIL'})\n`;
        }
        summary += '\n';
    }

    // Ошибки
    if (data.metrics.http_req_failed) {
        const failed = data.metrics.http_req_failed;
        const failRate = (failed.values.rate || 0) * 100;
        summary += 'Failed Requests:\n';
        summary += `  Rate: ${failRate.toFixed(2)}%\n`;
        summary += `  Count: ${failed.values.passes || 0}\n`;
        summary += `  Target: < 1%\n`;
        if (failRate < 1) {
            summary += '  ✅ Error rate target met!\n';
        } else {
            summary += `  ⚠️  Error rate too high\n`;
        }
        summary += '\n';
    }

    // VUs
    if (data.metrics.vus) {
        const vus = data.metrics.vus;
        summary += 'Virtual Users:\n';
        summary += `  max: ${vus.values.max || 0}\n`;
        summary += `  min: ${vus.values.min || 0}\n`;
        summary += '\n';
    }

    // Общий статус
    summary += '═══════════════════════════════════════════════════════\n';
    const allTargetsMet = 
        (data.metrics.http_reqs?.values.rate || 0) >= 10000 &&
        (data.metrics.http_req_failed?.values.rate || 0) < 0.01 &&
        (data.metrics.http_req_duration?.values['p(95)'] || 1000) < 500;

    if (allTargetsMet) {
        summary += '  ✅ All performance targets achieved!\n';
    } else {
        summary += '  ⚠️  Some performance targets not met\n';
    }
    summary += '═══════════════════════════════════════════════════════\n';

    return summary;
}