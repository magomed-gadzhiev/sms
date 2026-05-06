# Production Cutover Runbook

**Audience:** ops-инженер, выполняющий cutover. Один проход через документ — один cutover.

**Estimated duration:** 2-4 часа (initial deploy + canary release), 24h до full release.

**Pre-cutover artifacts (готовы):**
- Plan 7 Tasks 1-12 implementation (commits ec506b7..HEAD).
- `docs/ops/redis-firewall-apply.md` — Redis firewall procedure.
- `docs/ops/prod-migration-procedure.md` — safe migrations после first live-traffic.
- `docs/ops/prod-secrets-onboarding.md` — generate/store/rotate secrets.
- `docs/ops/prod-admin-bootstrap.md` — first admin creation procedure.

## Pre-cutover (за 24-48h до)

### Infrastructure
- [ ] **VM provisioned**. Минимум: 8 vCPU, 16GB RAM, 200GB SSD, Ubuntu 22.04 LTS.
- [ ] **DNS A-record** `${PORTAL_DOMAIN}` → VM IP, TTL ≤ 300s. Verify: `dig +short ${PORTAL_DOMAIN}`.
- [ ] **Firewall на VM (host-level):** ufw/iptables разрешает: 22 (SSH from ops bastion only), 80+443 (public), 2775+2776 (SMPP — IP-allowlist по согласованию с клиентами). Всё остальное — DROP.
- [ ] **Docker + docker-compose установлены**: `docker --version` ≥ 24.0, `docker compose version` ≥ **2.24** (требуется для `!reset` и `!override` в overlay).
- [ ] **`/opt/sms` склонирован**:
  ```
  ssh user@<vm>
  sudo mkdir -p /opt/sms && sudo chown $USER /opt/sms
  cd /opt && git clone https://github.com/<org>/sms.git
  ```

### Secrets
- [ ] **`.env.prod` собран** на основе `deployments/.env.prod.example`. Generate `__GENERATE__` placeholder'ы через `openssl rand -base64 32` (passwords) и `-base64 64` (JWT_SECRET). См. `docs/ops/prod-secrets-onboarding.md`.
- [ ] **`.env.prod` копирован** на VM в `/opt/sms/deployments/.env.prod`, `chmod 600`.
- [ ] **Verify .env.prod в .gitignore**: `grep -F .env.prod .gitignore`.
- [ ] **SMTP-relay creds готовы** — `ALERTMANAGER_SMTP_HOST`, `*_FROM`, `*_USER`, `*_PASSWORD`, `ALERTMANAGER_TO`. Test reachability: `telnet <smtp-host> 587` от prod-host.
- [ ] **PORTAL_ADMIN credentials decided** — какой email + initial password для cmd/seed-admin (Step 3 ниже).
- [ ] **PROMETHEUS_ENV=prod** в .env.prod (триггер email-routing в Alertmanager).
- [ ] **GRAFANA_ADMIN_PASSWORD** в .env.prod.
- [ ] **CANARY_CLIENT_IDS** в .env.prod пока **пустой** (заполнится после Step 5 Hour 1).

### Storage
- [ ] **Backup destination prepared**: `sudo mkdir -p /var/backups/sms-pg && sudo chown $USER /var/backups/sms-pg`.

## Cutover Hour 0 — initial deploy

- [ ] **T+0:** SSH на prod VM, `cd /opt/sms`, `git pull`. ВСЕ последующие команды выполняются ИЗ этой ssh-сессии (без nested ssh).

  Удобный alias на сессию (используется во всех snippets ниже):
  ```
  alias dc='docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml --env-file deployments/.env.prod'
  set -a; . /opt/sms/deployments/.env.prod; set +a   # экспортит POSTGRES_PASSWORD и др. в текущий shell
  ```

- [ ] **T+2:** Pre-pull images (избегаем blow timing budget на slow link):
  ```
  dc pull
  ```

- [ ] **T+5:** Bring up postgres + redis (только эти два):
  ```
  dc up -d postgres redis
  sleep 30
  ```

