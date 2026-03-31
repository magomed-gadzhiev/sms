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
	countryHandlers *handlers.CountryHandler,
	operatorHandlers *handlers.OperatorHandler,
	tarificationHandlers *handlers.TarificationHandler,
	hlrHandlers *handlers.HLRHandlers,
	clientRoutingHandlers *handlers.ClientRoutingHandlers,
	systemDefaultsHandlers *handlers.SystemDefaultsHandlers,
	stubConfigHandlers *handlers.StubConfigHandlers,
	userHandlers *handlers.UserHandlers,
	roleHandlers *handlers.RoleHandlers,
	senderNameHandlers *handlers.AdminSenderNameHandlers,
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

	// Client Routing endpoints (nested under clients)
	clientRouting := clients.PathPrefix("/{id}").Subrouter()
	clientRouting.HandleFunc("/providers", clientRoutingHandlers.AssignProvider).Methods("POST")
	clientRouting.HandleFunc("/providers", clientRoutingHandlers.ListProviders).Methods("GET")
	clientRouting.HandleFunc("/providers/{pid}", clientRoutingHandlers.RevokeProvider).Methods("DELETE")
	clientRouting.HandleFunc("/providers/share", clientRoutingHandlers.ShareProvider).Methods("POST")
	clientRouting.HandleFunc("/providers/share/{sid}", clientRoutingHandlers.RevokeShared).Methods("DELETE")
	clientRouting.HandleFunc("/routes", clientRoutingHandlers.CreateRoute).Methods("POST")
	clientRouting.HandleFunc("/routes", clientRoutingHandlers.ListRoutes).Methods("GET")
	clientRouting.HandleFunc("/routes/{rid}", clientRoutingHandlers.UpdateRoute).Methods("PUT")
	clientRouting.HandleFunc("/routes/{rid}", clientRoutingHandlers.DeleteRoute).Methods("DELETE")
	clientRouting.HandleFunc("/routing-strategy", clientRoutingHandlers.SetStrategy).Methods("PUT")
	clientRouting.HandleFunc("/routing-strategy", clientRoutingHandlers.GetStrategy).Methods("GET")
	clientRouting.HandleFunc("/analytics/margin", clientRoutingHandlers.GetMarginReport).Methods("GET")

	// Providers endpoints
	providers := adminV1.PathPrefix("/providers").Subrouter()
	providers.HandleFunc("", providerHandlers.CreateProvider).Methods("POST")
	providers.HandleFunc("", providerHandlers.ListProviders).Methods("GET")
	providers.HandleFunc("/{id}", providerHandlers.GetProvider).Methods("GET")
	providers.HandleFunc("/{id}", providerHandlers.UpdateProvider).Methods("PUT")
	providers.HandleFunc("/{id}", providerHandlers.DeleteProvider).Methods("DELETE")
	providers.HandleFunc("/{id}/health", providerHandlers.GetProviderHealth).Methods("GET")
	providers.HandleFunc("/{id}/stub-config", stubConfigHandlers.Get).Methods("GET")
	providers.HandleFunc("/{id}/stub-config", stubConfigHandlers.Upsert).Methods("PUT")

	// System defaults
	system := adminV1.PathPrefix("/system").Subrouter()
	system.HandleFunc("/defaults", systemDefaultsHandlers.GetAll).Methods("GET")
	system.HandleFunc("/defaults/{key}", systemDefaultsHandlers.Set).Methods("PUT")

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
	billing.HandleFunc("/clients/{id}/freeze", billingHandlers.FreezeAccount).Methods("POST")
	billing.HandleFunc("/clients/{id}/unfreeze", billingHandlers.UnfreezeAccount).Methods("POST")
	billing.HandleFunc("/clients/{id}/credit-limit", billingHandlers.SetCreditLimit).Methods("PUT")
	billing.HandleFunc("/clients/{id}/low-balance-threshold", billingHandlers.SetLowBalanceThreshold).Methods("PUT")
	billing.HandleFunc("/balances", billingHandlers.ListBalances).Methods("GET")

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
	templates.HandleFunc("/{id}/assign", templateHandlers.AssignReviewer).Methods("POST")
	templates.HandleFunc("/{id}/request-revision", templateHandlers.RequestRevision).Methods("POST")

	// Country endpoints
	countries := adminV1.PathPrefix("/countries").Subrouter()
	countries.HandleFunc("", countryHandlers.CreateCountry).Methods("POST")
	countries.HandleFunc("", countryHandlers.ListCountries).Methods("GET")
	countries.HandleFunc("/{id}", countryHandlers.GetCountry).Methods("GET")
	countries.HandleFunc("/{id}", countryHandlers.UpdateCountry).Methods("PUT")

	// Operator endpoints
	operators := adminV1.PathPrefix("/operators").Subrouter()
	operators.HandleFunc("", operatorHandlers.CreateOperator).Methods("POST")
	operators.HandleFunc("", operatorHandlers.ListOperators).Methods("GET")
	operators.HandleFunc("/{id}", operatorHandlers.GetOperator).Methods("GET")
	operators.HandleFunc("/{id}", operatorHandlers.UpdateOperator).Methods("PUT")
	operators.HandleFunc("/{id}/prefixes", operatorHandlers.CreateOperatorPrefix).Methods("POST")
	operators.HandleFunc("/{id}/prefixes", operatorHandlers.ListOperatorPrefixes).Methods("GET")
	operators.HandleFunc("/{id}/prefixes/{prefix_id}", operatorHandlers.DeleteOperatorPrefix).Methods("DELETE")

	// Tarification endpoints
	tarification := adminV1.PathPrefix("/tarification").Subrouter()
	tarification.HandleFunc("/sender-registrations", tarificationHandlers.CreateSenderRegistration).Methods("POST")
	tarification.HandleFunc("/sender-registrations", tarificationHandlers.ListSenderRegistrations).Methods("GET")
	tarification.HandleFunc("/sender-registrations/{id}", tarificationHandlers.UpdateSenderRegistration).Methods("PUT")
	tarification.HandleFunc("/tariff-plans", tarificationHandlers.CreateTariffPlan).Methods("POST")
	tarification.HandleFunc("/tariff-plans", tarificationHandlers.ListTariffPlans).Methods("GET")
	tarification.HandleFunc("/tariff-plans/{id}", tarificationHandlers.UpdateTariffPlan).Methods("PUT")
	tarification.HandleFunc("/tariff-periods", tarificationHandlers.CreateTariffPeriod).Methods("POST")
	tarification.HandleFunc("/tariff-tiers", tarificationHandlers.CreateTariffTier).Methods("POST")
	tarification.HandleFunc("/tariff-tiers/{id}", tarificationHandlers.UpdateTariffTier).Methods("PUT")
	tarification.HandleFunc("/pricing-periods", tarificationHandlers.CreatePricingPeriod).Methods("POST")
	tarification.HandleFunc("/prepaid-fees", tarificationHandlers.CreatePrepaidFee).Methods("POST")
	tarification.HandleFunc("/usage", tarificationHandlers.ListUsageCounters).Methods("GET")

	// HLR Provider endpoints
	hlrProviders := adminV1.PathPrefix("/hlr/providers").Subrouter()
	hlrProviders.HandleFunc("", hlrHandlers.CreateProvider).Methods("POST")
	hlrProviders.HandleFunc("", hlrHandlers.ListProviders).Methods("GET")
	hlrProviders.HandleFunc("/{id}", hlrHandlers.GetProvider).Methods("GET")
	hlrProviders.HandleFunc("/{id}", hlrHandlers.UpdateProvider).Methods("PUT")
	hlrProviders.HandleFunc("/{id}", hlrHandlers.DeleteProvider).Methods("DELETE")
	hlrProviders.HandleFunc("/{id}/health", hlrHandlers.GetProviderHealth).Methods("GET")

	// Smart Route Weight endpoints
	weights := adminV1.PathPrefix("/routing/weights").Subrouter()
	weights.HandleFunc("", hlrHandlers.SetWeights).Methods("POST")
	weights.HandleFunc("", hlrHandlers.ListWeights).Methods("GET")
	weights.HandleFunc("/{id}", hlrHandlers.DeleteWeights).Methods("DELETE")

	// Users endpoints
	users := adminV1.PathPrefix("/users").Subrouter()
	users.HandleFunc("", userHandlers.ListUsers).Methods("GET")
	users.HandleFunc("", userHandlers.CreateUser).Methods("POST")
	users.HandleFunc("/{id}", userHandlers.GetUser).Methods("GET")
	users.HandleFunc("/{id}", userHandlers.UpdateUser).Methods("PUT")
	users.HandleFunc("/{id}/deactivate", userHandlers.DeactivateUser).Methods("POST")
	users.HandleFunc("/{id}/reset-2fa", userHandlers.ResetUser2FA).Methods("POST")
	users.HandleFunc("/{id}/reset-password", userHandlers.ResetUserPassword).Methods("POST")

	// Roles endpoints
	roles := adminV1.PathPrefix("/roles").Subrouter()
	roles.HandleFunc("", roleHandlers.ListRoles).Methods("GET")
	roles.HandleFunc("", roleHandlers.CreateRole).Methods("POST")
	roles.HandleFunc("/{id}", roleHandlers.GetRole).Methods("GET")
	roles.HandleFunc("/{id}", roleHandlers.UpdateRole).Methods("PUT")
	roles.HandleFunc("/{id}", roleHandlers.DeleteRole).Methods("DELETE")

	// Permissions endpoint
	adminV1.HandleFunc("/permissions", roleHandlers.ListPermissions).Methods("GET")

	// Sender Names endpoints
	senderNames := adminV1.PathPrefix("/sender-names").Subrouter()
	senderNames.HandleFunc("", senderNameHandlers.ListAllSenderNames).Methods("GET")
	senderNames.HandleFunc("/{id}/approve", senderNameHandlers.ApproveSenderName).Methods("POST")
	senderNames.HandleFunc("/{id}/reject", senderNameHandlers.RejectSenderName).Methods("POST")
	senderNames.HandleFunc("/{id}/deactivate", senderNameHandlers.DeactivateSenderName).Methods("POST")

	// Health check endpoints (без аутентификации)
	router.HandleFunc("/health", healthChecker.Handler()).Methods("GET")
	router.HandleFunc("/health/live", healthChecker.LivenessHandler()).Methods("GET")
	router.HandleFunc("/health/ready", healthChecker.ReadinessHandler()).Methods("GET")

	return router
}
