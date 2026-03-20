# Admin API Contract

**Date**: 2026-03-20
**Protocol**: HTTP REST (port 8081) + gRPC (port 9091)
**Authentication**: JWT Bearer token with admin role

## Authentication

```
POST /api/v1/auth/login
{ "username": "admin", "password": "..." }
→ { "token": "jwt...", "expires_at": "..." }

All subsequent requests:
Authorization: Bearer {jwt_token}
```

## Client Management

```
POST   /api/v1/clients            - Create client
GET    /api/v1/clients             - List clients
GET    /api/v1/clients/{id}        - Get client details
PUT    /api/v1/clients/{id}        - Update client
DELETE /api/v1/clients/{id}        - Deactivate client
POST   /api/v1/clients/{id}/key    - Regenerate API key
```

## Provider Management

```
POST   /api/v1/providers           - Add SMSC provider
GET    /api/v1/providers            - List providers
GET    /api/v1/providers/{id}       - Get provider details + health
PUT    /api/v1/providers/{id}       - Update provider config
DELETE /api/v1/providers/{id}       - Remove provider
GET    /api/v1/providers/{id}/health - Provider health check
```

## Route Management

```
POST   /api/v1/routes              - Create routing rule
GET    /api/v1/routes               - List routes
GET    /api/v1/routes/{id}          - Get route details
PUT    /api/v1/routes/{id}          - Update route
DELETE /api/v1/routes/{id}          - Delete route
```

## Template Management

```
POST   /api/v1/templates           - Create template (status: draft)
GET    /api/v1/templates            - List templates (filter by status)
GET    /api/v1/templates/{id}       - Get template details
PUT    /api/v1/templates/{id}       - Update template
POST   /api/v1/templates/{id}/approve  - Approve template
POST   /api/v1/templates/{id}/reject   - Reject template (with reason)
DELETE /api/v1/templates/{id}       - Delete template
```

## Billing Management

```
GET    /api/v1/billing/accounts              - List all accounts
GET    /api/v1/billing/accounts/{client_id}  - Get account balance
POST   /api/v1/billing/accounts/{client_id}/credit  - Add credits
GET    /api/v1/billing/transactions          - Transaction history

POST   /api/v1/billing/pricing              - Create pricing rule
GET    /api/v1/billing/pricing              - List pricing rules
PUT    /api/v1/billing/pricing/{id}         - Update pricing rule
DELETE /api/v1/billing/pricing/{id}         - Delete pricing rule
```

## Analytics

```
GET /api/v1/analytics/stats?client_id=...&provider_id=...&status=...&from=...&to=...
GET /api/v1/analytics/reports?format=json|csv&from=...&to=...
```

## Audit Log

```
GET /api/v1/audit?actor_id=...&entity_type=...&from=...&to=...&page=1&per_page=50
```
