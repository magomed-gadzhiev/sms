package router

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/handlers"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
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
	sessionOrAPIKeyAuthMiddleware func(http.Handler) http.Handler,
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
	clientRoutingHandlers *handlers.ClientRoutingHandlers,
	senderNameHandlers *handlers.SenderNameHandlers,
	opRegHandlers *handlers.OperatorRegistrationHandlers,
	portalOpTplHandlers *handlers.PortalOperatorTemplateHandlers,
	resellerHandlers *handlers.ResellerModerationHandlers,
	resellerSenderNameHandlers *handlers.ResellerSenderNameHandlers,
	resellerTemplateHandlers *handlers.ResellerTemplateHandlers,
	resellerDashboardHandlers *handlers.ResellerDashboardHandlers,
	resellerRoutingHandlers *handlers.ResellerRoutingHandlers,
	resellerTariffHandlers *handlers.ResellerTariffHandlers,
	resellerTariffPlanHandlers *handlers.ResellerTariffPlanHandlers,
	networkTariffsSummaryHandler *handlers.NetworkTariffsSummaryHandler,
	networkTariffTemplatesHandler *handlers.NetworkTariffTemplatesHandler,
	networkTariffEditorHandler *handlers.NetworkTariffEditorHandler,
	networkTariffBulkHandler *handlers.NetworkTariffBulkHandler,
	clientTariffsEffectiveHandler *handlers.ClientTariffsEffectiveHandler,
	resellerAnalyticsHandlers *handlers.ResellerAnalyticsHandlers,
	networkStatsHandlers *handlers.NetworkStatisticsHandlers,
	notificationHandlers *handlers.NotificationHandlers,
	searchHandlers *handlers.SearchHandlers,
	exportHandlers *handlers.ExportHandlers,
	optOutHandlers *handlers.OptOutHandlers,
	routeHandlers *handlers.RouteHandlers,
	healthHandlers *handlers.HealthHandlers,
	alertsHandlers *handlers.AlertsHandlers,
	wsMessagesHandlers *handlers.WsMessagesHandlers,
	companyHandlers *handlers.CompanyHandlers,
	referencesHandlers *handlers.ReferencesHandlers,
	networkProvidersHandlers *handlers.NetworkProvidersHandlers,
	networkProviderSetsHandlers *handlers.NetworkProviderSetsHandlers,
	networkProviderSetItemsHandlers *handlers.NetworkProviderSetItemsHandlers,
	networkAssignmentsHandlers *handlers.NetworkAssignmentsHandlers,
	subAccountNetworkOverridesHandlers *handlers.SubAccountNetworkOverridesHandlers,
	dbPool *pgxpool.Pool,
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

	// Dashboard metrics alias for Command Center
	protected.HandleFunc("/dashboard/metrics", dashboardHandlers.GetDashboard).Methods("GET")

	// Provider health for Command Center health map
	protected.HandleFunc("/providers/health", healthHandlers.GetProviderHealth).Methods("GET")

	// Smart alerts for Command Center
	protected.HandleFunc("/alerts", alertsHandlers.GetAlerts).Methods("GET")

	// Messages endpoints
	messages := protected.PathPrefix("/messages").Subrouter()
	messages.HandleFunc("", messageHandlers.SendMessage).Methods("POST")
	messages.HandleFunc("", messageHandlers.ListMessages).Methods("GET")
	messages.HandleFunc("/stream", messageHandlers.StreamMessages).Methods("GET")
	messages.HandleFunc("/export", messageHandlers.ExportCSV).Methods("GET")
	messages.HandleFunc("/{id}", messageHandlers.GetMessage).Methods("GET")

	// API Keys endpoints
	apiKeys := protected.PathPrefix("/api-keys").Subrouter()
	apiKeys.HandleFunc("", apiKeyHandlers.CreateAPIKey).Methods("POST")
	apiKeys.HandleFunc("", apiKeyHandlers.ListAPIKeys).Methods("GET")
	apiKeys.HandleFunc("/{id}", apiKeyHandlers.GetAPIKey).Methods("GET")
	apiKeys.HandleFunc("/{id}", apiKeyHandlers.UpdateAPIKey).Methods("PUT")
	apiKeys.HandleFunc("/{id}", apiKeyHandlers.RevokeAPIKey).Methods("DELETE")
	apiKeys.HandleFunc("/{id}/rotate", apiKeyHandlers.RotateAPIKey).Methods("POST")

	// Webhooks endpoints
	webhooks := protected.PathPrefix("/webhooks").Subrouter()
	webhooks.HandleFunc("", webhookHandlers.CreateWebhook).Methods("POST")
	webhooks.HandleFunc("", webhookHandlers.ListWebhooks).Methods("GET")
	// D.9 (этап 25 obs-5 re-audit'a 2026-04-29): до фикса GET /{id} не был
	// зарегистрирован, sub-аккаунты получали 404 plain-text. Сейчас — JSON-ответ.
	webhooks.HandleFunc("/{id}", webhookHandlers.GetWebhook).Methods("GET")
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
	subAccounts.HandleFunc("/{id}/transactions", subAccountHandlers.GetSubAccountTransactions).Methods("GET")
	subAccounts.HandleFunc("/{id}/analytics", subAccountHandlers.GetSubAccountAnalytics).Methods("GET")
	subAccounts.HandleFunc("/{id}/api-keys", subAccountHandlers.GetSubAccountAPIKeys).Methods("GET")
	subAccounts.HandleFunc("/{id}/webhooks", subAccountHandlers.GetSubAccountWebhooks).Methods("GET")
	subAccounts.HandleFunc("/{id}/campaigns", subAccountHandlers.GetSubAccountCampaigns).Methods("GET")

	// Sub-account network overrides (Plan 1 — Task 11 wiring).
	// Specific path with /{provider_id} registered first so gorilla/mux does not
	// confuse it with the parent /provider-overrides route on routing decisions.
	subAccounts.HandleFunc("/{id}/network/provider-overrides/{provider_id}", subAccountNetworkOverridesHandlers.DeleteProviderOverride).Methods("DELETE")
	subAccounts.HandleFunc("/{id}/network/provider-overrides", subAccountNetworkOverridesHandlers.AddProviderOverride).Methods("POST")
	subAccounts.HandleFunc("/{id}/network/overview", subAccountNetworkOverridesHandlers.Overview).Methods("GET")
	// Plan 2 Task 14: route-overrides (специфичные пути с {route_id} раньше parent'ов).
	subAccounts.HandleFunc("/{id}/network/route-overrides/{route_id}", subAccountNetworkOverridesHandlers.UpdateRouteOverride).Methods("PUT")
	subAccounts.HandleFunc("/{id}/network/route-overrides/{route_id}", subAccountNetworkOverridesHandlers.DeleteRouteOverride).Methods("DELETE")
	subAccounts.HandleFunc("/{id}/network/route-overrides", subAccountNetworkOverridesHandlers.AddRouteOverride).Methods("POST")

	// Sub-account routing (reseller management)
	subAccounts.HandleFunc("/{id}/providers", subAccountRoutingHandlers.AssignProvider).Methods("POST")
	subAccounts.HandleFunc("/{id}/providers", subAccountRoutingHandlers.ListProviders).Methods("GET")
	subAccounts.HandleFunc("/{id}/providers/{pid}", subAccountRoutingHandlers.RevokeProvider).Methods("DELETE")
	subAccounts.HandleFunc("/{id}/routes", subAccountRoutingHandlers.CreateRoute).Methods("POST")
	subAccounts.HandleFunc("/{id}/routes", subAccountRoutingHandlers.ListRoutes).Methods("GET")

	// Client routing (own routing configuration)
	routing := protected.PathPrefix("/routing").Subrouter()
	routing.HandleFunc("/mode", clientRoutingHandlers.GetRoutingMode).Methods("GET")
	routing.HandleFunc("/mode", clientRoutingHandlers.SetRoutingMode).Methods("PUT")
	routing.HandleFunc("/operators", clientRoutingHandlers.ListOperators).Methods("GET")
	routing.HandleFunc("/routes", clientRoutingHandlers.CreateRoute).Methods("POST")
	routing.HandleFunc("/routes", clientRoutingHandlers.ListRoutes).Methods("GET")
	routing.HandleFunc("/routes/{id}", clientRoutingHandlers.UpdateRoute).Methods("PUT")
	routing.HandleFunc("/routes/{id}", clientRoutingHandlers.DeleteRoute).Methods("DELETE")
	routing.HandleFunc("/strategy", clientRoutingHandlers.GetStrategy).Methods("GET")
	routing.HandleFunc("/strategy", clientRoutingHandlers.SetStrategy).Methods("PUT")

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
	billing.HandleFunc("/low-balance-threshold", billingHandlers.SetLowBalanceThreshold).Methods("PUT")

	// Tariff endpoints
	tariffs := protected.PathPrefix("/tariffs").Subrouter()
	tariffs.HandleFunc("/current", tariffHandlers.GetCurrentTariff).Methods("GET")
	tariffs.HandleFunc("/plans", tariffHandlers.ListAvailablePlans).Methods("GET")
	tariffs.HandleFunc("/change", tariffHandlers.ChangePlan).Methods("POST")
	tariffs.HandleFunc("/usage", tariffHandlers.GetUsage).Methods("GET")

	// Task 22: client self-view effective tariff matrix (/tariffs page).
	// Any authenticated client can read their own effective prices — no
	// is_reseller gate. Resellers / standalone clients get an empty matrix.
	protected.HandleFunc("/client/tariffs/effective", clientTariffsEffectiveHandler.Get).Methods("GET")

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
	contactLists.HandleFunc("/{id}/segments", contactHandlers.GetContactListSegments).Methods("GET")

	// Segments
	segments := protected.PathPrefix("/segments").Subrouter()
	segments.HandleFunc("", segmentHandlers.CreateSegment).Methods("POST")
	segments.HandleFunc("", segmentHandlers.ListSegments).Methods("GET")
	segments.HandleFunc("/{id}", segmentHandlers.GetSegment).Methods("GET")
	segments.HandleFunc("/{id}", segmentHandlers.UpdateSegment).Methods("PUT")
	segments.HandleFunc("/{id}", segmentHandlers.DeleteSegment).Methods("DELETE")
	segments.HandleFunc("/{id}/estimate", segmentHandlers.EstimateSegment).Methods("POST")

	// Campaigns — поддерживают session-auth ИЛИ API-key (Authorization: Bearer sk_live_...).
	// Монтируем отдельно от `protected`, чтобы иметь свой auth-стек.
	// CSRF применяется для session-запросов; для API-key запросов CSRF пропускается внутри middleware.
	//
	// BUG-83 fix (2026-05-01) + A.1 финальная чистка (2026-05-02): для API-key auth
	// проверяем scope по HTTP-методу. GET → messages:read; POST/PUT/DELETE/PATCH →
	// messages:send. Session-auth bypass'ит scope-проверку. Scope'ы приходят в context
	// из APIKeyAuthMiddleware (ValidateTokenResponse.scopes), без дополнительных
	// SQL/gRPC-roundtrip'ов. См. middleware/scope.go.
	campaigns := portalV1.PathPrefix("/campaigns").Subrouter()
	campaigns.Use(sessionOrAPIKeyAuthMiddleware)
	campaigns.Use(tenantLoggerMiddleware)
	campaigns.Use(csrfMiddleware)
	campaigns.Use(middleware.RequireScopeByMethod("messages:read", "messages:send"))
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
	settings.HandleFunc("/default-senders", settingsHandlers.GetDefaultSenders).Methods("GET")
	settings.HandleFunc("/default-senders", settingsHandlers.SetDefaultSenders).Methods("PUT")

	// Sender Names endpoints
	senderNames := protected.PathPrefix("/sender-names").Subrouter()
	senderNames.HandleFunc("", senderNameHandlers.CreateSenderName).Methods("POST")
	senderNames.HandleFunc("", senderNameHandlers.ListSenderNames).Methods("GET")
	senderNames.HandleFunc("/{id}", senderNameHandlers.GetSenderName).Methods("GET")
	senderNames.HandleFunc("/{id}", senderNameHandlers.UpdateSenderName).Methods("PUT")
	senderNames.HandleFunc("/{id}/resubmit", senderNameHandlers.ResubmitSenderName).Methods("POST")
	senderNames.HandleFunc("/{id}/history", senderNameHandlers.GetSenderNameHistory).Methods("GET")
	senderNames.HandleFunc("/{id}/operator-registrations", opRegHandlers.ListOperatorRegistrations).Methods("GET")
	senderNames.HandleFunc("/{id}/operator-registrations", opRegHandlers.BulkSubmitOperatorRegistrations).Methods("POST")
	senderNames.HandleFunc("/{id}/operator-registrations/{rid}/resubmit", opRegHandlers.ResubmitOperatorRegistration).Methods("POST")
	senderNames.HandleFunc("/{id}/operator-templates", portalOpTplHandlers.ListPortalOperatorTemplates).Methods("GET")
	senderNames.HandleFunc("/{id}/operator-templates", portalOpTplHandlers.CreatePortalOperatorTemplate).Methods("POST")
	senderNames.HandleFunc("/{id}/operator-templates/{tid}", portalOpTplHandlers.UpdatePortalOperatorTemplate).Methods("PUT")
	senderNames.HandleFunc("/{id}/operator-templates/{tid}", portalOpTplHandlers.DeletePortalOperatorTemplate).Methods("DELETE")
	senderNames.HandleFunc("/{id}/operator-templates/{tid}/submit", portalOpTplHandlers.SubmitPortalOperatorTemplate).Methods("POST")
	senderNames.HandleFunc("/{id}/operator-templates/{tid}/resubmit", portalOpTplHandlers.ResubmitPortalOperatorTemplate).Methods("POST")

	// Companies endpoints
	companies := protected.PathPrefix("/companies").Subrouter()
	companies.HandleFunc("", companyHandlers.ListCompanies).Methods("GET")
	companies.HandleFunc("", companyHandlers.CreateCompany).Methods("POST")
	companies.HandleFunc("/{id}", companyHandlers.GetCompany).Methods("GET")
	companies.HandleFunc("/{id}", companyHandlers.UpdateCompany).Methods("PUT")
	companies.HandleFunc("/{id}/set-default", companyHandlers.SetDefaultCompany).Methods("POST")
	companies.HandleFunc("/{id}/detach", companyHandlers.DetachCompany).Methods("DELETE")

	// Sender Registration endpoints
	senderRegs := protected.PathPrefix("/sender-registrations").Subrouter()
	senderRegs.HandleFunc("", senderNameHandlers.CreateSenderRegistration).Methods("POST")
	senderRegs.HandleFunc("/{id}/billing", senderNameHandlers.GetSenderRegistrationBilling).Methods("GET")

	// Operator Sender Tariff
	protected.HandleFunc("/operators/{id}/sender-tariff", senderNameHandlers.GetOperatorSenderTariff).Methods("GET")
	// Operators list with registration types
	protected.HandleFunc("/operators", senderNameHandlers.ListOperators).Methods("GET")

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

	// Opt-out list endpoints
	optOut := protected.PathPrefix("/opt-out").Subrouter()
	optOut.HandleFunc("", optOutHandlers.ListOptOuts).Methods("GET")
	optOut.HandleFunc("", optOutHandlers.AddOptOut).Methods("POST")
	optOut.HandleFunc("/import", optOutHandlers.ImportOptOut).Methods("POST")
	optOut.HandleFunc("/{id}", optOutHandlers.RemoveOptOut).Methods("DELETE")

	// References endpoints (operators, countries for dropdowns)
	references := protected.PathPrefix("/references").Subrouter()
	references.HandleFunc("/operators", referencesHandlers.ListOperators).Methods("GET")
	references.HandleFunc("/countries", referencesHandlers.ListCountries).Methods("GET")

	// Route management (admin-managed default & client routes)
	routes := protected.PathPrefix("/routes").Subrouter()
	routes.HandleFunc("", routeHandlers.CreateRoute).Methods("POST")
	routes.HandleFunc("", routeHandlers.ListRoutes).Methods("GET")
	routes.HandleFunc("/providers", routeHandlers.ListRouteProviders).Methods("GET")
	routes.HandleFunc("/references", routeHandlers.GetReferences).Methods("GET")
	routes.HandleFunc("/{id}", routeHandlers.GetRoute).Methods("GET")
	routes.HandleFunc("/{id}", routeHandlers.UpdateRoute).Methods("PUT")
	routes.HandleFunc("/{id}", routeHandlers.DeleteRoute).Methods("DELETE")

	// Network tariffs — reseller-admin facing /network/tariffs page (spec 2026-04-22).
	// Path lives at the top of the authenticated portal tree (not under /reseller/)
	// because the frontend route is /network/tariffs. The handler enforces the
	// is_reseller check itself — there is no distinct "reseller_admin" role in this
	// project; the reseller flag on clients is the gate.
	networkTariffs := protected.PathPrefix("/network/tariffs").Subrouter()
	networkTariffs.HandleFunc("/subaccounts-summary", networkTariffsSummaryHandler.List).Methods("GET")

	// Task 3: /network/tariff-templates (sibling of /network/tariffs, not nested
	// under it — the frontend calls it at the /network/ level).
	protected.HandleFunc("/network/tariff-templates", networkTariffTemplatesHandler.List).Methods("GET")
	// Task 4: create / bind / duplicate template.
	protected.HandleFunc("/network/tariff-templates", networkTariffTemplatesHandler.Create).Methods("POST")
	protected.HandleFunc("/network/tariff-templates/{id}/bind", networkTariffTemplatesHandler.Bind).Methods("POST")
	protected.HandleFunc("/network/tariff-templates/{id}/duplicate", networkTariffTemplatesHandler.Duplicate).Methods("POST")

	// Task 5: inheritance-aware matrix editor read endpoint.
	protected.HandleFunc("/network/tariff-editor/{id}", networkTariffEditorHandler.Get).Methods("GET")

	// Task 6: bulk-write endpoints for the matrix editor (tiers/cells + periods).
	protected.HandleFunc("/network/tariff-plans/{plan_id}/bulk", networkTariffBulkHandler.BulkPatch).Methods("PATCH")
	protected.HandleFunc("/network/tariff-plans/{plan_id}/periods", networkTariffBulkHandler.CreatePeriod).Methods("POST")
	// Fix 2026-04-23 (UX /network/tariffs): create a fresh reseller_tariff_plan
	// from the editor empty-state — wraps plan + default period + default tier
	// in one transaction so the matrix has something to render immediately.
	protected.HandleFunc("/network/tariff-plans", networkTariffBulkHandler.CreatePlan).Methods("POST")
	// Update strategy across every active plan matching (template|sub_account,
	// country, sender_category, traffic_type). Surfaces as a single dropdown
	// in the editor top-bar.
	protected.HandleFunc("/network/tariff-plans/strategy", networkTariffBulkHandler.UpdateStrategy).Methods("PUT")

	// Reseller moderation queue
	reseller := protected.PathPrefix("/reseller").Subrouter()
	// Closes BUG-82 этапа 27/30: до фикса группа network-statistics handlers
	// (statistics, analytics-summary, monitoring, drilldown, export, views)
	// не вызывала handler-level checkReseller — sub-account и обычный user
	// получали leak агрегированной статистики parent'а через /reseller/*.
	// Middleware на subrouter'е защищает от регрессии: новые endpoint'ы под
	// /reseller/* автоматически получают guard.
	reseller.Use(middleware.ResellerOnlyMiddleware(dbPool))
	resellerOpRegs := reseller.PathPrefix("/operator-registrations").Subrouter()
	resellerOpRegs.HandleFunc("", resellerHandlers.ListResellerOperatorRegistrations).Methods("GET")
	resellerOpRegs.HandleFunc("/{id}/approve", resellerHandlers.ApproveResellerOperatorRegistration).Methods("POST")
	resellerOpRegs.HandleFunc("/{id}/reject", resellerHandlers.RejectResellerOperatorRegistration).Methods("POST")
	resellerOpRegs.HandleFunc("/{id}/request-revision", resellerHandlers.RequestRevisionResellerOperatorRegistration).Methods("POST")

	// Reseller dashboard
	reseller.HandleFunc("/dashboard", resellerDashboardHandlers.GetResellerDashboard).Methods("GET")

	// Moderation counts
	reseller.HandleFunc("/moderation/counts", resellerHandlers.GetModerationCounts).Methods("GET")

	// Reseller sender names moderation
	resellerSN := reseller.PathPrefix("/sender-names").Subrouter()
	resellerSN.HandleFunc("", resellerSenderNameHandlers.ListResellerSenderNames).Methods("GET")
	resellerSN.HandleFunc("/{id}/approve", resellerSenderNameHandlers.ApproveResellerSenderName).Methods("POST")
	resellerSN.HandleFunc("/{id}/reject", resellerSenderNameHandlers.RejectResellerSenderName).Methods("POST")

	// Reseller templates moderation
	resellerTpl := reseller.PathPrefix("/templates").Subrouter()
	resellerTpl.HandleFunc("", resellerTemplateHandlers.ListResellerTemplates).Methods("GET")
	resellerTpl.HandleFunc("/{id}/approve", resellerTemplateHandlers.ApproveResellerTemplate).Methods("POST")
	resellerTpl.HandleFunc("/{id}/reject", resellerTemplateHandlers.RejectResellerTemplate).Methods("POST")
	resellerTpl.HandleFunc("/{id}/request-revision", resellerTemplateHandlers.RequestRevisionResellerTemplate).Methods("POST")

	// Reseller network management (Plan 1 — Task 11 wiring)
	network := reseller.PathPrefix("/network").Subrouter()

	network.HandleFunc("/providers", networkProvidersHandlers.List).Methods("GET")
	network.HandleFunc("/providers", networkProvidersHandlers.Create).Methods("POST")
	network.HandleFunc("/providers/{id}", networkProvidersHandlers.Update).Methods("PUT")
	network.HandleFunc("/providers/{id}", networkProvidersHandlers.Delete).Methods("DELETE")

	network.HandleFunc("/provider-sets", networkProviderSetsHandlers.List).Methods("GET")
	network.HandleFunc("/provider-sets", networkProviderSetsHandlers.Create).Methods("POST")
	network.HandleFunc("/provider-sets/{id}", networkProviderSetsHandlers.Update).Methods("PUT")
	network.HandleFunc("/provider-sets/{id}", networkProviderSetsHandlers.Delete).Methods("DELETE")
	network.HandleFunc("/provider-sets/{id}/items", networkProviderSetItemsHandlers.ListItems).Methods("GET")
	network.HandleFunc("/provider-sets/{id}/items", networkProviderSetItemsHandlers.PutItems).Methods("PUT")

	// /assignments/bulk must be registered before /assignments/{client_id} so
	// gorilla/mux does not match the literal path against the {client_id} pattern.
	network.HandleFunc("/assignments", networkAssignmentsHandlers.List).Methods("GET")
	network.HandleFunc("/assignments/bulk", networkAssignmentsHandlers.Bulk).Methods("POST")
	network.HandleFunc("/assignments/bulk/dry-run", networkAssignmentsHandlers.BulkDryRun).Methods("POST")
	network.HandleFunc("/assignments/{client_id}", networkAssignmentsHandlers.PutOne).Methods("PUT")

	// Reseller routing overview
	resellerRouting := reseller.PathPrefix("/routing").Subrouter()
	resellerRouting.HandleFunc("/providers", resellerRoutingHandlers.ListNetworkProviders).Methods("GET")
	resellerRouting.HandleFunc("/routes", resellerRoutingHandlers.ListNetworkRoutes).Methods("GET")
	resellerRouting.HandleFunc("/bulk-assign", resellerRoutingHandlers.BulkAssignProvider).Methods("POST")

	// Reseller tariffs
	resellerTariffs := reseller.PathPrefix("/tariffs").Subrouter()
	resellerTariffs.HandleFunc("", resellerTariffHandlers.ListTariffs).Methods("GET")
	resellerTariffs.HandleFunc("", resellerTariffHandlers.UpsertTariffs).Methods("PUT")
	resellerTariffs.HandleFunc("/copy", resellerTariffHandlers.CopyTariffs).Methods("POST")

	// Reseller tariff plans (new system: templates, plans, periods, tiers)
	// NOTE: standalone routes MUST be registered before PathPrefix subrouters
	// to avoid gorilla/mux prefix matching conflicts
	reseller.HandleFunc("/tariff-overview", resellerTariffPlanHandlers.TariffOverview).Methods("GET")

	resellerTariffTemplates := reseller.PathPrefix("/tariff-templates").Subrouter()
	resellerTariffTemplates.HandleFunc("", resellerTariffPlanHandlers.ListTemplates).Methods("GET")
	resellerTariffTemplates.HandleFunc("", resellerTariffPlanHandlers.CreateTemplate).Methods("POST")
	resellerTariffTemplates.HandleFunc("/{id}", resellerTariffPlanHandlers.UpdateTemplate).Methods("PUT")
	resellerTariffTemplates.HandleFunc("/{id}", resellerTariffPlanHandlers.DeleteTemplate).Methods("DELETE")
	resellerTariffTemplates.HandleFunc("/{id}/assign", resellerTariffPlanHandlers.AssignTemplate).Methods("POST")
	resellerTariffTemplates.HandleFunc("/{id}/assign/{sub_account_id}", resellerTariffPlanHandlers.UnassignTemplate).Methods("DELETE")

	resellerPlans := reseller.PathPrefix("/tariff-plans").Subrouter()
	resellerPlans.HandleFunc("", resellerTariffPlanHandlers.ListPlans).Methods("GET")
	resellerPlans.HandleFunc("", resellerTariffPlanHandlers.CreatePlan).Methods("POST")
	resellerPlans.HandleFunc("/copy", resellerTariffPlanHandlers.CopyPlans).Methods("POST")
	resellerPlans.HandleFunc("/{id}", resellerTariffPlanHandlers.UpdatePlan).Methods("PUT")
	resellerPlans.HandleFunc("/{id}", resellerTariffPlanHandlers.DeletePlan).Methods("DELETE")
	resellerPlans.HandleFunc("/{id}/periods", resellerTariffPlanHandlers.ListPeriods).Methods("GET")
	resellerPlans.HandleFunc("/{id}/periods", resellerTariffPlanHandlers.CreatePeriod).Methods("POST")

	resellerPeriods := reseller.PathPrefix("/tariff-periods").Subrouter()
	resellerPeriods.HandleFunc("/{id}", resellerTariffPlanHandlers.UpdatePeriod).Methods("PUT")
	resellerPeriods.HandleFunc("/{id}", resellerTariffPlanHandlers.DeletePeriod).Methods("DELETE")
	resellerPeriods.HandleFunc("/{id}/tiers", resellerTariffPlanHandlers.ListTiers).Methods("GET")
	resellerPeriods.HandleFunc("/{id}/tiers", resellerTariffPlanHandlers.UpsertTiers).Methods("PUT", "POST")

	// Reseller analytics
	reseller.HandleFunc("/analytics", resellerAnalyticsHandlers.GetNetworkAnalytics).Methods("GET")

	// Network statistics / analytics / monitoring
	reseller.HandleFunc("/statistics", networkStatsHandlers.GetStatistics).Methods("GET")
	reseller.HandleFunc("/analytics-summary", networkStatsHandlers.GetAnalytics).Methods("GET")
	reseller.HandleFunc("/monitoring", networkStatsHandlers.GetMonitoring).Methods("GET")
	reseller.HandleFunc("/drilldown", networkStatsHandlers.GetDrillDown).Methods("GET")
	reseller.HandleFunc("/export", networkStatsHandlers.StartExport).Methods("POST")
	reseller.HandleFunc("/export/{id}/status", networkStatsHandlers.GetExportStatus).Methods("GET")
	reseller.HandleFunc("/export/{id}/download", networkStatsHandlers.GetExportStatus).Methods("GET")
	reseller.HandleFunc("/views", networkStatsHandlers.ListViews).Methods("GET")
	reseller.HandleFunc("/views", networkStatsHandlers.SaveView).Methods("POST")
	reseller.HandleFunc("/views/{id}", networkStatsHandlers.DeleteView).Methods("DELETE")

	// WebSocket: live message stream (bypasses CSRF — session auth only)
	wsProtected := portalV1.PathPrefix("").Subrouter()
	wsProtected.Use(sessionAuthMiddleware)
	wsProtected.Use(tenantLoggerMiddleware)
	wsProtected.HandleFunc("/ws/messages", wsMessagesHandlers.StreamMessages).Methods("GET")

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

