# Developer Portal Enhancement + SDK Generation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Дополнить существующий Developer Portal: (1) аудит и синхронизация OpenAPI spec со всеми реальными эндпоинтами, (2) sandbox quickstart страница с живыми примерами кода, (3) SDK пакеты для Go, Python, Node.js через openapi-generator.

**Architecture:** Developer Portal уже существует — `/docs` (Redoc), `/docs/swagger` (Swagger UI), `/docs/openapi.yaml` (сырой YAML). Spec хранится в `api/openapi/openapi.yaml` и встраивается через `api/openapi/embed.go`. SDK генерируется из OpenAPI spec через `openapi-generator-cli`. Quickstart — статическая HTML-страница, встроенная в docs handler.

**Tech Stack:** Go 1.24, OpenAPI 3.0.3, openapi-generator-cli (npm), Redoc, Swagger UI, TypeScript/Python/Go (сгенерированные SDK).

---

## File Map

| Файл | Действие | Что делаем |
|---|---|---|
| `api/openapi/openapi.yaml` | Modify | Аудит — добавить все missing эндпоинты и схемы |
| `internal/docs/docs.go` | Modify | Добавить quickstart handler + SDK download links |
| `internal/gateway/client/router/router.go` | Modify | Зарегистрировать `/docs/quickstart` роут |
| `api/sdk/go/` | Create | Сгенерированный Go SDK |
| `api/sdk/python/` | Create | Сгенерированный Python SDK |
| `api/sdk/nodejs/` | Create | Сгенерированный Node.js SDK |
| `scripts/generate-sdk.sh` | Create | Скрипт для регенерации SDK |
| `.github/workflows/sdk-release.yml` | Create | GitHub Action для публикации SDK при изменении openapi.yaml |

---

## Task 1: Аудит OpenAPI spec — добавить пропущенные эндпоинты

**Files:**
- Modify: `api/openapi/openapi.yaml`

- [ ] **Step 1.1: Составить список реальных эндпоинтов из роутера**

Прочитай `internal/gateway/client/router/router.go` и выпиши все зарегистрированные маршруты.

- [ ] **Step 1.2: Проверить что каждый эндпоинт задокументирован в openapi.yaml**

Открой `api/openapi/openapi.yaml` и для каждого маршрута из Step 1.1 проверь: есть ли соответствующий path в spec? Составь список пропущенных.

Типичные кандидаты для пропуска (проверь каждый):
- `GET /api/v1/sms/scheduled` (добавлен в Phase 1)
- `POST /api/v1/sms/batch`
- `GET /api/v1/lookup`, `POST /api/v1/lookup/bulk`
- Эндпоинты cascade (`/cascade/deliveries`)
- Эндпоинты templates, sender-names

- [ ] **Step 1.3: Добавить /sms/batch если отсутствует**

Если `POST /api/v1/sms/batch` не задокументирован, добавить в `api/openapi/openapi.yaml`:

```yaml
  /sms/batch:
    post:
      summary: Send batch SMS messages
      description: Send up to 1000 SMS messages in a single request.
      operationId: sendBatchSMS
      tags:
        - SMS
      security:
        - ApiKeyAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required:
                - messages
              properties:
                messages:
                  type: array
                  maxItems: 1000
                  items:
                    $ref: '#/components/schemas/SendSMSRequest'
                  description: List of messages to send
      responses:
        '200':
          description: Batch send results
          content:
            application/json:
              schema:
                type: object
                properties:
                  results:
                    type: array
                    items:
                      type: object
                      properties:
                        message_id:
                          type: string
                          format: uuid
                        status:
                          type: string
                        error:
                          type: string
                  total:
                    type: integer
                  success_count:
                    type: integer
                  error_count:
                    type: integer
        '400':
          $ref: '#/components/responses/BadRequest'
        '401':
          $ref: '#/components/responses/Unauthorized'
```

- [ ] **Step 1.4: Добавить оставшиеся пропущенные эндпоинты**

Для каждого пропущенного эндпоинта из Step 1.2: добавить минимальное, но точное описание по образцу существующих entries в файле.

- [ ] **Step 1.5: Проверить что все component schemas корректно определены**

```bash
# Если установлен npx/node:
npx @redocly/cli lint api/openapi/openapi.yaml
# Ожидаем: 0 errors
```

Если redocly не установлен, проверить визуально: открыть `/docs/swagger` в браузере и убедиться что нет ошибок загрузки.

- [ ] **Step 1.6: Коммит**

```bash
git add api/openapi/openapi.yaml
git commit -m "docs(openapi): audit and add missing endpoints to spec"
```

---

## Task 2: Quickstart страница

**Files:**
- Modify: `internal/docs/docs.go`
- Modify: `internal/gateway/client/router/router.go`

