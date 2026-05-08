package application_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/network_analytics/application"
	"github.com/smpp-server/smpp-server/internal/services/network_analytics/infrastructure/repository"
)

// TestAggregationWorker_BackfillJoinsTarificationLog verifies that the raw
// aggregation query JOINs tarification_log on message_id and writes
// SUM(tarification_log.total_amount) into network_stats_hourly.revenue.
//
// Cost stays at 0 (Slice 2 will JOIN provider_tarification_log).
//
// Requires TEST_DB_DSN pointing at a sandbox Postgres with all migrations
// applied. Skips silently when the variable is unset so pre-commit runs
// pass on developer machines without a live DB.
func TestAggregationWorker_BackfillJoinsTarificationLog(t *testing.T) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN not set")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	// Pick a historical hour inside an existing partition (2026-03-15 10:00 UTC).
	hour := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	// Use a partner_id that is unlikely to collide with the sequence-generated
	// IDs of pre-existing clients on the sandbox. The unique index on
	// clients.partner_id will reject the insert if it does.
	const partnerID int64 = 999

	clientID := uuid.New()
	countryID := uuid.New()
	operatorID := uuid.New()
	tariffPlanID := uuid.New()
	tariffPeriodID := uuid.New()

	suffix := uuid.NewString()[:8]

	// Cleanup runs in reverse order of FK dependencies. Best-effort: even
	// if a single statement fails (e.g. row missing) we keep cleaning.
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM network_stats_hourly WHERE partner_id = $1 AND hour = $2`, partnerID, hour)
		_, _ = pool.Exec(ctx, `DELETE FROM tarification_log WHERE client_id = $1`, clientID)
		_, _ = pool.Exec(ctx, `DELETE FROM messages WHERE client_id = $1`, clientID)
		_, _ = pool.Exec(ctx, `DELETE FROM tariff_periods WHERE id = $1`, tariffPeriodID)
		_, _ = pool.Exec(ctx, `DELETE FROM tariff_plans WHERE id = $1`, tariffPlanID)
		_, _ = pool.Exec(ctx, `DELETE FROM clients WHERE id = $1`, clientID)
		_, _ = pool.Exec(ctx, `DELETE FROM operators WHERE id = $1`, operatorID)
		_, _ = pool.Exec(ctx, `DELETE FROM countries WHERE id = $1`, countryID)
	}()

	// Country.
	_, err = pool.Exec(ctx, `
		INSERT INTO countries (id, name, iso_code, phone_code, currency)
		VALUES ($1, $2, $3, $4, $5)`,
		countryID, "Test-"+suffix, "T"+suffix[:1], "9"+suffix[:2], "RUB",
	)
	require.NoError(t, err)

	// Operator.
	_, err = pool.Exec(ctx, `
		INSERT INTO operators (id, country_id, name, code)
		VALUES ($1, $2, $3, $4)`,
		operatorID, countryID, "Op-"+suffix, "OP-"+suffix,
	)
	require.NoError(t, err)

	// Client with explicit partner_id=999.
	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, partner_id)
		VALUES ($1, $2, $3, 'secret', $4, true, $5)`,
		clientID,
		"agg-test-"+suffix,
		"apikey-"+suffix,
		"agg-"+suffix+"@t.local",
		partnerID,
	)
	require.NoError(t, err)

	// Tariff plan + period (minimal, just to satisfy NOT NULL on tarification_log).
	_, err = pool.Exec(ctx, `
		INSERT INTO tariff_plans (id, operator_id, sender_category, strategy, active)
		VALUES ($1, $2, 'shared', 'fixed', true)`,
		tariffPlanID, operatorID,
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO tariff_periods (id, tariff_plan_id, start_date, end_date)
		VALUES ($1, $2, '2026-01-01', '2027-01-01')`,
		tariffPeriodID, tariffPlanID,
	)
	require.NoError(t, err)

	// 5 delivered messages + 1 tarification_log row each at 1.5 RUB.
	for i := 0; i < 5; i++ {
		messageID := uuid.New()
		createdAt := hour.Add(time.Duration(i) * time.Minute)

		_, err = pool.Exec(ctx, `
			INSERT INTO messages (
				id, source, destination, text, status,
				client_id, operator_id, country_id, channel, send_method,
				service_type, created_at, updated_at
			) VALUES (
				$1, 'sender', '+79991234567', 'test', 'delivered',
				$2, $3, $4, 'sms', 'API',
				'transactional', $5, $5
			)`,
			messageID, clientID, operatorID, countryID, createdAt,
		)
		require.NoError(t, err)

		_, err = pool.Exec(ctx, `
			INSERT INTO tarification_log (
				client_id, message_id, operator_id,
				sender_category, strategy,
				tariff_plan_id, tariff_period_id,
				segment_count, price_per_segment, total_amount,
				idempotency_key, created_at
			) VALUES (
				$1, $2, $3,
				'shared', 'fixed',
				$4, $5,
				1, 1.5, 1.5,
				$6, $7
			)`,
			clientID, messageID, operatorID,
			tariffPlanID, tariffPeriodID,
			fmt.Sprintf("idem-%s-%d", suffix, i),
			createdAt,
		)
		require.NoError(t, err)
	}

	// Run the worker against [hour, hour+1h).
	logger := zerolog.Nop()
	statsRepo := repository.NewStatsRepo(pool)
	worker := application.NewAggregationWorker(pool, statsRepo, nil, logger)

	require.NoError(t, worker.BackfillWindow(ctx, hour, hour.Add(time.Hour)))

	var revenue float64
	err = pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(revenue), 0)
		FROM network_stats_hourly
		WHERE partner_id = $1 AND hour = $2`,
		partnerID, hour,
	).Scan(&revenue)
	require.NoError(t, err)

	const want = 7.5
	const eps = 0.001
	if revenue < want-eps || revenue > want+eps {
		t.Fatalf("revenue: want %.4f RUB, got %.4f", want, revenue)
	}
}
