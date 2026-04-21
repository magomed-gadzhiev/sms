package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smpp-server/smpp-server/internal/services/network_analytics/domain"
)

// StatsRepo implements domain.StatsRepository using PostgreSQL.
type StatsRepo struct {
	db *pgxpool.Pool
}

// NewStatsRepo creates a new StatsRepo.
func NewStatsRepo(db *pgxpool.Pool) *StatsRepo {
	return &StatsRepo{db: db}
}

// sliceColumn maps a group_by value to the corresponding SQL expression.
func sliceColumn(groupBy string) string {
	switch groupBy {
	// Dimensional groupings
	case "provider":
		return "provider_id::text"
	case "operator":
		return "operator"
	case "channel":
		return "channel"
	case "login":
		return "login"
	case "country":
		return "country"
	case "sender":
		return "sender_name"
	case "traffic_type":
		return "traffic_type"
	case "method":
		return "method"
	// Time-based groupings
	case "5min":
		return "(date_trunc('hour', hour) + INTERVAL '5 min' * FLOOR(EXTRACT(MINUTE FROM hour) / 5))::text"
	case "15min":
		return "(date_trunc('hour', hour) + INTERVAL '15 min' * FLOOR(EXTRACT(MINUTE FROM hour) / 15))::text"
	case "hour":
		return "date_trunc('hour', hour)::text"
	case "day":
		return "date_trunc('day', hour)::text"
	case "month":
		return "date_trunc('month', hour)::text"
	case "year":
		return "date_trunc('year', hour)::text"
	default:
		return "operator"
	}
}

// allowedSortColumns lists safe column names for ORDER BY.
var allowedSortColumns = map[string]string{
	"total":     "total",
	"sent":      "sent",
	"delivered": "delivered",
	"failed":    "failed",
	"pending":   "pending",
	"timeout":   "timeout",
	"error":     "error",
	"dlr_rate":  "dlr_rate",
	"revenue":   "revenue",
	"cost":      "cost",
	"profit":    "profit",
	"margin":    "margin",
	"slice":     "slice",
}

// buildWhereClause builds parameterized WHERE conditions from filter fields.
func buildWhereClause(filter *domain.SharedFilter) (string, []interface{}) {
	conditions := []string{"partner_id = $1"}
	args := []interface{}{filter.PartnerID}
	idx := 2

	if !filter.DateFrom.IsZero() {
		conditions = append(conditions, fmt.Sprintf("hour >= $%d", idx))
		args = append(args, filter.DateFrom)
		idx++
	}
	if !filter.DateTo.IsZero() {
		conditions = append(conditions, fmt.Sprintf("hour <= $%d", idx))
		args = append(args, filter.DateTo)
		idx++
	}
	if filter.Operator != "" {
		conditions = append(conditions, fmt.Sprintf("operator = $%d", idx))
		args = append(args, filter.Operator)
		idx++
	}
	if filter.Provider != "" {
		conditions = append(conditions, fmt.Sprintf("provider_id::text = $%d", idx))
		args = append(args, filter.Provider)
		idx++
	}
	if filter.Country != "" {
		conditions = append(conditions, fmt.Sprintf("country = $%d", idx))
		args = append(args, filter.Country)
		idx++
	}
	if filter.Channel != "" {
		conditions = append(conditions, fmt.Sprintf("channel = $%d", idx))
		args = append(args, filter.Channel)
		idx++
	}
	if filter.Login != "" {
		conditions = append(conditions, fmt.Sprintf("login = $%d", idx))
		args = append(args, filter.Login)
		idx++
	}
	if filter.SenderName != "" {
		conditions = append(conditions, fmt.Sprintf("sender_name = $%d", idx))
		args = append(args, filter.SenderName)
		idx++
	}
	if filter.TrafficType != "" {
		conditions = append(conditions, fmt.Sprintf("traffic_type = $%d", idx))
		args = append(args, filter.TrafficType)
		idx++
	}
	if filter.Method != "" {
		conditions = append(conditions, fmt.Sprintf("method = $%d", idx))
		args = append(args, filter.Method)
		idx++
	}

	return "WHERE " + strings.Join(conditions, " AND "), args
}