- [ ] **Step 2.1: Написать failing тест для quickstart handler**

Добавить в `internal/docs/docs_test.go` (создать файл если нет):

```go
package docs_test

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/stretchr/testify/assert"
    "your-module/internal/docs" // адаптируй под реальный import path
)

func TestQuickstartHandler(t *testing.T) {
    handler := docs.QuickstartHandler()
    req := httptest.NewRequest(http.MethodGet, "/docs/quickstart", nil)
    w := httptest.NewRecorder()

    handler.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
    assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
    body := w.Body.String()
    assert.Contains(t, body, "Quickstart")
    assert.Contains(t, body, "curl")
    assert.Contains(t, body, "/api/v1/sms/send")
}
```

- [ ] **Step 2.2: Запустить тест — убедиться, что падает**

```bash
go test ./internal/docs/... -run TestQuickstartHandler -v
```

Ожидаем: `FAIL — undefined: docs.QuickstartHandler`

- [ ] **Step 2.3: Реализовать QuickstartHandler**

Добавить в `internal/docs/docs.go` рядом с `RedocHandler`:

```go
// QuickstartHandler serves the quickstart guide page for developers.
func QuickstartHandler() http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(quickstartHTML))
    })
}

const quickstartHTML = `<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>SMS Platform — Quickstart</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; max-width: 860px; margin: 0 auto; padding: 2rem; color: #1a1a1a; line-height: 1.6; }
    h1 { color: #2563eb; }
    h2 { border-bottom: 2px solid #e5e7eb; padding-bottom: 0.5rem; margin-top: 2rem; }
    pre { background: #1e293b; color: #e2e8f0; padding: 1.25rem; border-radius: 8px; overflow-x: auto; }
    code { font-family: 'Fira Code', 'Consolas', monospace; font-size: 0.875rem; }
    .badge { background: #dbeafe; color: #1d4ed8; padding: 2px 8px; border-radius: 4px; font-size: 0.75rem; font-weight: 600; }
    .note { background: #fef3c7; border-left: 4px solid #f59e0b; padding: 1rem; margin: 1rem 0; border-radius: 0 4px 4px 0; }
    nav { display: flex; gap: 1rem; margin-bottom: 2rem; }
    nav a { color: #2563eb; text-decoration: none; }
    nav a:hover { text-decoration: underline; }
    .step { counter-increment: step; position: relative; padding-left: 2.5rem; margin: 1.5rem 0; }
    .step::before { content: counter(step); position: absolute; left: 0; width: 1.75rem; height: 1.75rem; background: #2563eb; color: white; border-radius: 50%; display: flex; align-items: center; justify-content: center; font-weight: 700; font-size: 0.875rem; }
    body { counter-reset: step; }
  </style>
</head>
<body>
  <nav>
    <a href="/docs">← API Reference</a>
    <a href="/docs/swagger">Swagger UI</a>
    <a href="/docs/openapi.yaml">OpenAPI Spec</a>
  </nav>

  <h1>Quickstart Guide</h1>
  <p>Отправьте первое SMS за 5 минут. Никаких SDK не нужно — достаточно <code>curl</code>.</p>

  <div class="note">
    Для тестирования используйте <strong>sandbox режим</strong> — добавьте <code>"is_sandbox": true</code> в запрос.
    Сообщения не будут отправлены реальным получателям.
  </div>

  <h2>Шаг 1: Получить API ключ</h2>
  <div class="step">
    Войдите в <a href="/">портал</a> → Настройки → API ключи → Создать ключ.
    Скопируйте ключ — он отображается только один раз.
  </div>

  <h2>Шаг 2: Отправить тестовое SMS</h2>
  <div class="step">
    <pre><code>curl -X POST https://api.your-domain.com/api/v1/sms/send \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "MyCompany",
    "destination": "+79991234567",
    "text": "Привет! Это тестовое сообщение.",
    "is_sandbox": true
  }'</code></pre>
    <p>Ответ:</p>
    <pre><code>{
  "message_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "queued",
  "created_at": "2026-04-12T10:00:00Z"
}</code></pre>
  </div>

  <h2>Шаг 3: Проверить статус доставки</h2>
  <div class="step">
    <pre><code>curl "https://api.your-domain.com/api/v1/sms/status/MESSAGE_ID" \
  -H "Authorization: Bearer YOUR_API_KEY"</code></pre>
    <p>Возможные статусы: <span class="badge">queued</span> <span class="badge">sent</span> <span class="badge">delivered</span> <span class="badge">failed</span></p>
  </div>

  <h2>Шаг 4: Получать уведомления о доставке (Webhook)</h2>
  <div class="step">
    <p>Настройте webhook в портале → Настройки → Webhooks. Мы отправим POST на ваш URL при изменении статуса:</p>
    <pre><code>{
  "message_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "delivered",
  "destination": "+79991234567",
  "delivered_at": "2026-04-12T10:00:05Z"
}</code></pre>
  </div>

  <h2>Пример: запланированная отправка</h2>
  <pre><code>curl -X POST https://api.your-domain.com/api/v1/sms/send \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "MyCompany",
    "destination": "+79991234567",
    "text": "Ваш заказ будет доставлен завтра!",
    "scheduled_at": "2026-04-13T09:00:00Z"
  }'</code></pre>

  <h2>Пример: пакетная отправка</h2>
  <pre><code>curl -X POST https://api.your-domain.com/api/v1/sms/batch \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      { "source": "MyCompany", "destination": "+79991234567", "text": "Сообщение 1" },
      { "source": "MyCompany", "destination": "+79997654321", "text": "Сообщение 2" }
    ]
  }'</code></pre>

  <h2>Скачать SDK</h2>
  <p>Официальные клиентские библиотеки генерируются из <a href="/docs/openapi.yaml">OpenAPI spec</a>.</p>
  <ul>
    <li><a href="/docs/sdk/go">Go SDK</a> — <code>go get github.com/your-org/sms-go-sdk</code></li>
    <li><a href="/docs/sdk/python">Python SDK</a> — <code>pip install sms-platform-sdk</code></li>
    <li><a href="/docs/sdk/nodejs">Node.js SDK</a> — <code>npm install @your-org/sms-sdk</code></li>
  </ul>

  <h2>Следующие шаги</h2>
  <ul>
    <li><a href="/docs">Полная документация API</a></li>
    <li><a href="/docs/swagger">Интерактивный Swagger UI</a></li>
    <li>Создать <a href="/campaigns">рассылку</a> для массовой отправки</li>
    <li>Настроить <a href="/templates">шаблоны</a> с переменными</li>
  </ul>
</body>
</html>`
```

- [ ] **Step 2.4: Зарегистрировать роут**

В `internal/gateway/client/router/router.go` добавить рядом с другими `/docs/` роутами:

```go
r.Handle("/docs/quickstart", docs.QuickstartHandler()).Methods(http.MethodGet)
```

- [ ] **Step 2.5: Запустить тест**

```bash
go test ./internal/docs/... -run TestQuickstartHandler -v
```

Ожидаем: `PASS`

- [ ] **Step 2.6: Коммит**

```bash
git add internal/docs/docs.go internal/docs/docs_test.go
git add internal/gateway/client/router/router.go
git commit -m "feat(docs): add quickstart guide page at /docs/quickstart"
```

---

## Task 3: Создать скрипт генерации SDK

**Files:**
- Create: `scripts/generate-sdk.sh`

- [ ] **Step 3.1: Проверить наличие openapi-generator**

```bash
# Проверить установлен ли openapi-generator
npx @openapitools/openapi-generator-cli version
# Если не установлен глобально — будет скачан автоматически через npx
```

- [ ] **Step 3.2: Создать скрипт generate-sdk.sh**

```bash
#!/usr/bin/env bash
set -euo pipefail

SPEC="api/openapi/openapi.yaml"
SDK_DIR="api/sdk"
VERSION="${SDK_VERSION:-1.0.0}"

echo "Generating SDKs from $SPEC (version $VERSION)..."

# Go SDK
npx @openapitools/openapi-generator-cli generate \
  -i "$SPEC" \
  -g go \
  -o "$SDK_DIR/go" \
  --additional-properties=packageName=smssdk,packageVersion="$VERSION",withGoMod=true \
  --global-property=apiDocs=false,modelDocs=false
echo "✓ Go SDK generated"

# Python SDK
npx @openapitools/openapi-generator-cli generate \
  -i "$SPEC" \
  -g python \
  -o "$SDK_DIR/python" \
  --additional-properties=packageName=sms_platform_sdk,packageVersion="$VERSION",projectName=sms-platform-sdk \
  --global-property=apiDocs=false,modelDocs=false
echo "✓ Python SDK generated"

# Node.js / TypeScript SDK
npx @openapitools/openapi-generator-cli generate \
  -i "$SPEC" \
  -g typescript-fetch \
  -o "$SDK_DIR/nodejs" \
  --additional-properties=npmName=@your-org/sms-sdk,npmVersion="$VERSION",typescriptThreePlus=true \
  --global-property=apiDocs=false,modelDocs=false
echo "✓ Node.js SDK generated"

echo "All SDKs generated in $SDK_DIR/"
```

- [ ] **Step 3.3: Сделать скрипт исполняемым**

```bash
chmod +x scripts/generate-sdk.sh
```

- [ ] **Step 3.4: Запустить генерацию**

```bash
cd /c/projects/sms
bash scripts/generate-sdk.sh
```

Ожидаем: создание директорий `api/sdk/go/`, `api/sdk/python/`, `api/sdk/nodejs/` с кодом.

- [ ] **Step 3.5: Добавить .gitignore для тяжёлых артефактов SDK**

Добавить в корневой `.gitignore` (или создать `api/sdk/.gitignore`):

```gitignore
# Generated SDK docs (regenerated from openapi spec)
api/sdk/*/docs/
api/sdk/*/.openapi-generator/
api/sdk/*/test/
```

- [ ] **Step 3.6: Проверить что сгенерированный Go SDK компилируется**

```bash
cd api/sdk/go
go build ./...
```

Ожидаем: компиляция без ошибок.

- [ ] **Step 3.7: Коммит**

```bash
git add scripts/generate-sdk.sh api/sdk/ .gitignore
git commit -m "feat(sdk): add openapi-generator script and generated Go/Python/Node SDKs"
```

---

## Task 4: Обновить docs handler — ссылки на SDK

**Files:**
- Modify: `internal/docs/docs.go`

- [ ] **Step 4.1: Добавить обработчики скачивания SDK info**

Добавить в `internal/docs/docs.go`:

```go
// SDKInfoHandler returns SDK information and installation instructions.
func SDKInfoHandler(lang string) http.Handler {
    type sdkInfo struct {
        Language    string `json:"language"`
        PackageName string `json:"package_name"`
        Version     string `json:"version"`
        Install     string `json:"install"`
        Repository  string `json:"repository"`
        GeneratedAt string `json:"generated_at"`
    }

    infos := map[string]sdkInfo{
        "go": {
            Language:    "Go",
            PackageName: "smssdk",
            Version:     "1.0.0",
            Install:     "go get github.com/your-org/sms-go-sdk",
            Repository:  "https://github.com/your-org/sms-go-sdk",
            GeneratedAt: "Generated from OpenAPI spec via openapi-generator",
        },
        "python": {
            Language:    "Python",
            PackageName: "sms_platform_sdk",
            Version:     "1.0.0",
            Install:     "pip install sms-platform-sdk",
            Repository:  "https://github.com/your-org/sms-python-sdk",
            GeneratedAt: "Generated from OpenAPI spec via openapi-generator",
        },
        "nodejs": {
            Language:    "Node.js / TypeScript",
            PackageName: "@your-org/sms-sdk",
            Version:     "1.0.0",
            Install:     "npm install @your-org/sms-sdk",
            Repository:  "https://github.com/your-org/sms-node-sdk",
            GeneratedAt: "Generated from OpenAPI spec via openapi-generator",
        },
    }

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        info, ok := infos[lang]
        if !ok {
            http.NotFound(w, r)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(info)
    })
}
```

- [ ] **Step 4.2: Зарегистрировать SDK роуты**

В `internal/gateway/client/router/router.go`:

```go
r.Handle("/docs/sdk/go",     docs.SDKInfoHandler("go")).Methods(http.MethodGet)
r.Handle("/docs/sdk/python", docs.SDKInfoHandler("python")).Methods(http.MethodGet)
r.Handle("/docs/sdk/nodejs", docs.SDKInfoHandler("nodejs")).Methods(http.MethodGet)
```

- [ ] **Step 4.3: Добавить import encoding/json если нужен**

```go
import (
    "encoding/json"
    // ... остальные imports
)
```

- [ ] **Step 4.4: Собрать проект**

```bash
go build ./...
```

Ожидаем: ошибок нет.

- [ ] **Step 4.5: Коммит**

```bash
git add internal/docs/docs.go internal/gateway/client/router/router.go
git commit -m "feat(docs): add SDK info endpoints at /docs/sdk/{go,python,nodejs}"
```

---

## Task 5: Финальная проверка Developer Portal

- [ ] **Step 5.1: Проверить все docs-эндпоинты**

Запустить сервис и проверить:
```
GET /docs              → Redoc UI без ошибок
GET /docs/swagger      → Swagger UI без ошибок
GET /docs/openapi.yaml → валидный YAML
GET /docs/quickstart   → HTML страница с примерами
GET /docs/sdk/go       → JSON с info
GET /docs/sdk/python   → JSON с info
GET /docs/sdk/nodejs   → JSON с info
```

- [ ] **Step 5.2: Запустить все тесты**

```bash
go test ./internal/docs/... -v
go test ./internal/gateway/client/... -v
```

Ожидаем: `PASS`

- [ ] **Step 5.3: Финальный коммит если нужен**

```bash
git status
# Если есть незакомиченные изменения:
git add .
git commit -m "feat(developer-portal): complete Phase 2 - OpenAPI audit, quickstart, SDK generation"
```
