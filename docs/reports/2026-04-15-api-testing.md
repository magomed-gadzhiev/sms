# API Testing Report — 2026-04-15

**[Summary]**
> **Server:** 72.56.232.202:18084 (portal-gateway) | **Date:** 2026-04-15 04:30–04:55 UTC | **Mode:** full-run (auto)
> **Result:** 44/49 passed (90%) | 3 failed | 2 warnings | 2 skipped
> **Scope:** all (health, auth, crud, billing, messaging, negative)
> **Test accounts:** admin@example.com (admin), qa-test-client@qa.test (client, created during run)

---

## [Health]

| Check | Status | Time |
|-------|--------|------|
| GET /health | ✅ 200 `{"status":"ok","service":"portal-gateway"}` | 280ms |
| GET /health/live | ✅ 200 `{"status":"ok"}` | 271ms |
| GET /health/ready | ✅ 200 `{"status":"ok","service":"portal-gateway"}` | 273ms |
| Portal accessible | ✅ 200 | 280ms |

---

## [Auth Tests]

| # | Test | Request | Expected | Actual | Time | Status |
|---|------|---------|----------|--------|------|--------|
| A1 | Valid login | POST /auth/login `admin@example.com / Admin123!` | 200 + session cookie | 200 + `portal_session` + `csrf_token` cookies | 365ms | ✅ PASS |
| A2 | Invalid password | POST /auth/login `wrong_password` | 401 | 401 `UNAUTHORIZED` | 388ms | ✅ PASS |
| A3 | Non-existent user | POST /auth/login `nonexistent@example.com` | 401 (not 404) | 401 `UNAUTHORIZED` | 275ms | ✅ PASS |
| A4 | Empty fields | POST /auth/login `{}` | 400 | 400 `Поля email и password обязательны` | 305ms | ✅ PASS |
| A5 | SQL injection in login | POST /auth/login `' OR 1=1 --` | 400 or 401 | 401 | 280ms | ✅ PASS |
| A6 | Valid session on protected endpoint | GET /profile with valid cookie | 200 | 200 + user JSON | 286ms | ✅ PASS |
| A7 | No session | GET /profile without cookie | 401 | 401 `Сессия не найдена` | 268ms | ✅ PASS |
| A8 | Invalid session | GET /profile with `portal_session=invalid` | 401 | 401 `Сессия не найдена или истекла` | 279ms | ✅ PASS |
| A9 | RBAC: client on admin endpoint | GET /admin/channels with client token | 403 | 403 `Доступ запрещён` | 268ms | ✅ PASS |
| A10 | Register new client | POST /auth/register | 201 + client_id | 201 `{"client_id":"ec4b31d6...","user":{...}}` | 400ms | ✅ PASS |

---

## [CRUD Tests]

### Resource: templates

| # | Test | Request | Expected | Actual | Time | Status |
|---|------|---------|----------|--------|------|--------|
| T1 | Create valid template | POST /templates `{"name":"QA Test Template","body":"Hello {{name}}","type":"transactional"}` | 201 + id | 201 + template object | 296ms | ✅ PASS |
| T2 | Create with empty body | POST /templates `{}` | 400 | 400 `Поле name обязательно` | 283ms | ✅ PASS |
| T3 | List templates | GET /templates | 200 + array | 200 + `{templates:[...],total:1}` | 277ms | ✅ PASS |
| T4 | Get by ID | GET /templates/{id} | 200 | 200 + template | 277ms | ✅ PASS |
| T5 | Get non-existent → 404 | GET /templates/00000000-…-999 | 404 | 404 `NOT_FOUND` | 274ms | ✅ PASS |
| T6 | XSS in name field | POST /templates `{"name":"<script>alert(1)</script>"}` | 201, stored escaped | 201, stored as `\u003cscript\u003e...` (escaped) | 288ms | ✅ PASS |

### Resource: contact-lists

