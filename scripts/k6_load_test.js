import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Кастомная метрика для отслеживания ошибок
const errorRate = new Rate('errors');

// Конфигурация теста - МАКСИМАЛЬНАЯ НАГРУЗКА
// Использует максимум ресурсов компьютера
export const options = {
    stages: [
        { duration: '1m', target: 500 },    // Быстрый разогрев: 500 пользователей за 1 минуту
        { duration: '2m', target: 1000 },   // Высокая нагрузка: 1000 пользователей за 2 минуты
        { duration: '3m', target: 2000 },   // Очень высокая нагрузка: 2000 пользователей за 3 минуты
        { duration: '5m', target: 3000 },   // Пиковая нагрузка: 3000 пользователей за 5 минут
        { duration: '3m', target: 5000 },   // Экстремальная нагрузка: 5000 пользователей за 3 минуты
        { duration: '2m', target: 3000 },   // Снижение до 3000
        { duration: '1m', target: 0 },      // Постепенное снижение до 0
    ],
    thresholds: {
        // Более мягкие пороги для максимальной нагрузки
        http_req_duration: ['p(95)<2000', 'p(99)<5000'], // 95% запросов < 2s, 99% < 5s
        http_req_failed: ['rate<0.05'],                  // Меньше 5% ошибок (более реалистично при максимальной нагрузке)
        errors: ['rate<0.05'],                           // Меньше 5% ошибок
    },
};

// Базовый URL API Gateway
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_KEY = __ENV.API_KEY || 'test-api-key';

// Функция для генерации случайного номера телефона
function randomPhoneNumber() {
    const prefix = '7900';
    const suffix = Math.floor(Math.random() * 10000000).toString().padStart(7, '0');
    return prefix + suffix;
}

// Функция для генерации случайного источника
function randomSource() {
    const sources = ['12345', '67890', 'SMS', 'API'];
    return sources[Math.floor(Math.random() * sources.length)];
}

// Основная функция теста
export default function () {
    // Подготовка данных запроса
    const payload = JSON.stringify({
        source: randomSource(),
        destination: randomPhoneNumber(),
        text: `Load test message #${__VU}-${__ITER}`,
    });

    const params = {
        headers: {
            'Content-Type': 'application/json',
            'X-API-Key': API_KEY,
        },
        tags: {
            name: 'SendSMS',
        },
    };

    // Отправка запроса
    const res = http.post(`${BASE_URL}/api/v1/sms/send`, payload, params);

    // Проверка результата
    const success = check(res, {
        'status is 200': (r) => r.status === 200,
        'response has message_id': (r) => {
            try {
                const body = JSON.parse(r.body);
                return body.message_id !== undefined;
            } catch (e) {
                return false;
            }
        },
        'response time < 500ms': (r) => r.timings.duration < 500,
    });

    // Регистрация ошибок
    errorRate.add(!success);

    // Минимальная задержка для максимальной нагрузки (0.1 секунды вместо 1)
    sleep(0.1);
}

// Функция для обработки результатов
export function handleSummary(data) {
    return {
        'stdout': textSummary(data, { indent: ' ', enableColors: true }),
        'summary.json': JSON.stringify(data),
    };
}

// Функция для текстового summary
function textSummary(data, options) {
    const indent = options.indent || '  ';
    const enableColors = options.enableColors || false;

    let summary = '\n';
    summary += `${indent}Test Summary\n`;
    summary += `${indent}============\n\n`;

    // HTTP метрики
    if (data.metrics.http_req_duration && data.metrics.http_req_duration.values) {
        const duration = data.metrics.http_req_duration;
        const values = duration.values;
        summary += `${indent}HTTP Request Duration:\n`;
        if (values.min !== undefined) summary += `${indent}  min: ${values.min.toFixed(2)}ms\n`;
        if (values.avg !== undefined) summary += `${indent}  avg: ${values.avg.toFixed(2)}ms\n`;
        if (values.max !== undefined) summary += `${indent}  max: ${values.max.toFixed(2)}ms\n`;
        if (values['p(95)'] !== undefined) summary += `${indent}  p95: ${values['p(95)'].toFixed(2)}ms\n`;
        if (values['p(99)'] !== undefined) summary += `${indent}  p99: ${values['p(99)'].toFixed(2)}ms\n`;
        summary += '\n';
    }

    // Throughput
    if (data.metrics.http_reqs && data.metrics.http_reqs.values) {
        const reqs = data.metrics.http_reqs;
        const values = reqs.values;
        summary += `${indent}HTTP Requests:\n`;
        if (values.count !== undefined) summary += `${indent}  total: ${values.count}\n`;
        if (values.rate !== undefined) summary += `${indent}  rate: ${values.rate.toFixed(2)} req/s\n`;
        summary += '\n';
    }

    // Ошибки
    if (data.metrics.http_req_failed && data.metrics.http_req_failed.values) {
        const failed = data.metrics.http_req_failed;
        const values = failed.values;
        summary += `${indent}Failed Requests:\n`;
        if (values.rate !== undefined) summary += `${indent}  rate: ${(values.rate * 100).toFixed(2)}%\n`;
        if (values.passes !== undefined) summary += `${indent}  count: ${values.passes}\n`;
        summary += '\n';
    }

    // VUs
    if (data.metrics.vus && data.metrics.vus.values) {
        const vus = data.metrics.vus;
        const values = vus.values;
        summary += `${indent}Virtual Users:\n`;
        if (values.max !== undefined) summary += `${indent}  max: ${values.max}\n`;
        if (values.min !== undefined) summary += `${indent}  min: ${values.min}\n`;
        summary += '\n';
    }

    return summary;
}