- [ ] **T+8:** Manual baseline snapshot (для возможного rollback ДО первой миграции — initial cutover sans deploy-prod.sh):
  ```
  mkdir -p /var/backups/sms-pg/pre-cutover-$(date +%Y%m%d-%H%M%S)
  SNAPSHOT_DIR=/var/backups/sms-pg/pre-cutover-$(date +%Y%m%d-%H%M%S)
  dc exec -T postgres pg_basebackup -U smpp -D - -Ft -P -X fetch | gzip > "$SNAPSHOT_DIR/base.tar.gz"
  git rev-parse HEAD > "$SNAPSHOT_DIR/prev_ref.txt"
  echo "Snapshot: $SNAPSHOT_DIR (для Abort criteria ниже, если потребуется rollback)"
  ```

- [ ] **T+10:** TLS staging toggle (ДО первого ACME-запроса, иначе после T+40 уже выпустится prod-cert):
  - Recommended: первый cutover в staging-mode чтобы не сжечь LE rate-limit (5 dups/week).
  - Раскомментировать `acme_ca https://acme-staging-v02.api.letsencrypt.org/directory` в `deployments/configs/caddy/Caddyfile`.
  - Continue. После полной верификации (T+45 ниже) — закомментировать обратно + restart caddy для prod-cert.

- [ ] **T+12:** Block sandbox-seed на prod (см. docs/ops/prod-admin-bootstrap.md Step 1):
  ```
  dc exec -T postgres psql -U smpp -d smpp_db -c "ALTER DATABASE smpp_db SET app.environment = 'production';"
  dc restart postgres
  sleep 15
  dc exec -T postgres psql -U smpp -d smpp_db -tAc "SHOW app.environment;"
  ```
  Expected: `production`.

- [ ] **T+15:** Apply migrations (POSTGRES_PASSWORD уже в shell после set -a из T+0):
  ```
  docker run --rm \
      -v /opt/sms/migrations:/migrations \
      --network deployments_smpp-network \
      migrate/migrate \
      -path /migrations \
      -database "postgres://smpp:${POSTGRES_PASSWORD}@postgres:5432/smpp_db?sslmode=disable" \
      up
  ```
  Логи: `NOTICE: Skipping sandbox seed (000116) on production environment`, последняя миграция без error.

  Verify нет sandbox users:
  ```
  dc exec -T postgres psql -U smpp -d smpp_db -tAc "SELECT count(*) FROM users WHERE email='aggregator@test.local';"
  ```
  Expected: 0.

- [ ] **T+22:** Bootstrap admin (см. docs/ops/prod-admin-bootstrap.md Step 3). `dev` сервис — utility-container с Go toolchain в base compose, `run --rm` стартует ad-hoc:
  ```
  PROD_ADMIN_PASSWORD=$(openssl rand -base64 24)
  echo "Save this password in secrets vault: $PROD_ADMIN_PASSWORD"
  read -p "Saved? Press Enter..."

  dc run --rm \
      -e ADMIN_USERNAME=ops \
      -e ADMIN_EMAIL=ops@<your-domain> \
      -e ADMIN_PASSWORD="$PROD_ADMIN_PASSWORD" \
      -e DATABASE_URL="postgres://smpp:${POSTGRES_PASSWORD}@postgres:5432/smpp_db?sslmode=disable" \
      dev go run ./cmd/seed-admin

  unset PROD_ADMIN_PASSWORD
  ```
  Expected: `Admin user created successfully: ops, ops@<domain>, <password>`.

- [ ] **T+28:** Bring up rest of stack:
  ```
  dc up -d
  ```

- [ ] **T+33:** Health check:
  ```
  ./scripts/healthcheck.sh
  ```
  Все services должны быть зелёные. Если FAIL — НЕ продолжать, см. Abort criteria ниже.

- [ ] **T+36:** Apply Redis firewall (см. docs/ops/redis-firewall-apply.md):
  ```
  sudo bash /opt/sms/scripts/redis-firewall.sh
  sudo apt-get install -y iptables-persistent && sudo netfilter-persistent save
  ```
  Verify: `nc -zv <vm-ip> 6379 -w 3` от ops bastion → timeout.

