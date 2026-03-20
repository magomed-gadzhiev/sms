# SMPP Protocol Contract

**Date**: 2026-03-20
**Protocol**: SMPP v3.4 over TCP (port 2775)
**Authentication**: system_id + password via bind PDU

## Supported Operations

### Session Management

| Operation | Direction | Description |
|-----------|-----------|-------------|
| bind_transmitter | Client → Server | Bind as transmitter (send only) |
| bind_receiver | Client → Server | Bind as receiver (receive DLR only) |
| bind_transceiver | Client → Server | Bind as transceiver (send + receive) |
| bind_*_resp | Server → Client | Bind response with status |
| unbind | Bidirectional | Close session |
| unbind_resp | Bidirectional | Unbind acknowledgement |
| enquire_link | Bidirectional | Keep-alive heartbeat |
| enquire_link_resp | Bidirectional | Heartbeat response |

### Message Operations

| Operation | Direction | Description |
|-----------|-----------|-------------|
| submit_sm | Client → Server | Submit SMS for delivery |
| submit_sm_resp | Server → Client | Response with message_id |
| deliver_sm | Server → Client | Delivery receipt (DLR) |
| deliver_sm_resp | Client → Server | DLR acknowledgement |
| query_sm | Client → Server | Query message status |
| query_sm_resp | Server → Client | Status response |

### Authentication Flow

1. Client opens TCP connection to port 2775
2. Client sends `bind_transceiver` with `system_id` and `password`
3. Server validates credentials via Auth Service
4. Server returns `bind_transceiver_resp` with status 0x00000000 (OK) or error
5. Session is established with per-session rate limiting

### Multipart SMS via UDH

For messages > 160 characters, client may send multiple `submit_sm` PDUs with UDH:
- ESM class: 0x40 (UDHI indicator)
- UDH: reference_number (2 bytes), total_parts (1 byte), part_number (1 byte)
- Server correlates segments by reference_number

### Error Codes

| Code | Description |
|------|-------------|
| 0x00000000 | OK |
| 0x00000001 | Message length invalid |
| 0x00000005 | Already bound |
| 0x0000000D | Bind failed |
| 0x00000045 | Throttling error |
| 0x00000058 | Invalid scheduled delivery time |
| 0x00000088 | Insufficient credits |
