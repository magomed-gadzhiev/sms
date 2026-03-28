package payment

import (
	"context"
	"time"
)

type PaymentProvider interface {
	CreatePayment(ctx context.Context, req CreatePaymentRequest) (*PaymentSession, error)
	HandleCallback(ctx context.Context, raw []byte, signature string) (*PaymentResult, error)
}

type CreatePaymentRequest struct {
	ClientID  string
	Amount    string
	Currency  string
	ReturnURL string
}

type PaymentSession struct {
	PaymentID  string
	PaymentURL string
	ExpiresAt  time.Time
}

type PaymentResult struct {
	PaymentID string
	Status    string
	Amount    string
	Currency  string
	ClientID  string
}
