# Full Test Coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Achieve comprehensive unit and functional test coverage for the entire SMS platform, fixing all broken tests and adding tests for every untested package — level by level from domain up to integration.

**Architecture:** Bottom-up approach: domain → application → gRPC → gateway/middleware → pipeline → infrastructure. Each level builds on the previous, ensuring mocks and interfaces are available before testing higher layers.

**Tech Stack:** Go 1.24+ testing, testify/assert + testify/mock + testify/require, existing testutil package.

---

## Level 0: Fix Broken Tests

### Task 1: Fix APIKeyRepository mock (auth service)

**Files:**
- Modify: `internal/services/auth/mocks/mock_api_key_repository.go`

The mock is missing the `Update` method required by the `APIKeyRepository` interface defined in `internal/services/auth/application/ports.go`.

- [ ] **Step 1: Add missing Update method to mock**

```go
// Add after the ListByUserID method (line 43) in mock_api_key_repository.go

func (m *MockAPIKeyRepository) Update(ctx context.Context, apiKey *domain.APIKey) error {
	args := m.Called(ctx, apiKey)
	return args.Error(0)
}
```

- [ ] **Step 2: Run auth service tests**

Run: `go test ./internal/services/auth/...`
Expected: All tests PASS (application and grpc packages compile and pass)

- [ ] **Step 3: Commit**

```bash
git add internal/services/auth/mocks/mock_api_key_repository.go
git commit -m "fix(auth): add missing Update method to MockAPIKeyRepository"
```

---

### Task 2: Fix AuthServiceClient mocks in gateway tests

**Files:**
- Modify: `internal/gateway/admin/middleware/auth_test.go`
- Modify: `internal/gateway/client/middleware/auth_test.go`
- Modify: `internal/gateway/portal/handlers/auth_test.go`

All three files have the same issue: `mockAuthServiceClient` / `mockAuthClient` lacks `ChangePassword` and `UpdateAPIKey` methods.

- [ ] **Step 1: Add missing methods to admin gateway mock**

Add after the last mock method (before the test functions) in `internal/gateway/admin/middleware/auth_test.go`:

```go
func (m *mockAuthServiceClient) UpdateAPIKey(ctx context.Context, in *authv1.UpdateAPIKeyRequest, opts ...grpc.CallOption) (*authv1.UpdateAPIKeyResponse, error) {
	return nil, nil
}

func (m *mockAuthServiceClient) ChangePassword(ctx context.Context, in *authv1.ChangePasswordRequest, opts ...grpc.CallOption) (*authv1.ChangePasswordResponse, error) {
	return nil, nil
}
```

- [ ] **Step 2: Add same methods to client gateway mock**

Add identical methods to the `mockAuthServiceClient` in `internal/gateway/client/middleware/auth_test.go`.

- [ ] **Step 3: Add same methods to portal handlers mock**

Add to the `mockAuthClient` in `internal/gateway/portal/handlers/auth_test.go`:

```go
func (m *mockAuthClient) UpdateAPIKey(ctx context.Context, in *authv1.UpdateAPIKeyRequest, opts ...grpc.CallOption) (*authv1.UpdateAPIKeyResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) ChangePassword(ctx context.Context, in *authv1.ChangePasswordRequest, opts ...grpc.CallOption) (*authv1.ChangePasswordResponse, error) {
	return nil, nil
}
```

Also add to the `mockAuthClient` in `internal/gateway/portal/handlers/api_keys_test.go` (same struct, but verify the mock type — it may share the same definition via the auth_test.go file in the same package).

- [ ] **Step 4: Run all gateway tests**