// isTimeGroup reports whether the group_by value is a time bucket.
// For time buckets the natural default sort is by slice ASC, not by total DESC.
func isTimeGroup(groupBy string) bool {
	switch groupBy {
	case "5min", "15min", "hour", "day", "month", "year":
		return true
	}
	return false
}

// buildStatsQuery builds the full aggregation query for network_stats_hourly.
func buildStatsQuery(filter *domain.SharedFilter, whereClause string, argCount int) string {
	col := sliceColumn(filter.GroupBy)

	// Default sort: time groupings get slice ASC (chronological); dimensional
	// groupings get total DESC (biggest first). Explicit sort_by overrides.
	sortCol := "total"
	sortDir := "DESC"
	if isTimeGroup(filter.GroupBy) {
		sortCol = "slice"
		sortDir = "ASC"
	}
	if sc, ok := allowedSortColumns[filter.SortBy]; ok {
		sortCol = sc
		sortDir = "DESC"
	}
	if filter.SortDir == "asc" {
		sortDir = "ASC"
	} else if filter.SortDir == "desc" {
		sortDir = "DESC"
	}

	return fmt.Sprintf(`
		SELECT
			COALESCE(%s, '') AS slice,
			SUM(total) AS total,
			SUM(sent) AS sent,
			SUM(delivered) AS delivered,
			SUM(failed) AS failed,
			SUM(pending) AS pending,
			SUM(timeout) AS timeout,
			SUM(error) AS error,
			CASE WHEN SUM(total) > 0 THEN SUM(delivered)::float / SUM(total)::float ELSE 0 END AS dlr_rate,
			COALESCE(SUM(revenue), 0) AS revenue,
			COALESCE(SUM(cost), 0) AS cost,
			COALESCE(SUM(revenue), 0) - COALESCE(SUM(cost), 0) AS profit,
			CASE WHEN COALESCE(SUM(revenue), 0) > 0
				THEN (COALESCE(SUM(revenue), 0) - COALESCE(SUM(cost), 0)) / SUM(revenue)
				ELSE 0 END AS margin,
			COALESCE(AVG(dlr_latency_p95), 0)::int AS dlr_latency_p95
		FROM network_stats_hourly
		%s
		GROUP BY %s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d`,
		col, whereClause, col, sortCol, sortDir, argCount+1, argCount+2,
	)
}

// GetStatistics returns grouped statistics rows with pagination.
func (r *StatsRepo) GetStatistics(ctx context.Context, filter *domain.SharedFilter) ([]domain.StatRow, int, error) {
	filter.Normalize()

	whereClause, args := buildWhereClause(filter)
	argCount := len(args)

	// Count total rows for pagination
	countQuery := fmt.Sprintf(`
		SELECT COUNT(DISTINCT COALESCE(%s, ''))
		FROM network_stats_hourly
		%s`, sliceColumn(filter.GroupBy), whereClause)

	var totalRows int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&totalRows); err != nil {
		return nil, 0, fmt.Errorf("count stats rows: %w", err)
	}

	offset := (filter.Page - 1) * filter.PageSize
	args = append(args, filter.PageSize, offset)

	query := buildStatsQuery(filter, whereClause, argCount)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query stats: %w", err)
	}
	defer rows.Close()

	var result []domain.StatRow
	var totalTotal, totalDelivered, totalFailed, totalPending, totalTimeout, totalError int64
	var totalRevenue, totalCost float64

	for rows.Next() {
		var row domain.StatRow
		var latencyP95 int
		if err := rows.Scan(
			&row.Slice, &row.Total, &row.Sent, &row.Delivered,
			&row.Failed, &row.Pending, &row.Timeout, &row.Error,
			&row.DLRRate, &row.Revenue, &row.Cost, &row.Profit, &row.Margin,
			&latencyP95,
		); err != nil {
			return nil, 0, fmt.Errorf("scan stats row: %w", err)
		}

		errRate := 0.0
		if row.Total > 0 {
			errRate = float64(row.Error) / float64(row.Total)
		}
		row.Health = domain.ComputeHealth(row.DLRRate, latencyP95, errRate, int(row.Pending))
		result = append(result, row)

		totalTotal += row.Total
		totalDelivered += row.Delivered
		totalFailed += row.Failed
		totalPending += row.Pending
		totalTimeout += row.Timeout
		totalError += row.Error
		totalRevenue += row.Revenue
		totalCost += row.Cost
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration: %w", err)
	}

	return result, totalRows, nil
}