- [ ] **T+40:** Verify TLS (caddy auto-issued cert — staging если включён в T+10):
  ```
  curl -fsSv https://${PORTAL_DOMAIN}/health
  ```
  Первый запрос триггерит ACME. Cert получается за ~10-120s (зависит от DNS propagation). Если зависает > 120s — проверь `dc logs caddy`, DNS, port 80 reachability.

  Если использовался staging — browser покажет cert warning. Это OK на этом этапе.

- [ ] **T+45:** Login через UI: `https://${PORTAL_DOMAIN}/admin` → `ops@<domain>` + saved password. Должен пустить (skip TLS warning если staging).

- [ ] **T+50:** **Switch staging → prod cert** (если использовался staging в T+10):
  - Закомментировать `acme_ca` в Caddyfile обратно.
  - `dc restart caddy`. Caddy выпустит prod-cert (~10-30s).
  - Verify: `curl -fsSv https://${PORTAL_DOMAIN}/health` без warning'ов.

## Cutover Hour 1 — canary release

Цель: первые 24h только один тест-клиент может отправлять. Real clients onboarding после Hour 24+.

- [ ] **T+60:** Создать canary client через UI:
  - Sidebar → Clients → Create.
  - Name: "Canary Test" (или подобное).
  - Email: `canary@<your-domain>`, password generated.
  - Verify: client_id (UUID) отображается в UI / API.
  Запиши UUID.

- [ ] **T+65:** Set `CANARY_CLIENT_IDS=<canary-client-uuid>` в `.env.prod`:
  ```
  vim /opt/sms/deployments/.env.prod
  # Edit: CANARY_CLIENT_IDS=<uuid>
  ```
  Restart gateways (env подхватывается на restart):
  ```
  dc up -d --no-deps client-gateway-1 client-gateway-2 admin-gateway-1 admin-gateway-2 smpp-gateway portal-gateway
  ```

- [ ] **T+70:** Run smoke (см. scripts/smoke-prod.sh):
  ```
  PORTAL_DOMAIN=<your-domain> ADMIN_EMAIL=ops@<your-domain> ADMIN_PASSWORD=<saved> ./scripts/smoke-prod.sh
  ```
  Expected: `=== AUTOMATED smoke OK ===`. Manual checks из output — выполни и проверь.

- [ ] **T+75:** Manual smoke — получить bearer-token canary-клиента и отправить тестовый SMS.

  Login для canary client (creates session token):
  ```
  CANARY_TOKEN=$(curl -fsS --max-time 10 -X POST "https://${PORTAL_DOMAIN}/portal/v1/auth/login" \
      -H 'Content-Type: application/json' \
      -d '{"email":"canary@<your-domain>","password":"<canary-password-saved-T+60>"}' \
      | jq -r '.token')
  test -n "$CANARY_TOKEN" || { echo "FAIL: canary login"; exit 1; }
  ```

  (Альтернатива — если portal exposes API-key для clients — создать API-key через UI/admin endpoint и использовать его. Точная procedure зависит от UI; общий fallback — login flow выше.)

  Send test SMS:
  ```
  curl -X POST "https://${PORTAL_DOMAIN}/api/v1/messages" \
      -H "Authorization: Bearer $CANARY_TOKEN" \
      -H 'Content-Type: application/json' \
      -d '{"to":"+77001234567","text":"Cutover smoke","sender":"TestSender"}'
  ```
  Expected: HTTP 202 + message_id.

  Verify rejection не-allowlisted client'а: получить token любого OTHER-client'а и повторить — expected 503 / "service in canary mode".

  Verify rejection не-allowlisted client'а: попробуй с другого client'а — expected 503 "service in canary mode".

- [ ] **T+80:** Verify в Grafana SRA-aggregator dashboard (`https://${PORTAL_DOMAIN}` или SSH-tunnel `ssh -L 3001:localhost:3001 prod-host` → `http://localhost:3001`, login admin / GRAFANA_ADMIN_PASSWORD из .env.prod):
  - `portal_sra_pending_retry_count` ≈ 0.
  - `portal_sra_retry_give_up_count` = 0.
  - `portal_sra_materialize_failure_total` rate ~0.

- [ ] **T+85:** Verify Alertmanager не fire'ится паникой (через SSH-tunnel):
  ```
  ssh -L 9093:localhost:9093 prod-host
  curl -s http://localhost:9093/api/v2/alerts | jq '.[] | select(.status.state=="active") | .labels.alertname'
  ```
  Expected: пусто или ожидаемые (Watchdog alertmanager built-in).

