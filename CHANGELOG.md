# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- DLR receipt lookups (`GetByMessageID`, `GetBySMPPMessageID`) no longer fail with a sqlx scan error: `shared.DLRReceipt` lacked the `message_created_at` column that `dlr_receipts` carries for partition-aligned foreign keys.

## [0.1.0] - 2026-09-23

Initial public release.

### Added

- SMPP gateway for client connections: bind/session handling, submit and delivery flows.
- HTTP and gRPC gateways for client, admin, and portal APIs; OpenAPI and protobuf definitions under `api/`.
- Routing Resolution engine with managed routes and provider management.
- Billing built on segments, tariffs, and idempotent charges (`tarification_log`, transactions).
- Campaign tooling: templates, contacts, webhooks, and campaigns running through a materializer → Kafka pipeline.
- Delivery reports (DLR) with per-status handling end to end.
- React portal frontend for operator, client, and reseller workflows.
- Simulator provider for local end-to-end testing without a real SMPP peer.
- Docker Compose deployment stack (local and production templates) and PostgreSQL migrations under `migrations/`.
- Functional, integration, e2e (Playwright), and load test suites with demo fixtures.
- Domain documentation: ubiquitous language in `CONTEXT.md`, architecture decision records in `docs/adr/`.

### Changed

- Consolidated the service architecture ahead of the release: retired the parallel worker pipeline, unified routing into a single Routing Resolution engine, introduced the Message Status vocabulary module, moved Charge idempotency behind the Charger port, and made make targets own test-launch provisioning.

[Unreleased]: https://github.com/magomed-gadzhiev/sms/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/magomed-gadzhiev/sms/releases/tag/v0.1.0
