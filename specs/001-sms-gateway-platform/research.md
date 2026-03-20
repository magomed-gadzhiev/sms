# Research: SMS Gateway Platform

**Date**: 2026-03-20
**Feature**: 001-sms-gateway-platform

## Findings

### 1. Multipart SMS (Concatenated SMS / UDH)

**Decision**: Use User Data Header (UDH) for multipart message segmentation per GSM 03.40 standard.

**Rationale**: UDH is the industry standard for concatenated SMS. Each segment carries a reference number, total parts count, and sequence number. Maximum 160 chars per segment (or 153 when UDH is present). The existing custom SMPP implementation in `internal/smpp/protocol/` already handles PDU encoding and can be extended to support UDH fields in `submit_sm`.

**Alternatives considered**:
- Message payload TLV (optional SMPP parameter) — less universally supported by SMSC providers
- Application-level splitting without UDH — recipient sees separate messages, poor UX

### 2. DLR Expiry Mechanism

**Decision**: Background goroutine that periodically scans for messages in "sent" status older than the configurable timeout (default 24h) and transitions them to "expired".

**Rationale**: Follows the same pattern as the existing scheduler goroutine in messaging-service (configured via `SCHEDULER_INTERVAL`). Leverages existing infrastructure. PostgreSQL partitioned `messages` table supports efficient time-range queries.

**Alternatives considered**:
- Per-message timer via Redis TTL — higher complexity, Redis memory pressure at scale
- Kafka delayed message — Kafka doesn't natively support delayed consumption

### 3. Data Retention / Purge Strategy

**Decision**: PostgreSQL partitioned tables (monthly partitions on `created_at`) with partition drop for expired data. 90 days for messages, 1 year for audit logs.

**Rationale**: The `messages` table is already partitioned by month. Dropping old partitions is O(1) and doesn't create table bloat or vacuum pressure. Audit log table (`audit_log`) is also partitioned monthly.

**Alternatives considered**:
- DELETE with batch processing — slow on large tables, creates dead tuples
- Archive to cold storage then delete — adds complexity, not needed at current scale

### 4. Per-Segment Billing

**Decision**: Charge is calculated at message submission time based on segment count. Each segment is billed independently using the destination-based pricing rule.

**Rationale**: SMSC providers bill per segment, so pass-through billing matches cost structure. Balance check at submission time prevents mid-delivery insufficient funds. The billing-service already has pricing rules with regex patterns on destinations.

**Alternatives considered**:
- Bill on delivery receipt — creates billing uncertainty, client can't predict cost
- Flat rate per logical message — doesn't reflect actual provider costs

### 5. Scheduling Horizon Validation

**Decision**: Validate `scheduled_at` at the gateway level: must be in the future and within 30 days. Reject with 400 error otherwise.

**Rationale**: Gateway-level validation provides fast feedback. 30-day limit aligns with message retention (90 days) and prevents orphaned scheduled messages. Existing client-gateway validation pattern (phone number, message body) extends naturally.

**Alternatives considered**:
- Validate in messaging-service — slower feedback, request already entered the system
- No maximum limit — risk of indefinitely stored pending messages

### 6. Existing Architecture Assessment

**Decision**: The existing codebase already implements the majority of the platform specification. Key gaps are: multipart SMS handling, DLR expiry, data purging, and per-segment billing calculation.

**Rationale**: Code exploration confirms:
- All 9 microservices exist with gRPC interfaces
- Client/Admin gateways with HTTP + gRPC protocols
- SMPP gateway with custom protocol implementation
- Kafka event streaming for async processing
- PostgreSQL storage with partitioning
- Redis caching and rate limiting
- Prometheus monitoring
- Scheduled message support (recently added)
- Webhook service exists
- Template service exists with moderation workflow

**Gaps identified**:
1. No multipart SMS splitting logic in SMPP protocol layer
2. No DLR expiry mechanism
3. No automated data purging
4. Billing calculates per message, not per segment
5. Message history endpoint doesn't expose segment count
