package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	campaignv1 "github.com/smpp-server/smpp-server/api/proto/campaignv1"
	"github.com/smpp-server/smpp-server/internal/services/campaign/application"
	"github.com/smpp-server/smpp-server/internal/services/campaign/domain"
)

// Server implements the gRPC CampaignServiceServer.
type Server struct {
	campaignv1.UnimplementedCampaignServiceServer
	service *application.CampaignService
	logger  zerolog.Logger
}

// NewServer creates a new gRPC server for the campaign service.
func NewServer(service *application.CampaignService) *Server {
	return &Server{
		service: service,
		logger:  log.With().Str("component", "campaign-grpc-server").Logger(),
	}
}

// --- Campaign CRUD ---

func (s *Server) CreateCampaign(ctx context.Context, req *campaignv1.CreateCampaignRequest) (*campaignv1.Campaign, error) {
	clientID, err := parseUUID(req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.GetContactListId() == "" {
		return nil, status.Error(codes.InvalidArgument, "contact_list_id is required")
	}

	var scheduledAt *time.Time
	if req.GetScheduledAt() != nil {
		t := req.GetScheduledAt().AsTime()
		scheduledAt = &t
	}

	c, err := s.service.CreateCampaign(ctx, clientID,
		req.GetName(), req.GetContactListId(), req.GetTemplateId(),
		req.GetSource(), req.GetSegmentRules(), req.GetSegmentTags(),
		req.GetSendRate(), scheduledAt,
	)
	if err != nil {
		return nil, s.mapError(err)
	}
	return campaignToProto(c), nil
}

func (s *Server) GetCampaign(ctx context.Context, req *campaignv1.GetCampaignRequest) (*campaignv1.Campaign, error) {
	id, clientID, err := parseTwoUUIDs(req.GetId(), "id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	c, err := s.service.GetCampaign(ctx, id, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}
	return campaignToProto(c), nil
}

func (s *Server) ListCampaigns(ctx context.Context, req *campaignv1.ListCampaignsRequest) (*campaignv1.CampaignPage, error) {
	clientID, err := parseUUID(req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	campaigns, total, err := s.service.ListCampaigns(ctx, clientID, req.GetStatus(), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, s.mapError(err)
	}

	items := make([]*campaignv1.Campaign, len(campaigns))
	for i, c := range campaigns {
		items[i] = campaignToProto(c)
	}

	return &campaignv1.CampaignPage{
		Campaigns: items,
		Total:     int32(total),
	}, nil
}

func (s *Server) UpdateCampaign(ctx context.Context, req *campaignv1.UpdateCampaignRequest) (*campaignv1.Campaign, error) {
	id, clientID, err := parseTwoUUIDs(req.GetId(), "id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	var scheduledAt *time.Time
	if req.GetScheduledAt() != nil {
		t := req.GetScheduledAt().AsTime()
		scheduledAt = &t
	}

	c, err := s.service.UpdateCampaign(ctx, id, clientID,
		req.GetName(), req.GetContactListId(), req.GetTemplateId(),
		req.GetSource(), req.GetSegmentRules(), req.GetSegmentTags(),
		req.GetSendRate(), scheduledAt,
	)
	if err != nil {
		return nil, s.mapError(err)
	}
	return campaignToProto(c), nil
}

func (s *Server) DeleteCampaign(ctx context.Context, req *campaignv1.DeleteCampaignRequest) (*emptypb.Empty, error) {
	id, clientID, err := parseTwoUUIDs(req.GetId(), "id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	if err := s.service.DeleteCampaign(ctx, id, clientID); err != nil {
		return nil, s.mapError(err)
	}
	return &emptypb.Empty{}, nil
}

// --- Lifecycle ---

func (s *Server) LaunchCampaign(ctx context.Context, req *campaignv1.LaunchCampaignRequest) (*campaignv1.Campaign, error) {
	id, clientID, err := parseTwoUUIDs(req.GetId(), "id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	c, err := s.service.LaunchCampaign(ctx, id, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}
	return campaignToProto(c), nil
}

func (s *Server) PauseCampaign(ctx context.Context, req *campaignv1.PauseCampaignRequest) (*campaignv1.Campaign, error) {
	id, clientID, err := parseTwoUUIDs(req.GetId(), "id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	c, err := s.service.PauseCampaign(ctx, id, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}
	return campaignToProto(c), nil
}

func (s *Server) ResumeCampaign(ctx context.Context, req *campaignv1.ResumeCampaignRequest) (*campaignv1.Campaign, error) {
	id, clientID, err := parseTwoUUIDs(req.GetId(), "id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	c, err := s.service.ResumeCampaign(ctx, id, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}
	return campaignToProto(c), nil
}

func (s *Server) CancelCampaign(ctx context.Context, req *campaignv1.CancelCampaignRequest) (*campaignv1.Campaign, error) {
	id, clientID, err := parseTwoUUIDs(req.GetId(), "id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	c, err := s.service.CancelCampaign(ctx, id, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}
	return campaignToProto(c), nil
}

// --- A/B Testing ---

func (s *Server) SetVariants(ctx context.Context, req *campaignv1.SetVariantsRequest) (*campaignv1.VariantList, error) {
	campaignID, clientID, err := parseTwoUUIDs(req.GetCampaignId(), "campaign_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	variants := protoVariantInputsToDomain(req.GetVariants())
	result, err := s.service.SetVariants(ctx, campaignID, clientID, variants)
	if err != nil {
		return nil, s.mapError(err)
	}

	return variantListToProto(result), nil
}

func (s *Server) SetABConfig(ctx context.Context, req *campaignv1.SetABConfigRequest) (*campaignv1.ABConfig, error) {
	campaignID, clientID, err := parseTwoUUIDs(req.GetCampaignId(), "campaign_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	if req.GetMetric() == "" {
		return nil, status.Error(codes.InvalidArgument, "metric is required")
	}

	cfg, err := s.service.SetABConfig(ctx, campaignID, clientID,
		req.GetMetric(), req.GetTestDurationHours(), req.GetAutoSelectWinner(),
	)
	if err != nil {
		return nil, s.mapError(err)
	}

	return abConfigToProto(cfg), nil
}

func (s *Server) SelectWinner(ctx context.Context, req *campaignv1.SelectWinnerRequest) (*campaignv1.Campaign, error) {
	campaignID, clientID, err := parseTwoUUIDs(req.GetCampaignId(), "campaign_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}
	variantID, err := parseUUID(req.GetVariantId(), "variant_id")
	if err != nil {
		return nil, err
	}

	c, err := s.service.SelectWinner(ctx, campaignID, clientID, variantID)
	if err != nil {
		return nil, s.mapError(err)
	}
	return campaignToProto(c), nil
}

// --- Retry ---

func (s *Server) SetRetryConfig(ctx context.Context, req *campaignv1.SetRetryConfigRequest) (*campaignv1.RetryConfig, error) {
	campaignID, clientID, err := parseTwoUUIDs(req.GetCampaignId(), "campaign_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	rc := protoRetryConfigToDomain(req.GetConfig())
	if rc == nil {
		return nil, status.Error(codes.InvalidArgument, "config is required")
	}

	result, err := s.service.SetRetryConfig(ctx, campaignID, clientID, rc)
	if err != nil {
		return nil, s.mapError(err)
	}
	return retryConfigToProto(result), nil
}

func (s *Server) RetryFailed(ctx context.Context, req *campaignv1.RetryFailedRequest) (*campaignv1.Campaign, error) {
	campaignID, clientID, err := parseTwoUUIDs(req.GetCampaignId(), "campaign_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	c, err := s.service.RetryFailed(ctx, campaignID, clientID, req.GetAlternativeTemplateId())
	if err != nil {
		return nil, s.mapError(err)
	}
	return campaignToProto(c), nil
}

// --- Analytics ---

func (s *Server) GetCampaignStats(ctx context.Context, req *campaignv1.GetCampaignStatsRequest) (*campaignv1.CampaignStats, error) {
	campaignID, clientID, err := parseTwoUUIDs(req.GetCampaignId(), "campaign_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	snap, counts, variants, err := s.service.GetStats(ctx, campaignID, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}

	var total int32
	for _, v := range counts {
		total += v
	}

	var deliveryRate float64
	if snap.Sent > 0 {
		deliveryRate = float64(snap.Delivered) / float64(snap.Sent)
	}

	stats := &campaignv1.CampaignStats{
		TotalRecipients:   total,
		Sent:              snap.Sent,
		Delivered:         snap.Delivered,
		Failed:            snap.Failed,
		Pending:           snap.Pending,
		Retry:             counts[domain.RecipientRetry],
		DeliveryRate:      deliveryRate,
		TotalCost:         snap.Cost,
		AvgDeliveryTimeMs: snap.AvgDeliveryTimeMs,
	}

	// Per-variant stats
	for _, v := range variants {
		var vdr float64
		if v.SentCount > 0 {
			vdr = float64(v.DeliveredCount) / float64(v.SentCount)
		}
		stats.PerVariant = append(stats.PerVariant, &campaignv1.VariantStats{
			VariantId:    v.ID.String(),
			VariantName:  v.Name,
			Sent:         v.SentCount,
			Delivered:    v.DeliveredCount,
			Failed:       v.FailedCount,
			DeliveryRate: vdr,
			IsWinner:     v.IsWinner,
		})
	}

	return stats, nil
}

func (s *Server) GetCampaignTimeline(ctx context.Context, req *campaignv1.GetCampaignTimelineRequest) (*campaignv1.TimelineData, error) {
	campaignID, clientID, err := parseTwoUUIDs(req.GetCampaignId(), "campaign_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	points, err := s.service.GetTimeline(ctx, campaignID, clientID, req.GetInterval(), req.GetMetric())
	if err != nil {
		return nil, s.mapError(err)
	}

	pbPoints := make([]*campaignv1.TimelinePoint, len(points))
	for i, p := range points {
		pbPoints[i] = &campaignv1.TimelinePoint{
			Timestamp: timestamppb.New(p.Timestamp),
			Value:     p.Value,
		}
	}

	return &campaignv1.TimelineData{Points: pbPoints}, nil
}

func (s *Server) GetVariantComparison(ctx context.Context, req *campaignv1.GetVariantComparisonRequest) (*campaignv1.VariantComparisonData, error) {
	campaignID, clientID, err := parseTwoUUIDs(req.GetCampaignId(), "campaign_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	comparisons, winnerID, err := s.service.GetVariantComparison(ctx, campaignID, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}

	rows := make([]*campaignv1.VariantComparisonRow, len(comparisons))
	for i, vc := range comparisons {
		rows[i] = &campaignv1.VariantComparisonRow{
			VariantId:         vc.VariantID.String(),
			VariantName:       vc.VariantName,
			AudienceSize:      vc.AudienceSize,
			Sent:              vc.Sent,
			Delivered:         vc.Delivered,
			Failed:            vc.Failed,
			DeliveryRate:      vc.DeliveryRate,
			AvgDeliveryTimeMs: vc.AvgDeliveryTimeMs,
			Cost:              vc.Cost,
		}
	}

	result := &campaignv1.VariantComparisonData{Rows: rows}
	if winnerID != nil {
		result.WinnerVariantId = winnerID.String()
	}

	return result, nil
}

func (s *Server) GetDeliveryHeatmap(ctx context.Context, req *campaignv1.GetDeliveryHeatmapRequest) (*campaignv1.HeatmapData, error) {
	campaignID, clientID, err := parseTwoUUIDs(req.GetCampaignId(), "campaign_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	cells, err := s.service.GetHeatmap(ctx, campaignID, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}

	pbCells := make([]*campaignv1.HeatmapCell, len(cells))
	for i, cell := range cells {
		pbCells[i] = &campaignv1.HeatmapCell{
			DayOfWeek:      cell.DayOfWeek,
			Hour:           cell.Hour,
			DeliveredCount: cell.DeliveredCount,
			DeliveryRate:   cell.DeliveryRate,
		}
	}

	return &campaignv1.HeatmapData{Cells: pbCells}, nil
}

func (s *Server) GetOptimalSendTime(ctx context.Context, req *campaignv1.GetOptimalSendTimeRequest) (*campaignv1.SendTimeRecommendation, error) {
	clientID, err := parseUUID(req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	slots, err := s.service.GetOptimalSendTimes(ctx, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}

	pbSlots := make([]*campaignv1.TimeSlot, len(slots))
	for i, slot := range slots {
		pbSlots[i] = &campaignv1.TimeSlot{
			DayOfWeek:         slot.DayOfWeek,
			Hour:              slot.Hour,
			DeliveryRate:      slot.DeliveryRate,
			AvgDeliveryTimeMs: slot.AvgDeliveryTimeMs,
			Score:             slot.Score,
		}
	}

	return &campaignv1.SendTimeRecommendation{Slots: pbSlots}, nil
}

func (s *Server) ExportReport(ctx context.Context, req *campaignv1.ExportReportRequest) (*campaignv1.ReportFile, error) {
	campaignID, clientID, err := parseTwoUUIDs(req.GetCampaignId(), "campaign_id", req.GetClientId(), "client_id")
	if err != nil {
		return nil, err
	}

	data, filename, contentType, err := s.service.ExportReport(ctx, campaignID, clientID, req.GetFormat())
	if err != nil {
		return nil, s.mapError(err)
	}

	return &campaignv1.ReportFile{
		Data:        data,
		Filename:    filename,
		ContentType: contentType,
	}, nil
}

// --- Error mapping ---

func (s *Server) mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrCampaignNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrVariantNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidCampaignStatus):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrCampaignNotDraft):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrCampaignNotRunning):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrCampaignNotPaused):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrVariantPercentageSum):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrTooFewVariants):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrTooManyVariants):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrWinnerAlreadySelected):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrNoFailedRecipients):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		s.logger.Error().Err(err).Msg("internal error")
		return status.Error(codes.Internal, err.Error())
	}
}
