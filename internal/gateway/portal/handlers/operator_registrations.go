package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type OperatorRegistrationHandlers struct {
	pool *pgxpool.Pool
}

func NewOperatorRegistrationHandlers(pool *pgxpool.Pool) *OperatorRegistrationHandlers {
	return &OperatorRegistrationHandlers{pool: pool}
}

// BulkSubmitOperatorRegistrations POST /portal/v1/sender-names/{id}/operator-registrations
func (h *OperatorRegistrationHandlers) BulkSubmitOperatorRegistrations(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	senderNameID := mux.Vars(r)["id"]
	if senderNameID == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}

	var snStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT status FROM sender_names WHERE id = $1 AND client_id = $2`,
		senderNameID, clientID,
	).Scan(&snStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("имя отправителя не найдено"))
		return
	}
	if snStatus != "approved" {
		respondError(w, shared.ErrInvalidInput("имя отправителя должно быть в статусе approved"))
		return
	}

	var req struct {
		Registrations []struct {
			OperatorID string `json:"operator_id"`
			Type       string `json:"type"`
		} `json:"registrations"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if len(req.Registrations) == 0 {
		respondError(w, shared.ErrInvalidInput("registrations не может быть пустым"))
		return
	}

	type resultItem struct {
		OperatorID string `json:"operator_id"`
		ID         string `json:"id,omitempty"`
		Status     string `json:"status"`
		Error      string `json:"error,omitempty"`
	}
	results := make([]resultItem, 0, len(req.Registrations))

	for _, reg := range req.Registrations {
		if reg.OperatorID == "" {
			results = append(results, resultItem{OperatorID: reg.OperatorID, Status: "error", Error: "operator_id обязателен"})
			continue
		}
		regType := reg.Type
		if regType == "" {
			regType = "free"
		}
		if regType != "free" && regType != "paid" {
			results = append(results, resultItem{OperatorID: reg.OperatorID, Status: "error", Error: "type должен быть free или paid"})
			continue
		}

		var newID string
		err := h.pool.QueryRow(r.Context(),
			`INSERT INTO operator_registrations
			    (sender_name_id, operator_id, registration_type, status, submitted_at, created_at, updated_at)
			 VALUES ($1, $2, $3, 'submitted', NOW(), NOW(), NOW())
			 ON CONFLICT (sender_name_id, operator_id) DO UPDATE
			    SET registration_type = EXCLUDED.registration_type,
			        status = 'submitted',
			        submitted_at = NOW(),
			        updated_at = NOW()
			 RETURNING id`,
			senderNameID, reg.OperatorID, regType,
		).Scan(&newID)
		if err != nil {
			log.Error().Err(err).Str("operator_id", reg.OperatorID).Msg("ошибка создания operator_registration")
			results = append(results, resultItem{OperatorID: reg.OperatorID, Status: "error", Error: "ошибка создания записи"})
			continue
		}

		_, _ = h.pool.Exec(r.Context(),
			`INSERT INTO operator_registration_history
			    (operator_registration_id, old_status, new_status, actor_id, actor_type, created_at)
			 VALUES ($1, NULL, 'submitted', $2, 'client', NOW())`,
			newID, clientID,
		)

		results = append(results, resultItem{OperatorID: reg.OperatorID, ID: newID, Status: "submitted"})
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{"results": results})
}

// ListOperatorRegistrations GET /portal/v1/sender-names/{id}/operator-registrations
func (h *OperatorRegistrationHandlers) ListOperatorRegistrations(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	senderNameID := mux.Vars(r)["id"]
	if senderNameID == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT or2.id, or2.operator_id, o.name, or2.registration_type, or2.status,
		        or2.approved_type, or2.approved_at, or2.moderator_note,
		        or2.submitted_at, or2.resolved_at
		 FROM operator_registrations or2
		 JOIN operators o ON o.id = or2.operator_id
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 WHERE or2.sender_name_id = $1 AND sn.client_id = $2
		 ORDER BY o.name`,
		senderNameID, clientID,
	)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения operator_registrations")
		respondError(w, shared.ErrInternalServer("ошибка получения регистраций"))
		return
	}
	defer rows.Close()

	type regJSON struct {
		ID               string     `json:"id"`
		OperatorID       string     `json:"operator_id"`
		OperatorName     string     `json:"operator_name"`
		RegistrationType string     `json:"registration_type"`
		Status           string     `json:"status"`
		ApprovedType     *string    `json:"approved_type"`
		ApprovedAt       *time.Time `json:"approved_at"`
		ModeratorNote    *string    `json:"moderator_note"`
		SubmittedAt      time.Time  `json:"submitted_at"`
		ResolvedAt       *time.Time `json:"resolved_at"`
	}
	regs := make([]regJSON, 0)
	for rows.Next() {
		var reg regJSON
		if err := rows.Scan(
			&reg.ID, &reg.OperatorID, &reg.OperatorName,
			&reg.RegistrationType, &reg.Status,
			&reg.ApprovedType, &reg.ApprovedAt, &reg.ModeratorNote,
			&reg.SubmittedAt, &reg.ResolvedAt,
		); err != nil {
			log.Error().Err(err).Msg("ошибка сканирования registration")
			respondError(w, shared.ErrInternalServer("ошибка получения регистраций"))
			return
		}
		regs = append(regs, reg)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"registrations": regs})
}

// ResubmitOperatorRegistration POST /portal/v1/sender-names/{id}/operator-registrations/{rid}/resubmit
func (h *OperatorRegistrationHandlers) ResubmitOperatorRegistration(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	senderNameID := mux.Vars(r)["id"]
	regID := mux.Vars(r)["rid"]

	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT or2.status FROM operator_registrations or2
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 WHERE or2.id = $1 AND or2.sender_name_id = $2 AND sn.client_id = $3`,
		regID, senderNameID, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("регистрация не найдена"))
		return
	}
	if currentStatus != "revision_requested" {
		respondError(w, shared.ErrInvalidInput("resubmit возможен только из статуса revision_requested"))
		return
	}

	_, err = h.pool.Exec(r.Context(),
		`UPDATE operator_registrations
		 SET status = 'submitted', submitted_at = NOW(), resolved_at = NULL,
		     moderator_note = NULL, updated_at = NOW()
		 WHERE id = $1`,
		regID,
	)
	if err != nil {
		log.Error().Err(err).Str("id", regID).Msg("ошибка resubmit")
		respondError(w, shared.ErrInternalServer("ошибка обновления статуса"))
		return
	}

	_, _ = h.pool.Exec(r.Context(),
		`INSERT INTO operator_registration_history
		    (operator_registration_id, old_status, new_status, actor_id, actor_type, created_at)
		 VALUES ($1, 'revision_requested', 'submitted', $2, 'client', NOW())`,
		regID, clientID,
	)

	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "submitted"})
}
