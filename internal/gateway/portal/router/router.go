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
	contactHandlers *handlers.ContactHandlers,
	campaignHandlers *handlers.CampaignHandlers,
	templateHandlers *handlers.TemplateHandlers,
	billingHandlers *handlers.BillingHandlers,
	tariffHandlers *handlers.TariffHandlers,
	domainHandlers *handlers.DomainHandlers,
	settingsHandlers *handlers.SettingsHandlers,
	segmentHandlers *handlers.SegmentHandlers,
	subAccountRoutingHandlers *handlers.SubAccountRoutingHandlers,
	senderNameHandlers *handlers.SenderNameHandlers,
	notificationHandlers *handlers.NotificationHandlers,
	searchHandlers *handlers.SearchHandlers,
	exportHandlers *handlers.ExportHandlers,
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

	// Billing callback — public, без session auth
	portalV1.HandleFunc("/billing/top-up/callback", billingHandlers.TopUpCallback).Methods("GET", "POST")

	// === Защищённые маршруты (с session auth + csrf) ===
	protected := portalV1.PathPrefix("").Subrouter()
	protected.Use(sessionAuthMiddleware)
	protected.Use(tenantLoggerMiddleware)
	protected.Use(csrfMiddleware)

	// Profile endpoints
	profile := protected.PathPrefix("/profile").Subrouter()
	profile.HandleFunc("", profileHandlers.GetProfile).Methods("GET")
	profile.HandleFunc("", profileHandlers.UpdateProfile).Methods("PUT")
	profile.HandleFunc("/password", profileHandlers.ChangePassword).Methods("PUT")
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
	messages.HandleFunc("/export", messageHandlers.ExportCSV).Methods("GET")
	messages.HandleFunc("/{id}", messageHandlers.GetMessage).Methods("GET")

	// API Keys endpoints
	apiKeys := protected.PathPrefix("/api-keys").Subrouter()
	apiKeys.HandleFunc("", apiKeyHandlers.CreateAPIKey).Methods("POST")
	apiKeys.HandleFunc("", apiKeyHandlers.ListAPIKeys).Methods("GET")
	apiKeys.HandleFunc("/{id}", apiKeyHandlers.GetAPIKey).Methods("GET")
	apiKeys.HandleFunc("/{id}", apiKeyHandlers.UpdateAPIKey).Methods("PUT")
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

	// Sub-account routing (reseller management)
	subAccounts.HandleFunc("/{id}/providers", subAccountRoutingHandlers.AssignProvider).Methods("POST")
	subAccounts.HandleFunc("/{id}/providers", subAccountRoutingHandlers.ListProviders).Methods("GET")
	subAccounts.HandleFunc("/{id}/providers/{pid}", subAccountRoutingHandlers.RevokeProvider).Methods("DELETE")
	subAccounts.HandleFunc("/{id}/routes", subAccountRoutingHandlers.CreateRoute).Methods("POST")
	subAccounts.HandleFunc("/{id}/routes", subAccountRoutingHandlers.ListRoutes).Methods("GET")

	// Lookup endpoints
	lookup := protected.PathPrefix("/lookup").Subrouter()
	lookup.HandleFunc("/history", lookupHandlers.GetLookupHistory).Methods("GET")
	lookup.HandleFunc("/stats", lookupHandlers.GetLookupStats).Methods("GET")
	lookup.HandleFunc("", lookupHandlers.NumberLookup).Methods("POST")
	lookup.HandleFunc("/bulk", lookupHandlers.BulkLookup).Methods("POST")

	// Templates endpoints
	templates := protected.PathPrefix("/templates").Subrouter()
	templates.HandleFunc("", templateHandlers.CreateTemplate).Methods("POST")
	templates.HandleFunc("", templateHandlers.ListTemplates).Methods("GET")
	templates.HandleFunc("/{id}", templateHandlers.GetTemplate).Methods("GET")
	templates.HandleFunc("/{id}", templateHandlers.UpdateTemplate).Methods("PUT")
	templates.HandleFunc("/{id}", templateHandlers.DeleteTemplate).Methods("DELETE")
	templates.HandleFunc("/{id}/render", templateHandlers.RenderTemplate).Methods("POST")
	templates.HandleFunc("/{id}/audit", templateHandlers.GetTemplateAuditLog).Methods("GET")
	templates.HandleFunc("/{id}/submit", templateHandlers.SubmitForReview).Methods("POST")

	// Billing endpoints
	billing := protected.PathPrefix("/billing").Subrouter()
	billing.HandleFunc("/balance", billingHandlers.GetBalance).Methods("GET")
	billing.HandleFunc("/transactions", billingHandlers.GetTransactions).Methods("GET")
	billing.HandleFunc("/top-up", billingHandlers.TopUp).Methods("POST")

	// Tariff endpoints
	tariffs := protected.PathPrefix("/tariffs").Subrouter()
	tariffs.HandleFunc("/current", tariffHandlers.GetCurrentTariff).Methods("GET")
	tariffs.HandleFunc("/plans", tariffHandlers.ListAvailablePlans).Methods("GET")
	tariffs.HandleFunc("/change", tariffHandlers.ChangePlan).Methods("POST")
	tariffs.HandleFunc("/usage", tariffHandlers.GetUsage).Methods("GET")

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

	// Contact Lists
	contactLists := protected.PathPrefix("/contact-lists").Subrouter()
	contactLists.HandleFunc("", contactHandlers.CreateContactList).Methods("POST")
	contactLists.HandleFunc("", contactHandlers.ListContactLists).Methods("GET")
	contactLists.HandleFunc("/{id}", contactHandlers.GetContactList).Methods("GET")
	contactLists.HandleFunc("/{id}", contactHandlers.UpdateContactList).Methods("PUT")
	contactLists.HandleFunc("/{id}", contactHandlers.DeleteContactList).Methods("DELETE")
	contactLists.HandleFunc("/{id}/attributes", contactHandlers.SetListAttributes).Methods("PUT")
	contactLists.HandleFunc("/{id}/attributes", contactHandlers.GetListAttributes).Methods("GET")
	contactLists.HandleFunc("/{id}/contacts", contactHandlers.CreateContact).Methods("POST")
	contactLists.HandleFunc("/{id}/contacts", contactHandlers.ListContacts).Methods("GET")
	contactLists.HandleFunc("/{id}/contacts/batch", contactHandlers.BatchUpsertContacts).Methods("POST")
	contactLists.HandleFunc("/{id}/contacts/tags", contactHandlers.AddTags).Methods("POST")
	contactLists.HandleFunc("/{id}/contacts/tags", contactHandlers.RemoveTags).Methods("DELETE")
	contactLists.HandleFunc("/{id}/contacts/{cid}", contactHandlers.UpdateContact).Methods("PUT")
	contactLists.HandleFunc("/{id}/contacts/{cid}", contactHandlers.DeleteContact).Methods("DELETE")
	contactLists.HandleFunc("/{id}/tags", contactHandlers.ListTags).Methods("GET")
	contactLists.HandleFunc("/{id}/imports/upload", contactHandlers.UploadImport).Methods("POST")
	contactLists.HandleFunc("/{id}/imports/{iid}/start", contactHandlers.StartImport).Methods("POST")
	contactLists.HandleFunc("/{id}/imports/{iid}", contactHandlers.GetImportStatus).Methods("GET")
	contactLists.HandleFunc("/{id}/imports", contactHandlers.ListImports).Methods("GET")
	contactLists.HandleFunc("/{id}/segment/preview", contactHandlers.PreviewSegment).Methods("POST")

	// Segments
	segments := protected.PathPrefix("/segments").Subrouter()
	segments.HandleFunc("", segmentHandlers.CreateSegment).Methods("POST")
	segments.HandleFunc("", segmentHandlers.ListSegments).Methods("GET")
	segments.HandleFunc("/{id}", segmentHandlers.GetSegment).Methods("GET")
	segments.HandleFunc("/{id}", segmentHandlers.UpdateSegment).Methods("PUT")
	segments.HandleFunc("/{id}", segmentHandlers.DeleteSegment).Methods("DELETE")
	segments.HandleFunc("/{id}/estimate", segmentHandlers.EstimateSegment).Methods("POST")

	// Campaigns
	campaigns := protected.PathPrefix("/campaigns").Subrouter()
	// Template preview (must be before /{id} routes)
	campaigns.HandleFunc("/templates/preview", campaignHandlers.PreviewTemplate).Methods("POST")
	campaigns.HandleFunc("", campaignHandlers.CreateCampaign).Methods("POST")
	campaigns.HandleFunc("", campaignHandlers.ListCampaigns).Methods("GET")
	campaigns.HandleFunc("/{id}", campaignHandlers.GetCampaign).Methods("GET")
	campaigns.HandleFunc("/{id}", campaignHandlers.UpdateCampaign).Methods("PUT")
	campaigns.HandleFunc("/{id}", campaignHandlers.DeleteCampaign).Methods("DELETE")
	campaigns.HandleFunc("/{id}/launch", campaignHandlers.LaunchCampaign).Methods("POST")
	campaigns.HandleFunc("/{id}/pause", campaignHandlers.PauseCampaign).Methods("POST")
	campaigns.HandleFunc("/{id}/resume", campaignHandlers.ResumeCampaign).Methods("POST")
	campaigns.HandleFunc("/{id}/cancel", campaignHandlers.CancelCampaign).Methods("POST")
	campaigns.HandleFunc("/{id}/variants", campaignHandlers.SetVariants).Methods("PUT")
	campaigns.HandleFunc("/{id}/ab-config", campaignHandlers.SetABConfig).Methods("PUT")
	campaigns.HandleFunc("/{id}/select-winner", campaignHandlers.SelectWinner).Methods("POST")
	campaigns.HandleFunc("/{id}/retry-config", campaignHandlers.SetRetryConfig).Methods("PUT")
	campaigns.HandleFunc("/{id}/retry", campaignHandlers.RetryFailed).Methods("POST")
	campaigns.HandleFunc("/{id}/stats", campaignHandlers.GetCampaignStats).Methods("GET")
	campaigns.HandleFunc("/{id}/timeline", campaignHandlers.GetTimeline).Methods("GET")
	campaigns.HandleFunc("/{id}/variants/compare", campaignHandlers.GetVariantComparison).Methods("GET")
	campaigns.HandleFunc("/{id}/heatmap", campaignHandlers.GetHeatmap).Methods("GET")
	campaigns.HandleFunc("/{id}/optimal-time", campaignHandlers.GetOptimalSendTime).Methods("GET")
	campaigns.HandleFunc("/{id}/report", campaignHandlers.ExportReport).Methods("GET")

	// Domains (settings)
	domains := protected.PathPrefix("/settings/domains").Subrouter()
	domains.HandleFunc("", domainHandlers.AddDomain).Methods("POST")
	domains.HandleFunc("", domainHandlers.ListDomains).Methods("GET")
	domains.HandleFunc("/{id}", domainHandlers.DeleteDomain).Methods("DELETE")

	// Settings endpoints
	settings := protected.PathPrefix("/settings").Subrouter()
	settings.HandleFunc("/frequency-caps", settingsHandlers.GetFrequencyCaps).Methods("GET")
	settings.HandleFunc("/frequency-caps", settingsHandlers.UpsertFrequencyCap).Methods("PUT")
	settings.HandleFunc("/quiet-hours", settingsHandlers.GetQuietHours).Methods("GET")
	settings.HandleFunc("/quiet-hours", settingsHandlers.UpsertQuietHours).Methods("PUT")

	// Sender Names endpoints
	senderNames := protected.PathPrefix("/sender-names").Subrouter()
	senderNames.HandleFunc("", senderNameHandlers.CreateSenderName).Methods("POST")
	senderNames.HandleFunc("", senderNameHandlers.ListSenderNames).Methods("GET")
	senderNames.HandleFunc("/{id}", senderNameHandlers.GetSenderName).Methods("GET")
	senderNames.HandleFunc("/{id}", senderNameHandlers.UpdateSenderName).Methods("PUT")
	senderNames.HandleFunc("/{id}/resubmit", senderNameHandlers.ResubmitSenderName).Methods("POST")
	senderNames.HandleFunc("/{id}/history", senderNameHandlers.GetSenderNameHistory).Methods("GET")

	// Sender Registration endpoints
	senderRegs := protected.PathPrefix("/sender-registrations").Subrouter()
	senderRegs.HandleFunc("", senderNameHandlers.CreateSenderRegistration).Methods("POST")
	senderRegs.HandleFunc("/{id}/billing", senderNameHandlers.GetSenderRegistrationBilling).Methods("GET")

	// Operator Sender Tariff
	protected.HandleFunc("/operators/{id}/sender-tariff", senderNameHandlers.GetOperatorSenderTariff).Methods("GET")

	// Notifications endpoints
	notifications := protected.PathPrefix("/notifications").Subrouter()
	notifications.HandleFunc("", notificationHandlers.GetNotifications).Methods("GET")
	notifications.HandleFunc("/read-all", notificationHandlers.MarkAllNotificationsRead).Methods("POST")
	notifications.HandleFunc("/{id}/read", notificationHandlers.MarkNotificationRead).Methods("POST")

	// Global search
	protected.HandleFunc("/search", searchHandlers.Search).Methods("GET")

	// CSV Export endpoints
	export := protected.PathPrefix("/export").Subrouter()
	export.HandleFunc("/start", exportHandlers.StartExport).Methods("POST")
	export.HandleFunc("/{job_id}/status", exportHandlers.GetExportStatus).Methods("GET")
	export.HandleFunc("/{job_id}/download", exportHandlers.DownloadExport).Methods("GET")

	return router
}

