package payment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mock implementation of PaymentProvider ---

type MockPaymentProvider struct {
	mock.Mock
}

func (m *MockPaymentProvider) CreatePayment(ctx context.Context, req CreatePaymentRequest) (*PaymentSession, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*PaymentSession), args.Error(1)
}

func (m *MockPaymentProvider) HandleCallback(ctx context.Context, raw []byte, signature string) (*PaymentResult, error) {
	args := m.Called(ctx, raw, signature)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*PaymentResult), args.Error(1)
}

// --- PaymentProvider interface compliance tests ---

func TestStubPaymentProvider_ImplementsInterface(t *testing.T) {
	var _ PaymentProvider = (*StubPaymentProvider)(nil)
}

func TestMockPaymentProvider_ImplementsInterface(t *testing.T) {
	var _ PaymentProvider = (*MockPaymentProvider)(nil)
}

// --- StubPaymentProvider tests ---

func TestNewStubPaymentProvider(t *testing.T) {
	provider := NewStubPaymentProvider()
	require.NotNil(t, provider)
}

func TestStubPaymentProvider_CreatePayment(t *testing.T) {
	provider := NewStubPaymentProvider()
	ctx := context.Background()

	req := CreatePaymentRequest{
		ClientID:  "client-1",
		Amount:    "100.00",
		Currency:  "RUB",
		ReturnURL: "https://example.com/return",
	}

	session, err := provider.CreatePayment(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, session)

	assert.NotEmpty(t, session.PaymentID, "PaymentID should be generated")
	assert.Contains(t, session.PaymentURL, session.PaymentID,
		"PaymentURL should contain the PaymentID")
	assert.Contains(t, session.PaymentURL, "/portal/v1/billing/top-up/callback")
	assert.Contains(t, session.PaymentURL, "status=success")
	assert.True(t, session.ExpiresAt.After(time.Now()),
		"ExpiresAt should be in the future")
}

func TestStubPaymentProvider_CreatePayment_UniqueIDs(t *testing.T) {
	provider := NewStubPaymentProvider()
	ctx := context.Background()

	req := CreatePaymentRequest{
		ClientID: "client-1",
		Amount:   "50.00",
		Currency: "RUB",
	}

	s1, err1 := provider.CreatePayment(ctx, req)
	s2, err2 := provider.CreatePayment(ctx, req)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.NotEqual(t, s1.PaymentID, s2.PaymentID,
		"Each call should produce a unique PaymentID")
}

func TestStubPaymentProvider_HandleCallback(t *testing.T) {
	provider := NewStubPaymentProvider()
	ctx := context.Background()

	result, err := provider.HandleCallback(ctx, []byte("raw-data"), "sig-value")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "stub", result.PaymentID)
	assert.Equal(t, "success", result.Status)
	assert.Equal(t, "0", result.Amount)
	assert.Equal(t, "RUB", result.Currency)
}

func TestStubPaymentProvider_HandleCallback_IgnoresInput(t *testing.T) {
	provider := NewStubPaymentProvider()
	ctx := context.Background()

	// The stub ignores all input and returns the same result
	r1, _ := provider.HandleCallback(ctx, nil, "")
	r2, _ := provider.HandleCallback(ctx, []byte("anything"), "any-sig")

	assert.Equal(t, r1.PaymentID, r2.PaymentID)
	assert.Equal(t, r1.Status, r2.Status)
}

// --- MockPaymentProvider tests ---

func TestMockPaymentProvider_CreatePayment_Success(t *testing.T) {
	m := new(MockPaymentProvider)
	ctx := context.Background()

	req := CreatePaymentRequest{
		ClientID:  "client-1",
		Amount:    "200.00",
		Currency:  "RUB",
		ReturnURL: "https://example.com",
	}

	expected := &PaymentSession{
		PaymentID:  "pay-123",
		PaymentURL: "https://pay.example.com/pay-123",
		ExpiresAt:  time.Now().Add(1 * time.Hour),
	}

	m.On("CreatePayment", ctx, req).Return(expected, nil)

	session, err := m.CreatePayment(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, expected, session)
	m.AssertExpectations(t)
}

func TestMockPaymentProvider_CreatePayment_Error(t *testing.T) {
	m := new(MockPaymentProvider)
	ctx := context.Background()

	req := CreatePaymentRequest{
		ClientID: "client-1",
		Amount:   "999.99",
		Currency: "EUR",
	}

	m.On("CreatePayment", ctx, req).Return(nil, errors.New("payment gateway unavailable"))

	session, err := m.CreatePayment(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "payment gateway unavailable")
	m.AssertExpectations(t)
}

func TestMockPaymentProvider_HandleCallback_Success(t *testing.T) {
	m := new(MockPaymentProvider)
	ctx := context.Background()
	raw := []byte(`{"payment_id":"pay-123","status":"success"}`)

	expected := &PaymentResult{
		PaymentID: "pay-123",
		Status:    "success",
		Amount:    "200.00",
		Currency:  "RUB",
		ClientID:  "client-1",
	}

	m.On("HandleCallback", ctx, raw, "valid-sig").Return(expected, nil)

	result, err := m.HandleCallback(ctx, raw, "valid-sig")
	require.NoError(t, err)
	assert.Equal(t, expected, result)
	m.AssertExpectations(t)
}

func TestMockPaymentProvider_HandleCallback_InvalidSignature(t *testing.T) {
	m := new(MockPaymentProvider)
	ctx := context.Background()

	m.On("HandleCallback", ctx, mock.Anything, "bad-sig").
		Return(nil, errors.New("invalid signature"))

	result, err := m.HandleCallback(ctx, []byte("data"), "bad-sig")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid signature")
	m.AssertExpectations(t)
}

// --- Data model tests ---

func TestCreatePaymentRequest_Fields(t *testing.T) {
	req := CreatePaymentRequest{
		ClientID:  "client-1",
		Amount:    "100.50",
		Currency:  "RUB",
		ReturnURL: "https://example.com/return",
	}

	assert.Equal(t, "client-1", req.ClientID)
	assert.Equal(t, "100.50", req.Amount)
	assert.Equal(t, "RUB", req.Currency)
	assert.Equal(t, "https://example.com/return", req.ReturnURL)
}

func TestPaymentSession_Fields(t *testing.T) {
	expiresAt := time.Now().Add(30 * time.Minute)
	session := PaymentSession{
		PaymentID:  "pay-1",
		PaymentURL: "https://pay.example.com/pay-1",
		ExpiresAt:  expiresAt,
	}

	assert.Equal(t, "pay-1", session.PaymentID)
	assert.Equal(t, "https://pay.example.com/pay-1", session.PaymentURL)
	assert.Equal(t, expiresAt, session.ExpiresAt)
}

func TestPaymentResult_Fields(t *testing.T) {
	result := PaymentResult{
		PaymentID: "pay-1",
		Status:    "success",
		Amount:    "100.00",
		Currency:  "RUB",
		ClientID:  "client-1",
	}

	assert.Equal(t, "pay-1", result.PaymentID)
	assert.Equal(t, "success", result.Status)
	assert.Equal(t, "100.00", result.Amount)
	assert.Equal(t, "RUB", result.Currency)
	assert.Equal(t, "client-1", result.ClientID)
}
