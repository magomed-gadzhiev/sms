# Production Migration Procedure

## Применимость

Greenfield first-deploy: накатывать `golang-migrate up` стандартно — таблиц нет, locks ни на что. **После первого live-traffic'а** любая новая миграция должна следовать процедурам ниже.

## Запрещённые в live-traffic операции

| Операция | Почему | Замена |
|---|---|---|
| `CREATE INDEX foo ON bar(...)` | ShareLock — writes блокированы | `CREATE INDEX CONCURRENTLY` (через psql вне migrate, см. ниже) |
| `ALTER TABLE ... ADD COLUMN ... NOT NULL DEFAULT v` | в pg < 11 rewrite всей таблицы | 3-step: ADD COLUMN nullable → backfill batched → SET NOT NULL |
| `ALTER TABLE ... ALTER COLUMN ... TYPE` | rewrite таблицы | новая колонка + backfill + DROP старой (multi-release) |
| `DROP COLUMN` без preparation | ломает inflight queries из старых деплоев | сначала remove из app code → deploy → DROP в следующем релизе |
| `ALTER TABLE ... ADD CONSTRAINT NOT VALID → VALIDATE` за один шаг | VALIDATE берёт ShareUpdateExclusive | `ADD ... NOT VALID` в одной миграции, `VALIDATE CONSTRAINT` в следующей после прогрева |

## CONCURRENTLY edge case (golang-migrate v4 specifics)

`CREATE INDEX CONCURRENTLY` нельзя запустить внутри транзакции. **golang-migrate v4 НЕ поддерживает** per-migration no-transaction pragma (`-- migrate:no-transaction` это Hasura-extension, не работает у нас).

**Workaround (primary):**

1. Подготовить миграцию `NNN_create_index_foo.up.sql` пустую или содержащую только non-DDL noop (например `SELECT 1;`). `.down.sql` — `DROP INDEX IF EXISTS foo;`.
2. Apply миграцию через стандартный `./scripts/server.sh migrate` (на prod-host) — version pointer продвигается, index ещё не создан.
3. Выполнить `CREATE INDEX CONCURRENTLY foo ON bar(...)` через psql напрямую:
   ```
   docker compose ... exec -T postgres psql -U smpp -d smpp_db -c \
       "CREATE INDEX CONCURRENTLY IF NOT EXISTS foo ON bar(col1, col2)"
   ```
4. Verify: `\d bar` через psql — индекс присутствует, без `INVALID` маркера.

**Альтернатива (если индекс уже создан вручную, нужно "догнать" version pointer):**

```
docker run --rm migrate/migrate -path /migrations -database "$DATABASE_URL" force <version-N-1>
docker run --rm migrate/migrate -path /migrations -database "$DATABASE_URL" up 1
```

## ADD COLUMN с дефолтом — 3-step pattern

Migration N (release K):
- `ALTER TABLE foo ADD COLUMN bar text NULL;`
- В app code: writes пишут `bar` (старые читатели игнорируют).

Backfill (release K+1, отдельная миграция или manual psql):
- Батчами по 10k, не одной транзакцией:
  ```
  DO $$
  DECLARE updated int;
  BEGIN
    LOOP
      UPDATE foo SET bar = 'default' WHERE bar IS NULL AND ctid IN (
          SELECT ctid FROM foo WHERE bar IS NULL LIMIT 10000
      );
      GET DIAGNOSTICS updated = ROW_COUNT;
      EXIT WHEN updated = 0;
      COMMIT;
    END LOOP;
  END $$;
  ```

Migration N+1 (release K+2):
- `ALTER TABLE foo ALTER COLUMN bar SET NOT NULL;`
- `ALTER TABLE foo ALTER COLUMN bar SET DEFAULT 'default';`

## Migration safety review checklist

Перед merge каждой post-cutover миграции:

- [ ] Локи: какие locks таблицы берёт миграция? (pg `pg_locks` view + EXPLAIN на тестовой копии)
- [ ] Backfill: если `UPDATE` затрагивает >10k rows — батчинг? cursor + LIMIT?
- [ ] Reversibility: `.down.sql` действительно откатывает .up без data loss? (Test через `migrate down 1; migrate up 1` на staging копии prod)
- [ ] App-code compat: новый код жёстко требует новую колонку, или graceful fallback? (Если жёстко — release order важен: миграция → deploy кода).
- [ ] Партиции: если меняется partitioned table — изменение применяется ко всем партициям? (golang-migrate сам не парсит, проверить вручную через `pg_inherits`).

## Procedure

1. Pre-deploy на staging копии prod (`pg_basebackup` от prod, restore на staging-host).
2. Замерить duration: `\timing on; <SQL>;`. Если > 30s на staging копии — пересмотреть стратегию.
3. Apply на prod с window: уведомить ops, запустить migrate из maintenance host (`./scripts/server.sh migrate` на prod-host).
4. Verify: `\d <table>` через `psql` — нет блокировок (`SELECT * FROM pg_stat_activity WHERE wait_event_type='Lock'`).
5. Watch: 15min на dashboard SRA-aggregator + general portal-gateway метрики — нет regression.

## Rollback миграции

Только если миграция reversible (`.down.sql` существует и тестирован). После live-data — обычно destructive (DROP COLUMN теряет data). Предпочтение — forward-fix (новая миграция, исправляющая broken state) над rollback.

## golang-migrate dirty state recovery

Если migrate up прерван посреди транзакции (например OOM):
```
docker run --rm migrate/migrate -path /migrations -database "$DATABASE_URL" version
```
Если version помечен dirty — manual cleanup в схеме (через psql), затем:
```
docker run --rm migrate/migrate -path /migrations -database "$DATABASE_URL" force <version>
```
И заново `up`.
