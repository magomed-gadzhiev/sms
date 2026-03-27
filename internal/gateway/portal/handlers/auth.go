package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/mail"
	"os"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/audit"
)

// isSecureCookie returns true if cookies should use Secure flag (HTTPS only).
func isSecureCookie() bool {
	env := os.Getenv("COOKIE_SECURE")
	if env == "false" || env == "0" {
		return false
	}
	// Default: secure in production, check SERVICE_ENV too
	svcEnv := os.Getenv("SERVICE_ENV")
	return svcEnv != "development" && svcEnv != "dev"
}

// AuthHandlers содержит handlers для аутентификации через портал
type AuthHandlers struct {
	authClient     authv1.AuthServiceClient
	auditPublisher *audit.Publisher
}

// NewAuthHandlers создает новый AuthHandlers
func NewAuthHandlers(authClient authv1.AuthServiceClient, auditPublisher *audit.Publisher) *AuthHandlers {
	return &AuthHandlers{
		authClient:     authClient,
		auditPublisher: auditPublisher,
	}
}

// loginRequest представляет запрос на вход в систему
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// login2FARequest представляет запрос на 2FA вход
type login2FARequest struct {
	LoginTicket string `json:"login_ticket"`
	TOTPCode    string `json:"totp_code"`
}

// passwordResetRequestBody представляет запрос на сброс пароля
type passwordResetRequestBody struct {
	Email string `json:"email"`
}

// resetPasswordBody представляет запрос на установку нового пароля
type resetPasswordBody struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// registerRequest представляет запрос на регистрацию нового клиента
type registerRequest struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	CompanyName   string `json:"company_name"`
	ContactPerson string `json:"contact_person"`
	Phone         string `json:"phone"`
	PlanName      string `json:"plan_name"`
}

// Login обрабатывает POST /auth/login
func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.Email == "" || req.Password == "" {
		respondError(w, shared.ErrInvalidInput("Поля email и password обязательны"))
		return
	}

	resp, err := h.authClient.LoginWithSession(r.Context(), &authv1.LoginWithSessionRequest{
		Email:     req.Email,
		Password:  req.Password,
		IpAddress: getIPAddress(r),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		log.Error().Err(err).Str("email", req.Email).Msg("ошибка входа в систему")
		respondGRPCError(w, err)
		return
	}

	// Если требуется 2FA — возвращаем тикет
	if resp.Requires_2Fa {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"requires_2fa": true,
			"login_ticket": resp.LoginTicket,
		})
		return
	}

	// Успешный вход — устанавливаем cookies
	setSessionCookies(w, resp.SessionId)

	// Публикуем audit event
	h.publishAuditEvent(r, resp.User, audit.ActionLogin, "")

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"user": buildUserInfoResponse(resp.User),
	})
}

// LoginWith2FA обрабатывает POST /auth/login/2fa
func (h *AuthHandlers) LoginWith2FA(w http.ResponseWriter, r *http.Request) {
	var req login2FARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.LoginTicket == "" || req.TOTPCode == "" {
		respondError(w, shared.ErrInvalidInput("Поля login_ticket и totp_code обязательны"))
		return
	}

	resp, err := h.authClient.LoginWithSession(r.Context(), &authv1.LoginWithSessionRequest{
		Email:     req.LoginTicket, // login_ticket передается через email при 2FA flow
		TotpCode:  req.TOTPCode,
		IpAddress: getIPAddress(r),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка 2FA входа в систему")
		respondGRPCError(w, err)
		return
	}

	// Устанавливаем cookies
	setSessionCookies(w, resp.SessionId)

	// Публикуем audit event
	h.publishAuditEvent(r, resp.User, audit.ActionLogin, "")

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"user": buildUserInfoResponse(resp.User),
	})
}

// Logout обрабатывает POST /auth/logout
func (h *AuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("portal_session")
	if err != nil || cookie.Value == "" {
		respondError(w, shared.ErrUnauthorized("Сессия не найдена"))
		return
	}

	sessionID := cookie.Value

	_, err = h.authClient.Logout(r.Context(), &authv1.LogoutRequest{
		SessionId: sessionID,
	})
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionID).Msg("ошибка выхода из системы")
		respondGRPCError(w, err)
		return
	}

	// Очищаем cookies
	clearSessionCookies(w)

	// Публикуем audit event
	h.publishAuditEvent(r, nil, audit.ActionLogout, "")

	w.WriteHeader(http.StatusNoContent)
}

// RequestPasswordReset обрабатывает POST /auth/password/reset-request
func (h *AuthHandlers) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req passwordResetRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.Email == "" {
		respondError(w, shared.ErrInvalidInput("Поле email обязательно"))
		return
	}

	// Вызываем сервис, но всегда возвращаем 202 для предотвращения перечисления
	_, err := h.authClient.RequestPasswordReset(r.Context(), &authv1.RequestPasswordResetRequest{
		Email: req.Email,
	})
	if err != nil {
		log.Error().Err(err).Str("email", req.Email).Msg("ошибка запроса сброса пароля")
		// Не раскрываем ошибку клиенту
	}

	respondJSON(w, http.StatusAccepted, map[string]interface{}{
		"message": "Если аккаунт с таким email существует, письмо с инструкциями по сбросу пароля отправлено",
	})
}

