package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smpp-server/smpp-server/internal/services/network_analytics/domain"
)

// ViewsRepo implements domain.ViewsRepository using PostgreSQL.
type ViewsRepo struct {
	db *pgxpool.Pool
}

// NewViewsRepo creates a new ViewsRepo.
func NewViewsRepo(db *pgxpool.Pool) *ViewsRepo {
	return &ViewsRepo{db: db}
}

// List returns saved views for the given partner and user, plus global templates (user_id IS NULL).
func (r *ViewsRepo) List(ctx context.Context, partnerID, userID int64) ([]domain.SavedView, error) {
	query := `
		SELECT id, partner_id, user_id, name, is_default, mode,
			COALESCE(filters, '{}'), COALESCE(group_by, ''), COALESCE(sort_by, ''), COALESCE(sort_dir, 'desc'), COALESCE(columns, '{}'), created_at, updated_at
		FROM saved_views
		WHERE (partner_id = $1 AND user_id = $2) OR (user_id IS NULL)
		ORDER BY is_default DESC, updated_at DESC`

	rows, err := r.db.Query(ctx, query, partnerID, userID)
	if err != nil {
		return nil, fmt.Errorf("list saved views: %w", err)
	}
	defer rows.Close()

	var views []domain.SavedView
	for rows.Next() {
		v, err := scanView(rows)
		if err != nil {
			return nil, fmt.Errorf("scan saved view: %w", err)
		}
		views = append(views, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return views, nil
}

// Save inserts or updates a saved view, upserting on (partner_id, user_id, name).
func (r *ViewsRepo) Save(ctx context.Context, view *domain.SavedView) (*domain.SavedView, error) {
	now := time.Now()
	if view.CreatedAt.IsZero() {
		view.CreatedAt = now
	}
	view.UpdatedAt = now

	query := `
		INSERT INTO saved_views (
			partner_id, user_id, name, is_default, mode,
			filters, group_by, sort_by, sort_dir, columns,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12
		)
		ON CONFLICT (partner_id, user_id, name) DO UPDATE SET
			is_default = EXCLUDED.is_default,
			mode       = EXCLUDED.mode,
			filters    = EXCLUDED.filters,
			group_by   = EXCLUDED.group_by,
			sort_by    = EXCLUDED.sort_by,
			sort_dir   = EXCLUDED.sort_dir,
			columns    = EXCLUDED.columns,
			updated_at = EXCLUDED.updated_at
		RETURNING id, partner_id, user_id, name, is_default, mode,
			COALESCE(filters, '{}'), COALESCE(group_by, ''), COALESCE(sort_by, ''), COALESCE(sort_dir, 'desc'), COALESCE(columns, '{}'), created_at, updated_at`

	row := r.db.QueryRow(ctx, query,
		view.PartnerID, view.UserID, view.Name, view.IsDefault, view.Mode,
		view.Filters, view.GroupBy, view.SortBy, view.SortDir, view.Columns,
		view.CreatedAt, view.UpdatedAt,
	)

	saved, err := scanViewRow(row)
	if err != nil {
		return nil, fmt.Errorf("save view: %w", err)
	}
	return saved, nil
}

// Delete removes a saved view. Returns one of the domain sentinel errors:
//   - domain.ErrViewNotFound     — no row with this id
//   - domain.ErrViewIsTemplate   — view is a system template (user_id IS NULL); clone instead
//   - domain.ErrViewForbidden    — view exists but is owned by a different (partner_id, user_id)
//
// The schema (migrations/000102) has no explicit is_template column — a view is a
// template iff user_id IS NULL (see seed in migration). That's what we check here.
func (r *ViewsRepo) Delete(ctx context.Context, id, partnerID, userID int64) error {
	var ownerPartner int64
	var ownerUser pgtype.Int8
	err := r.db.QueryRow(ctx,
		`SELECT partner_id, user_id FROM saved_views WHERE id = $1`,
		id,
	).Scan(&ownerPartner, &ownerUser)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrViewNotFound
		}
		return fmt.Errorf("lookup saved view %d: %w", id, err)
	}

	// user_id IS NULL → system template, no one may delete it.
	if !ownerUser.Valid {
		return domain.ErrViewIsTemplate
	}

	// Ownership mismatch.
	if ownerPartner != partnerID || ownerUser.Int64 != userID {
		return domain.ErrViewForbidden
	}

	if _, err := r.db.Exec(ctx, `DELETE FROM saved_views WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete saved view %d: %w", id, err)
	}
	return nil
}

// scanView scans a SavedView from pgx.Rows.
func scanView(rows pgx.Rows) (domain.SavedView, error) {
	var v domain.SavedView
	err := rows.Scan(
		&v.ID, &v.PartnerID, &v.UserID, &v.Name, &v.IsDefault, &v.Mode,
		&v.Filters, &v.GroupBy, &v.SortBy, &v.SortDir, &v.Columns,
		&v.CreatedAt, &v.UpdatedAt,
	)
	return v, err
}

// scanViewRow scans a SavedView from pgx.Row (used after RETURNING).
func scanViewRow(row pgx.Row) (*domain.SavedView, error) {
	var v domain.SavedView
	err := row.Scan(
		&v.ID, &v.PartnerID, &v.UserID, &v.Name, &v.IsDefault, &v.Mode,
		&v.Filters, &v.GroupBy, &v.SortBy, &v.SortDir, &v.Columns,
		&v.CreatedAt, &v.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}
