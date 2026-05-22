#!/bin/sh
# Resolve env-переменных в config-template'е перед стартом alertmanager.
# Без подмены alertmanager парсит '${VAR}' как литерал и валится.
#
# Используем sed (входит в busybox) вместо envsubst — последний отсутствует
# в prom/alertmanager:v0.27.0 (busybox-минимальный image без apk/gettext).
# Sed-подмена замещает только наши конкретные переменные, не трогая
# Go-template syntax {{ .CommonLabels.env }}.
set -e

TEMPLATE=/etc/alertmanager/alertmanager.yml.tpl
OUT=/tmp/alertmanager.yml

# Escape sed-специальных символов (& / \) в значениях env-vars.
escape_sed() {
    printf '%s' "$1" | sed -e 's/[\/&]/\\&/g'
}

H=$(escape_sed "$ALERTMANAGER_SMTP_HOST")
F=$(escape_sed "$ALERTMANAGER_SMTP_FROM")
U=$(escape_sed "$ALERTMANAGER_SMTP_USER")
P=$(escape_sed "$ALERTMANAGER_SMTP_PASSWORD")
T=$(escape_sed "$ALERTMANAGER_TO")

sed \
    -e "s/\${ALERTMANAGER_SMTP_HOST}/$H/g" \
    -e "s/\${ALERTMANAGER_SMTP_FROM}/$F/g" \
    -e "s/\${ALERTMANAGER_SMTP_USER}/$U/g" \
    -e "s/\${ALERTMANAGER_SMTP_PASSWORD}/$P/g" \
    -e "s/\${ALERTMANAGER_TO}/$T/g" \
    "$TEMPLATE" > "$OUT"

# Проверка, что подмена прошла — иначе alertmanager упадёт с невнятным error.
if grep -q '\${ALERTMANAGER' "$OUT"; then
    echo "ERROR: env-substitution incomplete — переменные не заданы в окружении." >&2
    grep '\${ALERTMANAGER' "$OUT" >&2
    exit 1
fi

exec /bin/alertmanager --config.file="$OUT" --storage.path=/alertmanager
