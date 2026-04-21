# Phase 0 Findings — 2026-04-21

**Scope:** инфраструктурная разведка перед фазами A1–C.

## Findings

### F1. OpenAPI location

**Path:** `api/openapi/openapi.yaml`

**Classification:** hand-written. No `# generated`, `DO NOT EDIT`, swaggo annotations, or openapi-generator headers found. The file is a manually authored OpenAPI 3.0.3 document.

**Age delta:** openapi.yaml last committed 2026-04-14; `internal/gateway/client/handlers/` last committed 2026-04-21. Delta: 7 days. Handlers have been modified since the spec was last touched.

**Вывод для A1:** spec is hand-written and already drifted from handlers (7-day gap). A1 is mandatory, not optional. There is no generator to re-run — diff must be done by hand comparison of spec paths vs handler routes. High probability of undocumented endpoints and stale request/response shapes.

---

### F2. Integration test infrastructure

**CI (`.github/workflows/ci.yml`):** single workflow file. Go job runs `go vet ./...`, `go build ./...`, `go test -count=1 -short ./...`. No postgres/redis service containers declared. Frontend job runs tsc + eslint. No integration-test job at all.

**Docker-compose in root:** no `docker-compose*.yml` found in repo root.

**Integration build-tag in codebase:** 10 files found with `//go:build integration` or `// +build integration`:
- `internal/services/billing/application/charge_dual_integration_test.go`
- `tests/integration/ratelimit_test.go`, `registration_test.go`, `rls_test.go`
- `test/integration/directus_test.go`, `grpc_services_test.go`, `kafka_test.go`, `service_communication_test.go`, `storage_test.go`
- `test/performance/performance_tuning.go`

These tests are completely excluded from CI (`-short` does not run them; no `-tags=integration` in CI command).

**`scripts/check.sh`:** no integration-test mode. `--with-tests` flag runs `go test -count=1 -short ./...` — still excludes integration-tagged files.

**Вывод для Фазы C:** integration infra is present in code but dark in CI — no postgres/redis service containers, no `-tags=integration` invocation anywhere automated. Фаза C tests that require real DB (e.g., C-billing dual-charge, C-visibility) will not run in CI as-is. Either CI needs service containers + `-tags=integration` added, or Фаза C must be limited to unit tests with fakes. This is a non-trivial infra gap, not a one-liner fix.

---

### F3. Dual-charge Phase 2 status

**Migration `aggregator_margin_log`:** present. File `migrations/000106_aggregator_margin_log_charge_mode.up.sql` extends `aggregator_margin_log` with `charge_mode`, `pool_segments`, `overage_segments` columns and adds `chk_segments_sum` constraint. The base `aggregator_margin_log` table itself was created in an earlier migration (not 000098; 000098 is `reseller_tariff_tables`).

**RPC `ChargeMessageDual` in proto:** present. `api/proto/billingv1/billing_grpc.pb.go` declares the full client/server interface at `/billing.v1.BillingService/ChargeMessageDual`. The generated file confirms the proto is compiled and included.

**`TarifyMessage` charge path:** `TarifyMessage` does NOT call `ChargeMessageDual` directly. In the legacy path it calls `s.saga.Charge()` (regular single-account charge). `ChargeMessageDual` is only invoked via `saga.ChargeDualAtomic()` which is called from `CommitCharge()` — the commit-on-submit flow activated by `commitOnSubmitEnabled=true`.

Concretely: when `commitOnSubmitEnabled=false` (default), `TarifyMessage` runs legacy `saga.Charge()` for sub-accounts, writing margin to `aggregator_margin_log` with `ChargeMode=pool` placeholder. When `commitOnSubmitEnabled=true`, `TarifyMessage` delegates to read-only `Calculate()`, and the actual dual-atomic charge fires in `CommitCharge()` → `saga.ChargeDualAtomic()` → `billing.ChargeMessageDual`.

**Вывод для C-billing:** the `ChargeMessageDual` RPC is fully wired in the commit-on-submit path (`CommitCharge`). However, the dual-charge path is behind a feature flag (`commitOnSubmitEnabled`). C-billing tests for `role=subaccount_parent` must validate both legacy (`Charge` x2) and commit-on-submit (`ChargeMessageDual`) paths, and must verify which flag value is active in the deployed environment before asserting expected behaviour.

---

### F4. SMPP gateway vs protocol-layer split

**`internal/smpp/server/handler.go` (protocol layer):** handles raw SMPP PDU parsing, session bind/authenticate against direct DB (`ClientRepository`), session-level rate limiting, and message queuing to Kafka (`queue.Producer`). Authenticates via direct storage call — no auth service adapter.

**`internal/gateway/smpp/server/handler.go` (gateway layer):** handles the same PDU types but authenticates via `AuthAdapter` (which calls an auth service / token resolver), manages opt-out via `OptOutRepository`, uses a `RedisStore` for deduplication, and references a separate `smppsession.Session` type from `internal/gateway/smpp/session`.

