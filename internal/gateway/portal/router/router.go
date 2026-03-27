package router

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/handlers"
	"github.com/smpp-server/smpp-server/internal/monitoring"
)

// notImplemented — placeholder-обработчик для маршрутов, которые ещё не реализованы
func notImplemented(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "NOT_IMPLEMENTED",
			"message": "Этот эндпоинт ещё не реализован",
		},
	})
}

// SetupRouter настраивает HTTP роутер для Portal Gateway
func SetupRouter(
	healthChecker *monitoring.HealthChecker,
	sessionAuthMiddleware func(http.Handler) http.Handler,
	csrfMiddleware func(http.Handler) http.Handler,
	loggingMiddleware func(http.Handler) http.Handler,
	recoveryMiddleware func(http.Handler) http.Handler,
	corsMiddleware func(http.Handler) http.Handler,
	tenantLoggerMiddleware func(http.Handler) http.Handler,
	authHandlers *handlers.AuthHandlers,
	profileHandlers *handlers.ProfileHandlers,
	dashboardHandlers *handlers.DashboardHandlers,
	messageHandlers *handlers.MessageHandlers,
	apiKeyHandlers *handlers.APIKeyHandlers,
	analyticsHandlers *handlers.AnalyticsHandlers,
	webhookHandlers *handlers.WebhookHandlers,
	subAccountHandlers *handlers.SubAccountHandlers,
	auditHandlers *handlers.AuditHandlers,
	lookupHandlers *handlers.LookupHandlers,
	plansHandlers *handlers.PlansHandlers,
	providerHandlers *handlers.ProviderHandlers,
) *mux.Router {
	router := mux.NewRouter()

	// Применяем глобальные middleware в правильном порядке
	router.Use(recoveryMiddleware)
	router.Use(loggingMiddleware)
	router.Use(corsMiddleware)

	// Health check endpoints (без аутентификации)
	router.HandleFunc("/health", healthChecker.Handler()).Methods("GET")
	router.HandleFunc("/health/live", healthChecker.LivenessHandler()).Methods("GET")
	router.HandleFunc("/health/ready", healthChecker.ReadinessHandler()).Methods("GET")

	// Portal API v1
	portalV1 := router.PathPrefix("/portal/v1").Subrouter()

	// === Публичные маршруты (без session auth) ===

	// Plans endpoint — публичный, не требует аутентификации
	portalV1.HandleFunc("/plans", plansHandlers.ListPlans).Methods("GET")

	// Auth endpoints — аутентификация, не требуют сессии
	auth := portalV1.PathPrefix("/auth").Subrouter()
	auth.HandleFunc("/login", authHandlers.Login).Methods("POST")
	auth.HandleFunc("/login/2fa", authHandlers.LoginWith2FA).Methods("POST")
	auth.HandleFunc("/register", authHandlers.Register).Methods("POST")
	auth.HandleFunc("/password/reset-request", authHandlers.RequestPasswordReset).Methods("POST")
	auth.HandleFunc("/password/reset", authHandlers.ResetPassword).Methods("POST")

	// Logout требует сессии (нужно знать session_id)
	authProtected := auth.PathPrefix("").Subrouter()
	authProtected.Use(sessionAuthMiddleware)
	authProtected.HandleFunc("/logout", authHandlers.Logout).Methods("POST")

	// === Защищённые маршруты (с session auth + csrf) ===
	protected := portalV1.PathPrefix("").Subrouter()
	protected.Use(sessionAuthMiddleware)
	protected.Use(tenantLoggerMiddleware)
	protected.Use(csrfMiddleware)

	// Profile endpoints
	profile := protected.PathPrefix("/profile").Subrouter()
	profile.HandleFunc("", profileHandlers.GetProfile).Methods("GET")
	profile.HandleFunc("", profileHandlers.UpdateProfile).Methods("PUT")
	profile.HandleFunc("/password", notImplemented).Methods("PUT")
	profile.HandleFunc("/sandbox", profileHandlers.ToggleSandbox).Methods("PUT")
	profile.HandleFunc("/2fa/setup", profileHandlers.SetupTOTP).Methods("POST")
	profile.HandleFunc("/2fa/verify", profileHandlers.VerifyTOTP).Methods("POST")
	profile.HandleFunc("/2fa", profileHandlers.DisableTOTP).Methods("DELETE")

	// Dashboard endpoints
	protected.HandleFunc("/dashboard", dashboardHandlers.GetDashboard).Methods("GET")

	// Messages endpoints
	messages := protected.PathPrefix("/messages").Subrouter()
	messages.HandleFunc("", messageHandlers.SendMessage).Methods("POST")
	messages.HandleFunc("", messageHandlers.ListMessages).Methods("GET")
	messages.HandleFunc("/{id}", notImplemented).Methods("GET")

	// API Keys endpoints
	apiKeys := protected.PathPrefix("/api-keys").Subrouter()
	apiKeys.HandleFunc("", apiKeyHandlers.CreateAPIKey).Methods("POST")
	apiKeys.HandleFunc("", apiKeyHandlers.ListAPIKeys).Methods("GET")
	apiKeys.HandleFunc("/{id}", notImplemented).Methods("GET")
	apiKeys.HandleFunc("/{id}", notImplemented).Methods("PUT")
	apiKeys.HandleFunc("/{id}", apiKeyHandlers.RevokeAPIKey).Methods("DELETE")

	// Webhooks endpoints
	webhooks := protected.PathPrefix("/webhooks").Subrouter()
	webhooks.HandleFunc("", webhookHandlers.CreateWebhook).Methods("POST")
	webhooks.HandleFunc("", webhookHandlers.ListWebhooks).Methods("GET")
	webhooks.HandleFunc("/{id}", webhookHandlers.UpdateWebhook).Methods("PUT")
	webhooks.HandleFunc("/{id}", webhookHandlers.DeleteWebhook).Methods("DELETE")
	webhooks.HandleFunc("/{id}/test", webhookHandlers.TestWebhook).Methods("POST")

	// Analytics endpoints
	protected.HandleFunc("/analytics", analyticsHandlers.GetAnalytics).Methods("GET")

	// Sub-accounts endpoints
	subAccounts := protected.PathPrefix("/sub-accounts").Subrouter()
	subAccounts.HandleFunc("", subAccountHandlers.ListSubAccounts).Methods("GET")
	subAccounts.HandleFunc("", subAccountHandlers.CreateSubAccount).Methods("POST")
	subAccounts.HandleFunc("/{id}", subAccountHandlers.GetSubAccount).Methods("GET")
	subAccounts.HandleFunc("/{id}/limits", subAccountHandlers.UpdateLimits).Methods("PUT")
	subAccounts.HandleFunc("/{id}/transfer", subAccountHandlers.TransferBalance).Methods("POST")
	subAccounts.HandleFunc("/{id}", subAccountHandlers.DeleteSubAccount).Methods("DELETE")
	subAccounts.HandleFunc("/{id}/messages", subAccountHandlers.GetSubAccountMessages).Methods("GET")
	subAccounts.HandleFunc("/{id}/analytics", subAccountHandlers.GetSubAccountAnalytics).Methods("GET")
	subAccounts.HandleFunc("/{id}/api-keys", subAccountHandlers.GetSubAccountAPIKeys).Methods("GET")
	subAccounts.HandleFunc("/{id}/webhooks", subAccountHandlers.GetSubAccountWebhooks).Methods("GET")

	// Lookup endpoints
	lookup := protected.PathPrefix("/lookup").Subrouter()
	lookup.HandleFunc("/history", lookupHandlers.GetLookupHistory).Methods("GET")
	lookup.HandleFunc("/stats", lookupHandlers.GetLookupStats).Methods("GET")

	// Audit log endpoints
	protected.HandleFunc("/audit-log", auditHandlers.ListAuditLog).Methods("GET")

	// Providers endpoints
	providers := protected.PathPrefix("/providers").Subrouter()
	providers.HandleFunc("", providerHandlers.ListProviders).Methods("GET")
	providers.HandleFunc("", providerHandlers.CreateProvider).Methods("POST")
	providers.HandleFunc("/test-connection", providerHandlers.TestProviderConnection).Methods("POST")
	providers.HandleFunc("/{id}", providerHandlers.GetProvider).Methods("GET")
	providers.HandleFunc("/{id}", providerHandlers.UpdateProvider).Methods("PUT")
	providers.HandleFunc("/{id}", providerHandlers.DeleteProvider).Methods("DELETE")

	return router
}
