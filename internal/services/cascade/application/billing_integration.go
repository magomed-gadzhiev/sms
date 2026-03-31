package application

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
	tarificationv1 "github.com/smpp-server/smpp-server/api/proto/tarificationv1"
	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
	cascadekafka "github.com/smpp-server/smpp-server/internal/services/cascade/infrastructure/kafka"
)

// BillingIntegration выполняет тарификацию и биллинг каскадных попыток
type BillingIntegration struct {
	tarification tarificationv1.TarificationServiceClient
	billing      billingv1.BillingServiceClient
	attempts     domain.AttemptRepository
	deliveries   domain.DeliveryRepository
	logger       zerolog.Logger
}

func NewBillingIntegration(
	tarificationConn *grpc.ClientConn,
	billingConn *grpc.ClientConn,
	attempts domain.AttemptRepository,
	deliveries domain.DeliveryRepository,
	logger zerolog.Logger,
) *BillingIntegration {
	return &BillingIntegration{
		tarification: tarificationv1.NewTarificationServiceClient(tarificationConn),
		billing:      billingv1.NewBillingServiceClient(billingConn),
		attempts:     attempts,
		deliveries:   deliveries,
		logger:       logger.With().Str("component", "cascade_billing").Logger(),
	}
}

// BillAttempt тарифицирует и списывает средства за попытку доставки
func (b *BillingIntegration) BillAttempt(ctx context.Context, cmd cascadekafka.CascadeBillingCommand) error {
	if !cmd.Billable {
		b.logger.Debug().Str("attempt_id", cmd.AttemptID).Msg("попытка не тарифицируется, пропускаем")
		return nil
	}

	attemptID, err := uuid.Parse(cmd.AttemptID)
	if err != nil {
		return fmt.Errorf("parse attempt_id: %w", err)
	}
	deliveryID, err := uuid.Parse(cmd.DeliveryID)
	if err != nil {
		return fmt.Errorf("parse delivery_id: %w", err)
	}

	attempt, err := b.attempts.Get(ctx, attemptID)
	if err != nil {
		return fmt.Errorf("get attempt for billing: %w", err)
	}

	// Пропускаем skipped и late_duplicate
	if attempt.Status == domain.AttemptSkipped || attempt.Status == domain.AttemptLateDuplicate {
		b.logger.Debug().Str("attempt_id", cmd.AttemptID).Str("status", string(attempt.Status)).Msg("попытка не тарифицируется по статусу")
		return nil
	}

	delivery, err := b.deliveries.Get(ctx, deliveryID)
	if err != nil {
		return fmt.Errorf("get delivery for billing: %w", err)
	}

	// 1. Тарификация
	tariffResp, err := b.tarification.TarifyMessage(ctx, &tarificationv1.TarifyMessageRequest{
		ClientId:       cmd.ClientID,
		MessageId:      cmd.AttemptID,
		SenderName:     delivery.SenderName,
		SegmentCount:   1,
		IdempotencyKey: cmd.IdempotencyKey,
	})
	if err != nil {
		return fmt.Errorf("tarify message: %w", err)
	}

	if !tariffResp.Approved {
		b.logger.Warn().
			Str("attempt_id", cmd.AttemptID).
			Str("reason", tariffResp.RejectionReason).
			Msg("тарификация отклонена")
		return nil
	}

	amount := tariffResp.TotalAmount
	if amount == "" {
		amount = "0"
	}

	// 2. Биллинг
	chargeResp, err := b.billing.ChargeMessage(ctx, &billingv1.ChargeMessageRequest{
		ClientId:     cmd.ClientID,
		MessageId:    cmd.AttemptID,
		Amount:       amount,
		SegmentCount: 1,
		Description:  fmt.Sprintf("cascade delivery attempt via %s", cmd.ChannelType),
	})
	if err != nil {
		return fmt.Errorf("charge message: %w", err)
	}

	if !chargeResp.Success {
		b.logger.Warn().
			Str("attempt_id", cmd.AttemptID).
			Str("error", chargeResp.Error).
			Msg("биллинг не прошёл")
		return nil
	}

	// 3. Обновить стоимость попытки
	cost, _ := strconv.ParseFloat(amount, 64)
	if err := b.attempts.UpdateCost(ctx, attemptID, cost); err != nil {
		b.logger.Warn().Err(err).Str("attempt_id", cmd.AttemptID).Msg("ошибка обновления стоимости попытки")
	}

	// Обновить общую стоимость доставки
	newTotal := delivery.TotalCost + cost
	if err := b.deliveries.UpdateCost(ctx, deliveryID, newTotal); err != nil {
		b.logger.Warn().Err(err).Str("delivery_id", cmd.DeliveryID).Msg("ошибка обновления стоимости доставки")
	}

	b.logger.Info().
		Str("attempt_id", cmd.AttemptID).
		Str("delivery_id", cmd.DeliveryID).
		Str("channel_type", cmd.ChannelType).
		Str("amount", amount).
		Msg("биллинг выполнен")

	return nil
}
