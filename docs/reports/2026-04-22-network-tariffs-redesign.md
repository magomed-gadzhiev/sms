# Network Tariffs Redesign — Implementation Report

**Date:** 2026-04-22
**Branch:** `feat/network-tariffs-redesign`
**Base:** `origin/master` (sha 80e15fd)
**Commits on branch:** 25 (see `git log origin/master..HEAD`)

## What shipped

Replaced three-tab UI `/network/tariffs` with URL-based pages + shared matrix component. Same matrix reused read-only on client-facing `/tariffs`.

**Backend (Go):**
- `GET /portal/v1/network/tariffs/subaccounts-summary` — list of sub-accounts with template bind, override count, avg price (Redis-cached 5 min).
- `GET /portal/v1/network/tariff-templates` — templates library with counters.
- `POST /portal/v1/network/tariff-templates` — create (optionally copy from existing).
- `POST /portal/v1/network/tariff-templates/:id/bind` — bulk bind to sub-accounts.
- `POST /portal/v1/network/tariff-templates/:id/duplicate` — clone.
- `GET /portal/v1/network/tariff-editor/:id` — inheritance-aware matrix read (template or override mode).
- `PATCH /portal/v1/network/tariff-plans/:plan_id/bulk` — atomic tier+cell upsert/delete.
- `POST /portal/v1/network/tariff-plans/:plan_id/periods` — create period with optional copy.
- `GET /portal/v1/client/tariffs/effective` — readonly effective sheet for authenticated sub-account.

**Frontend (TypeScript/React):**
- `/network/tariffs` — subaccount list page + legacy `?tab=*` redirect.
- `/network/tariffs/templates` — templates library + Create/Bind/Duplicate dialogs.
- `/network/tariffs/editor/:id` — matrix editor with top-bar filters, period dropdown (+ new period dialog), unsaved-changes guard.
- `<TariffMatrix>` — shared component: read/edit mode, a11y (table-semantic + aria-label per cell), inheritance markers (inherited italic/grey, override bold on amber, unset dash).
- `/tariffs` — client-side readonly now uses the shared matrix.

**Deleted legacy:** `NetworkTariffsPage.tsx`, `TariffOverviewTab.tsx`, `TariffTemplatesTab.tsx`, `TariffOverridesTab.tsx`, `TariffPlanEditor.tsx`.

**ESLint ratchet:** 69 → 56 after cleanup.

## Reviews

Every backend task (1–6) passed **two-stage review**: spec-compliance reviewer + code-quality reviewer (via `superpowers:code-reviewer`). Critical issues found during review (5 distinct):
- Task 4: name race → 500 (now 409 via pgconn).
- Task 4: source deactivated mid-copy (FOR SHARE lock added).
- Task 5: editor dropped `operator_id` dimension (rewritten per-operator + wildcard fallback).
- Task 5: scope JSON field mismatch (renamed per spec §5.1.5).
- Task 6: concurrent override-plan create → 500 (SAVEPOINT + re-SELECT).
- Task 6: orphan `price_per_segment=0` tier row (new tiers now require price in bulk body).

## Tests

~70 integration tests added across 8 handler files. All tests use `//go:build integration` + `TEST_DATABASE_URL` skip — **do not run without a seeded test DB**.

**Not runnable locally:** Go toolchain blocked by Windows Device Guard in this environment. `./scripts/check.sh` auto-skips Go with `[SKIP]` warning. **CI (Linux) must validate** before merge.

**Frontend tests:** portal-frontend has no test runner installed (no vitest/jest). Task 9–23 do NOT have UI tests. A dedicated `task-26: bootstrap frontend test runner + coverage sweep` is recommended before merging to master.

## Known limitations / deferred

1. **Wildcard vs operator-specific precedence (write-side).** Bulk PATCH rejects cells on wildcard plans (`operator_id IS NULL`) — user must create operator-specific plan first. Read-side (Task 2, Task 5) uses operator-specific → wildcard fallback. Document in UI when/if users hit this.
2. **Delete period button** — omitted from editor page; backend endpoint does not exist. Tracked as `TODO(task-26-delete-period)` in `NetworkTariffEditorPage.tsx`.
3. **Strategy editor** — strategy text is read-only in editor page; changing strategy would require recalc of existing tiers. Out of scope; tracked.
4. **Tier-schema divergence across operator-plans** — if operator plans have different `from_count` thresholds, cells at primary's thresholds that aren't present in operator-plan render as `source=unset`. Documented in `network_tariff_editor.go` header; consider relaxing wildcard suppression in `priceForOperator` if UX pain emerges.
5. **Frontend UI tests** — absent (see above).
6. **Smoke test on dev server** — NOT executed. Required before merge:
   - `cd /opt/sms && DEPLOY_BRANCH=feat/network-tariffs-redesign ./scripts/server.sh deploy`.
   - Manual prowl: list subs, open template editor, edit a cell, save, verify roundtrip; client `/tariffs` readonly.
   - Check CI (Linux): Go vet, build, integration tests with PG provisioned.

## Data model

No migrations added. Reuses tables from `migrations/000098_reseller_tariff_tables.up.sql` + `000099_reseller_period_optional_end_date.up.sql`:
- `reseller_tariff_templates`
- `reseller_tariff_plans` (template-level: `sub_account_id IS NULL`; override-level: `sub_account_id IS NOT NULL` per CHECK)
- `reseller_tariff_periods`
- `reseller_tariff_tiers`
- `sub_account_template_assignments`

Names mismatches between original spec/plan and real schema (e.g. `valid_from` vs `start_date`, `reseller_tariff_template_bindings` vs `sub_account_template_assignments`) were reconciled in-place during Task 1 and Task 2 (commits `860fab6`, `fc18883`).

## Merge checklist

- [ ] Push branch: `git push origin feat/network-tariffs-redesign`.
- [ ] CI green (Linux Go vet + build + `-tags=integration` tests with seeded PG).
- [ ] Manual smoke on dev server.
- [ ] Decide wildcard open question resolution (currently: specific plan wins on read; bulk PATCH rejects cells on wildcard plan).
- [ ] Task 26 (follow-up): frontend test-runner bootstrap + UI test coverage sweep.
- [ ] PR review — this is 25 commits, consider merging as a stack by phase or squashing per-task.
