package grpc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	tarificationv1 "github.com/smpp-server/smpp-server/api/proto/tarificationv1"
	"github.com/smpp-server/smpp-server/internal/services/tarification/application"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// Server реализует gRPC сервер тарификации
type Server struct {
	tarificationv1.UnimplementedTarificationServiceServer
	tarificationService  *application.TarificationService
	senderService        *application.SenderService
	tariffPlanService    *application.TariffPlanService
	senderBillingService *application.SenderBillingService
	providerPlanRepo     domain.ProviderTariffPlanRepository
	providerPeriodRepo   domain.ProviderTariffPeriodRepository
	providerTierRepo     domain.ProviderTariffTierRepository
	marginRepo           domain.MarginReportRepository
	providerTarification *application.ProviderTarificationService
}

// NewServer создает новый gRPC сервер
func NewServer(
	tarificationService *application.TarificationService,
	senderService *application.SenderService,
	tariffPlanService *application.TariffPlanService,
	senderBillingService *application.SenderBillingService,
) *Server {
	return &Server{
		tarificationService:  tarificationService,
		senderService:        senderService,
		tariffPlanService:    tariffPlanService,
		senderBillingService: senderBillingService,
	}
}

// SetProviderTarificationDeps устанавливает зависимости для провайдерской тарификации
func (s *Server) SetProviderTarificationDeps(
	planRepo domain.ProviderTariffPlanRepository,
	periodRepo domain.ProviderTariffPeriodRepository,
	tierRepo domain.ProviderTariffTierRepository,
	marginRepo domain.MarginReportRepository,
	providerTarification *application.ProviderTarificationService,
) {
	s.providerPlanRepo = planRepo
	s.providerPeriodRepo = periodRepo
	s.providerTierRepo = tierRepo
	s.marginRepo = marginRepo
	s.providerTarification = providerTarification
}

// TarifyMessage тарифицирует сообщение
func (s *Server) TarifyMessage(ctx context.Context, req *tarificationv1.TarifyMessageRequest) (*tarificationv1.TarifyMessageResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id: %v", err)
	}
	messageID, err := uuid.Parse(req.MessageId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid message_id: %v", err)
	}
	operatorID, err := uuid.Parse(req.OperatorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid operator_id: %v", err)
	}

	result, err := s.tarificationService.TarifyMessage(ctx, &application.TarifyMessageRequest{
		ClientID:       clientID,
		MessageID:      messageID,
		OperatorID:     operatorID,
		SenderName:     req.SenderName,
		SegmentCount:   int(req.SegmentCount),
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		log.Error().Err(err).Msg("tarification failed")
		return nil, status.Errorf(codes.Internal, "tarification failed: %v", err)
	}

	return &tarificationv1.TarifyMessageResponse{
		Approved:         result.Approved,
		TotalAmount:      result.TotalAmount,
		Currency:         result.Currency,
		Strategy:         result.Strategy,
		TariffPlanId:     result.TariffPlanID,
		RejectionReason:  result.RejectionReason,
		ThresholdCrossed: result.ThresholdCrossed,
		RecalcAmount:     result.RecalcAmount,
	}, nil
}

// CreateSenderRegistration создает регистрацию имени отправителя
func (s *Server) CreateSenderRegistration(ctx context.Context, req *tarificationv1.CreateSenderRegistrationRequest) (*tarificationv1.SenderRegistration, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id: %v", err)
	}
	operatorID, err := uuid.Parse(req.OperatorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid operator_id: %v", err)
	}

	reg, err := s.senderService.CreateRegistration(ctx, clientID, operatorID, req.SenderName, domain.SenderRegistrationType(req.Type))
	if err != nil {
		return nil, mapDomainError(err)
	}

	return senderRegToProto(reg), nil
}

