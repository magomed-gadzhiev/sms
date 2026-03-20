package router

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/smpp-server/smpp-server/internal/gateway/client/handlers"
	"github.com/smpp-server/smpp-server/internal/monitoring"
)

// SetupRouter настраивает HTTP роутер для Client Gateway
func SetupRouter(
	smsHandlers *handlers.SMSHandlers,
	accountHandlers *handlers.AccountHandlers,
	webhookHandlers *handlers.WebhookHandlers,
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

	// Client API v1
	apiV1 := router.PathPrefix("/api/v1").Subrouter()

	// SMS endpoints
	sms := apiV1.PathPrefix("/sms").Subrouter()
	sms.HandleFunc("/send", smsHandlers.SendSMS).Methods("POST")
	sms.HandleFunc("/batch", smsHandlers.SendBatch).Methods("POST")
	sms.HandleFunc("/status/{id}", smsHandlers.GetStatus).Methods("GET")
	sms.HandleFunc("/history", smsHandlers.GetHistory).Methods("GET")

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

	// Health check endpoints (без аутентификации)
	router.HandleFunc("/health", healthChecker.Handler()).Methods("GET")
	router.HandleFunc("/health/live", healthChecker.LivenessHandler()).Methods("GET")
	router.HandleFunc("/health/ready", healthChecker.ReadinessHandler()).Methods("GET")

	return router
}
