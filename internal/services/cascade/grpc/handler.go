package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	cascadev1 "github.com/smpp-server/smpp-server/api/proto/cascadev1"
	"github.com/smpp-server/smpp-server/internal/services/cascade/application"
	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
)

// Server реализует все три gRPC сервиса каскада
type Server struct {
	cascadev1.UnimplementedCascadeServiceServer
	cascadev1.UnimplementedChannelAdminServiceServer
	cascadev1.UnimplementedStrategyAdminServiceServer

	cascade    *application.CascadeService
	channels   *application.ChannelService
	strategies *application.StrategyService
	deliveries *application.DeliveryService
	ocs        domain.OperatorSupportRepository
}

func NewServer(
	cascade *application.CascadeService,
	channels *application.ChannelService,
	strategies *application.StrategyService,
	deliveries *application.DeliveryService,
	ocs domain.OperatorSupportRepository,
) *Server {
	return &Server{
		cascade:    cascade,
		channels:   channels,
		strategies: strategies,
		deliveries: deliveries,
		ocs:        ocs,
	}
}

// Register регистрирует все три сервиса на gRPC сервере
func (s *Server) Register(grpcServer *grpc.Server) {
	cascadev1.RegisterCascadeServiceServer(grpcServer, s)
	cascadev1.RegisterChannelAdminServiceServer(grpcServer, s)
	cascadev1.RegisterStrategyAdminServiceServer(grpcServer, s)
}

// ─── CascadeService ───────────────────────────────────────────────────────────

func (s *Server) CreateDelivery(ctx context.Context, req *cascadev1.CreateDeliveryRequest) (*cascadev1.DeliveryResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, fmt.Errorf("invalid client_id: %w", err)
	}
	strategyID, err := uuid.Parse(req.StrategyId)
	if err != nil {
		return nil, fmt.Errorf("invalid strategy_id: %w", err)
	}

	var messageID *uuid.UUID
	if req.MessageId != "" {
		mid, err := uuid.Parse(req.MessageId)
		if err == nil {
			messageID = &mid
		}
	}

	delivery, err := s.cascade.CreateDelivery(ctx, clientID, strategyID, req.Recipient, req.Text, req.SenderName, req.RequestId, messageID)
	if err != nil {
		return nil, fmt.Errorf("create delivery: %w", err)
	}

	return deliveryToProto(delivery), nil
}

