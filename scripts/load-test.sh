#!/bin/bash
# Скрипт для запуска нагрузочного тестирования с помощью k6
# Использование: ./scripts/load-test.sh [--base-url <url>] [--api-key <key>] [--duration <duration>] [--vus <vus>]

BASE_URL="${BASE_URL:-http://localhost:8080}"
API_KEY="${API_KEY:-test-api-key}"
DURATION=0
VUS=0

# Парсинг аргументов
while [[ $# -gt 0 ]]; do
    case $1 in
        --base-url)
            BASE_URL="$2"
            shift 2
            ;;
        --api-key)
            API_KEY="$2"
            shift 2
            ;;
        --duration)
            DURATION="$2"
            shift 2
            ;;
        --vus)
            VUS="$2"
            shift 2
            ;;
        *)
            echo "Неизвестный параметр: $1"
            echo "Использование: $0 [--base-url <url>] [--api-key <key>] [--duration <duration>] [--vus <vus>]"
            exit 1
            ;;
    esac
done

echo "========================================"
echo "  Нагрузочное тестирование SMPP Server"
echo "========================================"
echo ""

# Проверка Docker
if ! command -v docker &> /dev/null; then
    echo "Ошибка: Docker не установлен"
    exit 1
fi

# Проверка доступности Docker
if ! docker ps &> /dev/null; then
    echo "Ошибка: Docker не запущен или недоступен"
    exit 1
fi

# Проверка сервисов
echo "Проверка доступности сервисов..."
SERVICES=$(docker ps --format "{{.Names}}" | grep -E "haproxy|api-gateway" || true)
if [ -z "$SERVICES" ]; then
    echo "Предупреждение: Не найдены запущенные сервисы (haproxy, api-gateway)"
    echo "Убедитесь, что сервисы запущены: ./scripts/start.sh"
    echo ""
fi

# Определение URL для Docker сети
DOCKER_NETWORK="deployments_smpp-network"
if docker network ls --format "{{.Name}}" | grep -q "$DOCKER_NETWORK"; then
    TEST_URL="http://haproxy:8080"
    echo "Используется Docker сеть: $DOCKER_NETWORK"
    echo "URL для тестирования: $TEST_URL"
else
    TEST_URL="$BASE_URL"
    echo "Используется локальный URL: $TEST_URL"
fi

echo ""
echo "Параметры теста:"
echo "  Base URL: $TEST_URL"
echo "  API Key: $API_KEY"
if [ "$DURATION" -gt 0 ]; then
    echo "  Duration: ${DURATION}s"
fi
if [ "$VUS" -gt 0 ]; then
    echo "  Virtual Users: $VUS"
fi
echo ""

# Подготовка команды k6
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SCRIPT_PATH="$SCRIPT_DIR/k6_load_test.js"

K6_CMD="docker run --rm"
if docker network ls --format "{{.Name}}" | grep -q "$DOCKER_NETWORK"; then
    K6_CMD="$K6_CMD --network $DOCKER_NETWORK"
fi
K6_CMD="$K6_CMD -v \"$SCRIPT_PATH:/scripts/k6_load_test.js:ro\""
K6_CMD="$K6_CMD grafana/k6 run"
K6_CMD="$K6_CMD /scripts/k6_load_test.js"
K6_CMD="$K6_CMD -e BASE_URL=$TEST_URL"
K6_CMD="$K6_CMD -e API_KEY=$API_KEY"

if [ "$DURATION" -gt 0 ]; then
    K6_CMD="$K6_CMD --duration ${DURATION}s"
fi

if [ "$VUS" -gt 0 ]; then
    K6_CMD="$K6_CMD --vus $VUS"
fi

echo "Запуск нагрузочного теста..."
echo "Команда: $K6_CMD"
echo ""

# Запуск теста
eval $K6_CMD

echo ""
echo "Тест завершен!"
