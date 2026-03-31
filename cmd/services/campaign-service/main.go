package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	campaignv1 "github.com/smpp-server/smpp-server/api/proto/campaignv1"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/pipeline"
	"github.com/smpp-server/smpp-server/internal/services/campaign/application"
	campaigngrpc "github.com/smpp-server/smpp-server/internal/services/campaign/grpc"
	campaignrepo "github.com/smpp-server/smpp-server/internal/services/campaign/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/database"
	"github.com/smpp-server/smpp-server/internal/storage"
)

func main() {
	shared.InitLogger("development")
	logger := shared.WithService("campaign-service")

	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	logger.Info().
		Str("version", cfg.Service.Version).
		Str("env", cfg.Service.Env).
		Msg("запуск Campaign Service")

	// Wait for database
	logger.Info().Msg("ожидание готовности базы данных")
	if err := storage.WaitForDatabase(cfg.Database.GetDSN(), 30, 2*time.Second); err != nil {
		logger.Fatal().Err(err).Msg("база данных недоступна")
	}

	dbConn, err := database.NewDBWithConfig(database.Config{
		DSN:             cfg.Database.GetDSN(),
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Database.ConnMaxIdleTime,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка подключения к базе данных")
	}
	defer dbConn.Close()
	logger.Info().Msg("подключение к базе данных установлено")

	dbx := sqlx.NewDb(dbConn.DB, "pgx")

	// --- Kafka producer (best-effort) ---
	var kafkaProducer sarama.SyncProducer
	if len(cfg.Kafka.Brokers) > 0 {
		saramaConfig := sarama.NewConfig()
		saramaConfig.Producer.Return.Successes = true
		saramaConfig.Producer.Return.Errors = true
		saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
		saramaConfig.Producer.Retry.Max = cfg.Kafka.MaxRetries
		saramaConfig.Producer.Retry.Backoff = cfg.Kafka.RetryBackoff
		saramaConfig.Producer.Compression = sarama.CompressionSnappy
		saramaConfig.Net.MaxOpenRequests = 1

		kafkaProducer, err = sarama.NewSyncProducer(cfg.Kafka.Brokers, saramaConfig)
		if err != nil {
			logger.Warn().Err(err).Msg("не удалось создать Kafka producer, продолжаем без Kafka")
		} else {
			logger.Info().Msg("Kafka producer создан")
			defer kafkaProducer.Close()
		}
	}

	// --- Kafka consumer (best-effort) ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if len(cfg.Kafka.Brokers) > 0 && kafkaProducer != nil {
		saramaConsumerConfig := sarama.NewConfig()
		saramaConsumerConfig.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
		saramaConsumerConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
		saramaConsumerConfig.Consumer.Return.Errors = true
		saramaConsumerConfig.Version = sarama.V2_6_0_0

		consumerGroup, cgErr := sarama.NewConsumerGroup(cfg.Kafka.Brokers, "campaign-status-consumer", saramaConsumerConfig)
		if cgErr != nil {
			logger.Warn().Err(cgErr).Msg("не удалось создать Kafka consumer group, продолжаем без потребления статусов")
		} else {
			logger.Info().Msg("Kafka consumer group 'campaign-status-consumer' создан")
			defer consumerGroup.Close()

			// Start consuming sms.status in background
			go func() {
				statusTopic := cfg.Kafka.TopicStatus
				if statusTopic == "" {
					statusTopic = "sms.status"
				}
				for {
					select {
					case <-ctx.Done():
						return
					default:
						handler := &statusConsumerHandler{dbx: dbx, logger: logger}
						if err := consumerGroup.Consume(ctx, []string{statusTopic}, handler); err != nil {
							logger.Error().Err(err).Msg("ошибка потребления сообщений из Kafka")
						}
					}
				}
			}()
		}
	}

	// Repositories
	campaignRepo := campaignrepo.NewCampaignRepository(dbx)
	recipientRepo := campaignrepo.NewRecipientRepository(dbx)
	statsRepo := campaignrepo.NewStatsRepository(dbx)

	// Application service
	campaignService := application.NewCampaignService(campaignRepo, recipientRepo, statsRepo)

	// Materialization worker
	if kafkaProducer != nil {
		startMaterializationWorker(ctx, dbx, kafkaProducer, cfg.Kafka.TopicOutgoing, recipientRepo, logger)
		logger.Info().Msg("воркер материализации кампаний запущен")
	} else {
		logger.Warn().Msg("Kafka недоступна, воркер материализации отключён")
	}

	// Reconciler: syncs campaign_recipients status from messages table
	// and closes fully-processed running campaigns.
	// Runs regardless of Kafka availability — uses only PostgreSQL.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := reconcileRunningCampaigns(ctx, dbx, logger); err != nil {
					logger.Error().Err(err).Msg("ошибка reconcile running campaigns")
				}
			}
		}
	}()

	// Health checker
	healthChecker := monitoring.NewHealthChecker("campaign-service", cfg.Service.Version)
	healthChecker.SetDatabase(dbConn.DB)

	// gRPC server
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.API.GRPC.MaxRecv),
		grpc.MaxSendMsgSize(cfg.API.GRPC.MaxSend),
	)

	campaignGrpcServer := campaigngrpc.NewServer(campaignService)
	campaignv1.RegisterCampaignServiceServer(grpcServer, campaignGrpcServer)

	if cfg.Service.Env == "development" {
		reflection.Register(grpcServer)
		logger.Info().Msg("gRPC reflection включен")
	}

	grpcListener, err := net.Listen("tcp", ":5013")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания gRPC listener")
	}

	go func() {
		logger.Info().Str("addr", grpcListener.Addr().String()).Msg("gRPC сервер запущен")
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.Fatal().Err(err).Msg("ошибка запуска gRPC сервера")
		}
	}()

	// Metrics HTTP server
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/health", healthChecker.Handler())
	metricsMux.HandleFunc("/health/live", healthChecker.LivenessHandler())
	metricsMux.HandleFunc("/health/ready", healthChecker.ReadinessHandler())

	if cfg.Monitoring.Prometheus.Enabled {
		metricsMux.Handle(cfg.Monitoring.Prometheus.Path, promhttp.Handler())
		logger.Info().Str("path", cfg.Monitoring.Prometheus.Path).Msg("Prometheus metrics endpoint включен")
	}

	metricsServer := &http.Server{
		Addr:         ":2131",
		Handler:      metricsMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		logger.Info().Str("addr", metricsServer.Addr).Msg("HTTP сервер для метрик запущен")
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("ошибка запуска HTTP сервера")
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info().Msg("получен сигнал завершения, остановка сервиса")

	cancel() // cancel background goroutines

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	grpcServer.GracefulStop()
	logger.Info().Msg("gRPC сервер остановлен")

	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("ошибка при остановке HTTP сервера")
	}
	logger.Info().Msg("HTTP сервер остановлен")

	logger.Info().Msg("Campaign Service остановлен")
}