// RegisterCascadeWebhookRoutes добавляет маршруты webhook для каскадных каналов
func RegisterCascadeWebhookRoutes(router *mux.Router, h *handlers.CascadeWebhookHandlers) {
	router.HandleFunc("/webhooks/cascade/flash-call/{attempt_id}", h.FlashCallWebhook).Methods("POST")
}

// RegisterMaxMessengerWebhookRoute добавляет маршрут webhook для Max Messenger
func RegisterMaxMessengerWebhookRoute(router *mux.Router, handler http.HandlerFunc) {
	router.HandleFunc("/webhooks/cascade/max_messenger", handler).Methods("POST")
}

// RegisterCascadeDeliveryRoutes добавляет маршруты для истории каскадных доставок (клиентский портал)
func RegisterCascadeDeliveryRoutes(
	router *mux.Router,
	sessionAuthMiddleware func(http.Handler) http.Handler,
	h *handlers.CascadeDeliveryHandlers,
) {
	cascade := router.PathPrefix("/portal/v1/cascade").Subrouter()
	cascade.Use(sessionAuthMiddleware)
	cascade.HandleFunc("/deliveries", h.ListDeliveries).Methods("GET")
	cascade.HandleFunc("/deliveries/{id}", h.GetDelivery).Methods("GET")
	cascade.HandleFunc("/stats", h.GetStats).Methods("GET")
}

