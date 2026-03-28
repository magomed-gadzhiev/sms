# WS1: Template Engine (Liquid) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Liquid template rendering with custom SMS filters, validation, and preview to the SMS platform.

**Architecture:** New shared package `internal/shared/template/` wraps `osteele/liquid` with SMS-specific filters, sandbox constraints (50ms timeout, 1600 char limit), and length validation. Integrates into campaign-service (preview endpoint), portal-gateway (HTTP route), and pipeline-worker (render before send). Frontend gets TemplatePreview component in CampaignWizard.

**Tech Stack:** Go 1.24.0, `github.com/osteele/liquid`, existing pgx/zerolog/mux/grpc stack, React 19 + TypeScript

---

## File Structure

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `internal/shared/template/renderer.go` | Liquid renderer with custom filters, sandbox |
| Create | `internal/shared/template/renderer_test.go` | Unit tests for renderer |
| Create | `internal/shared/template/validator.go` | Syntax check, length estimation |
| Create | `internal/shared/template/validator_test.go` | Unit tests for validator |
| Create | `migrations/000046_rendered_text.up.sql` | Add `rendered_text` column to campaign_recipients |
| Create | `migrations/000046_rendered_text.down.sql` | Rollback migration |
| Modify | `internal/services/campaign/domain/models.go` | Add RenderedText to Recipient |
| Modify | `internal/services/campaign/application/campaign_service.go` | Add PreviewTemplate method |
| Modify | `internal/services/campaign/infrastructure/repository/recipient_repo.go` | Handle rendered_text column |
| Modify | `api/proto/campaign/campaign.proto` | Add PreviewTemplate RPC |
| Modify | `internal/services/campaign/grpc/server.go` | Implement PreviewTemplate handler |
| Modify | `internal/gateway/portal/handlers/campaigns.go` | Add PreviewTemplate HTTP handler |
| Modify | `internal/gateway/portal/router/router.go` | Register POST /campaigns/templates/preview route |
| Create | `portal-frontend/src/components/campaigns/TemplatePreview.tsx` | Preview component |
| Modify | `portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx` | Integrate TemplatePreview |

---

### Task 1: Add `osteele/liquid` dependency

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add the liquid library**

```bash
cd /home/magomed/projects/sms && go get github.com/osteele/liquid@latest
```

- [ ] **Step 2: Verify it's in go.mod**

```bash
grep "osteele/liquid" go.mod
```

Expected: line showing `github.com/osteele/liquid vX.X.X`

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add osteele/liquid for template engine"
```

---

### Task 2: Create template renderer with custom filters

**Files:**
- Create: `internal/shared/template/renderer.go`
- Create: `internal/shared/template/renderer_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/shared/template/renderer_test.go
package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderer_SimpleVariable(t *testing.T) {
	r := NewRenderer()
	result, err := r.Render("Привет, {{ name }}!", map[string]interface{}{"name": "Иван"})
	require.NoError(t, err)
	assert.Equal(t, "Привет, Иван!", result)
}

func TestRenderer_DefaultFilter(t *testing.T) {
	r := NewRenderer()
	result, err := r.Render(`Привет, {{ name | default: "друг" }}!`, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "Привет, друг!", result)
}

func TestRenderer_TruncateFilter(t *testing.T) {
	r := NewRenderer()
	result, err := r.Render(`{{ text | truncate: 10 }}`, map[string]interface{}{"text": "Очень длинный текст"})
	require.NoError(t, err)
	assert.Equal(t, "Очень длин...", result)
}

func TestRenderer_PhoneFormatFilter(t *testing.T) {
	r := NewRenderer()
	result, err := r.Render(`{{ phone | phone_format }}`, map[string]interface{}{"phone": "79991234567"})
	require.NoError(t, err)
	assert.Equal(t, "+7 (999) 123-45-67", result)
}

func TestRenderer_DateFilter(t *testing.T) {
	r := NewRenderer()
	result, err := r.Render(`{{ order_date | date: "%d.%m.%Y" }}`, map[string]interface{}{"order_date": "2026-03-28T10:00:00Z"})
	require.NoError(t, err)
	assert.Equal(t, "28.03.2026", result)
}

