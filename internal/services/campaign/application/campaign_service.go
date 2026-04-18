package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/campaign/domain"
	"github.com/smpp-server/smpp-server/internal/services/campaign/infrastructure/repository"
	smstpl "github.com/smpp-server/smpp-server/internal/shared/template"
)

// CampaignService implements the business logic for the campaign management domain.
type CampaignService struct {
	campaignRepo  *repository.CampaignRepository
	recipientRepo *repository.RecipientRepository
	statsRepo     *repository.StatsRepository
	logger        zerolog.Logger
}

// NewCampaignService creates a new CampaignService.
func NewCampaignService(
	campaignRepo *repository.CampaignRepository,
	recipientRepo *repository.RecipientRepository,
	statsRepo *repository.StatsRepository,
) *CampaignService {
	return &CampaignService{
		campaignRepo:  campaignRepo,
		recipientRepo: recipientRepo,
		statsRepo:     statsRepo,
		logger:        log.With().Str("component", "campaign-service").Logger(),
	}
}

// --- Campaign CRUD ---

// CreateCampaign creates a new campaign in draft status.
func (s *CampaignService) CreateCampaign(ctx context.Context, clientID uuid.UUID, name, contactListID, templateID, source, segmentRules string, segmentTags []string, sendRate int32, scheduledAt *time.Time, useSubscriberTimezone bool) (*domain.Campaign, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	clID, err := uuid.Parse(contactListID)
	if err != nil {
		return nil, fmt.Errorf("invalid contact_list_id: %w", err)
	}

	c := &domain.Campaign{
		ID:                    uuid.New(),
		ClientID:              clientID,
		Name:                  name,
		Status:                domain.StatusDraft,
		ContactListID:         clID,
		Source:                source,
		SegmentRules:          segmentRules,
		SegmentTags:           segmentTags,
		SendRate:              sendRate,
		ScheduledAt:           scheduledAt,
		UseSubscriberTimezone: useSubscriberTimezone,
	}

	if templateID != "" {
		id, err := uuid.Parse(templateID)
		if err != nil {
			return nil, fmt.Errorf("invalid template_id: %w", err)
		}
		c.TemplateID = &id
	}

	if c.SegmentTags == nil {
		c.SegmentTags = []string{}
	}

	return s.campaignRepo.Create(ctx, c)
}

// GetCampaign retrieves a campaign by ID with its variants and AB config.
func (s *CampaignService) GetCampaign(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error) {
	c, err := s.campaignRepo.GetByID(ctx, id, clientID)
	if err != nil {
		return nil, err
	}

	// Load variants
	variants, err := s.campaignRepo.GetVariants(ctx, id)
	if err != nil {
		return nil, err
	}
	c.Variants = variants

	// Load AB config
	abConfig, err := s.campaignRepo.GetABConfig(ctx, id)
	if err != nil {
		return nil, err
	}
	c.ABConfig = abConfig

	return c, nil
}

// ListCampaigns lists campaigns for a client with optional status filter.
func (s *CampaignService) ListCampaigns(ctx context.Context, clientID uuid.UUID, status string, limit, offset int) ([]*domain.Campaign, int, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.campaignRepo.List(ctx, clientID, status, limit, offset)
}

// UpdateCampaign updates a campaign's mutable fields (only if draft).
func (s *CampaignService) UpdateCampaign(ctx context.Context, id, clientID uuid.UUID, name, contactListID, templateID, source, segmentRules string, segmentTags []string, sendRate int32, scheduledAt *time.Time) (*domain.Campaign, error) {
	existing, err := s.campaignRepo.GetByID(ctx, id, clientID)
	if err != nil {
		return nil, err
	}

	if existing.Status != domain.StatusDraft && existing.Status != domain.StatusScheduled {
		return nil, domain.ErrCampaignNotDraft
	}

	if name != "" {
		existing.Name = name
	}
	if contactListID != "" {
		clID, err := uuid.Parse(contactListID)
		if err != nil {
			return nil, fmt.Errorf("invalid contact_list_id: %w", err)
		}
		existing.ContactListID = clID
	}
	if templateID != "" {
		tID, err := uuid.Parse(templateID)
		if err != nil {
			return nil, fmt.Errorf("invalid template_id: %w", err)
		}
		existing.TemplateID = &tID
	}
	if source != "" {
		existing.Source = source
	}
	existing.SegmentRules = segmentRules
	if segmentTags != nil {
		existing.SegmentTags = segmentTags
	}
	if sendRate > 0 {
		existing.SendRate = sendRate
	}
	existing.ScheduledAt = scheduledAt

	return s.campaignRepo.Update(ctx, existing)
}

// DeleteCampaign deletes a campaign (only if draft).
func (s *CampaignService) DeleteCampaign(ctx context.Context, id, clientID uuid.UUID) error {
	return s.campaignRepo.Delete(ctx, id, clientID)
}

