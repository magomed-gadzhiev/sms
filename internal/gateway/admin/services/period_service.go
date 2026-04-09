package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// sentinel errors
var (
	ErrMissingCountry           = errors.New("operator_id requires country_id")
	ErrMissingOperator          = errors.New("sender_category requires operator_id")
	ErrMissingSenderCategory    = errors.New("traffic_type requires sender_category")
	ErrNoParentPeriod           = errors.New("no active period at parent level for these dates")
	ErrChildPeriodExceedsParent = errors.New("period exceeds parent period bounds")
	ErrChildPeriodsBlock        = errors.New("cannot close period: active child periods exist")
	ErrPeriodOverlap            = errors.New("period overlaps with existing period in this scope")
	ErrHasChildPeriods          = errors.New("cannot delete: child periods exist")
	ErrPeriodNotFound           = errors.New("period not found")
	ErrRetroactiveStart         = errors.New("start_date must be today or in the future")
	ErrInvalidStrategy          = errors.New("invalid strategy")
	ErrInvalidSenderCategory    = errors.New("invalid sender_category")
	ErrInvalidTrafficType       = errors.New("invalid traffic_type")
)

// PeriodDimensions holds all optional dimension values for a hierarchical period.
type PeriodDimensions struct {
	CountryID      *uuid.UUID
	OperatorID     *uuid.UUID
	SenderCategory *string
	TrafficType    *string
	ClientID       *uuid.UUID
}

// HierarchicalPeriod is the full period record returned from DB.
type HierarchicalPeriod struct {
	ID            uuid.UUID
	PeriodDimensions
	ScopeKey      string
	ScopePriority int
	Strategy      string
	StartDate     time.Time
	EndDate       *time.Time // nil = open-ended
	CreatedAt     time.Time
}

// CreatePeriodInput is the validated input for period creation.
type CreatePeriodInput struct {
	PeriodDimensions
	Strategy  string
	StartDate time.Time
	EndDate   *time.Time
}

// UpdatePeriodInput allows editing end_date and strategy only.
type UpdatePeriodInput struct {
	EndDate  *time.Time // nil = open-ended
	Strategy *string
}

// AutoCloseWarning describes a period that will be auto-closed.
type AutoCloseWarning struct {
	PeriodID   uuid.UUID
	OldEndDate *time.Time // nil = was open-ended
	NewEndDate time.Time
}

// ComputeScopeKey builds the canonical scope key from filled dimensions,
// sorted alphabetically so the key is stable regardless of insertion order.
func ComputeScopeKey(d PeriodDimensions) string {
	parts := map[string]string{}
	if d.ClientID != nil {
		parts["client"] = d.ClientID.String()
	}
	if d.CountryID != nil {
		parts["country"] = d.CountryID.String()
	}
	if d.OperatorID != nil {
		parts["operator"] = d.OperatorID.String()
	}
	if d.SenderCategory != nil {
		parts["sender_category"] = *d.SenderCategory
	}
	if d.TrafficType != nil {
		parts["traffic_type"] = *d.TrafficType
	}
	if len(parts) == 0 {
		return "global"
	}
	keys := make([]string, 0, len(parts))
	for k := range parts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	segments := make([]string, 0, len(keys))
	for _, k := range keys {
		segments = append(segments, k+":"+parts[k])
	}
	return strings.Join(segments, "|")
}

// ComputeScopePriority returns the numeric priority based on the most specific
// dimension filled (excluding client_id), plus 100 if client_id is set.
func ComputeScopePriority(d PeriodDimensions) int {
	base := 0
	if d.CountryID != nil {
		base = 10
	}
	if d.OperatorID != nil {
		base = 20
	}
	if d.SenderCategory != nil {
		base = 30
	}
	if d.TrafficType != nil {
		base = 40
	}
	if d.ClientID != nil {
		base += 100
	}
	return base
}

// ValidateDimensionHierarchy enforces the strict hierarchy rule.
func ValidateDimensionHierarchy(d PeriodDimensions) error {
	if d.OperatorID != nil && d.CountryID == nil {
		return ErrMissingCountry
	}
	if d.SenderCategory != nil && d.OperatorID == nil {
		return ErrMissingOperator
	}
	if d.TrafficType != nil && d.SenderCategory == nil {
		return ErrMissingSenderCategory
	}
	return nil
}