Run: `go test ./internal/gateway/admin/middleware/... ./internal/gateway/client/middleware/... ./internal/gateway/portal/handlers/...`
Expected: admin and client middleware compile and pass. Portal handlers compile and pass.

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/admin/middleware/auth_test.go internal/gateway/client/middleware/auth_test.go internal/gateway/portal/handlers/auth_test.go
git commit -m "fix(gateway): add ChangePassword and UpdateAPIKey to auth mock interfaces"
```

---

### Task 3: Fix RoutingServiceClient mock in client gateway

**Files:**
- Modify: `internal/gateway/client/handlers/lookup_test.go`

The `mockRoutingClientForLookup` is missing `AssignProviderToClient`, `RevokeProviderFromClient`, `ListClientProviders`, `UpdateClientProvider`, `ShareProviderWithChild`, `RevokeSharedProvider`, `CreateClientRoute`, and other client-provider methods added to the `RoutingServiceClient` interface.

- [ ] **Step 1: Read the full RoutingServiceClient interface**

Run: `grep -n "func.*RoutingServiceClient" internal/gateway/client/handlers/lookup_test.go | wc -l` and compare with:
Run: `grep -c "opts ...grpc.CallOption" api/proto/routingv1/routing_grpc.pb.go`

- [ ] **Step 2: Add all missing stub methods**

Add after `RouteMessageWithHLR` (line 133) in `internal/gateway/client/handlers/lookup_test.go`:

```go
func (m *mockRoutingClientForLookup) AssignProviderToClient(ctx context.Context, in *routingv1.AssignProviderRequest, opts ...grpc.CallOption) (*routingv1.ClientProviderProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) RevokeProviderFromClient(ctx context.Context, in *routingv1.RevokeProviderRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) ListClientProviders(ctx context.Context, in *routingv1.ListClientProvidersRequest, opts ...grpc.CallOption) (*routingv1.ListClientProvidersResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) UpdateClientProvider(ctx context.Context, in *routingv1.UpdateClientProviderRequest, opts ...grpc.CallOption) (*routingv1.ClientProviderProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) ShareProviderWithChild(ctx context.Context, in *routingv1.ShareProviderRequest, opts ...grpc.CallOption) (*routingv1.ClientProviderProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) RevokeSharedProvider(ctx context.Context, in *routingv1.RevokeSharedProviderRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) CreateClientRoute(ctx context.Context, in *routingv1.CreateClientRouteRequest, opts ...grpc.CallOption) (*routingv1.ClientRouteProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) ListClientRoutes(ctx context.Context, in *routingv1.ListClientRoutesRequest, opts ...grpc.CallOption) (*routingv1.ListClientRoutesResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) DeleteClientRoute(ctx context.Context, in *routingv1.DeleteClientRouteRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
```

Add import for `"google.golang.org/protobuf/types/known/emptypb"` if not already present.

- [ ] **Step 3: Run client handler tests**

Run: `go test ./internal/gateway/client/handlers/...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/client/handlers/lookup_test.go
git commit -m "fix(gateway): add missing routing client methods to lookup test mock"
```

---

### Task 4: Fix API auth middleware (implementation + tests)

**Files:**
- Modify: `internal/api/middleware/auth.go`
- Modify: `internal/api/middleware/auth_test.go`

The middleware currently only supports load-test bypass mode and returns 401 otherwise. The tests expect full API key authentication. Need to implement the real auth logic.

- [ ] **Step 1: Implement full auth middleware**

Replace the `AuthMiddleware` function in `internal/api/middleware/auth.go`:

```go
func AuthMiddleware(clientRepo ClientRepository, cfg *config.AuthConfig) func(http.Handler) http.Handler {
	dummyID := uuid.MustParse("c0000000-0000-0000-0000-000000000001")
	apiKeyHeader := "X-API-Key"
	if cfg != nil && cfg.APIKeyHeader != "" {
		apiKeyHeader = cfg.APIKeyHeader
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip health/metrics endpoints
			if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			// Load test mode bypass
			if isLoadTestMode() {
				ctx := context.WithValue(r.Context(), ClientIDKey, dummyID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Extract API key from header or Authorization bearer
			apiKey := r.Header.Get(apiKeyHeader)
			if apiKey == "" {
				auth := r.Header.Get("Authorization")
				if strings.HasPrefix(auth, "Bearer ") {
					apiKey = strings.TrimPrefix(auth, "Bearer ")
				}
			}

			if apiKey == "" {
				respondError(w, &shared.AppError{
					HTTPStatus: http.StatusUnauthorized,
					Code:       "MISSING_API_KEY",
					Message:    "API key is required",
				})
				return
			}

			// Lookup client by API key
			client, err := clientRepo.GetByAPIKey(r.Context(), apiKey)
			if err != nil {
				if err.Error() == "not found" || err == shared.ErrNotFound {
					respondError(w, &shared.AppError{
						HTTPStatus: http.StatusUnauthorized,
						Code:       "INVALID_API_KEY",
						Message:    "invalid API key",
					})
					return
				}
				respondError(w, &shared.AppError{
					HTTPStatus: http.StatusInternalServerError,
					Code:       "AUTH_ERROR",
					Message:    "authentication error",
				})
				return
			}

			if !client.Active {
				respondError(w, &shared.AppError{
					HTTPStatus: http.StatusForbidden,
					Code:       "CLIENT_INACTIVE",
					Message:    "client account is inactive",
				})
				return
			}

			ctx := context.WithValue(r.Context(), ClientIDKey, client.ID)
			ctx = context.WithValue(ctx, ClientKey, client)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

- [ ] **Step 2: Check import for storage.ErrNotFound in test**

The test uses `storage.ErrNotFound`. Check that the `ClientRepository.GetByAPIKey` in `testutil.MockClientRepository` returns proper errors. The middleware should handle `ErrNotFound` from the `storage` package. Update the error check in the middleware to match:

```go
import "github.com/smpp-server/smpp-server/internal/storage"

// In the error handling:
if errors.Is(err, storage.ErrNotFound) {
```

Add `"errors"` to imports.

- [ ] **Step 3: Run API middleware tests**

Run: `go test ./internal/api/middleware/...`
Expected: All tests PASS

- [ ] **Step 4: Commit**

```bash
git add internal/api/middleware/auth.go
git commit -m "fix(middleware): implement full API key auth with client lookup"
```

---

### Task 5: Fix CSRF middleware (add public path exemption)

**Files:**
- Modify: `internal/gateway/portal/middleware/csrf.go`

The test `PublicPath_POST_NoCSRFRequired` expects POST to `/portal/v1/auth/login` to skip CSRF. The middleware needs path-based exemptions for auth endpoints.

- [ ] **Step 1: Add public path exemption to CSRF middleware**

Replace the `CSRFMiddleware` function:

```go
// publicCSRFPaths are paths that skip CSRF validation even for mutating methods.
var publicCSRFPaths = []string{
	"/portal/v1/auth/login",
	"/portal/v1/auth/register",
	"/portal/v1/auth/password-reset/request",
	"/portal/v1/auth/password-reset/confirm",
}

func isPublicCSRFPath(path string) bool {
	for _, p := range publicCSRFPaths {
		if path == p {
			return true
		}
	}
	return false
}

func CSRFMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			if isPublicCSRFPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie("csrf_token")
			if err != nil || cookie.Value == "" {
				respondCSRFError(w)
				return
			}

			headerToken := r.Header.Get("X-CSRF-Token")
			if headerToken == "" || subtle.ConstantTimeCompare([]byte(headerToken), []byte(cookie.Value)) != 1 {
				respondCSRFError(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 2: Run CSRF middleware tests**

Run: `go test ./internal/gateway/portal/middleware/... -run TestCSRFMiddleware -v`
Expected: All subtests PASS including `PublicPath_POST_NoCSRFRequired`

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/middleware/csrf.go
git commit -m "fix(portal): add public path exemption to CSRF middleware"
```

- [ ] **Step 4: Run ALL tests to verify Level 0 fixes**

Run: `go test ./internal/... 2>&1 | grep -E "^(FAIL|ok)" | sort`
Expected: No FAIL lines remain (all 8 broken packages fixed)

- [ ] **Step 5: Commit level 0 complete**

If any remaining issues, fix them. Then:
```bash
git add -A && git commit -m "fix: resolve all broken test compilations and failures"
```

---

## Level 1: Domain Layer Tests

### Task 6: Provider domain tests

**Files:**
- Create: `internal/services/provider/domain/provider_test.go`

- [ ] **Step 1: Write provider validation tests**

```go
package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvider_Validate(t *testing.T) {
	validProvider := func() *Provider {
		return &Provider{
			Name:           "test-provider",
			Host:           "smpp.example.com",
			Port:           2775,
			SystemID:       "sysid",
			Password:       "pass",
			BindType:       BindTypeTransceiver,
			MaxConnections: 5,
		}
	}

	t.Run("valid provider", func(t *testing.T) {
		p := validProvider()
		assert.NoError(t, p.Validate())
	})

	t.Run("empty name", func(t *testing.T) {
		p := validProvider()
		p.Name = ""
		assert.ErrorIs(t, p.Validate(), ErrProviderNameRequired)
	})

	t.Run("empty host", func(t *testing.T) {
		p := validProvider()
		p.Host = ""
		assert.ErrorIs(t, p.Validate(), ErrProviderHostRequired)
	})

	t.Run("invalid port zero", func(t *testing.T) {
		p := validProvider()
		p.Port = 0
		assert.ErrorIs(t, p.Validate(), ErrProviderPortInvalid)
	})

	t.Run("invalid port exceeds 65535", func(t *testing.T) {
		p := validProvider()
		p.Port = 70000
		assert.ErrorIs(t, p.Validate(), ErrProviderPortInvalid)
	})

	t.Run("negative port", func(t *testing.T) {
		p := validProvider()
		p.Port = -1
		assert.ErrorIs(t, p.Validate(), ErrProviderPortInvalid)
	})

	t.Run("empty system_id", func(t *testing.T) {
		p := validProvider()
		p.SystemID = ""
		assert.ErrorIs(t, p.Validate(), ErrProviderSystemIDRequired)
	})

	t.Run("empty password", func(t *testing.T) {
		p := validProvider()
		p.Password = ""
		assert.ErrorIs(t, p.Validate(), ErrProviderPasswordRequired)
	})

	t.Run("negative max connections", func(t *testing.T) {
		p := validProvider()
		p.MaxConnections = -1
		assert.ErrorIs(t, p.Validate(), ErrProviderMaxConnectionsInvalid)
	})

	t.Run("zero max connections is valid", func(t *testing.T) {
		p := validProvider()
		p.MaxConnections = 0
		assert.NoError(t, p.Validate())
	})

	t.Run("invalid bind type", func(t *testing.T) {
		p := validProvider()
		p.BindType = "invalid"
		assert.ErrorIs(t, p.Validate(), ErrProviderBindTypeInvalid)
	})
}

func TestProvider_IsValidBindType(t *testing.T) {
	tests := []struct {
		bindType BindType
		valid    bool
	}{
		{BindTypeTransceiver, true},
		{BindTypeTransmitter, true},
		{BindTypeReceiver, true},
		{"", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.bindType), func(t *testing.T) {
			p := &Provider{BindType: tt.bindType}
			assert.Equal(t, tt.valid, p.IsValidBindType())
		})
	}
}

func TestProvider_IsActive(t *testing.T) {
	t.Run("active", func(t *testing.T) {
		p := &Provider{Active: true}
		assert.True(t, p.IsActive())
	})
	t.Run("inactive", func(t *testing.T) {
		p := &Provider{Active: false}
		assert.False(t, p.IsActive())
	})
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/services/provider/domain/... -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/services/provider/domain/provider_test.go
git commit -m "test(provider): add domain validation tests"
```

---

### Task 7: Template domain tests

**Files:**
- Create: `internal/services/template/domain/models_test.go`

- [ ] **Step 1: Write template domain tests**

```go
package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractVariables(t *testing.T) {
	t.Run("single variable", func(t *testing.T) {
		vars := ExtractVariables("Hello {{name}}")
		assert.Equal(t, []string{"name"}, vars)
	})

	t.Run("multiple variables", func(t *testing.T) {
		vars := ExtractVariables("{{greeting}} {{name}}, your code is {{code}}")
		assert.Equal(t, []string{"greeting", "name", "code"}, vars)
	})

	t.Run("duplicate variables", func(t *testing.T) {
		vars := ExtractVariables("{{name}} and {{name}} again")
		assert.Equal(t, []string{"name"}, vars)
	})

	t.Run("no variables", func(t *testing.T) {
		vars := ExtractVariables("plain text message")
		assert.Nil(t, vars)
	})

	t.Run("empty string", func(t *testing.T) {
		vars := ExtractVariables("")
		assert.Nil(t, vars)
	})

	t.Run("nested braces ignored", func(t *testing.T) {
		vars := ExtractVariables("{{{name}}}")
		assert.Equal(t, []string{"name"}, vars)
	})

	t.Run("underscored variable names", func(t *testing.T) {
		vars := ExtractVariables("{{first_name}} {{last_name}}")
		assert.Equal(t, []string{"first_name", "last_name"}, vars)
	})
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/services/template/domain/... -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/services/template/domain/models_test.go
git commit -m "test(template): add domain ExtractVariables tests"
```

---

### Task 8: Webhook domain tests

**Files:**
- Create: `internal/services/webhook/domain/models_test.go`

- [ ] **Step 1: Write webhook domain tests**

```go
package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSMPPStatToEventType(t *testing.T) {
	tests := []struct {
		stat     string
		expected string
		exists   bool
	}{
		{"DELIVRD", "delivered", true},
		{"UNDELIV", "failed", true},
		{"EXPIRED", "expired", true},
		{"DELETED", "failed", true},
		{"REJECTD", "rejected", true},
		{"UNKNOWN", "failed", true},
		{"INVALID", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.stat, func(t *testing.T) {
			eventType, ok := SMPPStatToEventType[tt.stat]
			assert.Equal(t, tt.exists, ok)
			if ok {
				assert.Equal(t, tt.expected, eventType)
			}
		})
	}
}

func TestValidEventTypes(t *testing.T) {
	valid := []string{"delivered", "failed", "expired", "rejected"}
	for _, et := range valid {
		t.Run(et+"_valid", func(t *testing.T) {
			assert.True(t, ValidEventTypes[et])
		})
	}

	invalid := []string{"sent", "pending", "queued", ""}
	for _, et := range invalid {
		t.Run(et+"_invalid", func(t *testing.T) {
			assert.False(t, ValidEventTypes[et])
		})
	}
}

func TestMaxSubscriptionsPerClient(t *testing.T) {
	assert.Equal(t, 10, MaxSubscriptionsPerClient)
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/services/webhook/domain/... -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/services/webhook/domain/models_test.go
git commit -m "test(webhook): add domain model tests"
```

---

### Task 9: Campaign domain tests

**Files:**
- Create: `internal/services/campaign/domain/models_test.go`

- [ ] **Step 1: Write campaign domain tests**

```go
package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCampaignStatusConstants(t *testing.T) {
	assert.Equal(t, "draft", StatusDraft)
	assert.Equal(t, "scheduled", StatusScheduled)
	assert.Equal(t, "materializing", StatusMaterializing)
	assert.Equal(t, "running", StatusRunning)
	assert.Equal(t, "paused", StatusPaused)
	assert.Equal(t, "completed", StatusCompleted)
	assert.Equal(t, "cancelled", StatusCancelled)
}

func TestCampaignErrors(t *testing.T) {
	// Verify sentinel errors are distinct
	errors := []error{
		ErrCampaignNotFound,
		ErrVariantNotFound,
		ErrInvalidCampaignStatus,
		ErrCampaignNotDraft,
		ErrCampaignNotRunning,
		ErrCampaignNotPaused,
		ErrVariantPercentageSum,
		ErrTooFewVariants,
		ErrTooManyVariants,
		ErrWinnerAlreadySelected,
		ErrNoFailedRecipients,
	}

	for i, e1 := range errors {
		for j, e2 := range errors {
			if i != j {
				assert.NotEqual(t, e1.Error(), e2.Error(), "errors %d and %d should be distinct", i, j)
			}
		}
	}
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/services/campaign/domain/... -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/services/campaign/domain/models_test.go
git commit -m "test(campaign): add domain model and error tests"
```

---

### Task 10: Contact domain tests

**Files:**
- Create: `internal/services/contact/domain/segment_test.go`
- Create: `internal/services/contact/domain/models_test.go`

- [ ] **Step 1: Write contact domain error tests**

```go
// internal/services/contact/domain/models_test.go
package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContactErrors(t *testing.T) {
	assert.NotNil(t, ErrContactListNotFound)
	assert.NotNil(t, ErrContactNotFound)
	assert.NotNil(t, ErrImportNotFound)
	assert.NotNil(t, ErrDuplicatePhone)
	assert.NotNil(t, ErrInvalidPhone)
	assert.NotNil(t, ErrInvalidSegmentRules)
	assert.NotNil(t, ErrSegmentDepthExceeded)
	assert.NotNil(t, ErrImportAlreadyStarted)
}

func TestImportStatuses(t *testing.T) {
	assert.Equal(t, "pending", ImportStatusPending)
	assert.Equal(t, "processing", ImportStatusProcessing)
	assert.Equal(t, "completed", ImportStatusCompleted)
	assert.Equal(t, "failed", ImportStatusFailed)
}
```

- [ ] **Step 2: Read segment.go for SegmentToSQL and write segment tests**

Check if `SegmentToSQL()` exists in `internal/services/contact/domain/segment.go` and test the SQL generation logic. Read the full file to see what exported functions/methods exist.

```go
// internal/services/contact/domain/segment_test.go
package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSegmentErrors(t *testing.T) {
	assert.NotNil(t, ErrSegmentNotFound)
	assert.Contains(t, ErrSegmentNotFound.Error(), "segment not found")
}
```

Note: If `SegmentToSQL` exists with testable logic, add tests for it with various rule combinations (eq, neq, contains, nested groups). Read the actual file first.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/services/contact/domain/... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/services/contact/domain/
git commit -m "test(contact): add domain model and segment tests"
```

---

## Level 2: Application Layer Tests

### Task 11: Provider application service tests

**Files:**
- Create: `internal/services/provider/application/provider_service_test.go`

- [ ] **Step 1: Write provider service unit tests**

```go
package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/provider/domain"
	"github.com/smpp-server/smpp-server/internal/services/provider/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func validProvider() *domain.Provider {
	return &domain.Provider{
		Name:           "test-provider",
		Host:           "smpp.example.com",
		Port:           2775,
		SystemID:       "sysid",
		Password:       "pass",
		BindType:       domain.BindTypeTransceiver,
		MaxConnections: 5,
	}
}

func TestProviderService_CreateProvider(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := new(mocks.MockProviderRepository)
		svc := NewProviderService(repo)

		p := validProvider()
		repo.On("GetByName", mock.Anything, p.Name).Return(nil, domain.ErrProviderNotFound)
		repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Provider")).Return(nil)

		err := svc.CreateProvider(context.Background(), p)

		assert.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, p.ID)
		assert.False(t, p.CreatedAt.IsZero())
		repo.AssertExpectations(t)
	})

	t.Run("validation error", func(t *testing.T) {
		repo := new(mocks.MockProviderRepository)
		svc := NewProviderService(repo)

		p := &domain.Provider{} // empty — will fail validation
		err := svc.CreateProvider(context.Background(), p)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "валидация")
	})

	t.Run("duplicate name", func(t *testing.T) {
		repo := new(mocks.MockProviderRepository)
		svc := NewProviderService(repo)

		p := validProvider()
		existing := &domain.Provider{ID: uuid.New(), Name: p.Name}
		repo.On("GetByName", mock.Anything, p.Name).Return(existing, nil)

		err := svc.CreateProvider(context.Background(), p)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "уже существует")
	})
}