**`client_id` first assigned in protocol layer:** `internal/smpp/server/handler.go` lines 126–129 (bind_receiver), 166–169 (bind_transmitter), 206–209 (bind_transceiver). In each bind handler, `client.ID` from the direct DB lookup is assigned to a local `clientID` pointer and passed to `session.Bind()`. The field `session.ClientID` is set at `internal/smpp/server/session.go:105`.

**`client_id` in gateway layer:** `internal/gateway/smpp/server/handler.go` lines 137–138, 185–186, 233–234. Set from `userInfo.ClientID` returned by `AuthAdapter`. `AuthAdapter` currently uses `user_id` as `client_id` proxy (see `auth_adapter.go:55`, comment: "временная реализация").

**`is_reseller` / `parent_client_id` in SMPP paths:** nowhere. Neither `internal/smpp/**` nor `internal/gateway/smpp/**` reference `is_reseller`, `IsReseller`, `parent_client`, or `ParentClient`. Aggregator/subaccount context is invisible at the SMPP layer — it is resolved downstream in the tarification service via `clientInfoRepo.GetAccountInfo`.

**Вывод для A6 и C:** the split is architecturally clean but asymmetric. Protocol layer (A6 scope) does direct-DB auth with no auth adapter — this is the known auth asymmetry from Task 3. Gateway layer (C scope) routes through AuthAdapter with `user_id=client_id` conflation that is explicitly marked temporary. `is_reseller` / `parent_client_id` do not leak into either SMPP layer, confirming clean separation for tenant routing. However, the `user_id=client_id` kludge in AuthAdapter means C-visibility tests cannot rely on `client_id` being a true client UUID until that adapter is fixed or a real Client Service lookup is wired.

## Blockers for next phases

### B1: No integration test infra in CI
**Blocks:** Phase C
**What's wrong:** F2 — CI has no postgres/redis services; 10 integration-tagged test files exist but never run in CI (only unit pass with `-short`).
**Required action before that phase starts:** Either add service containers + `-tags=integration` job to a workflow, or explicitly downscope Phase C to unit+fakes and acknowledge the coverage loss.

### B2: AuthAdapter user_id=client_id kludge
**Blocks:** Phase C (specifically C-visibility tests with subaccount identity)
**What's wrong:** F4 — `internal/gateway/smpp/server/auth_adapter.go:55` has an explicit temporary kludge treating `user_id` as `client_id`. Comment: "временная реализация".
**Required action before that phase starts:** Fix the conflation before writing C-visibility SMPP tests, or the tests will assert incorrect invariants (binding as user X will resolve as client X, not the actual client).

### B3: Dual-charge path ambiguity
**Blocks:** C-billing assertions for role=subaccount_parent (does not block starting Phase C, but shapes what the tests measure)
**What's wrong:** F3 — `ChargeMessageDual` is wired only into `CommitCharge()` behind the `commitOnSubmitEnabled` feature flag, not into `TarifyMessage`. Under flag=off the dual-charge path is never exercised.
**Required action before that phase starts:** Decide whether Phase C tests exercise flag=on or flag=off, and document the expected behavior for `subaccount_parent` per the aggregator decisions (quota = regulator with hard-stop in `TarifyMessage`).

## Go/no-go decision for Phase A1

**Decision: GO for Phase A1.**

B1, B2, B3 do not block A1 (OpenAPI↔code comparison is a pure HTTP-contract task, independent of SMPP and integration infra).

### A1 scope estimate

Per inventory, Phase A1 needs to compare `api/openapi/openapi.yaml` against 28 HTTP endpoints (excluding health and docs). Given the spec is hand-written and 7 days behind the handlers, expect drift in at least:
- Cascade endpoints (identified in Task 2 as returning proto types directly vs the other handlers' inline maps — likely not reflected in openapi.yaml yet).
- Any endpoint added or modified in the last 7 days (verify via `git log --since=2026-04-14 internal/gateway/client/handlers/`).

### Go/no-go for other phases

- **Phase A2** (error format): GO. Independent of blockers.
- **Phase A3** (idempotency): GO. Finding expected: idempotency largely absent; the plan will mostly be "add Idempotency-Key middleware + SMPP dedup", likely a crowded but unblocked phase.
- **Phase A6** (SMPP contract): GO. Huge scope based on Task 4 finding (no TLV constants at all) — this will be the largest downstream plan.
- **Phase C** (subaccount correctness): **NO-GO** until B1 resolved (or explicit downscope). Recommend: write a separate mini-plan `api-review-phase-c-infra.md` that either adds postgres/redis service containers + integration CI job, or formally reduces Phase C to unit+fakes with explicit loss-of-coverage acknowledged. Additionally B2 must be fixed before SMPP C-visibility tests land.