func (s *Server) GetDelivery(ctx context.Context, req *cascadev1.GetDeliveryRequest) (*cascadev1.DeliveryResponse, error) {
	deliveryID, err := uuid.Parse(req.DeliveryId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid delivery_id format")
	}
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	delivery, err := s.deliveries.GetDelivery(ctx, deliveryID, clientID)
	if err != nil {
		// Маппим domain-ошибки на gRPC коды, иначе остаются Unknown → 500
		// на portal-gateway. Cross-tenant и not-exists свёрнуты в один
		// NotFound, чтобы не подтверждать существование чужого delivery_id.
		if errors.Is(err, domain.ErrDeliveryNotFound) {
			return nil, status.Error(codes.NotFound, "delivery not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return deliveryToProto(delivery), nil
}

func (s *Server) ListDeliveries(ctx context.Context, req *cascadev1.ListDeliveriesRequest) (*cascadev1.ListDeliveriesResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, fmt.Errorf("invalid client_id: %w", err)
	}

	filter := domain.DeliveryFilter{
		ClientID: clientID,
		Status:   req.Status,
		Page:     int(req.Page),
		PageSize: int(req.PageSize),
	}

	if req.StrategyId != "" {
		sid, err := uuid.Parse(req.StrategyId)
		if err == nil {
			filter.StrategyID = &sid
		}
	}
	if req.DateFrom != "" {
		if t, err := time.Parse(time.RFC3339, req.DateFrom); err == nil {
			filter.DateFrom = &t
		}
	}
	if req.DateTo != "" {
		if t, err := time.Parse(time.RFC3339, req.DateTo); err == nil {
			filter.DateTo = &t
		}
	}

	deliveries, total, err := s.deliveries.ListDeliveries(ctx, filter)
	if err != nil {
		return nil, err
	}

	resp := &cascadev1.ListDeliveriesResponse{
		Total: int32(total),
		Page:  req.Page,
	}
	for _, d := range deliveries {
		resp.Deliveries = append(resp.Deliveries, deliveryToProto(d))
	}
	return resp, nil
}

func (s *Server) GetDeliveryStats(ctx context.Context, req *cascadev1.GetDeliveryStatsRequest) (*cascadev1.DeliveryStatsResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, fmt.Errorf("invalid client_id: %w", err)
	}

	filter := domain.StatsFilter{
		ClientID: clientID,
		DateFrom: time.Now().AddDate(0, -1, 0),
		DateTo:   time.Now(),
	}
	if req.StrategyId != "" {
		sid, err := uuid.Parse(req.StrategyId)
		if err == nil {
			filter.StrategyID = &sid
		}
	}
	if req.DateFrom != "" {
		if t, err := time.Parse(time.RFC3339, req.DateFrom); err == nil {
			filter.DateFrom = t
		}
	}
	if req.DateTo != "" {
		if t, err := time.Parse(time.RFC3339, req.DateTo); err == nil {
			filter.DateTo = t
		}
	}

	stats, err := s.deliveries.GetStats(ctx, filter)
	if err != nil {
		return nil, err
	}

	resp := &cascadev1.DeliveryStatsResponse{
		TotalDeliveries: stats.Total,
		DeliveredCount:  stats.DeliveredCount,
		FailedCount:     stats.FailedCount,
		DeliveryRate:    fmt.Sprintf("%.2f", stats.DeliveryRate),
		AvgCost:         fmt.Sprintf("%.4f", stats.AvgCost),
		TotalCost:       fmt.Sprintf("%.4f", stats.TotalCost),
		Currency:        stats.Currency,
	}
	for _, cs := range stats.ChannelStats {
		resp.ChannelStats = append(resp.ChannelStats, &cascadev1.ChannelStatItem{
			ChannelType:    cs.ChannelType,
			AttemptCount:   cs.AttemptCount,
			DeliveredCount: cs.DeliveredCount,
			DeliveryRate:   fmt.Sprintf("%.2f", cs.DeliveryRate),
			AvgLatencyMs:   fmt.Sprintf("%.0f", cs.AvgLatencyMs),
			AvgCost:        fmt.Sprintf("%.4f", cs.AvgCost),
		})
	}
	return resp, nil
}

// ─── ChannelAdminService ──────────────────────────────────────────────────────

func (s *Server) ListChannels(ctx context.Context, _ *cascadev1.ListChannelsRequest) (*cascadev1.ListChannelsResponse, error) {
	channels, err := s.channels.List(ctx)
	if err != nil {
		return nil, err
	}
	resp := &cascadev1.ListChannelsResponse{}
	for _, ch := range channels {
		resp.Channels = append(resp.Channels, channelToProto(ch))
	}
	return resp, nil
}

func (s *Server) GetChannel(ctx context.Context, req *cascadev1.GetChannelRequest) (*cascadev1.ChannelResponse, error) {
	id, err := uuid.Parse(req.ChannelId)
	if err != nil {
		return nil, fmt.Errorf("invalid channel_id: %w", err)
	}
	ch, err := s.channels.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return channelToProto(ch), nil
}

func (s *Server) CreateChannel(ctx context.Context, req *cascadev1.CreateChannelRequest) (*cascadev1.ChannelResponse, error) {
	ct, err := domain.ChannelTypeFromString(req.ChannelType)
	if err != nil {
		return nil, fmt.Errorf("invalid channel_type: %w", err)
	}

	var config map[string]interface{}
	if req.ConfigJson != "" {
		_ = json.Unmarshal([]byte(req.ConfigJson), &config)
	}

	ch, err := s.channels.Create(ctx, ct, req.Name, req.Description, config)
	if err != nil {
		return nil, err
	}
	return channelToProto(ch), nil
}

func (s *Server) UpdateChannel(ctx context.Context, req *cascadev1.UpdateChannelRequest) (*cascadev1.ChannelResponse, error) {
	id, err := uuid.Parse(req.ChannelId)
	if err != nil {
		return nil, fmt.Errorf("invalid channel_id: %w", err)
	}

	var config map[string]interface{}
	if req.ConfigJson != "" {
		_ = json.Unmarshal([]byte(req.ConfigJson), &config)
	}

	ch, err := s.channels.Update(ctx, id, req.Name, req.Description, config)
	if err != nil {
		return nil, err
	}
	return channelToProto(ch), nil
}

func (s *Server) ToggleChannel(ctx context.Context, req *cascadev1.ToggleChannelRequest) (*cascadev1.ChannelResponse, error) {
	id, err := uuid.Parse(req.ChannelId)
	if err != nil {
		return nil, fmt.Errorf("invalid channel_id: %w", err)
	}
	ch, err := s.channels.Toggle(ctx, id, req.Active)
	if err != nil {
		return nil, err
	}
	return channelToProto(ch), nil
}

// ─── StrategyAdminService ─────────────────────────────────────────────────────

func (s *Server) ListStrategies(ctx context.Context, req *cascadev1.ListStrategiesRequest) (*cascadev1.ListStrategiesResponse, error) {
	strategies, err := s.strategies.List(ctx, req.ActiveOnly)
	if err != nil {
		return nil, err
	}
	resp := &cascadev1.ListStrategiesResponse{}
	for _, str := range strategies {
		resp.Strategies = append(resp.Strategies, strategyToProto(str))
	}
	return resp, nil
}

func (s *Server) GetStrategy(ctx context.Context, req *cascadev1.GetStrategyRequest) (*cascadev1.StrategyResponse, error) {
	id, err := uuid.Parse(req.StrategyId)
	if err != nil {
		return nil, fmt.Errorf("invalid strategy_id: %w", err)
	}
	str, err := s.strategies.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return strategyToProto(str), nil
}

func (s *Server) CreateStrategy(ctx context.Context, req *cascadev1.CreateStrategyRequest) (*cascadev1.StrategyResponse, error) {
	mode := domain.StrategyMode(req.Mode)
	if !mode.IsValid() {
		return nil, fmt.Errorf("invalid mode: %s", req.Mode)
	}

	input := application.CreateStrategyInput{
		Name:        req.Name,
		Description: req.Description,
		Mode:        mode,
	}
	for _, step := range req.Steps {
		channelID, err := uuid.Parse(step.ChannelId)
		if err != nil {
			return nil, fmt.Errorf("invalid channel_id in step: %w", err)
		}
		input.Steps = append(input.Steps, application.CreateStrategyStepInput{
			ChannelID: channelID,
			StepOrder: int(step.StepOrder),
			TimeoutS:  int(step.TimeoutS),
			Billable:  step.Billable,
		})
	}

	str, err := s.strategies.Create(ctx, input)
	if err != nil {
		return nil, err
	}
	return strategyToProto(str), nil
}

func (s *Server) UpdateStrategy(ctx context.Context, req *cascadev1.UpdateStrategyRequest) (*cascadev1.StrategyResponse, error) {
	id, err := uuid.Parse(req.StrategyId)
	if err != nil {
		return nil, fmt.Errorf("invalid strategy_id: %w", err)
	}

	mode := domain.StrategyMode(req.Mode)
	input := application.CreateStrategyInput{
		Name:        req.Name,
		Description: req.Description,
		Mode:        mode,
	}
	for _, step := range req.Steps {
		channelID, err := uuid.Parse(step.ChannelId)
		if err != nil {
			return nil, fmt.Errorf("invalid channel_id in step: %w", err)
		}
		input.Steps = append(input.Steps, application.CreateStrategyStepInput{
			ChannelID: channelID,
			StepOrder: int(step.StepOrder),
			TimeoutS:  int(step.TimeoutS),
			Billable:  step.Billable,
		})
	}

	str, err := s.strategies.Update(ctx, id, input)
	if err != nil {
		return nil, err
	}
	return strategyToProto(str), nil
}

func (s *Server) DeleteStrategy(ctx context.Context, req *cascadev1.DeleteStrategyRequest) (*cascadev1.DeleteStrategyResponse, error) {
	id, err := uuid.Parse(req.StrategyId)
	if err != nil {
		return nil, fmt.Errorf("invalid strategy_id: %w", err)
	}
	if err := s.strategies.Delete(ctx, id); err != nil {
		return &cascadev1.DeleteStrategyResponse{Success: false}, err
	}
	return &cascadev1.DeleteStrategyResponse{Success: true}, nil
}

func (s *Server) GetOperatorChannelSupport(ctx context.Context, req *cascadev1.GetOCSRequest) (*cascadev1.GetOCSResponse, error) {
	var operatorID *uuid.UUID
	if req.OperatorId != "" {
		id, err := uuid.Parse(req.OperatorId)
		if err != nil {
			return nil, fmt.Errorf("invalid operator_id: %w", err)
		}
		operatorID = &id
	}

	entries, err := s.ocs.List(ctx, operatorID)
	if err != nil {
		return nil, err
	}

	resp := &cascadev1.GetOCSResponse{}
	for _, e := range entries {
		resp.Entries = append(resp.Entries, &cascadev1.OCSEntry{
			OperatorId:  e.OperatorID.String(),
			ChannelType: string(e.ChannelType),
			Supported:   e.Supported,
			Notes:       e.Notes,
		})
	}
	return resp, nil
}

func (s *Server) UpdateOperatorChannelSupport(ctx context.Context, req *cascadev1.UpdateOCSRequest) (*cascadev1.UpdateOCSResponse, error) {
	operatorID, err := uuid.Parse(req.OperatorId)
	if err != nil {
		return nil, fmt.Errorf("invalid operator_id: %w", err)
	}
	ct, err := domain.ChannelTypeFromString(req.ChannelType)
	if err != nil {
		return nil, fmt.Errorf("invalid channel_type: %w", err)
	}

	ocs := &domain.OperatorChannelSupport{
		OperatorID:  operatorID,
		ChannelType: ct,
		Supported:   req.Supported,
		Notes:       req.Notes,
	}
	if err := s.ocs.Upsert(ctx, ocs); err != nil {
		return nil, err
	}

	return &cascadev1.UpdateOCSResponse{
		Entry: &cascadev1.OCSEntry{
			OperatorId:  req.OperatorId,
			ChannelType: req.ChannelType,
			Supported:   req.Supported,
			Notes:       req.Notes,
		},
	}, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func channelToProto(ch *domain.ChannelConfig) *cascadev1.ChannelResponse {
	return &cascadev1.ChannelResponse{
		ChannelId:   ch.ID.String(),
		ChannelType: string(ch.ChannelType),
		Name:        ch.Name,
		Description: ch.Description,
		Active:      ch.Active,
		CreatedAt:   timestamppb.New(ch.CreatedAt),
		UpdatedAt:   timestamppb.New(ch.UpdatedAt),
	}
}

func strategyToProto(str *domain.DeliveryStrategy) *cascadev1.StrategyResponse {
	resp := &cascadev1.StrategyResponse{
		StrategyId:  str.ID.String(),
		Name:        str.Name,
		Description: str.Description,
		Mode:        string(str.Mode),
		Active:      str.Active,
		CreatedAt:   timestamppb.New(str.CreatedAt),
		UpdatedAt:   timestamppb.New(str.UpdatedAt),
	}
	for _, step := range str.Steps {
		resp.Steps = append(resp.Steps, &cascadev1.StrategyStepInfo{
			ChannelId:   step.ChannelID.String(),
			ChannelType: string(step.ChannelType),
			ChannelName: step.ChannelName,
			StepOrder:   int32(step.StepOrder),
			TimeoutS:    int32(step.TimeoutS),
			Billable:    step.Billable,
		})
	}
	return resp
}

func deliveryToProto(d *domain.Delivery) *cascadev1.DeliveryResponse {
	resp := &cascadev1.DeliveryResponse{
		Id:           d.ID.String(),
		ClientId:     d.ClientID.String(),
		StrategyId:   d.StrategyID.String(),
		Recipient:    d.Recipient,
		Status:       string(d.Status),
		DeliveredVia: d.DeliveredVia,
		TotalCost:    fmt.Sprintf("%.4f", d.TotalCost),
		Currency:     d.Currency,
		CreatedAt:    timestamppb.New(d.CreatedAt),
		UpdatedAt:    timestamppb.New(d.UpdatedAt),
	}
	for _, a := range d.Attempts {
		info := &cascadev1.AttemptInfo{
			Id:           a.ID.String(),
			ChannelType:  a.ChannelType,
			StepOrder:    int32(a.StepOrder),
			Status:       string(a.Status),
			Cost:         fmt.Sprintf("%.4f", a.Cost),
			Currency:     a.Currency,
			ErrorMessage: a.ErrorMessage,
		}
		if a.SentAt != nil {
			info.SentAt = timestamppb.New(*a.SentAt)
		}
		if a.ResultAt != nil {
			info.ResultAt = timestamppb.New(*a.ResultAt)
		}
		resp.Attempts = append(resp.Attempts, info)
	}
	return resp
}
