# Aggregator Routing — Plan 3: Hardening & Polish — DONE

**Дата завершения:** 2026-05-05
**Спецификация:** `docs/superpowers/specs/2026-05-04-aggregator-routing-management-design.md`
**План:** `docs/superpowers/plans/2026-05-04-aggregator-routing-plan-3-hardening.md`
**Предыдущие планы:** `2026-05-04-aggregator-routing-plan-1-DONE.md`, `2026-05-04-aggregator-routing-plan-2-DONE.md`

## Сделано (12/12)

| # | Task | Commits |
|---|------|---------|
| 1 | Drop sandbox-only `uq_cell_provider` (миграция 000138 + 000139 для INDEX-формы) | `fcbd72d`, `f17af84` |
| 2 | `RouteSignature` + pre-check `duplicate_route_signature` в Add/UpdateRouteOverride | `be04777`, `89dd136` |
| 3 | `ResellerOnlyMiddleware` на `/sub-accounts/*` subrouter (defence-in-depth) | `6038e32` |
| 4 | Materializer partial-failure → 200+warnings + Prometheus `MaterializeFailureTotal{operation}` + `ProviderMaterializer`/`RouteMaterializer` interfaces | `98e687b` |
| 5 | Concurrency stress test для Route+Provider materializer'ов (PASS без advisory_lock — DELETE+INSERT под row-locks естественно сериализуется) | `fa55423` |
| 6 | `trg_sra_orphan_cleanup` AFTER UPDATE trigger на `subaccount_routing_assignment` (миграция 000140) | `9209aa9`, `7f33d2f`, `1af61cf` (test alignment) |
| 7 | Extracted `middleware.VerifySubAccountOwnership` (3+6 call sites дедуплицированы) | `99ca6aa` |
| 8 | Always-visible amber banner + `title=` info-icon в Preview UI про operator/country аппроксимации | `155c2b4` |
| 9 | ESLint baseline 56 → **51** lockdown (package.json, check.sh, ci.yml, CLAUDE.md) | `b5d245d` |
| 10 | DB schema gap audit → DROPPED orphan `aggregator_profiles` + `aggregator_audit_log` (+7 partition'ов), миграция 000141. Aggregator-pivot к `is_reseller` (см. `project_aggregator_decisions.md`) | `5442a47` |
| 11 | Unskipped `e2e/tests/reseller/network-routing.spec.ts:118` — bulk-assign conflict-modal coverage (API setup + UI assertion) | `4a3f002` |
| 12 | Final smoke + DONE marker | (этот коммит) |

## Финальный smoke

- `./scripts/check.sh` — PASSED. ESLint 51/51 warnings, baseline lock на 51.
- Backend tests на sandbox via Docker dev container:
  - `internal/services/network/...` — PASS (0.934s)
  - `internal/gateway/portal/handlers/...` — PASS (3.199s)
  - `internal/gateway/portal/middleware/...` — PASS (42.028s)
- Sandbox schema на migration version=141, `dirty=false`. Все out-of-tree объекты устранены.
- Все 12 task'ов прошли через subagent-driven flow с двухэтапным review (spec + code-quality), zero skipped review-cycle.

## Архитектурные изменения и долги

### Defence-in-depth на /sub-accounts/*
До Plan 3 защита шла через per-handler `verifyOwnership` (404 на чужих) — defence-by-accident. Сейчас `ResellerOnlyMiddleware` стоит на subrouter'е. Любой новый endpoint под `/sub-accounts/*` автоматически получает reseller-guard.

### Override duplicates
Защита от смысловых дубликатов перенесена с БД-инварианта (`uq_cell_provider`) на handler-уровень (`RouteSignature` + pre-check внутри транзакции). Trade-off: если в будущем появится новый writer `client_routes` (admin-API, импорт), он создаст дубликаты тихо. Mitigation: review-gate на новые writer'ы.

### Materializer partial-failure
`PutOne`/`Bulk` больше не возвращают 500 при сбое материализации — клиент получает 200+warnings. Prometheus counter `MaterializeFailureTotal{operation}` для real alerting (вместо ложных 500-spike). Frontend пока warnings не рендерит — backend-контракт стабилен (warnings absent или array, никогда empty array).

### SRA orphan cleanup
Trigger `trg_sra_orphan_cleanup` автоматически DELETE'ит `subaccount_routing_assignment` row при `(NULL, NULL)`. Регрессия в `TestAssignments_PutOne_NullProviderSet_ClearsInherited` пойманa и исправлена в `1af61cf`.

### Materializer concurrency
Stress test в `internal/services/network/concurrency_stress_test.go` подтверждает: PostgreSQL row-locks естественно сериализуют DELETE+INSERT-pattern в `ApplyToClient`. Advisory lock не понадобился. Если pattern сломается в будущем — комментарии в тесте указывают на `pg_advisory_xact_lock(client_id)` как fix.

### Schema parity
Sandbox теперь точно соответствует `migrations/`. Чистый dev rebuild через `migrate up` приведёт к идентичной схеме.

## OUT of scope (Plan 4 кандидаты)

- **C7 — Audit-log UI** — отложено per spec §6.7 («данные пишутся, страница позже»)
- **D13 — Redis hardening** (requirepass + firewall) — отдельный security-план, перед prod-rollout. Memory `project_redis_hijack_2026_05_04` ещё актуален
- **A3 расширение — cron retry** для committed-but-unmaterialized SRA-rows (сейчас retry на frontend идемпотентный)
- **C8 — operator-condition matching в preview** (сейчас закрыто tooltip-warning'ом, фикс через JOIN на operators — отдельная задача)
- **Frontend warnings rendering** — `AssignmentsPage` пока не показывает `warnings` array из bulk-response. Не блокирует — backend-контракт стабилен
- **Bulk-conflict E2E run-через-CI** — тест написан и компилируется, но не выполнен против live backend (на sandbox-server нет npx). Запуск E2E нуждается в отдельной инфраструктурной задаче

## Memory updates after DONE

- Обновить `project_aggregator_decisions.md` — добавить, что pre-check signature заменил БД-инвариант для override-дубликатов
- Обновить `feedback` про ESLint baseline — теперь 51 в трёх gate-файлах + CLAUDE.md
- Подтвердить актуальность `project_redis_hijack_2026_05_04` — Redis hardening остаётся открытым для prod
