package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/smpp-server/smpp-server/internal/services/campaign/domain"
)

// CampaignRepository handles CRUD operations for campaigns, variants, and AB config.
type CampaignRepository struct {
	db *sqlx.DB
}

// NewCampaignRepository creates a new CampaignRepository.
func NewCampaignRepository(db *sqlx.DB) *CampaignRepository {
	return &CampaignRepository{db: db}
}

// --- row structs ---

type campaignRow struct {
	ID              uuid.UUID      `db:"id"`
	ClientID        uuid.UUID      `db:"client_id"`
	Name            string         `db:"name"`
	Status          string         `db:"status"`
	ContactListID   uuid.UUID      `db:"contact_list_id"`
	TemplateID      sql.NullString `db:"template_id"`
	Source          string         `db:"source"`
	SegmentRules    sql.NullString `db:"segment_rules"`
	SegmentTags     pq.StringArray `db:"segment_tags"`
	SendRate        int32          `db:"send_rate"`
	ScheduledAt     sql.NullTime   `db:"scheduled_at"`
	StartedAt       sql.NullTime   `db:"started_at"`
	CompletedAt     sql.NullTime   `db:"completed_at"`
	RetryConfig     sql.NullString `db:"retry_config"`
	TotalRecipients int32          `db:"total_recipients"`
	SentCount       int32          `db:"sent_count"`
	DeliveredCount  int32          `db:"delivered_count"`
	FailedCount     int32          `db:"failed_count"`
	CreatedAt       sql.NullTime   `db:"created_at"`
	UpdatedAt       sql.NullTime   `db:"updated_at"`
}

func (r *campaignRow) toDomain() *domain.Campaign {
	c := &domain.Campaign{
		ID:              r.ID,
		ClientID:        r.ClientID,
		Name:            r.Name,
		Status:          r.Status,
		ContactListID:   r.ContactListID,
		Source:          r.Source,
		SegmentTags:     []string(r.SegmentTags),
		SendRate:        r.SendRate,
		TotalRecipients: r.TotalRecipients,
		SentCount:       r.SentCount,
		DeliveredCount:  r.DeliveredCount,
		FailedCount:     r.FailedCount,
	}
	if r.TemplateID.Valid {
		id, err := uuid.Parse(r.TemplateID.String)
		if err == nil {
			c.TemplateID = &id
		}
	}
	if r.SegmentRules.Valid {
		c.SegmentRules = r.SegmentRules.String
	}
	if r.ScheduledAt.Valid {
		t := r.ScheduledAt.Time
		c.ScheduledAt = &t
	}
	if r.StartedAt.Valid {
		t := r.StartedAt.Time
		c.StartedAt = &t
	}
	if r.CompletedAt.Valid {
		t := r.CompletedAt.Time
		c.CompletedAt = &t
	}
	if r.RetryConfig.Valid && r.RetryConfig.String != "" {
		var rc domain.RetryConfig
		if err := json.Unmarshal([]byte(r.RetryConfig.String), &rc); err == nil {
			c.RetryConfig = &rc
		}
	}
	if r.CreatedAt.Valid {
		c.CreatedAt = r.CreatedAt.Time
	}
	if r.UpdatedAt.Valid {
		c.UpdatedAt = r.UpdatedAt.Time
	}
	if c.SegmentTags == nil {
		c.SegmentTags = []string{}
	}
	return c
}

type variantRow struct {
	ID             uuid.UUID      `db:"id"`
	CampaignID     uuid.UUID      `db:"campaign_id"`
	Name           string         `db:"name"`
	TemplateID     sql.NullString `db:"template_id"`
	Percentage     int32          `db:"percentage"`
	IsWinner       bool           `db:"is_winner"`
	IsControl      bool           `db:"is_control"`
	SentCount      int32          `db:"sent_count"`
	DeliveredCount int32          `db:"delivered_count"`
	FailedCount    int32          `db:"failed_count"`
}