// ListSenderRegistrations получает список регистраций
func (s *Server) ListSenderRegistrations(ctx context.Context, req *tarificationv1.ListSenderRegistrationsRequest) (*tarificationv1.ListSenderRegistrationsResponse, error) {
	var clientID, operatorID *uuid.UUID
	if req.ClientId != "" {
		id, err := uuid.Parse(req.ClientId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid client_id: %v", err)
		}
		clientID = &id
	}
	if req.OperatorId != "" {
		id, err := uuid.Parse(req.OperatorId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid operator_id: %v", err)
		}
		operatorID = &id
	}

	limit, offset := int(req.Limit), int(req.Offset)
	if limit <= 0 {
		limit = 50
	}

	regs, total, err := s.senderService.ListRegistrations(ctx, clientID, operatorID, limit, offset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list failed: %v", err)
	}

	protoRegs := make([]*tarificationv1.SenderRegistration, len(regs))
	for i, r := range regs {
		protoRegs[i] = senderRegToProto(r)
	}

	return &tarificationv1.ListSenderRegistrationsResponse{
		Registrations: protoRegs,
		Total:         int32(total),
	}, nil
}

// UpdateSenderRegistration обновляет регистрацию
func (s *Server) UpdateSenderRegistration(ctx context.Context, req *tarificationv1.UpdateSenderRegistrationRequest) (*tarificationv1.SenderRegistration, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid id: %v", err)
	}

	var regType *domain.SenderRegistrationType
	if req.Type != "" {
		t := domain.SenderRegistrationType(req.Type)
		regType = &t
	}

	reg, err := s.senderService.UpdateRegistration(ctx, id, domain.SenderRegistrationStatus(req.Status), regType)
	if err != nil {
		return nil, mapDomainError(err)
	}

	return senderRegToProto(reg), nil
}

// CreateTariffPlan создает тарифный план
func (s *Server) CreateTariffPlan(ctx context.Context, req *tarificationv1.CreateTariffPlanRequest) (*tarificationv1.TariffPlan, error) {
	operatorID, err := uuid.Parse(req.OperatorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid operator_id: %v", err)
	}

	plan, err := s.tariffPlanService.CreatePlan(ctx, operatorID, domain.SenderCategory(req.SenderCategory), domain.TarificationStrategy(req.Strategy))
	if err != nil {
		return nil, mapDomainError(err)
	}

	return tariffPlanToProto(plan), nil
}

// GetTariffPlan получает тарифный план по ID
func (s *Server) GetTariffPlan(ctx context.Context, req *tarificationv1.GetTariffPlanRequest) (*tarificationv1.TariffPlan, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid id: %v", err)
	}

	plan, err := s.tariffPlanService.GetPlan(ctx, id)
	if err != nil {
		return nil, mapDomainError(err)
	}

	return tariffPlanToProto(plan), nil
}

// ListTariffPlans получает список тарифных планов
func (s *Server) ListTariffPlans(ctx context.Context, req *tarificationv1.ListTariffPlansRequest) (*tarificationv1.ListTariffPlansResponse, error) {
	var operatorID *uuid.UUID
	if req.OperatorId != "" {
		id, err := uuid.Parse(req.OperatorId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid operator_id: %v", err)
		}
		operatorID = &id
	}

	limit, offset := int(req.Limit), int(req.Offset)
	if limit <= 0 {
		limit = 50
	}

	plans, total, err := s.tariffPlanService.ListPlans(ctx, operatorID, req.ActiveOnly, limit, offset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list failed: %v", err)
	}

	protoPlans := make([]*tarificationv1.TariffPlan, len(plans))
	for i, p := range plans {
		protoPlans[i] = tariffPlanToProto(p)
	}

	return &tarificationv1.ListTariffPlansResponse{
		Plans: protoPlans,
		Total: int32(total),
	}, nil
}

// UpdateTariffPlan обновляет тарифный план
func (s *Server) UpdateTariffPlan(ctx context.Context, req *tarificationv1.UpdateTariffPlanRequest) (*tarificationv1.TariffPlan, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid id: %v", err)
	}

	plan, err := s.tariffPlanService.UpdatePlan(ctx, id, req.Active)
	if err != nil {
		return nil, mapDomainError(err)
	}

	return tariffPlanToProto(plan), nil
}

