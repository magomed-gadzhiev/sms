package router

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/smpp-server/smpp-server/internal/docs"
	"github.com/smpp-server/smpp-server/internal/gateway/client/handlers"
	"github.com/smpp-server/smpp-server/internal/monitoring"
)

// SetupRouter настраивает HTTP роутер для Client Gateway
func SetupRouter(
	smsHandlers *handlers.SMSHandlers,
	accountHandlers *handlers.AccountHandlers,
	webhookHandlers *handlers.WebhookHandlers,
	templateHandlers *handlers.TemplateHandlers,
	lookupHandlers *handlers.LookupHandlers,
	healthChecker *monitoring.HealthChecker,
	authMiddleware func(http.Handler) http.Handler,
	loggingMiddleware func(http.Handler) http.Handler,
	recoveryMiddleware func(http.Handler) http.Handler,
	corsMiddleware func(http.Handler) http.Handler,
	rateLimitMiddleware func(http.Handler) http.Handler,
	quotaMiddleware func(http.Handler) http.Handler,
) *mux.Router {
	router := mux.NewRouter()

	// Применяем middleware в правильном порядке
	router.Use(recoveryMiddleware)
	router.Use(loggingMiddleware)
	router.Use(corsMiddleware)
	router.Use(authMiddleware)
	router.Use(rateLimitMiddleware)
	router.Use(quotaMiddleware)

	// Client API v1
	apiV1 := router.PathPrefix("/api/v1").Subrouter()

	// SMS endpoints
	sms := apiV1.PathPrefix("/sms").Subrouter()
	sms.HandleFunc("/send", smsHandlers.SendSMS).Methods("POST")
	sms.HandleFunc("/batch", smsHandlers.SendBatch).Methods("POST")
	sms.HandleFunc("/status/{id}", smsHandlers.GetStatus).Methods("GET")
	sms.HandleFunc("/history", smsHandlers.GetHistory).Methods("GET")
	sms.HandleFunc("/{id}", smsHandlers.CancelSMS).Methods("DELETE")

	// Account endpoints
	account := apiV1.PathPrefix("/account").Subrouter()
	account.HandleFunc("/balance", accountHandlers.GetBalance).Methods("GET")
	account.HandleFunc("/stats", accountHandlers.GetStats).Methods("GET")

	// Webhook endpoints
	webhooks := apiV1.PathPrefix("/webhooks").Subrouter()
	webhooks.HandleFunc("", webhookHandlers.CreateWebhook).Methods("POST")
	webhooks.HandleFunc("", webhookHandlers.ListWebhooks).Methods("GET")
	webhooks.HandleFunc("/{id}", webhookHandlers.GetWebhook).Methods("GET")
	webhooks.HandleFunc("/{id}", webhookHandlers.UpdateWebhook).Methods("PUT")
	webhooks.HandleFunc("/{id}", webhookHandlers.DeleteWebhook).Methods("DELETE")

	// Lookup endpoints
	lookup := apiV1.PathPrefix("/lookup").Subrouter()
	lookup.HandleFunc("", lookupHandlers.SingleLookup).Methods("POST")
	lookup.HandleFunc("/bulk", lookupHandlers.BulkLookup).Methods("POST")
	lookup.HandleFunc("/history", lookupHandlers.GetLookupHistory).Methods("GET")

	// Template endpoints
	templates := apiV1.PathPrefix("/templates").Subrouter()
	templates.HandleFunc("", templateHandlers.CreateTemplate).Methods("POST")
	templates.HandleFunc("", templateHandlers.ListTemplates).Methods("GET")
	templates.HandleFunc("/{id}", templateHandlers.GetTemplate).Methods("GET")
	templates.HandleFunc("/{id}", templateHandlers.UpdateTemplate).Methods("PUT")
	templates.HandleFunc("/{id}", templateHandlers.DeleteTemplate).Methods("DELETE")
	templates.HandleFunc("/{id}/audit", templateHandlers.GetTemplateAudit).Methods("GET")

	// Health check endpoints (без аутентификации)
	router.HandleFunc("/health", healthChecker.Handler()).Methods("GET")
	router.HandleFunc("/health/live", healthChecker.LivenessHandler()).Methods("GET")
	router.HandleFunc("/health/ready", healthChecker.ReadinessHandler()).Methods("GET")

	// API документация (без аутентификации)
	router.HandleFunc("/docs", docs.SwaggerUIHandler()).Methods("GET")
	router.HandleFunc("/docs/openapi.yaml", docs.OpenAPISpecHandler()).Methods("GET")

	return router
}
