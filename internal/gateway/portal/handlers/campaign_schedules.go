package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type CampaignScheduleHandlers struct {
	db *pgxpool.Pool
}

func NewCampaignScheduleHandlers(db *pgxpool.Pool) *CampaignScheduleHandlers {
	return &CampaignScheduleHandlers{db: db}
}

type CampaignSchedule struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	TemplateCampaignID string  `json:"template_campaign_id"`
	Frequency          string  `json:"frequency"`
	CronExpression     *string `json:"cron_expression,omitempty"`
	NextRunAt          *string `json:"next_run_at,omitempty"`
	LastRunAt          *string `json:"last_run_at,omitempty"`
	IsActive           bool    `json:"is_active"`
	RunCount           int     `json:"run_count"`
	MaxRuns            *int    `json:"max_runs,omitempty"`
	CreatedAt          string  `json:"created_at"`
}

func (h *CampaignScheduleHandlers) List(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	rows, err := h.db.Query(r.Context(), `
		SELECT id::text, name, template_campaign_id::text, frequency,
			cron_expression, next_run_at::text, last_run_at::text,
			is_active, run_count, max_runs, created_at::text
		FROM campaign_schedules
		WHERE client_id = $1
		ORDER BY created_at DESC
	`, clientID.String())
	if err != nil {
		respondError(w, shared.ErrInternalServer("Ошибка получения расписаний"))
		return
	}
	defer rows.Close()
	schedules := make([]CampaignSchedule, 0)
	for rows.Next() {
		var s CampaignSchedule
		if err := rows.Scan(
			&s.ID, &s.Name, &s.TemplateCampaignID, &s.Frequency,
			&s.CronExpression, &s.NextRunAt, &s.LastRunAt,
			&s.IsActive, &s.RunCount, &s.MaxRuns, &s.CreatedAt,
		); err != nil {
			continue
		}
		schedules = append(schedules, s)
	}
	if err := rows.Err(); err != nil {
		respondError(w, shared.ErrInternalServer("Ошибка итерации строк"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"schedules": schedules})
}

type createScheduleRequest struct {
	Name               string  `json:"name"`
	TemplateCampaignID string  `json:"template_campaign_id"`
	Frequency          string  `json:"frequency"`
	CronExpression     *string `json:"cron_expression,omitempty"`
	MaxRuns            *int    `json:"max_runs,omitempty"`
}

func (h *CampaignScheduleHandlers) Create(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	var req createScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Некорректное тело запроса"))
		return
	}
	if req.Name == "" || req.TemplateCampaignID == "" || req.Frequency == "" {
		respondError(w, shared.ErrInvalidInput("name, template_campaign_id и frequency обязательны"))
		return
	}
	id := uuid.New()
	_, err := h.db.Exec(r.Context(), `
		INSERT INTO campaign_schedules (id, client_id, name, template_campaign_id, frequency, cron_expression, max_runs, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, true)
	`, id, clientID.String(), req.Name, req.TemplateCampaignID, req.Frequency, req.CronExpression, req.MaxRuns)
	if err != nil {
		respondError(w, shared.ErrInternalServer("Ошибка создания расписания"))
		return
	}
	respondJSON(w, http.StatusCreated, map[string]string{"id": id.String()})
}

func (h *CampaignScheduleHandlers) Toggle(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Некорректное тело запроса"))
		return
	}
	res, err := h.db.Exec(r.Context(), `
		UPDATE campaign_schedules SET is_active = $1, updated_at = NOW()
		WHERE id = $2::uuid AND client_id = $3
	`, req.IsActive, id, clientID.String())
	if err != nil {
		respondError(w, shared.ErrInternalServer("Ошибка обновления расписания"))
		return
	}
	if res.RowsAffected() == 0 {
		respondError(w, shared.ErrNotFound("Расписание не найдено"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *CampaignScheduleHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	res, err := h.db.Exec(r.Context(), `
		DELETE FROM campaign_schedules WHERE id = $1::uuid AND client_id = $2
	`, id, clientID.String())
	if err != nil {
		respondError(w, shared.ErrInternalServer("Ошибка удаления расписания"))
		return
	}
	if res.RowsAffected() == 0 {
		respondError(w, shared.ErrNotFound("Расписание не найдено"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