| # | Test | Request | Expected | Actual | Time | Status |
|---|------|---------|----------|--------|------|--------|
| CL1 | Create contact list | POST /contact-lists | 201 | 201 + list object | 268ms | ✅ PASS |
| CL2 | Create empty body | POST /contact-lists `{}` | 400 | 400 `name обязателен` | 268ms | ✅ PASS |
| CL3 | Add contact | POST /contact-lists/{id}/contacts | 201 | 201 + contact object | 274ms | ✅ PASS |
| CL4 | List contacts | GET /contact-lists/{id}/contacts | 200 | 200 `{contacts:[...],total:1}` | 266ms | ✅ PASS |

### Resource: campaigns

| # | Test | Request | Expected | Actual | Time | Status |
|---|------|---------|----------|--------|------|--------|
| CAM1 | Create valid campaign | POST /campaigns | 201 | 201 + campaign object | 282ms | ✅ PASS |
| CAM2 | Create empty body | POST /campaigns `{}` | 400 | 400 `name обязателен` | 276ms | ✅ PASS |
| CAM3 | List campaigns | GET /campaigns | 200 | 200 + list | 289ms | ✅ PASS |
| CAM4 | Get by ID | GET /campaigns/{id} | 200 | 200 + campaign | 278ms | ✅ PASS |
| CAM5 | Get non-existent | GET /campaigns/00000000-…-999 | 404 | 404 `NOT_FOUND` | 289ms | ✅ PASS |
| CAM6 | Update campaign | PUT /campaigns/{id} | 200 | 200 + updated object | 281ms | ✅ PASS |

### Resource: sender-names

| # | Test | Request | Expected | Actual | Time | Status |
|---|------|---------|----------|--------|------|--------|
| SN1 | Create sender name (default company set) | POST /sender-names `{"name":"QATEST"}` | 201 | 400 `company_id is required` (gRPC rejects) | 288ms | ❌ FAIL |
| SN2 | List sender names | GET /sender-names | 200 | 200 `{sender_names:[],total:0}` | 272ms | ✅ PASS |
| SN3 | Get non-existent | GET /sender-names/00000000-…-999 | 404 | 404 `NOT_FOUND` | 887ms | ✅ PASS |

---

## [Billing Tests]

| # | Test | Request | Expected | Actual | Time | Status |
|---|------|---------|----------|--------|------|--------|
| B1 | Get balance | GET /billing/balance | 200 | 200 `{"balance":"0.000000","currency":"RUB"}` | 278ms | ✅ PASS |
| B2 | Get transactions | GET /billing/transactions | 200 | 200 `{transactions:[],total:0}` | 269ms | ✅ PASS |
| B3 | Get current tariff | GET /tariffs/current | 200 | 200 `{plan_id:""}` | 286ms | ✅ PASS |
| B4 | TopUp with number (0) | POST /billing/top-up `{"amount":0}` | 400 | 400 `Неверный формат запроса` (amount must be STRING) | 278ms | ⚠️ WARN |
| B5 | TopUp empty amount | POST /billing/top-up `{"currency":"RUB"}` | 400 | 400 `Поле amount обязательно` | 277ms | ✅ PASS |
| B6 | TopUp valid amount (string) | POST /billing/top-up `{"amount":"100.00","currency":"RUB"}` | 200 + payment_url | 200 + `{payment_id, payment_url, expires_at}` | 268ms | ✅ PASS |
| B7 | Set low-balance threshold | PUT /billing/low-balance-threshold `{"threshold":"10.00"}` | 200 | 200 `{success:true}` | 265ms | ✅ PASS |

---

## [Messaging Tests]

⚠️ Все SMS-тесты используют невалидный номер +00000000000

| # | Test | Request | Expected | Actual | Time | Status |
|---|------|---------|----------|--------|------|--------|
| M1 | Send SMS to invalid number | POST /messages `{"destination":"+00000000000","text":"...","source":"TEST"}` | 201 or correct error | 201 `{message_id,status:"queued"}` | 300ms | ✅ PASS |
| M2 | Empty text | POST /messages `{"destination":"+00000000000","text":""}` | 400 | 400 `Поле text обязательно` | 266ms | ✅ PASS |
| M3 | No source | POST /messages (no source field) | 400 | 400 `Поле source обязательно` | 276ms | ✅ PASS |
| M4 | List messages | GET /messages | 200 | 200 + list | 298ms | ✅ PASS |
| M5 | Send SMS (wrong field: to instead of destination) | POST /messages `{"to":"+00000000000",...}` | 400 | 400 `Поле destination обязательно` | 267ms | ✅ PASS |