## Cutover Hour 1-24 — observation

- [ ] **T+120 (2h):** 30min check на Grafana dashboard. Никаких alerts в Alertmanager.
- [ ] **T+360 (6h):** Repeat check.
- [ ] **T+720 (12h):** Repeat check.
- [ ] **T+1440 (24h):** Repeat check + verify daily backup ran (см. T14):
  ```
  ssh prod-host "ls -lh /var/backups/sms-pg/daily-*.tar.gz | tail -3"
  ```
  Expected: один свежий snapshot за последние 24h.

## Cutover Hour 24+ — full release

- [ ] **Verify zero alerts fired за 24h:**
  ```
  ssh prod-host "docker compose ... logs alertmanager 2>&1 | grep -i 'sent' | head"
  ```
  Если что-то кроме `Watchdog` — investigate перед remove canary.

- [ ] **Remove canary:**
  ```
  vim /opt/sms/deployments/.env.prod
  # Edit: CANARY_CLIENT_IDS=  (пустая строка)

  docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml --env-file deployments/.env.prod \
      up -d --no-deps client-gateway-1 client-gateway-2 admin-gateway-1 admin-gateway-2 smpp-gateway portal-gateway
  ```

- [ ] **Onboard real clients** через UI (sidebar → Clients → Create).

- [ ] **Communicate go-live** ops + клиенты.

## Abort criteria (rollback trigger)

Любое из:
- Health check (`./scripts/healthcheck.sh`) не зелёный спустя 10min after deploy.
- Alertmanager fired `SRARetryGiveUpRows` или `SRAMaterializeInitialFailureRate` в первые 60min.
- Manual SMS-send из canary failed (HTTP non-2xx) или message stuck в pending > 5min.
- Database errors в logs `portal-gateway`/`worker` с rate > 10/min.
- TLS не выпустился за 5min после первого https-request.

**Procedure:**

Для initial cutover (snapshot создан в T+8 как `pre-cutover-*`):
```
cd /opt/sms
SNAPSHOT_DIR=$(ls -td /var/backups/sms-pg/pre-cutover-* | head -1)
PREV_REF=$(cat "$SNAPSHOT_DIR/prev_ref.txt")
./scripts/rollback-prod.sh "$PREV_REF" "$SNAPSHOT_DIR"
```

Для последующих deploy'ев (snapshot создан в `pre-deploy-*` через `deploy-prod.sh`):
```
cd /opt/sms
SNAPSHOT_DIR=$(ls -td /var/backups/sms-pg/pre-deploy-* | head -1)
PREV_REF=$(cat "$SNAPSHOT_DIR/prev_ref.txt")
./scripts/rollback-prod.sh "$PREV_REF" "$SNAPSHOT_DIR"
```

`rollback-prod.sh` запросит interactive confirm перед DB restore (DESTRUCTIVE). Если auto-rollback из deploy-prod.sh — он передаёт только `$PREV_REF` (code-only rollback, без DB restore). Manual DB restore — следуй on-screen инструкциям из rollback скрипта (offline tar в stopped container).

Если rollback тоже failed — escalate ops on-call, manual remediation.

## Post-cutover — week 1

- **Daily:** Grafana dashboard review, проверь нет ли trend'ов в metrics.
- **Day 3:** Intentional alert-fire test (см. `docs/ops/intentional-alert-test.md`, Task 14), verify email доехал в `ALERTMANAGER_TO`.
- **Day 7:** Backup restore test — `./scripts/restore-test.sh` (Task 14), verify pg_basebackup actually restorable.
- **Day 14:** Review canary metrics — есть ли false-positive alert'ы из новых alerts? Tune thresholds если нужно.

## Known gaps (Plan 8 candidates)

- HaProxy gateways (client/admin/portal) пока без TLS — Caddy expansion.
- Multi-replica worker safety (advisory-lock в retry-loop) deferred (Plan 7 T2 design flaw).
- DB-restore mechanics в rollback-prod.sh manual-only — full automation TODO Plan 8.
- Audit-log retention (auto-drop через N месяцев) не реализован.
