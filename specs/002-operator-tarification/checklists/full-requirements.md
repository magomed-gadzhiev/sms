# Full Requirements Quality Checklist: Operator-Based SMS Tarification System

**Purpose**: Полная проверка качества требований — полнота, ясность, консистентность, покрытие сценариев и граничных случаев
**Created**: 2026-03-20
**Feature**: [spec.md](../spec.md)
**Depth**: Standard
**Audience**: Self-review before merge

## Requirement Completeness

- [ ] CHK001 - Are requirements defined for all 4 tarification strategies (fixed, threshold, threshold_recalc, prepaid_threshold) with distinct behavioral descriptions? [Completeness, Spec §FR-011]
- [ ] CHK002 - Are the exact pricing calculation formulas specified for each strategy, including split-segment billing at threshold boundaries? [Completeness, Spec §FR-016..FR-019]
- [ ] CHK003 - Is the behavior specified when a client sends a message but has no SenderRegistration and no tariff plan for "shared" category? [Completeness, Gap]
- [ ] CHK004 - Are requirements for the PrepaidFee scheduler defined — trigger frequency, retry policy, notification mechanism? [Completeness, Spec §FR-020]
- [ ] CHK005 - Is the full lifecycle of SenderRegistration documented (pending → active → expired), including transition triggers and who can perform them? [Completeness, Spec §FR-006]
- [ ] CHK006 - Are requirements for operator deletion/deactivation specified — cascade behavior on prefixes, impact on existing tariff plans and sender registrations? [Completeness, Gap]
- [ ] CHK007 - Is the behavior specified when routing-service cannot determine country (phone_code not matched) but prefix partially matches? [Completeness, Spec §FR-004]
- [ ] CHK008 - Are requirements specified for what happens when a tariff period expires mid-day — are messages sent before midnight tarified under the old period or rejected? [Completeness, Gap]
- [ ] CHK009 - Is the debt tracking mechanism fully specified — data model, threshold for blocking, admin visibility, and debt clearance process? [Completeness, Spec §FR-026, FR-030]

## Requirement Clarity

- [ ] CHK010 - Is "existing SLA" in SC-003 quantified with specific latency thresholds for tarification? [Clarity, Spec §SC-003]
- [ ] CHK011 - Is "exponential backoff" in US3 acceptance scenario 3 quantified — initial delay, max delay, max retries? [Clarity, Spec US3]
- [ ] CHK012 - Is "одна рабочая сессия" in SC-006 defined with a time bound or number of steps? [Clarity, Spec §SC-006]
- [ ] CHK013 - Are the terms "тарифный период" (TariffPeriod) and "ценовой период" (PricingPeriod) clearly differentiated in the spec, with their relationship explicitly defined? [Clarity, Spec §FR-012..FR-015]
- [ ] CHK014 - Is "активный период" unambiguously defined — does it mean start_date <= today <= end_date, or start_date <= today regardless of end_date? [Clarity, Spec §FR-028, FR-029]

## Requirement Consistency

- [ ] CHK015 - Are the 3 sender categories (shared, paid_registered, free_registered) used consistently between spec entities, proto definitions, and task descriptions? [Consistency]
- [ ] CHK016 - Is the tarification flow description in US2 consistent with the Saga pattern described in the brainstorming session (sync gRPC charge, async recalc)? [Consistency, Spec §US2 vs §FR-022..FR-024]
- [ ] CHK017 - Is FR-007 (operator type validation for sender registration) consistent with the fact that tarification-service doesn't own operator data? [Consistency, Spec §FR-007]
- [ ] CHK018 - Are currency handling requirements consistent — FR-027 says currency from country, but is there a requirement for what happens when client's account currency differs from operator's country currency? [Consistency, Spec §FR-027]
- [ ] CHK019 - Is the port assignment for tarification-service consistent between tasks.md (9098/2119) and the actual docker-compose configuration (9100/2121)? [Consistency, tasks.md T008]

## Acceptance Criteria Quality

