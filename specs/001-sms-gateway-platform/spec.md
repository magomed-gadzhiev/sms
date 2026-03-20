# Feature Specification: SMS Gateway Platform

**Feature Branch**: `001-sms-gateway-platform`
**Created**: 2026-03-20
**Status**: Implemented
**Input**: User description: "SMS Gateway Platform - enterprise microservices SMS delivery system"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Send Single SMS (Priority: P1)

As an API client, I want to submit an SMS message for delivery so that my end users receive notifications, alerts, or communications via text message.

**Why this priority**: Message sending is the core value proposition of the platform. Without it, no other feature has meaning.

**Independent Test**: Can be fully tested by sending a single SMS via the API and confirming delivery status updates through to final delivery receipt.

**Acceptance Scenarios**:

1. **Given** a client with a valid API key and sufficient balance, **When** they submit a single SMS with a valid destination number and message body, **Then** the system accepts the message, returns a unique message ID, and the message progresses through delivery stages until delivered.
2. **Given** a client with a valid API key but insufficient balance, **When** they attempt to send an SMS, **Then** the system rejects the request with a clear error indicating insufficient funds.
3. **Given** a client with an invalid or expired API key, **When** they attempt to send an SMS, **Then** the system rejects the request with an authentication error.
4. **Given** a client submitting a message with an invalid destination number, **When** the request is processed, **Then** the system returns a validation error specifying the issue.

---

### User Story 2 - Track Message Delivery Status (Priority: P1)

As an API client, I want to check the delivery status of a submitted message so that I know whether my message was successfully delivered.

**Why this priority**: Delivery tracking is essential for clients to confirm message receipt and take action on failures. Equally critical to sending.

**Independent Test**: Can be tested by sending a message and then querying its status at each lifecycle stage (queued, sent, delivered, or failed).

**Acceptance Scenarios**:

1. **Given** a previously submitted message, **When** the client queries the message status by ID, **Then** the system returns the current status (pending, queued, sent, delivered, failed, or expired) with timestamps.
2. **Given** a message ID that does not exist, **When** the client queries for it, **Then** the system returns a "not found" error.
3. **Given** a message belonging to another client, **When** a client queries for it, **Then** the system denies access.

---

### User Story 3 - Send Batch SMS (Priority: P2)

As an API client, I want to send multiple SMS messages in a single request so that I can efficiently deliver campaigns or bulk notifications.

**Why this priority**: Batch sending is a core business requirement for marketing campaigns and bulk alerts, but depends on single-message sending working first.

**Independent Test**: Can be tested by submitting a batch of messages and verifying each receives an individual message ID and progresses through delivery independently.

**Acceptance Scenarios**:

1. **Given** a client with valid credentials and sufficient balance, **When** they submit a batch of valid SMS messages, **Then** the system accepts all messages and returns individual message IDs for each.
2. **Given** a batch containing some invalid messages (e.g., bad phone numbers), **When** submitted, **Then** the system accepts valid messages and returns errors for invalid ones (partial success).
3. **Given** a batch exceeding the maximum allowed size, **When** submitted, **Then** the system rejects the entire batch with an appropriate error.

---

### User Story 4 - Schedule SMS for Future Delivery (Priority: P2)

As an API client, I want to schedule an SMS for delivery at a specific future time so that I can plan communications in advance.

**Why this priority**: Scheduled messaging enables planned campaigns and time-sensitive notifications, adding significant business value beyond immediate sending.

**Independent Test**: Can be tested by scheduling a message for a future time, confirming it enters "scheduled" status, and verifying it is sent at the designated time.

**Acceptance Scenarios**:

1. **Given** a client submitting a message with a future `scheduled_at` timestamp, **When** the system accepts it, **Then** the message enters "scheduled" status and is held until the specified time.
2. **Given** a scheduled message that has not yet been sent, **When** the client requests cancellation, **Then** the message status changes to "cancelled" and it is not delivered.
3. **Given** a scheduled message whose delivery time has arrived, **When** the scheduler processes it, **Then** the message enters the normal delivery pipeline and is sent.
4. **Given** a client submitting a message with a `scheduled_at` time in the past, **When** the system receives it, **Then** the system rejects the request with a validation error.

---

### User Story 5 - Manage Client Accounts (Priority: P2)

As a platform administrator, I want to create, configure, and manage API client accounts so that I can onboard new clients and control their access and rate limits.

**Why this priority**: Client management is essential for operating a multi-tenant platform, but the platform can initially operate with pre-configured clients.

**Independent Test**: Can be tested by creating a client account via the admin API, configuring rate limits and allowed source addresses, and verifying the client can authenticate and send messages within those constraints.

**Acceptance Scenarios**:

