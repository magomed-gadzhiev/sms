#!/bin/bash

# Bash скрипт для запуска SMPP Server

set -e

BUILD=false
MIGRATE=false
CLEAN=false

# Парсинг аргументов
while [[ $# -gt 0 ]]; do
    case $1 in
        --build)
            BUILD=true
            shift
            ;;
        --migrate)
            MIGRATE=true
            shift
            ;;
        --clean)
            CLEAN=true
            shift
            ;;
        *)
            echo "Неизвестный аргумент: $1"
            echo "Использование: $0 [--build] [--migrate] [--clean]"
            exit 1
            ;;
    esac
done

echo "=== SMPP Server Startup Script ==="

# Проверка Docker
echo -e "\nПроверка Docker..."
if ! command -v docker &> /dev/null; then
    echo "✗ Docker не найден. Установите Docker"
    exit 1
fi
echo "✓ Docker установлен: $(docker --version)"

# Проверка docker-compose
echo -e "\nПроверка docker-compose..."
if ! command -v docker-compose &> /dev/null; then
    echo "✗ docker-compose не найден"
    exit 1
fi
echo "✓ docker-compose установлен: $(docker-compose --version)"

# Переход в корень проекта
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

echo -e "\nРабочая директория: $PROJECT_ROOT"

# Очистка (если указан флаг)
if [ "$CLEAN" = true ]; then
    echo -e "\nОчистка контейнеров и volumes..."
    docker-compose -f deployments/docker-compose.yml down -v
    echo "✓ Очистка завершена"
fi

# Запуск инфраструктурных сервисов
echo -e "\nЗапуск инфраструктурных сервисов..."
docker-compose -f deployments/docker-compose.yml up -d postgres redis zookeeper kafka

echo -e "\nОжидание готовности PostgreSQL..."
RETRIES=30
READY=false
for i in $(seq 1 $RETRIES); do
    if docker-compose -f deployments/docker-compose.yml exec -T postgres pg_isready -U smpp &> /dev/null; then
        READY=true
        break
    fi
    sleep 2
    echo -n "."
done
echo ""

if [ "$READY" = false ]; then
    echo "✗ PostgreSQL не запустился"
    exit 1
fi
echo "✓ PostgreSQL готов"

# Применение миграций (если указан флаг)
if [ "$MIGRATE" = true ]; then
    echo -e "\nПрименение миграций БД..."
    docker run --rm \
        -v "${PROJECT_ROOT}/migrations:/migrations" \
        --network deployments_smpp-network \
        migrate/migrate \
        -path /migrations \
        -database "postgres://smpp:smpp_password@postgres:5432/smpp_db?sslmode=disable" \
        up
    
    if [ $? -eq 0 ]; then
        echo "✓ Миграции применены"
    else
        echo "✗ Ошибка применения миграций"
        exit 1
    fi
fi

# Сборка и запуск приложений
if [ "$BUILD" = true ]; then
    echo -e "\nСборка и запуск приложений..."
    docker-compose -f deployments/docker-compose.yml up -d --build \
        api-gateway-1 api-gateway-2 smpp-server worker-1 worker-2 \
        haproxy prometheus grafana
else
    echo -e "\nЗапуск приложений..."
    docker-compose -f deployments/docker-compose.yml up -d \
        api-gateway-1 api-gateway-2 smpp-server worker-1 worker-2 \
        haproxy prometheus grafana
fi

if [ $? -ne 0 ]; then
    echo "✗ Ошибка запуска приложений"
    exit 1
fi

# Проверка статуса
echo -e "\nОжидание запуска сервисов..."
sleep 10

echo -e "\nСтатус сервисов:"
docker-compose -f deployments/docker-compose.yml ps

echo -e "\n=== SMPP Server запущен ==="
echo -e "\nДоступные endpoints:"
echo "  HTTP API:    http://localhost:8080"
echo "  gRPC API:    http://localhost:9090"
echo "  SMPP Server: localhost:2775"
echo "  Grafana:     http://localhost:3000 (admin/admin)"
echo "  Prometheus:  http://localhost:9091"
echo "  HAProxy:     http://localhost:8404/stats"

echo -e "\nДля просмотра логов:"
echo "  docker-compose -f deployments/docker-compose.yml logs -f [service-name]"

echo -e "\nДля остановки:"
echo "  ./scripts/stop.sh"
