# Production Admin Bootstrap

## Когда применять

Один раз при first cutover на prod, ДО первого `migrate up`. Блокирует автоматический seed sandbox-test-users (миграция 000116) и описывает создание реального admin.

## Step 1 — выключить sandbox seed на prod

`migrations/000116_seed_sandbox_test_users.up.sql` имеет guard: skip'ается если postgres-level GUC `app.environment = 'production'`. Default (NULL) — seed runs (для sandbox/CI).

ОДНОРАЗОВО на свежем prod-cluster, ДО первого migrate:

```
ssh prod-host
cd /opt/sms
docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml --env-file deployments/.env.prod up -d postgres
sleep 15
docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml exec -T postgres \
    psql -U smpp -d smpp_db -c "ALTER DATABASE smpp_db SET app.environment = 'production';"
docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml restart postgres
```

Verify:
```
docker compose ... exec -T postgres psql -U smpp -d smpp_db -tAc "SHOW app.environment;"
# Expected: production
```

## Step 2 — apply миграций

```
./scripts/server.sh migrate
```

Логи должны содержать `NOTICE: Skipping sandbox seed (000116) on production environment`. Verify:
```
docker compose ... exec -T postgres psql -U smpp -d smpp_db -c "SELECT count(*) FROM users WHERE email='aggregator@test.local';"
# Expected: 0
```

## Step 3 — создать первого admin

`cmd/seed-admin` принимает creds через ENV. Generate strong password ВНЕ shell history (через `openssl rand`).

КРИТИЧНО: `${POSTGRES_PASSWORD}` в DATABASE_URL должен резолвиться в shell ДО docker run (compose --env-file экспортит только в контейнер, не в host shell). Source .env.prod перед invocation:

```
set -a
. /opt/sms/deployments/.env.prod
set +a

PROD_ADMIN_PASSWORD=$(openssl rand -base64 24)
echo "Save this password in secrets vault BEFORE proceeding: $PROD_ADMIN_PASSWORD"
read -p "Saved? Press Enter..."

docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml --env-file deployments/.env.prod \
    run --rm \
    -e ADMIN_USERNAME=ops \
    -e ADMIN_EMAIL=ops@example.com \
    -e ADMIN_PASSWORD="$PROD_ADMIN_PASSWORD" \
    -e DATABASE_URL="postgres://smpp:${POSTGRES_PASSWORD}@postgres:5432/smpp_db?sslmode=disable" \
    dev go run ./cmd/seed-admin

unset PROD_ADMIN_PASSWORD POSTGRES_PASSWORD
```

(`dev` сервис — utility-container с Go toolchain, существует в base compose. `run --rm` стартует ad-hoc — не зависит от того, был ли `dev` поднят через `up`.)

Output: `Admin user created successfully: ops, ops@example.com, <password>`. Сохранить в vault, **НЕ коммитить**.

## Step 4 — verify login

Через UI: `https://${PORTAL_DOMAIN}/admin` → email `ops@example.com` + сохранённый пароль. UI должен пустить.

После успешного login — **рекомендуется** через UI сменить пароль на ещё более длинный (если такая форма есть). Если нет — следующая ротация через step 5.

## Rotation / Recovery

cmd/seed-admin idempotent (skip if user exists by username OR email — см. main.go:44), не умеет change-password. Если password утерян:

```
docker compose ... exec -T postgres psql -U smpp -d smpp_db -c "DELETE FROM users WHERE email='ops@example.com' OR username='ops';"
# Затем заново Step 3 с новым PROD_ADMIN_PASSWORD.
```

ВНИМАНИЕ: DELETE удалит associated audit-log entries по cascade'у только если FK включает CASCADE — verify через `\d users` перед DELETE на prod с реальной активностью. Альтернатива:

```
docker compose ... exec -T postgres psql -U smpp -d smpp_db -c "
UPDATE users SET password_hash='<new-bcrypt-hash>' WHERE email='ops@example.com';
"
```

(bcrypt hash generated через любую библиотеку с cost=10; формат `\$2a\$10\$...`).

## Known limitations

- `cmd/seed-admin` теперь fail-fast если `ADMIN_PASSWORD` не задан или короче 16 символов (Plan 8 Task 1, commit 22a0bc3). Default `Admin123!` удалён. Сгенерировать пароль на prod: `openssl rand -base64 24`.
- Нет mandatory password change на первом login. Ops обязан manually rotate после bootstrap.
- adminRoleID hardcoded в `cmd/seed-admin`. Если admin role UUID изменится в будущем — invocation сломается silent (создаст user с invalid role_id).
