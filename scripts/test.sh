#!/bin/bash

# Скрипт для запуска тестов

set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Running tests...${NC}"

# Unit тесты
echo -e "${YELLOW}Running unit tests...${NC}"
go test -v -race -coverprofile=coverage.out ./internal/...

# Показываем coverage
if [ -f coverage.out ]; then
    echo -e "${YELLOW}Coverage report:${NC}"
    go tool cover -func=coverage.out | tail -1
    echo -e "${YELLOW}Opening coverage report in browser...${NC}"
    go tool cover -html=coverage.out -o coverage.html
fi

# Integration тесты (требуют запущенных сервисов)
if [ "$1" == "--integration" ]; then
    echo -e "${YELLOW}Running integration tests...${NC}"
    go test -v -tags=integration ./test/integration/...
fi

# Load тесты (требуют запущенного API Gateway)
if [ "$1" == "--load" ]; then
    echo -e "${YELLOW}Running load tests...${NC}"
    go test -v -tags=load ./test/load/...
fi

echo -e "${GREEN}Tests completed!${NC}"
