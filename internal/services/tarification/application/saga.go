package application

import (
	"context"
	"fmt"
	"math/big"

	"github.com/google/uuid"
	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// SagaOrchestrator управляет транзакциями между tarification и billing сервисами
type SagaOrchestrator struct {
	billingClient billingv1.BillingServiceClient
}

// NewSagaOrchestrator создает нового оркестратора саг
func NewSagaOrchestrator(billingClient billingv1.BillingServiceClient) *SagaOrchestrator {
	return &SagaOrchestrator{billingClient: billingClient}
}

// ChargeResult результат списания
type ChargeResult struct {
	TransactionID string
	NewBalance    string
	Success       bool
	Error         string
}

// Charge списывает средства за сообщение через billing-service
func (s *SagaOrchestrator) Charge(ctx context.Context, clientID, messageID, amount, currency, description string, _ int32) (*ChargeResult, error) {
	resp, err := s.billingClient.ChargeMessage(ctx, &billingv1.ChargeMessageRequest{
		ClientId:    clientID,
		MessageId:   messageID,
		Amount:      amount,
		Currency:    currency,
		Description: description,
	})
	if err != nil {
		return nil, fmt.Errorf("billing charge failed: %w", err)
	}

	return &ChargeResult{
		TransactionID: resp.TransactionId,
		NewBalance:    resp.NewBalance,
		Success:       resp.Success,
		Error:         resp.Error,
	}, nil
}

// Refund возвращает средства на баланс клиента
func (s *SagaOrchestrator) Refund(ctx context.Context, clientID, amount, currency, description string) (*ChargeResult, error) {
	resp, err := s.billingClient.AddCredits(ctx, &billingv1.AddCreditsRequest{
		ClientId:    clientID,
		Amount:      amount,
		Currency:    currency,
		Description: description,
	})
	if err != nil {
		return nil, fmt.Errorf("billing refund failed: %w", err)
	}

	return &ChargeResult{
		TransactionID: resp.TransactionId,
		NewBalance:    resp.NewBalance,
		Success:       resp.Success,
		Error:         resp.Error,
	}, nil
}

// DeductForRecalc доначисляет средства при пересчёте (если новая цена выше)
func (s *SagaOrchestrator) DeductForRecalc(ctx context.Context, clientID, amount, currency, description string) (*ChargeResult, error) {
	resp, err := s.billingClient.DeductCredits(ctx, &billingv1.DeductCreditsRequest{
		ClientId:    clientID,
		Amount:      amount,
		Currency:    currency,
		Description: description,
	})
	if err != nil {
		return nil, fmt.Errorf("billing deduct for recalc failed: %w", err)
	}

	return &ChargeResult{
		TransactionID: resp.TransactionId,
		NewBalance:    resp.NewBalance,
		Success:       resp.Success,
		Error:         resp.Error,
	}, nil
}

// DualChargeResult результат двойного списания (субаккаунт + агрегатор)
type DualChargeResult struct {
	SubAccountCharge *ChargeResult
	AggregatorCharge *ChargeResult
	Success          bool
	Error            string
}

// ChargeDual списывает средства с субаккаунта (по тарифу агрегатора) и с агрегатора (по платформенному тарифу)
func (s *SagaOrchestrator) ChargeDual(
	ctx context.Context,
	subAccountID, aggregatorID, messageID string,
	subAccountAmount, aggregatorAmount, currency string,
) (*DualChargeResult, error) {
	// Сначала списываем с субаккаунта по тарифу агрегатора
	subResp, err := s.billingClient.ChargeMessage(ctx, &billingv1.ChargeMessageRequest{
		ClientId:    subAccountID,
		MessageId:   messageID,
		Amount:      subAccountAmount,
		Currency:    currency,
		Description: "SMS субаккаунт: тариф агрегатора",
	})
	if err != nil {
		return nil, fmt.Errorf("sub-account billing charge failed: %w", err)
	}
	if !subResp.Success {
		return &DualChargeResult{
			SubAccountCharge: &ChargeResult{Success: false, Error: subResp.Error},
			Success:          false,
			Error:            subResp.Error,
		}, nil
	}

	// Затем списываем с агрегатора по платформенному тарифу
	// Генерируем уникальный UUID для транзакции агрегатора (детерминированно от messageID)
	aggMessageID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(messageID+"_agg")).String()
	aggResp, err := s.billingClient.ChargeMessage(ctx, &billingv1.ChargeMessageRequest{
		ClientId:    aggregatorID,
		MessageId:   aggMessageID,
		Amount:      aggregatorAmount,
		Currency:    currency,
		Description: "SMS агрегатор: платформенный тариф",
	})
	if err != nil {
		// Компенсация: возвращаем средства субаккаунту
		_, _ = s.Refund(ctx, subAccountID, subAccountAmount, currency, "компенсация: ошибка списания агрегатора")
		return nil, fmt.Errorf("aggregator billing charge failed: %w", err)
	}

	return &DualChargeResult{
		SubAccountCharge: &ChargeResult{
			TransactionID: subResp.TransactionId,
			NewBalance:    subResp.NewBalance,
			Success:       true,
		},
		AggregatorCharge: &ChargeResult{
			TransactionID: aggResp.TransactionId,
			NewBalance:    aggResp.NewBalance,
			Success:       aggResp.Success,
		},
		Success: aggResp.Success,
	}, nil
}

// HandleRecalc обрабатывает пересчёт при переходе порога
func (s *SagaOrchestrator) HandleRecalc(ctx context.Context, clientID, recalcAmount, currency string) (*ChargeResult, error) {
	amount, _, err := big.ParseFloat(recalcAmount, 10, 128, big.ToNearestEven)
	if err != nil {
		return nil, fmt.Errorf("invalid recalc amount: %w", err)
	}

	zero := new(big.Float).SetFloat64(0)

	if amount.Cmp(zero) < 0 {
		// Отрицательная сумма — возврат (новая цена ниже старой)
		absAmount := new(big.Float).Abs(amount)
		return s.Refund(ctx, clientID, absAmount.Text('f', 6), currency,
			"threshold recalculation refund")
	} else if amount.Cmp(zero) > 0 {
		// Положительная сумма — доначисление (новая цена выше)
		result, err := s.DeductForRecalc(ctx, clientID, recalcAmount, currency,
			"threshold recalculation charge")
		if err != nil {
			return nil, err
		}
		if !result.Success {
			// Недостаточно средств — фиксируем долг (eventual consistency)
			return &ChargeResult{
				Success: false,
				Error:   domain.ErrDebtBlocking.Error(),
			}, nil
		}
		return result, nil
	}

	// Сумма пересчёта = 0, ничего не делаем
	return &ChargeResult{Success: true}, nil
}