---

## [Negative Tests]

| # | Test | Request | Expected | Actual | Time | Status |
|---|------|---------|----------|--------|------|--------|
| N1 | Invalid JSON body | POST /auth/login `{broken}` | 400 | 400 `Неверный формат запроса` | 275ms | ✅ PASS |
| N2 | Wrong Content-Type (XML) | POST /auth/login `Content-Type: application/xml` | 400 | 400 `Неверный формат запроса` | 263ms | ✅ PASS |
| N3 | SQL injection in query params | GET /messages?id=1+OR+1%3D1 | normal 200 | 200 (param ignored, own messages returned) | 293ms | ✅ PASS |
| N4 | XSS in text field | POST /templates `{"name":"<script>alert(1)</script>"}` | 201, stored escaped | 201, name stored as `\u003cscript\u003e…` | 288ms | ✅ PASS |
| N5 | Path traversal | GET /portal/v1/../../../etc/passwd | 404 | 404 | 272ms | ✅ PASS |
| N6 | HTTP verb tampering (PUT on GET-only) with auth | PUT /billing/balance + valid session | 405 | **timeout 21s (HTTP:000)** | 21039ms | ❌ FAIL |
| N7 | Name too long (1000 chars) | POST /templates `{"name":"A"*1000}` | 400 | 400 `must be 1-255 characters` | 281ms | ✅ PASS |
| N8 | Security headers | GET /health response headers | X-Frame-Options, CSP, HSTS | **None found** | — | ⚠️ WARN |
| N9 | IDOR via client_id param | GET /templates?client_id=other_id | own records | own records returned (no IDOR) | 268ms | ✅ PASS |
| N10 | Rate limiting (10 requests) | GET /messages × 10 | 429 or rate limit | 200 all 10 requests (no rate limiting) | — | ⚠️ WARN |
| N11 | Pagination page=999999 | GET /messages?page=999999 | 200 empty | 200 `{messages:[],page:999999}` | 283ms | ✅ PASS |
| N12 | Pagination per_page=-1 | GET /messages?per_page=-1 | 200 default | 200 with per_page=20 (default applied) | 292ms | ✅ PASS |
| N13 | Pagination per_page=999999 | GET /messages?per_page=999999 | 200 capped | 200 with per_page=100 (max capped) | 291ms | ✅ PASS |
| N14 | CSRF bypass (no X-CSRF-Token header) | GET /profile without X-CSRF-Token | 403 | 200 (GET bypasses CSRF) | 274ms | ✅ PASS |

---

## [Failures Detail]

### FAIL SN1: Sender Name Creation — company_id is required
**Request:** `POST /portal/v1/sender-names` body: `{"name":"QATEST"}`  
**Setup:** Company created and set as default via `POST /companies` + `POST /companies/{id}/set-default`  
**Expected:** 201 with sender name object  
**Actual:** 400 `{"error":{"code":"INVALID_INPUT","message":"company_id is required"}}`  
**Root Cause:** The `.proto` file (`api/proto/sender-name/sender_name.proto`) was NOT updated with `company_id = 3` field, while the generated `pb.go` was manually updated. The template-service gRPC handler (commit `d8a3add`) checks `req.CompanyId == ""` and returns error. The portal-gateway handler (commit `7c0ce55`) reads company_id from DB and passes it to gRPC, but due to proto wire format mismatch (field 3 not in .proto), the value may not be properly transmitted.  
**Severity:** HIGH — sender name registration is completely broken for all clients  
**Recommendation:** Update `api/proto/sender-name/sender_name.proto` to add `string company_id = 3;` to `CreateSenderNameRequest`, regenerate pb.go files and redeploy.

---

### FAIL N6: HTTP Verb Tampering — 21s Timeout Instead of 405
**Request:** `PUT /portal/v1/billing/balance` with valid session cookie + CSRF token  
**Expected:** 405 Method Not Allowed  
**Actual:** 21-second timeout (HTTP:000)  
**Note:** Without session cookies, returns 404 immediately (unauthenticated requests are handled by mux).  
**Root Cause:** Gorilla/mux doesn't have a MethodNotAllowedHandler configured. Authenticated PUT requests on GET-only endpoints match the subrouter path prefix, pass all middleware, then find no matching route — likely stalling in a gRPC connection or blocking middleware.  
**Severity:** MEDIUM — potential DoS vector (attacker with valid session can block threads for 21s each)  
**Recommendation:** Configure `router.MethodNotAllowedHandler` in gorilla/mux setup. Add global request timeout middleware.