func TestProviderService_GetProvider(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo := new(mocks.MockProviderRepository)
		svc := NewProviderService(repo)

		id := uuid.New()
		expected := &domain.Provider{ID: id, Name: "test"}
		repo.On("GetByID", mock.Anything, id).Return(expected, nil)

		result, err := svc.GetProvider(context.Background(), id)

		require.NoError(t, err)
		assert.Equal(t, expected.Name, result.Name)
	})

	t.Run("not found", func(t *testing.T) {
		repo := new(mocks.MockProviderRepository)
		svc := NewProviderService(repo)

		id := uuid.New()
		repo.On("GetByID", mock.Anything, id).Return(nil, domain.ErrProviderNotFound)

		_, err := svc.GetProvider(context.Background(), id)

		assert.Error(t, err)
	})
}

func TestProviderService_ListProviders(t *testing.T) {
	t.Run("defaults for invalid params", func(t *testing.T) {
		repo := new(mocks.MockProviderRepository)
		svc := NewProviderService(repo)

		repo.On("List", mock.Anything, true, 100, 0).Return([]*domain.Provider{}, 0, nil)

		_, total, err := svc.ListProviders(context.Background(), true, -1, -5)

		assert.NoError(t, err)
		assert.Equal(t, 0, total)
		repo.AssertCalled(t, "List", mock.Anything, true, 100, 0)
	})

	t.Run("caps limit at 1000", func(t *testing.T) {
		repo := new(mocks.MockProviderRepository)
		svc := NewProviderService(repo)

		repo.On("List", mock.Anything, false, 1000, 0).Return([]*domain.Provider{}, 0, nil)

		svc.ListProviders(context.Background(), false, 5000, 0)

		repo.AssertCalled(t, "List", mock.Anything, false, 1000, 0)
	})
}

