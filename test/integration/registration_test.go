//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getPortalURL(t *testing.T) string {
	t.Helper()
	if u := os.Getenv("TEST_PORTAL_URL"); u != "" {
		return u
	}
	return "http://localhost:8082"
}

type registerRequest struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	CompanyName   string `json:"company_name"`
	ContactPerson string `json:"contact_person"`
	Phone         string `json:"phone"`
	PlanName      string `json:"plan_name"`
}

type registerResponse struct {
	ClientID string `json:"client_id"`
	User     struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Role  string `json:"role"`
	} `json:"user"`
}

func doRegister(t *testing.T, body registerRequest) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)

	url := fmt.Sprintf("%s/portal/v1/auth/register", getPortalURL(t))
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload)) //nolint:noctx
	if err != nil {
		// The portal is optional for this suite: degrade to a skip when it is
		// not deployed at all (e.g. the CI integration job runs postgres only),
		// mirroring the 404/405 skips below. HTTP coverage of registration
		// lives in e2e (e2e/tests/auth/auth-public.spec.ts); point
		// TEST_PORTAL_URL at a running portal to exercise these tests.
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			t.Skipf("portal not reachable at %s: %v", url, err)
		}
		require.NoError(t, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestRegistration_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	email := fmt.Sprintf("success+%s@test.example", uuid.New().String())
	resp := doRegister(t, registerRequest{
		Email:         email,
		Password:      "securepassword123",
		CompanyName:   "Test Company",
		ContactPerson: "Test Person",
		Phone:         "+79001234567",
		PlanName:      "free",
	})
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		t.Skipf("portal does not expose /portal/v1/auth/register (HTTP %d)", resp.StatusCode)
	}

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var body registerResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	assert.NotEmpty(t, body.ClientID, "client_id should be set")
	assert.NotEmpty(t, body.User.ID, "user.id should be set")
	assert.Equal(t, email, body.User.Email, "user.email should match the registered email")
	assert.Equal(t, "client", body.User.Role, "user.role should be 'client'")

	var sessionCookieFound bool
	for _, c := range resp.Cookies() {
		if c.Name == "portal_session" {
			sessionCookieFound = true
			assert.NotEmpty(t, c.Value, "portal_session cookie value should not be empty")
			break
		}
	}
	assert.True(t, sessionCookieFound, "portal_session cookie should be set after registration")
}

func TestRegistration_DuplicateEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	email := fmt.Sprintf("dup+%s@test.example", uuid.New().String())
	req := registerRequest{
		Email:         email,
		Password:      "securepassword123",
		CompanyName:   "Dup Company",
		ContactPerson: "Dup Person",
		Phone:         "+79001234568",
		PlanName:      "free",
	}

	first := doRegister(t, req)
	if first.StatusCode == http.StatusNotFound || first.StatusCode == http.StatusMethodNotAllowed {
		t.Skipf("portal does not expose /portal/v1/auth/register (HTTP %d)", first.StatusCode)
	}
	require.Equal(t, http.StatusCreated, first.StatusCode, "first registration should succeed")

	second := doRegister(t, req)
	assert.Equal(t, http.StatusConflict, second.StatusCode, "second registration with same email should return 409")
}

func TestRegistration_InvalidInput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("missing company_name", func(t *testing.T) {
		email := fmt.Sprintf("invalid+%s@test.example", uuid.New().String())
		resp := doRegister(t, registerRequest{
			Email:         email,
			Password:      "securepassword123",
			CompanyName:   "",
			ContactPerson: "Some Person",
			Phone:         "+79001234569",
			PlanName:      "free",
		})
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
			t.Skipf("portal does not expose /portal/v1/auth/register (HTTP %d)", resp.StatusCode)
		}
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "missing company_name should return 400")
	})

	t.Run("short password", func(t *testing.T) {
		email := fmt.Sprintf("shortpw+%s@test.example", uuid.New().String())
		resp := doRegister(t, registerRequest{
			Email:         email,
			Password:      "short",
			CompanyName:   "Some Company",
			ContactPerson: "Some Person",
			Phone:         "+79001234570",
			PlanName:      "free",
		})
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
			t.Skipf("portal does not expose /portal/v1/auth/register (HTTP %d)", resp.StatusCode)
		}
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "password shorter than 8 chars should return 400")
	})

	t.Run("missing email", func(t *testing.T) {
		resp := doRegister(t, registerRequest{
			Email:         "",
			Password:      "securepassword123",
			CompanyName:   "Some Company",
			ContactPerson: "Some Person",
			Phone:         "+79001234571",
			PlanName:      "free",
		})
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
			t.Skipf("portal does not expose /portal/v1/auth/register (HTTP %d)", resp.StatusCode)
		}
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "missing email should return 400")
	})
}
