#!/bin/sh
# Resolve env-переменных в config-template'е перед стартом alertmanager.
# Без envsubst alertmanager парсит '${VAR}' как литерал и валится.
set -e

# envsubst замещает ТОЛЬКО переменные, перечисленные в формате $VAR/${VAR}.
# Чтобы не задеть Go-template syntax {{ .CommonLabels.env }}, ограничим
# только нашими env-vars через явный список.
TEMPLATE=/etc/alertmanager/alertmanager.yml.tpl
OUT=/tmp/alertmanager.yml

envsubst '$ALERTMANAGER_SMTP_HOST $ALERTMANAGER_SMTP_FROM $ALERTMANAGER_SMTP_USER $ALERTMANAGER_SMTP_PASSWORD $ALERTMANAGER_TO' < "$TEMPLATE" > "$OUT"

# Проверка, что подмена прошла — иначе alertmanager упадёт с невнятным error.
if grep -q '\${ALERTMANAGER' "$OUT"; then
    echo "ERROR: env-substitution incomplete — переменные не заданы в окружении." >&2
    grep '\${ALERTMANAGER' "$OUT" >&2
    exit 1
fi

exec /bin/alertmanager --config.file="$OUT" --storage.path=/alertmanager
