# Intentional Alert Fire Test

## Когда применять

- **Day 3 после cutover** — verify, что alert→email pipeline работает.
- После любого изменения alertmanager config или SMTP credentials.
- Перед onboarding real-public-clients (после canary 24h).

## Method A — manual fire через alertmanager API

Полу-проверка SMTP route'а без реального alert source.

```
ssh prod-host
curl -X POST http://localhost:9093/api/v2/alerts \
    -H 'Content-Type: application/json' \
    -d '[
  {
    "labels": {
      "alertname": "IntentionalTestAlert",
      "severity": "warning",
      "env": "prod",
      "component": "ops-test"
    },
    "annotations": {
      "summary": "Intentional test alert (Plan 7 verification)",
      "description": "If you receive this email, alertmanager → SMTP → ops-mailbox pipeline works."
    },
    "startsAt": "'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'"
  }
]'
```

Wait 30-60s. Check `ALERTMANAGER_TO` mailbox.

Expected: email с subject `[SMS-Platform prod] IntentionalTestAlert (firing)`.

После 5min — alert auto-resolve (default `resolve_timeout=5m`). Получишь второй email `(resolved)`.

## Method B — реальный alert через scale-up retry-count (для staging)

```sql
-- НА STAGING (НЕ prod!):
INSERT INTO subaccount_routing_assignment (client_id, last_materialize_error_at, materialize_retry_count)
SELECT gen_random_uuid(), now(), 100
FROM generate_series(1, 6);
```

Это создаст 6 stuck rows c retry_count=100 → alert `SRARetryGiveUpRows` fire'ится в течение 5min (threshold > 0). Cleanup:
```sql
DELETE FROM subaccount_routing_assignment WHERE materialize_retry_count = 100;
```

**Использовать только Method A на prod.** Method B — для staging копии.

## Если email не пришёл

1. Check alertmanager logs: `docker compose ... logs alertmanager 2>&1 | tail -50` — ошибки SMTP.
2. Verify SMTP credentials: `telnet $ALERTMANAGER_SMTP_HOST` от prod-host. TLS handshake должен пройти.
3. Check spam folder.
4. Verify routing: alert label `env: prod` НЕ должен match'иться с sandbox-route (если оба правила существуют — порядок важен, env=sandbox должен быть первым в routes:).
5. Check Alertmanager API:
   ```
   curl -s http://localhost:9093/api/v2/alerts | jq '.[] | {alertname: .labels.alertname, status: .status.state}'
   ```
   Если alert в списке как `firing` но email не пришёл — проблема в SMTP config, не в Alertmanager.
6. Manual SMTP test:
   ```
   docker run --rm alpine sh -c 'apk add msmtp && echo "Test" | msmtp --host=$ALERTMANAGER_SMTP_HOST --auth=on --user=$ALERTMANAGER_SMTP_USER --password=$ALERTMANAGER_SMTP_PASSWORD $ALERTMANAGER_TO'
   ```