// ParentScopeKey computes the parent's scope key by removing the most specific dimension.
// Returns ("", false) for global scope (no parent).
func ParentScopeKey(d PeriodDimensions) (string, bool) {
	if d.ClientID != nil {
		noClient := d
		noClient.ClientID = nil
		return ComputeScopeKey(noClient), true
	}
	if d.TrafficType != nil {
		noTraffic := d
		noTraffic.TrafficType = nil
		return ComputeScopeKey(noTraffic), true
	}
	if d.SenderCategory != nil {
		noCat := d
		noCat.SenderCategory = nil
		return ComputeScopeKey(noCat), true
	}
	if d.OperatorID != nil {
		noOp := d
		noOp.OperatorID = nil
		return ComputeScopeKey(noOp), true
	}
	if d.CountryID != nil {
		return "global", true
	}
	return "", false
}

// PeriodService implements business logic for hierarchical tariff periods.
type PeriodService struct {
	db     *storage.DB
	logger zerolog.Logger
}

// NewPeriodService creates a new PeriodService.
func NewPeriodService(db *storage.DB) *PeriodService {
	return &PeriodService{
		db:     db,
		logger: log.With().Str("component", "period-service").Logger(),
	}
}

func validateStrategy(s string) error {
	switch s {
	case "fixed", "threshold", "threshold_recalc", "prepaid_threshold":
		return nil
	}
	return ErrInvalidStrategy
}

func validateSenderCategory(s string) error {
	switch s {
	case "shared", "paid_registered", "free_registered":
		return nil
	}
	return ErrInvalidSenderCategory
}

func validateTrafficType(s string) error {
	switch s {
	case "authorization", "transactional", "service", "extensible":
		return nil
	}
	return ErrInvalidTrafficType
}

