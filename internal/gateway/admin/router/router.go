package router

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/smpp-server/smpp-server/internal/gateway/admin/handlers"
	"github.com/smpp-server/smpp-server/internal/monitoring"
)

// SetupRouter настраивает HTTP роутер для Admin Gateway
func SetupRouter(
	clientHandlers *handlers.ClientHandlers,
	providerHandlers *handlers.ProviderHandlers,
	routingHandlers *handlers.RoutingHandlers,
	analyticsHandlers *handlers.AnalyticsHandlers,
	billingHandlers *handlers.BillingHandlers,
	webhookHandlers *handlers.WebhookHandlers,
	templateHandlers *handlers.TemplateHandlers,
	healthChecker *monitoring.HealthChecker,
	authMiddleware func(http.Handler) http.Handler,
	loggingMiddleware func(http.Handler) http.Handler,
	recoveryMiddleware func(http.Handler) http.Handler,
	corsMiddleware func(http.Handler) http.Handler,
) *mux.Router {
	router := mux.NewRouter()

	// Применяем middleware в правильном порядке
	router.Use(recoveryMiddleware)
	router.Use(loggingMiddleware)
	router.Use(corsMiddleware)
	router.Use(authMiddleware)

	// Admin API v1
	adminV1 := router.PathPrefix("/admin/v1").Subrouter()

	// Clients endpoints
	clients := adminV1.PathPrefix("/clients").Subrouter()
	clients.HandleFunc("", clientHandlers.CreateClient).Methods("POST")
	clients.HandleFunc("", clientHandlers.ListClients).Methods("GET")
	clients.HandleFunc("/{id}", clientHandlers.GetClient).Methods("GET")
	clients.HandleFunc("/{id}", clientHandlers.UpdateClient).Methods("PUT")
	clients.HandleFunc("/{id}", clientHandlers.DeleteClient).Methods("DELETE")
	clients.HandleFunc("/{id}/config", clientHandlers.GetClientConfig).Methods("GET")
	clients.HandleFunc("/{id}/config", clientHandlers.UpdateClientConfig).Methods("PUT")
	clients.HandleFunc("/{id}/rate-limits", clientHandlers.UpdateClientRateLimits).Methods("PUT")

	// Providers endpoints
	providers := adminV1.PathPrefix("/providers").Subrouter()
	providers.HandleFunc("", providerHandlers.CreateProvider).Methods("POST")
	providers.HandleFunc("", providerHandlers.ListProviders).Methods("GET")
	providers.HandleFunc("/{id}", providerHandlers.GetProvider).Methods("GET")
	providers.HandleFunc("/{id}", providerHandlers.UpdateProvider).Methods("PUT")
	providers.HandleFunc("/{id}", providerHandlers.DeleteProvider).Methods("DELETE")
	providers.HandleFunc("/{id}/health", providerHandlers.GetProviderHealth).Methods("GET")

	// Routes endpoints
	routes := adminV1.PathPrefix("/routes").Subrouter()
	routes.HandleFunc("", routingHandlers.CreateRoute).Methods("POST")
	routes.HandleFunc("", routingHandlers.ListRoutes).Methods("GET")
	routes.HandleFunc("/{id}", routingHandlers.UpdateRoute).Methods("PUT")
	routes.HandleFunc("/{id}", routingHandlers.DeleteRoute).Methods("DELETE")

	// Analytics endpoints
	analytics := adminV1.PathPrefix("/analytics").Subrouter()
	analytics.HandleFunc("/stats", analyticsHandlers.GetStatistics).Methods("GET")
	analytics.HandleFunc("/reports", analyticsHandlers.GenerateReport).Methods("POST")
	analytics.HandleFunc("/metrics/realtime", analyticsHandlers.GetRealtimeMetrics).Methods("GET")
	analytics.HandleFunc("/providers/{id}/performance", analyticsHandlers.GetProviderPerformance).Methods("GET")

	// Billing endpoints
	billing := adminV1.PathPrefix("/billing").Subrouter()
	billing.HandleFunc("/clients/{id}/balance", billingHandlers.GetBalance).Methods("GET")
	billing.HandleFunc("/clients/{id}/credits", billingHandlers.AddCredits).Methods("POST")
	billing.HandleFunc("/transactions", billingHandlers.GetTransactionHistory).Methods("GET")
	billing.HandleFunc("/pricing-rules", billingHandlers.GetPricingRules).Methods("GET")
	billing.HandleFunc("/pricing-rules", billingHandlers.CreatePricingRule).Methods("POST")

	// Webhook endpoints
	webhooks := adminV1.PathPrefix("/webhooks").Subrouter()
	webhooks.HandleFunc("", webhookHandlers.CreateWebhook).Methods("POST")
	webhooks.HandleFunc("", webhookHandlers.ListWebhooks).Methods("GET")
	webhooks.HandleFunc("/{id}", webhookHandlers.GetWebhook).Methods("GET")
	webhooks.HandleFunc("/{id}", webhookHandlers.UpdateWebhook).Methods("PUT")
	webhooks.HandleFunc("/{id}", webhookHandlers.DeleteWebhook).Methods("DELETE")

	// Template endpoints
	templates := adminV1.PathPrefix("/templates").Subrouter()
	templates.HandleFunc("", templateHandlers.ListTemplates).Methods("GET")
	templates.HandleFunc("/{id}", templateHandlers.GetTemplate).Methods("GET")
	templates.HandleFunc("/{id}/approve", templateHandlers.ApproveTemplate).Methods("POST")
	templates.HandleFunc("/{id}/reject", templateHandlers.RejectTemplate).Methods("POST")
	templates.HandleFunc("/{id}/audit", templateHandlers.GetTemplateAudit).Methods("GET")

	// Health check endpoints (без аутентификации)
	router.HandleFunc("/health", healthChecker.Handler()).Methods("GET")
	router.HandleFunc("/health/live", healthChecker.LivenessHandler()).Methods("GET")
	router.HandleFunc("/health/ready", healthChecker.ReadinessHandler()).Methods("GET")

	return router
}
