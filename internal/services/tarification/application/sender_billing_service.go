package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

var billingRecordsCreatedTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "billing_records_created_total",
		Help: "Общее количество созданных billing records",
	},
	[]string{"type"}, // initial | monthly
)

// SenderBillingService управляет billing-записями для платных имён отправителей
type SenderBillingService struct {
	repo domain.SenderBillingRepository
}

// NewSenderBillingService создаёт новый SenderBillingService
func NewSenderBillingService(repo domain.SenderBillingRepository) *SenderBillingService {
	return &SenderBillingService{repo: repo}
}

// CreateBillingRecord создаёт billing-запись для текущего месяца.
// Если запись за текущий месяц уже существует, возвращает (false, nil).
func (s *SenderBillingService) CreateBillingRecord(
	ctx context.Context,
	senderRegistrationID, clientID, operatorID uuid.UUID,
	amount string,
) (*domain.SenderNameBillingRecord, bool, error) {
	now := time.Now().UTC()
	record := &domain.SenderNameBillingRecord{
		ID:                   uuid.New(),
		SenderRegistrationID: senderRegistrationID,
		ClientID:             clientID,
		OperatorID:           operatorID,
		BillingMonth:         domain.BillingMonthKey(now),
		Amount:               amount,
		CreatedAt:            now,
	}

	created, err := s.repo.CreateIfNotExists(ctx, record)
	if err != nil {
		log.Error().Err(err).
			Str("sender_registration_id", senderRegistrationID.String()).
			Msg("ошибка создания billing record")
		return nil, false, err
	}
	if created {
		billingRecordsCreatedTotal.WithLabelValues("initial").Inc()
	}
	return record, created, nil
}

// ListBillingRecords возвращает историю начислений по регистрации
func (s *SenderBillingService) ListBillingRecords(
	ctx context.Context,
	senderRegistrationID uuid.UUID,
	limit, offset int,
) ([]*domain.SenderNameBillingRecord, int, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.repo.ListByRegistration(ctx, senderRegistrationID, limit, offset)
}
