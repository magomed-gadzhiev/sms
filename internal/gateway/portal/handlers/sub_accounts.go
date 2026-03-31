package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/api/proto/analyticsv1"
	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/api/proto/billingv1"
	"github.com/smpp-server/smpp-server/api/proto/clientv1"
	"github.com/smpp-server/smpp-server/api/proto/messagingv1"
	webhookv1 "github.com/smpp-server/smpp-server/api/proto/webhookv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/audit"
)

// SubAccountHandlers содержит handlers для управления суб-аккаунтами
type SubAccountHandlers struct {
	clientClient    clientv1.ClientServiceClient
	billingClient   billingv1.BillingServiceClient
	authClient      authv1.AuthServiceClient
	messagingClient messagingv1.MessagingServiceClient
	analyticsClient analyticsv1.AnalyticsServiceClient
	webhookClient   webhookv1.WebhookServiceClient
	auditPublisher  *audit.Publisher
}

// NewSubAccountHandlers создает новый SubAccountHandlers
func NewSubAccountHandlers(
	clientClient clientv1.ClientServiceClient,
	billingClient billingv1.BillingServiceClient,
	authClient authv1.AuthServiceClient,
	messagingClient messagingv1.MessagingServiceClient,
	analyticsClient analyticsv1.AnalyticsServiceClient,
	webhookClient webhookv1.WebhookServiceClient,
	auditPublisher *audit.Publisher,
) *SubAccountHandlers {
	return &SubAccountHandlers{
		clientClient:    clientClient,
		billingClient:   billingClient,
		authClient:      authClient,
		messagingClient: messagingClient,
		analyticsClient: analyticsClient,
		webhookClient:   webhookClient,
		auditPublisher:  auditPublisher,
	}
}

// createSubAccountRequest представляет запрос на создание суб-аккаунта
type createSubAccountRequest struct {
	Name           string `json:"name"`
	Email          string `json:"email"`
	ContactPerson  string `json:"contact_person,omitempty"`
	InitialBalance string `json:"initial_balance,omitempty"`
	DailyLimit     int32  `json:"daily_limit,omitempty"`
	MonthlyLimit   int32  `json:"monthly_limit,omitempty"`
}

// updateSubAccountLimitsRequest представляет запрос на обновление лимитов
type updateSubAccountLimitsRequest struct {
	DailyLimit   int32 `json:"daily_limit"`
	MonthlyLimit int32 `json:"monthly_limit"`
}

// transferBalanceRequest представляет запрос на перевод средств
type transferBalanceRequest struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency,omitempty"`
}

// verifyParentOwnership проверяет, что текущий пользователь является родителем суб-аккаунта
func (h *SubAccountHandlers) getParentClientID(r *http.Request) (string, *shared.AppError) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		return "", shared.ErrUnauthorized("Пользователь не аутентифицирован")
	}
	return clientID.String(), nil
}

// ListSubAccounts обрабатывает GET /sub-accounts
func (h *SubAccountHandlers) ListSubAccounts(w http.ResponseWriter, r *http.Request) {
	parentClientID, appErr := h.getParentClientID(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}

	resp, err := h.clientClient.ListSubAccounts(r.Context(), &clientv1.ListSubAccountsRequest{
		ParentClientId: parentClientID,
	})
	if err != nil {
		log.Error().Err(err).Str("parent_client_id", parentClientID).Msg("ошибка получения списка суб-аккаунтов")
		respondGRPCError(w, err)
		return
	}

	// Обогащаем суб-аккаунты данными о балансе
	subAccounts := make([]map[string]interface{}, len(resp.SubAccounts))
	for i, sa := range resp.SubAccounts {
		saMap := subAccountToMap(sa)

		// Получаем баланс
		balResp, err := h.billingClient.GetBalance(r.Context(), &billingv1.GetBalanceRequest{
			ClientId: sa.Id,
		})
		if err == nil {
			saMap["balance"] = balResp.Balance
			saMap["currency"] = balResp.Currency
		}

		subAccounts[i] = saMap
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sub_accounts":    subAccounts,
		"max_sub_accounts": resp.MaxSubAccounts,
		"current_count":   resp.CurrentCount,
	})
}