func (r *variantRow) toDomain() domain.Variant {
	v := domain.Variant{
		ID:             r.ID,
		CampaignID:     r.CampaignID,
		Name:           r.Name,
		Percentage:     r.Percentage,
		IsWinner:       r.IsWinner,
		IsControl:      r.IsControl,
		SentCount:      r.SentCount,
		DeliveredCount: r.DeliveredCount,
		FailedCount:    r.FailedCount,
	}
	if r.TemplateID.Valid {
		id, err := uuid.Parse(r.TemplateID.String)
		if err == nil {
			v.TemplateID = &id
		}
	}
	return v
}

type abConfigRow struct {
	CampaignID        uuid.UUID      `db:"campaign_id"`
	Metric            string         `db:"metric"`
	TestDurationHours int32          `db:"test_duration_hours"`
	AutoSelectWinner  bool           `db:"auto_select_winner"`
	WinnerVariantID   sql.NullString `db:"winner_variant_id"`
	WinnerSelectedAt  sql.NullTime   `db:"winner_selected_at"`
}

func (r *abConfigRow) toDomain() *domain.ABConfig {
	cfg := &domain.ABConfig{
		CampaignID:        r.CampaignID,
		Metric:            r.Metric,
		TestDurationHours: r.TestDurationHours,
		AutoSelectWinner:  r.AutoSelectWinner,
	}
	if r.WinnerVariantID.Valid {
		id, err := uuid.Parse(r.WinnerVariantID.String)
		if err == nil {
			cfg.WinnerVariantID = &id
		}
	}
	if r.WinnerSelectedAt.Valid {
		t := r.WinnerSelectedAt.Time
		cfg.WinnerSelectedAt = &t
	}
	return cfg
}

// --- Campaign CRUD ---

const campaignColumns = `id, client_id, name, status, contact_list_id, template_id, source,
	segment_rules, segment_tags, send_rate, scheduled_at, started_at, completed_at,
	retry_config, total_recipients, sent_count, delivered_count, failed_count, created_at, updated_at`

// Create inserts a new campaign.
func (r *CampaignRepository) Create(ctx context.Context, c *domain.Campaign) (*domain.Campaign, error) {
	query := fmt.Sprintf(`INSERT INTO campaigns (id, client_id, name, status, contact_list_id, template_id, source,
		segment_rules, segment_tags, send_rate, scheduled_at, retry_config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING %s`, campaignColumns)

	var templateID sql.NullString
	if c.TemplateID != nil {
		templateID = sql.NullString{String: c.TemplateID.String(), Valid: true}
	}

	var segmentRules sql.NullString
	if c.SegmentRules != "" {
		segmentRules = sql.NullString{String: c.SegmentRules, Valid: true}
	}

	var scheduledAt sql.NullTime
	if c.ScheduledAt != nil {
		scheduledAt = sql.NullTime{Time: *c.ScheduledAt, Valid: true}
	}

	var retryConfigJSON sql.NullString
	if c.RetryConfig != nil {
		data, err := json.Marshal(c.RetryConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal retry config: %w", err)
		}
		retryConfigJSON = sql.NullString{String: string(data), Valid: true}
	}

	tags := pq.StringArray(c.SegmentTags)
	if tags == nil {
		tags = pq.StringArray{}
	}

	var row campaignRow
	err := r.db.QueryRowxContext(ctx, query,
		c.ID, c.ClientID, c.Name, c.Status, c.ContactListID, templateID, c.Source,
		segmentRules, tags, c.SendRate, scheduledAt, retryConfigJSON,
	).StructScan(&row)
	if err != nil {
		return nil, fmt.Errorf("failed to create campaign: %w", err)
	}
	return row.toDomain(), nil
}

// GetByID retrieves a campaign by ID and client ID.
func (r *CampaignRepository) GetByID(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error) {
	query := fmt.Sprintf(`SELECT %s FROM campaigns WHERE id = $1 AND client_id = $2`, campaignColumns)

	var row campaignRow
	err := r.db.QueryRowxContext(ctx, query, id, clientID).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrCampaignNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get campaign: %w", err)
	}
	return row.toDomain(), nil
}

