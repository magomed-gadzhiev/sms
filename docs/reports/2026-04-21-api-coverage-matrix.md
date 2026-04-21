# API Coverage Matrix — 2026-04-21

**Snapshot date:** 2026-04-21
**Spec:** `docs/superpowers/specs/2026-04-21-api-client-subaccount-review-design.md`
**Phase completed:** 0 (skeleton only — все строки в UNKNOWN)
**OK rows with test_file (ratchet baseline):** 0
**Total rows:** 318

## Legend

- `current_state`: `OK` / `DRIFT` / `MISSING` / `BROKEN` / `UNKNOWN`
- `severity`: `critical` (биллинг/auth) / `major` (контракт) / `minor` (косметика)
- `action`: `fixed-in-PR#<N>` / `plan:<path>` / `test-added:<path>` / `wontfix:<reason>`

## Matrix

| endpoint_or_pdu | transport | role | axis | current_state | test_file | gap | severity | action |
|---|---|---|---|---|---|---|---|---|
| POST /api/v1/sms/send | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/send | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/send | HTTP | client | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/send | HTTP | client | C-billing | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/send | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/send | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/send | HTTP | subaccount_child | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/send | HTTP | subaccount_child | C-billing | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/send | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/send | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/send | HTTP | subaccount_parent | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/send | HTTP | subaccount_parent | C-billing | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/batch | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/batch | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/batch | HTTP | client | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/batch | HTTP | client | C-billing | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/batch | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/batch | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/batch | HTTP | subaccount_child | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/batch | HTTP | subaccount_child | C-billing | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/batch | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/batch | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/batch | HTTP | subaccount_parent | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/sms/batch | HTTP | subaccount_parent | C-billing | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/status/{id} | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/status/{id} | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/status/{id} | HTTP | client | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/status/{id} | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/status/{id} | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/status/{id} | HTTP | subaccount_child | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/status/{id} | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/status/{id} | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/status/{id} | HTTP | subaccount_parent | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/history | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/history | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/history | HTTP | client | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/history | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/history | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/history | HTTP | subaccount_child | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/history | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/history | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/history | HTTP | subaccount_parent | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/scheduled | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/scheduled | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/scheduled | HTTP | client | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/scheduled | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/scheduled | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/scheduled | HTTP | subaccount_child | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/scheduled | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/scheduled | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/sms/scheduled | HTTP | subaccount_parent | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| DELETE /api/v1/sms/{id} | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| DELETE /api/v1/sms/{id} | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| DELETE /api/v1/sms/{id} | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| DELETE /api/v1/sms/{id} | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| DELETE /api/v1/sms/{id} | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| DELETE /api/v1/sms/{id} | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/account/balance | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/account/balance | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/account/balance | HTTP | client | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/account/balance | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/account/balance | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/account/balance | HTTP | subaccount_child | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/account/balance | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/account/balance | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/account/balance | HTTP | subaccount_parent | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/account/stats | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/account/stats | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/account/stats | HTTP | client | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/account/stats | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/account/stats | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/account/stats | HTTP | subaccount_child | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/account/stats | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/account/stats | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/account/stats | HTTP | subaccount_parent | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/webhooks | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/webhooks | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/webhooks | HTTP | client | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/webhooks | HTTP | client | C-dlr-routing | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/webhooks | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/webhooks | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/webhooks | HTTP | subaccount_child | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/webhooks | HTTP | subaccount_child | C-dlr-routing | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/webhooks | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/webhooks | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/webhooks | HTTP | subaccount_parent | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/webhooks | HTTP | subaccount_parent | C-dlr-routing | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/webhooks | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/webhooks | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/webhooks | HTTP | client | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/webhooks | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/webhooks | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/webhooks | HTTP | subaccount_child | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/webhooks | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/webhooks | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/webhooks | HTTP | subaccount_parent | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/webhooks/{id} | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/webhooks/{id} | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/webhooks/{id} | HTTP | client | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/webhooks/{id} | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/webhooks/{id} | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/webhooks/{id} | HTTP | subaccount_child | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/webhooks/{id} | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/webhooks/{id} | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/webhooks/{id} | HTTP | subaccount_parent | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| PUT /api/v1/webhooks/{id} | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| PUT /api/v1/webhooks/{id} | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| PUT /api/v1/webhooks/{id} | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| PUT /api/v1/webhooks/{id} | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| PUT /api/v1/webhooks/{id} | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| PUT /api/v1/webhooks/{id} | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| DELETE /api/v1/webhooks/{id} | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| DELETE /api/v1/webhooks/{id} | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| DELETE /api/v1/webhooks/{id} | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| DELETE /api/v1/webhooks/{id} | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| DELETE /api/v1/webhooks/{id} | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| DELETE /api/v1/webhooks/{id} | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup | HTTP | client | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup | HTTP | client | C-billing | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup | HTTP | subaccount_child | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup | HTTP | subaccount_child | C-billing | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup | HTTP | subaccount_parent | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup | HTTP | subaccount_parent | C-billing | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup/bulk | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup/bulk | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup/bulk | HTTP | client | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup/bulk | HTTP | client | C-billing | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup/bulk | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup/bulk | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup/bulk | HTTP | subaccount_child | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup/bulk | HTTP | subaccount_child | C-billing | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup/bulk | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup/bulk | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup/bulk | HTTP | subaccount_parent | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/lookup/bulk | HTTP | subaccount_parent | C-billing | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/lookup/history | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/lookup/history | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/lookup/history | HTTP | client | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/lookup/history | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/lookup/history | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/lookup/history | HTTP | subaccount_child | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/lookup/history | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/lookup/history | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/lookup/history | HTTP | subaccount_parent | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/templates | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/templates | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/templates | HTTP | client | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/templates | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/templates | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/templates | HTTP | subaccount_child | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/templates | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/templates | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/templates | HTTP | subaccount_parent | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates | HTTP | client | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates | HTTP | subaccount_child | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates | HTTP | subaccount_parent | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates/{id} | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates/{id} | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates/{id} | HTTP | client | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates/{id} | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates/{id} | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates/{id} | HTTP | subaccount_child | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates/{id} | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates/{id} | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates/{id} | HTTP | subaccount_parent | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| PUT /api/v1/templates/{id} | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| PUT /api/v1/templates/{id} | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| PUT /api/v1/templates/{id} | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| PUT /api/v1/templates/{id} | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| PUT /api/v1/templates/{id} | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| PUT /api/v1/templates/{id} | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| DELETE /api/v1/templates/{id} | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| DELETE /api/v1/templates/{id} | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| DELETE /api/v1/templates/{id} | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| DELETE /api/v1/templates/{id} | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| DELETE /api/v1/templates/{id} | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| DELETE /api/v1/templates/{id} | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates/{id}/audit | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates/{id}/audit | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates/{id}/audit | HTTP | client | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates/{id}/audit | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates/{id}/audit | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates/{id}/audit | HTTP | subaccount_child | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates/{id}/audit | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates/{id}/audit | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/templates/{id}/audit | HTTP | subaccount_parent | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/cascade/deliveries | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/cascade/deliveries | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/cascade/deliveries | HTTP | client | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/cascade/deliveries | HTTP | client | C-billing | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/cascade/deliveries | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/cascade/deliveries | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/cascade/deliveries | HTTP | subaccount_child | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/cascade/deliveries | HTTP | subaccount_child | C-billing | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/cascade/deliveries | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/cascade/deliveries | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/cascade/deliveries | HTTP | subaccount_parent | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| POST /api/v1/cascade/deliveries | HTTP | subaccount_parent | C-billing | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/deliveries | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/deliveries | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/deliveries | HTTP | client | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/deliveries | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/deliveries | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/deliveries | HTTP | subaccount_child | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/deliveries | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/deliveries | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/deliveries | HTTP | subaccount_parent | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/deliveries/{id} | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/deliveries/{id} | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/deliveries/{id} | HTTP | client | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/deliveries/{id} | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/deliveries/{id} | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/deliveries/{id} | HTTP | subaccount_child | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/deliveries/{id} | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/deliveries/{id} | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/deliveries/{id} | HTTP | subaccount_parent | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/stats | HTTP | client | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/stats | HTTP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/stats | HTTP | client | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/stats | HTTP | subaccount_child | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/stats | HTTP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/stats | HTTP | subaccount_child | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/stats | HTTP | subaccount_parent | A1 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/stats | HTTP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| GET /api/v1/cascade/stats | HTTP | subaccount_parent | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| bind_receiver | SMPP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| bind_receiver | SMPP | client | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| bind_receiver | SMPP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| bind_receiver | SMPP | subaccount_child | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| bind_receiver | SMPP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| bind_receiver | SMPP | subaccount_parent | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| bind_transmitter | SMPP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| bind_transmitter | SMPP | client | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| bind_transmitter | SMPP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| bind_transmitter | SMPP | subaccount_child | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| bind_transmitter | SMPP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| bind_transmitter | SMPP | subaccount_parent | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| bind_transceiver | SMPP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| bind_transceiver | SMPP | client | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| bind_transceiver | SMPP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| bind_transceiver | SMPP | subaccount_child | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| bind_transceiver | SMPP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| bind_transceiver | SMPP | subaccount_parent | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| unbind | SMPP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| unbind | SMPP | client | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| unbind | SMPP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| unbind | SMPP | subaccount_child | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| unbind | SMPP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| unbind | SMPP | subaccount_parent | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| submit_sm | SMPP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| submit_sm | SMPP | client | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| submit_sm | SMPP | client | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| submit_sm | SMPP | client | C-billing | UNKNOWN |  | phase 0 skeleton |  |  |
| submit_sm | SMPP | client | C-dlr-routing | UNKNOWN |  | phase 0 skeleton |  |  |
| submit_sm | SMPP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| submit_sm | SMPP | subaccount_child | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| submit_sm | SMPP | subaccount_child | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| submit_sm | SMPP | subaccount_child | C-billing | UNKNOWN |  | phase 0 skeleton |  |  |
| submit_sm | SMPP | subaccount_child | C-dlr-routing | UNKNOWN |  | phase 0 skeleton |  |  |
| submit_sm | SMPP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| submit_sm | SMPP | subaccount_parent | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| submit_sm | SMPP | subaccount_parent | A3 | UNKNOWN |  | phase 0 skeleton |  |  |
| submit_sm | SMPP | subaccount_parent | C-billing | UNKNOWN |  | phase 0 skeleton |  |  |
| submit_sm | SMPP | subaccount_parent | C-dlr-routing | UNKNOWN |  | phase 0 skeleton |  |  |
| deliver_sm (MO) | SMPP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| deliver_sm (MO) | SMPP | client | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| deliver_sm (MO) | SMPP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| deliver_sm (MO) | SMPP | subaccount_child | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| deliver_sm (MO) | SMPP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| deliver_sm (MO) | SMPP | subaccount_parent | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| deliver_sm (DLR) | SMPP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| deliver_sm (DLR) | SMPP | client | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| deliver_sm (DLR) | SMPP | client | C-dlr-routing | UNKNOWN |  | phase 0 skeleton |  |  |
| deliver_sm (DLR) | SMPP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| deliver_sm (DLR) | SMPP | subaccount_child | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| deliver_sm (DLR) | SMPP | subaccount_child | C-dlr-routing | UNKNOWN |  | phase 0 skeleton |  |  |
| deliver_sm (DLR) | SMPP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| deliver_sm (DLR) | SMPP | subaccount_parent | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| deliver_sm (DLR) | SMPP | subaccount_parent | C-dlr-routing | UNKNOWN |  | phase 0 skeleton |  |  |
| enquire_link | SMPP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| enquire_link | SMPP | client | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| enquire_link | SMPP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| enquire_link | SMPP | subaccount_child | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| enquire_link | SMPP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| enquire_link | SMPP | subaccount_parent | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| query_sm | SMPP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| query_sm | SMPP | client | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| query_sm | SMPP | client | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| query_sm | SMPP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| query_sm | SMPP | subaccount_child | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| query_sm | SMPP | subaccount_child | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| query_sm | SMPP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| query_sm | SMPP | subaccount_parent | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| query_sm | SMPP | subaccount_parent | C-visibility | UNKNOWN |  | phase 0 skeleton |  |  |
| cancel_sm | SMPP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| cancel_sm | SMPP | client | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| cancel_sm | SMPP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| cancel_sm | SMPP | subaccount_child | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| cancel_sm | SMPP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| cancel_sm | SMPP | subaccount_parent | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| replace_sm | SMPP | client | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| replace_sm | SMPP | client | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| replace_sm | SMPP | subaccount_child | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| replace_sm | SMPP | subaccount_child | A6 | UNKNOWN |  | phase 0 skeleton |  |  |
| replace_sm | SMPP | subaccount_parent | A2 | UNKNOWN |  | phase 0 skeleton |  |  |
| replace_sm | SMPP | subaccount_parent | A6 | UNKNOWN |  | phase 0 skeleton |  |  |

## Machine-readable copy

См. `2026-04-21-api-coverage-matrix.csv` — тот же контент, одно сочетание на строку.