// CreateSubAccount обрабатывает POST /sub-accounts
func (h *SubAccountHandlers) CreateSubAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	parentClientID, appErr := h.getParentClientID(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}

	var req createSubAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("Поле name обязательно"))
		return
	}

	// Создаем суб-аккаунт через client service
	createResp, err := h.clientClient.CreateSubAccount(r.Context(), &clientv1.CreateSubAccountRequest{
		ParentClientId: parentClientID,
		Name:           req.Name,
		Email:          req.Email,
		ContactPerson:  req.ContactPerson,
		DailyLimit:     req.DailyLimit,
		MonthlyLimit:   req.MonthlyLimit,
	})
	if err != nil {
		log.Error().Err(err).Str("parent_client_id", parentClientID).Msg("ошибка создания суб-аккаунта")
		respondGRPCError(w, err)
		return
	}

	result := subAccountToMap(createResp.SubAccount)

	// Если указан начальный баланс, переводим средства
	if req.InitialBalance != "" && req.InitialBalance != "0" {
		currency := "RUB"
		transferResp, err := h.billingClient.TransferBalance(r.Context(), &billingv1.TransferBalanceRequest{
			FromClientId: parentClientID,
			ToClientId:   createResp.SubAccount.Id,
			Amount:       req.InitialBalance,
			Currency:     currency,
		})
		if err != nil {
			log.Error().Err(err).
				Str("parent_client_id", parentClientID).
				Str("sub_account_id", createResp.SubAccount.Id).
				Str("amount", req.InitialBalance).
				Msg("ошибка перевода начального баланса суб-аккаунту")
			// Суб-аккаунт создан, но баланс не переведён
			result["balance_transfer_error"] = "не удалось перевести начальный баланс"
		} else {
			result["balance"] = transferResp.ToBalance
			result["parent_balance"] = transferResp.FromBalance
		}
	}

	// Публикуем audit event
	event := audit.NewAuditEvent(parentClientID, userID.String(), audit.ActionSubAccountCreated, audit.ResourceSubAccount, createResp.SubAccount.Id)
	event.IPAddress = getIPAddress(r)
	event.Details = map[string]any{
		"name":            req.Name,
		"email":           req.Email,
		"initial_balance": req.InitialBalance,
	}
	if err := h.auditPublisher.Publish(r.Context(), event); err != nil {
		log.Error().Err(err).Msg("ошибка публикации audit event")
	}

	respondJSON(w, http.StatusCreated, result)
}

// GetSubAccount обрабатывает GET /sub-accounts/{id}
func (h *SubAccountHandlers) GetSubAccount(w http.ResponseWriter, r *http.Request) {
	parentClientID, appErr := h.getParentClientID(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}

	vars := mux.Vars(r)
	subAccountID := vars["id"]
	if subAccountID == "" {
		respondError(w, shared.ErrInvalidInput("ID суб-аккаунта обязателен"))
		return
	}

	resp, err := h.clientClient.GetSubAccount(r.Context(), &clientv1.GetSubAccountRequest{
		SubAccountId:   subAccountID,
		ParentClientId: parentClientID,
	})
	if err != nil {
		log.Error().Err(err).Str("sub_account_id", subAccountID).Msg("ошибка получения суб-аккаунта")
		respondGRPCError(w, err)
		return
	}

	result := subAccountToMap(resp.SubAccount)

	// Получаем баланс
	balResp, err := h.billingClient.GetBalance(r.Context(), &billingv1.GetBalanceRequest{
		ClientId: subAccountID,
	})
	if err == nil {
		result["balance"] = balResp.Balance
		result["currency"] = balResp.Currency
	}

	respondJSON(w, http.StatusOK, result)
}

