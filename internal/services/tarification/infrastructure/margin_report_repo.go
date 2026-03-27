package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

type MarginReportRepo struct {
	pool *pgxpool.Pool
}

func NewMarginReportRepo(pool *pgxpool.Pool) *MarginReportRepo {
	return &MarginReportRepo{pool: pool}
}

func (r *MarginReportRepo) GetMarginReport(ctx context.Context, clientID uuid.UUID, from, to time.Time) ([]*domain.MarginReportEntry, error) {
	query := `
		SELECT
			COALESCE(rev.operator_id, cost.operator_id) AS operator_id,
			COALESCE(o.name, '') AS operator_name,
			COALESCE(cost.provider_id, '00000000-0000-0000-0000-000000000000') AS provider_id,
			COALESCE(p.name, '') AS provider_name,
			COALESCE(rev.segments, 0) + COALESCE(cost.segments, 0) AS segments,
			COALESCE(rev.revenue, 0) AS revenue,
			COALESCE(cost.total_cost, 0) AS cost,
			COALESCE(rev.revenue, 0) - COALESCE(cost.total_cost, 0) AS margin
		FROM (
			SELECT operator_id, SUM(segment_count) AS segments, SUM(total_amount) AS revenue
			FROM tarification_log
			WHERE client_id = $1 AND created_at >= $2 AND created_at < $3
			GROUP BY operator_id
		) rev
		FULL OUTER JOIN (
			SELECT operator_id, provider_id, SUM(segment_count) AS segments, SUM(total_cost) AS total_cost
			FROM provider_tarification_log
			WHERE client_id = $1 AND created_at >= $2 AND created_at < $3
			GROUP BY operator_id, provider_id
		) cost ON rev.operator_id = cost.operator_id
		LEFT JOIN operators o ON o.id = COALESCE(rev.operator_id, cost.operator_id)
		LEFT JOIN providers p ON p.id = cost.provider_id
		ORDER BY operator_name, provider_name`

	rows, err := r.pool.Query(ctx, query, clientID, from, to)
	if err != nil {
		return nil, fmt.Errorf("margin report query: %w", err)
	}
	defer rows.Close()

	var result []*domain.MarginReportEntry
	for rows.Next() {
		e := &domain.MarginReportEntry{}
		if err := rows.Scan(&e.OperatorID, &e.OperatorName, &e.ProviderID, &e.ProviderName, &e.Segments, &e.Revenue, &e.Cost, &e.Margin); err != nil {
			return nil, fmt.Errorf("scan margin report row: %w", err)
		}
		result = append(result, e)
	}
	return result, nil
}