// computeKPIs builds KPI cards from aggregated totals.
func computeKPIs(total, delivered, failed int64, revenue, cost float64) []domain.KPI {
	dlrRate := 0.0
	if total > 0 {
		dlrRate = float64(delivered) / float64(total)
	}
	profit := revenue - cost

	dlrStatus := domain.HealthOK
	if dlrRate < domain.DLRRateDanger {
		dlrStatus = domain.HealthDanger
	} else if dlrRate < domain.DLRRateWarning {
		dlrStatus = domain.HealthWarning
	}

	return []domain.KPI{
		{Name: "Всего", Value: float64(total)},
		{Name: "Доставлено", Value: float64(delivered)},
		{Name: "Доставляемость", Value: dlrRate, Status: dlrStatus},
		{Name: "Ошибки", Value: float64(failed)},
		{Name: "Прибыль", Value: profit},
	}
}

// GetAnalyticsSummary returns statistics with trends and signals.
func (r *StatsRepo) GetAnalyticsSummary(ctx context.Context, filter *domain.SharedFilter) (*domain.AnalyticsResult, error) {
	filter.Normalize()

	// Query current period rows
	rows, totalRows, err := r.GetStatistics(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Aggregate totals from rows
	var total, delivered, failed int64
	var revenue, cost float64
	for _, row := range rows {
		total += row.Total
		delivered += row.Delivered
		failed += row.Failed
		revenue += row.Revenue
		cost += row.Cost
	}
	kpis := computeKPIs(total, delivered, failed, revenue, cost)

	// Build previous period filter (same duration shifted back)
	duration := filter.DateTo.Sub(filter.DateFrom)
	prevFilter := *filter
	prevFilter.DateFrom = filter.DateFrom.Add(-duration)
	prevFilter.DateTo = filter.DateFrom
	prevFilter.Page = 1
	prevFilter.PageSize = domain.MaxPageSize

	prevRows, _, err := r.GetStatistics(ctx, &prevFilter)
	if err != nil {
		return nil, err
	}

	var prevTotal, prevDelivered, prevFailed int64
	var prevRevenue, prevCost float64
	for _, row := range prevRows {
		prevTotal += row.Total
		prevDelivered += row.Delivered
		prevFailed += row.Failed
		prevRevenue += row.Revenue
		prevCost += row.Cost
	}
	prevKPIs := computeKPIs(prevTotal, prevDelivered, prevFailed, prevRevenue, prevCost)

	// Compute deltas into current KPIs
	for i := range kpis {
		if i < len(prevKPIs) && prevKPIs[i].Value != 0 {
			kpis[i].Delta = (kpis[i].Value - prevKPIs[i].Value) / prevKPIs[i].Value * 100
		}
	}

	// Compute trends from time-series
	trends, err := r.computeTrends(ctx, filter)
	if err != nil {
		// Non-fatal: trends are supplementary
		trends = nil
	}

	// Generate signals
	signals := generateSignals(kpis, prevKPIs)

	pages := totalRows / filter.PageSize
	if totalRows%filter.PageSize != 0 {
		pages++
	}

	return &domain.AnalyticsResult{
		KPIs:         kpis,
		PreviousKPIs: prevKPIs,
		Trends:       trends,
		Signals:      signals,
		Rows:         rows,
		Pagination: domain.Pagination{
			Page:       filter.Page,
			PageSize:   filter.PageSize,
			TotalRows:  totalRows,
			TotalPages: pages,
		},
	}, nil
}

// computeTrends queries time-series data for trend charts.
func (r *StatsRepo) computeTrends(ctx context.Context, filter *domain.SharedFilter) ([]domain.Trend, error) {
	query := `
		SELECT
			date_trunc('hour', hour) AS ts,
			SUM(delivered)::float / NULLIF(SUM(total), 0) AS dlr_rate,
			COALESCE(SUM(revenue), 0) - COALESCE(SUM(cost), 0) AS profit
		FROM network_stats_hourly
		WHERE partner_id = $1 AND hour >= $2 AND hour <= $3
		GROUP BY ts
		ORDER BY ts ASC`

	rows, err := r.db.Query(ctx, query, filter.PartnerID, filter.DateFrom, filter.DateTo)
	if err != nil {
		return nil, fmt.Errorf("query trends: %w", err)
	}
	defer rows.Close()

	var dlrPoints, profitPoints []domain.MetricPoint
	for rows.Next() {
		var ts time.Time
		var dlrRate, profit float64
		if err := rows.Scan(&ts, &dlrRate, &profit); err != nil {
			return nil, fmt.Errorf("scan trend row: %w", err)
		}
		ms := ts.UnixMilli()
		dlrPoints = append(dlrPoints, domain.MetricPoint{Timestamp: ms, Value: dlrRate * 100})
		profitPoints = append(profitPoints, domain.MetricPoint{Timestamp: ms, Value: profit})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return []domain.Trend{
		{Metric: "dlr_rate", Points: dlrPoints},
		{Metric: "profit", Points: profitPoints},
	}, nil
}

// generateSignals builds actionable signals from KPI comparison.
func generateSignals(current, previous []domain.KPI) []domain.Signal {
	var signals []domain.Signal

	// Map KPIs by name
	prevMap := make(map[string]float64, len(previous))
	for _, k := range previous {
		prevMap[k.Name] = k.Value
	}

	for _, k := range current {
		prev, hasPrev := prevMap[k.Name]
		if !hasPrev || prev == 0 {
			continue
		}
		changePct := (k.Value - prev) / prev * 100

		switch k.Name {
		case "Доставляемость":
			if changePct < -5 {
				signals = append(signals, domain.Signal{
					Severity:  "danger",
					Text:      fmt.Sprintf("Доставляемость упала на %.1f%%", -changePct),
					LinkType:  "filter",
					LinkValue: "dlr_rate",
				})
			}
		case "Прибыль":
			if changePct < -10 {
				signals = append(signals, domain.Signal{
					Severity:  "warning",
					Text:      fmt.Sprintf("Маржинальность снизилась на %.1f%%", -changePct),
					LinkType:  "filter",
					LinkValue: "margin",
				})
			} else if changePct > 10 {
				signals = append(signals, domain.Signal{
					Severity:  "success",
					Text:      fmt.Sprintf("Прибыль выросла на %.1f%%", changePct),
					LinkType:  "filter",
					LinkValue: "profit",
				})
			}
		}
	}

	return signals
}

// GetDrillDown returns detailed breakdown for a specific slice.
func (r *StatsRepo) GetDrillDown(ctx context.Context, params *domain.DrillDownParams) (*domain.DrillDownResult, error) {
	if params.Filter == nil {
		params.Filter = &domain.SharedFilter{PartnerID: params.PartnerID}
	}
	params.Filter.PartnerID = params.PartnerID
	params.Filter.Normalize()

	// Determine child dimension based on detail view
	childDim := "operator"
	switch params.DetailView {
	case "statuses":
		// handled separately below: aggregate table stores statuses as
		// counter columns, not rows, so we unpivot them into synthetic rows.
	case "errors":
		childDim = "error"
	case "timeline":
		childDim = "hour"
	case "money":
		// handled separately below: money view is a fixed 4-row breakdown
		// (Revenue / Cost / Profit / Margin), not a grouping by dimension.
	case "operators":
		childDim = "operator"
	}

	whereClause, args := buildWhereClause(params.Filter)

	// Append parent slice condition
	parentCol := sliceColumn(params.SliceType)
	idx := len(args) + 1
	whereClause += fmt.Sprintf(" AND %s = $%d", parentCol, idx)
	args = append(args, params.SliceValue)

	if params.DetailView == "statuses" {
		return r.drillDownStatuses(ctx, whereClause, args)
	}
	if params.DetailView == "money" {
		return r.drillDownMoney(ctx, whereClause, args)
	}

	col := sliceColumn(childDim)

	query := fmt.Sprintf(`
		SELECT
			COALESCE(%s, '') AS slice,
			SUM(total) AS total,
			SUM(sent) AS sent,
			SUM(delivered) AS delivered,
			SUM(failed) AS failed,
			SUM(pending) AS pending,
			SUM(timeout) AS timeout,
			SUM(error) AS error,
			CASE WHEN SUM(total) > 0 THEN SUM(delivered)::float / SUM(total)::float ELSE 0 END AS dlr_rate,
			COALESCE(SUM(revenue), 0) AS revenue,
			COALESCE(SUM(cost), 0) AS cost,
			COALESCE(SUM(revenue), 0) - COALESCE(SUM(cost), 0) AS profit,
			CASE WHEN COALESCE(SUM(revenue), 0) > 0
				THEN (COALESCE(SUM(revenue), 0) - COALESCE(SUM(cost), 0)) / SUM(revenue)
				ELSE 0 END AS margin,
			COALESCE(AVG(dlr_latency_p95), 0)::int AS dlr_latency_p95
		FROM network_stats_hourly
		%s
		GROUP BY %s
		ORDER BY total DESC
		LIMIT 100`, col, whereClause, col)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query drill-down: %w", err)
	}
	defer rows.Close()

	var statRows []domain.StatRow
	var totalTotal, totalDelivered, totalFailed int64
	var totalRevenue, totalCost float64

	for rows.Next() {
		var row domain.StatRow
		var latencyP95 int
		if err := rows.Scan(
			&row.Slice, &row.Total, &row.Sent, &row.Delivered,
			&row.Failed, &row.Pending, &row.Timeout, &row.Error,
			&row.DLRRate, &row.Revenue, &row.Cost, &row.Profit, &row.Margin,
			&latencyP95,
		); err != nil {
			return nil, fmt.Errorf("scan drill-down row: %w", err)
		}
		errRate := 0.0
		if row.Total > 0 {
			errRate = float64(row.Error) / float64(row.Total)
		}
		row.Health = domain.ComputeHealth(row.DLRRate, latencyP95, errRate, int(row.Pending))
		statRows = append(statRows, row)

		totalTotal += row.Total
		totalDelivered += row.Delivered
		totalFailed += row.Failed
		totalRevenue += row.Revenue
		totalCost += row.Cost
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	summary := computeKPIs(totalTotal, totalDelivered, totalFailed, totalRevenue, totalCost)

	overallHealth := domain.HealthOK
	dlrRate := 0.0
	if totalTotal > 0 {
		dlrRate = float64(totalDelivered) / float64(totalTotal)
	}
	errRate := 0.0
	if totalTotal > 0 {
		errRate = float64(totalFailed) / float64(totalTotal)
	}
	if dlrRate < domain.DLRRateDanger || errRate > domain.ErrorRateDanger {
		overallHealth = domain.HealthDanger
	} else if dlrRate < domain.DLRRateWarning || errRate > domain.ErrorRateWarning {
		overallHealth = domain.HealthWarning
	}

	return &domain.DrillDownResult{
		Summary: summary,
		Rows:    statRows,
		Trends:  nil,
		Health:  overallHealth,
	}, nil
}

// drillDownStatuses unpivots status counter columns into synthetic rows,
// one row per status bucket (sent/delivered/failed/pending/timeout/error).
// The aggregate table network_stats_hourly does not have a `status` column —
// counts are stored in separate columns — so grouping by a dimension here
// would yield a non-status slice value (e.g. channel name). Instead we
// build a fixed list of status rows from column sums.
func (r *StatsRepo) drillDownStatuses(ctx context.Context, whereClause string, args []interface{}) (*domain.DrillDownResult, error) {
	query := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(total), 0)     AS total,
			COALESCE(SUM(sent), 0)      AS sent,
			COALESCE(SUM(delivered), 0) AS delivered,
			COALESCE(SUM(failed), 0)    AS failed,
			COALESCE(SUM(pending), 0)   AS pending,
			COALESCE(SUM(timeout), 0)   AS timeout,
			COALESCE(SUM(error), 0)     AS error,
			COALESCE(SUM(revenue), 0)   AS revenue,
			COALESCE(SUM(cost), 0)      AS cost
		FROM network_stats_hourly
		%s`, whereClause)

	row := r.db.QueryRow(ctx, query, args...)
	var total, sent, delivered, failed, pending, timeoutCnt, errorCnt int64
	var revenue, cost float64
	if err := row.Scan(&total, &sent, &delivered, &failed, &pending, &timeoutCnt, &errorCnt, &revenue, &cost); err != nil {
		return nil, fmt.Errorf("scan drill-down statuses: %w", err)
	}

	buckets := []struct {
		name  string
		count int64
	}{
		{"sent", sent},
		{"delivered", delivered},
		{"failed", failed},
		{"pending", pending},
		{"timeout", timeoutCnt},
		{"error", errorCnt},
	}

	statRows := make([]domain.StatRow, 0, len(buckets))
	for _, b := range buckets {
		share := 0.0
		if total > 0 {
			share = float64(b.count) / float64(total)
		}
		statRows = append(statRows, domain.StatRow{
			Slice:   b.name,
			Total:   b.count,
			DLRRate: share, // share of total for this status bucket
			Health:  domain.HealthOK,
		})
	}

	summary := computeKPIs(total, delivered, failed, revenue, cost)

	overallHealth := domain.HealthOK
	dlrRate := 0.0
	if total > 0 {
		dlrRate = float64(delivered) / float64(total)
	}
	errRate := 0.0
	if total > 0 {
		errRate = float64(failed) / float64(total)
	}
	if dlrRate < domain.DLRRateDanger || errRate > domain.ErrorRateDanger {
		overallHealth = domain.HealthDanger
	} else if dlrRate < domain.DLRRateWarning || errRate > domain.ErrorRateWarning {
		overallHealth = domain.HealthWarning
	}

	return &domain.DrillDownResult{
		Summary: summary,
		Rows:    statRows,
		Trends:  nil,
		Health:  overallHealth,
	}, nil
}

// drillDownMoney returns a fixed 4-row financial breakdown:
// Выручка (revenue), Себестоимость (cost), Прибыль (profit = revenue - cost),
// Маржа (margin = profit / revenue). The aggregate table stores money as
// separate revenue/cost columns, not a "money dimension" — grouping by any
// real column (channel, sender) here would yield a single empty-slice row
// when filters have already narrowed the query to one value. Instead we
// unpivot money columns into synthetic rows, analogous to drillDownStatuses.
func (r *StatsRepo) drillDownMoney(ctx context.Context, whereClause string, args []interface{}) (*domain.DrillDownResult, error) {
	query := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(total), 0)     AS total,
			COALESCE(SUM(delivered), 0) AS delivered,
			COALESCE(SUM(failed), 0)    AS failed,
			COALESCE(SUM(revenue), 0)   AS revenue,
			COALESCE(SUM(cost), 0)      AS cost
		FROM network_stats_hourly
		%s`, whereClause)

	row := r.db.QueryRow(ctx, query, args...)
	var total, delivered, failed int64
	var revenue, cost float64
	if err := row.Scan(&total, &delivered, &failed, &revenue, &cost); err != nil {
		return nil, fmt.Errorf("scan drill-down money: %w", err)
	}

	profit := revenue - cost
	margin := 0.0
	if revenue > 0 {
		margin = profit / revenue
	}

	// Build 4 synthetic rows. We put the numeric value in both Total (for
	// the default "Total" column rendering) and the matching money field
	// (Revenue/Cost/Profit/Margin) so the frontend can pick whichever it
	// prefers without losing precision on fractional currency.
	statRows := []domain.StatRow{
		{Slice: "Выручка", Total: int64(revenue), Revenue: revenue, Health: domain.HealthOK},
		{Slice: "Себестоимость", Total: int64(cost), Cost: cost, Health: domain.HealthOK},
		{Slice: "Прибыль", Total: int64(profit), Profit: profit, Health: domain.HealthOK},
		{Slice: "Маржа", Total: int64(margin * 100), Margin: margin, DLRRate: margin, Health: domain.HealthOK},
	}

	summary := computeKPIs(total, delivered, failed, revenue, cost)

	return &domain.DrillDownResult{
		Summary: summary,
		Rows:    statRows,
		Trends:  nil,
		Health:  domain.HealthOK,
	}, nil
}