// UpdateLimits обрабатывает PUT /sub-accounts/{id}/limits
func (h *SubAccountHandlers) UpdateLimits(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	parentClientID, appErr := h.getParentClientID(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}

	vars := mux.Vars(r)
	subAccountID := vars["id"]
	if subAccountID == "" {
		respondError(w, shared.ErrInvalidInput("ID суб-аккаунта обязателен"))
		return
	}

	var req updateSubAccountLimitsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	resp, err := h.clientClient.UpdateSubAccountLimits(r.Context(), &clientv1.UpdateSubAccountLimitsRequest{
		SubAccountId:   subAccountID,
		ParentClientId: parentClientID,
		DailyLimit:     req.DailyLimit,
		MonthlyLimit:   req.MonthlyLimit,
	})
	if err != nil {
		log.Error().Err(err).Str("sub_account_id", subAccountID).Msg("ошибка обновления лимитов суб-аккаунта")
		respondGRPCError(w, err)
		return
	}

	// Публикуем audit event
	event := audit.NewAuditEvent(parentClientID, userID.String(), audit.ActionSubAccountLimitUpdated, audit.ResourceSubAccount, subAccountID)
	event.IPAddress = getIPAddress(r)
	event.Details = map[string]any{
		"daily_limit":   req.DailyLimit,
		"monthly_limit": req.MonthlyLimit,
	}
	if err := h.auditPublisher.Publish(r.Context(), event); err != nil {
		log.Error().Err(err).Msg("ошибка публикации audit event")
	}

	respondJSON(w, http.StatusOK, subAccountToMap(resp.SubAccount))
}

// TransferBalance обрабатывает POST /sub-accounts/{id}/transfer
func (h *SubAccountHandlers) TransferBalance(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	parentClientID, appErr := h.getParentClientID(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}

	vars := mux.Vars(r)
	subAccountID := vars["id"]
	if subAccountID == "" {
		respondError(w, shared.ErrInvalidInput("ID суб-аккаунта обязателен"))
		return
	}

	var req transferBalanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.Amount == "" {
		respondError(w, shared.ErrInvalidInput("Поле amount обязательно"))
		return
	}

	currency := req.Currency
	if currency == "" {
		currency = "RUB"
	}

	// Проверяем, что суб-аккаунт принадлежит родителю
	_, err := h.clientClient.GetSubAccount(r.Context(), &clientv1.GetSubAccountRequest{
		SubAccountId:   subAccountID,
		ParentClientId: parentClientID,
	})
	if err != nil {
		log.Error().Err(err).Str("sub_account_id", subAccountID).Msg("ошибка верификации суб-аккаунта")
		respondGRPCError(w, err)
		return
	}

	resp, err := h.billingClient.TransferBalance(r.Context(), &billingv1.TransferBalanceRequest{
		FromClientId: parentClientID,
		ToClientId:   subAccountID,
		Amount:       req.Amount,
		Currency:     currency,
	})
	if err != nil {
		log.Error().Err(err).
			Str("parent_client_id", parentClientID).
			Str("sub_account_id", subAccountID).
			Msg("ошибка перевода средств")
		respondGRPCError(w, err)
		return
	}

	// Публикуем audit event
	event := audit.NewAuditEvent(parentClientID, userID.String(), audit.ActionBalanceTransferOut, audit.ResourceBalance, subAccountID)
	event.IPAddress = getIPAddress(r)
	event.Details = map[string]any{
		"amount":         req.Amount,
		"currency":       currency,
		"to_sub_account": subAccountID,
		"transfer_id":    resp.TransferId,
	}
	if err := h.auditPublisher.Publish(r.Context(), event); err != nil {
		log.Error().Err(err).Msg("ошибка публикации audit event")
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"transfer_id":    resp.TransferId,
		"from_balance":   resp.FromBalance,
		"to_balance":     resp.ToBalance,
	})
}

