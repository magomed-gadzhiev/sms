# Cycle 2 — SMPP + tenant-glue Findings

**Spec:** `docs/superpowers/specs/2026-04-21-api-review-v2-umbrella.md`
**Plan:** `docs/superpowers/plans/2026-04-21-api-review-v2-cycle2-smpp-tenant.md`
**Branch:** `review/api-v2-cycle2-smpp-tenant`

## A6-auth — SMPP password ignored

**Severity:** critical

### Call chain

1. Protocol layer (`internal/smpp/protocol/pdu.go:18-26`): `BindPDU` struct has `Password string` field; the decoder populates it from the wire frame.
2. SMPP handler (`internal/gateway/smpp/server/handler.go:126`, `174`, `222`): each bind variant calls `h.authenticate(bind.SystemID, bind.Password)` — both fields are extracted from the decoded PDU and passed forward.
3. Dispatch (`internal/gateway/smpp/server/handler.go:637-652`): `authenticate()` receives `systemID, password string` and calls `h.authAdapter.AuthenticateBySystemID(ctx, systemID, password)`.
4. Adapter (`internal/gateway/smpp/server/auth_adapter.go:71-115`): `AuthenticateBySystemID(ctx context.Context, systemID, password string)` — signature accepts `password` but the body constructs `&authv1.AuthenticateRequest{ApiKey: systemID}`, leaving the `password` argument silently discarded.
5. authv1 call: `authv1.AuthenticateRequest` has three fields — `Username string`, `Password string`, `ApiKey string` (`api/proto/authv1/auth.pb.go:26-33`). The adapter sets only `ApiKey: systemID`; `Password` is never populated.
6. auth-service impl (`internal/services/auth/grpc/server.go:62-76`): `Authenticate()` branches on `req.ApiKey != ""` and calls `AuthenticateByAPIKey(ctx, req.ApiKey)` — which looks up the API key hash in the database. No password check of any kind is performed in this path.

### Root cause

The break is at step 4: `AuthenticateBySystemID` in `auth_adapter.go:76-78` maps `systemID` to `ApiKey` and drops the `password` argument entirely, so the auth-service never receives it and performs a pure API-key lookup with zero password verification.

### Evidence

```
// auth_adapter.go:76-78
resp, err := a.authClient.Authenticate(ctx, &authv1.AuthenticateRequest{
    ApiKey: systemID,
})

// auth_adapter.go:71 — password param accepted but unused
func (a *AuthAdapter) AuthenticateBySystemID(ctx context.Context, systemID, password string) (UserInfo, error) {

// auth.pb.go:26-30 — Password field exists in proto message
type AuthenticateRequest struct {
    Username string `protobuf:"bytes,1,opt,name=username,proto3"`
    Password string `protobuf:"bytes,2,opt,name=password,proto3"`
    ApiKey   string `protobuf:"bytes,3,opt,name=api_key,json=apiKey,proto3"`

// grpc/server.go:68-69 — API-key path never touches password
if req.ApiKey != "" {
    user, err = s.authService.AuthenticateByAPIKey(ctx, req.ApiKey)
```

### Impact

Any SMPP client that knows a valid `system_id` (API key) can bind with an arbitrary password and gain full access. All three bind types — `bind_transmitter`, `bind_receiver`, `bind_transceiver` — are affected for every client role (client, subaccount_child, subaccount_parent). This is an authentication bypass.

### Decision

This is a **large fix** per umbrella ε+hard-gate criteria (touches auth logic). NOT an in-PR fix. Separate plan: `docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md`.

## A6-tlv + A6-submit-ext

_TODO: Task 3_

## A6-dlr + A6-registered-delivery

_TODO: Task 4_

## A6-datacoding + A6-long-msg

_TODO: Task 5_

## A7 — gRPC tenant propagation

_TODO: Task 6_

## Сводная таблица critical / major находок

_TODO: Task 7_