func TestProviderService_DeleteProvider(t *testing.T) {
	t.Run("soft delete sets inactive", func(t *testing.T) {
		repo := new(mocks.MockProviderRepository)
		svc := NewProviderService(repo)

		id := uuid.New()
		existing := &domain.Provider{ID: id, Name: "test", Active: true, Host: "h", Port: 2775, SystemID: "s", Password: "p", BindType: domain.BindTypeTransceiver}
		repo.On("GetByID", mock.Anything, id).Return(existing, nil)
		repo.On("Update", mock.Anything, mock.MatchedBy(func(p *domain.Provider) bool {
			return !p.Active
		})).Return(nil)

		err := svc.DeleteProvider(context.Background(), id)

		assert.NoError(t, err)
		repo.AssertExpectations(t)
	})
}

func TestProviderService_UpdateProvider(t *testing.T) {
	t.Run("updates fields", func(t *testing.T) {
		repo := new(mocks.MockProviderRepository)
		svc := NewProviderService(repo)

		id := uuid.New()
		existing := &domain.Provider{
			ID: id, Name: "old", Host: "old.com", Port: 2775,
			SystemID: "sys", Password: "pass", BindType: domain.BindTypeTransceiver,
		}
		repo.On("GetByID", mock.Anything, id).Return(existing, nil)
		repo.On("GetByName", mock.Anything, "new-name").Return(nil, domain.ErrProviderNotFound)
		repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Provider")).Return(nil)

		updates := &domain.Provider{Name: "new-name", Active: true}
		err := svc.UpdateProvider(context.Background(), id, updates)

		assert.NoError(t, err)
		repo.AssertExpectations(t)
	})
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/services/provider/application/... -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/services/provider/application/provider_service_test.go
git commit -m "test(provider): add application service unit tests"
```

---

### Task 12: Template application — introduce interfaces for testability

**Files:**
- Create: `internal/services/template/application/ports.go`
- Modify: `internal/services/template/application/template_service.go`
- Create: `internal/services/template/mocks/mocks.go`
- Create: `internal/services/template/application/template_service_test.go`

The TemplateService uses concrete repository types. To unit-test it, we need interfaces.

- [ ] **Step 1: Create ports.go with repository interfaces**

```go
// internal/services/template/application/ports.go
package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
)

