package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// HierarchicalPeriodLookupResult is the result of a hierarchical tariff period lookup.
type HierarchicalPeriodLookupResult struct {
	PeriodID      uuid.UUID
	Strategy      string
	ScopePriority int
}

// HierarchicalPeriodRepository provides lookup queries against tariff_periods_new.
type HierarchicalPeriodRepository struct {
	db *sqlx.DB
}

// NewHierarchicalPeriodRepository creates the repository.
func NewHierarchicalPeriodRepository(db *sqlx.DB) *HierarchicalPeriodRepository {
	return &HierarchicalPeriodRepository{db: db}
}

// FindBestPeriod returns the highest-priority matching period for the given dimensions and date.
// NULL dimension values match any value in that dimension (i.e., pass nil to match any).
func (r *HierarchicalPeriodRepository) FindBestPeriod(
	ctx context.Context,
	countryID, operatorID, clientID *uuid.UUID,
	senderCategory, trafficType *string,
	date time.Time,
) (*HierarchicalPeriodLookupResult, error) {
	var result HierarchicalPeriodLookupResult
	// The lookup: for each dimension, match exact value OR match rows where that dimension is NULL (any)
	// Client is special: prefer client-specific over any-client (handled by scope_priority DESC)
	err := r.db.QueryRowContext(ctx, `
		SELECT id, strategy, scope_priority
		FROM tariff_periods_new
		WHERE (country_id = $1 OR country_id IS NULL)
		  AND (operator_id = $2 OR operator_id IS NULL)
		  AND (sender_category = $3 OR sender_category IS NULL)
		  AND (traffic_type = $4 OR traffic_type IS NULL)
		  AND (client_id = $5 OR client_id IS NULL)
		  AND start_date <= $6
		  AND (end_date IS NULL OR end_date >= $6)
		ORDER BY scope_priority DESC
		LIMIT 1`,
		countryID, operatorID, senderCategory, trafficType, clientID, date,
	).Scan(&result.PeriodID, &result.Strategy, &result.ScopePriority)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// FindTiersWithFallback returns tiers and strategy for the hierarchical lookup.
// Strategy comes from the HIGHEST-priority matching period.
// Tiers come from the highest-priority matching period THAT HAS TIERS (tier inheritance).
// Returns nil tiers (not an error) if no matching period has tiers.
// Implements domain.HierarchicalPeriodLookup.
func (r *HierarchicalPeriodRepository) FindTiersWithFallback(
	ctx context.Context,
	countryID, operatorID, clientID *uuid.UUID,
	senderCategory, trafficType *string,
	date time.Time,
) (tiers []domain.HierarchicalTierItem, strategy string, err error) {
	// Get strategy from highest-priority period
	best, err := r.FindBestPeriod(ctx, countryID, operatorID, clientID, senderCategory, trafficType, date)
	if err != nil || best == nil {
		return nil, "", err
	}
	strategy = best.Strategy

	// Get tiers from highest-priority period that has tiers (tier inheritance)
	rows, err := r.db.QueryContext(ctx, `
		WITH matched AS (
		  SELECT id, scope_priority
		  FROM tariff_periods_new
		  WHERE (country_id = $1 OR country_id IS NULL)
		    AND (operator_id = $2 OR operator_id IS NULL)
		    AND (sender_category = $3 OR sender_category IS NULL)
		    AND (traffic_type = $4 OR traffic_type IS NULL)
		    AND (client_id = $5 OR client_id IS NULL)
		    AND start_date <= $6
		    AND (end_date IS NULL OR end_date >= $6)
		  ORDER BY scope_priority DESC
		)
		SELECT tt.from_count, tt.price_per_segment::text
		FROM tariff_tiers_new tt
		WHERE tt.tariff_period_id = (
		  SELECT id FROM matched m
		  WHERE EXISTS (SELECT 1 FROM tariff_tiers_new WHERE tariff_period_id = m.id)
		  ORDER BY m.scope_priority DESC
		  LIMIT 1
		)
		ORDER BY tt.from_count ASC`,
		countryID, operatorID, senderCategory, trafficType, clientID, date,
	)
	if err != nil {
		return nil, strategy, err
	}
	defer rows.Close()

	for rows.Next() {
		var t domain.HierarchicalTierItem
		if err := rows.Scan(&t.FromCount, &t.PricePerSegment); err != nil {
			return nil, strategy, err
		}
		tiers = append(tiers, t)
	}
	return tiers, strategy, rows.Err()
}
