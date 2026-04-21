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

**Severity:** major

### A6-tlv: inbound TLV discard

TLVs are correctly parsed at the protocol layer. `DecodeSubmitSM` in `internal/smpp/protocol/decoder.go` calls `readTLV()` and stores the result in `submit.TLV` (a `map[uint16][]byte`). The struct field is populated on every submit_sm that carries optional parameters. The problem occurs one layer up: `handleSubmitSM` in `internal/gateway/smpp/server/handler.go` decodes the PDU into a `*SubmitSMPDU` but then constructs a `queue.KafkaMessage` using only mandatory fields — `submit.ShortMessage`, `submit.SourceAddr`, `submit.DestinationAddr`, `submit.ESMClass`, `submit.DataCoding`, `submit.PriorityFlag`, `submit.RegisteredDelivery`, and the address TON/NPI fields. `submit.TLV` is never read at any point. The map goes out of scope with the local `submit` variable and is garbage-collected.

**Evidence:**

```
// decoder.go:255-264 — TLV is parsed and stored in submit.TLV
if d.reader.Len() > 0 {
    tlv, err := d.readTLV()
    ...
    submit.TLV = tlv
}

// handler.go:297-360 — submit decoded, TLV never referenced
submit, err := h.decoder.DecodeSubmitSM(pdu.Body)
...
kafkaMsg := &queue.KafkaMessage{
    Source:      submit.SourceAddr,
    Destination: submit.DestinationAddr,
    Text:        messageText,
    // submit.TLV — not accessed anywhere in this function
}
```

**Missing TLV reading (impact list):**

- `sar_msg_ref_num` / `sar_total_segments` / `sar_segment_seqnum` (0x020C/0x020E/0x020F) — long message segmentation via TLV SAR: not read at all. Gateway only handles long messages via UDH (`ESMClass & 0x40`). Integrators that split long messages using TLV SAR (instead of UDH) will have every segment accepted with ESME_ROK but arrive at Kafka with no segmentation metadata — the downstream consumer cannot reassemble them. The result is N separate truncated messages delivered independently.
- `user_message_reference` (0x0204) — request correlation: many enterprise SME clients set this to track their own message IDs end-to-end. It is never forwarded to `KafkaMessage.Metadata` or any downstream model. Integrators relying on this field for correlation cannot match send requests to DLRs.
- `message_payload` (0x0424) — allows body > 254 bytes without UDH encoding: not read. If a client sets `sm_length=0` and puts the text in `message_payload`, `submit.ShortMessage` will be empty and `messageText` will be an empty string. The message is accepted (ESME_ROK) and published to Kafka with an empty body.
- `receipted_message_id` (0x001E) and `message_state` (0x0427) on inbound side — not applicable here (these are outbound DLR TLVs). The outbound DLR path in `internal/smsc/pool_async.go:342-358` does read them correctly as a fallback for parsing DLRs from providers. This is unrelated to the inbound submit_sm gap.

### A6-submit-ext: submit_multi_sm / data_sm

**submit_multi_sm:** declared at `internal/smpp/protocol/constants.go:24` as `SubmitMultiSM = 0x00000021`. No case for `protocol.SubmitMultiSM` in the `switch pdu.CommandID` at `handler.go:73-100`. Falls through to `default:` at `handler.go:94-100`, which calls `h.sendGenericNack(pdu.SequenceNumber, protocol.ESME_RINVCMDID)`.

**data_sm:** declared at `internal/smpp/protocol/constants.go:30` as `DataSM = 0x00000103`. Same path — no handler case, falls to `default:` → `sendGenericNack(ESME_RINVCMDID)`.

**Real-world usage risk:**

`submit_multi_sm` was deprecated in SMPP v3.4 in favor of multiple `submit_sm` calls, but it remains in active use by legacy enterprise aggregators and some tier-2 operators that were written against SMPP v3.3 or early v3.4 toolkits (Openwave, CMG, SEMA group libraries). The PDU sends one request with a destination list and receives a single `submit_multi_resp` with per-destination status codes. A client using it will get `generic_nack` with `ESME_RINVCMDID` (invalid command ID), which most SMPP clients interpret as a protocol error and may disconnect or retry in a tight loop. The risk is real for any aggregator integration that hasn't been audited for PDU type.

