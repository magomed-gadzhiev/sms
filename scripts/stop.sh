#!/bin/bash

# Bash скрипт для остановки SMPP Server

set -e

CLEAN=false

# Парсинг аргументов
while [[ $# -gt 0 ]]; do
    case $1 in
        --clean)
            CLEAN=true
            shift
            ;;
        *)
            echo "Неизвестный аргумент: $1"
            echo "Использование: $0 [--clean]"
            exit 1
            ;;
    esac
done

echo "=== Остановка SMPP Server ==="

# Переход в корень проекта
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

echo -e "\nОстановка сервисов..."

if [ "$CLEAN" = true ]; then
    echo "Остановка с удалением volumes (все данные будут удалены)..."
    docker-compose -f deployments/docker-compose.yml down -v
else
    docker-compose -f deployments/docker-compose.yml down
fi

if [ $? -eq 0 ]; then
    echo -e "\n✓ Сервисы остановлены"
else
    echo -e "\n✗ Ошибка остановки сервисов"
    exit 1
fi