// CreateTariffPeriod создает тарифный период
func (s *Server) CreateTariffPeriod(ctx context.Context, req *tarificationv1.CreateTariffPeriodRequest) (*tarificationv1.TariffPeriod, error) {
	planID, err := uuid.Parse(req.TariffPlanId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid tariff_plan_id: %v", err)
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid start_date format: %v", err)
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid end_date format: %v", err)
	}

	period, err := s.tariffPlanService.CreatePeriod(ctx, planID, startDate, endDate)
	if err != nil {
		return nil, mapDomainError(err)
	}

	return tariffPeriodToProto(period), nil
}

// CreateTariffTier создает порог тарификации
func (s *Server) CreateTariffTier(ctx context.Context, req *tarificationv1.CreateTariffTierRequest) (*tarificationv1.TariffTier, error) {
	periodID, err := uuid.Parse(req.TariffPeriodId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid tariff_period_id: %v", err)
	}

	tier, err := s.tariffPlanService.CreateTier(ctx, periodID, int(req.FromCount), req.PricePerSegment)
	if err != nil {
		return nil, mapDomainError(err)
	}

	return tariffTierToProto(tier), nil
}

// UpdateTariffTier обновляет порог тарификации
func (s *Server) UpdateTariffTier(ctx context.Context, req *tarificationv1.UpdateTariffTierRequest) (*tarificationv1.TariffTier, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid id: %v", err)
	}

	tier, err := s.tariffPlanService.UpdateTier(ctx, id, int(req.FromCount), req.PricePerSegment)
	if err != nil {
		return nil, mapDomainError(err)
	}

	return tariffTierToProto(tier), nil
}

// CreatePricingPeriod создает ценовой период
func (s *Server) CreatePricingPeriod(ctx context.Context, req *tarificationv1.CreatePricingPeriodRequest) (*tarificationv1.PricingPeriod, error) {
	tariffPeriodID, err := uuid.Parse(req.TariffPeriodId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid tariff_period_id: %v", err)
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid start_date format: %v", err)
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid end_date format: %v", err)
	}

	pp, err := s.tariffPlanService.CreatePricingPeriod(ctx, tariffPeriodID, startDate, endDate)
	if err != nil {
		return nil, mapDomainError(err)
	}

	return pricingPeriodToProto(pp), nil
}

// CreatePrepaidFee создает предоплату
func (s *Server) CreatePrepaidFee(ctx context.Context, req *tarificationv1.CreatePrepaidFeeRequest) (*tarificationv1.PrepaidFee, error) {
	planID, err := uuid.Parse(req.TariffPlanId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid tariff_plan_id: %v", err)
	}
	periodID, err := uuid.Parse(req.TariffPeriodId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid tariff_period_id: %v", err)
	}

	fee, err := s.tariffPlanService.CreatePrepaidFee(ctx, planID, periodID, req.Amount, req.Currency)
	if err != nil {
		return nil, mapDomainError(err)
	}

	return prepaidFeeToProto(fee), nil
}

// GetUsageCounter получает счётчик использования
func (s *Server) GetUsageCounter(ctx context.Context, req *tarificationv1.GetUsageCounterRequest) (*tarificationv1.UsageCounter, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id: %v", err)
	}
	planID, err := uuid.Parse(req.TariffPlanId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid tariff_plan_id: %v", err)
	}
	periodID, err := uuid.Parse(req.TariffPeriodId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid tariff_period_id: %v", err)
	}

	counter, err := s.tarificationService.GetUsageCounter(ctx, clientID, planID, periodID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get counter: %v", err)
	}

	return usageCounterToProto(counter), nil
}