// RegisterAggregatorQuotaRoutes добавляет маршруты просмотра квоты для агрегаторов
func RegisterAggregatorQuotaRoutes(
	router *mux.Router,
	sessionAuthMiddleware func(http.Handler) http.Handler,
	csrfMiddleware func(http.Handler) http.Handler,
	h *handlers.AggregatorQuotaHandlers,
) {
	quota := router.PathPrefix("/portal/v1/quota").Subrouter()
	quota.Use(sessionAuthMiddleware)
	quota.Use(csrfMiddleware)
	quota.HandleFunc("", h.GetMyQuota).Methods("GET")
	quota.HandleFunc("/history", h.GetQuotaSpending).Methods("GET")
}

// RegisterDetalizationRoutes добавляет маршруты детализации сообщений для клиентского портала
func RegisterDetalizationRoutes(
	router *mux.Router,
	sessionAuthMiddleware func(http.Handler) http.Handler,
	csrfMiddleware func(http.Handler) http.Handler,
	h *handlers.DetalizationHandlers,
) {
	detalization := router.PathPrefix("/portal/v1/detalization").Subrouter()
	detalization.Use(sessionAuthMiddleware)
	detalization.Use(csrfMiddleware)
	detalization.HandleFunc("", h.ListMessages).Methods("GET")
	detalization.HandleFunc("/{id}", h.GetMessage).Methods("GET")
}