type TemplateRepository interface {
	Create(ctx context.Context, tmpl *domain.Template) (*domain.Template, error)
	GetByID(ctx context.Context, id, clientID uuid.UUID) (*domain.Template, error)
	GetByIDAdmin(ctx context.Context, id uuid.UUID) (*domain.Template, error)
	ListByClientID(ctx context.Context, clientID uuid.UUID, status string, limit, offset int) ([]*domain.Template, int, error)
	Update(ctx context.Context, tmpl *domain.Template) (*domain.Template, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status, reason string) (*domain.Template, error)
	Delete(ctx context.Context, id, clientID uuid.UUID) error
}

type AuditRepository interface {
	Create(ctx context.Context, entry *domain.AuditEntry) error
	ListByTemplateID(ctx context.Context, templateID uuid.UUID, limit, offset int) ([]*domain.AuditEntry, int, error)
}
```

- [ ] **Step 2: Refactor TemplateService to use interfaces**

In `internal/services/template/application/template_service.go`, change the struct fields:

```go
type TemplateService struct {
	templateRepo TemplateRepository
	auditRepo    AuditRepository
	logger       zerolog.Logger
}

func NewTemplateService(templateRepo TemplateRepository, auditRepo AuditRepository) *TemplateService {
	return &TemplateService{
		templateRepo: templateRepo,
		auditRepo:    auditRepo,
		logger:       log.With().Str("component", "template-service").Logger(),
	}
}
```

Remove the import of `"github.com/smpp-server/smpp-server/internal/services/template/infrastructure/repository"`.

- [ ] **Step 3: Verify compilation**

Run: `go build ./internal/services/template/...`
Expected: Build succeeds (infrastructure/repository types implement the new interfaces)

- [ ] **Step 4: Create mock implementations**

```go
// internal/services/template/mocks/mocks.go
package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
	"github.com/stretchr/testify/mock"
)

type MockTemplateRepository struct {
	mock.Mock
}

func (m *MockTemplateRepository) Create(ctx context.Context, tmpl *domain.Template) (*domain.Template, error) {
	args := m.Called(ctx, tmpl)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Template), args.Error(1)
}

func (m *MockTemplateRepository) GetByID(ctx context.Context, id, clientID uuid.UUID) (*domain.Template, error) {
	args := m.Called(ctx, id, clientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Template), args.Error(1)
}

func (m *MockTemplateRepository) GetByIDAdmin(ctx context.Context, id uuid.UUID) (*domain.Template, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Template), args.Error(1)
}

func (m *MockTemplateRepository) ListByClientID(ctx context.Context, clientID uuid.UUID, status string, limit, offset int) ([]*domain.Template, int, error) {
	args := m.Called(ctx, clientID, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Template), args.Int(1), args.Error(2)
}

func (m *MockTemplateRepository) Update(ctx context.Context, tmpl *domain.Template) (*domain.Template, error) {
	args := m.Called(ctx, tmpl)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Template), args.Error(1)
}

func (m *MockTemplateRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status, reason string) (*domain.Template, error) {
	args := m.Called(ctx, id, status, reason)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Template), args.Error(1)
}

func (m *MockTemplateRepository) Delete(ctx context.Context, id, clientID uuid.UUID) error {
	args := m.Called(ctx, id, clientID)
	return args.Error(0)
}

type MockAuditRepository struct {
	mock.Mock
}

func (m *MockAuditRepository) Create(ctx context.Context, entry *domain.AuditEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockAuditRepository) ListByTemplateID(ctx context.Context, templateID uuid.UUID, limit, offset int) ([]*domain.AuditEntry, int, error) {
	args := m.Called(ctx, templateID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.AuditEntry), args.Int(1), args.Error(2)
}
```

- [ ] **Step 5: Write template service tests**

```go
// internal/services/template/application/template_service_test.go
package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
	"github.com/smpp-server/smpp-server/internal/services/template/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTemplateService_CreateTemplate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tmplRepo := new(mocks.MockTemplateRepository)
		auditRepo := new(mocks.MockAuditRepository)
		svc := NewTemplateService(tmplRepo, auditRepo)

		clientID := uuid.New()
		created := &domain.Template{ID: uuid.New(), ClientID: clientID, Name: "test", Body: "Hello {{name}}", Status: domain.StatusDraft}

		tmplRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Template")).Return(created, nil)
		auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditEntry")).Return(nil)

		result, err := svc.CreateTemplate(context.Background(), clientID, "test", "Hello {{name}}")

		require.NoError(t, err)
		assert.Equal(t, "test", result.Name)
		tmplRepo.AssertExpectations(t)
	})

	t.Run("empty name", func(t *testing.T) {
		svc := NewTemplateService(new(mocks.MockTemplateRepository), new(mocks.MockAuditRepository))

		_, err := svc.CreateTemplate(context.Background(), uuid.New(), "", "body")
		assert.ErrorIs(t, err, domain.ErrInvalidTemplateName)
	})

	t.Run("empty body", func(t *testing.T) {
		svc := NewTemplateService(new(mocks.MockTemplateRepository), new(mocks.MockAuditRepository))

		_, err := svc.CreateTemplate(context.Background(), uuid.New(), "name", "")
		assert.ErrorIs(t, err, domain.ErrInvalidTemplateBody)
	})
}

