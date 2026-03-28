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
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	linkv1 "github.com/smpp-server/smpp-server/api/proto/linkv1"
	"github.com/smpp-server/smpp-server/internal/services/link/application"
	linkgrpc "github.com/smpp-server/smpp-server/internal/services/link/grpc"
	"github.com/smpp-server/smpp-server/internal/services/link/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/services/link/redirect"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Database
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		envOrDefault("POSTGRES_USER", "smpp"),
		envOrDefault("POSTGRES_PASSWORD", "smpp_password"),
		envOrDefault("POSTGRES_HOST", "postgres"),
		envOrDefault("POSTGRES_PORT", "5432"),
		envOrDefault("POSTGRES_DB", "smpp_db"),
	)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatal().Err(err).Msg("database connection failed")
	}
	defer pool.Close()

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", envOrDefault("REDIS_HOST", "redis"), envOrDefault("REDIS_PORT", "6379")),
	})

	// Repositories
	linkRepo := repository.NewLinkRepository(pool)
	domainRepo := repository.NewDomainRepository(pool)
	clickRepo := repository.NewClickRepository(pool)

	// Service
	service := application.NewLinkService(linkRepo, domainRepo, clickRepo, rdb)

	// gRPC server
	grpcPort := envOrDefault("LINK_GRPC_PORT", "9102")
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen gRPC")
	}

	grpcServer := grpc.NewServer()
	linkv1.RegisterLinkServiceServer(grpcServer, linkgrpc.NewLinkGrpcServer(service))
	linkv1.RegisterDomainServiceServer(grpcServer, linkgrpc.NewDomainGrpcServer(service))
	reflection.Register(grpcServer)

	go func() {
		log.Info().Str("port", grpcPort).Msg("link-service gRPC started")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("gRPC serve failed")
		}
	}()

	// HTTP redirect server
	redirectPort := 8085
	go func() {
		log.Info().Int("port", redirectPort).Msg("redirect HTTP server started")
		if err := redirect.StartRedirectServer(ctx, redirectPort, service); err != nil {
			log.Error().Err(err).Msg("redirect server error")
		}
	}()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Info().Msg("shutting down link-service")
	cancel()
	grpcServer.GracefulStop()
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