// ListUsageCounters получает список счётчиков
func (s *Server) ListUsageCounters(ctx context.Context, req *tarificationv1.ListUsageCountersRequest) (*tarificationv1.ListUsageCountersResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id: %v", err)
	}

	var planID *uuid.UUID
	if req.TariffPlanId != "" {
		id, parseErr := uuid.Parse(req.TariffPlanId)
		if parseErr != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid tariff_plan_id: %v", parseErr)
		}
		planID = &id
	}

	limit, offset := int(req.Limit), int(req.Offset)
	if limit <= 0 {
		limit = 50
	}

	counters, total, err := s.tarificationService.ListUsageCounters(ctx, clientID, planID, limit, offset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list failed: %v", err)
	}

	protoCounters := make([]*tarificationv1.UsageCounter, len(counters))
	for i, c := range counters {
		protoCounters[i] = usageCounterToProto(c)
	}

	return &tarificationv1.ListUsageCountersResponse{
		Counters: protoCounters,
		Total:    int32(total),
	}, nil
}

// Helpers

func mapDomainError(err error) error {
	switch err {
	case domain.ErrTariffPlanNotFound, domain.ErrTariffPeriodNotFound,
		domain.ErrTariffTierNotFound, domain.ErrPricingPeriodNotFound,
		domain.ErrPrepaidFeeNotFound, domain.ErrSenderRegistrationNotFound:
		return status.Errorf(codes.NotFound, "%v", err)
	case domain.ErrTariffPlanDuplicate, domain.ErrSenderRegistrationDuplicate,
		domain.ErrTariffTierDuplicate, domain.ErrIdempotencyKeyExists:
		return status.Errorf(codes.AlreadyExists, "%v", err)
	case domain.ErrTariffPlanHasActivePeriod, domain.ErrTariffTierPeriodActive,
		domain.ErrPricingPeriodOutOfBounds, domain.ErrInsufficientBalance:
		return status.Errorf(codes.FailedPrecondition, "%v", err)
	case domain.ErrTariffPlanInvalidStrategy, domain.ErrTariffPlanInvalidCategory,
		domain.ErrSenderRegistrationInvalidType, domain.ErrTariffPeriodInvalidDates,
		domain.ErrTariffTierInvalidPrice:
		return status.Errorf(codes.InvalidArgument, "%v", err)
	default:
		return status.Errorf(codes.Internal, "%v", err)
	}
}

func senderRegToProto(r *domain.SenderRegistration) *tarificationv1.SenderRegistration {
	return &tarificationv1.SenderRegistration{
		Id:         r.ID.String(),
		ClientId:   r.ClientID.String(),
		OperatorId: r.OperatorID.String(),
		SenderName: r.SenderName,
		Type:       string(r.Type),
		Status:     string(r.Status),
		CreatedAt:  timestamppb.New(r.CreatedAt),
		UpdatedAt:  timestamppb.New(r.UpdatedAt),
	}
}

func tariffPlanToProto(p *domain.TariffPlan) *tarificationv1.TariffPlan {
	return &tarificationv1.TariffPlan{
		Id:             p.ID.String(),
		OperatorId:     p.OperatorID.String(),
		SenderCategory: string(p.SenderCategory),
		Strategy:       string(p.Strategy),
		Active:         p.Active,
		CreatedAt:      timestamppb.New(p.CreatedAt),
		UpdatedAt:      timestamppb.New(p.UpdatedAt),
	}
}

func tariffPeriodToProto(p *domain.TariffPeriod) *tarificationv1.TariffPeriod {
	return &tarificationv1.TariffPeriod{
		Id:           p.ID.String(),
		TariffPlanId: p.TariffPlanID.String(),
		StartDate:    p.StartDate.Format("2006-01-02"),
		EndDate:      p.EndDate.Format("2006-01-02"),
		CreatedAt:    timestamppb.New(p.CreatedAt),
	}
}

func tariffTierToProto(t *domain.TariffTier) *tarificationv1.TariffTier {
	return &tarificationv1.TariffTier{
		Id:              t.ID.String(),
		TariffPeriodId:  t.TariffPeriodID.String(),
		FromCount:       int32(t.FromCount),
		PricePerSegment: t.PricePerSegment,
	}
}

