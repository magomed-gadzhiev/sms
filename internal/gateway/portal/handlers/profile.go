package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/api/proto/clientv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/audit"
)

// ProfileHandlers содержит handlers для управления профилем пользователя
type ProfileHandlers struct {
	authClient     authv1.AuthServiceClient
	clientClient   clientv1.ClientServiceClient
	auditPublisher *audit.Publisher
}

// NewProfileHandlers создает новый ProfileHandlers
func NewProfileHandlers(
	authClient authv1.AuthServiceClient,
	clientClient clientv1.ClientServiceClient,
	auditPublisher *audit.Publisher,
) *ProfileHandlers {
	return &ProfileHandlers{
		authClient:     authClient,
		clientClient:   clientClient,
		auditPublisher: auditPublisher,
	}
}

// updateProfileRequest представляет запрос на обновление профиля
type updateProfileRequest struct {
	ContactPerson string `json:"contact_person"`
	Phone         string `json:"phone"`
}

// setupTOTPResponse представляет ответ настройки TOTP
type verifyTOTPRequest struct {
	TOTPCode string `json:"totp_code"`
}

// disableTOTPRequest представляет запрос на отключение TOTP
type disableTOTPRequest struct {
	Password string `json:"password"`
}

// GetProfile обрабатывает GET /profile
func (h *ProfileHandlers) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}

	clientID, _ := middleware.GetClientID(r.Context())
	role, _ := middleware.GetRole(r.Context())

	// Формируем ответ на основе данных из контекста сессии
	response := map[string]interface{}{
		"id":             userID.String(),
		"email":          "",
		"company_name":   "",
		"contact_person": "",
		"phone":          "",
		"totp_enabled":   false,
		"is_sandbox":     false,
	}

	// Пытаемся получить данные пользователя через ValidateSession
	cookie, err := r.Cookie("portal_session")
	if err == nil && cookie.Value != "" {
		sessionResp, err := h.authClient.ValidateSession(r.Context(), &authv1.ValidateSessionRequest{
			SessionId: cookie.Value,
		})
		if err == nil && sessionResp.User != nil {
			response["id"] = sessionResp.User.Id
			response["email"] = sessionResp.User.Email
			response["company_name"] = sessionResp.User.Username
		}
	}

	// Всегда возвращаем роль пользователя
	response["role"] = role

	// Получаем информацию о клиенте (если clientID валидный)
	nilUUID := [16]byte{}
	if clientID != uuid.UUID(nilUUID) {
		clientResp, err := h.clientClient.GetClient(r.Context(), &clientv1.GetClientRequest{
			ClientId: clientID.String(),
		})
		if err == nil && clientResp.Client != nil {
			response["company_name"] = clientResp.Client.Name
			response["email"] = clientResp.Client.Email
			response["contact_person"] = clientResp.Client.ContactPerson
			response["phone"] = clientResp.Client.Phone
			response["is_sandbox"] = clientResp.Client.IsSandbox
		} else {
			log.Warn().Err(err).Str("client_id", clientID.String()).Msg("клиент не найден, продолжаем без данных клиента")
		}
	}

	respondJSON(w, http.StatusOK, response)
}

// toggleSandboxRequest представляет запрос на переключение sandbox-режима
type toggleSandboxRequest struct {
	Enable bool `json:"enable"`
}

// ToggleSandbox обрабатывает PUT /profile/sandbox
func (h *ProfileHandlers) ToggleSandbox(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	var req toggleSandboxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	resp, err := h.clientClient.ToggleSandbox(r.Context(), &clientv1.ToggleSandboxRequest{
		ClientId: clientID.String(),
		Enable:   req.Enable,
	})
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID.String()).Msg("ошибка переключения sandbox-режима")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"is_sandbox": resp.IsSandbox,
	})
}

// UpdateProfile обрабатывает PUT /profile
func (h *ProfileHandlers) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}

	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if len(req.ContactPerson) > 255 {
		respondError(w, shared.ErrInvalidInput("Поле contact_person не должно превышать 255 символов"))
		return
	}
	if len(req.Phone) > 50 {
		respondError(w, shared.ErrInvalidInput("Поле phone не должно превышать 50 символов"))
		return
	}

	_, err := h.clientClient.UpdateClient(r.Context(), &clientv1.UpdateClientRequest{
		ClientId:      clientID.String(),
		ContactPerson: req.ContactPerson,
		Phone:         req.Phone,
	})
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID.String()).Msg("ошибка обновления профиля")
		respondGRPCError(w, err)
		return
	}

	// Публикуем audit event
	event := audit.NewAuditEvent(clientID.String(), userID.String(), audit.ActionProfileUpdated, audit.ResourceProfile, clientID.String())
	event.IPAddress = getIPAddress(r)
	event.Details = map[string]any{
		"contact_person": req.ContactPerson,
		"phone":          req.Phone,
	}
	if err := h.auditPublisher.Publish(r.Context(), event); err != nil {
		log.Error().Err(err).Msg("ошибка публикации audit event")
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Профиль успешно обновлён",
	})
}

