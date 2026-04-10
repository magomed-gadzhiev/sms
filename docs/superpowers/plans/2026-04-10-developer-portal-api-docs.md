# Developer Portal / API Docs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add public `/docs` (Redoc), `/docs/swagger` (Swagger UI) and `/docs/grpc` (gRPC reference) routes to the Client Gateway — served from embedded static files, no new services.

**Architecture:** `internal/docs/docs.go` is the single package responsible for all documentation handlers. It embeds a hand-authored `grpc.html` (can be regenerated with `protoc-gen-doc`) and serves pre-baked HTML referencing CDN-hosted Redoc/Swagger UI. Router registers these routes before auth middleware so they are public.

**Tech Stack:** Go 1.24, gorilla/mux, `//go:embed`, Redoc CDN (`cdn.jsdelivr.net`), Swagger UI CDN (`unpkg.com`)

---

## File Map

| Action | Path | Responsibility |
|---|---|---|
| Modify | `api/openapi/openapi.yaml` | Add Getting Started block to `info.description` |
| Create | `internal/docs/grpc.html` | Hand-authored gRPC service reference page |
| Modify | `internal/docs/docs.go` | Add `RedocHandler`, `GRPCDocsHandler`, embed `grpc.html` |
| Create | `internal/docs/docs_test.go` | Unit tests for all four handlers |
| Modify | `internal/gateway/client/router/router.go` | `/docs` → Redoc, `/docs/swagger` → Swagger UI, `/docs/grpc` → gRPC docs |
| Create | `scripts/gen-proto-docs.sh` | Helper to regenerate `grpc.html` with `protoc-gen-doc` |
| Create | `Makefile` | `gen-proto-docs`, `build-client-gateway` targets |

---

## Task 1: Update openapi.yaml with Getting Started

**Files:**
- Modify: `api/openapi/openapi.yaml` (lines 4–11, `info.description`)

- [ ] **Step 1: Replace info.description**

Open `api/openapi/openapi.yaml` and replace the existing `info.description` block (lines 4–11) with:

```yaml
  description: |
    ## Getting Started

    1. Получите API-ключ в [личном кабинете](http://72.56.232.202:8083)
    2. Добавьте заголовок: `X-API-Key: <your-api-key>`
    3. Базовый URL: `http://72.56.232.202:8080/api/v1`

    ### Первый запрос

    ```bash
    curl -X POST http://72.56.232.202:8080/api/v1/sms/send \
      -H "X-API-Key: YOUR_API_KEY" \
      -H "Content-Type: application/json" \
      -d '{"phone": "+79001234567", "text": "Hello!"}'
    ```

    ---

    API для отправки SMS сообщений через SMS Gateway платформу.

    Поддерживает отправку одиночных и пакетных SMS, получение статуса сообщений,
    историю, управление шаблонами, вебхуками и HLR-запросы.

    ## Аутентификация
    Все запросы к `/api/v1/*` требуют заголовок `X-API-Key` с вашим API ключом.
```

- [ ] **Step 2: Verify YAML parses**

```bash
cd /c/projects/sms
python3 -c "import yaml,sys; yaml.safe_load(open('api/openapi/openapi.yaml'))" 2>&1 || \
  go run -mod=mod golang.org/x/tools/cmd/goimports@latest -l . 2>&1 | head -5