// statusConsumerHandler implements sarama.ConsumerGroupHandler for processing delivery status updates.
type statusConsumerHandler struct {
	dbx    *sqlx.DB
	logger zerolog.Logger
}

func (h *statusConsumerHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *statusConsumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
func (h *statusConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		h.processStatusMessage(msg)
		session.MarkMessage(msg, "")
	}
	return nil
}

func (h *statusConsumerHandler) processStatusMessage(msg *sarama.ConsumerMessage) {
	var status pipeline.StatusUpdate
	if err := json.Unmarshal(msg.Value, &status); err != nil {
		h.logger.Warn().Err(err).Msg("не удалось десериализовать StatusUpdate")
		return
	}

	// Map pipeline status to recipient status
	recipientStatus := mapToRecipientStatus(status.Status)
	if recipientStatus == "" {
		return
	}

	// Find and update recipient by message_id
	var recipientID, campaignID string
	err := h.dbx.QueryRow(
		`UPDATE campaign_recipients SET status = $1, updated_at = now()
		 WHERE message_id = $2 AND status NOT IN ('delivered', 'failed', 'cancelled')
		 RETURNING id, campaign_id`,
		recipientStatus, status.MessageID,
	).Scan(&recipientID, &campaignID)
	if err != nil {
		// Not a campaign message or already in final status — skip
		return
	}

	// Update campaign counters
	counterCol := statusToCounterColumn(recipientStatus)
	if counterCol != "" {
		_, err = h.dbx.Exec(
			fmt.Sprintf(`UPDATE campaigns SET %s = %s + 1, updated_at = now() WHERE id = $1`, counterCol, counterCol),
			campaignID,
		)
		if err != nil {
			h.logger.Error().Err(err).Str("campaign_id", campaignID).Msg("ошибка обновления счётчика кампании")
		}
	}

	// Check if campaign is complete (all recipients processed)
	var pending int
	if err := h.dbx.QueryRow(
		`SELECT COUNT(*) FROM campaign_recipients WHERE campaign_id = $1 AND status IN ('pending', 'sent')`,
		campaignID,
	).Scan(&pending); err == nil && pending == 0 {
		_, _ = h.dbx.Exec(
			`UPDATE campaigns SET status = 'completed', completed_at = now(), updated_at = now()
			 WHERE id = $1 AND status = 'running'`,
			campaignID,
		)
		h.logger.Info().Str("campaign_id", campaignID).Msg("кампания завершена")
	}
}