// RegisterNotificationSettingsRoutes добавляет маршруты настроек уведомлений для клиентского портала
func RegisterNotificationSettingsRoutes(
	router *mux.Router,
	sessionAuthMiddleware func(http.Handler) http.Handler,
	csrfMiddleware func(http.Handler) http.Handler,
	h *handlers.NotificationSettingsHandlers,
) {
	notifSettings := router.PathPrefix("/portal/v1/settings/notifications").Subrouter()
	notifSettings.Use(sessionAuthMiddleware)
	notifSettings.Use(csrfMiddleware)
	notifSettings.HandleFunc("", h.GetSettings).Methods("GET")
	notifSettings.HandleFunc("", h.PutSettings).Methods("PUT")
}

// RegisterCampaignScheduleRoutes добавляет маршруты для повторяющихся кампаний
func RegisterCampaignScheduleRoutes(
	router *mux.Router,
	sessionAuthMiddleware func(http.Handler) http.Handler,
	csrfMiddleware func(http.Handler) http.Handler,
	h *handlers.CampaignScheduleHandlers,
) {
	schedules := router.PathPrefix("/portal/v1/campaign-schedules").Subrouter()
	schedules.Use(sessionAuthMiddleware)
	schedules.Use(csrfMiddleware)
	schedules.HandleFunc("", h.List).Methods("GET")
	schedules.HandleFunc("", h.Create).Methods("POST")
	schedules.HandleFunc("/{id}", h.Update).Methods("PATCH")
	schedules.HandleFunc("/{id}", h.Toggle).Methods("PUT")
	schedules.HandleFunc("/{id}", h.Delete).Methods("DELETE")
}