```

If Python isn't available, just check the file visually for correct YAML indentation — `description:` block must be indented under `info:`.

- [ ] **Step 3: Commit**

```bash
git add api/openapi/openapi.yaml
git commit -m "docs: add Getting Started section to openapi.yaml"
```

---

## Task 2: Create grpc.html

**Files:**
- Create: `internal/docs/grpc.html`

- [ ] **Step 1: Create the file**

Create `internal/docs/grpc.html` with the following content:

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>SMS Gateway — gRPC API</title>
  <style>
    body { font-family: Roboto, Arial, sans-serif; margin: 0; padding: 0; background: #fafafa; }
    .header { background: #2a3f5f; color: white; padding: 32px 48px; }
    .header h1 { margin: 0; font-size: 28px; }
    .header p { margin: 8px 0 0; opacity: 0.8; font-size: 15px; }
    .container { max-width: 960px; margin: 0 auto; padding: 32px 48px; }
    .note { background: #e8f4fd; border-left: 4px solid #1a73e8; padding: 16px 24px; border-radius: 4px; margin-bottom: 32px; font-size: 14px; line-height: 1.6; }
    .note a { color: #1a73e8; }
    .note code { background: #d0e8fa; padding: 2px 6px; border-radius: 3px; font-family: monospace; font-size: 13px; }
    .service { background: white; border: 1px solid #e0e0e0; border-radius: 8px; margin-bottom: 24px; }
    .service-header { padding: 16px 24px; border-bottom: 1px solid #e0e0e0; }
    .service-header h2 { margin: 0; font-size: 18px; color: #2a3f5f; }
    .service-header p { margin: 4px 0 0; color: #666; font-size: 14px; }
    .method { padding: 12px 24px; border-bottom: 1px solid #f0f0f0; display: flex; align-items: baseline; gap: 16px; }
    .method:last-child { border-bottom: none; }
    .method-name { font-family: monospace; font-size: 14px; font-weight: bold; color: #1a73e8; min-width: 240px; }
    .method-desc { color: #555; font-size: 14px; }
  </style>
</head>
<body>
  <div class="header">
    <h1>SMS Gateway — gRPC API</h1>
    <p>Internal gRPC service reference. Proto sources: <code>api/proto/</code></p>
  </div>
  <div class="container">
    <div class="note">
      gRPC services communicate between internal microservices. For external integration use
      <a href="/docs">REST API (Redoc)</a> or <a href="/docs/swagger">Swagger UI</a>.<br>
      To regenerate this page from proto sources: <code>make gen-proto-docs</code>
    </div>

    <div class="service">
      <div class="service-header"><h2>Messaging</h2><p>Core SMS sending and message lifecycle</p></div>
      <div class="method"><span class="method-name">SendSMS</span><span class="method-desc">Send a single SMS message</span></div>
      <div class="method"><span class="method-name">SendBatch</span><span class="method-desc">Send a batch of SMS messages</span></div>
      <div class="method"><span class="method-name">GetMessageStatus</span><span class="method-desc">Get status of a message by ID</span></div>
      <div class="method"><span class="method-name">GetMessageHistory</span><span class="method-desc">Paginated message history for an account</span></div>
      <div class="method"><span class="method-name">CancelMessage</span><span class="method-desc">Cancel a pending message</span></div>
    </div>

    <div class="service">
      <div class="service-header"><h2>Auth</h2><p>Authentication and API key management</p></div>
      <div class="method"><span class="method-name">ValidateAPIKey</span><span class="method-desc">Validate an API key and return account context</span></div>
      <div class="method"><span class="method-name">CreateAPIKey</span><span class="method-desc">Create a new API key for an account</span></div>
      <div class="method"><span class="method-name">RevokeAPIKey</span><span class="method-desc">Revoke an existing API key</span></div>
      <div class="method"><span class="method-name">ListAPIKeys</span><span class="method-desc">List all API keys for an account</span></div>
    </div>

    <div class="service">
      <div class="service-header"><h2>Billing</h2><p>Account balance and transaction management</p></div>
      <div class="method"><span class="method-name">GetBalance</span><span class="method-desc">Get current balance for an account</span></div>
      <div class="method"><span class="method-name">Deduct</span><span class="method-desc">Deduct amount from account balance</span></div>
      <div class="method"><span class="method-name">GetTransactions</span><span class="method-desc">Get transaction history</span></div>
    </div>

    <div class="service">
      <div class="service-header"><h2>Routing</h2><p>SMS route selection and provider assignment</p></div>
      <div class="method"><span class="method-name">GetRoute</span><span class="method-desc">Get routing rule for a destination</span></div>
      <div class="method"><span class="method-name">ListRoutes</span><span class="method-desc">List all routing rules</span></div>
    </div>

    <div class="service">
      <div class="service-header"><h2>Analytics</h2><p>Delivery statistics and reporting</p></div>
      <div class="method"><span class="method-name">GetDeliveryStats</span><span class="method-desc">Delivery rate statistics by time range</span></div>
      <div class="method"><span class="method-name">GetAccountStats</span><span class="method-desc">Message volume and cost by account</span></div>
    </div>

    <div class="service">
      <div class="service-header"><h2>Template</h2><p>SMS template management</p></div>
      <div class="method"><span class="method-name">CreateTemplate</span><span class="method-desc">Create a message template</span></div>
      <div class="method"><span class="method-name">GetTemplate</span><span class="method-desc">Get template by ID</span></div>
      <div class="method"><span class="method-name">ListTemplates</span><span class="method-desc">List templates for an account</span></div>
      <div class="method"><span class="method-name">UpdateTemplate</span><span class="method-desc">Update template content</span></div>
      <div class="method"><span class="method-name">DeleteTemplate</span><span class="method-desc">Delete a template</span></div>
    </div>

    <div class="service">
      <div class="service-header"><h2>Webhook</h2><p>Webhook endpoint management and event delivery</p></div>
      <div class="method"><span class="method-name">CreateWebhook</span><span class="method-desc">Register a new webhook endpoint</span></div>
      <div class="method"><span class="method-name">ListWebhooks</span><span class="method-desc">List webhooks for an account</span></div>
      <div class="method"><span class="method-name">DeleteWebhook</span><span class="method-desc">Remove a webhook</span></div>
      <div class="method"><span class="method-name">DeliverEvent</span><span class="method-desc">Deliver an event to registered webhooks</span></div>
    </div>

    <div class="service">
      <div class="service-header"><h2>Cascade</h2><p>Multi-channel cascade delivery</p></div>
      <div class="method"><span class="method-name">CreateDelivery</span><span class="method-desc">Create a cascade delivery (SMS + fallbacks)</span></div>
      <div class="method"><span class="method-name">GetDelivery</span><span class="method-desc">Get cascade delivery status</span></div>
      <div class="method"><span class="method-name">ListDeliveries</span><span class="method-desc">List cascade deliveries for an account</span></div>
    </div>

    <div class="service">
      <div class="service-header"><h2>Tarification</h2><p>Pricing plans and tariff management</p></div>
      <div class="method"><span class="method-name">GetTariff</span><span class="method-desc">Get tariff for an account and destination</span></div>
      <div class="method"><span class="method-name">ListPlans</span><span class="method-desc">List available pricing plans</span></div>
    </div>

    <div class="service">
      <div class="service-header"><h2>Campaign</h2><p>Bulk SMS campaign management</p></div>
      <div class="method"><span class="method-name">CreateCampaign</span><span class="method-desc">Create a new SMS campaign</span></div>
      <div class="method"><span class="method-name">GetCampaign</span><span class="method-desc">Get campaign details and progress</span></div>
      <div class="method"><span class="method-name">StartCampaign</span><span class="method-desc">Start a pending campaign</span></div>
      <div class="method"><span class="method-name">StopCampaign</span><span class="method-desc">Pause or cancel a running campaign</span></div>
    </div>

    <div class="service">
      <div class="service-header"><h2>SenderName</h2><p>Sender name registration and approval</p></div>
      <div class="method"><span class="method-name">RegisterSenderName</span><span class="method-desc">Submit sender name for operator approval</span></div>
      <div class="method"><span class="method-name">GetSenderName</span><span class="method-desc">Get sender name registration status</span></div>
      <div class="method"><span class="method-name">ListSenderNames</span><span class="method-desc">List sender names for an account</span></div>
    </div>

    <div class="service">
      <div class="service-header"><h2>Contact</h2><p>Contact list management</p></div>
      <div class="method"><span class="method-name">CreateList</span><span class="method-desc">Create a contact list</span></div>
      <div class="method"><span class="method-name">AddContacts</span><span class="method-desc">Add contacts to a list</span></div>
      <div class="method"><span class="method-name">ListContacts</span><span class="method-desc">Paginated contacts from a list</span></div>
    </div>

    <div class="service">
      <div class="service-header"><h2>Audit</h2><p>Audit log for account actions</p></div>
      <div class="method"><span class="method-name">LogEvent</span><span class="method-desc">Record an audit event</span></div>
      <div class="method"><span class="method-name">GetEvents</span><span class="method-desc">Query audit events by filter</span></div>
    </div>

    <div class="service">
      <div class="service-header"><h2>Link</h2><p>Short link creation and click tracking</p></div>
      <div class="method"><span class="method-name">CreateShortLink</span><span class="method-desc">Create a trackable short link</span></div>
      <div class="method"><span class="method-name">GetLinkStats</span><span class="method-desc">Get click statistics for a link</span></div>
    </div>

    <div class="service">
      <div class="service-header"><h2>Provider</h2><p>SMS provider integration</p></div>
      <div class="method"><span class="method-name">GetProvider</span><span class="method-desc">Get provider configuration by ID</span></div>
      <div class="method"><span class="method-name">ListProviders</span><span class="method-desc">List all configured providers</span></div>
    </div>
  </div>
</body>
</html>
```