func pricingPeriodToProto(p *domain.PricingPeriod) *tarificationv1.PricingPeriod {
	return &tarificationv1.PricingPeriod{
		Id:             p.ID.String(),
		TariffPeriodId: p.TariffPeriodID.String(),
		StartDate:      p.StartDate.Format("2006-01-02"),
		EndDate:        p.EndDate.Format("2006-01-02"),
		CreatedAt:      timestamppb.New(p.CreatedAt),
	}
}

func prepaidFeeToProto(f *domain.PrepaidFee) *tarificationv1.PrepaidFee {
	proto := &tarificationv1.PrepaidFee{
		Id:             f.ID.String(),
		TariffPlanId:   f.TariffPlanID.String(),
		TariffPeriodId: f.TariffPeriodID.String(),
		Amount:         f.Amount,
		Currency:       f.Currency,
		Charged:        f.Charged,
		CreatedAt:      timestamppb.New(f.CreatedAt),
	}
	if f.ChargedAt != nil {
		proto.ChargedAt = timestamppb.New(*f.ChargedAt)
	}
	return proto
}

func usageCounterToProto(c *domain.UsageCounter) *tarificationv1.UsageCounter {
	return &tarificationv1.UsageCounter{
		Id:             c.ID.String(),
		ClientId:       c.ClientID.String(),
		TariffPlanId:   c.TariffPlanID.String(),
		TariffPeriodId: c.TariffPeriodID.String(),
		SegmentCount:   int32(c.SegmentCount),
		UpdatedAt:      timestamppb.New(c.UpdatedAt),
	}
}

// ==================== Provider Tariff Plan methods ====================

func (s *Server) CreateProviderTariffPlan(ctx context.Context, req *tarificationv1.CreateProviderTariffPlanRequest) (*tarificationv1.ProviderTariffPlanProto, error) {
	providerID, err := uuid.Parse(req.ProviderId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid provider_id: %v", err)
	}
	operatorID, err := uuid.Parse(req.OperatorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid operator_id: %v", err)
	}

	plan := domain.NewProviderTariffPlan(providerID, operatorID, domain.TarificationStrategy(req.Strategy))
	if err := plan.Validate(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	if err := s.providerPlanRepo.Create(ctx, plan); err != nil {
		return nil, status.Errorf(codes.Internal, "create failed: %v", err)
	}

	return providerTariffPlanToProto(plan), nil
}

func (s *Server) GetProviderTariffPlan(ctx context.Context, req *tarificationv1.GetProviderTariffPlanRequest) (*tarificationv1.ProviderTariffPlanProto, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid id: %v", err)
	}

	plan, err := s.providerPlanRepo.GetByID(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "not found: %v", err)
	}

	return providerTariffPlanToProto(plan), nil
}

func (s *Server) ListProviderTariffPlans(ctx context.Context, req *tarificationv1.ListProviderTariffPlansRequest) (*tarificationv1.ListProviderTariffPlansResponse, error) {
	var providerID *uuid.UUID
	if req.ProviderId != "" {
		parsed, err := uuid.Parse(req.ProviderId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid provider_id: %v", err)
		}
		providerID = &parsed
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}
	offset := int(req.Offset)

	plans, total, err := s.providerPlanRepo.List(ctx, providerID, req.ActiveOnly, limit, offset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list failed: %v", err)
	}

	resp := &tarificationv1.ListProviderTariffPlansResponse{Total: int32(total)}
	for _, p := range plans {
		resp.Plans = append(resp.Plans, providerTariffPlanToProto(p))
	}
	return resp, nil
}