// List retrieves campaigns for a client with optional status filter and pagination.
func (r *CampaignRepository) List(ctx context.Context, clientID uuid.UUID, status string, limit, offset int) ([]*domain.Campaign, int, error) {
	countQuery := `SELECT COUNT(*) FROM campaigns WHERE client_id = $1`
	listQuery := fmt.Sprintf(`SELECT %s FROM campaigns WHERE client_id = $1`, campaignColumns)
	args := []interface{}{clientID}

	if status != "" {
		countQuery += ` AND status = $2`
		listQuery += ` AND status = $2`
		args = append(args, status)
	}

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count campaigns: %w", err)
	}

	paramIdx := len(args) + 1
	listQuery += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, paramIdx, paramIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.QueryxContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list campaigns: %w", err)
	}
	defer rows.Close()

	var campaigns []*domain.Campaign
	for rows.Next() {
		var row campaignRow
		if err := rows.StructScan(&row); err != nil {
			return nil, 0, fmt.Errorf("failed to scan campaign: %w", err)
		}
		campaigns = append(campaigns, row.toDomain())
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}
	return campaigns, total, nil
}

// Update updates a campaign's mutable fields.
func (r *CampaignRepository) Update(ctx context.Context, c *domain.Campaign) (*domain.Campaign, error) {
	query := fmt.Sprintf(`UPDATE campaigns SET
		name = $1, contact_list_id = $2, template_id = $3, source = $4,
		segment_rules = $5, segment_tags = $6, send_rate = $7, scheduled_at = $8,
		updated_at = now()
		WHERE id = $9 AND client_id = $10
		RETURNING %s`, campaignColumns)

	var templateID sql.NullString
	if c.TemplateID != nil {
		templateID = sql.NullString{String: c.TemplateID.String(), Valid: true}
	}

	var segmentRules sql.NullString
	if c.SegmentRules != "" {
		segmentRules = sql.NullString{String: c.SegmentRules, Valid: true}
	}

	var scheduledAt sql.NullTime
	if c.ScheduledAt != nil {
		scheduledAt = sql.NullTime{Time: *c.ScheduledAt, Valid: true}
	}

	tags := pq.StringArray(c.SegmentTags)
	if tags == nil {
		tags = pq.StringArray{}
	}

	var row campaignRow
	err := r.db.QueryRowxContext(ctx, query,
		c.Name, c.ContactListID, templateID, c.Source,
		segmentRules, tags, c.SendRate, scheduledAt,
		c.ID, c.ClientID,
	).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrCampaignNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update campaign: %w", err)
	}
	return row.toDomain(), nil
}