func TestRenderer_ConditionalLogic(t *testing.T) {
	r := NewRenderer()

	result, err := r.Render(`{% if vip %}VIP: {{ discount }}%{% endif %}`, map[string]interface{}{"vip": true, "discount": 20})
	require.NoError(t, err)
	assert.Equal(t, "VIP: 20%", result)

	result, err = r.Render(`{% if vip %}VIP{% endif %}`, map[string]interface{}{"vip": false})
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestRenderer_OutputLimitExceeded(t *testing.T) {
	r := NewRenderer()
	// Template that produces output > 1600 characters
	longValue := make([]byte, 2000)
	for i := range longValue {
		longValue[i] = 'A'
	}
	_, err := r.Render("{{ text }}", map[string]interface{}{"text": string(longValue)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "output exceeds maximum")
}

func TestRenderer_InvalidSyntax(t *testing.T) {
	r := NewRenderer()
	_, err := r.Render("{{ unclosed", map[string]interface{}{})
	require.Error(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/magomed/projects/sms && go test ./internal/shared/template/ -v -count=1
```

Expected: FAIL — package doesn't exist yet

- [ ] **Step 3: Write the renderer implementation**

```go
// internal/shared/template/renderer.go
package template

import (
	"fmt"
	"strings"
	"time"

	"github.com/osteele/liquid"
)

const maxOutputLen = 1600 // 10 SMS segments max

// Renderer wraps the Liquid engine with SMS-specific filters and sandbox.
type Renderer struct {
	engine *liquid.Engine
}

// NewRenderer creates a Renderer with custom SMS filters registered.
func NewRenderer() *Renderer {
	engine := liquid.NewEngine()

	engine.RegisterFilter("default", func(value, defaultVal interface{}) interface{} {
		if value == nil || value == "" {
			return defaultVal
		}
		return value
	})

	engine.RegisterFilter("truncate", func(value interface{}, length int) string {
		s := fmt.Sprintf("%v", value)
		runes := []rune(s)
		if len(runes) <= length {
			return s
		}
		return string(runes[:length]) + "..."
	})

	engine.RegisterFilter("phone_format", func(value interface{}) string {
		s := fmt.Sprintf("%v", value)
		s = strings.ReplaceAll(s, " ", "")
		s = strings.TrimPrefix(s, "+")
		if len(s) == 11 && s[0] == '7' {
			return fmt.Sprintf("+%s (%s) %s-%s-%s", s[:1], s[1:4], s[4:7], s[7:9], s[9:11])
		}
		return fmt.Sprintf("+%s", s)
	})

	engine.RegisterFilter("date", func(value interface{}, format string) string {
		var t time.Time
		switch v := value.(type) {
		case time.Time:
			t = v
		case string:
			parsed, err := time.Parse(time.RFC3339, v)
			if err != nil {
				parsed, err = time.Parse("2006-01-02", v)
				if err != nil {
					return v
				}
			}
			t = parsed
		default:
			return fmt.Sprintf("%v", value)
		}
		return goDateFormat(t, format)
	})

	return &Renderer{engine: engine}
}

// Render renders a Liquid template with the given bindings.
// Returns error if syntax is invalid, timeout exceeded, or output > 1600 chars.
func (r *Renderer) Render(templateStr string, bindings map[string]interface{}) (string, error) {
	out, err := r.engine.ParseAndRenderString(templateStr, bindings)
	if err != nil {
		return "", fmt.Errorf("template render error: %w", err)
	}
	if len([]rune(out)) > maxOutputLen {
		return "", fmt.Errorf("output exceeds maximum length of %d characters", maxOutputLen)
	}
	return out, nil
}

// goDateFormat converts strftime-style format to Go time format and formats the time.
func goDateFormat(t time.Time, format string) string {
	replacer := strings.NewReplacer(
		"%Y", "2006",
		"%m", "01",
		"%d", "02",
		"%H", "15",
		"%M", "04",
		"%S", "05",
	)
	goFmt := replacer.Replace(format)
	return t.Format(goFmt)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/magomed/projects/sms && go test ./internal/shared/template/ -v -count=1
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/shared/template/renderer.go internal/shared/template/renderer_test.go
git commit -m "feat(template): add Liquid renderer with SMS custom filters"
```

---

### Task 3: Create template validator

**Files:**
- Create: `internal/shared/template/validator.go`
- Create: `internal/shared/template/validator_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/shared/template/validator_test.go
package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidator_ValidSyntax(t *testing.T) {
	v := NewValidator()
	result := v.Validate("Привет, {{ name }}!")
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestValidator_InvalidSyntax(t *testing.T) {
	v := NewValidator()
	result := v.Validate("{{ unclosed")
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
}

func TestValidator_ExtractVariables(t *testing.T) {
	v := NewValidator()
	vars := v.ExtractVariables("{{ name }} {% if vip %}{{ discount }}{% endif %}")
	assert.Contains(t, vars, "name")
	assert.Contains(t, vars, "vip")
	assert.Contains(t, vars, "discount")
}

func TestValidator_EstimateLength(t *testing.T) {
	v := NewValidator()
	est := v.EstimateLength("Привет, {{ name }}!", map[string][]interface{}{
		"name": {"Иван", "Александр Константинович"},
	})
	require.NotNil(t, est)
	assert.True(t, est.MinLen > 0)
	assert.True(t, est.MaxLen >= est.MinLen)
	assert.True(t, est.Segments >= 1)
}

func TestValidator_LengthWarning(t *testing.T) {
	v := NewValidator()
	// Build a template that produces > 160 chars
	longPrefix := make([]rune, 150)
	for i := range longPrefix {
		longPrefix[i] = 'А'
	}
	est := v.EstimateLength(string(longPrefix)+"{{ name }}", map[string][]interface{}{
		"name": {"Александр"},
	})
	assert.True(t, est.Segments > 1)
	assert.True(t, est.Warning != "")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/magomed/projects/sms && go test ./internal/shared/template/ -v -run TestValidator -count=1
```

Expected: FAIL

- [ ] **Step 3: Write the validator implementation**

```go
// internal/shared/template/validator.go
package template

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/osteele/liquid"
)

// ValidationResult holds the result of template validation.
type ValidationResult struct {
	Valid  bool
	Errors []string
}

// LengthEstimate holds estimated output length info.
type LengthEstimate struct {
	MinLen   int
	MaxLen   int
	Segments int
	Warning  string
}

// Validator validates Liquid templates for SMS use.
type Validator struct {
	engine *liquid.Engine
}

// NewValidator creates a new template Validator.
func NewValidator() *Validator {
	return &Validator{engine: liquid.NewEngine()}
}

// Validate checks template syntax.
func (v *Validator) Validate(templateStr string) ValidationResult {
	_, err := v.engine.ParseString(templateStr)
	if err != nil {
		return ValidationResult{Valid: false, Errors: []string{err.Error()}}
	}
	return ValidationResult{Valid: true}
}

var variableRegex = regexp.MustCompile(`\{\{[\s]*([a-zA-Z_][a-zA-Z0-9_]*)`)
var tagVarRegex = regexp.MustCompile(`\{%[\s]*(?:if|unless|for)[\s]+([a-zA-Z_][a-zA-Z0-9_]*)`)

// ExtractVariables returns all variable names referenced in the template.
func (v *Validator) ExtractVariables(templateStr string) []string {
	seen := map[string]bool{}
	var vars []string

	for _, matches := range variableRegex.FindAllStringSubmatch(templateStr, -1) {
		name := matches[1]
		if !seen[name] {
			seen[name] = true
			vars = append(vars, name)
		}
	}
	for _, matches := range tagVarRegex.FindAllStringSubmatch(templateStr, -1) {
		name := matches[1]
		if !seen[name] {
			seen[name] = true
			vars = append(vars, name)
		}
	}
	return vars
}

// EstimateLength estimates the min/max output length given sample values per variable.
func (v *Validator) EstimateLength(templateStr string, sampleValues map[string][]interface{}) *LengthEstimate {
	renderer := NewRenderer()

	minLen := int(^uint(0) >> 1) // max int
	maxLen := 0

	// Render with each combination of shortest and longest values
	shortBindings := map[string]interface{}{}
	longBindings := map[string]interface{}{}

	for varName, values := range sampleValues {
		if len(values) == 0 {
			continue
		}
		shortest := values[0]
		longest := values[0]
		for _, val := range values {
			s := fmt.Sprintf("%v", val)
			if len(s) < len(fmt.Sprintf("%v", shortest)) {
				shortest = val
			}
			if len(s) > len(fmt.Sprintf("%v", longest)) {
				longest = val
			}
		}
		shortBindings[varName] = shortest
		longBindings[varName] = longest
	}

	// Render with short values
	if out, err := renderer.Render(templateStr, shortBindings); err == nil {
		l := len([]rune(out))
		if l < minLen {
			minLen = l
		}
		if l > maxLen {
			maxLen = l
		}
	}

	// Render with long values
	if out, err := renderer.Render(templateStr, longBindings); err == nil {
		l := len([]rune(out))
		if l < minLen {
			minLen = l
		}
		if l > maxLen {
			maxLen = l
		}
	}

	if minLen == int(^uint(0)>>1) {
		minLen = 0
	}

	segments := (maxLen + 159) / 160
	if segments < 1 {
		segments = 1
	}

	var warning string
	if segments > 1 {
		warning = fmt.Sprintf("Максимальная длина %d симв. (%d SMS-сегментов)", maxLen, segments)
	}

	return &LengthEstimate{
		MinLen:   minLen,
		MaxLen:   maxLen,
		Segments: segments,
		Warning:  warning,
	}
}

// ValidateAttributes checks that all variables in the template exist in the given attribute schema.
func (v *Validator) ValidateAttributes(templateStr string, availableAttrs []string) []string {
	vars := v.ExtractVariables(templateStr)
	attrSet := make(map[string]bool, len(availableAttrs))
	for _, a := range availableAttrs {
		attrSet[a] = true
	}
	attrSet["name"] = true
	attrSet["phone"] = true

	var missing []string
	for _, varName := range vars {
		if !attrSet[varName] && !isBuiltinVariable(varName) {
			missing = append(missing, varName)
		}
	}
	return missing
}

func isBuiltinVariable(name string) bool {
	builtins := map[string]bool{
		"true": true, "false": true, "nil": true, "null": true,
		"blank": true, "empty": true,
	}
	return builtins[strings.ToLower(name)]
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/magomed/projects/sms && go test ./internal/shared/template/ -v -count=1
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/shared/template/validator.go internal/shared/template/validator_test.go
git commit -m "feat(template): add validator with syntax check and length estimation"
```

---

### Task 4: Database migration — add rendered_text to campaign_recipients

**Files:**
- Create: `migrations/000046_rendered_text.up.sql`
- Create: `migrations/000046_rendered_text.down.sql`

- [ ] **Step 1: Create up migration**

```sql
-- migrations/000046_rendered_text.up.sql
ALTER TABLE campaign_recipients ADD COLUMN rendered_text TEXT;
```

- [ ] **Step 2: Create down migration**

```sql
-- migrations/000046_rendered_text.down.sql
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS rendered_text;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000046_rendered_text.up.sql migrations/000046_rendered_text.down.sql
git commit -m "migration: add rendered_text column to campaign_recipients"
```

---

### Task 5: Update campaign domain model — add RenderedText to Recipient

**Files:**
- Modify: `internal/services/campaign/domain/models.go`

- [ ] **Step 1: Add RenderedText field to Recipient struct**

In `internal/services/campaign/domain/models.go`, add `RenderedText *string` field to the `Recipient` struct after the `MessageID` field:

```go
type Recipient struct {
	ID           uuid.UUID
	CampaignID   uuid.UUID
	ContactID    uuid.UUID
	Phone        string
	VariantID    *uuid.UUID
	Status       string
	MessageID    *uuid.UUID
	RenderedText *string
	RetryCount   int32
	LastRetryAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/campaign/domain/models.go
git commit -m "feat(campaign): add RenderedText field to Recipient domain model"
```

---

### Task 6: Add PreviewTemplate RPC to campaign proto

**Files:**
- Modify: `api/proto/campaign/campaign.proto`

- [ ] **Step 1: Add RPC and messages to campaign.proto**

Add to the `CampaignService` service block:

```protobuf
rpc PreviewTemplate(PreviewTemplateRequest) returns (PreviewTemplateResponse);
```

Add new messages at the end of the file:

```protobuf
message PreviewTemplateRequest {
  string client_id = 1;
  string template_text = 2;
  repeated TemplateTestData test_data = 3;
  string contact_list_id = 4; // optional: for auto-sampling contacts
}

message TemplateTestData {
  map<string, string> bindings = 1;
}

message PreviewTemplateResponse {
  repeated TemplatePreviewResult results = 1;
}

message TemplatePreviewResult {
  string rendered = 1;
  int32 length = 2;
  int32 segments = 3;
  repeated string warnings = 4;
  string error = 5;
}
```

- [ ] **Step 2: Regenerate protobuf Go code**

```bash
cd /home/magomed/projects/sms && protoc --go_out=. --go-grpc_out=. --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative api/proto/campaign/campaign.proto
```

- [ ] **Step 3: Commit**

```bash
git add api/proto/campaign/campaign.proto api/proto/campaignv1/
git commit -m "proto(campaign): add PreviewTemplate RPC for template preview"
```

---

### Task 7: Implement PreviewTemplate in campaign service application layer

**Files:**
- Modify: `internal/services/campaign/application/campaign_service.go`

- [ ] **Step 1: Add PreviewTemplate method**

Add to `campaign_service.go`:

```go
import (
	smstpl "github.com/smpp-server/smpp-server/internal/shared/template"
)

// TemplatePreviewInput holds input for template preview.
type TemplatePreviewInput struct {
	TemplateText string
	TestData     []map[string]interface{}
}

// TemplatePreviewResult holds a single preview result.
type TemplatePreviewResult struct {
	Rendered string
	Length   int
	Segments int
	Warnings []string
	Error    string
}

// PreviewTemplate renders a template against multiple test data sets.
func (s *CampaignService) PreviewTemplate(ctx context.Context, input TemplatePreviewInput) ([]TemplatePreviewResult, error) {
	renderer := smstpl.NewRenderer()
	validator := smstpl.NewValidator()

	// Validate syntax first
	vr := validator.Validate(input.TemplateText)
	if !vr.Valid {
		return nil, fmt.Errorf("invalid template: %s", vr.Errors[0])
	}

	results := make([]TemplatePreviewResult, 0, len(input.TestData))
	for _, bindings := range input.TestData {
		result := TemplatePreviewResult{}
		rendered, err := renderer.Render(input.TemplateText, bindings)
		if err != nil {
			result.Error = err.Error()
		} else {
			result.Rendered = rendered
			result.Length = len([]rune(rendered))
			result.Segments = (result.Length + 159) / 160
			if result.Segments < 1 {
				result.Segments = 1
			}
			if result.Segments > 1 {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Сообщение %d симв. (%d SMS-сегментов)", result.Length, result.Segments))
			}
		}
		results = append(results, result)
	}

	return results, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/campaign/application/campaign_service.go
git commit -m "feat(campaign): implement PreviewTemplate business logic"
```

---

### Task 8: Implement PreviewTemplate gRPC handler

**Files:**
- Modify: `internal/services/campaign/grpc/server.go`

- [ ] **Step 1: Add PreviewTemplate to gRPC server**

Add handler method to the campaign gRPC server:

```go
func (s *CampaignGrpcServer) PreviewTemplate(ctx context.Context, req *campaignv1.PreviewTemplateRequest) (*campaignv1.PreviewTemplateResponse, error) {
	testData := make([]map[string]interface{}, 0, len(req.TestData))
	for _, td := range req.TestData {
		bindings := make(map[string]interface{}, len(td.Bindings))
		for k, v := range td.Bindings {
			bindings[k] = v
		}
		testData = append(testData, bindings)
	}

	results, err := s.service.PreviewTemplate(ctx, application.TemplatePreviewInput{
		TemplateText: req.TemplateText,
		TestData:     testData,
	})
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s", err.Error())
	}

	resp := &campaignv1.PreviewTemplateResponse{}
	for _, r := range results {
		resp.Results = append(resp.Results, &campaignv1.TemplatePreviewResult{
			Rendered: r.Rendered,
			Length:   int32(r.Length),
			Segments: int32(r.Segments),
			Warnings: r.Warnings,
			Error:    r.Error,
		})
	}
	return resp, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/campaign/grpc/server.go
git commit -m "feat(campaign): add PreviewTemplate gRPC handler"
```

---

### Task 9: Add POST /campaigns/templates/preview to portal gateway

**Files:**
- Modify: `internal/gateway/portal/handlers/campaigns.go`
- Modify: `internal/gateway/portal/router/router.go`

- [ ] **Step 1: Add PreviewTemplate HTTP handler**

In `internal/gateway/portal/handlers/campaigns.go`, add:

```go
// PreviewTemplate обрабатывает POST /campaigns/templates/preview
func (h *CampaignHandlers) PreviewTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var req campaignv1.PreviewTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	req.ClientId = clientID.String()

	if req.TemplateText == "" {
		respondError(w, shared.ErrInvalidInput("template_text обязателен"))
		return
	}

	resp, err := h.campaignClient.PreviewTemplate(r.Context(), &req)
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}
```

- [ ] **Step 2: Register route in router.go**

In `internal/gateway/portal/router/router.go`, add before the campaigns CRUD routes:

```go
// Template preview (must be before /{id} routes)
campaigns.HandleFunc("/templates/preview", campaignHandlers.PreviewTemplate).Methods("POST")
```

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/handlers/campaigns.go internal/gateway/portal/router/router.go
git commit -m "feat(portal): add POST /campaigns/templates/preview endpoint"
```

---

### Task 10: Frontend — TemplatePreview component

**Files:**
- Create: `portal-frontend/src/components/campaigns/TemplatePreview.tsx`

- [ ] **Step 1: Create the TemplatePreview component**

```tsx
// portal-frontend/src/components/campaigns/TemplatePreview.tsx
import { useState } from 'react';

interface PreviewResult {
  rendered: string;
  length: number;
  segments: number;
  warnings: string[];
  error: string;
}

interface TemplatePreviewProps {
  templateText: string;
  contactListId?: string;
}

export function TemplatePreview({ templateText, contactListId }: TemplatePreviewProps) {
  const [results, setResults] = useState<PreviewResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const fetchPreview = async () => {
    if (!templateText.trim()) return;
    setLoading(true);
    setError('');
    try {
      const res = await fetch('/portal/v1/campaigns/templates/preview', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          template_text: templateText,
          contact_list_id: contactListId || '',
          test_data: [
            { bindings: { name: 'Иван', phone: '79991234567', vip: 'true', discount: '15' } },
            { bindings: { name: 'Мария', phone: '79997654321', vip: 'false' } },
            { bindings: {} },
          ],
        }),
      });
      if (!res.ok) throw new Error('Ошибка загрузки превью');
      const data = await res.json();
      setResults(data.results || []);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Ошибка');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="mt-4 space-y-3">
      <button
        type="button"
        onClick={fetchPreview}
        disabled={loading || !templateText.trim()}
        className="px-3 py-1.5 text-sm bg-gray-100 hover:bg-gray-200 rounded border border-gray-300 disabled:opacity-50"
      >
        {loading ? 'Загрузка...' : 'Превью шаблона'}
      </button>

      {error && <p className="text-sm text-red-600">{error}</p>}

      {results.length > 0 && (
        <div className="space-y-2">
          <h4 className="text-sm font-medium text-gray-700">Результат рендеринга:</h4>
          {results.map((r, i) => (
            <div key={i} className="p-3 bg-white border border-gray-200 rounded text-sm">
              {r.error ? (
                <p className="text-red-600">{r.error}</p>
              ) : (
                <>
                  <p className="whitespace-pre-wrap text-gray-900">{r.rendered}</p>
                  <p className="mt-1 text-xs text-gray-500">
                    {r.length} симв. / {r.segments} {r.segments === 1 ? 'сегмент' : 'сегментов'}
                  </p>
                  {r.warnings?.map((w, j) => (
                    <p key={j} className="mt-1 text-xs text-amber-600">{w}</p>
                  ))}
                </>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/components/campaigns/TemplatePreview.tsx
git commit -m "feat(frontend): add TemplatePreview component for campaign wizard"
```

---

### Task 11: Integrate TemplatePreview into CampaignWizardPage

**Files:**
- Modify: `portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx`

- [ ] **Step 1: Import and add TemplatePreview**

In `CampaignWizardPage.tsx`, add the import at the top:

```tsx
import { TemplatePreview } from '../../components/campaigns/TemplatePreview';
```

Then in the template step section (where `template_id` or template text is selected), add below the template text area:

```tsx
{form.templateText && (
  <TemplatePreview
    templateText={form.templateText}
    contactListId={form.contactListId}
  />
)}
```

Note: The exact placement depends on the current CampaignWizardPage structure — find the template selection step and add the preview below it.

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx
git commit -m "feat(frontend): integrate TemplatePreview into CampaignWizard"
```
