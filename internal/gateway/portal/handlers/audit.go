package handlers

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/auditv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// AuditHandlers содержит handlers для аудит-логов
type AuditHandlers struct {
	auditClient auditv1.AuditServiceClient
}

// NewAuditHandlers создает новый AuditHandlers
func NewAuditHandlers(auditClient auditv1.AuditServiceClient) *AuditHandlers {
	return &AuditHandlers{
		auditClient: auditClient,
	}
}

// ListAuditLog обрабатывает GET /audit-log
func (h *AuditHandlers) ListAuditLog(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	query := r.URL.Query()
	page, perPage := parsePagination(r)

	// Построение запроса
	req := &auditv1.QueryAuditLogRequest{
		TenantId:     clientID.String(),
		Action:       query.Get("action"),
		UserId:       query.Get("user_id"),
		ResourceType: query.Get("resource_type"),
		Page:         page,
		PerPage:      perPage,
	}

	// Парсим даты
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
		// Конец дня
		req.DateTo = timestamppb.New(parsed.Add(24*time.Hour - time.Second))
	}

	if h.auditClient == nil {
		// Audit gRPC service не подключен — возвращаем пустой результат
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"entries":     []interface{}{},
			"total":       0,
			"page":        page,
			"total_pages": 0,
		})
		return
	}

	resp, err := h.auditClient.QueryAuditLog(r.Context(), req)
	if err != nil {
		log.Error().Err(err).Msg("ошибка запроса аудит-лога")
		respondGRPCError(w, err)
		return
	}

	// Преобразуем в JSON-ответ
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
