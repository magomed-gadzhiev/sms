package maintenance

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// EnsureFuturePartitions гарантирует, что для partitioned-таблицы tableName
// существуют монтьные партиции от месяца now до now+forwardMonths включительно.
// Idempotent: использует CREATE TABLE IF NOT EXISTS PARTITION OF.
//
// Применимо к таблицам с RANGE partitioning по timestamp-колонке (created_at)
// с месячным шагом и naming `<table>_yYYYYmMM` — совместимо с существующими
// миграциями 000001/000029. Для таблиц с другой стратегией (HOURLY, by-status,
// etc.) функция не подходит.
//
// Plan 7 Task 6.
func EnsureFuturePartitions(ctx context.Context, pool *pgxpool.Pool, tableName string, now time.Time, forwardMonths int) error {
	if forwardMonths <= 0 {
		return fmt.Errorf("forwardMonths must be positive, got %d", forwardMonths)
	}
	for i := 0; i <= forwardMonths; i++ {
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, i, 0)
		monthEnd := monthStart.AddDate(0, 1, 0)
		partName := fmt.Sprintf("%s_y%dm%02d", tableName, monthStart.Year(), int(monthStart.Month()))
		ddl := fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')`,
			partName, tableName,
			monthStart.Format("2006-01-02"),
			monthEnd.Format("2006-01-02"),
		)
		if _, err := pool.Exec(ctx, ddl); err != nil {
			return fmt.Errorf("create partition %s: %w", partName, err)
		}
	}
	return nil
}

// RunPartitionMaintenanceLoop запускает EnsureFuturePartitions для каждой
// таблицы из tableNames раз в interval (рекомендуется 24h). Первый прогон —
// сразу при старте. graceful exit при ctx.Done.
//
// Plan 7 Task 6.
func RunPartitionMaintenanceLoop(ctx context.Context, pool *pgxpool.Pool, tableNames []string, forwardMonths int, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()

	runOnce := func() {
		for _, tbl := range tableNames {
			if err := EnsureFuturePartitions(ctx, pool, tbl, time.Now().UTC(), forwardMonths); err != nil {
				log.Error().Err(err).Str("table", tbl).Msg("partition maintenance failed")
			} else {
				log.Info().Str("table", tbl).Int("forward_months", forwardMonths).
					Msg("partition maintenance: ensured future partitions")
			}
		}
	}

	runOnce()
	log.Info().Strs("tables", tableNames).Dur("interval", interval).
		Msg("partition maintenance loop started")
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("partition maintenance loop stopped")
			return
		case <-t.C:
			runOnce()
		}
	}
}
