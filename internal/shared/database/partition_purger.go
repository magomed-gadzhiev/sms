package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/config"
)

// PartitionPurger drops PostgreSQL partitions older than a given retention threshold.
type PartitionPurger struct {
	logger zerolog.Logger
}

// NewPartitionPurger creates a new PartitionPurger instance.
func NewPartitionPurger() *PartitionPurger {
	return &PartitionPurger{
		logger: log.With().Str("component", "partition_purger").Logger(),
	}
}

// PurgeOldPartitions drops partitions of parentTable that are older than retentionDays.
// It queries pg_catalog for child tables with a YYYY_MM suffix, parses the date,
// and drops any partition whose month-end is older than the retention cutoff.
// Returns the list of dropped partition names.
func (p *PartitionPurger) PurgeOldPartitions(db *sql.DB, parentTable string, retentionDays int) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), config.DefaultPartitionPurgeInterval)
	defer cancel()

	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	// Query pg_catalog for child (partition) tables of the parent table.
	// pg_inherits links child to parent; we join pg_class + pg_namespace to get qualified names.
	query := `
		SELECT c.relname
		FROM pg_inherits i
		JOIN pg_class p ON p.oid = i.inhparent
		JOIN pg_class c ON c.oid = i.inhrelid
		JOIN pg_namespace n ON n.oid = p.relnamespace
		WHERE p.relname = $1
		ORDER BY c.relname
	`

	rows, err := db.QueryContext(ctx, query, parentTable)
	if err != nil {
		return nil, fmt.Errorf("failed to query partitions for %s: %w", parentTable, err)
	}
	defer rows.Close()

	var dropped []string

	for rows.Next() {
		var childName string
		if err := rows.Scan(&childName); err != nil {
			return dropped, fmt.Errorf("failed to scan partition name: %w", err)
		}

		partDate, ok := parsePartitionSuffix(childName, parentTable)
		if !ok {
			p.logger.Debug().
				Str("partition", childName).
				Msg("skipping partition with unparseable date suffix")
			continue
		}

		// The partition covers the entire month; consider it expired only if
		// the last day of that month is before the cutoff date.
		endOfMonth := partDate.AddDate(0, 1, -1)
		if endOfMonth.Before(cutoff) {
			dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS %s", childName)
			if _, err := db.ExecContext(ctx, dropSQL); err != nil {
				p.logger.Error().
					Err(err).
					Str("partition", childName).
					Msg("failed to drop partition")
				continue
			}

			p.logger.Info().
				Str("partition", childName).
				Str("parent_table", parentTable).
				Time("partition_date", partDate).
				Time("cutoff", cutoff).
				Msg("dropped old partition")

			dropped = append(dropped, childName)
		}
	}

	if err := rows.Err(); err != nil {
		return dropped, fmt.Errorf("error iterating partitions: %w", err)
	}

	return dropped, nil
}

// parsePartitionSuffix extracts a YYYY_MM date from a partition name like "messages_2025_01".
// It strips the parentTable prefix and underscore, then parses the remaining YYYY_MM suffix.
func parsePartitionSuffix(childName, parentTable string) (time.Time, bool) {
	prefix := parentTable + "_"
	if !strings.HasPrefix(childName, prefix) {
		return time.Time{}, false
	}

	suffix := childName[len(prefix):]
	t, err := time.Parse("2006_01", suffix)
	if err != nil {
		return time.Time{}, false
	}

	return t, true
}
