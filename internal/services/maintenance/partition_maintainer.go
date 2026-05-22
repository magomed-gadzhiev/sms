package maintenance

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// validIdentifier — разрешённый формат table-name'а: lowercase + digits + _,
// начинается с буквы/_. Защита от SQL-injection через interpolated DDL
// (tableName идёт в fmt.Sprintf без quoting'а; pgx.Identifier{}.Sanitize()
// не используется потому что оборачивает в "..." и ломает имена партиций
// производные от него).
var validIdentifier = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// PartitionNaming описывает суффикс-схему имени монтьной партиции.
//
// В текущей кодовой базе используются ДВА несовместимых формата
// (исторический drift между миграциями):
//
//   - NamingYMM:    `<table>_yYYYYmMM`  — audit_log, messages
//     (миграции 000001, 000029, и др.)
//   - NamingYYYYMM: `<table>_YYYY_MM`   — lookup_log, deliveries, delivery_attempts
//     (миграции lookup_log, deliveries.sql)
//
// Задача Plan 7 Task 6 — поддержать обе, не ломая существующие партиции.
// Нормализация имён в один формат — отдельная задача (рисковый rename).
type PartitionNaming int

const (
	// NamingYMM — формат `<table>_yYYYYmMM`, например `audit_log_y2026m05`.
	NamingYMM PartitionNaming = iota
	// NamingYYYYMM — формат `<table>_YYYY_MM`, например `lookup_log_2026_05`.
	NamingYYYYMM
)

// PartitionedTable описывает одну partitioned-таблицу с её naming-схемой.
type PartitionedTable struct {
	Name   string
	Naming PartitionNaming
}

func formatPartitionName(table string, naming PartitionNaming, t time.Time) string {
	switch naming {
	case NamingYYYYMM:
		return fmt.Sprintf("%s_%d_%02d", table, t.Year(), int(t.Month()))
	case NamingYMM:
		fallthrough
	default:
		return fmt.Sprintf("%s_y%dm%02d", table, t.Year(), int(t.Month()))
	}
}

// EnsureFuturePartitions гарантирует, что для partitioned-таблицы tableName
// существуют монтьные партиции от месяца now до now+forwardMonths включительно
// (всего forwardMonths+1 партиций; forwardMonths=6 → 7 партиций: текущий + 6).
// Idempotent: использует CREATE TABLE IF NOT EXISTS PARTITION OF.
//
// Применимо к таблицам с RANGE partitioning по timestamp-колонке (created_at)
// с месячным шагом. naming определяет суффикс имени партиции.
//
// Caveats:
//   - Errors из pool.Exec возвращаются caller'у; в RunPartitionMaintenanceLoop
//     они логируются и swallow'ятся, retry на следующем тике (24h).
//   - "IF NOT EXISTS PARTITION OF" idempotent ТОЛЬКО по имени. Если партиция
//     с тем же range, но другим именем (например, наследие старого naming)
//     уже существует — PG вернёт ошибку "partition would overlap". Это
//     desired: вызывает alert, ops видит drift, не silent corruption.
//   - tableName ВАЛИДИРУЕТСЯ против [a-z_][a-z0-9_]* — interpolated DDL без
//     этого = SQL-injection vector. Любой каллер, использующий пользовательский
//     ввод для tableName, обязан понимать ограничение.
//
// Plan 7 Task 6.
func EnsureFuturePartitions(ctx context.Context, pool *pgxpool.Pool, tableName string, naming PartitionNaming, now time.Time, forwardMonths int) error {
	if forwardMonths <= 0 {
		return fmt.Errorf("forwardMonths must be positive, got %d", forwardMonths)
	}
	if !validIdentifier.MatchString(tableName) {
		return fmt.Errorf("invalid tableName %q: must match %s", tableName, validIdentifier.String())
	}
	for i := 0; i <= forwardMonths; i++ {
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, i, 0)
		monthEnd := monthStart.AddDate(0, 1, 0)
		partName := formatPartitionName(tableName, naming, monthStart)
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
// таблицы из tables раз в interval (рекомендуется 24h). Первый прогон —
// сразу при старте. graceful exit при ctx.Done.
//
// Plan 7 Task 6.
func RunPartitionMaintenanceLoop(ctx context.Context, pool *pgxpool.Pool, tables []PartitionedTable, forwardMonths int, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()

	tableNames := make([]string, 0, len(tables))
	for _, tbl := range tables {
		tableNames = append(tableNames, tbl.Name)
	}

	runOnce := func() {
		for _, tbl := range tables {
			if err := EnsureFuturePartitions(ctx, pool, tbl.Name, tbl.Naming, time.Now().UTC(), forwardMonths); err != nil {
				log.Error().Err(err).Str("table", tbl.Name).Msg("partition maintenance failed")
			} else {
				log.Info().Str("table", tbl.Name).Int("forward_months", forwardMonths).
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
