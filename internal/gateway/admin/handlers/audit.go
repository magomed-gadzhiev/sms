package handlers

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/auditv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// AdminAuditHandlers содержит handlers для аудит-логов (admin)
type AdminAuditHandlers struct {
	auditClient auditv1.AuditServiceClient
}

// NewAdminAuditHandlers создает новый AdminAuditHandlers
func NewAdminAuditHandlers(auditClient auditv1.AuditServiceClient) *AdminAuditHandlers {
	return &AdminAuditHandlers{
		auditClient: auditClient,
	}
}

// ListAuditLog обрабатывает GET /admin/v1/audit
// Admin видит все записи (без фильтрации по tenant_id).
//
// BUG-56: до фикса при `auditClient == nil` хендлер тихо возвращал пустой
// список с HTTP 200 и без валидации параметров. Это маскировало конфигурационный
// баг (отсутствие AUDIT_SERVICE_ADDR в admin-gateway env, BUG-55) — UI показывал
// "записей нет" вместо "сервис недоступен". Теперь возвращаем 503 явно, чтобы
// конфигурационная ошибка была видна сразу.
func (h *AdminAuditHandlers) ListAuditLog(w http.ResponseWriter, r *http.Request) {
	if h.auditClient == nil {
		log.Error().Msg("AdminAuditHandlers: auditClient не инициализирован — проверьте AUDIT_SERVICE_ADDR в env admin-gateway")
		respondError(w, shared.ErrServiceUnavailable("Audit service не настроен"))
		return
	}

	query := r.URL.Query()
	page, perPage := parsePagination(r)

	clientID := query.Get("client_id")
	// gRPC AuditService.QueryAuditLog требует tenant_id (см. internal/services/audit/grpc/server.go).
	// Admin global view (без tenant_id) на уровне gRPC не поддерживается — это
	// архитектурное ограничение, аналог BUG-38 (admin global webhook view).
	// Возвращаем понятное 400 вместо проброса сырого gRPC-сообщения.
	if clientID == "" {
		respondError(w, shared.ErrInvalidInput("укажите client_id — admin global view audit-лога не поддерживается"))
		return
	}

	req := &auditv1.QueryAuditLogRequest{
		TenantId: clientID,
		Action:   query.Get("action"),
		UserId:   query.Get("user_id"),
		Page:     page,
		PerPage:  perPage,
	}

	if from := query.Get("date_from"); from != "" {
		parsed, err := time.Parse("2006-01-02", from)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат date_from, ожидается YYYY-MM-DD"))
			return
		}
		req.DateFrom = timestamppb.New(parsed)
	}
	if to := query.Get("date_to"); to != "" {
		parsed, err := time.Parse("2006-01-02", to)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат date_to, ожидается YYYY-MM-DD"))
			return
		}
		req.DateTo = timestamppb.New(parsed.Add(24*time.Hour - time.Second))
	}

	resp, err := h.auditClient.QueryAuditLog(r.Context(), req)
	if err != nil {
		log.Error().Err(err).Msg("ошибка запроса аудит-лога (admin)")
		respondGRPCError(w, err)
		return
	}

	entries := make([]map[string]interface{}, 0, len(resp.Entries))
	for _, e := range resp.Entries {
		entry := map[string]interface{}{
			"id":            e.Id,
			"tenant_id":     e.TenantId,
			"user_id":       e.UserId,
			"action":        e.Action,
			"resource_type": e.ResourceType,
			"resource_id":   e.ResourceId,
			"details":       e.Details,
			"ip_address":    e.IpAddress,
		}
		if e.CreatedAt != nil {
			entry["created_at"] = e.CreatedAt.AsTime().Format(time.RFC3339)
		}
		entries = append(entries, entry)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries":     entries,
		"total":       resp.Total,
		"page":        resp.Page,
		"total_pages": resp.TotalPages,
	})
}