// DeleteSubAccount обрабатывает DELETE /sub-accounts/{id}
func (h *SubAccountHandlers) DeleteSubAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	parentClientID, appErr := h.getParentClientID(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}

	vars := mux.Vars(r)
	subAccountID := vars["id"]
	if subAccountID == "" {
		respondError(w, shared.ErrInvalidInput("ID суб-аккаунта обязателен"))
		return
	}

	// Проверяем баланс суб-аккаунта и возвращаем родителю если > 0
	balResp, err := h.billingClient.GetBalance(r.Context(), &billingv1.GetBalanceRequest{
		ClientId: subAccountID,
	})
	if err == nil && balResp.Balance != "0" && balResp.Balance != "0.000000" {
		// Переводим остаток обратно родителю
		_, err := h.billingClient.TransferBalance(r.Context(), &billingv1.TransferBalanceRequest{
			FromClientId: subAccountID,
			ToClientId:   parentClientID,
			Amount:       balResp.Balance,
			Currency:     balResp.Currency,
		})
		if err != nil {
			log.Error().Err(err).
				Str("sub_account_id", subAccountID).
				Str("balance", balResp.Balance).
				Msg("ошибка возврата баланса при удалении суб-аккаунта")
			respondGRPCError(w, err)
			return
		}
	}

	// Удаляем суб-аккаунт
	deleteResp, err := h.clientClient.DeleteSubAccount(r.Context(), &clientv1.DeleteSubAccountRequest{
		SubAccountId:   subAccountID,
		ParentClientId: parentClientID,
	})
	if err != nil {
		log.Error().Err(err).Str("sub_account_id", subAccountID).Msg("ошибка удаления суб-аккаунта")
		respondGRPCError(w, err)
		return
	}

	// Публикуем audit event
	event := audit.NewAuditEvent(parentClientID, userID.String(), audit.ActionSubAccountDeleted, audit.ResourceSubAccount, subAccountID)
	event.IPAddress = getIPAddress(r)
	event.Details = map[string]any{
		"returned_balance": deleteResp.ReturnedBalance,
	}
	if err := h.auditPublisher.Publish(r.Context(), event); err != nil {
		log.Error().Err(err).Msg("ошибка публикации audit event")
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"returned_balance": deleteResp.ReturnedBalance,
	})
}