// SetupTOTP обрабатывает POST /profile/2fa/setup
func (h *ProfileHandlers) SetupTOTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}

	resp, err := h.authClient.SetupTOTP(r.Context(), &authv1.SetupTOTPRequest{
		UserId: userID.String(),
	})
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("ошибка настройки TOTP")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"secret":         resp.Secret,
		"qr_code_url":   resp.QrCodeUrl,
		"recovery_codes": resp.RecoveryCodes,
	})
}

// VerifyTOTP обрабатывает POST /profile/2fa/verify
func (h *ProfileHandlers) VerifyTOTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}

	clientID, _ := middleware.GetClientID(r.Context())

	var req verifyTOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.TOTPCode == "" {
		respondError(w, shared.ErrInvalidInput("Поле totp_code обязательно"))
		return
	}

	resp, err := h.authClient.VerifyTOTP(r.Context(), &authv1.VerifyTOTPRequest{
		UserId:   userID.String(),
		TotpCode: req.TOTPCode,
	})
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("ошибка верификации TOTP")
		respondGRPCError(w, err)
		return
	}

	if !resp.Success {
		respondError(w, shared.ErrInvalidInput("Неверный TOTP код"))
		return
	}

	// Публикуем audit event
	event := audit.NewAuditEvent(clientID.String(), userID.String(), audit.ActionTOTPEnabled, audit.ResourceAuth, userID.String())
	event.IPAddress = getIPAddress(r)
	if err := h.auditPublisher.Publish(r.Context(), event); err != nil {
		log.Error().Err(err).Msg("ошибка публикации audit event")
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"totp_enabled": resp.TotpEnabled,
		"message":      "2FA успешно включена",
	})
}

// DisableTOTP обрабатывает DELETE /profile/2fa
func (h *ProfileHandlers) DisableTOTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}

	clientID, _ := middleware.GetClientID(r.Context())

	var req disableTOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.Password == "" {
		respondError(w, shared.ErrInvalidInput("Поле password обязательно"))
		return
	}

	resp, err := h.authClient.DisableTOTP(r.Context(), &authv1.DisableTOTPRequest{
		UserId:   userID.String(),
		Password: req.Password,
	})
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("ошибка отключения TOTP")
		respondGRPCError(w, err)
		return
	}

	if !resp.Success {
		respondError(w, shared.ErrInvalidInput("Не удалось отключить 2FA"))
		return
	}

	// Публикуем audit event
	event := audit.NewAuditEvent(clientID.String(), userID.String(), audit.ActionTOTPDisabled, audit.ResourceAuth, userID.String())
	event.IPAddress = getIPAddress(r)
	if err := h.auditPublisher.Publish(r.Context(), event); err != nil {
		log.Error().Err(err).Msg("ошибка публикации audit event")
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "2FA успешно отключена",
	})
}

// changePasswordRequest представляет запрос на смену пароля
type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePassword обрабатывает PUT /profile/password
func (h *ProfileHandlers) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}

	clientID, _ := middleware.GetClientID(r.Context())

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.CurrentPassword == "" {
		respondError(w, shared.ErrInvalidInput("Поле current_password обязательно"))
		return
	}
	if req.NewPassword == "" {
		respondError(w, shared.ErrInvalidInput("Поле new_password обязательно"))
		return
	}
	if len(req.NewPassword) < 8 {
		respondError(w, shared.ErrInvalidInput("Новый пароль должен содержать минимум 8 символов"))
		return
	}

	_, err := h.authClient.ChangePassword(r.Context(), &authv1.ChangePasswordRequest{
		UserId:          userID.String(),
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	if h.auditPublisher != nil {
		event := audit.NewAuditEvent(clientID.String(), userID.String(), "password.changed", audit.ResourceAuth, userID.String())
		event.IPAddress = getIPAddress(r)
		if err := h.auditPublisher.Publish(r.Context(), event); err != nil {
			log.Error().Err(err).Msg("ошибка публикации audit event")
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"message": "Пароль успешно изменён"})
}
