# Plan 6 — Aggregator Routing Production Hardening — DONE

**Дата завершения:** 2026-05-06
**Spec:** docs/superpowers/specs/2026-05-04-aggregator-routing-management-design.md
**Plan:** docs/superpowers/plans/2026-05-05-aggregator-routing-plan-6.md

## Summary

Закрыты production-readiness блокеры aggregator routing подсистемы: retry-cap give-up surface для stuck-rows, Redis defense-in-depth (protected-mode + idle-timeout + iptables firewall script), observability (Grafana dashboard + Prometheus alerts).

## Что построено

### A5-extended: Retry-cap + give-up UI surface (Tasks 1-4)

- **Migration 000144** (commit ec506b7) — partial index `idx_sra_pending_retry_active` для retry_loop SELECT с фильтром `materialize_retry_count < 100`.
- **SRARetryGiveUpGauge** + retry-loop filter (commit cf452a6) — stuck rows (retry_count >= 100) больше не пикапятся; gauge показывает count для ops surface.
- **Admin endpoints** GET /portal/v1/admin/network/sra-stuck + POST /sra-stuck/{client_id}/reset (commits c219958, 35c8bf2, dac016d, 0551ae5, a3c3d33). Reset endpoint: транзакция, 404/409/200, audit-log entry. Documented orphan-trigger caveat в doc-comment.
- **SRAStuckPage UI** /admin/sra-stuck (commits e7607b8, ab2737f). Таблица + Reset кнопка + window.confirm + toast. Sidebar item, route registered. 5 vitest tests.

### Redis defense-in-depth (Tasks 5-6)

- **redis.conf** + compose mount (commits fbb0710, e4514d4) — protected-mode yes, bind 0.0.0.0, timeout 0 (выбран ноль вместо 300 чтобы не убить PubSub без heartbeat). Persistence preserved.
- **Host iptables firewall script** scripts/redis-firewall.sh (commits a494b89, 70a0144) — idempotent, persistence через netfilter-persistent или iptables-save fallback. Применение требует sudo пользователя.

### Observability (Tasks 7-8)

- **Prometheus alert rules** deployments/configs/prometheus/rules/sra-aggregator.yml (commits eaf3669, a1e7c4c) — 4 алерта: SRARetryGiveUpRows (warning), SRAMaterializeInitialFailureRate (warning), SRARetryBacklogOverflow (warning), SRARetryPanic (critical). Group inherits 15s evaluation interval.
- **Grafana dashboard** deployments/configs/grafana/dashboards/sra-aggregator.json (commit 2b370f8) — 8 panels: stuck/pending stats, initial/retry failure rates, queue trend, backlog overflow, panic count, ops reference text.

## Метрики и observability (после Plan 6)

**Новые метрики:**
- `portal_sra_retry_give_up_count` (Gauge) — count of rows hitting retry-cap.

**Существующие метрики (Plan 5):**
- `portal_materialize_failure_total{operation, source}` (CounterVec).
- `portal_sra_pending_retry_count` (Gauge).
- `portal_sra_retry_backlog_overflow_total` (Counter).

**Alerts (новые):**
- SRARetryGiveUpRows — give_up_count > 0 for 10m → ops manual reset.
- SRAMaterializeInitialFailureRate — rate > 0.05/sec for 5m → real degradation.
- SRARetryBacklogOverflow — rate > 0 for 15m → tick capacity insufficient.
- SRARetryPanic — increase > 0 in 15m, for 0m → critical.

**Dashboards:**
- "SRA Aggregator Routing" (uid sra-aggregator) — 8 panels.

## Не вошло в Plan 6 (отложено)

- B3: Multi-replica retry advisory-lock — нет scale-up в обозримой перспективе.
- C1: Bulk ResourceID structured detail — нет реальной потребности от ops.
- C4: toast.warning variant — отдельный UX-sweep.
- E1: E2E in CI (GitHub Actions + playwright) — низкий ROI пока тестов мало.
- E3: Race-detector в CI — требует CI-pipeline overhaul.

## WONTFIX