func mapToRecipientStatus(pipelineStatus string) string {
	switch pipelineStatus {
	case "sent", "accepted":
		return "sent"
	case "delivered":
		return "delivered"
	case "failed", "rejected", "expired", "undeliverable":
		return "failed"
	default:
		return ""
	}
}

func statusToCounterColumn(recipientStatus string) string {
	switch recipientStatus {
	case "sent":
		return "sent_count"
	case "delivered":
		return "delivered_count"
	case "failed":
		return "failed_count"
	default:
		return ""
	}
}

// isCampaignComplete returns true when there are no recipients in a non-terminal
// state (pending or sent). Extracted as a pure function for testability.
func isCampaignComplete(counts map[string]int) bool {
	return counts["pending"] == 0 && counts["sent"] == 0
}

// reconcileRunningCampaigns fixes campaigns whose campaign_recipients.status
// has diverged from messages.status (e.g. after a service restart that missed
// sms.status Kafka events).
//
// It runs three SQL steps:
//  1. Sync recipient status from messages table.
//  2. Recount per-campaign status totals and update counters.
//  3. Mark campaigns with no pending/sent recipients as completed.
func reconcileRunningCampaigns(ctx context.Context, dbx *sqlx.DB, logger zerolog.Logger) error {
	// Step 1: sync campaign_recipients.status from messages for running campaigns.
	res, err := dbx.ExecContext(ctx, `
		UPDATE campaign_recipients cr
		SET    status     = CASE m.status
		                        WHEN 'delivered' THEN 'delivered'
		                        ELSE                 'failed'
		                    END,
		       updated_at = now()
		FROM   messages m
		WHERE  m.id = cr.message_id
		  AND  m.status IN ('delivered', 'failed', 'expired', 'rejected', 'undeliverable')
		  AND  cr.status IN ('pending', 'sent')
		  AND  cr.campaign_id IN (
		           SELECT id FROM campaigns WHERE status = 'running'
		       )
	`)
	if err != nil {
		return fmt.Errorf("reconcile sync recipients: %w", err)
	}
	synced, _ := res.RowsAffected()
	if synced == 0 {
		return nil // nothing to do
	}
	logger.Info().Int64("synced", synced).Msg("reconcile: обновлены статусы получателей")

	// Step 2: recount counters for affected campaigns and check completion.
	rows, err := dbx.QueryContext(ctx, `
		SELECT campaign_id, status, COUNT(*) as cnt
		FROM   campaign_recipients
		WHERE  campaign_id IN (SELECT id FROM campaigns WHERE status = 'running')
		GROUP BY campaign_id, status
	`)
	if err != nil {
		return fmt.Errorf("reconcile count recipients: %w", err)
	}
	defer rows.Close()

	// Aggregate counts per campaign.
	type campaignCounts struct {
		counts map[string]int
	}
	campaignMap := map[string]*campaignCounts{}
	for rows.Next() {
		var campaignID, status string
		var cnt int
		if err := rows.Scan(&campaignID, &status, &cnt); err != nil {
			continue
		}
		if campaignMap[campaignID] == nil {
			campaignMap[campaignID] = &campaignCounts{counts: map[string]int{}}
		}
		campaignMap[campaignID].counts[status] = cnt
	}
	rows.Close()

	for campaignID, cc := range campaignMap {
		// Update counters.
		_, err := dbx.ExecContext(ctx, `
			UPDATE campaigns SET
				sent_count      = $1,
				delivered_count = $2,
				failed_count    = $3,
				updated_at      = now()
			WHERE id = $4`,
			cc.counts["sent"],
			cc.counts["delivered"],
			cc.counts["failed"],
			campaignID,
		)
		if err != nil {
			logger.Error().Err(err).Str("campaign_id", campaignID).Msg("reconcile: ошибка обновления счётчиков")
			continue
		}

		// Step 3: mark complete if no pending/sent remain.
		if isCampaignComplete(cc.counts) {
			res, err := dbx.ExecContext(ctx, `
				UPDATE campaigns
				SET    status       = 'completed',
				       completed_at = now(),
				       updated_at   = now()
				WHERE  id     = $1
				  AND  status = 'running'`,
				campaignID,
			)
			if err != nil {
				logger.Error().Err(err).Str("campaign_id", campaignID).Msg("reconcile: ошибка завершения кампании")
				continue
			}
			if n, _ := res.RowsAffected(); n > 0 {
				logger.Info().Str("campaign_id", campaignID).Msg("reconcile: кампания завершена")
			}
		}
	}
	return nil
}
