# Remote Server Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a single bash script `scripts/server.sh` for deploying and managing the SMS platform on a remote server via SSH, usable by AI agents and humans alike.

**Architecture:** One bash script with subcommands (setup, deploy, exec, logs, status, restart, stop, migrate, seed, sync). Uses rsync for file transfer and SSH for remote command execution. SSH alias `sms-server` abstracts connection details.

**Tech Stack:** Bash, rsync, SSH, Docker Compose

---

### Task 1: Create `scripts/server.sh` with core structure and help

**Files:**
- Create: `scripts/server.sh`

- [ ] **Step 1: Create the script with config, helper functions, and usage**

```bash
#!/bin/bash
set -euo pipefail

# === Configuration ===
SSH_HOST="sms-server"
REMOTE_DIR="/opt/sms"
COMPOSE_FILE="deployments/docker-compose.yml"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

RSYNC_EXCLUDES=(
    .git/
    node_modules/
    test/
    docs/
    specs/
    "*.md"
    pipeline-worker
    worker
)

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
    local exclude_args=()
    for pattern in "${RSYNC_EXCLUDES[@]}"; do
        exclude_args+=(--exclude "$pattern")
    done
    info "Синхронизация файлов на $SSH_HOST:$REMOTE_DIR ..."
    rsync -az --delete "${exclude_args[@]}" "$PROJECT_ROOT/" "$SSH_HOST:$REMOTE_DIR/"
    ok "Файлы синхронизированы"
}

usage() {
    cat <<'EOF'
Usage: scripts/server.sh <command> [args]

Commands:
  setup              Initial server setup: create dir, sync, build & start
  deploy [service]   Sync files and rebuild (all or one service)
  sync               Sync files only (no restart)
  status             Show container statuses
  logs <service>     Tail logs for a service (last 100 lines + follow)
  restart <service>  Restart a service
  stop               Stop all services
  exec <command>     Run arbitrary command on server
  migrate            Apply database migrations
  seed               Load test data from seed.sql

Examples:
  scripts/server.sh deploy
  scripts/server.sh deploy messaging-service
  scripts/server.sh logs worker
  scripts/server.sh exec "docker ps"
  scripts/server.sh exec "df -h"
EOF
}

# === Commands ===
cmd_setup() {
    info "Первоначальная настройка сервера..."
    remote "mkdir -p $REMOTE_DIR"
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
    rsync -az "$PROJECT_ROOT/test/load/fixtures/seed.sql" "$SSH_HOST:$REMOTE_DIR/test/load/fixtures/"
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
```

- [ ] **Step 2: Make script executable**

Run: `chmod +x scripts/server.sh`

- [ ] **Step 3: Verify script parses without errors**

Run: `bash -n scripts/server.sh`
Expected: no output (no syntax errors)

- [ ] **Step 4: Verify help output**

Run: `scripts/server.sh`
Expected: usage text with all commands listed

- [ ] **Step 5: Commit**

```bash
git add scripts/server.sh
git commit -m "feat: add remote server management script"
```

---

### Task 2: Update CLAUDE.md with server management instructions

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add Server Management section to CLAUDE.md**

Add before `<!-- MANUAL ADDITIONS START -->`:

```markdown
## Server Management

Сервер: claude@72.56.232.202 (SSH alias: sms-server), деплой в /opt/sms

Управление через `scripts/server.sh <command>`:
- setup — первоначальная настройка сервера
- deploy — синхронизировать и перезапустить
- deploy <service> — обновить один сервис
- status — статус контейнеров
- logs <service> — логи сервиса (tail -f)
- exec "<cmd>" — выполнить команду на сервере
- migrate — применить миграции БД
- seed — накатить тестовые данные
- restart <service> / stop — управление стеком
- sync — только синхронизация файлов

Перед деплоем всегда проверяй что код компилируется локально.
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add server management instructions to CLAUDE.md"
```

---

### Task 3: SSH setup and end-to-end verification

**Prerequisites:** SSH key must be configured before this task.

- [ ] **Step 1: Check if SSH key exists**

Run: `ls ~/.ssh/id_*.pub 2>/dev/null || echo "no key"`

If no key: `ssh-keygen -t ed25519 -C "sms-deploy" -f ~/.ssh/id_ed25519 -N ""`

- [ ] **Step 2: Copy SSH key to server**

Run: `ssh-copy-id claude@72.56.232.202`
Enter password when prompted.

- [ ] **Step 3: Add SSH alias to ~/.ssh/config**

Append to `~/.ssh/config` (create if missing):

```
Host sms-server
    HostName 72.56.232.202
    User claude
    Port 22
```

- [ ] **Step 4: Verify passwordless SSH works**

Run: `ssh sms-server echo "ok"`
Expected: `ok`

- [ ] **Step 5: Test full setup command**

Run: `scripts/server.sh setup`
Expected: files synced, Docker Compose builds and starts on server

- [ ] **Step 6: Test status command**

Run: `scripts/server.sh status`
Expected: list of running containers

- [ ] **Step 7: Test exec command**

Run: `scripts/server.sh exec "docker --version"`
Expected: Docker version output