1. **Given** an administrator with valid credentials, **When** they create a new client account with API key, rate limits, and allowed source addresses, **Then** the client account is active and the client can authenticate using the generated API key.
2. **Given** an existing client account, **When** the administrator deactivates it, **Then** the client can no longer authenticate or send messages.
3. **Given** a client with configured rate limits, **When** the client exceeds their allowed sending rate, **Then** the system throttles or rejects excess messages with a rate-limit error.

---

### User Story 6 - Receive Delivery Notifications via Webhook (Priority: P3)

As an API client, I want to receive push notifications about message delivery status changes so that I can react to delivery events in real time without polling.

**Why this priority**: Webhooks improve client integration experience and reduce polling load, but clients can function with status polling alone.

**Independent Test**: Can be tested by registering a webhook endpoint, sending a message, and verifying the system delivers a signed callback when the message status changes.

**Acceptance Scenarios**:

1. **Given** a client with a registered webhook URL, **When** a message status changes (sent, delivered, failed), **Then** the system sends an HTTP POST to the webhook URL with the message details and a cryptographic signature.
2. **Given** a webhook delivery that fails, **When** the system retries, **Then** it uses exponential backoff for up to 5 attempts before marking the notification as failed.
3. **Given** a client managing their webhooks, **When** they create, update, or delete a webhook subscription, **Then** the changes take effect immediately.

---

### User Story 7 - Use Message Templates (Priority: P3)

As an API client, I want to send messages using pre-approved templates with variable substitution so that I can ensure message consistency and compliance.

**Why this priority**: Templates add compliance and convenience but are not required for basic messaging operations.

**Independent Test**: Can be tested by creating a template with placeholders, getting it approved by an admin, and sending a message referencing the template with variable values.

**Acceptance Scenarios**:

1. **Given** an approved template with placeholders (e.g., `{{name}}`), **When** a client sends a message referencing the template with variable values, **Then** the system renders the final message by substituting the variables and delivers it.
2. **Given** a template in "draft" status, **When** a client tries to use it, **Then** the system rejects the request indicating the template is not yet approved.
3. **Given** a template with required variables, **When** a client sends a message missing one or more required variables, **Then** the system returns a validation error.

---

### User Story 8 - View Analytics and Reports (Priority: P3)

As a platform administrator, I want to view message statistics, provider performance metrics, and generate reports so that I can monitor platform health and make informed decisions.

**Why this priority**: Analytics provide operational insight and business intelligence, but the platform operates without them.

**Independent Test**: Can be tested by sending several messages, then querying the analytics API for statistics by time period, client, provider, and status, and generating a downloadable report.

**Acceptance Scenarios**:

1. **Given** messages have been sent over a time period, **When** an administrator queries statistics filtered by client, provider, status, or date range, **Then** the system returns accurate aggregated metrics.
2. **Given** an administrator requesting a report, **When** they specify the format (JSON, CSV), **Then** the system generates and returns the report in the requested format.

---

### User Story 9 - Connect via SMPP Protocol (Priority: P3)

As a legacy SMPP client, I want to connect to the platform using the SMPP v3.4 protocol so that I can send and receive SMS without changing my existing integration.

**Why this priority**: SMPP support enables legacy client compatibility, but modern clients use HTTP/gRPC APIs.

**Independent Test**: Can be tested by establishing an SMPP bind session, submitting a message via `submit_sm`, and receiving a delivery receipt via `deliver_sm`.

**Acceptance Scenarios**:

1. **Given** valid SMPP credentials, **When** a client sends a bind request (receiver, transmitter, or transceiver), **Then** the system authenticates and establishes a session.
2. **Given** an active SMPP session, **When** the client submits a `submit_sm` PDU, **Then** the system accepts the message and returns a `submit_sm_resp` with a message ID.
3. **Given** a delivered message, **When** the provider returns a delivery receipt, **Then** the system forwards a `deliver_sm` PDU to the connected SMPP client.

---

### Edge Cases

- What happens when all configured SMSC providers are unavailable? The system should queue messages and retry when providers become available, with appropriate error reporting after maximum retry attempts.
- What happens when a message exceeds 160 characters? The system automatically splits it into multipart segments (concatenated SMS) and delivers them as one logical message. Each segment is tracked individually but presented to the client as a single message.
- What happens during a message broker outage? The system should gracefully handle the failure, return an appropriate error to the client, and not lose acknowledged messages.
- What happens when a client's balance reaches zero mid-batch? The system should process messages until the balance is depleted, then reject remaining messages with an insufficient-balance error.
- What happens when a webhook endpoint is permanently unreachable? After exhausting all retry attempts, the system should log the failure and make it visible through the admin interface.
- What happens when a scheduled message is cancelled at the exact moment the scheduler picks it up for sending? The system should use optimistic locking (check status before transition) to prevent race conditions — if the message was already cancelled, skip sending.

## Clarifications

### Session 2026-03-20

