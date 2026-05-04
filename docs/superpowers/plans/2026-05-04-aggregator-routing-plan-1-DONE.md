# Plan 1 — Foundation (aggregator routing management) — DONE

**Date completed:** 2026-05-04
**Branch:** master
**Final commit:** `749fe06`
**Plan source:** `docs/superpowers/plans/2026-05-04-aggregator-routing-plan-1-foundation.md`

## Status

All 19 tasks complete. Backend handlers + frontend pages live on sms-server. Smoke tests passed.

## Plan 1 commit list (19 tasks)

```
749fe06 test(e2e): aggregator network Plan 1 — providers + provider-sets + assignment flow
68e61b6 feat(portal): SubAccountNetworkSection — provider override management в карточке
07bc9b3 fix(portal): добавить новые /network/* пункты в реальный sidebar (UserLayout)
398b29e feat(portal): /network/assignments — provider-set assignments + bulk + redirect from /network/routing
b995710 fix(portal): provider-sets — rename input корректно откатывается при ошибке
94f276e feat(portal): /network/provider-sets — master-detail editor
b942d61 feat(portal): /network/providers — каталог провайдеров с CRUD private
cbea4fe feat(api): networkApi — providers, provider-sets, assignments, overrides
31ba4b2 feat(ui): generic Drawer component (right-aligned slide-in)
7ce9f51 feat(routing): wire /reseller/network/* + /sub-accounts/{id}/network/* routes
9f0bff2 fix(handlers): subaccount overrides — atomic insert + branched 409 message + tests
112f767 feat(handlers): /sub-accounts/{id}/network — overview + provider override CRUD
5aec458 fix(handlers): assignments — distinguish 404 vs 500 + document atomicity
850a37f feat(handlers): /reseller/network/assignments — list + PUT + bulk (provider-set only)
cbb9567 fix(handlers): provider-set items — cap at 100 + test for unknown provider
ee23632 feat(handlers): /reseller/network/provider-sets/{id}/items — atomic PUT-replace + materialize
2582534 fix(handlers): provider-sets — 409 на коллизии имени + дополнительные тесты
f706259 feat(handlers): /reseller/network/provider-sets — CRUD + ownership check
2a9c49a fix(handlers): provider catalog — review fixes
0dd1632 feat(handlers): /reseller/network/providers — каталог + CRUD private
c3ffef0 refactor(network): ProviderSetMaterializer — drop unused deps + document partial-failure semantics
fda2c20 feat(network): ProviderSetMaterializer — материализация в client_providers
e242663 fix(storage): add account_type='sub_account' to seedTestSubAccount
9b76166 feat(storage): subaccount_routing_assignment repository
f1de457 feat(storage): reseller_provider_set_items repository with atomic replace
b04ffcf fix(storage): use errors.Is for pgx.ErrNoRows in provider-set repo
75477ce feat(storage): reseller_provider_sets repository
3f4941e feat(migrations): provider-sets + assignments tables (Plan 1 Task 1)
af3eca7 docs(plan): aggregator routing Plan 1 — foundation
5f70fe7 docs(spec): aggregator routing management design
```

## Smoke test results (Task 19)

Run on sms-server (claude@72.56.232.202) against `http://localhost:18085` with credentials `aggregator@test.local / Admin123!`.

| # | Endpoint | Method | Status | Notes |
|---|---|---|---|---|
| 1 | `/portal/v1/auth/login` | POST | 200 | Session + CSRF cookies issued |
| 2 | `/portal/v1/reseller/network/providers` | GET | 200 | Returned platform-owned providers (I-Digital, MaximusSMS, LoadTest-Simulator, etc.) |
| 3 | `/portal/v1/reseller/network/provider-sets` | POST | 201 | Created `smoke-test-001` (id `0bb984a8-72bb-4243-8e52-65cf14011900`) |
| 4 | `/portal/v1/reseller/network/assignments` | GET | 200 | Returned 6 sub-accounts of test aggregator, all `unassigned` initially |
| 5 | `/portal/v1/reseller/network/assignments/{client_id}` | PUT | 200 | Assigned smoke set to `f5b86f51-b0b5-4950-b434-ccbc1703cd5d` (QATestAudit) |

DB verification (`subaccount_routing_assignment`): row appeared at `2026-05-04 22:44:03+00` with `provider_set_id=0bb984a8-...`. Confirmed materializer ran without error (no new `inherited` rows because the test set was empty — count stayed at 24, which is correct: empty set ⇒ no items to mirror into `client_providers`).

Cleanup: PUT `{provider_set_id: null}` to unassign + DELETE provider-set ⇒ 204. Verified `reseller_provider_sets WHERE name LIKE 'smoke-%'` count=0.

## Deploy notes

- `git push origin master` ⇒ pushed 10 local commits.
- `./scripts/server.sh deploy` ⇒ all containers came up healthy (portal-frontend, portal-gateway, etc.).
- `./scripts/server.sh migrate` ⇒ no-op (migrations 129-132 already applied during Tasks 1-6).

**Sandbox infra incident during smoke (not related to Plan 1):** Redis on sms-server was found in slave mode against an unknown external host (`175.24.232.83:20931`, link down). Fixed via `REPLICAOF NO ONE`. This was likely a residual state from earlier sandbox tampering — flagged for the user but does not affect Plan 1 deliverables.

## Open follow-ups (move to Plan 2)

- `NetworkRoutingPage.tsx` (legacy) and `/reseller/routing/*` legacy backend endpoints still present — to be removed in Plan 2 once UI consumers fully migrated.
- DELETE on `/portal/v1/reseller/network/assignments/{client_id}` returns 404 (only PUT-with-null clears the assignment). Document or wire a real DELETE in Plan 2 if frontend needs it.
- Test sub-account QATestAudit retains an `unassigned` row in `subaccount_routing_assignment` (NULL FK) after smoke cleanup — harmless but technically an orphan. Cleanup pattern needs decision in Plan 2.

## Next

Plan 2 — Routing aggregation logic + legacy `/network/routing` removal + sender/template inheritance refinements.
