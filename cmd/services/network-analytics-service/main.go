package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"google.golang.org/grpc"

	networkanalyticsv1 "github.com/smpp-server/smpp-server/api/proto/networkanalyticsv1"
	"github.com/smpp-server/smpp-server/internal/services/network_analytics/application"
	networkgrpc "github.com/smpp-server/smpp-server/internal/services/network_analytics/grpc"
	"github.com/smpp-server/smpp-server/internal/services/network_analytics/infrastructure/repository"
)

func main() {
	// Logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Config
	viper.AutomaticEnv()
	viper.SetDefault("GRPC_PORT", "50060")
	viper.SetDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/sms?sslmode=disable")
	viper.SetDefault("REDIS_URL", "redis://localhost:6379/0")
	viper.SetDefault("EXPORT_DIR", "/exports")

	// PostgreSQL
	dbPool, err := pgxpool.New(context.Background(), viper.GetString("DATABASE_URL"))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
	}
	defer dbPool.Close()
	log.Info().Msg("Connected to PostgreSQL")

	// Redis
	redisOpts, err := redis.ParseURL(viper.GetString("REDIS_URL"))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse Redis URL")
	}
	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close()
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Warn().Err(err).Msg("Redis ping failed (continuing without real-time metrics)")
	} else {
		log.Info().Msg("Connected to Redis")
	}

	// Repositories
	statsRepo := repository.NewStatsRepo(dbPool)
	monitoringRepo := repository.NewMonitoringRepo(dbPool, redisClient)
	exportRepo := repository.NewExportRepo(dbPool)
	viewsRepo := repository.NewViewsRepo(dbPool)

	// Application service
	service := application.NewNetworkAnalyticsService(statsRepo, monitoringRepo, exportRepo, viewsRepo)

	// Aggregation workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker := application.NewAggregationWorker(dbPool, statsRepo, monitoringRepo, log.Logger)
	go worker.RunHourlyAggregation(ctx)
	go worker.RunSnapshotCollection(ctx)
	log.Info().Msg("Aggregation workers started")

	exportWorker := application.NewExportWorker(service, exportRepo, viper.GetString("EXPORT_DIR"), log.Logger)
	go exportWorker.Run(ctx)
	log.Info().Str("dir", viper.GetString("EXPORT_DIR")).Msg("Export worker started")

	// gRPC server
	grpcServer := grpc.NewServer()
	networkanalyticsv1.RegisterNetworkAnalyticsServiceServer(grpcServer, networkgrpc.NewServer(service))

	port := viper.GetString("GRPC_PORT")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal().Err(err).Str("port", port).Msg("Failed to listen")
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Info().Str("signal", sig.String()).Msg("Shutting down")
		cancel()
		grpcServer.GracefulStop()
	}()

	log.Info().Str("port", port).Msg("Network Analytics Service started")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("gRPC server failed")
	}
}