// UpsertHourlyStats inserts or updates hourly aggregated rows.
func (r *StatsRepo) UpsertHourlyStats(ctx context.Context, rows []domain.HourlyStatsRow) error {
	if len(rows) == 0 {
		return nil
	}

	query := `
		INSERT INTO network_stats_hourly (
			partner_id, hour, provider_id, operator, country, channel,
			login, sender_name, traffic_type, method,
			total, sent, delivered, failed, pending, timeout, error,
			revenue, cost,
			dlr_latency_sum, dlr_latency_cnt, dlr_latency_p50, dlr_latency_p95,
			throughput_max
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17,
			$18, $19,
			$20, $21, $22, $23,
			$24
		)
		ON CONFLICT (partner_id, hour, provider_id, operator, country, channel, login, sender_name, traffic_type, method)
		DO UPDATE SET
			total          = network_stats_hourly.total + EXCLUDED.total,
			sent           = network_stats_hourly.sent + EXCLUDED.sent,
			delivered      = network_stats_hourly.delivered + EXCLUDED.delivered,
			failed         = network_stats_hourly.failed + EXCLUDED.failed,
			pending        = EXCLUDED.pending,
			timeout        = network_stats_hourly.timeout + EXCLUDED.timeout,
			error    = network_stats_hourly.error + EXCLUDED.error,
			revenue        = network_stats_hourly.revenue + EXCLUDED.revenue,
			cost           = network_stats_hourly.cost + EXCLUDED.cost,
			dlr_latency_sum = network_stats_hourly.dlr_latency_sum + EXCLUDED.dlr_latency_sum,
			dlr_latency_cnt = network_stats_hourly.dlr_latency_cnt + EXCLUDED.dlr_latency_cnt,
			dlr_latency_p50 = EXCLUDED.dlr_latency_p50,
			dlr_latency_p95 = EXCLUDED.dlr_latency_p95,
			throughput_max  = GREATEST(network_stats_hourly.throughput_max, EXCLUDED.throughput_max)`

	for _, row := range rows {
		_, err := r.db.Exec(ctx, query,
			row.PartnerID, row.Hour, row.ProviderID, row.Operator, row.Country, row.Channel,
			row.Login, row.SenderName, row.TrafficType, row.Method,
			row.Total, row.Sent, row.Delivered, row.Failed, row.Pending, row.Timeout, row.Error,
			row.Revenue, row.Cost,
			row.DLRLatencySum, row.DLRLatencyCnt, row.DLRLatencyP50, row.DLRLatencyP95,
			row.ThroughputMax,
		)
		if err != nil {
			return fmt.Errorf("upsert hourly stats (partner=%d, hour=%s): %w", row.PartnerID, row.Hour, err)
		}
	}
	return nil
}