// ResetPassword обрабатывает POST /auth/password/reset
func (h *AuthHandlers) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.Token == "" || req.NewPassword == "" {
		respondError(w, shared.ErrInvalidInput("Поля token и new_password обязательны"))
		return
	}

	_, err := h.authClient.ResetPassword(r.Context(), &authv1.ResetPasswordRequest{
		Token:       req.Token,
		NewPassword: req.NewPassword,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка сброса пароля")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Пароль успешно изменён",
	})
}

// Register обрабатывает POST /auth/register
func (h *AuthHandlers) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.Email == "" {
		respondError(w, shared.ErrInvalidInput("Поле email обязательно"))
		return
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат email"))
		return
	}
	if len(req.Password) < 8 {
		respondError(w, shared.ErrInvalidInput("Пароль должен содержать не менее 8 символов"))
		return
	}
	if req.CompanyName == "" {
		respondError(w, shared.ErrInvalidInput("Поле company_name обязательно"))
		return
	}
	if len(req.CompanyName) > 500 {
		respondError(w, shared.ErrInvalidInput("Название компании не может превышать 500 символов"))
		return
	}

	resp, err := h.authClient.RegisterClient(r.Context(), &authv1.RegisterClientRequest{
		Email:         req.Email,
		Password:      req.Password,
		CompanyName:   req.CompanyName,
		ContactPerson: req.ContactPerson,
		Phone:         req.Phone,
		PlanName:      req.PlanName,
	})
	if err != nil {
		log.Error().Err(err).Str("email", req.Email).Msg("ошибка регистрации клиента")
		respondGRPCError(w, err)
		return
	}

	// Устанавливаем cookies сессии
	setSessionCookies(w, resp.SessionId)

	// Публикуем audit event
	h.publishAuditEvent(r, resp.User, audit.ActionRegister, resp.ClientId)

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"client_id": resp.ClientId,
		"user":      buildUserInfoResponse(resp.User),
	})
}

// setSessionCookies устанавливает cookies для сессии портала
func setSessionCookies(w http.ResponseWriter, sessionID string) {
	secure := isSecureCookie()
	sameSite := http.SameSiteStrictMode
	if !secure {
		sameSite = http.SameSiteLaxMode
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "portal_session",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})

	// Generate a separate random CSRF token (not linked to session ID)
	csrfBytes := make([]byte, 32)
	if _, err := rand.Read(csrfBytes); err != nil {
		// crypto/rand failure is a critical system error
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	csrfToken := hex.EncodeToString(csrfBytes)

	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    csrfToken,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: false,
		Secure:   secure,
		SameSite: sameSite,
	})
}

// clearSessionCookies очищает cookies сессии
func clearSessionCookies(w http.ResponseWriter) {
	secure := isSecureCookie()
	sameSite := http.SameSiteStrictMode
	if !secure {
		sameSite = http.SameSiteLaxMode
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "portal_session",
		Value:    "",
		Path:     "/",
		MaxAge:   0,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    "",
		Path:     "/",
		MaxAge:   0,
		HttpOnly: false,
		Secure:   secure,
		SameSite: sameSite,
	})
}

// getIPAddress извлекает IP адрес клиента из запроса
func getIPAddress(r *http.Request) string {
	// Проверяем X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	// Проверяем X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Используем RemoteAddr (отсекаем порт)
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// buildUserInfoResponse формирует ответ с информацией о пользователе
func buildUserInfoResponse(user *authv1.UserInfo) map[string]interface{} {
	if user == nil {
		return nil
	}
	result := map[string]interface{}{
		"id":       user.Id,
		"username": user.Username,
		"email":    user.Email,
		"active":   user.Active,
	}
	if user.Role != nil {
		result["role"] = user.Role.Name
	}
	if user.CreatedAt != nil {
		result["created_at"] = user.CreatedAt.AsTime()
	}
	return result
}

// publishAuditEvent публикует audit event
func (h *AuthHandlers) publishAuditEvent(r *http.Request, user *authv1.UserInfo, action string, resourceID string) {
	if h.auditPublisher == nil {
		return
	}

	userID := ""
	tenantID := ""
	if user != nil {
		userID = user.Id
	}

	event := audit.NewAuditEvent(tenantID, userID, action, audit.ResourceAuth, resourceID)
	event.IPAddress = getIPAddress(r)

	if err := h.auditPublisher.Publish(r.Context(), event); err != nil {
		log.Error().Err(err).Str("action", action).Msg("ошибка публикации audit event")
	}
}
