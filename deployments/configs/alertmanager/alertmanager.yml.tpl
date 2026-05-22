# Template — все ${VAR} resolve'ятся в entrypoint.sh через sed (envsubst
# отсутствует в prom/alertmanager:v0.27.0 busybox-image, apk недоступен).
# Финальный alertmanager.yml пишется в /tmp/alertmanager.yml на старте
# (mount /etc/alertmanager — read-only).

global:
  smtp_smarthost: '${ALERTMANAGER_SMTP_HOST}'
  smtp_from: '${ALERTMANAGER_SMTP_FROM}'
  smtp_auth_username: '${ALERTMANAGER_SMTP_USER}'
  smtp_auth_password: '${ALERTMANAGER_SMTP_PASSWORD}'
  smtp_require_tls: true
  resolve_timeout: 5m

route:
  group_by: ['alertname', 'severity', 'env']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  receiver: 'email-default'
  routes:
    # Sandbox → noop receiver ВСЕГДА (включая critical) — логируем, не
    # email'им. ДОЛЖЕН быть первым: alertmanager evaluates routes top-down,
    # default continue=false; sandbox-critical иначе попадает в email-critical
    # → SMTP fail на dummy creds.
    - matchers:
        - env = sandbox
      receiver: 'log-only'
    # Prod critical → email-critical (1h repeat вместо 4h, 10s group_wait).
    - matchers:
        - severity = critical
      receiver: 'email-critical'
      group_wait: 10s
      repeat_interval: 1h

receivers:
  - name: 'email-default'
    email_configs:
      - to: '${ALERTMANAGER_TO}'
        send_resolved: true
        headers:
          Subject: '[SMS-Platform {{ .CommonLabels.env }}] {{ .CommonLabels.alertname }} ({{ .Status }})'

  - name: 'email-critical'
    email_configs:
      - to: '${ALERTMANAGER_TO}'
        send_resolved: true
        headers:
          Subject: '[SMS-Platform CRITICAL {{ .CommonLabels.env }}] {{ .CommonLabels.alertname }}'

  - name: 'log-only'
    # Без email_configs — alert логируется в alertmanager logs, никуда не уходит.
    # Используется для sandbox env.

inhibit_rules:
  - source_matchers:
      - severity = critical
    target_matchers:
      - severity = warning
    equal: ['alertname', 'env']
