package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/schedules"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type CampaignScheduleHandlers struct {
	db *pgxpool.Pool
}

func NewCampaignScheduleHandlers(db *pgxpool.Pool) *CampaignScheduleHandlers {
	return &CampaignScheduleHandlers{db: db}
}

type CampaignSchedule struct {
	ID                   string  `json:"id"`
	Name                 string  `json:"name"`
	TemplateCampaignID   string  `json:"template_campaign_id"`
	TemplateCampaignName string  `json:"template_campaign_name"`
	Frequency            string  `json:"frequency"`
	CronExpression       *string `json:"cron_expression,omitempty"`
	NextRunAt            *string `json:"next_run_at,omitempty"`
	LastRunAt            *string `json:"last_run_at,omitempty"`
	IsActive             bool    `json:"is_active"`
	RunCount             int     `json:"run_count"`
	MaxRuns              *int    `json:"max_runs,omitempty"`
	CreatedAt            string  `json:"created_at"`
}

func (h *CampaignScheduleHandlers) List(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	rows, err := h.db.Query(r.Context(), `
		SELECT cs.id::text, cs.name, cs.template_campaign_id::text,
			COALESCE(c.name, ''), cs.frequency,
			cs.cron_expression, cs.next_run_at::text, cs.last_run_at::text,
			cs.is_active, cs.run_count, cs.max_runs, cs.created_at::text
		FROM campaign_schedules cs
		LEFT JOIN campaigns c ON c.id = cs.template_campaign_id
		WHERE cs.client_id = $1
		ORDER BY cs.created_at DESC
	`, clientID.String())
	if err != nil {
		respondError(w, shared.ErrInternalServer("Ошибка получения расписаний"))
		return
	}
	defer rows.Close()
	scheduleList := make([]CampaignSchedule, 0)
	for rows.Next() {
		var s CampaignSchedule
		if err := rows.Scan(
			&s.ID, &s.Name, &s.TemplateCampaignID, &s.TemplateCampaignName,
			&s.Frequency, &s.CronExpression, &s.NextRunAt, &s.LastRunAt,
			&s.IsActive, &s.RunCount, &s.MaxRuns, &s.CreatedAt,
		); err != nil {
			continue
		}
		scheduleList = append(scheduleList, s)
	}
	if err := rows.Err(); err != nil {
		respondError(w, shared.ErrInternalServer("Ошибка итерации строк"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"schedules": scheduleList})
}

type createScheduleRequest struct {
	Name               string  `json:"name"`
	TemplateCampaignID string  `json:"template_campaign_id"`
	Frequency          string  `json:"frequency"`
	CronExpression     *string `json:"cron_expression,omitempty"`
	MaxRuns            *int    `json:"max_runs,omitempty"`
}

var validFrequencies = map[string]bool{
	"daily": true, "weekly": true, "monthly": true, "custom": true,
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

	req.Name = strings.TrimSpace(req.Name)

	// Required field validation
	if req.Name == "" || req.TemplateCampaignID == "" || req.Frequency == "" {
		respondError(w, shared.ErrInvalidInput("name, template_campaign_id и frequency обязательны"))
		return
	}

	// Frequency must be one of the allowed values
	if !validFrequencies[req.Frequency] {
		respondError(w, shared.ErrInvalidInput("frequency: допустимые значения: daily, weekly, monthly, custom"))
		return
	}

	// Cron expression is required for custom frequency and must be valid
	if req.Frequency == "custom" {
		if req.CronExpression == nil || *req.CronExpression == "" {
			respondError(w, shared.ErrInvalidInput("cron_expression обязателен для частоты 'custom'"))
			return
		}
		if err := schedules.ValidateCronExpression(*req.CronExpression); err != nil {
			respondError(w, shared.ErrInvalidInput("Некорректное cron-выражение: "+err.Error()))
			return
		}
	}

	// template_campaign_id must be a valid UUID
	if _, err := uuid.Parse(req.TemplateCampaignID); err != nil {
		respondError(w, shared.ErrInvalidInput("template_campaign_id: некорректный UUID"))
		return
	}

	// template_campaign_id must exist and belong to this client
	var exists bool
	if err := h.db.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM campaigns WHERE id = $1::uuid AND client_id = $2)`,
		req.TemplateCampaignID, clientID.String(),
	).Scan(&exists); err != nil {
		respondError(w, shared.ErrInternalServer("Ошибка проверки кампании-шаблона"))
		return
	}
	if !exists {
		respondError(w, shared.ErrNotFound("Кампания-шаблон не найдена или не принадлежит вашему аккаунту"))
		return
	}

	// max_runs must be positive if provided
	if req.MaxRuns != nil && *req.MaxRuns <= 0 {
		respondError(w, shared.ErrInvalidInput("max_runs должно быть положительным числом"))
		return
	}

	// Compute first next_run_at
	cronExpr := ""
	if req.CronExpression != nil {
		cronExpr = *req.CronExpression
	}
	nextRunAt := schedules.NextRunTime(req.Frequency, cronExpr, time.Now().UTC())

	id := uuid.New()
	_, err := h.db.Exec(r.Context(), `
		INSERT INTO campaign_schedules
			(id, client_id, name, template_campaign_id, frequency, cron_expression, max_runs, next_run_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, true)
	`, id, clientID.String(), req.Name, req.TemplateCampaignID, req.Frequency, req.CronExpression, req.MaxRuns, nextRunAt)
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