func (s *Server) UpdateProviderTariffPlan(ctx context.Context, req *tarificationv1.UpdateProviderTariffPlanRequest) (*tarificationv1.ProviderTariffPlanProto, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid id: %v", err)
	}

	plan, err := s.providerPlanRepo.GetByID(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "not found: %v", err)
	}

	plan.Active = req.Active
	if err := s.providerPlanRepo.Update(ctx, plan); err != nil {
		return nil, status.Errorf(codes.Internal, "update failed: %v", err)
	}

	return providerTariffPlanToProto(plan), nil
}

func (s *Server) CreateProviderTariffPeriod(ctx context.Context, req *tarificationv1.CreateProviderTariffPeriodRequest) (*tarificationv1.ProviderTariffPeriodProto, error) {
	planID, err := uuid.Parse(req.ProviderTariffPlanId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid plan_id: %v", err)
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid start_date: %v", err)
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid end_date: %v", err)
	}

	period := &domain.ProviderTariffPeriod{
		ID:                   uuid.New(),
		ProviderTariffPlanID: planID,
		StartDate:            startDate,
		EndDate:              endDate,
		CreatedAt:            time.Now(),
	}

	if err := s.providerPeriodRepo.Create(ctx, period); err != nil {
		return nil, status.Errorf(codes.Internal, "create failed: %v", err)
	}

	return &tarificationv1.ProviderTariffPeriodProto{
		Id:                   period.ID.String(),
		ProviderTariffPlanId: period.ProviderTariffPlanID.String(),
		StartDate:            req.StartDate,
		EndDate:              req.EndDate,
		CreatedAt:            timestamppb.New(period.CreatedAt),
	}, nil
}

func (s *Server) CreateProviderTariffTier(ctx context.Context, req *tarificationv1.CreateProviderTariffTierRequest) (*tarificationv1.ProviderTariffTierProto, error) {
	periodID, err := uuid.Parse(req.ProviderTariffPeriodId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid period_id: %v", err)
	}

	tier := &domain.ProviderTariffTier{
		ID:                     uuid.New(),
		ProviderTariffPeriodID: periodID,
		FromCount:              int(req.FromCount),
		PricePerSegment:        req.PricePerSegment,
	}

	if err := s.providerTierRepo.Create(ctx, tier); err != nil {
		return nil, status.Errorf(codes.Internal, "create failed: %v", err)
	}

	return &tarificationv1.ProviderTariffTierProto{
		Id:                     tier.ID.String(),
		ProviderTariffPeriodId: tier.ProviderTariffPeriodID.String(),
		FromCount:              int32(tier.FromCount),
		PricePerSegment:        tier.PricePerSegment,
	}, nil
}

func (s *Server) UpdateProviderTariffTier(ctx context.Context, req *tarificationv1.UpdateProviderTariffTierRequest) (*tarificationv1.ProviderTariffTierProto, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid id: %v", err)
	}

	tier, err := s.providerTierRepo.GetByID(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "not found: %v", err)
	}

	tier.FromCount = int(req.FromCount)
	tier.PricePerSegment = req.PricePerSegment

	if err := s.providerTierRepo.Update(ctx, tier); err != nil {
		return nil, status.Errorf(codes.Internal, "update failed: %v", err)
	}

	return &tarificationv1.ProviderTariffTierProto{
		Id:                     tier.ID.String(),
		ProviderTariffPeriodId: tier.ProviderTariffPeriodID.String(),
		FromCount:              int32(tier.FromCount),
		PricePerSegment:        tier.PricePerSegment,
	}, nil
}

// ==================== Margin Report ====================

func (s *Server) GetMarginReport(ctx context.Context, req *tarificationv1.MarginReportRequest) (*tarificationv1.MarginReportResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id")
	}
	from, err := time.Parse("2006-01-02", req.FromDate)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid from_date format (YYYY-MM-DD)")
	}
	to, err := time.Parse("2006-01-02", req.ToDate)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid to_date format (YYYY-MM-DD)")
	}

	entries, err := s.marginRepo.GetMarginReport(ctx, clientID, from, to)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "margin report: %v", err)
	}

	resp := &tarificationv1.MarginReportResponse{}
	for _, e := range entries {
		resp.Entries = append(resp.Entries, &tarificationv1.MarginReportEntry{
			OperatorId:   e.OperatorID.String(),
			OperatorName: e.OperatorName,
			ProviderId:   e.ProviderID.String(),
			ProviderName: e.ProviderName,
			Segments:     int32(e.Segments),
			Revenue:      e.Revenue,
			Cost:         e.Cost,
			Margin:       e.Margin,
		})
	}

	return resp, nil
}

