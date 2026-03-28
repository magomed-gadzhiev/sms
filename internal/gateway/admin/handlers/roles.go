package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// RoleHandlers обрабатывает HTTP запросы для управления ролями
type RoleHandlers struct {
	authClient authv1.AuthServiceClient
}

// NewRoleHandlers создает новый экземпляр RoleHandlers
func NewRoleHandlers(authClient authv1.AuthServiceClient) *RoleHandlers {
	return &RoleHandlers{
		authClient: authClient,
	}
}

// ListRoles обрабатывает GET /admin/v1/roles
func (h *RoleHandlers) ListRoles(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)

	resp, err := h.authClient.ListRoles(r.Context(), &authv1.ListRolesRequest{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка ролей")
		respondGRPCError(w, err)
		return
	}

	roles := make([]map[string]interface{}, len(resp.Roles))
	for i, role := range resp.Roles {
		roles[i] = roleDetailToMap(role)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"roles": roles,
		"total": resp.Total,
	})
}

// GetRole обрабатывает GET /admin/v1/roles/:id
func (h *RoleHandlers) GetRole(w http.ResponseWriter, r *http.Request) {
	roleID := mux.Vars(r)["id"]
	if roleID == "" {
		respondError(w, shared.ErrInvalidInput("role_id обязателен"))
		return
	}

	resp, err := h.authClient.GetRole(r.Context(), &authv1.GetRoleRequest{
		RoleId: roleID,
	})
	if err != nil {
		log.Error().Err(err).Str("role_id", roleID).Msg("ошибка получения роли")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, roleDetailToMap(resp.Role))
}

// CreateRole обрабатывает POST /admin/v1/roles
func (h *RoleHandlers) CreateRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string   `json:"name"`
		Description   string   `json:"description"`
		PermissionIDs []string `json:"permission_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("name обязателен"))
		return
	}

	resp, err := h.authClient.CreateRole(r.Context(), &authv1.CreateRoleRequest{
		Name:          req.Name,
		Description:   req.Description,
		PermissionIds: req.PermissionIDs,
	})
	if err != nil {
		log.Error().Err(err).Str("name", req.Name).Msg("ошибка создания роли")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, roleDetailToMap(resp.Role))
}

// UpdateRole обрабатывает PUT /admin/v1/roles/:id
func (h *RoleHandlers) UpdateRole(w http.ResponseWriter, r *http.Request) {
	roleID := mux.Vars(r)["id"]
	if roleID == "" {
		respondError(w, shared.ErrInvalidInput("role_id обязателен"))
		return
	}

	var req struct {
		Name          string   `json:"name"`
		Description   string   `json:"description"`
		PermissionIDs []string `json:"permission_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	resp, err := h.authClient.UpdateRole(r.Context(), &authv1.UpdateRoleRequest{
		RoleId:        roleID,
		Name:          req.Name,
		Description:   req.Description,
		PermissionIds: req.PermissionIDs,
	})
	if err != nil {
		log.Error().Err(err).Str("role_id", roleID).Msg("ошибка обновления роли")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, roleDetailToMap(resp.Role))
}

// DeleteRole обрабатывает DELETE /admin/v1/roles/:id
func (h *RoleHandlers) DeleteRole(w http.ResponseWriter, r *http.Request) {
	roleID := mux.Vars(r)["id"]
	if roleID == "" {
		respondError(w, shared.ErrInvalidInput("role_id обязателен"))
		return
	}

	resp, err := h.authClient.DeleteRole(r.Context(), &authv1.DeleteRoleRequest{
		RoleId: roleID,
	})
	if err != nil {
		log.Error().Err(err).Str("role_id", roleID).Msg("ошибка удаления роли")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": resp.Success,
	})
}

// ListPermissions обрабатывает GET /admin/v1/permissions
func (h *RoleHandlers) ListPermissions(w http.ResponseWriter, r *http.Request) {
	resp, err := h.authClient.ListAllPermissions(r.Context(), &authv1.ListAllPermissionsRequest{})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка прав")
		respondGRPCError(w, err)
		return
	}

	permissions := make([]map[string]interface{}, len(resp.Permissions))
	for i, p := range resp.Permissions {
		permissions[i] = map[string]interface{}{
			"id":       p.Id,
			"resource": p.Resource,
			"action":   p.Action,
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"permissions": permissions,
	})
}

func roleDetailToMap(role *authv1.RoleDetail) map[string]interface{} {
	result := map[string]interface{}{
		"id":          role.Id,
		"name":        role.Name,
		"description": role.Description,
		"builtin":     role.Builtin,
		"user_count":  role.UserCount,
	}

	permissions := make([]map[string]interface{}, len(role.Permissions))
	for i, p := range role.Permissions {
		permissions[i] = map[string]interface{}{
			"id":       p.Id,
			"resource": p.Resource,
			"action":   p.Action,
		}
	}
	result["permissions"] = permissions

	if role.CreatedAt != nil {
		result["created_at"] = role.CreatedAt.AsTime()
	}
	if role.UpdatedAt != nil {
		result["updated_at"] = role.UpdatedAt.AsTime()
	}
	return result
}