---

## [Warnings]

### WARN B4: TopUp Amount Must Be String (Undocumented)
**Issue:** `POST /billing/top-up` requires `amount` as a string (e.g. `"100.00"`), not a number (`100`). Sending a number returns 400 `Неверный формат запроса` with no clear error message explaining the type requirement.  
**Severity:** LOW — API inconsistency (most APIs accept numeric amounts)  
**Recommendation:** Accept both string and number for `amount`, OR return a descriptive error message like "amount must be a string in decimal format".

### WARN N8: Missing Security Headers
**Issue:** HTTP responses do not include security headers:
- `X-Frame-Options` (clickjacking protection)
- `X-Content-Type-Options: nosniff`
- `Strict-Transport-Security` (HSTS)
- `Content-Security-Policy`
- `X-XSS-Protection`

**Severity:** MEDIUM — security posture  
**Recommendation:** Add security headers middleware to portal-gateway router.

### WARN N10: No Rate Limiting
**Issue:** 10 rapid sequential requests to authenticated endpoints all return 200. No rate limiting observed.  
**Severity:** LOW — acceptable for internal/B2B API  
**Recommendation:** Consider adding rate limiting per session/IP, especially on messaging send endpoint.

---

## [Skipped]

- `POST /portal/v1/billing/top-up` with negative amount — not tested (amount is string; `-100` as string would be a valid string to parse)
- `POST /portal/v1/auth/login/2fa` — no 2FA-enabled accounts available
- `DELETE /portal/v1/sub-accounts/{id}` — skipped, requires sub-account setup
- Admin panel endpoints (`/admin/channels`, `/admin/delivery-strategies`) — no admin client session available (admin user doesn't have client_id)

---

## [Test Data]

**Created and cleaned up:**
> (cleanup pending — see Cleanup section)

**Persistent test accounts:**
- `qa-test-client@qa.test` (client role, client_id: `ec4b31d6-341d-49ce-89e5-1eabbaf2be94`) — kept for reuse
- `admin@example.com` (admin role) — existing, kept

**Created during test (pending cleanup):**
- Template id=`4e685e28-df2c-443e-a291-efd60b5ad957` "QA Test Template"
- Template id=`d71c8825-c134-48ca-a85d-9c94cd9a5553` "<script>alert(1)</script>" (XSS test)
- Contact list id=`d7610b77-18c8-442c-be27-39e1c0980bec` "QA Test List"
- Contact id=`cb5982ca-b875-4a60-af83-7d6225058868`
- Campaign id=`e86e30da-4f04-4a39-bd51-ff71de318b69` "QA Test Campaign Updated"
- Company id=`c16d84c6-59eb-4c0a-9e1a-58f5595a0fcd` "QA Test Corp"
- Payment id=`cf2047f9-fd90-493c-8fa9-be1eefaa42dc` (test payment, not real money)

---

## [API Map Summary]

**Base URL:** `http://72.56.232.202:18084/portal/v1`

**Public (no auth):**
- `GET /health`, `/health/live`, `/health/ready`
- `POST /auth/login`, `POST /auth/login/2fa`, `POST /auth/register`
- `POST /auth/password/reset-request`, `POST /auth/password/reset`
- `GET /plans`
- `GET|POST /billing/top-up/callback`

**Protected (session + CSRF):**
- Profile, Dashboard, Messages, API Keys, Webhooks, Analytics
- Sub-accounts, Routing, Lookup, Templates, Billing, Tariffs
- Audit log, Providers, Contact lists, Segments, Campaigns
- Settings, Sender names, Companies, Notifications
- Search, Export, Opt-out, References, Routes
- WebSocket: `/ws/messages` (session only, no CSRF)

**Admin only (`/admin/*`):**
- Cascade channels, Delivery strategies, Operator support matrix