- Q: Что должно происходить, когда сообщение превышает 160 символов? → A: Автоматически разбивать на multipart-сегменты (concatenated SMS) и доставлять как одно логическое сообщение.
- Q: Что происходит, если отчёт о доставке (DLR) не получен? → A: Через настраиваемый таймаут (по умолчанию 24 часа) сообщение переводится в статус "expired".
- Q: Какой срок хранения данных о сообщениях? → A: 90 дней для данных сообщений, 1 год для логов аудита.
- Q: Как тарифицировать multipart-сообщения? → A: За каждый сегмент отдельно (сообщение из 3 сегментов = 3 SMS).
- Q: Какой максимальный срок планирования сообщения в будущее? → A: До 30 дней.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST accept SMS submissions via HTTP REST, gRPC, and SMPP v3.4 protocols.
- **FR-002**: System MUST authenticate all API clients using API keys before accepting any message submission.
- **FR-003**: System MUST authenticate all administrative operations using JWT tokens with role-based access control.
- **FR-004**: System MUST route messages to SMSC providers based on configurable routing rules (destination patterns, load balancing, explicit provider selection).
- **FR-005**: System MUST automatically failover to alternative providers when the primary provider is unavailable or returns errors.
- **FR-006**: System MUST track each message through its complete lifecycle (pending, queued, sent, delivered, failed, expired, scheduled, cancelled) and make status available to the submitting client.
- **FR-007**: System MUST process delivery receipts from SMSC providers and update message status accordingly.
- **FR-008**: System MUST enforce per-client rate limits at configurable thresholds (per second, per minute, per hour).
- **FR-009**: System MUST support batch message submission with individual tracking per message.
- **FR-010**: System MUST support scheduling messages for future delivery (up to 30 days ahead) with cancellation capability.
- **FR-011**: System MUST manage client account balances and charge per SMS segment based on destination-specific pricing rules (multipart messages are billed per segment).
- **FR-012**: System MUST reject message submissions when the client's balance is insufficient.
- **FR-013**: System MUST deliver webhook notifications to registered client endpoints for message status changes, signed with a cryptographic signature.
- **FR-014**: System MUST retry failed webhook deliveries with exponential backoff up to a configurable maximum number of attempts.
- **FR-015**: System MUST support pre-approved message templates with variable substitution and an admin moderation workflow (draft, approved, rejected).
- **FR-016**: System MUST provide administrative APIs for managing clients, providers, routing rules, templates, billing, and analytics.
- **FR-017**: System MUST aggregate message statistics and provide queryable analytics by client, provider, status, and time period.
- **FR-018**: System MUST maintain an audit log of all administrative operations and security-relevant events.
- **FR-019**: System MUST support horizontal scaling of gateway components behind a load balancer.
- **FR-020**: System MUST preserve message ordering within a single client session where applicable.
- **FR-021**: System MUST automatically split messages exceeding 160 characters into multipart (concatenated) SMS segments and deliver them as a single logical message to the recipient.
- **FR-022**: System MUST transition messages from "sent" to "expired" status if no delivery receipt is received within a configurable timeout (default: 24 hours).
- **FR-023**: System MUST automatically purge message data older than 90 days and audit log data older than 1 year.

### Key Entities

- **Message**: Represents a single SMS with sender, recipient, body, status, timestamps, and optional scheduling and template references. Central entity that flows through the entire system lifecycle.
- **Client**: An API consumer account with credentials (API key), rate limits, allowed source addresses, and an associated billing account. Owns all messages they submit.
- **Provider**: An SMSC connection configuration with host, credentials, throughput limits, and health status. Providers are the downstream delivery partners.
- **Route**: A routing rule that maps destination patterns to providers with priority, load balancing strategy, and failover configuration.
- **Template**: A pre-approved message pattern with placeholder variables, moderation status (draft/approved/rejected), and audit trail.
- **Webhook Subscription**: A client-registered HTTP endpoint for receiving delivery notifications, with signing secret and retry configuration.
- **Account**: A billing account linked to a client, tracking balance, transactions, and pricing rules.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Clients can submit a single SMS and receive a confirmation with message ID in under 1 second under normal load.
- **SC-002**: The platform supports at least 10,000 concurrent connected clients without service degradation.
- **SC-003**: 99.9% of messages that enter the system are either successfully delivered or return a definitive failure status (no messages lost or stuck indefinitely).
- **SC-004**: Scheduled messages are sent within 60 seconds of their designated delivery time.
- **SC-005**: Webhook notifications are delivered to reachable endpoints within 5 seconds of the triggering status change.
- **SC-006**: Clients can retrieve the current status of any submitted message within 500 milliseconds.
- **SC-007**: The system automatically fails over to an alternative provider within 30 seconds of detecting a provider outage.
- **SC-008**: Batch submissions of up to 10,000 messages are accepted and acknowledged within 10 seconds.
- **SC-009**: Administrative reports for up to 30 days of data are generated within 30 seconds.
- **SC-010**: The platform maintains 99.95% uptime measured monthly, excluding planned maintenance windows.