// ListPeriods returns periods filtered by any combination of dimensions.
func (s *PeriodService) ListPeriods(ctx context.Context, filter PeriodDimensions) ([]*HierarchicalPeriod, error) {
	query := `
		SELECT id, country_id, operator_id, sender_category, traffic_type, client_id,
		       scope_key, scope_priority, strategy, start_date, end_date, created_at
		FROM tariff_periods_new
		WHERE ($1::uuid IS NULL OR country_id = $1)
		  AND ($2::uuid IS NULL OR operator_id = $2)
		  AND ($3::text IS NULL OR sender_category = $3)
		  AND ($4::text IS NULL OR traffic_type = $4)
		  AND ($5::uuid IS NULL OR client_id = $5)
		ORDER BY scope_priority DESC, start_date DESC`

	rows, err := s.db.QueryContext(ctx, query,
		filter.CountryID, filter.OperatorID, filter.SenderCategory, filter.TrafficType, filter.ClientID)
	if err != nil {
		return nil, fmt.Errorf("list periods: %w", err)
	}
	defer rows.Close()

	var result []*HierarchicalPeriod
	for rows.Next() {
		p, err := scanPeriod(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// GetPeriod returns a single period by ID.
func (s *PeriodService) GetPeriod(ctx context.Context, id uuid.UUID) (*HierarchicalPeriod, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, country_id, operator_id, sender_category, traffic_type, client_id,
		       scope_key, scope_priority, strategy, start_date, end_date, created_at
		FROM tariff_periods_new WHERE id = $1`, id)
	p, err := scanPeriod(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPeriodNotFound
	}
	return p, err
}

// CreatePeriod creates a new hierarchical period with full lifecycle validation.
func (s *PeriodService) CreatePeriod(ctx context.Context, in CreatePeriodInput) (*HierarchicalPeriod, *AutoCloseWarning, error) {
	if err := validateStrategy(in.Strategy); err != nil {
		return nil, nil, err
	}
	if in.SenderCategory != nil {
		if err := validateSenderCategory(*in.SenderCategory); err != nil {
			return nil, nil, err
		}
	}
	if in.TrafficType != nil {
		if err := validateTrafficType(*in.TrafficType); err != nil {
			return nil, nil, err
		}
	}
	if err := ValidateDimensionHierarchy(in.PeriodDimensions); err != nil {
		return nil, nil, err
	}

	today := time.Now().Truncate(24 * time.Hour)
	if in.StartDate.Before(today) {
		return nil, nil, ErrRetroactiveStart
	}

	scopeKey := ComputeScopeKey(in.PeriodDimensions)
	scopePriority := ComputeScopePriority(in.PeriodDimensions)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Containment check (skip for global scope)
	if scopePriority > 0 {
		parentKey, hasParent := ParentScopeKey(in.PeriodDimensions)
		if hasParent {
			parent, err := findActivePeriodInScope(ctx, tx, parentKey, in.StartDate)
			if err != nil {
				return nil, nil, fmt.Errorf("find parent period: %w", err)
			}
			if parent == nil {
				return nil, nil, ErrNoParentPeriod
			}
			if in.StartDate.Before(parent.StartDate) {
				return nil, nil, fmt.Errorf("%w: starts before parent (%s)", ErrChildPeriodExceedsParent, parent.StartDate.Format("2006-01-02"))
			}
			if parent.EndDate != nil && in.EndDate != nil && in.EndDate.After(*parent.EndDate) {
				return nil, nil, fmt.Errorf("%w: ends after parent (%s)", ErrChildPeriodExceedsParent, parent.EndDate.Format("2006-01-02"))
			}
			if parent.EndDate != nil && in.EndDate == nil {
				return nil, nil, fmt.Errorf("%w: parent is not open-ended, child cannot be open-ended", ErrChildPeriodExceedsParent)
			}
		}
	}

	// Auto-close: find open or future-overlapping period in same scope
	var warning *AutoCloseWarning
	prev, err := findOpenOrFuturePeriodInScope(ctx, tx, scopeKey, in.StartDate)
	if err != nil {
		return nil, nil, fmt.Errorf("check existing periods: %w", err)
	}
	if prev != nil {
		if prev.EndDate != nil {
			return nil, nil, fmt.Errorf("%w with period %s–%s", ErrPeriodOverlap, prev.StartDate.Format("2006-01-02"), prev.EndDate.Format("2006-01-02"))
		}
		if err := assertNoActiveChildren(ctx, tx, prev.ID, in.StartDate.AddDate(0, 0, -1)); err != nil {
			return nil, nil, err
		}
		newEnd := in.StartDate.AddDate(0, 0, -1)
		_, err = tx.ExecContext(ctx, `UPDATE tariff_periods_new SET end_date = $1 WHERE id = $2`, newEnd, prev.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("auto-close previous period: %w", err)
		}
		warning = &AutoCloseWarning{PeriodID: prev.ID, OldEndDate: prev.EndDate, NewEndDate: newEnd}
	}

	id := uuid.New()
	now := time.Now()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tariff_periods_new
		  (id, country_id, operator_id, sender_category, traffic_type, client_id,
		   scope_key, scope_priority, strategy, start_date, end_date, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		id,
		in.CountryID, in.OperatorID, in.SenderCategory, in.TrafficType, in.ClientID,
		scopeKey, scopePriority, in.Strategy, in.StartDate, in.EndDate, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "no_overlap_in_scope") {
			return nil, nil, ErrPeriodOverlap
		}
		return nil, nil, fmt.Errorf("insert period: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit: %w", err)
	}

	s.logger.Info().
		Str("id", id.String()).
		Str("scope_key", scopeKey).
		Int("scope_priority", scopePriority).
		Msg("hierarchical period created")

	period := &HierarchicalPeriod{
		ID:               id,
		PeriodDimensions: in.PeriodDimensions,
		ScopeKey:         scopeKey,
		ScopePriority:    scopePriority,
		Strategy:         in.Strategy,
		StartDate:        in.StartDate,
		EndDate:          in.EndDate,
		CreatedAt:        now,
	}
	return period, warning, nil
}

// UpdatePeriod allows editing end_date and strategy only.
func (s *PeriodService) UpdatePeriod(ctx context.Context, id uuid.UUID, in UpdatePeriodInput) (*HierarchicalPeriod, error) {
	// First, get the period (can be outside tx since we re-read inside)
	period, err := s.GetPeriod(ctx, id)
	if err != nil {
		return nil, err
	}

	if in.Strategy != nil {
		if err := validateStrategy(*in.Strategy); err != nil {
			return nil, err
		}
		period.Strategy = *in.Strategy
	}

	endDateChanged := (in.EndDate == nil) != (period.EndDate == nil) ||
		(in.EndDate != nil && period.EndDate != nil && !in.EndDate.Equal(*period.EndDate))

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if endDateChanged {
		if in.EndDate != nil {
			if err := assertNoChildExceedsDateTx(ctx, tx, period.ScopeKey, *in.EndDate); err != nil {
				return nil, err
			}
		}
		period.EndDate = in.EndDate
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE tariff_periods_new SET strategy = $1, end_date = $2 WHERE id = $3`,
		period.Strategy, period.EndDate, id)
	if err != nil {
		return nil, fmt.Errorf("update period: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return period, nil
}

// DeletePeriod deletes a period if it has no child periods. Cascades to tiers.
func (s *PeriodService) DeletePeriod(ctx context.Context, id uuid.UUID) error {
	period, err := s.GetPeriod(ctx, id)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var count int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tariff_periods_new
		WHERE scope_priority > $1
		  AND scope_key LIKE $2 || '%'
		  AND scope_key != $2`, period.ScopePriority, period.ScopeKey).Scan(&count)
	if err != nil {
		return fmt.Errorf("check child periods: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%w (%d)", ErrHasChildPeriods, count)
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM tariff_periods_new WHERE id = $1`, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// --- helpers ---

type scanner interface {
	Scan(dest ...any) error
}

func scanPeriod(row scanner) (*HierarchicalPeriod, error) {
	var p HierarchicalPeriod
	var countryID, operatorID, clientID sql.NullString
	var senderCategory, trafficType sql.NullString
	var endDate sql.NullTime

	err := row.Scan(
		&p.ID, &countryID, &operatorID, &senderCategory, &trafficType, &clientID,
		&p.ScopeKey, &p.ScopePriority, &p.Strategy, &p.StartDate, &endDate, &p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if countryID.Valid {
		id := uuid.MustParse(countryID.String)
		p.CountryID = &id
	}
	if operatorID.Valid {
		id := uuid.MustParse(operatorID.String)
		p.OperatorID = &id
	}
	if clientID.Valid {
		id := uuid.MustParse(clientID.String)
		p.ClientID = &id
	}
	if senderCategory.Valid {
		p.SenderCategory = &senderCategory.String
	}
	if trafficType.Valid {
		p.TrafficType = &trafficType.String
	}
	if endDate.Valid {
		t := endDate.Time
		p.EndDate = &t
	}
	return &p, nil
}

func findActivePeriodInScope(ctx context.Context, tx *sql.Tx, scopeKey string, date time.Time) (*HierarchicalPeriod, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, country_id, operator_id, sender_category, traffic_type, client_id,
		       scope_key, scope_priority, strategy, start_date, end_date, created_at
		FROM tariff_periods_new
		WHERE scope_key = $1
		  AND start_date <= $2
		  AND (end_date IS NULL OR end_date >= $2)
		LIMIT 1`, scopeKey, date)
	p, err := scanPeriod(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

func findOpenOrFuturePeriodInScope(ctx context.Context, tx *sql.Tx, scopeKey string, startDate time.Time) (*HierarchicalPeriod, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, country_id, operator_id, sender_category, traffic_type, client_id,
		       scope_key, scope_priority, strategy, start_date, end_date, created_at
		FROM tariff_periods_new
		WHERE scope_key = $1
		  AND (end_date IS NULL OR end_date >= $2)
		LIMIT 1`, scopeKey, startDate)
	p, err := scanPeriod(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

func assertNoActiveChildren(ctx context.Context, tx *sql.Tx, parentID uuid.UUID, newEnd time.Time) error {
	var childKeys []string
	rows, err := tx.QueryContext(ctx, `
		SELECT p.scope_key
		FROM tariff_periods_new p
		WHERE p.scope_priority > (SELECT scope_priority FROM tariff_periods_new WHERE id = $1)
		  AND p.scope_key LIKE (SELECT scope_key FROM tariff_periods_new WHERE id = $1) || '%'
		  AND (p.end_date IS NULL OR p.end_date > $2)`, parentID, newEnd)
	if err != nil {
		return fmt.Errorf("check children: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return err
		}
		childKeys = append(childKeys, k)
	}
	if len(childKeys) > 0 {
		return fmt.Errorf("%w: %s", ErrChildPeriodsBlock, strings.Join(childKeys, ", "))
	}
	return nil
}

func assertNoChildExceedsDate(ctx context.Context, db *storage.DB, scopeKey string, newEnd time.Time) error {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tariff_periods_new
		WHERE scope_key LIKE $1 || '%'
		  AND scope_key != $1
		  AND (end_date IS NULL OR end_date > $2)`, scopeKey, newEnd).Scan(&count)
	if err != nil {
		return fmt.Errorf("check child end dates: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%w: %d child periods extend beyond new end_date", ErrChildPeriodExceedsParent, count)
	}
	return nil
}

func assertNoChildExceedsDateTx(ctx context.Context, tx *sql.Tx, scopeKey string, newEnd time.Time) error {
	var count int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tariff_periods_new
		WHERE scope_key LIKE $1 || '%'
		  AND scope_key != $1
		  AND (end_date IS NULL OR end_date > $2)`, scopeKey, newEnd).Scan(&count)
	if err != nil {
		return fmt.Errorf("check child end dates: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%w: %d child periods extend beyond new end_date", ErrChildPeriodExceedsParent, count)
	}
	return nil
}