- [ ] **Step 2: Commit**

```bash
git add internal/docs/grpc.html
git commit -m "docs: add hand-authored gRPC service reference page"
```

---

## Task 3: Update internal/docs/docs.go

**Files:**
- Modify: `internal/docs/docs.go`

Current state: file has `SwaggerUIHandler()`, `OpenAPISpecHandler()`, and `swaggerUIHTML` const.

- [ ] **Step 1: Write failing tests first**

Create `internal/docs/docs_test.go`:

```go
package docs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRedocHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	w := httptest.NewRecorder()
	RedocHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected text/html; charset=utf-8, got %q", ct)
	}
	if !strings.Contains(w.Body.String(), "redoc") {
		t.Error("body should contain 'redoc'")
	}
}

func TestSwaggerUIHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/docs/swagger", nil)
	w := httptest.NewRecorder()
	SwaggerUIHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected text/html; charset=utf-8, got %q", ct)
	}
	if !strings.Contains(w.Body.String(), "swagger-ui") {
		t.Error("body should contain 'swagger-ui'")
	}
}

func TestGRPCDocsHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/docs/grpc", nil)
	w := httptest.NewRecorder()
	GRPCDocsHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected text/html; charset=utf-8, got %q", ct)
	}
	if len(w.Body.Bytes()) == 0 {
		t.Error("body should not be empty")
	}
}

func TestOpenAPISpecHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil)
	w := httptest.NewRecorder()
	OpenAPISpecHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/yaml" {
		t.Errorf("expected application/yaml, got %q", ct)
	}
	if len(w.Body.Bytes()) == 0 {
		t.Error("body should not be empty")
	}
	if !strings.Contains(w.Body.String(), "openapi:") {
		t.Error("body should contain openapi spec content")
	}
}
```