- C6: правка Plan 5 DONE prose (label `kind` → `operation`) — historical record не трогаем.
- D10: N-row signature compare optimization в overrides — WONTFIX до scale-trigger.
- H1, H2: Operator preview enhancements (RU support, cache) — нет драйвера.

## Известные ограничения

1. **Reset endpoint orphan-trigger caveat:** если row имеет provider_set_id IS NULL AND route_set_id IS NULL, trigger trg_sra_orphan_cleanup (migration 000140) удаляет row после UPDATE. Endpoint вернёт 200 OK, но row пропадёт. Документировано в handler doc-comment. Practically unreachable — stuck row всегда имеет non-null set_id (иначе materializer не запустился бы).
2. **Redis firewall (Task 6) применяется вручную** — script зафиксирован в repo, но iptables apply требует sudo пользователя. Без applying — defense-in-depth incomplete (но requirepass + protected-mode остаются активны).
3. **Reset не атомарен с auto-retry:** если row резетнули в момент retry-loop tick'а — race; ущерб minimal (либо retry tick'нет уже сброшенный row, либо reset overwrite'нет тик success/failure update).
4. **Initial-failure alert threshold (0.05/sec):** на dev/sandbox может false-positive с 4+ initial failures за 5 минут. На prod traffic — real signal. Threshold tunable per-environment.

## Smoke-test results (2026-05-06)

| Check | Result | Detail |
|-------|--------|--------|
| A. Worker logs — no Redis errors | PASS | No matching lines in last 10m |
| B. Portal-gateway — no SRA/panic/fatal | PASS | No matching lines in last 10m |
| C. Migration 000144 applied | PASS | `SELECT version … LIMIT 3` → top row = 144 |
| D. idx_sra_pending_retry_active exists | PASS | Index on `subaccount_routing_assignment`, owner smpp |
| E. Redis protected-mode=yes, timeout=0 | PASS | CONFIG GET confirmed both values |
| F. Prometheus rules loaded (4 alerts) | PASS | SRARetryGiveUpRows, SRAMaterializeInitialFailureRate, SRARetryBacklogOverflow, SRARetryPanic |
| G. Grafana dashboard provisioned | PASS | uid=sra-aggregator, title="SRA Aggregator Routing" |
| H. SRA-stuck endpoint → 401 (no session) | PASS | `curl -w '%{http_code}'` → 401 (port 18084) |
| I. Redis iptables firewall | PENDING | Script committed; sudo apply is manual user step |

## Commit history Plan 6

```
2b370f8 ops(metrics): Grafana dashboard for SRA aggregator routing (Plan 6 Task 8)
a1e7c4c fix(metrics): correct misleading summary + remove 1m interval override (Plan 6 Task 7 followup)
eaf3669 ops(metrics): Prometheus alert rules for SRA aggregator (Plan 6 Task 7)
70a0144 fix(redis-fw): reliable docker bridge name detection (Plan 6 Task 6 followup)
a494b89 ops(redis): host iptables firewall script for port 6379 (Plan 6 Task 6)
e4514d4 fix(redis): timeout=0 to avoid silent PubSub disconnect (Plan 6 Task 5 followup)
fbb0710 ops(redis): protected-mode + idle-timeout via redis.conf (Plan 6 Task 5)
ab2737f fix(sra): comment apiFetch usage + list-error test + title null fix (Plan 6 Task 4 followup)
e7607b8 feat(sra): admin UI page for stuck-row management (Plan 6 Task 4)
a3c3d33 docs(sra): document orphan-trigger caveat on Reset (Plan 6 Task 3 followup)
0551ae5 fix(sra): audit-log entry on stuck-row reset (Plan 6 Task 3 followup)
dac016d fix(sra): seed ordering test rows with provider_set_id to avoid orphan trigger
35c8bf2 fix(sra): fix test ordering filter + seed with provider_set for reset trigger safety
c219958 feat(sra): admin endpoints for stuck-row list/reset (Plan 6 Task 3)
cf452a6 feat(sra): retry-cap=100 with SRARetryGiveUpGauge surface (Plan 6 Task 2)
ec506b7 migration(sra): add partial index with retry_count<100 cap (Plan 6 Task 1)
```
