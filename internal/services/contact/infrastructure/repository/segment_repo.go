package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
)

type SegmentRepository struct {
	pool *pgxpool.Pool
}

func NewSegmentRepository(pool *pgxpool.Pool) *SegmentRepository {
	return &SegmentRepository{pool: pool}
}

func (r *SegmentRepository) Create(ctx context.Context, seg *domain.SavedSegment) error {
	seg.ID = uuid.New()
	rulesJSON, err := json.Marshal(seg.Rules)
	if err != nil {
		return fmt.Errorf("marshal rules: %w", err)
	}
	var tagJSON []byte
	if seg.TagRules != nil {
		tagJSON, _ = json.Marshal(seg.TagRules)
	}

	_, err = r.pool.Exec(ctx,
		`INSERT INTO saved_segments (id, client_id, name, description, contact_list_ids, rules, tag_rules)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		seg.ID, seg.ClientID, seg.Name, seg.Description, seg.ContactListIDs, rulesJSON, tagJSON,
	)
	return err
}

func (r *SegmentRepository) GetByID(ctx context.Context, id, clientID uuid.UUID) (*domain.SavedSegment, error) {
	var seg domain.SavedSegment
	var rulesJSON, tagJSON []byte
	err := r.pool.QueryRow(ctx,
		`SELECT id, client_id, name, description, contact_list_ids, rules, tag_rules, estimated_count, estimated_at, created_at, updated_at
		 FROM saved_segments WHERE id = $1 AND client_id = $2`, id, clientID,
	).Scan(&seg.ID, &seg.ClientID, &seg.Name, &seg.Description, &seg.ContactListIDs,
		&rulesJSON, &tagJSON, &seg.EstimatedCount, &seg.EstimatedAt, &seg.CreatedAt, &seg.UpdatedAt)
	if err != nil {
		return nil, domain.ErrSegmentNotFound
	}
	json.Unmarshal(rulesJSON, &seg.Rules)
	if tagJSON != nil {
		seg.TagRules = &domain.TagRules{}
		json.Unmarshal(tagJSON, seg.TagRules)
	}
	return &seg, nil
}

func (r *SegmentRepository) ListByClient(ctx context.Context, clientID uuid.UUID) ([]*domain.SavedSegment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, description, contact_list_ids, estimated_count, estimated_at, created_at
		 FROM saved_segments WHERE client_id = $1 ORDER BY created_at DESC`, clientID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var segments []*domain.SavedSegment
	for rows.Next() {
		var seg domain.SavedSegment
		seg.ClientID = clientID
		if err := rows.Scan(&seg.ID, &seg.Name, &seg.Description, &seg.ContactListIDs,
			&seg.EstimatedCount, &seg.EstimatedAt, &seg.CreatedAt); err != nil {
			return nil, err
		}
		segments = append(segments, &seg)
	}
	return segments, nil
}

func (r *SegmentRepository) Update(ctx context.Context, seg *domain.SavedSegment) error {
	rulesJSON, err := json.Marshal(seg.Rules)
	if err != nil {
		return err
	}
	var tagJSON []byte
	if seg.TagRules != nil {
		tagJSON, _ = json.Marshal(seg.TagRules)
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE saved_segments SET name=$3, description=$4, contact_list_ids=$5, rules=$6, tag_rules=$7, updated_at=now()
		 WHERE id=$1 AND client_id=$2`,
		seg.ID, seg.ClientID, seg.Name, seg.Description, seg.ContactListIDs, rulesJSON, tagJSON,
	)
	return err
}

func (r *SegmentRepository) Delete(ctx context.Context, id, clientID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM saved_segments WHERE id = $1 AND client_id = $2`, id, clientID)
	return err
}

func (r *SegmentRepository) UpdateEstimate(ctx context.Context, id uuid.UUID, count int32) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE saved_segments SET estimated_count = $2, estimated_at = now() WHERE id = $1`, id, count,
	)
	return err
}