// --- Lifecycle ---

// LaunchCampaign transitions a campaign from draft/scheduled to materializing.
func (s *CampaignService) LaunchCampaign(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error) {
	c, err := s.campaignRepo.GetByID(ctx, id, clientID)
	if err != nil {
		return nil, err
	}

	if c.Status != domain.StatusDraft && c.Status != domain.StatusScheduled {
		return nil, domain.ErrCampaignNotDraft
	}

	now := time.Now()
	if err := s.campaignRepo.UpdateStatus(ctx, id, domain.StatusMaterializing, &now, nil); err != nil {
		return nil, err
	}

	s.logger.Info().
		Str("campaign_id", id.String()).
		Str("client_id", clientID.String()).
		Msg("campaign launched, starting materialization")

	// Return updated campaign
	return s.GetCampaign(ctx, id, clientID)
}

// PauseCampaign pauses a running campaign.
func (s *CampaignService) PauseCampaign(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error) {
	c, err := s.campaignRepo.GetByID(ctx, id, clientID)
	if err != nil {
		return nil, err
	}

	if c.Status != domain.StatusRunning {
		return nil, domain.ErrCampaignNotRunning
	}

	if err := s.campaignRepo.UpdateStatus(ctx, id, domain.StatusPaused, nil, nil); err != nil {
		return nil, err
	}

	s.logger.Info().
		Str("campaign_id", id.String()).
		Msg("campaign paused")

	return s.GetCampaign(ctx, id, clientID)
}

// ResumeCampaign resumes a paused campaign.
func (s *CampaignService) ResumeCampaign(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error) {
	c, err := s.campaignRepo.GetByID(ctx, id, clientID)
	if err != nil {
		return nil, err
	}

	if c.Status != domain.StatusPaused {
		return nil, domain.ErrCampaignNotPaused
	}

	if err := s.campaignRepo.UpdateStatus(ctx, id, domain.StatusRunning, nil, nil); err != nil {
		return nil, err
	}

	s.logger.Info().
		Str("campaign_id", id.String()).
		Msg("campaign resumed")

	return s.GetCampaign(ctx, id, clientID)
}

// CancelCampaign cancels a running/paused/materializing campaign.
func (s *CampaignService) CancelCampaign(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error) {
	c, err := s.campaignRepo.GetByID(ctx, id, clientID)
	if err != nil {
		return nil, err
	}

	if c.Status != domain.StatusRunning && c.Status != domain.StatusPaused && c.Status != domain.StatusMaterializing {
		return nil, domain.ErrInvalidCampaignStatus
	}

	// Cancel pending recipients
	cancelled, err := s.recipientRepo.CancelPending(ctx, id)
	if err != nil {
		s.logger.Error().Err(err).Str("campaign_id", id.String()).Msg("failed to cancel pending recipients")
	} else {
		s.logger.Info().
			Str("campaign_id", id.String()).
			Int64("cancelled_count", cancelled).
			Msg("cancelled pending recipients")
	}

	now := time.Now()
	if err := s.campaignRepo.UpdateStatus(ctx, id, domain.StatusCancelled, nil, &now); err != nil {
		return nil, err
	}

	s.logger.Info().
		Str("campaign_id", id.String()).
		Msg("campaign cancelled")

	return s.GetCampaign(ctx, id, clientID)
}

// --- A/B Testing ---

// SetVariants replaces all variants for a campaign (must be draft).
func (s *CampaignService) SetVariants(ctx context.Context, campaignID, clientID uuid.UUID, variants []domain.Variant) ([]domain.Variant, error) {
	c, err := s.campaignRepo.GetByID(ctx, campaignID, clientID)
	if err != nil {
		return nil, err
	}

	if c.Status != domain.StatusDraft {
		return nil, domain.ErrCampaignNotDraft
	}

	if len(variants) < 2 {
		return nil, domain.ErrTooFewVariants
	}
	if len(variants) > 5 {
		return nil, domain.ErrTooManyVariants
	}

	// Validate percentages sum to 100
	var total int32
	for _, v := range variants {
		total += v.Percentage
	}
	if total != 100 {
		return nil, domain.ErrVariantPercentageSum
	}

	// Set campaign_id on all variants
	for i := range variants {
		variants[i].CampaignID = campaignID
	}

	return s.campaignRepo.SetVariants(ctx, campaignID, variants)
}

// SetABConfig sets the A/B testing configuration for a campaign.
func (s *CampaignService) SetABConfig(ctx context.Context, campaignID, clientID uuid.UUID, metric string, testDurationHours int32, autoSelectWinner bool) (*domain.ABConfig, error) {
	// Verify ownership
	if _, err := s.campaignRepo.GetByID(ctx, campaignID, clientID); err != nil {
		return nil, err
	}

	cfg := &domain.ABConfig{
		CampaignID:        campaignID,
		Metric:            metric,
		TestDurationHours: testDurationHours,
		AutoSelectWinner:  autoSelectWinner,
	}

	return s.campaignRepo.SetABConfig(ctx, cfg)
}

