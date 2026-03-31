package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	routingv1 "github.com/smpp-server/smpp-server/api/proto/routingv1"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

var billingSchedulerRunTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "billing_scheduler_run_total",
		Help: "Количество запусков планировщика биллинга",
	},
	[]string{"status"}, // success | error
)

// BillingScheduler ежемесячно начисляет плату за все активные платные имена отправителей.
// Идемпотентен: повторный запуск в том же месяце не создаёт дублирующих записей.
type BillingScheduler struct {
	regRepo        domain.SenderRegistrationRepository
	billingService *SenderBillingService
	routingClient  routingv1.RoutingServiceClient
	logger         zerolog.Logger
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewBillingScheduler создаёт новый BillingScheduler
func NewBillingScheduler(
	regRepo domain.SenderRegistrationRepository,
	billingService *SenderBillingService,
	routingClient routingv1.RoutingServiceClient,
) *BillingScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &BillingScheduler{
		regRepo:        regRepo,
		billingService: billingService,
		routingClient:  routingClient,
		logger:         log.With().Str("component", "billing_scheduler").Logger(),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start запускает планировщик в отдельной горутине
func (s *BillingScheduler) Start() {
	s.logger.Info().Msg("billing scheduler started")
	go s.run()
}

// Stop останавливает планировщик
func (s *BillingScheduler) Stop() {
	s.cancel()
	s.logger.Info().Msg("billing scheduler stopped")
}

// run — основной цикл планировщика
func (s *BillingScheduler) run() {
	for {
		now := time.Now().UTC()
		next := nextFirstOfMonth(now)
		s.logger.Info().
			Time("next_run", next).
			Msg("billing scheduler waiting for next run")

		select {
		case <-s.ctx.Done():
			return
		case <-time.After(time.Until(next)):
			s.logger.Info().Msg("billing scheduler run started")
			created, failed := s.runBilling()
			if failed > 0 {
				billingSchedulerRunTotal.WithLabelValues("error").Inc()
			} else {
				billingSchedulerRunTotal.WithLabelValues("success").Inc()
			}
			s.logger.Info().
				Int("created", created).
				Int("failed", failed).
				Msg("billing scheduler run completed")
		}
	}
}

// runBilling выполняет начисление для всех активных платных регистраций.
// Возвращает количество созданных и упавших записей.
func (s *BillingScheduler) runBilling() (created, failed int) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	regs, err := s.regRepo.ListActivePaid(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("ошибка получения активных платных регистраций")
		return 0, 1
	}

	s.logger.Info().Int("registrations", len(regs)).Msg("найдено активных платных регистраций")

	// Кэш тарифов операторов, чтобы не запрашивать несколько раз одного оператора
	tariffCache := make(map[uuid.UUID]string)

	for _, reg := range regs {
		tariff, ok := tariffCache[reg.OperatorID]
		if !ok {
			op, err := s.routingClient.GetOperator(ctx, &routingv1.GetOperatorRequest{Id: reg.OperatorID.String()})
			if err != nil {
				s.logger.Error().Err(err).
					Str("operator_id", reg.OperatorID.String()).
					Msg("не удалось получить тариф оператора")
				failed++
				continue
			}
			tariff = op.MonthlyTariffAmount
			tariffCache[reg.OperatorID] = tariff
		}

		if tariff == "" {
			s.logger.Warn().
				Str("operator_id", reg.OperatorID.String()).
				Str("registration_id", reg.ID.String()).
				Msg("тариф оператора не задан, пропускаем")
			continue
		}

		_, wasCreated, err := s.billingService.CreateBillingRecord(ctx, reg.ID, reg.ClientID, reg.OperatorID, tariff)
		if err != nil {
			s.logger.Error().Err(err).
				Str("registration_id", reg.ID.String()).
				Msg("ошибка создания billing record")
			failed++
			continue
		}
		if wasCreated {
			created++
			billingRecordsCreatedTotal.WithLabelValues("monthly").Inc()
		}
	}

	return created, failed
}

// nextFirstOfMonth возвращает время 00:01 UTC первого числа следующего месяца
func nextFirstOfMonth(now time.Time) time.Time {
	first := time.Date(now.UTC().Year(), now.UTC().Month()+1, 1, 0, 1, 0, 0, time.UTC)
	return first
}
