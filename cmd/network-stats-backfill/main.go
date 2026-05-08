// Command network-stats-backfill replays AggregationWorker.BackfillWindow over a
// historical time range to repair network_stats_hourly rows. Idempotent thanks to
// UpsertHourlyStats replace semantics (Task 5).
//
// Usage:
//
//	DATABASE_URL=postgres://... network-stats-backfill \
//	    --from 2026-04-01T00:00:00Z --to now [--truncate] [--yes]
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/network_analytics/application"
	"github.com/smpp-server/smpp-server/internal/services/network_analytics/infrastructure/repository"
)

func main() {
	var (
		fromStr  string
		toStr    string
		truncate bool
		yes      bool
	)

	flag.StringVar(&fromStr, "from", "", "Start of backfill window (RFC3339, required)")
	flag.StringVar(&toStr, "to", "", "End of backfill window (RFC3339 or 'now', required)")
	flag.BoolVar(&truncate, "truncate", false, "Delete existing network_stats_hourly rows in window before backfill")
	flag.BoolVar(&yes, "yes", false, "Skip stdin YES confirmation when --truncate is set")
	flag.Parse()

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	log.Logger = logger

	if fromStr == "" || toStr == "" {
		logger.Fatal().Msg("--from and --to are required")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		logger.Fatal().Msg("DATABASE_URL env var is required")
	}

	fromT, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		logger.Fatal().Err(err).Str("from", fromStr).Msg("invalid --from (expected RFC3339)")
	}

	var toT time.Time
	if strings.EqualFold(strings.TrimSpace(toStr), "now") {
		toT = time.Now().UTC()
	} else {
		toT, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			logger.Fatal().Err(err).Str("to", toStr).Msg("invalid --to (expected RFC3339 or 'now')")
		}
	}

	if !toT.After(fromT) {
		logger.Fatal().Time("from", fromT).Time("to", toT).Msg("--to must be after --from")
	}

	if truncate && !yes {
		fmt.Fprintf(os.Stderr,
			"About to TRUNCATE network_stats_hourly rows in [%s, %s).\nType YES to proceed: ",
			fromT.Format(time.RFC3339), toT.Format(time.RFC3339))
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		if strings.TrimRight(line, "\r\n") != "YES" {
			logger.Fatal().Msg("confirmation not received; aborting")
		}
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Fatal().Err(err).Msg("failed to ping database")
	}

	if truncate {
		logger.Info().
			Time("from", fromT).
			Time("to", toT).
			Msg("truncating network_stats_hourly in window")
		tag, err := pool.Exec(ctx,
			`DELETE FROM network_stats_hourly WHERE hour >= $1 AND hour < $2`,
			fromT, toT)
		if err != nil {
			logger.Fatal().Err(err).Msg("truncate failed")
		}
		logger.Info().Int64("rows_deleted", tag.RowsAffected()).Msg("truncate complete")
	}

	statsRepo := repository.NewStatsRepo(pool)
	monRepo := repository.NewMonitoringRepo(pool, nil)
	worker := application.NewAggregationWorker(pool, statsRepo, monRepo, logger)

	logger.Info().
		Time("from", fromT).
		Time("to", toT).
		Bool("truncate", truncate).
		Msg("starting BackfillWindow")

	start := time.Now()
	if err := worker.BackfillWindow(ctx, fromT, toT); err != nil {
		logger.Fatal().Err(err).Msg("BackfillWindow failed")
	}
	elapsed := time.Since(start)

	logger.Info().
		Time("from", fromT).
		Time("to", toT).
		Dur("elapsed", elapsed).
		Msg("backfill complete")

	fmt.Fprintf(os.Stdout, "Backfill complete: window=[%s, %s) elapsed=%s\n",
		fromT.Format(time.RFC3339), toT.Format(time.RFC3339), elapsed)
}
