#!/usr/bin/env bash
# Инициализирует .claude/bugs/state.json, если его нет.
# При corrupt JSON делает backup в state.json.bak.
# Env: BUGS_DIR (default: .claude/bugs)

set -euo pipefail

BUGS_DIR="${BUGS_DIR:-.claude/bugs}"
STATE="$BUGS_DIR/state.json"

mkdir -p "$BUGS_DIR"

write_fresh() {
  cat > "$STATE" <<'EOF'
{
  "version": 1,
  "chrome_pid": null,
  "max_parallel": 3,
  "bugs": {},
  "deploy_queue": [],
  "last_deploy_at": null
}
EOF
}

if [[ ! -f "$STATE" ]]; then
  write_fresh
  echo "state.json создан"
  exit 0
fi

# Проверка валидности JSON
if ! python -c "import json, sys; json.load(open(sys.argv[1]))" "$STATE" 2>/dev/null; then
  cp "$STATE" "$STATE.bak"
  write_fresh
  echo "state.json был corrupt, бэкап в $STATE.bak, создан новый"
  exit 0
fi

echo "state.json валиден, ничего не делаю"