// Delete removes a campaign (only if draft).
func (r *CampaignRepository) Delete(ctx context.Context, id, clientID uuid.UUID) error {
	query := `DELETE FROM campaigns WHERE id = $1 AND client_id = $2 AND status = 'draft'`
	result, err := r.db.ExecContext(ctx, query, id, clientID)
	if err != nil {
		return fmt.Errorf("failed to delete campaign: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return domain.ErrCampaignNotFound
	}
	return nil
}

// UpdateStatus updates a campaign's status and optional timestamp fields.
func (r *CampaignRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, startedAt, completedAt *time.Time) error {
	var sa sql.NullTime
	if startedAt != nil {
		sa = sql.NullTime{Time: *startedAt, Valid: true}
	}
	var ca sql.NullTime
	if completedAt != nil {
		ca = sql.NullTime{Time: *completedAt, Valid: true}
	}

	query := `UPDATE campaigns SET status = $1, started_at = COALESCE($2, started_at),
		completed_at = COALESCE($3, completed_at), updated_at = now()
		WHERE id = $4`

	result, err := r.db.ExecContext(ctx, query, status, sa, ca, id)
	if err != nil {
		return fmt.Errorf("failed to update campaign status: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return domain.ErrCampaignNotFound
	}
	return nil
}

// UpdateCounters updates the campaign aggregate counters.
func (r *CampaignRepository) UpdateCounters(ctx context.Context, id uuid.UUID, totalRecipients, sentCount, deliveredCount, failedCount int32) error {
	query := `UPDATE campaigns SET total_recipients = $1, sent_count = $2, delivered_count = $3,
		failed_count = $4, updated_at = now() WHERE id = $5`

	_, err := r.db.ExecContext(ctx, query, totalRecipients, sentCount, deliveredCount, failedCount, id)
	if err != nil {
		return fmt.Errorf("failed to update campaign counters: %w", err)
	}
	return nil
}

// UpdateRetryConfig updates the retry config JSONB field.
func (r *CampaignRepository) UpdateRetryConfig(ctx context.Context, id uuid.UUID, rc *domain.RetryConfig) error {
	data, err := json.Marshal(rc)
	if err != nil {
		return fmt.Errorf("failed to marshal retry config: %w", err)
	}

	query := `UPDATE campaigns SET retry_config = $1, updated_at = now() WHERE id = $2`
	_, err = r.db.ExecContext(ctx, query, string(data), id)
	if err != nil {
		return fmt.Errorf("failed to update retry config: %w", err)
	}
	return nil
}

// --- Variants ---

// SetVariants replaces all variants for a campaign (delete + insert in a transaction).
func (r *CampaignRepository) SetVariants(ctx context.Context, campaignID uuid.UUID, variants []domain.Variant) ([]domain.Variant, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing variants
	_, err = tx.ExecContext(ctx, `DELETE FROM campaign_variants WHERE campaign_id = $1`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete old variants: %w", err)
	}

	insertQuery := `INSERT INTO campaign_variants (id, campaign_id, name, template_id, percentage, is_winner, is_control,
		sent_count, delivered_count, failed_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 0, 0, 0)
		RETURNING id, campaign_id, name, template_id, percentage, is_winner, is_control, sent_count, delivered_count, failed_count`

	var result []domain.Variant
	for _, v := range variants {
		if v.ID == uuid.Nil {
			v.ID = uuid.New()
		}
		var templateID sql.NullString
		if v.TemplateID != nil {
			templateID = sql.NullString{String: v.TemplateID.String(), Valid: true}
		}

		var row variantRow
		err := tx.QueryRowxContext(ctx, insertQuery,
			v.ID, campaignID, v.Name, templateID, v.Percentage, v.IsWinner, v.IsControl,
		).StructScan(&row)
		if err != nil {
			return nil, fmt.Errorf("failed to insert variant %s: %w", v.Name, err)
		}
		result = append(result, row.toDomain())
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit variants: %w", err)
	}
	return result, nil
}

// GetVariants retrieves all variants for a campaign.
func (r *CampaignRepository) GetVariants(ctx context.Context, campaignID uuid.UUID) ([]domain.Variant, error) {
	query := `SELECT id, campaign_id, name, template_id, percentage, is_winner, is_control,
		sent_count, delivered_count, failed_count
		FROM campaign_variants WHERE campaign_id = $1
		ORDER BY percentage DESC`

	rows, err := r.db.QueryxContext(ctx, query, campaignID)
	if err != nil {
		return nil, fmt.Errorf("failed to get variants: %w", err)
	}
	defer rows.Close()

	var variants []domain.Variant
	for rows.Next() {
		var row variantRow
		if err := rows.StructScan(&row); err != nil {
			return nil, fmt.Errorf("failed to scan variant: %w", err)
		}
		variants = append(variants, row.toDomain())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return variants, nil
}

// --- AB Config ---

// SetABConfig upserts the A/B config for a campaign.
func (r *CampaignRepository) SetABConfig(ctx context.Context, cfg *domain.ABConfig) (*domain.ABConfig, error) {
	query := `INSERT INTO campaign_ab_config (campaign_id, metric, test_duration_hours, auto_select_winner)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (campaign_id) DO UPDATE SET
			metric = EXCLUDED.metric,
			test_duration_hours = EXCLUDED.test_duration_hours,
			auto_select_winner = EXCLUDED.auto_select_winner
		RETURNING campaign_id, metric, test_duration_hours, auto_select_winner, winner_variant_id, winner_selected_at`

	var row abConfigRow
	err := r.db.QueryRowxContext(ctx, query,
		cfg.CampaignID, cfg.Metric, cfg.TestDurationHours, cfg.AutoSelectWinner,
	).StructScan(&row)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert AB config: %w", err)
	}
	return row.toDomain(), nil
}

// GetABConfig retrieves the A/B config for a campaign.
func (r *CampaignRepository) GetABConfig(ctx context.Context, campaignID uuid.UUID) (*domain.ABConfig, error) {
	query := `SELECT campaign_id, metric, test_duration_hours, auto_select_winner, winner_variant_id, winner_selected_at
		FROM campaign_ab_config WHERE campaign_id = $1`

	var row abConfigRow
	err := r.db.QueryRowxContext(ctx, query, campaignID).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get AB config: %w", err)
	}
	return row.toDomain(), nil
}

// SelectWinner marks a variant as winner in both AB config and variants table.
func (r *CampaignRepository) SelectWinner(ctx context.Context, campaignID, variantID uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Reset all variants
	_, err = tx.ExecContext(ctx, `UPDATE campaign_variants SET is_winner = false WHERE campaign_id = $1`, campaignID)
	if err != nil {
		return fmt.Errorf("failed to reset variant winners: %w", err)
	}

	// Set the winner
	result, err := tx.ExecContext(ctx, `UPDATE campaign_variants SET is_winner = true WHERE id = $1 AND campaign_id = $2`, variantID, campaignID)
	if err != nil {
		return fmt.Errorf("failed to set variant winner: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return domain.ErrVariantNotFound
	}

	// Update AB config
	now := time.Now()
	_, err = tx.ExecContext(ctx,
		`UPDATE campaign_ab_config SET winner_variant_id = $1, winner_selected_at = $2 WHERE campaign_id = $3`,
		variantID.String(), now, campaignID)
	if err != nil {
		return fmt.Errorf("failed to update AB config winner: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit winner selection: %w", err)
	}
	return nil
}

// GetRunningCampaigns retrieves all campaigns with status 'running'.
func (r *CampaignRepository) GetRunningCampaigns(ctx context.Context) ([]*domain.Campaign, error) {
	query := fmt.Sprintf(`SELECT %s FROM campaigns WHERE status = 'running' ORDER BY started_at ASC`, campaignColumns)

	rows, err := r.db.QueryxContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get running campaigns: %w", err)
	}
	defer rows.Close()

	var campaigns []*domain.Campaign
	for rows.Next() {
		var row campaignRow
		if err := rows.StructScan(&row); err != nil {
			return nil, fmt.Errorf("failed to scan campaign: %w", err)
		}
		campaigns = append(campaigns, row.toDomain())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return campaigns, nil
}

// GetCampaignsWithRetry retrieves campaigns that have retry config enabled and are running or completed.
func (r *CampaignRepository) GetCampaignsWithRetry(ctx context.Context) ([]*domain.Campaign, error) {
	query := fmt.Sprintf(`SELECT %s FROM campaigns
		WHERE retry_config IS NOT NULL
		AND retry_config::jsonb->>'enabled' = 'true'
		AND status IN ('running', 'completed')
		ORDER BY created_at ASC`, campaignColumns)

	rows, err := r.db.QueryxContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get campaigns with retry: %w", err)
	}
	defer rows.Close()

	var campaigns []*domain.Campaign
	for rows.Next() {
		var row campaignRow
		if err := rows.StructScan(&row); err != nil {
			return nil, fmt.Errorf("failed to scan campaign: %w", err)
		}
		campaigns = append(campaigns, row.toDomain())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return campaigns, nil
}
