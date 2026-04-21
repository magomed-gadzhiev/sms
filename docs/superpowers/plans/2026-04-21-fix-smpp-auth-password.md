# Fix Plan: SMPP bind password verification

**Severity:** critical
**Source:** Cycle 2 finding A6-auth (`docs/reports/2026-04-21-cycle2-findings.md`)
**Spec reference:** `docs/superpowers/specs/2026-04-21-api-review-v2-umbrella.md`

## Problem

Any SMPP client that knows a valid `system_id` (API key) can bind with an arbitrary password and gain full access. All three bind types — `bind_transmitter`, `bind_receiver`, `bind_transceiver` — are affected for every client role (client, subaccount_child, subaccount_parent). This is an authentication bypass.

Root cause: `AuthenticateBySystemID` in `internal/gateway/smpp/server/auth_adapter.go:76-78` accepts a `password` parameter but constructs `&authv1.AuthenticateRequest{ApiKey: systemID}`, silently dropping `password`. The `authv1.AuthenticateRequest` proto message already has a `Password string` field (field 2), so no proto change is strictly required — the field is present but never populated. The auth-service `Authenticate()` handler (`internal/services/auth/grpc/server.go:68-69`) currently branches on `req.ApiKey != ""` and calls `AuthenticateByAPIKey`, which performs no password check.

## Proposed fix

The design must thread the password all the way from the bind PDU into the auth-service and verify it there.

Three changes are required:

1. **Auth-service application layer** — add `AuthenticateByAPIKeyAndPassword(ctx, apiKey, password string) (*domain.User, error)` (or extend `AuthenticateByAPIKey` with an optional password argument). When a non-empty password is supplied, compare it against the stored password hash for the user who owns the API key using `PasswordHasher.Compare`. Return `ErrAPIKeyInvalid` on mismatch to avoid distinguishing "wrong key" from "wrong password" (timing side-channel prevention).

2. **Auth-service gRPC handler** (`internal/services/auth/grpc/server.go:62`) — when `req.ApiKey != ""` and `req.Password != ""`, call the new `AuthenticateByAPIKeyAndPassword` instead of the plain `AuthenticateByAPIKey`. This is backward-compatible: existing callers that send only `ApiKey` (portals, REST gateway) are unaffected.

3. **SMPP AuthAdapter** (`internal/gateway/smpp/server/auth_adapter.go:76-78`) — pass `Password: password` in the `AuthenticateRequest`:
   ```
   &authv1.AuthenticateRequest{
       ApiKey:   systemID,
       Password: password,
   }
   ```

Consider backward-compatibility: existing SMPP clients may have been binding with arbitrary passwords because there was no enforcement. Rolling out password verification will break them. A transition plan is required (see Tasks below).

## Tasks (to be detailed when taken up)

- [ ] Задача 1: Verify proto has password field — confirmed present (`auth.pb.go:29`). No proto change needed. Document this so the implementer doesn't regenerate unnecessarily.
- [ ] Задача 2: Add `AuthenticateByAPIKeyAndPassword` to auth-service application layer; use `PasswordHasher.Compare` against the user's password hash retrieved via the API-key lookup path.
- [ ] Задача 3: Update `internal/services/auth/grpc/server.go` `Authenticate()` to invoke the new method when both `ApiKey` and `Password` are non-empty.
- [ ] Задача 4: Update `AuthenticateBySystemID` in `internal/gateway/smpp/server/auth_adapter.go` to populate `Password: password` in `AuthenticateRequest`.
- [ ] Задача 5: Unit test: valid system_id + wrong password → `ESME_RINVPASWD` bind_resp.
- [ ] Задача 6: Unit test: valid system_id + correct password → bind succeeds.
- [ ] Задача 7: Transition plan — decide soft-rollout window. Options: (a) immediate hard-fail (breaks existing clients), (b) 2-week warning period with `log.Warn` on empty/wrong password but still allow bind, then flip to hard-fail. Recommendation: option (b) to avoid surprise breakage on first deploy.
- [ ] Задача 8: Production rollout with monitoring — alert on `ESME_RINVPASWD` spike after rollout; have rollback procedure ready (feature flag or config toggle `smpp.enforce_password_check: bool`).

## Risks

- Rolling out password verification immediately will break clients currently binding with any password. Need soft-rollout window of at least 1-2 weeks with advance notice.
- If users have never set an SMPP password (field left empty by the operator), the stored hash may be absent or correspond to an empty string. The fix must handle the `password == ""` case explicitly (reject or treat as legacy grace period).
- The auth-service password hash lookup requires joining the API-key record to the owning user's password hash. This is an additional DB read per bind; acceptable at bind frequency but must be considered.

## Dependencies

- No external dependencies. Depends on decision about soft-rollout window (immediate vs. grace period) — needs product sign-off.
- `PasswordHasher` interface and `PasswordHasherImpl` (bcrypt) already exist in `internal/services/auth/infrastructure/password_hasher.go`.

## Estimated size

Large per ε + hard-gate (auth code, touches three layers: adapter, gRPC handler, application service). Estimated 3-4 hours implementation + test + rollout plan.
