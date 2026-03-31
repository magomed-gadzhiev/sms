package payment

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
)

type StubPaymentProvider struct {
	mu       sync.Mutex
	sessions map[string]CreatePaymentRequest
}

func NewStubPaymentProvider() *StubPaymentProvider {
	return &StubPaymentProvider{sessions: make(map[string]CreatePaymentRequest)}
}

func (s *StubPaymentProvider) CreatePayment(_ context.Context, req CreatePaymentRequest) (*PaymentSession, error) {
	paymentID := uuid.New().String()
	s.mu.Lock()
	s.sessions[paymentID] = req
	s.mu.Unlock()
	return &PaymentSession{
		PaymentID:  paymentID,
		PaymentURL: fmt.Sprintf("/portal/v1/billing/top-up/callback?payment_id=%s&status=success", paymentID),
		ExpiresAt:  time.Now().Add(30 * time.Minute),
	}, nil
}

func (s *StubPaymentProvider) HandleCallback(_ context.Context, _ []byte, signature string) (*PaymentResult, error) {
	params, err := url.ParseQuery(signature)
	if err == nil {
		paymentID := params.Get("payment_id")
		s.mu.Lock()
		req, ok := s.sessions[paymentID]
		if ok {
			delete(s.sessions, paymentID)
		}
		s.mu.Unlock()
		if ok {
			return &PaymentResult{
				PaymentID: paymentID,
				Status:    "success",
				Amount:    req.Amount,
				Currency:  req.Currency,
				ClientID:  req.ClientID,
			}, nil
		}
	}
	return &PaymentResult{
		PaymentID: "stub",
		Status:    "success",
		Amount:    "0",
		Currency:  "RUB",
	}, nil
}