// RegisterCascadeAdminRoutes добавляет admin-маршруты для управления каналами и стратегиями
func RegisterCascadeAdminRoutes(
	router *mux.Router,
	sessionAuthMiddleware func(http.Handler) http.Handler,
	channels *handlers.CascadeChannelHandlers,
	strategies *handlers.CascadeStrategyHandlers,
) {
	admin := router.PathPrefix("/portal/v1/admin").Subrouter()
	admin.Use(sessionAuthMiddleware)

	// Channels
	ch := admin.PathPrefix("/channels").Subrouter()
	ch.HandleFunc("", channels.ListChannels).Methods("GET")
	ch.HandleFunc("", channels.CreateChannel).Methods("POST")
	ch.HandleFunc("/{id}", channels.GetChannel).Methods("GET")
	ch.HandleFunc("/{id}", channels.UpdateChannel).Methods("PUT")
	ch.HandleFunc("/{id}/toggle", channels.ToggleChannel).Methods("PUT")

	// Delivery strategies
	st := admin.PathPrefix("/delivery-strategies").Subrouter()
	st.HandleFunc("", strategies.ListStrategies).Methods("GET")
	st.HandleFunc("", strategies.CreateStrategy).Methods("POST")
	st.HandleFunc("/{id}", strategies.GetStrategy).Methods("GET")
	st.HandleFunc("/{id}", strategies.UpdateStrategy).Methods("PUT")
	st.HandleFunc("/{id}", strategies.DeleteStrategy).Methods("DELETE")

	// Operator channel support matrix
	admin.HandleFunc("/operator-channel-support", strategies.GetOperatorChannelSupport).Methods("GET")
	admin.HandleFunc("/operator-channel-support", strategies.UpdateOperatorChannelSupport).Methods("PUT")
}