- [ ] **Step 2: Run tests — expect failure on RedocHandler and GRPCDocsHandler**

```bash
cd /c/projects/sms
go test ./internal/docs/... -v 2>&1 | head -40
```

Expected: build failure — `./docs_test.go: undefined: RedocHandler` (or similar). No tests will run until the next step.

- [ ] **Step 3: Replace internal/docs/docs.go with updated version**

```go
package docs

import (
	_ "embed"
	"net/http"

	openapi "github.com/smpp-server/smpp-server/api/openapi"
)

//go:embed grpc.html
var grpcDocsHTML []byte

// RedocHandler returns the Redoc documentation page.
func RedocHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(redocHTML))
	}
}

// SwaggerUIHandler returns the Swagger UI page.
func SwaggerUIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(swaggerUIHTML))
	}
}

// GRPCDocsHandler returns the gRPC service reference page.
func GRPCDocsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(grpcDocsHTML)
	}
}

// OpenAPISpecHandler returns the raw OpenAPI YAML spec.
func OpenAPISpecHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(openapi.Spec)
	}
}

const redocHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <title>SMS Gateway API</title>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link href="https://fonts.googleapis.com/css?family=Montserrat:300,400,700|Roboto:300,400,700" rel="stylesheet">
  <style>body { margin: 0; padding: 0; }</style>
</head>
<body>
  <redoc spec-url='/docs/openapi.yaml'></redoc>
  <script src="https://cdn.jsdelivr.net/npm/redoc/bundles/redoc.standalone.js"></script>
</body>
</html>`

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="UTF-8">
  <title>SMS Gateway API — Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <style>
    body { margin: 0; }
    .topbar { display: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: "/docs/openapi.yaml",
      dom_id: "#swagger-ui",
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
      layout: "BaseLayout",
      deepLinking: true,
      tryItOutEnabled: true,
    });
  </script>
</body>
</html>`
```

- [ ] **Step 4: Run tests — expect all pass**

```bash
cd /c/projects/sms
go test ./internal/docs/... -v
```

Expected output:
```
--- PASS: TestRedocHandler (0.00s)
--- PASS: TestSwaggerUIHandler (0.00s)
--- PASS: TestGRPCDocsHandler (0.00s)
--- PASS: TestOpenAPISpecHandler (0.00s)
PASS
```

- [ ] **Step 5: Commit**

```bash
git add internal/docs/docs.go internal/docs/docs_test.go
git commit -m "feat(docs): add RedocHandler and GRPCDocsHandler with tests"
```

---

## Task 4: Update router to add /docs/swagger and /docs/grpc