func TestTemplateService_RenderTemplate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tmplRepo := new(mocks.MockTemplateRepository)
		auditRepo := new(mocks.MockAuditRepository)
		svc := NewTemplateService(tmplRepo, auditRepo)

		clientID := uuid.New()
		templateID := uuid.New()
		tmpl := &domain.Template{
			ID: templateID, ClientID: clientID,
			Name: "test", Body: "Hello {{name}}, code: {{code}}",
			Variables: []string{"name", "code"}, Status: domain.StatusApproved,
		}
		tmplRepo.On("GetByID", mock.Anything, templateID, clientID).Return(tmpl, nil)

		rendered, name, err := svc.RenderTemplate(context.Background(), templateID, clientID, map[string]string{
			"name": "World",
			"code": "123",
		})

		require.NoError(t, err)
		assert.Equal(t, "Hello World, code: 123", rendered)
		assert.Equal(t, "test", name)
	})

	t.Run("not approved", func(t *testing.T) {
		tmplRepo := new(mocks.MockTemplateRepository)
		svc := NewTemplateService(tmplRepo, new(mocks.MockAuditRepository))

		clientID := uuid.New()
		templateID := uuid.New()
		tmpl := &domain.Template{ID: templateID, Status: domain.StatusDraft}
		tmplRepo.On("GetByID", mock.Anything, templateID, clientID).Return(tmpl, nil)

		_, _, err := svc.RenderTemplate(context.Background(), templateID, clientID, nil)
		assert.ErrorIs(t, err, domain.ErrTemplateNotApproved)
	})

	t.Run("missing variables", func(t *testing.T) {
		tmplRepo := new(mocks.MockTemplateRepository)
		svc := NewTemplateService(tmplRepo, new(mocks.MockAuditRepository))

		clientID := uuid.New()
		templateID := uuid.New()
		tmpl := &domain.Template{
			ID: templateID, Body: "Hi {{name}}",
			Variables: []string{"name"}, Status: domain.StatusApproved,
		}
		tmplRepo.On("GetByID", mock.Anything, templateID, clientID).Return(tmpl, nil)

		_, _, err := svc.RenderTemplate(context.Background(), templateID, clientID, map[string]string{})
		assert.ErrorIs(t, err, domain.ErrMissingVariables)
	})
}

func TestTemplateService_ApproveTemplate(t *testing.T) {
	t.Run("success from draft", func(t *testing.T) {
		tmplRepo := new(mocks.MockTemplateRepository)
		auditRepo := new(mocks.MockAuditRepository)
		svc := NewTemplateService(tmplRepo, auditRepo)

		id := uuid.New()
		tmpl := &domain.Template{ID: id, Status: domain.StatusDraft}
		approved := &domain.Template{ID: id, Status: domain.StatusApproved}

		tmplRepo.On("GetByIDAdmin", mock.Anything, id).Return(tmpl, nil)
		tmplRepo.On("UpdateStatus", mock.Anything, id, domain.StatusApproved, "").Return(approved, nil)
		auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditEntry")).Return(nil)

		result, err := svc.ApproveTemplate(context.Background(), id, nil)

		require.NoError(t, err)
		assert.Equal(t, domain.StatusApproved, result.Status)
	})

	t.Run("reject non-draft", func(t *testing.T) {
		tmplRepo := new(mocks.MockTemplateRepository)
		svc := NewTemplateService(tmplRepo, new(mocks.MockAuditRepository))

		id := uuid.New()
		tmpl := &domain.Template{ID: id, Status: domain.StatusApproved}
		tmplRepo.On("GetByIDAdmin", mock.Anything, id).Return(tmpl, nil)

		_, err := svc.ApproveTemplate(context.Background(), id, nil)
		assert.ErrorIs(t, err, domain.ErrInvalidStatus)
	})
}

