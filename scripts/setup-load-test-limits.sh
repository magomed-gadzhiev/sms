#!/bin/bash
# Скрипт для настройки лимитов rate limiting для нагрузочного тестирования
# Использование: ./scripts/setup-load-test-limits.sh [--api-key <key>] [--per-second <n>] [--per-minute <n>] [--per-hour <n>]

API_KEY="${API_KEY:-test-api-key}"
PER_SECOND="${PER_SECOND:-10000}"
PER_MINUTE="${PER_MINUTE:-600000}"
PER_HOUR="${PER_HOUR:-36000000}"

# Парсинг аргументов
while [[ $# -gt 0 ]]; do
    case $1 in
        --api-key)
            API_KEY="$2"
            shift 2
            ;;
        --per-second)
            PER_SECOND="$2"
            shift 2
            ;;
        --per-minute)
            PER_MINUTE="$2"
            shift 2
            ;;
        --per-hour)
            PER_HOUR="$2"
            shift 2
            ;;
        *)
            echo "Неизвестный параметр: $1"
            echo "Использование: $0 [--api-key <key>] [--per-second <n>] [--per-minute <n>] [--per-hour <n>]"
            exit 1
            ;;
    esac
done

echo "========================================"
echo "  Настройка лимитов для нагрузочного тестирования"
echo "========================================"
echo ""

# Проверка Docker
if ! command -v docker &> /dev/null; then
    echo "Ошибка: Docker не установлен"
    exit 1
fi

# Проверка доступности PostgreSQL
if ! docker ps --format "{{.Names}}" | grep -q "postgres"; then
    echo "Ошибка: PostgreSQL контейнер не запущен"
    echo "Запустите сервисы: ./scripts/start.sh"
    exit 1
fi

# Проверка доступности Redis
REDIS_RUNNING=$(docker ps --format "{{.Names}}" | grep -q "redis" && echo "yes" || echo "no")

echo "Параметры:"
echo "  API Key: $API_KEY"
echo "  Лимит в секунду: $PER_SECOND"
echo "  Лимит в минуту: $PER_MINUTE"
echo "  Лимит в час: $PER_HOUR"
echo ""

# Обновление лимитов в PostgreSQL
echo "Обновление лимитов в базе данных..."
UPDATE_QUERY="UPDATE clients SET rate_limit_per_second = $PER_SECOND, rate_limit_per_minute = $PER_MINUTE, rate_limit_per_hour = $PER_HOUR WHERE api_key = '$API_KEY';"
if ! docker exec postgres psql -U smpp -d smpp_db -c "$UPDATE_QUERY" > /dev/null 2>&1; then
    echo "Ошибка при обновлении лимитов в БД"
    exit 1
fi

# Проверка результата
CHECK_QUERY="SELECT name, api_key, rate_limit_per_second, rate_limit_per_minute, rate_limit_per_hour FROM clients WHERE api_key = '$API_KEY';"
CHECK_RESULT=$(docker exec postgres psql -U smpp -d smpp_db -c "$CHECK_QUERY" 2>&1)

if echo "$CHECK_RESULT" | grep -q "0 rows"; then
    echo "Предупреждение: Клиент с API ключом '$API_KEY' не найден в базе данных"
    echo "Создайте клиента перед запуском нагрузочного тестирования"
else
    echo "Лимиты успешно обновлены:"
    echo "$CHECK_RESULT"
fi

# Очистка счетчиков в Redis (если доступен)
if [ "$REDIS_RUNNING" = "yes" ]; then
    echo ""
    echo "Очистка счетчиков rate limiting в Redis..."
    if docker exec redis redis-cli FLUSHDB > /dev/null 2>&1; then
        echo "Счетчики Redis очищены"
    else
        echo "Предупреждение: Не удалось очистить Redis"
    fi
fi

echo ""
echo "Настройка завершена! Теперь можно запускать нагрузочное тестирование."
echo "Команда: ./scripts/load-test.sh"
