package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// defaultEventTypes lists the 6 canonical notification event types.
var defaultEventTypes = []string{
	"campaign_completed",
	"campaign_failed",
	"low_balance",
	"sender_name_approved",
	"sender_name_rejected",
	"balance_topped_up",
}

// NotifSetting represents one event-type row.
type NotifSetting struct {
	EventType string `json:"event_type"`
	InApp     bool   `json:"in_app"`
	Email     bool   `json:"email"`
}

// notifSettingsResponse is the GET response shape.
type notifSettingsResponse struct {
	Settings    []NotifSetting `json:"settings"`
	ExtraEmails []string       `json:"extra_emails"`
}

// notifSettingsPutRequest is the PUT request body shape.
type notifSettingsPutRequest struct {
	Settings    []NotifSetting `json:"settings"`
	ExtraEmails []string       `json:"extra_emails"`
}

// NotificationSettingsHandlers handles per-user notification preference endpoints.
type NotificationSettingsHandlers struct {
	db *pgxpool.Pool
}

// NewNotificationSettingsHandlers creates a new NotificationSettingsHandlers.
func NewNotificationSettingsHandlers(db *pgxpool.Pool) *NotificationSettingsHandlers {
	return &NotificationSettingsHandlers{db: db}
}

// GetSettings handles GET /portal/v1/settings/notifications
func (h *NotificationSettingsHandlers) GetSettings(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}

	ctx := r.Context()

	// Fetch existing settings rows.
	rows, err := h.db.Query(ctx,
		`SELECT event_type, in_app, email FROM notification_settings WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		log.Error().Err(err).Msg("notification_settings: ошибка запроса настроек")
		respondError(w, shared.ErrInternalServer("Ошибка получения настроек уведомлений"))
		return
	}
	defer rows.Close()

	existing := make(map[string]NotifSetting)
	for rows.Next() {
		var s NotifSetting
		if err := rows.Scan(&s.EventType, &s.InApp, &s.Email); err != nil {
			log.Error().Err(err).Msg("notification_settings: ошибка сканирования строки")
			continue
		}
		existing[s.EventType] = s
	}
	if err := rows.Err(); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка итерации настроек"))
		return
	}

	// Build full list — use DB value if present, otherwise defaults.
	settings := make([]NotifSetting, 0, len(defaultEventTypes))
	for _, et := range defaultEventTypes {
		if s, found := existing[et]; found {
			settings = append(settings, s)
		} else {
			settings = append(settings, NotifSetting{EventType: et, InApp: true, Email: false})
		}
	}

	// Fetch extra emails.
	emailRows, err := h.db.Query(ctx,
		`SELECT email FROM notification_extra_emails WHERE user_id = $1 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		log.Error().Err(err).Msg("notification_settings: ошибка запроса extra emails")
		respondError(w, shared.ErrInternalServer("Ошибка получения дополнительных email"))
		return
	}
	defer emailRows.Close()

	extraEmails := []string{}
	for emailRows.Next() {
		var email string
		if err := emailRows.Scan(&email); err != nil {
			log.Error().Err(err).Msg("notification_settings: ошибка сканирования email")
			continue
		}
		extraEmails = append(extraEmails, email)
	}
	if err := emailRows.Err(); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка итерации email"))
		return
	}

	respondJSON(w, http.StatusOK, notifSettingsResponse{
		Settings:    settings,
		ExtraEmails: extraEmails,
	})
}

// PutSettings handles PUT /portal/v1/settings/notifications
func (h *NotificationSettingsHandlers) PutSettings(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}

	var req notifSettingsPutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Некорректный JSON"))
		return
	}

	ctx := r.Context()

	tx, err := h.db.Begin(ctx)
	if err != nil {
		log.Error().Err(err).Msg("notification_settings: ошибка начала транзакции")
		respondError(w, shared.ErrInternalServer("Ошибка сохранения настроек"))
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Upsert each setting row.
	for _, s := range req.Settings {
		_, err := tx.Exec(ctx,
			`INSERT INTO notification_settings (user_id, event_type, in_app, email)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (user_id, event_type) DO UPDATE
			 SET in_app = EXCLUDED.in_app, email = EXCLUDED.email`,
			userID, s.EventType, s.InApp, s.Email,
		)
		if err != nil {
			log.Error().Err(err).Str("event_type", s.EventType).Msg("notification_settings: ошибка upsert настройки")
			respondError(w, shared.ErrInternalServer("Ошибка сохранения настроек"))
			return
		}
	}

	// Replace extra emails atomically: delete all, then re-insert.
	if _, err := tx.Exec(ctx,
		`DELETE FROM notification_extra_emails WHERE user_id = $1`,
		userID,
	); err != nil {
		log.Error().Err(err).Msg("notification_settings: ошибка удаления extra emails")
		respondError(w, shared.ErrInternalServer("Ошибка сохранения email"))
		return
	}

	for _, email := range req.ExtraEmails {
		if _, err := tx.Exec(ctx,
			`INSERT INTO notification_extra_emails (user_id, email) VALUES ($1, $2)
			 ON CONFLICT (user_id, email) DO NOTHING`,
			userID, email,
		); err != nil {
			log.Error().Err(err).Str("email", email).Msg("notification_settings: ошибка вставки extra email")
			respondError(w, shared.ErrInternalServer("Ошибка сохранения email"))
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Error().Err(err).Msg("notification_settings: ошибка коммита транзакции")
		respondError(w, shared.ErrInternalServer("Ошибка сохранения настроек"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
