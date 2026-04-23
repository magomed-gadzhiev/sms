// internal/gateway/portal/handlers/settings.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type SettingsHandlers struct {
	pool *pgxpool.Pool
}

func NewSettingsHandlers(pool *pgxpool.Pool) *SettingsHandlers {
	return &SettingsHandlers{pool: pool}
}

// --- Frequency Caps ---

type FrequencyCapRequest struct {
	CapType     string `json:"cap_type"`
	MaxMessages int    `json:"max_messages"`
	PeriodHours int    `json:"period_hours"`
	Enabled     bool   `json:"enabled"`
}

func (h *SettingsHandlers) GetFrequencyCaps(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, cap_type, max_messages, period_hours, enabled FROM frequency_caps WHERE client_id = $1`, clientID)
	if err != nil {
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	defer rows.Close()

	var caps []map[string]interface{}
	for rows.Next() {
		var id, capType string
		var maxMsg, periodH int
		var enabled bool
		rows.Scan(&id, &capType, &maxMsg, &periodH, &enabled)
		caps = append(caps, map[string]interface{}{
			"id": id, "cap_type": capType, "max_messages": maxMsg,
			"period_hours": periodH, "enabled": enabled,
		})
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"caps": caps})
}

func (h *SettingsHandlers) UpsertFrequencyCap(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var req FrequencyCapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат"))
		return
	}

	_, err := h.pool.Exec(r.Context(),
		`INSERT INTO frequency_caps (client_id, cap_type, max_messages, period_hours, enabled)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (client_id, cap_type)
		 DO UPDATE SET max_messages = $3, period_hours = $4, enabled = $5, updated_at = now()`,
		clientID, req.CapType, req.MaxMessages, req.PeriodHours, req.Enabled,
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Quiet Hours ---

type QuietHoursRequest struct {
	Enabled   bool   `json:"enabled"`
	StartHour int    `json:"start_hour"`
	EndHour   int    `json:"end_hour"`
	Action    string `json:"action"`
}

func (h *SettingsHandlers) GetQuietHours(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var enabled bool
	var startH, endH int
	var action string
	err := h.pool.QueryRow(r.Context(),
		`SELECT enabled, start_hour, end_hour, action FROM client_quiet_hours WHERE client_id = $1`, clientID,
	).Scan(&enabled, &startH, &endH, &action)
	if err != nil {
		// No config yet — return defaults
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": false, "start_hour": 22, "end_hour": 8, "action": "postpone",
		})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"enabled": enabled, "start_hour": startH, "end_hour": endH, "action": action,
	})
}

func (h *SettingsHandlers) UpsertQuietHours(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var req QuietHoursRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат"))
		return
	}

	_, err := h.pool.Exec(r.Context(),
		`INSERT INTO client_quiet_hours (client_id, enabled, start_hour, end_hour, action)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (client_id)
		 DO UPDATE SET enabled = $2, start_hour = $3, end_hour = $4, action = $5, updated_at = now()`,
		clientID, req.Enabled, req.StartHour, req.EndHour, req.Action,
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Default Sender Names ---

// GetDefaultSenders handles GET /portal/v1/settings/default-senders
func (h *SettingsHandlers) GetDefaultSenders(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	rows, err := h.pool.Query(r.Context(),
		`SELECT channel, sender_name_id::text FROM default_sender_names WHERE client_id = $1`,
		clientID.String(),
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("Ошибка получения настроек"))
		return
	}
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var channel, senderNameID string
		if err := rows.Scan(&channel, &senderNameID); err == nil {
			result[channel] = senderNameID
		}
	}
	respondJSON(w, http.StatusOK, result)
}

// allowedDefaultSenderChannels — белый список каналов, поддерживаемых таблицей
// default_sender_names (см. CHECK constraint в миграции).
var allowedDefaultSenderChannels = map[string]struct{}{
	"sms":   {},
	"viber": {},
	"max":   {},
}

// SetDefaultSenders handles PUT /portal/v1/settings/default-senders.
// Принимает map channel → sender_name_id. Пустой sender_name_id очищает default
// для канала. Перед записью валидирует:
//  1. channel ∈ {sms, viber, max};
//  2. sender_name принадлежит вызывающему клиенту;
//  3. sender_name.status = approved.
func (h *SettingsHandlers) SetDefaultSenders(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Некорректное тело запроса"))
		return
	}
	ctx := r.Context()
	for channel, senderNameID := range req {
		if channel == "" {
			continue
		}
		if _, allowed := allowedDefaultSenderChannels[channel]; !allowed {
			respondError(w, shared.ErrInvalidInput("неподдерживаемый канал: "+channel))
			return
		}
		if senderNameID == "" {
			if _, err := h.pool.Exec(ctx,
				`DELETE FROM default_sender_names WHERE client_id = $1 AND channel = $2`,
				clientID.String(), channel,
			); err != nil {
				respondError(w, shared.ErrInternalServer("ошибка очистки default sender для канала "+channel))
				return
			}
			continue
		}

		var (
			ownerID string
			status  string
		)
		err := h.pool.QueryRow(ctx,
			`SELECT client_id::text, status FROM sender_names WHERE id = $1::uuid`,
			senderNameID,
		).Scan(&ownerID, &status)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("имя отправителя не найдено: "+senderNameID))
			return
		}
		if ownerID != clientID.String() {
			respondError(w, shared.ErrInvalidInput("имя отправителя не принадлежит вашему аккаунту"))
			return
		}
		if status != "approved" {
			respondError(w, shared.ErrInvalidInput(
				"имя отправителя должно быть в статусе approved (текущий: "+status+")"))
			return
		}

		if _, err := h.pool.Exec(ctx, `
			INSERT INTO default_sender_names (client_id, channel, sender_name_id)
			VALUES ($1, $2, $3::uuid)
			ON CONFLICT (client_id, channel)
			DO UPDATE SET sender_name_id = $3::uuid
		`, clientID.String(), channel, senderNameID); err != nil {
			respondError(w, shared.ErrInternalServer("ошибка сохранения default для канала "+channel))
			return
		}
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
