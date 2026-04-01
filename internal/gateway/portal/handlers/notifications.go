package handlers

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

var notificationsCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "portal_notifications_created_total",
	Help: "Total number of portal notifications created",
})

// NotificationHandlers содержит handlers для центра уведомлений
type NotificationHandlers struct {
	pool *pgxpool.Pool
}

// NewNotificationHandlers создает NotificationHandlers
func NewNotificationHandlers(pool *pgxpool.Pool) *NotificationHandlers {
	return &NotificationHandlers{pool: pool}
}

// IncrementNotificationsCreated позволяет планировщику инкрементировать счётчик
func IncrementNotificationsCreated() {
	notificationsCreatedTotal.Inc()
}

// GetNotifications обрабатывает GET /notifications
func (h *NotificationHandlers) GetNotifications(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}

	if h.pool == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"items": []interface{}{}, "unread_count": 0,
		})
		return
	}

	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(q.Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	ctx := r.Context()

	rows, err := h.pool.Query(ctx,
		`SELECT id, type, body, object_type, object_id, is_read, created_at
		 FROM notifications
		 WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		userID, perPage, offset,
	)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения уведомлений")
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	defer rows.Close()

	type notificationItem struct {
		ID         string  `json:"id"`
		Type       string  `json:"type"`
		Body       string  `json:"body"`
		ObjectType *string `json:"object_type,omitempty"`
		ObjectID   *string `json:"object_id,omitempty"`
		IsRead     bool    `json:"is_read"`
		CreatedAt  string  `json:"created_at"`
	}

	items := make([]notificationItem, 0)
	for rows.Next() {
		var n notificationItem
		var createdAt interface{}
		if err := rows.Scan(&n.ID, &n.Type, &n.Body, &n.ObjectType, &n.ObjectID, &n.IsRead, &createdAt); err != nil {
			continue
		}
		if t, ok := createdAt.(interface{ Format(string) string }); ok {
			n.CreatedAt = t.Format("2006-01-02T15:04:05Z07:00")
		} else {
			n.CreatedAt = ""
		}
		items = append(items, n)
	}

	var unreadCount int
	h.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = false`,
		userID,
	).Scan(&unreadCount)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"items":        items,
		"unread_count": unreadCount,
	})
}

// MarkNotificationRead обрабатывает POST /notifications/{id}/read
func (h *NotificationHandlers) MarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}

	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID уведомления обязателен"))
		return
	}

	if h.pool == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
		return
	}

	tag, err := h.pool.Exec(r.Context(),
		`UPDATE notifications SET is_read = true WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		log.Error().Err(err).Msg("ошибка обновления уведомления")
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	if tag.RowsAffected() == 0 {
		respondError(w, shared.ErrNotFound("Уведомление не найдено"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// MarkAllNotificationsRead обрабатывает POST /notifications/read-all
func (h *NotificationHandlers) MarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}

	if h.pool == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
		return
	}

	_, err := h.pool.Exec(r.Context(),
		`UPDATE notifications SET is_read = true WHERE user_id = $1 AND is_read = false`,
		userID,
	)
	if err != nil {
		log.Error().Err(err).Msg("ошибка массового обновления уведомлений")
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