- [ ] CHK020 - Are acceptance scenarios for US1 testable without depending on US2 (tarification flow)? [Acceptance Criteria, Spec §US1]
- [ ] CHK021 - Does US3 acceptance scenario 1 correctly specify the expected math — is (1×10₽ + 2×8₽ = 26₽) accurate for a 3-segment message crossing from tier 0-100 to 101+? [Acceptance Criteria, Spec §US3]
- [ ] CHK022 - Are measurable success criteria defined for recalculation correctness (SC-002) — how is "корректная корректировка" verified beyond the single example? [Acceptance Criteria, Spec §SC-002]
- [ ] CHK023 - Does SC-004 (concurrent safety) have a defined test approach — specific concurrency level, expected outcome? [Acceptance Criteria, Spec §SC-004]
- [ ] CHK024 - Is SC-007 (Saga recovery) measurable — what constitutes "автоматически" and what is the acceptable recovery time? [Acceptance Criteria, Spec §SC-007]

## Scenario Coverage

- [ ] CHK025 - Are requirements defined for the scenario where a client has a registered sender name at one operator but sends to a different operator's number? [Coverage, Gap]
- [ ] CHK026 - Are requirements defined for bulk message sending — does tarification handle batch requests or only individual messages? [Coverage, Gap]
- [ ] CHK027 - Is the behavior specified when two tariff periods exist for the same plan with a gap between them (no active period on some dates)? [Coverage, Spec §FR-012]
- [ ] CHK028 - Are requirements defined for concurrent admin operations — two admins modifying the same tariff plan simultaneously? [Coverage, Gap]
- [ ] CHK029 - Is the MNP lookup integration defined as a future requirement with clear scope boundaries and fallback behavior? [Coverage, Spec §FR-004]

## Edge Case Coverage

- [ ] CHK030 - Is the behavior specified when segment_count = 0 (empty message or metadata-only)? [Edge Case, Gap]
- [ ] CHK031 - Are requirements defined for extremely large segment counts (e.g., 10-segment message crossing multiple tier boundaries)? [Edge Case, Spec §FR-017]
- [ ] CHK032 - Is the behavior specified when all tiers have price = 0 (free messaging tier)? [Edge Case, Gap]
- [ ] CHK033 - Are requirements defined for tariff period with start_date = end_date (single day period)? [Edge Case, Spec §FR-012]
- [ ] CHK034 - Is the behavior specified when a tariff plan has periods defined but zero tiers in the active period? [Edge Case, Gap]

## Non-Functional Requirements

- [ ] CHK035 - Are performance requirements for tarification latency specified with measurable targets (e.g., p95 < Xms)? [Non-Functional, Gap]
- [ ] CHK036 - Are observability requirements specified for tarification — which business metrics are mandatory beyond those listed in plan.md? [Non-Functional, Spec §Constitution IV]
- [ ] CHK037 - Is the request_id tracing requirement from Constitution Principle IV addressed for tarification-service gRPC handlers? [Non-Functional, Constitution §IV]
- [ ] CHK038 - Are data retention requirements for usage_counters justified — "бессрочно" means growing indefinitely, are storage growth projections considered? [Non-Functional, Spec §FR-031]
- [ ] CHK039 - Are security requirements specified for admin endpoints — who can create/modify tariff plans, operator restrictions? [Non-Functional, Gap]

## Dependencies & Assumptions

- [ ] CHK040 - Is the assumption that billing-service API remains unchanged documented and validated? [Assumption]
- [ ] CHK041 - Is the dependency on routing-service enriching messages with operator_id explicitly documented as a prerequisite for tarification? [Dependency, Spec §FR-005]
- [ ] CHK042 - Is the assumption that PostgreSQL exclusion constraints (btree_gist) are available documented? [Assumption, data-model.md]
- [ ] CHK043 - Is the cross-service dependency for FR-007 (operator type validation) documented — tarification-service needs to query routing-service? [Dependency, Spec §FR-007]

## Notes

- Check items off as completed: `[x]`
- Items reference spec sections as `[Spec §FR-NNN]` or `[Spec §US-N]`
- `[Gap]` marks requirements that are missing entirely
- `[Ambiguity]` marks requirements that exist but lack clarity
- This checklist validates requirements quality, NOT implementation correctness
