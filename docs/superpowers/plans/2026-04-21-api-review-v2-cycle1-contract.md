# Cycle 1 (Contract) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development.

**Goal:** Ревью осей A1 (HTTP OpenAPI↔код), A1-grpc (gRPC proto↔реализация), A2 (единый формат ошибок), A3 (идемпотентность).

**Architecture:** Ветка `review/api-v2-cycle1-contract` от master. Один PR к master.

**Spec:** `docs/superpowers/specs/2026-04-21-api-review-v2-umbrella.md`.

## Tasks

1. **Scaffold** — matrix + findings + plan. Контроллер.
2. **A1** — OpenAPI ↔ HTTP router диф. Subagent.
3. **A1-grpc** — Unimplemented RPCs, dead code, .proto source gaps. Subagent.
4. **A2** — HTTP error envelope consistency, SMPP ESME map, gRPC status. Subagent.
5. **A3** — Idempotency-Key / dedup check на 4 транспортах. Subagent.
6. **Finalize + PR** — summary table, push. Контроллер.
