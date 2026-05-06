# Production Secrets Onboarding

## Generate

Все `__GENERATE__` placeholder'ы в `.env.prod` заменить через:

```
openssl rand -base64 32   # passwords
openssl rand -base64 64   # JWT_SECRET
```

## Storage

`.env.prod` хранится **только** на prod-VM в `/opt/sms/deployments/.env.prod` с правами `chmod 600` (owner = deploy user). Файл **не** в git — verify через `grep .env.prod .gitignore` (должно match'иться).

## Rotation

JWT_SECRET ротация = logout всех active sessions:
1. Generate new secret.
2. Update `.env.prod`.
3. `docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml --env-file deployments/.env.prod restart portal-gateway admin-gateway-1 admin-gateway-2 client-gateway-1 client-gateway-2`.
4. Все active sessions invalidated.

POSTGRES_PASSWORD/REDIS_PASSWORD — change в `.env.prod` + `down + up`. ВНИМАНИЕ для Postgres: password change на live data требует SQL `ALTER USER smpp WITH PASSWORD '...'` ДО docker compose down (POSTGRES_PASSWORD env применяется только при первом init на пустом volume).

## Backup of secrets

`.env.prod` бэкапится в encrypted vault (gpg --symmetric с ops master password) ежемесячно:

```
gpg --symmetric --cipher-algo AES256 -o env.prod.$(date +%F).gpg /opt/sms/deployments/.env.prod
```

Move `env.prod.YYYY-MM-DD.gpg` в защищённое хранилище (vault / encrypted backup).

## Known gap (Plan 7 → Plan 8)

`.env.prod.example` явно отмечает: HaProxy-fronted gateways не имеют TLS на cutover-day. Reception of public-facing clients откладывается до Plan 8 Caddy expansion.