`data_sm` is the SMPP v3.4 mechanism for sending messages with large payloads exclusively via the `message_payload` TLV (no short_message field). It is used by MMS gateways and some modern binary-content platforms. Enterprise usage is lower than `submit_sm` but not negligible. Clients sending `data_sm` will also receive `generic_nack(ESME_RINVCMDID)`.

Neither PDU type silently corrupts data — the client receives an explicit error. The risk is integration failure (not silent data loss), which makes this `major` rather than `critical`. That said, returning `ESME_RINVCMDID` (invalid command ID) instead of `ESME_RINVSYSTYP` or a properly formatted `data_sm_resp`/`submit_multi_resp` is spec-noncompliant and will confuse well-behaved SMPP client stacks.

### Decisions

- **A6-tlv**: крупное (требует TLV tag constants + gateway-to-service passthrough + messaging.Message model change). Fix-plan `docs/superpowers/plans/2026-04-21-fix-smpp-tlv-support.md` — **не создаём в этом task'е**, выделим в финализации Cycle 2 если нужно. Здесь только matrix + gap.
- **A6-submit-ext**: если низкое использование — `wontfix` с пометкой что клиенты должны использовать submit_sm. Если высокое — отдельный план.

## A6-dlr + A6-registered-delivery

**Severity:** major

### A6-registered-delivery (outbound submit_sm)

**registered_delivery byte extraction:** yes. `handler.go:339` — `"registered_delivery": submit.RegisteredDelivery` is set into the `metadata` map of `queue.KafkaMessage`.

**Where it's propagated:** into `KafkaMessage.Metadata` as a raw `interface{}` value under the key `"registered_delivery"`. It is NOT forwarded to the provider as part of outbound SMPP submit_sm. The metadata map is opaque — downstream consumers (the outbound worker, the SMSC pool `SubmitSM` call in `internal/smsc/`) receive the Kafka message but there is no evidence in the SMSC layer that this metadata key is read back and used to set `registered_delivery` on the outbound `submit_sm` PDU sent to the provider. In effect the byte is logged but dropped before provider transmission.

**SMPP v3.4 semantics:** bits 0-1 = receipt request (00=never, 01=always, 10=on failure, 11=reserved); bits 2-3 = SME-originated acknowledgement; bits 4-5 = intermediate notification request.

**Gap:** Bit-level semantics are entirely ignored. The byte is captured and placed in an opaque metadata map, but: (a) it is not used to condition whether the outbound submit_sm to the provider requests a receipt (bits 0-1), (b) bits 2-5 are never acted upon. The downstream consumer would need explicit code to extract `registered_delivery` from `KafkaMessage.Metadata`, coerce it to `byte`, and write it into the provider-bound submit_sm. That code does not exist. Result: every outbound message to providers behaves as if `registered_delivery=0x01` (or whatever is hardcoded in the SMSC pool `SubmitSM` builder) regardless of what the client requested.

### A6-dlr (inbound via SMPP link vs gRPC)

Two DLR delivery paths exist in the codebase:

1. **Via SMPP deliver_sm from provider** (inbound via `handleDeliverSM` in gateway):
   - Does handler check `esm_class & 0x04`? No. `handleDeliverSM` at `handler.go:547-591` decodes the PDU, reads only `deliver.ShortMessage`, trims it, checks for opt-out stop-keywords, and returns `ESME_ROK`. There is zero branching on `deliver.ESMClass`. A DLR arriving with `esm_class=0x04` is processed identically to a plain MO with `esm_class=0x00`.
   - Does handler read `receipted_message_id` (0x001E) or `message_state` (0x0427) TLV? No. These TLVs exist in the protocol layer (`deliver.TLV`) but `handleDeliverSM` never accesses `deliver.TLV` at all.
   - What happens to a real DLR arriving via deliver_sm on the gateway link? It is silently treated as a MO. The message text (the DLR receipt string such as `id:XXX sub:001 dlvrd:001 ... stat:DELIVRD`) is compared against opt-out keywords (`STOP`, `СТОП`, etc.) — it will not match — and then `ESME_ROK` is returned. The DLR data is discarded. It is never matched to the original message, never persisted as a delivery status update, and never forwarded to the sending client. Completely lost.
   - Note: correct DLR parsing (regex + TLV fallback) exists in `internal/smsc/pool_async.go:322-363` — but this is the *outbound* SMPP connection to providers. Provider → gateway direction is handled. The *inbound* gateway direction (client → gateway, where client acts as a provider and pushes DLRs) is the broken path.

