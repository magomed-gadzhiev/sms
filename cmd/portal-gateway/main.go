package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/smpp-server/smpp-server/internal/config"
	sharedmw "github.com/smpp-server/smpp-server/internal/api/middleware"
	"github.com/smpp-server/smpp-server/internal/gateway/portal"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/handlers"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/payment"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	portalrouter "github.com/smpp-server/smpp-server/internal/gateway/portal/router"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/audit"
)

func main() {
	// Инициализация логгера
	shared.InitLogger("development")
	logger := shared.WithService("portal-gateway")

	// Загрузка конфигурации
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	// Переопределение порта для Portal Gateway через переменную окружения
	portalHTTPPort := cfg.API.HTTP.Port
	if portStr := os.Getenv("PORTAL_HTTP_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			portalHTTPPort = port
		}
	}
	if portalHTTPPort == cfg.API.HTTP.Port {
		// По умолчанию используем 8082 для Portal Gateway
		portalHTTPPort = 8082
	}

	logger.Info().
		Str("version", cfg.Service.Version).
		Str("env", cfg.Service.Env).
		Int("http_port", portalHTTPPort).
		Msg("запуск Portal Gateway")

	// Получение адресов сервисов из переменных окружения или использование значений по умолчанию
	serviceAddresses := portal.ServiceAddresses{
		Auth:      getEnvOrDefault("AUTH_SERVICE_ADDR", "localhost:9090"),
		Client:    getEnvOrDefault("CLIENT_SERVICE_ADDR", "localhost:9090"),
		Billing:   getEnvOrDefault("BILLING_SERVICE_ADDR", "localhost:9090"),
		Messaging: getEnvOrDefault("MESSAGING_SERVICE_ADDR", "localhost:9090"),
		Analytics: getEnvOrDefault("ANALYTICS_SERVICE_ADDR", "localhost:9090"),
		Webhook:   getEnvOrDefault("WEBHOOK_SERVICE_ADDR", "localhost:9098"),
		Audit:     getEnvOrDefault("AUDIT_SERVICE_ADDR", ""),
		Routing:   getEnvOrDefault("ROUTING_SERVICE_ADDR", "localhost:9090"),
		Provider:  getEnvOrDefault("PROVIDER_SERVICE_ADDR", "localhost:9094"),
		Contact:   getEnvOrDefault("CONTACT_SERVICE_ADDR", "localhost:5012"),
		Campaign:     getEnvOrDefault("CAMPAIGN_SERVICE_ADDR", "localhost:5013"),
		Template:     getEnvOrDefault("TEMPLATE_SERVICE_ADDR", "localhost:9099"),
		Tarification: getEnvOrDefault("TARIFICATION_SERVICE_ADDR", "localhost:9100"),
		Link:         getEnvOrDefault("LINK_SERVICE_ADDR", "localhost:9103"),
	}

	// Инициализация gRPC клиентов
	serviceClients, err := portal.NewServiceClients(serviceAddresses)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания gRPC клиентов")
	}
	defer serviceClients.Close()

	logger.Info().Msg("gRPC клиенты инициализированы")

	// Создание health checker
	healthChecker := monitoring.NewHealthChecker("portal-gateway", cfg.Service.Version)

	// Создание Redis клиента для сессий
	redisAddr := getEnvOrDefault("REDIS_ADDR", "localhost:6379")
	redisPassword := getEnvOrDefault("REDIS_PASSWORD", "")
	redisDB := 0
	if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
		if db, err := strconv.Atoi(dbStr); err == nil {
			redisDB = db
		}
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})
	defer redisClient.Close()

	logger.Info().Str("addr", redisAddr).Msg("Redis клиент создан")

	// Создание пула PostgreSQL для прямых запросов (сегменты и т.д.)
	dbDSN := os.Getenv("DATABASE_URL")
	if dbDSN == "" {
		pgHost := getEnvOrDefault("POSTGRES_HOST", "localhost")
		pgPort := getEnvOrDefault("POSTGRES_PORT", "5432")
		pgUser := getEnvOrDefault("POSTGRES_USER", "smpp")
		pgPass := getEnvOrDefault("POSTGRES_PASSWORD", "smpp_password")
		pgDB := getEnvOrDefault("POSTGRES_DB", "smpp_db")
		dbDSN = "postgres://" + pgUser + ":" + pgPass + "@" + pgHost + ":" + pgPort + "/" + pgDB + "?sslmode=disable"
	}
	dbPool, err := pgxpool.New(context.Background(), dbDSN)
	if err != nil {
		logger.Warn().Err(err).Msg("не удалось создать PostgreSQL pool, segment handlers будут недоступны")
	}
	if dbPool != nil {
		defer dbPool.Close()
	}

	// Создание middleware
	sessionAuthMw := middleware.SessionAuthMiddleware(redisClient)
	csrfMw := middleware.CSRFMiddleware()
	loggingMw := sharedmw.LoggingMiddleware(logger)
	recoveryMw := sharedmw.RecoveryMiddleware()
	corsMw := sharedmw.CORSMiddlewareFromConfig(os.Getenv("CORS_ALLOWED_ORIGINS"))
	tenantLoggerMw := middleware.TenantLoggerMiddleware(logger)

	// Создание Kafka producer для audit events
	kafkaBrokers := getEnvOrDefault("KAFKA_BROKERS", "localhost:9092")
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Producer.RequiredAcks = sarama.WaitForAll
	kafkaConfig.Producer.Retry.Max = 3
	kafkaConfig.Producer.Return.Successes = true

	kafkaProducer, err := sarama.NewSyncProducer([]string{kafkaBrokers}, kafkaConfig)
	if err != nil {
		logger.Warn().Err(err).Msg("не удалось создать Kafka producer, audit events будут недоступны")
	}
	if kafkaProducer != nil {
		defer kafkaProducer.Close()
	}

	// Создание audit publisher
	auditPublisher := audit.NewPublisher(kafkaProducer, "audit.events", logger)

	// Создание handlers
	authHandlers := handlers.NewAuthHandlers(serviceClients.AuthClient, auditPublisher)
	profileHandlers := handlers.NewProfileHandlers(serviceClients.AuthClient, serviceClients.ClientClient, auditPublisher)
	dashboardHandlers := handlers.NewDashboardHandlers(
		serviceClients.BillingClient,
		serviceClients.AnalyticsClient,
		serviceClients.AuthClient,
		serviceClients.WebhookClient,
	)
	messageHandlers := handlers.NewMessageHandlers(serviceClients.MessagingClient)
	apiKeyHandlers := handlers.NewAPIKeyHandlers(serviceClients.AuthClient, auditPublisher)
	analyticsHandlers := handlers.NewAnalyticsHandlers(serviceClients.AnalyticsClient, serviceClients.BillingClient)
	webhookHandlers := handlers.NewWebhookHandlers(serviceClients.WebhookClient, auditPublisher)
	subAccountHandlers := handlers.NewSubAccountHandlers(
		serviceClients.ClientClient,
		serviceClients.BillingClient,
		serviceClients.AuthClient,
		serviceClients.MessagingClient,
		serviceClients.AnalyticsClient,
		serviceClients.WebhookClient,
		auditPublisher,
	)
	auditHandlers := handlers.NewAuditHandlers(serviceClients.AuditClient)
	lookupHandlers := handlers.NewLookupHandlers(serviceClients.RoutingClient)
	plansHandlers := handlers.NewPlansHandlers(serviceClients.ClientClient)
	providerHandlers := handlers.NewProviderHandlers(serviceClients.ProviderClient)
	contactHandlers := handlers.NewContactHandlers(serviceClients.ContactClient)
	campaignHandlers := handlers.NewCampaignHandlers(serviceClients.CampaignClient)
	templateHandlers := handlers.NewTemplateHandlers(serviceClients.TemplateClient)
	billingHandlers := handlers.NewBillingHandlers(serviceClients.BillingClient, payment.NewStubPaymentProvider())
	tariffHandlers := handlers.NewTariffHandlers(serviceClients.ClientClient, serviceClients.TarificationClient)
	domainHandlers := handlers.NewDomainHandlers(serviceClients.LinkDomainClient)

	settingsHandlers := handlers.NewSettingsHandlers(dbPool)
	segmentHandlers := handlers.NewSegmentHandlers(dbPool)

	// Настройка HTTP роутера
	router := portalrouter.SetupRouter(
		healthChecker,
		sessionAuthMw,
		csrfMw,
		loggingMw,
		recoveryMw,
		corsMw,
		tenantLoggerMw,
		authHandlers,
		profileHandlers,
		dashboardHandlers,
		messageHandlers,
		apiKeyHandlers,
		analyticsHandlers,
		webhookHandlers,
		subAccountHandlers,
		auditHandlers,
		lookupHandlers,
		plansHandlers,
		providerHandlers,
		contactHandlers,
		campaignHandlers,
		templateHandlers,
		billingHandlers,
		tariffHandlers,
		domainHandlers,
		settingsHandlers,
		segmentHandlers,
	)

	// Добавляем Prometheus metrics endpoint
	if cfg.Monitoring.Prometheus.Enabled {
		router.Handle(cfg.Monitoring.Prometheus.Path, promhttp.Handler()).Methods("GET")
		logger.Info().
			Str("path", cfg.Monitoring.Prometheus.Path).
			Msg("Prometheus metrics endpoint включен")
	}

	// Создание HTTP сервера
	httpServer := &http.Server{
		Addr:         net.JoinHostPort(cfg.API.HTTP.Host, strconv.Itoa(portalHTTPPort)),
		Handler:      router,
		ReadTimeout:  cfg.API.HTTP.ReadTimeout,
		WriteTimeout: cfg.API.HTTP.WriteTimeout,
		IdleTimeout:  cfg.API.HTTP.IdleTimeout,
	}

	// Запуск HTTP сервера
	go func() {
		logger.Info().
			Str("addr", httpServer.Addr).
			Msg("HTTP сервер запущен")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("ошибка запуска HTTP сервера")
		}
	}()

	// Ожидание сигнала для graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	logger.Info().Msg("получен сигнал остановки")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Остановка HTTP сервера
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("ошибка остановки HTTP сервера")
	} else {
		logger.Info().Msg("HTTP сервер остановлен")
	}

	logger.Info().Msg("Portal Gateway остановлен")
}

// getEnvOrDefault возвращает значение переменной окружения или значение по умолчанию
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
