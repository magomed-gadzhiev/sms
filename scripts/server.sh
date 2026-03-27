#!/bin/bash
set -euo pipefail

# === Configuration ===
SSH_HOST="sms-server"
REMOTE_DIR="/opt/sms"
COMPOSE_FILE="deployments/docker-compose.yml"
GIT_REPO="git@github.com:magomed-gadzhiev/sms.git"
DEPLOY_BRANCH="${DEPLOY_BRANCH:-master}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# === Helpers ===
info()  { echo -e "\033[1;34m[INFO]\033[0m $*"; }
ok()    { echo -e "\033[1;32m[OK]\033[0m $*"; }
err()   { echo -e "\033[1;31m[ERR]\033[0m $*" >&2; }

remote() {
    ssh "$SSH_HOST" "$@"
}

remote_compose() {
    remote "cd $REMOTE_DIR && docker compose -f $COMPOSE_FILE $*"
}

do_sync() {
    local branch="${1:-$DEPLOY_BRANCH}"
    info "Git pull на $SSH_HOST:$REMOTE_DIR (ветка: $branch) ..."
    remote "cd $REMOTE_DIR && git fetch origin && git checkout $branch && git reset --hard origin/$branch"
    ok "Код обновлён (ветка: $branch)"
}

usage() {
    cat <<'EOF'
Usage: scripts/server.sh <command> [args]

Commands:
  setup              Initial server setup: clone repo, build & start
  deploy [service]   Git pull and rebuild (all or one service)
  sync               Git pull only (no restart)
  status             Show container statuses
  logs <service>     Tail logs for a service (last 100 lines + follow)
  restart <service>  Restart a service
  stop               Stop all services
  exec <command>     Run arbitrary command on server
  migrate            Apply database migrations
  seed               Load test data from seed.sql

Environment:
  DEPLOY_BRANCH      Branch to deploy (default: master)

Examples:
  scripts/server.sh deploy
  scripts/server.sh deploy messaging-service
  DEPLOY_BRANCH=feature/x scripts/server.sh deploy
  scripts/server.sh logs worker
  scripts/server.sh exec "docker ps"
EOF
}

# === Commands ===
cmd_setup() {
    info "Первоначальная настройка сервера..."
    remote "git clone $GIT_REPO $REMOTE_DIR || (cd $REMOTE_DIR && git fetch origin)"
    do_sync
    info "Сборка и запуск стека..."
    remote_compose "up -d --build"
    ok "Сервер настроен и запущен"
    cmd_status
}

cmd_deploy() {
    local service="${1:-}"
    do_sync
    if [ -n "$service" ]; then
        info "Пересборка и перезапуск: $service"
        remote_compose "up -d --build $service"
    else
        info "Пересборка и перезапуск всего стека..."
        remote_compose "up -d --build"
    fi
    ok "Деплой завершён"
    cmd_status
}

cmd_sync() {
    do_sync
}

cmd_status() {
    remote_compose "ps"
}

cmd_logs() {
    local service="${1:?Укажите сервис: scripts/server.sh logs <service>}"
    remote_compose "logs --tail=100 -f $service"
}

cmd_restart() {
    local service="${1:?Укажите сервис: scripts/server.sh restart <service>}"
    info "Перезапуск: $service"
    remote_compose "restart $service"
    ok "$service перезапущен"
}

cmd_stop() {
    info "Остановка стека..."
    remote_compose "down"
    ok "Стек остановлен"
}

cmd_exec() {
    if [ $# -eq 0 ]; then
        err "Укажите команду: scripts/server.sh exec \"<command>\""
        exit 1
    fi
    remote "$*"
}

cmd_migrate() {
    info "Применение миграций..."
    do_sync
    remote "docker run --rm \
        -v $REMOTE_DIR/migrations:/migrations \
        --network deployments_smpp-network \
        migrate/migrate \
        -path /migrations \
        -database 'postgres://smpp:smpp_password@postgres:5432/smpp_db?sslmode=disable' \
        up"
    ok "Миграции применены"
}

cmd_seed() {
    info "Загрузка тестовых данных..."
    do_sync
    remote_compose "exec -T postgres psql -U smpp -d smpp_db < $REMOTE_DIR/test/load/fixtures/seed.sql"
    ok "Тестовые данные загружены"
}

# === Main ===
command="${1:-}"
shift || true

case "$command" in
    setup)   cmd_setup ;;
    deploy)  cmd_deploy "$@" ;;
    sync)    cmd_sync ;;
    status)  cmd_status ;;
    logs)    cmd_logs "$@" ;;
    restart) cmd_restart "$@" ;;
    stop)    cmd_stop ;;
    exec)    cmd_exec "$@" ;;
    migrate) cmd_migrate ;;
    seed)    cmd_seed ;;
    *)       usage; exit 1 ;;
esac
