# API Review v2 — Surface Inventory

**Snapshot date:** 2026-04-21
**Spec:** `docs/superpowers/specs/2026-04-21-api-review-v2-umbrella.md`
**Revision (master HEAD):** <ХЭШ> — заполнить в Task 8

## 1. HTTP Endpoints (cmd/client-gateway)

_TODO: Task 2_

## 2. gRPC External (cmd/client-gateway CLIENT_GRPC_PORT)

_TODO: Task 3_

## 3. SMPP (cmd/smpp-gateway)

### 3.1. PDU Commands

_TODO: Task 4_

### 3.2. SMPP TLVs

_TODO: Task 4_

### 3.3. smppv1 control-plane RPC

_TODO: Task 4_

## 4. gRPC Internal (top-5 hot-path services)

_TODO: Task 5_

## 5. Findings

### F1. R1 — cmd/api legacy status

_TODO: Task 6_

### F2. B1 — Integration test infrastructure

_TODO: Task 7_

### F3. B2 — AuthAdapter user_id=client_id kludge

_TODO: Task 7_

### F4. B3 — Dual-charge wiring in TarifyMessage

_TODO: Task 7_

## 6. Go/no-go для циклов 1/2/3

_TODO: Task 8_
