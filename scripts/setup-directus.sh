#!/bin/bash

# Скрипт настройки коллекций в Directus
#
# Использование:
#   ./scripts/setup-directus.sh [--url http://localhost:8055] [--email admin@example.com] [--password admin]

set -e

DIRECTUS_URL="${DIRECTUS_URL:-http://localhost:8055}"
DIRECTUS_EMAIL="${DIRECTUS_ADMIN_EMAIL:-admin@example.com}"
DIRECTUS_PASSWORD="${DIRECTUS_ADMIN_PASSWORD:-admin}"

# Парсинг аргументов
while [[ $# -gt 0 ]]; do
    case $1 in
        --url)
            DIRECTUS_URL="$2"
            shift 2
            ;;
        --email)
            DIRECTUS_EMAIL="$2"
            shift 2
            ;;
        --password)
            DIRECTUS_PASSWORD="$2"
            shift 2
            ;;
        *)
            echo "Неизвестный аргумент: $1"
            echo "Использование: $0 [--url URL] [--email EMAIL] [--password PASSWORD]"
            exit 1
            ;;
    esac
done

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

echo "=== Настройка коллекций Directus ==="
echo "URL: $DIRECTUS_URL"
echo "Email: $DIRECTUS_EMAIL"
echo ""

# Проверка наличия Node.js
if ! command -v node &> /dev/null; then
    echo "❌ Node.js не найден. Установите Node.js или запустите скрипт в Docker контейнере."
    exit 1
fi

# Запуск скрипта настройки
export DIRECTUS_URL="$DIRECTUS_URL"
export DIRECTUS_ADMIN_EMAIL="$DIRECTUS_EMAIL"
export DIRECTUS_ADMIN_PASSWORD="$DIRECTUS_PASSWORD"

node scripts/setup-directus.js \
    --url "$DIRECTUS_URL" \
    --email "$DIRECTUS_EMAIL" \
    --password "$DIRECTUS_PASSWORD"