func TestTemplateService_ListTemplates(t *testing.T) {
	t.Run("defaults for invalid params", func(t *testing.T) {
		tmplRepo := new(mocks.MockTemplateRepository)
		svc := NewTemplateService(tmplRepo, new(mocks.MockAuditRepository))

		clientID := uuid.New()
		tmplRepo.On("ListByClientID", mock.Anything, clientID, "", 100, 0).Return([]*domain.Template{}, 0, nil)

		_, _, err := svc.ListTemplates(context.Background(), clientID, "", -1, -5)
		assert.NoError(t, err)
		tmplRepo.AssertCalled(t, "ListByClientID", mock.Anything, clientID, "", 100, 0)
	})
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/services/template/... -v`
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add internal/services/template/application/ports.go internal/services/template/application/template_service.go internal/services/template/mocks/ internal/services/template/application/template_service_test.go
git commit -m "test(template): introduce interfaces and add application service tests"
```

---

### Task 13: Webhook application — introduce interfaces and test

**Files:**
- Create: `internal/services/webhook/application/ports.go`
- Modify: `internal/services/webhook/application/webhook_service.go`
- Create: `internal/services/webhook/application/webhook_service_test.go`

Same pattern as template: extract interfaces from concrete repository types.

- [ ] **Step 1: Read webhook_service.go fully to understand all repo methods used**

Run: `grep -n "s.subRepo\." internal/services/webhook/application/webhook_service.go`

- [ ] **Step 2: Create ports.go with SubscriptionRepository interface**

```go
// internal/services/webhook/application/ports.go
package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/webhook/domain"
)

type SubscriptionRepository interface {
	Create(ctx context.Context, sub *domain.Subscription) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Subscription, error)
	List(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*domain.Subscription, int, error)
	Update(ctx context.Context, sub *domain.Subscription) error
	Delete(ctx context.Context, id uuid.UUID) error
	CountByClientID(ctx context.Context, clientID uuid.UUID) (int, error)
}
```

- [ ] **Step 3: Refactor WebhookService to use interface**

Change struct field `subRepo *repository.SubscriptionRepository` to `subRepo SubscriptionRepository`.
Update constructor signature accordingly. Remove unused repository import.

- [ ] **Step 4: Create mock and write tests**

Follow the same pattern as Task 12. Test: CreateSubscription (success, max limit, invalid URL), GetSubscription, ListSubscriptions, DeleteSubscription.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/services/webhook/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/services/webhook/
git commit -m "test(webhook): introduce interfaces and add service tests"
```

---

### Task 14: Audit service tests

**Files:**
- Create: `internal/services/audit/mocks/mocks.go`
- Create: `internal/services/audit/grpc/server_test.go`

The audit service is simple — just a gRPC server wrapping a repository.

- [ ] **Step 1: Create mock for AuditLogRepository**

```go
// internal/services/audit/mocks/mocks.go
package mocks

import (
	"context"

	"github.com/smpp-server/smpp-server/internal/services/audit/domain"
	"github.com/stretchr/testify/mock"
)

type MockAuditLogRepository struct {
	mock.Mock
}

func (m *MockAuditLogRepository) QueryAuditLog(ctx context.Context, filters *domain.AuditLogFilters) ([]*domain.AuditLogEntry, int, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.AuditLogEntry), args.Int(1), args.Error(2)
}
```

- [ ] **Step 2: Write gRPC server test**

```go
// internal/services/audit/grpc/server_test.go
package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/smpp-server/smpp-server/api/proto/auditv1"
	"github.com/smpp-server/smpp-server/internal/services/audit/domain"
	"github.com/smpp-server/smpp-server/internal/services/audit/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestServer_QueryAuditLog(t *testing.T) {
	t.Run("returns entries", func(t *testing.T) {
		repo := new(mocks.MockAuditLogRepository)
		srv := NewServer(repo)

		entries := []*domain.AuditLogEntry{
			{ID: "1", TenantID: "t1", Action: "login", UserID: "u1", CreatedAt: time.Now()},
		}
		repo.On("QueryAuditLog", mock.Anything, mock.AnythingOfType("*domain.AuditLogFilters")).Return(entries, 1, nil)

		resp, err := srv.QueryAuditLog(context.Background(), &auditv1.QueryAuditLogRequest{
			TenantId: "t1",
			Page:     1,
			PerPage:  10,
		})

		require.NoError(t, err)
		assert.Len(t, resp.Entries, 1)
		assert.Equal(t, int32(1), resp.Total)
	})

	t.Run("empty tenant returns error", func(t *testing.T) {
		repo := new(mocks.MockAuditLogRepository)
		srv := NewServer(repo)

		_, err := srv.QueryAuditLog(context.Background(), &auditv1.QueryAuditLogRequest{
			TenantId: "",
		})

		assert.Error(t, err)
	})
}
```

Note: Read `internal/services/audit/grpc/server.go` QueryAuditLog method to verify the exact validation and response mapping before writing the test. Adjust assertions based on actual implementation.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/services/audit/... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/services/audit/
git commit -m "test(audit): add mock and gRPC server tests"
```

---

## Level 3: Pipeline & Infrastructure Tests

### Task 15: BatchAccumulator tests

**Files:**
- Create: `internal/pipeline/batch/batcher_test.go`

- [ ] **Step 1: Write batch accumulator tests**

```go
package batch

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBatchAccumulator_FlushOnMaxSize(t *testing.T) {
	b := New[int](3, 10*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	b.Start(ctx)

	b.Add(1)
	b.Add(2)
	b.Add(3) // triggers flush

	select {
	case batch := <-b.FlushCh():
		assert.Equal(t, []int{1, 2, 3}, batch)
	case <-time.After(time.Second):
		t.Fatal("expected flush on max size")
	}

	cancel()
}

func TestBatchAccumulator_FlushOnMaxWait(t *testing.T) {
	b := New[string](100, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	b.Start(ctx)

	b.Add("hello")

	select {
	case batch := <-b.FlushCh():
		assert.Equal(t, []string{"hello"}, batch)
	case <-time.After(time.Second):
		t.Fatal("expected flush on max wait timeout")
	}

	cancel()
}

func TestBatchAccumulator_FlushOnContextCancel(t *testing.T) {
	b := New[int](100, 10*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	b.Start(ctx)

	b.Add(42)
	cancel()

	select {
	case batch := <-b.FlushCh():
		require.Len(t, batch, 1)
		assert.Equal(t, 42, batch[0])
	case <-time.After(time.Second):
		t.Fatal("expected flush on cancel")
	}

	// Channel should be closed after cancel
	_, open := <-b.FlushCh()
	assert.False(t, open)
}

func TestBatchAccumulator_EmptyOnCancel(t *testing.T) {
	b := New[int](100, 10*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	b.Start(ctx)

	cancel()

	// Should close without sending a batch
	_, open := <-b.FlushCh()
	assert.False(t, open)
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/pipeline/batch/... -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/pipeline/batch/batcher_test.go
git commit -m "test(pipeline): add BatchAccumulator unit tests"
```

---

### Task 16: Backpressure manager tests

**Files:**
- Create: `internal/pipeline/backpressure/manager_test.go`

- [ ] **Step 1: Read the manager source to understand the API**

Run: `grep -n "^func" internal/pipeline/backpressure/manager.go`

- [ ] **Step 2: Write backpressure tests**

Test: Allow/TryConsume token logic, refill over time, throttling state. The exact test code depends on the public API — read the file first and write tests for each exported method.

Key behaviors to test:
- Token bucket starts full
- Consuming tokens decreases available
- Tokens refill over time
- Over-consumption triggers throttled state
- In-flight counter management

- [ ] **Step 3: Run and commit**

Run: `go test ./internal/pipeline/backpressure/... -v`

```bash
git add internal/pipeline/backpressure/manager_test.go
git commit -m "test(pipeline): add backpressure manager tests"
```

---

### Task 17: SMPP server session and rate limiter tests

**Files:**
- Create: `internal/smpp/server/session_test.go`
- Create: `internal/smpp/server/ratelimiter_test.go`

- [ ] **Step 1: Write rate limiter tests**