// SelectWinner selects the winning variant for an A/B test.
func (s *CampaignService) SelectWinner(ctx context.Context, campaignID, clientID, variantID uuid.UUID) (*domain.Campaign, error) {
	// Verify ownership
	if _, err := s.campaignRepo.GetByID(ctx, campaignID, clientID); err != nil {
		return nil, err
	}

	// Check if winner already selected
	abConfig, err := s.campaignRepo.GetABConfig(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if abConfig != nil && abConfig.WinnerVariantID != nil {
		return nil, domain.ErrWinnerAlreadySelected
	}

	if err := s.campaignRepo.SelectWinner(ctx, campaignID, variantID); err != nil {
		return nil, err
	}

	s.logger.Info().
		Str("campaign_id", campaignID.String()).
		Str("variant_id", variantID.String()).
		Msg("winner variant selected")

	return s.GetCampaign(ctx, campaignID, clientID)
}

// --- Retry ---

// SetRetryConfig sets the retry configuration for a campaign.
func (s *CampaignService) SetRetryConfig(ctx context.Context, campaignID, clientID uuid.UUID, rc *domain.RetryConfig) (*domain.RetryConfig, error) {
	// Verify ownership
	c, err := s.campaignRepo.GetByID(ctx, campaignID, clientID)
	if err != nil {
		return nil, err
	}

	if err := s.campaignRepo.UpdateRetryConfig(ctx, c.ID, rc); err != nil {
		return nil, err
	}

	return rc, nil
}

// RetryFailed resets failed recipients for retry.
func (s *CampaignService) RetryFailed(ctx context.Context, campaignID, clientID uuid.UUID, alternativeTemplateID string) (*domain.Campaign, error) {
	c, err := s.campaignRepo.GetByID(ctx, campaignID, clientID)
	if err != nil {
		return nil, err
	}

	maxRetries := int32(3) // default
	if c.RetryConfig != nil && c.RetryConfig.MaxRetries > 0 {
		maxRetries = c.RetryConfig.MaxRetries
	}

	failed, err := s.recipientRepo.GetFailedForRetry(ctx, campaignID, maxRetries)
	if err != nil {
		return nil, err
	}

	if len(failed) == 0 {
		return nil, domain.ErrNoFailedRecipients
	}

	// Increment retry count for all failed
	for _, rec := range failed {
		if err := s.recipientRepo.IncrementRetry(ctx, rec.ID); err != nil {
			s.logger.Error().Err(err).Str("recipient_id", rec.ID.String()).Msg("failed to increment retry")
		}
	}

	// Update retry config if alternative template provided
	if alternativeTemplateID != "" && c.RetryConfig != nil {
		c.RetryConfig.AlternativeTemplateID = alternativeTemplateID
		if err := s.campaignRepo.UpdateRetryConfig(ctx, campaignID, c.RetryConfig); err != nil {
			s.logger.Error().Err(err).Msg("failed to update retry config with alternative template")
		}
	}

	s.logger.Info().
		Str("campaign_id", campaignID.String()).
		Int("retry_count", len(failed)).
		Msg("retrying failed recipients")

	return s.GetCampaign(ctx, campaignID, clientID)
}

// --- Analytics ---

// GetStats retrieves current campaign stats by counting recipients.
func (s *CampaignService) GetStats(ctx context.Context, campaignID, clientID uuid.UUID) (*domain.StatsSnapshot, map[string]int32, []domain.Variant, error) {
	// Verify ownership
	c, err := s.campaignRepo.GetByID(ctx, campaignID, clientID)
	if err != nil {
		return nil, nil, nil, err
	}

	// Get counts by status
	counts, err := s.recipientRepo.CountByStatus(ctx, campaignID)
	if err != nil {
		return nil, nil, nil, err
	}

	total, err := s.recipientRepo.CountTotal(ctx, campaignID)
	if err != nil {
		return nil, nil, nil, err
	}

	snap := &domain.StatsSnapshot{
		CampaignID: campaignID,
		Sent:       counts[domain.RecipientSent],
		Delivered:  counts[domain.RecipientDelivered],
		Failed:     counts[domain.RecipientFailed],
		Pending:    counts[domain.RecipientPending],
	}
	// Use campaign-level counters if recipients not yet materialized
	if total == 0 {
		snap.Sent = c.SentCount
		snap.Delivered = c.DeliveredCount
		snap.Failed = c.FailedCount
	}

	// Get variants
	variants, err := s.campaignRepo.GetVariants(ctx, campaignID)
	if err != nil {
		return nil, nil, nil, err
	}

	return snap, counts, variants, nil
}

// GetTimeline retrieves timeline data for a campaign.
func (s *CampaignService) GetTimeline(ctx context.Context, campaignID, clientID uuid.UUID, interval, metric string) ([]domain.TimelinePoint, error) {
	// Verify ownership
	if _, err := s.campaignRepo.GetByID(ctx, campaignID, clientID); err != nil {
		return nil, err
	}

	return s.statsRepo.GetTimeline(ctx, campaignID, interval, metric)
}

// GetHeatmap retrieves delivery heatmap data for a campaign.
func (s *CampaignService) GetHeatmap(ctx context.Context, campaignID, clientID uuid.UUID) ([]domain.HeatmapCell, error) {
	// Verify ownership
	if _, err := s.campaignRepo.GetByID(ctx, campaignID, clientID); err != nil {
		return nil, err
	}

	return s.statsRepo.GetHeatmap(ctx, campaignID)
}

// GetVariantComparison retrieves per-variant comparison data.
func (s *CampaignService) GetVariantComparison(ctx context.Context, campaignID, clientID uuid.UUID) ([]domain.VariantComparison, *uuid.UUID, error) {
	// Verify ownership
	if _, err := s.campaignRepo.GetByID(ctx, campaignID, clientID); err != nil {
		return nil, nil, err
	}

	comparisons, err := s.statsRepo.GetVariantComparison(ctx, campaignID)
	if err != nil {
		return nil, nil, err
	}

	// Get winner from AB config
	abConfig, err := s.campaignRepo.GetABConfig(ctx, campaignID)
	if err != nil {
		return nil, nil, err
	}

	var winnerID *uuid.UUID
	if abConfig != nil {
		winnerID = abConfig.WinnerVariantID
	}

	return comparisons, winnerID, nil
}

// GetOptimalSendTimes retrieves optimal send times based on historical data.
func (s *CampaignService) GetOptimalSendTimes(ctx context.Context, clientID uuid.UUID) ([]domain.TimeSlot, error) {
	return s.statsRepo.GetOptimalSendTimes(ctx, clientID)
}

// ExportReport generates a report for a campaign (simplified: returns JSON).
func (s *CampaignService) ExportReport(ctx context.Context, campaignID, clientID uuid.UUID, format string) ([]byte, string, string, error) {
	c, err := s.GetCampaign(ctx, campaignID, clientID)
	if err != nil {
		return nil, "", "", err
	}

	counts, cErr := s.recipientRepo.CountByStatus(ctx, campaignID)
	if cErr != nil {
		return nil, "", "", cErr
	}

	report := map[string]interface{}{
		"campaign": map[string]interface{}{
			"id":               c.ID.String(),
			"name":             c.Name,
			"status":           c.Status,
			"total_recipients": c.TotalRecipients,
			"sent_count":       c.SentCount,
			"delivered_count":  c.DeliveredCount,
			"failed_count":     c.FailedCount,
		},
		"status_breakdown": counts,
		"variants":         c.Variants,
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to marshal report: %w", err)
	}

	filename := fmt.Sprintf("campaign_%s_report.json", c.ID.String()[:8])
	return data, filename, "application/json", nil
}

// --- Template Preview ---

// TemplatePreviewInput holds input for template preview.
type TemplatePreviewInput struct {
	TemplateText string
	TestData     []map[string]interface{}
}

// TemplatePreviewResult holds a single preview result.
type TemplatePreviewResult struct {
	Rendered string
	Length   int
	Segments int
	Warnings []string
	Error    string
}

// PreviewTemplate renders a template against multiple test data sets.
func (s *CampaignService) PreviewTemplate(ctx context.Context, input TemplatePreviewInput) ([]TemplatePreviewResult, error) {
	renderer := smstpl.NewRenderer()
	validator := smstpl.NewValidator()

	// Validate syntax first
	vr := validator.Validate(input.TemplateText)
	if !vr.Valid {
		return nil, fmt.Errorf("invalid template: %s", vr.Errors[0])
	}

	results := make([]TemplatePreviewResult, 0, len(input.TestData))
	for _, bindings := range input.TestData {
		result := TemplatePreviewResult{}
		rendered, err := renderer.Render(input.TemplateText, bindings)
		if err != nil {
			result.Error = err.Error()
		} else {
			result.Rendered = rendered
			result.Length = len([]rune(rendered))
			result.Segments = (result.Length + 159) / 160
			if result.Segments < 1 {
				result.Segments = 1
			}
			if result.Segments > 1 {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Сообщение %d симв. (%d SMS-сегментов)", result.Length, result.Segments))
			}
		}
		results = append(results, result)
	}

	return results, nil
}
