# Comprehensive Requirements Quality Checklist: Multi-tenant Self-Service Portal + Sub-accounts

**Purpose**: Thorough validation of requirement quality across Security, API, and Data & Compliance domains — pre-release gate
**Created**: 2026-03-21
**Feature**: [spec.md](../spec.md)

## Security Requirements Completeness

- [ ] CHK001 Are password complexity requirements (minimum length, character classes) specified? [Gap, Spec §FR-017]
- [ ] CHK002 Are session expiration and renewal requirements defined beyond the 24h max-age? [Clarity, Spec §FR-017]
- [ ] CHK003 Is the maximum number of concurrent sessions per user specified in requirements? [Gap, Clarifications §Session]
- [ ] CHK004 Are TOTP recovery code requirements fully specified — count, format, one-time-use behavior, regeneration flow? [Completeness, Spec §FR-017]
- [ ] CHK005 Is the TOTP secret storage encryption method specified in requirements (algorithm, key management)? [Gap, Spec §FR-017]
- [ ] CHK006 Are password reset token invalidation requirements defined — e.g., invalidate on successful use, on password change, or on new reset request? [Completeness, Spec §FR-018]
- [ ] CHK007 Is the brute-force lockout scope clearly specified — per email, per IP, or both? [Clarity, Spec §FR-019]
- [ ] CHK008 Are CSRF protection requirements documented for all state-mutating endpoints? [Gap]
- [ ] CHK009 Are session invalidation requirements defined for security-critical events (password change, 2FA enable/disable)? [Gap, Spec §FR-017]
- [ ] CHK010 Are cookie security attributes (HttpOnly, Secure, SameSite) specified as requirements? [Gap]
- [ ] CHK011 Is the IP whitelist validation behavior specified for edge cases — IPv6, malformed input, empty list semantics? [Edge Case, Spec §FR-006]

## Tenant Isolation & Multi-tenancy Requirements

- [ ] CHK012 Is "полная изоляция данных" quantified with specific enforcement points (query-level, API-level, UI-level)? [Measurability, Spec §FR-016]
- [ ] CHK013 Are cross-tenant access prevention requirements defined for all data entities (messages, keys, webhooks, audit, sub-accounts)? [Coverage, Spec §FR-016]
- [ ] CHK014 Are tenant isolation requirements specified for the reseller↔sub-account boundary — can a reseller impersonate a sub-account's session? [Gap, Spec §FR-020]
- [ ] CHK015 Is the sub-account's own data visibility defined — can a sub-account see it belongs to a reseller? Can it see the reseller's name? [Gap, Spec §US6]
- [ ] CHK016 Are requirements defined for what happens when a reseller's own account is deactivated while sub-accounts exist? [Edge Case, Spec §Edge Cases L117]

## API Contract Requirements

- [ ] CHK017 Are error response formats (structure, error codes) consistently specified across all endpoints? [Consistency, Contracts §portal-api.md]
- [ ] CHK018 Are input validation requirements defined for all request fields — max lengths, allowed characters, format constraints? [Gap]
- [ ] CHK019 Are rate limiting requirements specified per endpoint or globally — with specific thresholds beyond login? [Clarity, Spec §Edge Cases L113]
- [ ] CHK020 Is the pagination behavior specified for boundary cases — page 0, negative page, per_page exceeding max? [Edge Case, Spec §FR-022]
- [ ] CHK021 Are webhook URL validation requirements defined — HTTPS only? Localhost rejection? Internal IP blocking? [Gap, Spec §FR-009]
- [ ] CHK022 Are API versioning requirements documented for the portal HTTP API? [Gap]
- [ ] CHK023 Is the "real-time" balance view requirement quantified — polling interval, websocket, or on-demand refresh? [Ambiguity, Spec §FR-001]

## Data Model & Financial Integrity Requirements

- [ ] CHK024 Are balance transfer atomicity requirements explicitly documented — what constitutes a failed transfer, rollback behavior? [Completeness, Spec §FR-013]
- [ ] CHK025 Is the balance precision requirement specified — decimal places, rounding rules? [Gap, Spec §FR-013]
- [ ] CHK026 Are currency requirements defined — single currency per account, multi-currency support, conversion rules? [Gap, Spec §Key Entities]
- [ ] CHK027 Is the minimum transfer amount defined? Can a reseller transfer 0.01? [Edge Case, Spec §FR-013]
- [ ] CHK028 Are concurrent balance operation requirements defined — two simultaneous transfers from the same reseller? [Edge Case, Spec §FR-013]
- [ ] CHK029 Is the sub-account deletion and balance return process fully specified — timing, audit trail, reversibility? [Completeness, Spec §FR-021]
- [ ] CHK030 Are requirements defined for negative balance scenarios — can a balance go negative due to in-flight messages? [Edge Case, Gap]