**Files:**
- Modify: `internal/gateway/client/router/router.go` (lines near bottom, the existing docs block)

Current router has:
```go
router.HandleFunc("/docs", docs.SwaggerUIHandler()).Methods("GET")
router.HandleFunc("/docs/openapi.yaml", docs.OpenAPISpecHandler()).Methods("GET")
```

- [ ] **Step 1: Replace the docs block**

Find and replace the two existing docs lines with four lines:

```go
	// API документация (без аутентификации)
	router.HandleFunc("/docs", docs.RedocHandler()).Methods("GET")
	router.HandleFunc("/docs/swagger", docs.SwaggerUIHandler()).Methods("GET")
	router.HandleFunc("/docs/grpc", docs.GRPCDocsHandler()).Methods("GET")
	router.HandleFunc("/docs/openapi.yaml", docs.OpenAPISpecHandler()).Methods("GET")
```

- [ ] **Step 2: Verify the project compiles**

```bash
cd /c/projects/sms
go build ./cmd/client-gateway/... 2>&1
```

Expected: no output (clean build).

- [ ] **Step 3: Run all docs tests**

```bash
cd /c/projects/sms
go test ./internal/docs/... -v
```

Expected: 4 PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/client/router/router.go
git commit -m "feat(router): add /docs/swagger and /docs/grpc routes, /docs now serves Redoc"
```

---

## Task 5: Add Makefile and gen-proto-docs script

**Files:**
- Create: `Makefile`
- Create: `scripts/gen-proto-docs.sh`

- [ ] **Step 1: Create scripts/gen-proto-docs.sh**

```bash
#!/bin/bash
# Regenerates internal/docs/grpc.html from all .proto files in api/proto/.
# Requires: protoc, protoc-gen-doc
# Install: go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@latest
set -e

PROTO_DIR="api/proto"
OUT_FILE="internal/docs/grpc.html"

echo "Generating gRPC docs from $PROTO_DIR → $OUT_FILE"

protoc \
  --doc_out="$(dirname "$OUT_FILE")" \
  --doc_opt=html,"$(basename "$OUT_FILE")" \
  --proto_path="$PROTO_DIR" \
  $(find "$PROTO_DIR" -name "*.proto" | sort)

echo "Done. Commit $OUT_FILE to persist the changes."
```

- [ ] **Step 2: Make the script executable**

```bash
chmod +x scripts/gen-proto-docs.sh
```

- [ ] **Step 3: Create Makefile**

```makefile
.PHONY: gen-proto-docs build-client-gateway test

gen-proto-docs:
	bash scripts/gen-proto-docs.sh

build-client-gateway:
	go build ./cmd/client-gateway/...

test:
	go test ./... 2>&1
```

- [ ] **Step 4: Verify make targets are recognised**

```bash
cd /c/projects/sms
make build-client-gateway 2>&1
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add Makefile scripts/gen-proto-docs.sh
git commit -m "build: add Makefile with build-client-gateway and gen-proto-docs targets"
```

---

## Task 6: Smoke test all routes

- [ ] **Step 1: Start the client gateway locally (or use the server)**

Option A — local (if postgres/redis/grpc deps available):
```bash
cd /c/projects/sms
go run ./cmd/client-gateway/... &
sleep 2
```

Option B — deploy to server and test there:
```bash
bash scripts/server.sh deploy client-gateway
```

- [ ] **Step 2: Check all four /docs routes**

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/docs
# Expected: 200

curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/docs/swagger
# Expected: 200

curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/docs/grpc
# Expected: 200

curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/docs/openapi.yaml
# Expected: 200
```

Replace `localhost:8080` with `72.56.232.202:8080` if testing on server.

- [ ] **Step 3: Verify /docs returns Redoc (not Swagger UI)**

```bash
curl -s http://localhost:8080/docs | grep -i redoc
# Expected: one or more lines containing "redoc"
```

- [ ] **Step 4: Verify /docs/swagger returns Swagger UI**

```bash
curl -s http://localhost:8080/docs/swagger | grep -i swagger
# Expected: one or more lines containing "swagger"
```

- [ ] **Step 5: Kill local server if started in Step 1**

```bash
kill %1 2>/dev/null || true
```

- [ ] **Step 6: Final commit if any fixes were needed**

```bash
git add -p
git commit -m "fix(docs): smoke test fixes" 2>/dev/null || echo "nothing to commit"
```
