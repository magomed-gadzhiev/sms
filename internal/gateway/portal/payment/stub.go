package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type StubPaymentProvider struct{}

func NewStubPaymentProvider() *StubPaymentProvider {
	return &StubPaymentProvider{}
}

func (s *StubPaymentProvider) CreatePayment(_ context.Context, req CreatePaymentRequest) (*PaymentSession, error) {
	paymentID := uuid.New().String()
	return &PaymentSession{
		PaymentID:  paymentID,
		PaymentURL: fmt.Sprintf("/portal/v1/billing/top-up/callback?payment_id=%s&status=success", paymentID),
		ExpiresAt:  time.Now().Add(30 * time.Minute),
	}, nil
}

func (s *StubPaymentProvider) HandleCallback(_ context.Context, _ []byte, _ string) (*PaymentResult, error) {
	return &PaymentResult{
		PaymentID: "stub",
		Status:    "success",
		Amount:    "0",
		Currency:  "RUB",
	}, nil
}
