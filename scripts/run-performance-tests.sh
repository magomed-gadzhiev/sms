#!/bin/bash

# Скрипт для запуска performance тестов
# Использование: ./scripts/run-performance-tests.sh [--integration] [--load] [--performance] [--all]

set -e

INTEGRATION=false
LOAD=false
PERFORMANCE=false

# Парсинг аргументов
while [[ $# -gt 0 ]]; do
    case $1 in
        --integration)
            INTEGRATION=true
            shift
            ;;
        --load)
            LOAD=true
            shift
            ;;
        --performance)
            PERFORMANCE=true
            shift
            ;;
        --all)
            INTEGRATION=true
            LOAD=true
            PERFORMANCE=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Если ничего не указано, запускаем все тесты
if [ "$INTEGRATION" = false ] && [ "$LOAD" = false ] && [ "$PERFORMANCE" = false ]; then
    INTEGRATION=true
    LOAD=true
    PERFORMANCE=true
fi

echo "=== Performance Tests Runner ==="

# Integration тесты
if [ "$INTEGRATION" = true ]; then
    echo ""
    echo "[1/3] Running Integration Tests..."
    go test -tags=integration -v ./test/integration/... -timeout 10m || exit 1
fi

# Performance тесты
if [ "$PERFORMANCE" = true ]; then
    echo ""
    echo "[2/3] Running Performance Tests..."
    go test -tags=integration -v ./test/performance/... -timeout 10m || exit 1
fi

# Load тесты
if [ "$LOAD" = true ]; then
    echo ""
    echo "[3/3] Running Load Tests (10K msg/s target)..."
    echo "Note: Load tests require running services"
    
    # Проверяем доступность API Gateway
    if curl -s -f http://localhost:8080/health > /dev/null 2>&1; then
        echo "API Gateway is available, starting load tests..."
        go test -tags=load -v ./test/load/... -timeout 30m || exit 1
    else
        echo "API Gateway not available at http://localhost:8080"
        echo "Skipping load tests. Start services first with: docker-compose up -d"
    fi
fi

echo ""
echo "=== All tests completed! ==="

# Опционально: запуск k6 тестов
read -p "Run k6 10K load test? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo ""
    echo "Running k6 10K load test..."
    k6 run scripts/k6_10k_load_test.js
fi