## Audit & Compliance Requirements

- [ ] CHK031 Is the exhaustive list of auditable actions defined, or is "все критичные действия" open to interpretation? [Ambiguity, Spec §FR-023]
- [ ] CHK032 Are audit log immutability requirements specified — can entries be modified or deleted by any role? [Gap, Spec §Key Entities]
- [ ] CHK033 Is the audit log retention period explicitly stated in requirements (not just assumptions)? [Gap, Spec §Assumptions L186]
- [ ] CHK034 Are requirements defined for who can view audit logs — only the tenant, or also admin? Sub-account's own audit log? [Gap, Spec §FR-024]
- [ ] CHK035 Is the audit log data completeness defined — what "details" field must contain for each action type? [Clarity, Spec §FR-023]
- [ ] CHK036 Are data export requirements specified for audit logs — format, filtering, size limits? [Gap, Spec §FR-024]
- [ ] CHK037 Are data retention auto-purge requirements specified with a mechanism (cron, partition drop, policy)? [Gap, Spec §Assumptions L186]

## Acceptance Criteria Quality

- [ ] CHK038 Is SC-001 ("90% рутинных операций самостоятельно") measurable — what baseline, what measurement method? [Measurability, Spec §SC-001]
- [ ] CHK039 Is SC-002 ("80% снижение обращений") measurable — current baseline documented? [Measurability, Spec §SC-002]
- [ ] CHK040 Is SC-005 ("< 3 секунды") specified with load conditions — empty dataset vs. 1M messages? [Clarity, Spec §SC-005]
- [ ] CHK041 Is SC-006 ("500 пользователей") defined with specific scenario — concurrent logins, concurrent page views, concurrent API calls? [Clarity, Spec §SC-006]
- [ ] CHK042 Is SC-008 ("10% реселлеров") actionable as a technical requirement or purely a business metric? [Measurability, Spec §SC-008]

## Scenario Coverage

- [ ] CHK043 Are requirements defined for sub-account user experience — what navigation/features differ from a regular tenant? [Gap, Spec §US6]
- [ ] CHK044 Are requirements defined for the reseller upgrading a sub-account's limits in real-time — does the sub-account see changes immediately? [Gap, Spec §FR-014]
- [ ] CHK045 Are error message requirements specified — user-facing language, level of detail, localization? [Gap]
- [ ] CHK046 Are requirements specified for handling webhook test endpoint failures — timeout, retry, error display? [Gap, Spec §FR-010]
- [ ] CHK047 Is the analytics "group by country" requirement defined with a source — how is destination country determined (prefix parsing, external lookup)? [Clarity, Spec §US4 L71]
- [ ] CHK048 Are i18n requirements actionable — which components need localization, what languages beyond RU/EN? [Clarity, Spec §Assumptions L188]

## Dependencies & Assumptions

- [ ] CHK049 Is the assumption "клиенты уже зарегистрированы через администратора" validated against the sub-account flow — who creates sub-account user credentials? [Assumption, Spec §Assumptions L183]
- [ ] CHK050 Is the email sending dependency documented — password reset and webhook deactivation notifications require SMTP infrastructure not specified in requirements? [Dependency, Gap]
- [ ] CHK051 Are the existing service dependencies (analytics, messaging, billing gRPC) validated — do they expose the RPCs the portal handlers call? [Dependency, Assumption]
- [ ] CHK052 Is the "90 дней для оперативного доступа" assumption promoted to a requirement with defined enforcement mechanism? [Assumption, Spec §Assumptions L186]

## Notes

- Check items off as completed: `[x]`
- Items reference spec sections as `[Spec §section]` for traceability
- `[Gap]` marks requirements that may be missing entirely
- `[Ambiguity]` marks terms needing quantification
- `[Edge Case]` marks boundary conditions needing specification
- `[Assumption]` marks assumptions needing validation or promotion to requirements