2. **Via smppv1.DeliverDLR gRPC** (control-plane DLR injection):
   - Handler file: `internal/gateway/smpp/server/grpc_server.go:45` — `func (g *GRPCServer) DeliverDLR(ctx context.Context, req *smppv1.DeliverDLRRequest) (*smppv1.DeliverDLRResponse, error)`.
   - Caller: `internal/services/dlr/dispatcher.go:52` — `Dispatcher.Dispatch()` calls `d.client.DeliverDLR(ctx, req.SystemID, req.SourceAddr, req.DestAddr, req.Receipt)`. The dispatcher is the DLR service that processes provider-side DLRs (from pool_async `dlrCallback`) and routes them to clients.
   - What it does: looks up the active SMPP session by `system_id`, checks `sess.CanReceive()` (receiver/transceiver only), builds a `deliver_sm` PDU with `ESMClass=0x04` and the DLR receipt text in `ShortMessage`, and writes it directly to the session's TCP connection (`grpc_server.go:129-176`). Has prometheus metrics (`SMPPDLRDelivered`, `SMPPDLRDeliveryFailed`) and retry logic in the dispatcher (3 retries with exponential backoff).
   - This path is fully implemented and tested (`internal/gateway/smpp/server/grpc_server_test.go` — 4 test cases covering success, receiver bind, session not found, connection closed).

**DLR → client flow (SMPP receiver bind):**

Outbound deliver_sm to SMPP-bound clients is supported and implemented via the gRPC path. The flow is: provider delivers DLR → `pool_async.go` `dlrCallback` parses DLR data → DLR service invokes `Dispatcher.Dispatch()` → gRPC call to `GRPCServer.DeliverDLR()` → `sendDeliverSM()` writes `deliver_sm` with `esm_class=0x04` to the client's TCP connection. Clients bound as `receiver` or `transceiver` receive the DLR over their SMPP link.

The gap is not in outbound delivery to clients — that works. The gap is specifically in the *inbound gateway handler* (`handleDeliverSM`), which is the path used when a connected SMPP peer (acting as a provider) pushes DLRs to the gateway via the gateway's listen port. This is a less common topology (gateway as ESME, not SMSC) but it is the path that `handleDeliverSM` exists to serve.

### Decisions

- **A6-registered-delivery:** partial support — byte is extracted and stashed in Kafka metadata but never read downstream to condition the outbound submit_sm to the provider. Fix is medium-sized: requires tracing `KafkaMessage.Metadata["registered_delivery"]` through the outbound worker and into the SMSC pool's `SubmitSM` call. Estimate ~50-80 lines across 2-3 files. Not an inline Cycle 2 PR fix; crупное — document and plan separately.
- **A6-dlr inbound via SMPP:** крупное. The `handleDeliverSM` function needs `esm_class & 0x04` branching, TLV reading for `receipted_message_id` / `message_state`, and routing to the DLR service instead of the opt-out handler. Separate fix-plan needed: `docs/superpowers/plans/2026-04-21-fix-smpp-deliver-dlr-branching.md` — **не создаём сейчас**, помечаем в matrix.
- **A6-dlr outbound к SMPP-клиенту:** fully supported via `smppv1.DeliverDLR` gRPC + `sendDeliverSM`. Not a gap. Matrix row: `OK`.

## A6-datacoding + A6-long-msg

_TODO: Task 5_

## A7 — gRPC tenant propagation

_TODO: Task 6_

## Сводная таблица critical / major находок

_TODO: Task 7_