```go
package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(10) // 10 tokens per second

	// Should allow initial requests
	for i := 0; i < 10; i++ {
		assert.True(t, rl.Allow(), "request %d should be allowed", i)
	}

	// Should reject when tokens exhausted
	assert.False(t, rl.Allow(), "should be throttled after exhausting tokens")
}

func TestRateLimiter_Refill(t *testing.T) {
	rl := NewRateLimiter(10)

	// Exhaust tokens
	for i := 0; i < 10; i++ {
		rl.Allow()
	}
	assert.False(t, rl.Allow())

	// Wait for refill
	time.Sleep(200 * time.Millisecond)

	// Should have refilled some tokens
	assert.True(t, rl.Allow())
}
```

Note: Adjust constructor name (`NewRateLimiter`) based on actual code.

- [ ] **Step 2: Write session state tests**

Test session state transitions (OPEN → BOUND_TRX), sequence number generation, activity tracking.

- [ ] **Step 3: Run and commit**

Run: `go test ./internal/smpp/server/... -v`

```bash
git add internal/smpp/server/session_test.go internal/smpp/server/ratelimiter_test.go
git commit -m "test(smpp): add session and rate limiter tests"
```

---

### Task 18: Shared SMS segment tests

**Files:**
- The file `internal/shared/sms_segment.go` has logic for GSM7/UCS2 detection, segment counting, and UDH generation.

Check if `internal/shared/sms_segment_test.go` already exists (it was listed in the shared tests). If not:

- [ ] **Step 1: Verify existing coverage**

Run: `go test ./internal/shared/... -v -run Segment`

If no tests exist for sms_segment.go:

- [ ] **Step 2: Write SMS segment tests**

```go
// internal/shared/sms_segment_test.go (if not exists)
package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectEncoding(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{"ASCII", "Hello World", "GSM7"},
		{"GSM7 with special chars", "Hello @ World", "GSM7"},
		{"Cyrillic", "Привет мир", "UCS2"},
		{"Chinese", "你好世界", "UCS2"},
		{"Emoji", "Hello 😀", "UCS2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectEncoding(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCountSegments(t *testing.T) {
	t.Run("single GSM7 segment", func(t *testing.T) {
		text := "Hello World" // 11 chars, fits in 160
		assert.Equal(t, 1, CountSegments(text))
	})

	t.Run("multipart GSM7", func(t *testing.T) {
		// 161 chars forces 2 segments (153 + 8)
		text := ""
		for i := 0; i < 161; i++ {
			text += "a"
		}
		assert.Equal(t, 2, CountSegments(text))
	})
}
```

Note: Adjust function names (`DetectEncoding`, `CountSegments`) to match actual exports in `sms_segment.go`.

- [ ] **Step 3: Run and commit**

---

### Task 19: Shared context helpers tests

**Files:**
- Create: `internal/shared/context_test.go` (if not exists)

- [ ] **Step 1: Write context utility tests**

Test `RequestID`, `UserID`, `ClientID` context getters/setters.

- [ ] **Step 2: Run and commit**

---

## Level 4: gRPC Server Tests for Untested Services

### Task 20: Campaign gRPC server tests

**Files:**
- Create: `internal/services/campaign/grpc/server_test.go`

- [ ] **Step 1: Read campaign service interfaces**

Identify the `CampaignService` methods the gRPC server calls and create appropriate mocks.

- [ ] **Step 2: Create campaign mocks**

Create `internal/services/campaign/mocks/mocks.go` with `MockCampaignService` if the gRPC server uses a service interface, or mock the repository if it uses the service directly.

- [ ] **Step 3: Write gRPC server tests**

Test: CreateCampaign, GetCampaign, LaunchCampaign, PauseCampaign, CancelCampaign — verify request validation, error mapping, response format.

- [ ] **Step 4: Run and commit**

```bash
git add internal/services/campaign/
git commit -m "test(campaign): add gRPC server and mock tests"
```

---

### Task 21: Contact gRPC server tests

**Files:**
- Create: `internal/services/contact/grpc/server_test.go`
- Create: `internal/services/contact/mocks/mocks.go` (if not exists)

Same pattern as campaign — mock the service, test RPC methods.

- [ ] **Step 1-4: Follow same pattern as Task 20**

---

### Task 22: Provider gRPC server tests

**Files:**
- Create: `internal/services/provider/grpc/server_test.go`

- [ ] **Step 1: Create ConnectionPoolService mock**

The Provider gRPC server depends on `ProviderService`, `ConnectionPoolService`, and `SenderService`. Create mocks for each.

- [ ] **Step 2: Write gRPC tests**

Test: CreateProvider (validation + success), GetProvider (found/not found), UpdateProvider, ListProviders.

- [ ] **Step 3: Run and commit**

---

### Task 23: Template gRPC server tests

**Files:**
- Create: `internal/services/template/grpc/server_test.go`

- [ ] **Step 1: Write tests using existing mocks from Task 12**

Test: CreateTemplate (validation), GetTemplate, UpdateTemplate, DeleteTemplate, RenderTemplate.

- [ ] **Step 2: Run and commit**

---

### Task 24: Webhook gRPC server tests

**Files:**
- Create: `internal/services/webhook/grpc/server_test.go`

Same pattern — mock WebhookService, test all RPC methods.

---

## Level 5: Comprehensive Verification

### Task 25: Full test run and coverage report

- [ ] **Step 1: Run all unit tests**

Run: `go test ./internal/... 2>&1 | grep -E "^(FAIL|ok)"`
Expected: NO FAIL lines

- [ ] **Step 2: Generate coverage report**

Run: `go test ./internal/... -coverprofile=coverage.out -covermode=atomic`
Run: `go tool cover -func=coverage.out | tail -1`

Report overall coverage percentage.

- [ ] **Step 3: List remaining uncovered packages**

Run: `go test ./internal/... -cover 2>&1 | grep "0.0%"`

These are packages with zero coverage that may need attention in future iterations.

- [ ] **Step 4: Commit coverage config**

```bash
echo "coverage.out" >> .gitignore
git add .gitignore
git commit -m "chore: add coverage.out to gitignore"
```

---

## Summary

| Level | Tasks | Packages Affected | Type |
|-------|-------|-------------------|------|
| 0 | 1-5 | 8 broken packages | Fix existing |
| 1 | 6-10 | 5 domain packages | New unit tests |
| 2 | 11-14 | 4 application packages + refactor | New unit tests |
| 3 | 15-19 | 5 infrastructure packages | New unit tests |
| 4 | 20-24 | 5 gRPC servers | New unit tests |
| 5 | 25 | All | Verification |

**Total: ~25 tasks, ~40+ new test files**