// GetSubAccountMessages обрабатывает GET /sub-accounts/{id}/messages
func (h *SubAccountHandlers) GetSubAccountMessages(w http.ResponseWriter, r *http.Request) {
	parentClientID, appErr := h.getParentClientID(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}

	vars := mux.Vars(r)
	subAccountID := vars["id"]
	if subAccountID == "" {
		respondError(w, shared.ErrInvalidInput("ID суб-аккаунта обязателен"))
		return
	}

	// Проверяем принадлежность суб-аккаунта
	_, err := h.clientClient.GetSubAccount(r.Context(), &clientv1.GetSubAccountRequest{
		SubAccountId:   subAccountID,
		ParentClientId: parentClientID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	page, perPage := parsePagination(r)
	offset := (page - 1) * perPage

	resp, err := h.messagingClient.GetMessageHistory(r.Context(), &messagingv1.GetMessageHistoryRequest{
		ClientId: subAccountID,
		Limit:    perPage,
		Offset:   offset,
	})
	if err != nil {
		log.Error().Err(err).Str("sub_account_id", subAccountID).Msg("ошибка получения сообщений суб-аккаунта")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"messages": resp.Messages,
		"total":    resp.Total,
		"page":     page,
		"per_page": perPage,
	})
}

// GetSubAccountAnalytics обрабатывает GET /sub-accounts/{id}/analytics
func (h *SubAccountHandlers) GetSubAccountAnalytics(w http.ResponseWriter, r *http.Request) {
	parentClientID, appErr := h.getParentClientID(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}

	vars := mux.Vars(r)
	subAccountID := vars["id"]
	if subAccountID == "" {
		respondError(w, shared.ErrInvalidInput("ID суб-аккаунта обязателен"))
		return
	}

	// Проверяем принадлежность суб-аккаунта
	_, err := h.clientClient.GetSubAccount(r.Context(), &clientv1.GetSubAccountRequest{
		SubAccountId:   subAccountID,
		ParentClientId: parentClientID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	resp, err := h.analyticsClient.GetStatistics(r.Context(), &analyticsv1.GetStatisticsRequest{
		ClientId: subAccountID,
	})
	if err != nil {
		log.Error().Err(err).Str("sub_account_id", subAccountID).Msg("ошибка получения аналитики суб-аккаунта")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetSubAccountAPIKeys обрабатывает GET /sub-accounts/{id}/api-keys
func (h *SubAccountHandlers) GetSubAccountAPIKeys(w http.ResponseWriter, r *http.Request) {
	parentClientID, appErr := h.getParentClientID(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}

	vars := mux.Vars(r)
	subAccountID := vars["id"]
	if subAccountID == "" {
		respondError(w, shared.ErrInvalidInput("ID суб-аккаунта обязателен"))
		return
	}

	// Проверяем принадлежность суб-аккаунта
	_, err := h.clientClient.GetSubAccount(r.Context(), &clientv1.GetSubAccountRequest{
		SubAccountId:   subAccountID,
		ParentClientId: parentClientID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	resp, err := h.authClient.ListAPIKeys(r.Context(), &authv1.ListAPIKeysRequest{
		UserId: subAccountID,
	})
	if err != nil {
		log.Error().Err(err).Str("sub_account_id", subAccountID).Msg("ошибка получения API ключей суб-аккаунта")
		respondGRPCError(w, err)
		return
	}

	keys := make([]map[string]interface{}, len(resp.Keys))
	for i, key := range resp.Keys {
		keys[i] = map[string]interface{}{
			"id":          key.Id,
			"name":        key.Name,
			"prefix":      key.Prefix,
			"active":      key.Active,
			"scopes":      key.Scopes,
			"allowed_ips": key.AllowedIps,
		}
		if key.CreatedAt != nil {
			keys[i]["created_at"] = key.CreatedAt.AsTime()
		}
		if key.ExpiresAt != nil {
			keys[i]["expires_at"] = key.ExpiresAt.AsTime()
		}
		if key.LastUsedAt != nil {
			keys[i]["last_used_at"] = key.LastUsedAt.AsTime()
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"keys": keys,
	})
}

// GetSubAccountWebhooks обрабатывает GET /sub-accounts/{id}/webhooks
func (h *SubAccountHandlers) GetSubAccountWebhooks(w http.ResponseWriter, r *http.Request) {
	parentClientID, appErr := h.getParentClientID(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}

	vars := mux.Vars(r)
	subAccountID := vars["id"]
	if subAccountID == "" {
		respondError(w, shared.ErrInvalidInput("ID суб-аккаунта обязателен"))
		return
	}

	// Проверяем принадлежность суб-аккаунта
	_, err := h.clientClient.GetSubAccount(r.Context(), &clientv1.GetSubAccountRequest{
		SubAccountId:   subAccountID,
		ParentClientId: parentClientID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	resp, err := h.webhookClient.ListSubscriptions(r.Context(), &webhookv1.ListSubscriptionsRequest{
		ClientId: subAccountID,
	})
	if err != nil {
		log.Error().Err(err).Str("sub_account_id", subAccountID).Msg("ошибка получения вебхуков суб-аккаунта")
		respondGRPCError(w, err)
		return
	}

	subscriptions := make([]map[string]interface{}, len(resp.Subscriptions))
	for i, sub := range resp.Subscriptions {
		subscriptions[i] = portalSubscriptionToMap(sub)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"webhooks": subscriptions,
	})
}

// subAccountToMap преобразует proto SubAccount в map для JSON ответа
func subAccountToMap(sa *clientv1.SubAccount) map[string]interface{} {
	if sa == nil {
		return nil
	}

	result := map[string]interface{}{
		"id":             sa.Id,
		"name":           sa.Name,
		"email":          sa.Email,
		"contact_person": sa.ContactPerson,
		"active":         sa.Active,
		"daily_limit":    sa.DailyLimit,
		"monthly_limit":  sa.MonthlyLimit,
		"messages_today": sa.MessagesToday,
		"messages_this_month": sa.MessagesThisMonth,
	}

	if sa.CreatedAt != nil {
		result["created_at"] = sa.CreatedAt.AsTime()
	}

	return result
}
