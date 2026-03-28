package mocks

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/campaign/application"
	"github.com/smpp-server/smpp-server/internal/services/campaign/domain"
)

// MockCampaignServicer is a hand-written mock for grpc.CampaignServicer.
type MockCampaignServicer struct {
	CreateCampaignFunc      func(ctx context.Context, clientID uuid.UUID, name, contactListID, templateID, source, segmentRules string, segmentTags []string, sendRate int32, scheduledAt *time.Time) (*domain.Campaign, error)
	GetCampaignFunc         func(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error)
	ListCampaignsFunc       func(ctx context.Context, clientID uuid.UUID, status string, limit, offset int) ([]*domain.Campaign, int, error)
	UpdateCampaignFunc      func(ctx context.Context, id, clientID uuid.UUID, name, contactListID, templateID, source, segmentRules string, segmentTags []string, sendRate int32, scheduledAt *time.Time) (*domain.Campaign, error)
	DeleteCampaignFunc      func(ctx context.Context, id, clientID uuid.UUID) error
	LaunchCampaignFunc      func(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error)
	PauseCampaignFunc       func(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error)
	ResumeCampaignFunc      func(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error)
	CancelCampaignFunc      func(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error)
	SetVariantsFunc         func(ctx context.Context, campaignID, clientID uuid.UUID, variants []domain.Variant) ([]domain.Variant, error)
	SetABConfigFunc         func(ctx context.Context, campaignID, clientID uuid.UUID, metric string, testDurationHours int32, autoSelectWinner bool) (*domain.ABConfig, error)
	SelectWinnerFunc        func(ctx context.Context, campaignID, clientID, variantID uuid.UUID) (*domain.Campaign, error)
	SetRetryConfigFunc      func(ctx context.Context, campaignID, clientID uuid.UUID, rc *domain.RetryConfig) (*domain.RetryConfig, error)
	RetryFailedFunc         func(ctx context.Context, campaignID, clientID uuid.UUID, alternativeTemplateID string) (*domain.Campaign, error)
	GetStatsFunc            func(ctx context.Context, campaignID, clientID uuid.UUID) (*domain.StatsSnapshot, map[string]int32, []domain.Variant, error)
	GetTimelineFunc         func(ctx context.Context, campaignID, clientID uuid.UUID, interval, metric string) ([]domain.TimelinePoint, error)
	GetHeatmapFunc          func(ctx context.Context, campaignID, clientID uuid.UUID) ([]domain.HeatmapCell, error)
	GetVariantComparisonFunc func(ctx context.Context, campaignID, clientID uuid.UUID) ([]domain.VariantComparison, *uuid.UUID, error)
	GetOptimalSendTimesFunc func(ctx context.Context, clientID uuid.UUID) ([]domain.TimeSlot, error)
	ExportReportFunc        func(ctx context.Context, campaignID, clientID uuid.UUID, format string) ([]byte, string, string, error)
	PreviewTemplateFunc     func(ctx context.Context, input application.TemplatePreviewInput) ([]application.TemplatePreviewResult, error)
}

func (m *MockCampaignServicer) CreateCampaign(ctx context.Context, clientID uuid.UUID, name, contactListID, templateID, source, segmentRules string, segmentTags []string, sendRate int32, scheduledAt *time.Time) (*domain.Campaign, error) {
	if m.CreateCampaignFunc != nil {
		return m.CreateCampaignFunc(ctx, clientID, name, contactListID, templateID, source, segmentRules, segmentTags, sendRate, scheduledAt)
	}
	return nil, nil
}

func (m *MockCampaignServicer) GetCampaign(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error) {
	if m.GetCampaignFunc != nil {
		return m.GetCampaignFunc(ctx, id, clientID)
	}
	return nil, nil
}

func (m *MockCampaignServicer) ListCampaigns(ctx context.Context, clientID uuid.UUID, status string, limit, offset int) ([]*domain.Campaign, int, error) {
	if m.ListCampaignsFunc != nil {
		return m.ListCampaignsFunc(ctx, clientID, status, limit, offset)
	}
	return nil, 0, nil
}

func (m *MockCampaignServicer) UpdateCampaign(ctx context.Context, id, clientID uuid.UUID, name, contactListID, templateID, source, segmentRules string, segmentTags []string, sendRate int32, scheduledAt *time.Time) (*domain.Campaign, error) {
	if m.UpdateCampaignFunc != nil {
		return m.UpdateCampaignFunc(ctx, id, clientID, name, contactListID, templateID, source, segmentRules, segmentTags, sendRate, scheduledAt)
	}
	return nil, nil
}

func (m *MockCampaignServicer) DeleteCampaign(ctx context.Context, id, clientID uuid.UUID) error {
	if m.DeleteCampaignFunc != nil {
		return m.DeleteCampaignFunc(ctx, id, clientID)
	}
	return nil
}

