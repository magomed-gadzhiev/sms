package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// UserHandlers обрабатывает HTTP запросы для управления пользователями
type UserHandlers struct {
	authClient authv1.AuthServiceClient
}

// NewUserHandlers создает новый экземпляр UserHandlers
func NewUserHandlers(authClient authv1.AuthServiceClient) *UserHandlers {
	return &UserHandlers{
		authClient: authClient,
	}
}

// ListUsers обрабатывает GET /admin/v1/users
func (h *UserHandlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	roleID := r.URL.Query().Get("role_id")
	activeOnly := r.URL.Query().Get("active_only") == "true"
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)

	resp, err := h.authClient.ListUsers(r.Context(), &authv1.ListUsersRequest{
		Search:     search,
		RoleId:     roleID,
		ActiveOnly: activeOnly,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка пользователей")
		respondGRPCError(w, err)
		return
	}

	users := make([]map[string]interface{}, len(resp.Users))
	for i, u := range resp.Users {
		users[i] = userDetailToMap(u)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"users":  users,
		"total":  resp.Total,
		"limit":  resp.Limit,
		"offset": resp.Offset,
	})
}

// GetUser обрабатывает GET /admin/v1/users/:id
func (h *UserHandlers) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["id"]
	if userID == "" {
		respondError(w, shared.ErrInvalidInput("user_id обязателен"))
		return
	}

	resp, err := h.authClient.GetUser(r.Context(), &authv1.GetUserRequest{
		UserId: userID,
	})
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("ошибка получения пользователя")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, userDetailToMap(resp.User))
}

// CreateUser обрабатывает POST /admin/v1/users
func (h *UserHandlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		RoleID   string `json:"role_id"`
		Active   bool   `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Username == "" {
		respondError(w, shared.ErrInvalidInput("username обязателен"))
		return
	}
	if req.Email == "" {
		respondError(w, shared.ErrInvalidInput("email обязателен"))
		return
	}
	if req.Password == "" {
		respondError(w, shared.ErrInvalidInput("password обязателен"))
		return
	}

	resp, err := h.authClient.CreateUser(r.Context(), &authv1.CreateUserRequest{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
		RoleId:   req.RoleID,
		Active:   req.Active,
	})
	if err != nil {
		log.Error().Err(err).Str("username", req.Username).Msg("ошибка создания пользователя")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, userInfoToMap(resp.User))
}

// UpdateUser обрабатывает PUT /admin/v1/users/:id
func (h *UserHandlers) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["id"]
	if userID == "" {
		respondError(w, shared.ErrInvalidInput("user_id обязателен"))
		return
	}

	var req struct {
		Email  string `json:"email"`
		RoleID string `json:"role_id"`
		Active bool   `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	resp, err := h.authClient.UpdateUser(r.Context(), &authv1.UpdateUserRequest{
		UserId: userID,
		Email:  req.Email,
		RoleId: req.RoleID,
		Active: req.Active,
	})
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("ошибка обновления пользователя")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, userInfoToMap(resp.User))
}

// DeactivateUser обрабатывает POST /admin/v1/users/:id/deactivate
func (h *UserHandlers) DeactivateUser(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["id"]
	if userID == "" {
		respondError(w, shared.ErrInvalidInput("user_id обязателен"))
		return
	}

	resp, err := h.authClient.DeactivateUser(r.Context(), &authv1.DeactivateUserRequest{
		UserId: userID,
	})
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("ошибка деактивации пользователя")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": resp.Success,
	})
}

// ResetUser2FA обрабатывает POST /admin/v1/users/:id/reset-2fa
func (h *UserHandlers) ResetUser2FA(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["id"]
	if userID == "" {
		respondError(w, shared.ErrInvalidInput("user_id обязателен"))
		return
	}

	resp, err := h.authClient.ResetUser2FA(r.Context(), &authv1.ResetUser2FARequest{
		UserId: userID,
	})
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("ошибка сброса 2FA")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": resp.Success,
	})
}

// ResetUserPassword обрабатывает POST /admin/v1/users/:id/reset-password
func (h *UserHandlers) ResetUserPassword(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["id"]
	if userID == "" {
		respondError(w, shared.ErrInvalidInput("user_id обязателен"))
		return
	}

	resp, err := h.authClient.ResetUserPassword(r.Context(), &authv1.ResetUserPasswordRequest{
		UserId: userID,
	})
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("ошибка сброса пароля")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"temporary_password": resp.TemporaryPassword,
	})
}

func userDetailToMap(u *authv1.UserDetailInfo) map[string]interface{} {
	result := map[string]interface{}{
		"id":           u.Id,
		"username":     u.Username,
		"email":        u.Email,
		"active":       u.Active,
		"totp_enabled": u.TotpEnabled,
	}
	if u.Role != nil {
		result["role"] = map[string]interface{}{
			"id":          u.Role.Id,
			"name":        u.Role.Name,
			"description": u.Role.Description,
		}
	}
	if u.LastLoginAt != nil {
		result["last_login_at"] = u.LastLoginAt.AsTime()
	}
	if u.CreatedAt != nil {
		result["created_at"] = u.CreatedAt.AsTime()
	}
	if u.UpdatedAt != nil {
		result["updated_at"] = u.UpdatedAt.AsTime()
	}
	return result
}

func userInfoToMap(u *authv1.UserInfo) map[string]interface{} {
	result := map[string]interface{}{
		"id":       u.Id,
		"username": u.Username,
		"email":    u.Email,
		"active":   u.Active,
	}
	if u.Role != nil {
		result["role"] = map[string]interface{}{
			"id":          u.Role.Id,
			"name":        u.Role.Name,
			"description": u.Role.Description,
		}
	}
	if u.CreatedAt != nil {
		result["created_at"] = u.CreatedAt.AsTime()
	}
	return result
}
