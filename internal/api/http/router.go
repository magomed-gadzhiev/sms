package http

import (
	"net/http"

	"github.com/gorilla/mux"
)

// SetupRouter настраивает HTTP роутер
func SetupRouter(
	handler *Handler,
	authMiddleware func(http.Handler) http.Handler,
	rateLimitMiddleware func(http.Handler) http.Handler,
	loggingMiddleware func(http.Handler) http.Handler,
	recoveryMiddleware func(http.Handler) http.Handler,
	corsMiddleware func(http.Handler) http.Handler,
	tenantLoggerMiddleware func(http.Handler) http.Handler,
) *mux.Router {
	router := mux.NewRouter()

	router.Use(recoveryMiddleware)
	router.Use(loggingMiddleware)
	router.Use(corsMiddleware)
	router.Use(authMiddleware)
	router.Use(tenantLoggerMiddleware)
	router.Use(rateLimitMiddleware)

	// API v1
	v1 := router.PathPrefix("/api/v1").Subrouter()

	// SMS endpoints
	v1.HandleFunc("/sms/send", handler.SendSMS).Methods("POST")
	v1.HandleFunc("/sms/batch", handler.SendBatchSMS).Methods("POST")
	v1.HandleFunc("/sms/status/{id}", handler.GetStatus).Methods("GET")
	v1.HandleFunc("/sms/history", handler.GetHistory).Methods("GET")
	v1.HandleFunc("/sms/scheduled", handler.GetScheduled).Methods("GET")
	v1.HandleFunc("/sms/{id}", handler.CancelSMS).Methods("DELETE")

	// Account endpoints
	v1.HandleFunc("/account/balance", handler.GetBalance).Methods("GET")
	v1.HandleFunc("/account/stats", handler.GetStats).Methods("GET")

	// Health check endpoints (без аутентификации)
	router.HandleFunc("/health", handler.Health).Methods("GET")

	// API документация (без аутентификации)
	router.HandleFunc("/docs", DocsHandler()).Methods("GET")
	router.HandleFunc("/docs/openapi.yaml", OpenAPISpecHandler()).Methods("GET")

	return router
}