// RegisterCostEstimateRoutes добавляет маршрут оценки стоимости кампании
func RegisterCostEstimateRoutes(
	router *mux.Router,
	sessionAuthMiddleware func(http.Handler) http.Handler,
	csrfMiddleware func(http.Handler) http.Handler,
	h *handlers.CostEstimateHandlers,
) {
	campaigns := router.PathPrefix("/portal/v1/campaigns").Subrouter()
	campaigns.Use(sessionAuthMiddleware)
	campaigns.Use(csrfMiddleware)
	campaigns.HandleFunc("/estimate-cost", h.Estimate).Methods("POST")
}

// RegisterCascadeDeliveryRoutes добавляет маршруты для истории каскадных доставок (клиентский портал)
func RegisterCascadeDeliveryRoutes(
	router *mux.Router,
	sessionAuthMiddleware func(http.Handler) http.Handler,
	h *handlers.CascadeDeliveryHandlers,
	strategies *handlers.CascadeStrategyHandlers,
) {
	cascade := router.PathPrefix("/portal/v1/cascade").Subrouter()
	cascade.Use(sessionAuthMiddleware)
	cascade.HandleFunc("/deliveries", h.ListDeliveries).Methods("GET")
	cascade.HandleFunc("/deliveries/{id}", h.GetDelivery).Methods("GET")
	cascade.HandleFunc("/stats", h.GetStats).Methods("GET")
	if strategies != nil {
		cascade.HandleFunc("/strategies", strategies.ListStrategiesClient).Methods("GET")
	}
}

// RegisterCascadeAdminRoutes добавляет admin-маршруты для управления каналами и стратегиями
func RegisterCascadeAdminRoutes(
	router *mux.Router,
	sessionAuthMiddleware func(http.Handler) http.Handler,
	csrfMiddleware func(http.Handler) http.Handler,
	channels *handlers.CascadeChannelHandlers,
	strategies *handlers.CascadeStrategyHandlers,
) {
	admin := router.PathPrefix("/portal/v1/admin").Subrouter()
	admin.Use(sessionAuthMiddleware)
	admin.Use(middleware.AdminRoleMiddleware)
	admin.Use(csrfMiddleware)

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