// ==================== Provider Tariff Proto helpers ====================

func providerTariffPlanToProto(p *domain.ProviderTariffPlan) *tarificationv1.ProviderTariffPlanProto {
	return &tarificationv1.ProviderTariffPlanProto{
		Id:         p.ID.String(),
		ProviderId: p.ProviderID.String(),
		OperatorId: p.OperatorID.String(),
		Strategy:   string(p.Strategy),
		Active:     p.Active,
		CreatedAt:  timestamppb.New(p.CreatedAt),
		UpdatedAt:  timestamppb.New(p.UpdatedAt),
	}
}

// ==================== Sender Name Billing ====================

// CreateSenderBillingRecord создаёт billing-запись для платного имени отправителя
func (s *Server) CreateSenderBillingRecord(ctx context.Context, req *tarificationv1.CreateSenderBillingRecordRequest) (*tarificationv1.CreateSenderBillingRecordResponse, error) {
	if req.SenderRegistrationId == "" {
		return nil, status.Error(codes.InvalidArgument, "sender_registration_id is required")
	}
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	if req.OperatorId == "" {
		return nil, status.Error(codes.InvalidArgument, "operator_id is required")
	}
	if req.Amount == "" {
		return nil, status.Error(codes.InvalidArgument, "amount is required")
	}

	regID, err := uuid.Parse(req.SenderRegistrationId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid sender_registration_id")
	}
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id")
	}
	operatorID, err := uuid.Parse(req.OperatorId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid operator_id")
	}

	record, created, err := s.senderBillingService.CreateBillingRecord(ctx, regID, clientID, operatorID, req.Amount)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create billing record: %v", err)
	}

	return &tarificationv1.CreateSenderBillingRecordResponse{
		Record:        senderBillingRecordToProto(record),
		AlreadyExisted: !created,
	}, nil
}

// ListSenderBillingRecords возвращает историю начислений по регистрации
func (s *Server) ListSenderBillingRecords(ctx context.Context, req *tarificationv1.ListSenderBillingRecordsRequest) (*tarificationv1.ListSenderBillingRecordsResponse, error) {
	if req.SenderRegistrationId == "" {
		return nil, status.Error(codes.InvalidArgument, "sender_registration_id is required")
	}

	regID, err := uuid.Parse(req.SenderRegistrationId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid sender_registration_id")
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}
	offset := int(req.Offset)

	records, total, err := s.senderBillingService.ListBillingRecords(ctx, regID, limit, offset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list billing records: %v", err)
	}

	protoRecords := make([]*tarificationv1.SenderNameBillingRecordProto, len(records))
	for i, r := range records {
		protoRecords[i] = senderBillingRecordToProto(r)
	}

	return &tarificationv1.ListSenderBillingRecordsResponse{
		Records: protoRecords,
		Total:   int32(total),
	}, nil
}

func senderBillingRecordToProto(r *domain.SenderNameBillingRecord) *tarificationv1.SenderNameBillingRecordProto {
	return &tarificationv1.SenderNameBillingRecordProto{
		Id:                   r.ID.String(),
		SenderRegistrationId: r.SenderRegistrationID.String(),
		ClientId:             r.ClientID.String(),
		OperatorId:           r.OperatorID.String(),
		BillingMonth:         r.BillingMonth.Format("2006-01-02"),
		Amount:               r.Amount,
		CreatedAt:            timestamppb.New(r.CreatedAt),
	}
}
