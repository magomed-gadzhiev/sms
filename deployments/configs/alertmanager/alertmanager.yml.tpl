# Template — все ${VAR} resolve'ятся в entrypoint.sh через envsubst.
# Финальный alertmanager.yml пишется в /etc/alertmanager/alertmanager.yml
# и не bind-mount'ится с host (он генерируется при старте контейнера).

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
    # Critical → отдельный repeat-cadence (1h вместо 4h).
    - matchers:
        - severity = critical
      receiver: 'email-critical'
      group_wait: 10s
      repeat_interval: 1h
    # Sandbox → noop receiver (логируем, не email'им — слишком шумно
    # для тестового traffic'а с ad-hoc провайдер-конфигами).
    - matchers:
        - env = sandbox
      receiver: 'log-only'

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