func (m *MockCampaignServicer) LaunchCampaign(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error) {
	if m.LaunchCampaignFunc != nil {
		return m.LaunchCampaignFunc(ctx, id, clientID)
	}
	return nil, nil
}

func (m *MockCampaignServicer) PauseCampaign(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error) {
	if m.PauseCampaignFunc != nil {
		return m.PauseCampaignFunc(ctx, id, clientID)
	}
	return nil, nil
}

func (m *MockCampaignServicer) ResumeCampaign(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error) {
	if m.ResumeCampaignFunc != nil {
		return m.ResumeCampaignFunc(ctx, id, clientID)
	}
	return nil, nil
}

func (m *MockCampaignServicer) CancelCampaign(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error) {
	if m.CancelCampaignFunc != nil {
		return m.CancelCampaignFunc(ctx, id, clientID)
	}
	return nil, nil
}

func (m *MockCampaignServicer) SetVariants(ctx context.Context, campaignID, clientID uuid.UUID, variants []domain.Variant) ([]domain.Variant, error) {
	if m.SetVariantsFunc != nil {
		return m.SetVariantsFunc(ctx, campaignID, clientID, variants)
	}
	return nil, nil
}

func (m *MockCampaignServicer) SetABConfig(ctx context.Context, campaignID, clientID uuid.UUID, metric string, testDurationHours int32, autoSelectWinner bool) (*domain.ABConfig, error) {
	if m.SetABConfigFunc != nil {
		return m.SetABConfigFunc(ctx, campaignID, clientID, metric, testDurationHours, autoSelectWinner)
	}
	return nil, nil
}

func (m *MockCampaignServicer) SelectWinner(ctx context.Context, campaignID, clientID, variantID uuid.UUID) (*domain.Campaign, error) {
	if m.SelectWinnerFunc != nil {
		return m.SelectWinnerFunc(ctx, campaignID, clientID, variantID)
	}
	return nil, nil
}

func (m *MockCampaignServicer) SetRetryConfig(ctx context.Context, campaignID, clientID uuid.UUID, rc *domain.RetryConfig) (*domain.RetryConfig, error) {
	if m.SetRetryConfigFunc != nil {
		return m.SetRetryConfigFunc(ctx, campaignID, clientID, rc)
	}
	return nil, nil
}

func (m *MockCampaignServicer) RetryFailed(ctx context.Context, campaignID, clientID uuid.UUID, alternativeTemplateID string) (*domain.Campaign, error) {
	if m.RetryFailedFunc != nil {
		return m.RetryFailedFunc(ctx, campaignID, clientID, alternativeTemplateID)
	}
	return nil, nil
}

func (m *MockCampaignServicer) GetStats(ctx context.Context, campaignID, clientID uuid.UUID) (*domain.StatsSnapshot, map[string]int32, []domain.Variant, error) {
	if m.GetStatsFunc != nil {
		return m.GetStatsFunc(ctx, campaignID, clientID)
	}
	return nil, nil, nil, nil
}

func (m *MockCampaignServicer) GetTimeline(ctx context.Context, campaignID, clientID uuid.UUID, interval, metric string) ([]domain.TimelinePoint, error) {
	if m.GetTimelineFunc != nil {
		return m.GetTimelineFunc(ctx, campaignID, clientID, interval, metric)
	}
	return nil, nil
}

func (m *MockCampaignServicer) GetHeatmap(ctx context.Context, campaignID, clientID uuid.UUID) ([]domain.HeatmapCell, error) {
	if m.GetHeatmapFunc != nil {
		return m.GetHeatmapFunc(ctx, campaignID, clientID)
	}
	return nil, nil
}

func (m *MockCampaignServicer) GetVariantComparison(ctx context.Context, campaignID, clientID uuid.UUID) ([]domain.VariantComparison, *uuid.UUID, error) {
	if m.GetVariantComparisonFunc != nil {
		return m.GetVariantComparisonFunc(ctx, campaignID, clientID)
	}
	return nil, nil, nil
}

func (m *MockCampaignServicer) GetOptimalSendTimes(ctx context.Context, clientID uuid.UUID) ([]domain.TimeSlot, error) {
	if m.GetOptimalSendTimesFunc != nil {
		return m.GetOptimalSendTimesFunc(ctx, clientID)
	}
	return nil, nil
}

func (m *MockCampaignServicer) ExportReport(ctx context.Context, campaignID, clientID uuid.UUID, format string) ([]byte, string, string, error) {
	if m.ExportReportFunc != nil {
		return m.ExportReportFunc(ctx, campaignID, clientID, format)
	}
	return nil, "", "", nil
}

func (m *MockCampaignServicer) PreviewTemplate(ctx context.Context, input application.TemplatePreviewInput) ([]application.TemplatePreviewResult, error) {
	if m.PreviewTemplateFunc != nil {
		return m.PreviewTemplateFunc(ctx, input)
	}
	return nil, nil
}
