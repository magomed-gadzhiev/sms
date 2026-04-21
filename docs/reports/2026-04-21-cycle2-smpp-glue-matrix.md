# Cycle 2 — SMPP + tenant-glue Coverage Matrix

**Snapshot date:** 2026-04-21
**Spec:** `docs/superpowers/specs/2026-04-21-api-review-v2-umbrella.md`
**Plan:** `docs/superpowers/plans/2026-04-21-api-review-v2-cycle2-smpp-tenant.md`
**Phase completed:** skeleton (all UNKNOWN)
**OK rows with test_file (ratchet baseline):** 0
**Total rows:** заполнить в финале

## Legend

- `current_state`: `OK` / `DRIFT` / `MISSING` / `BROKEN` / `UNKNOWN`
- `severity`: `critical` (биллинг/auth) / `major` (контракт) / `minor` (косметика)
- `action`: `fixed-in-PR#<N>` / `plan:<path>` / `test-added:<path>` / `wontfix:<reason>`

## Matrix

| surface | transport | role | axis | current_state | test_file | gap | severity | action |
|---|---|---|---|---|---|---|---|---|
| SMPP bind_transmitter | SMPP | client | A6-auth | BROKEN | | password ignored, system_id only | critical | plan:docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md |
| SMPP bind_transmitter | SMPP | subaccount_child | A6-auth | BROKEN | | password ignored, system_id only | critical | plan:docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md |
| SMPP bind_transmitter | SMPP | subaccount_parent | A6-auth | BROKEN | | password ignored, system_id only | critical | plan:docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md |
| SMPP bind_receiver | SMPP | client | A6-auth | BROKEN | | password ignored, system_id only | critical | plan:docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md |
| SMPP bind_receiver | SMPP | subaccount_child | A6-auth | BROKEN | | password ignored, system_id only | critical | plan:docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md |
| SMPP bind_receiver | SMPP | subaccount_parent | A6-auth | BROKEN | | password ignored, system_id only | critical | plan:docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md |
| SMPP bind_transceiver | SMPP | client | A6-auth | BROKEN | | password ignored, system_id only | critical | plan:docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md |
| SMPP bind_transceiver | SMPP | subaccount_child | A6-auth | BROKEN | | password ignored, system_id only | critical | plan:docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md |
| SMPP bind_transceiver | SMPP | subaccount_parent | A6-auth | BROKEN | | password ignored, system_id only | critical | plan:docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md |

## Machine-readable

См. `2026-04-21-cycle2-smpp-glue-matrix.csv`.
