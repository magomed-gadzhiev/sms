package application

import (
	"context"
	"database/sql"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/shared/database"
)

// DataPurgerConfig holds configuration for the DataPurger.
type DataPurgerConfig struct {
	MessagesRetentionDays int
	AuditRetentionDays    int
	PurgeInterval         time.Duration
}

// DataPurger is a background service that periodically drops old partitions
// for the messages and audit_log tables.
type DataPurger struct {
	db     *sql.DB
	purger *database.PartitionPurger
	config DataPurgerConfig
	logger zerolog.Logger
	ctx    context.Context
	cancel context.CancelFunc
}

// NewDataPurger creates a new DataPurger instance.
func NewDataPurger(db *sql.DB, cfg DataPurgerConfig) *DataPurger {
	ctx, cancel := context.WithCancel(context.Background())
	return &DataPurger{
		db:     db,
		purger: database.NewPartitionPurger(),
		config: cfg,
		logger: log.With().Str("component", "data_purger").Logger(),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start launches the data purger goroutine.
func (d *DataPurger) Start() {
	d.logger.Info().
		Int("messages_retention_days", d.config.MessagesRetentionDays).
		Int("audit_retention_days", d.config.AuditRetentionDays).
		Dur("purge_interval", d.config.PurgeInterval).
		Msg("data purger started")

	go d.run()
}

// Stop signals the data purger to shut down.
func (d *DataPurger) Stop() {
	d.cancel()
	d.logger.Info().Msg("data purger stopped")
}

func (d *DataPurger) run() {
	// Run once immediately on startup, then on the ticker interval.
	d.purge()

	ticker := time.NewTicker(d.config.PurgeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.purge()
		}
	}
}

func (d *DataPurger) purge() {
	d.logger.Info().Msg("starting partition purge cycle")

	// Purge old message partitions
	dropped, err := d.purger.PurgeOldPartitions(d.db, "messages", d.config.MessagesRetentionDays)
	if err != nil {
		d.logger.Error().Err(err).Msg("error purging messages partitions")
	}
	if len(dropped) > 0 {
		d.logger.Info().
			Strs("partitions", dropped).
			Int("count", len(dropped)).
			Msg("purged old messages partitions")
	}

	// Purge old audit_log partitions
	dropped, err = d.purger.PurgeOldPartitions(d.db, "audit_log", d.config.AuditRetentionDays)
	if err != nil {
		d.logger.Error().Err(err).Msg("error purging audit_log partitions")
	}
	if len(dropped) > 0 {
		d.logger.Info().
			Strs("partitions", dropped).
			Int("count", len(dropped)).
			Msg("purged old audit_log partitions")
	}

	d.logger.Info().Msg("partition purge cycle complete")
}
