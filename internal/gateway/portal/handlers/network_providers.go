package handlers

// /reseller/network/providers — каталог провайдеров для агрегатора.
//
// Возвращает список платформенных провайдеров (ownership='platform') + private
// (ownership='private', source_client_id = текущий агрегатор). CRUD работает
// только с private; платформенные защищены через 403.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// NetworkProvidersHandlers обрабатывает /portal/v1/reseller/network/providers.
type NetworkProvidersHandlers struct {
	pool *pgxpool.Pool
}

// NewNetworkProvidersHandlers конструирует handler с pgx-пулом.
func NewNetworkProvidersHandlers(pool *pgxpool.Pool) *NetworkProvidersHandlers {
	return &NetworkProvidersHandlers{pool: pool}
}

// reseller извлекает client_id из контекста (положенного session middleware).
func (h *NetworkProvidersHandlers) reseller(r *http.Request) (uuid.UUID, *shared.AppError) {
	cid, ok := middleware.GetClientID(r.Context())
	if !ok || cid == uuid.Nil {
		return uuid.Nil, shared.ErrUnauthorized("Клиент не найден")
	}
	return cid, nil
}

// providerOut — JSON-форма провайдера.
// SMPP-поля map'ятся на колонки БД host/port/system_id/system_type. API использует
// префикс smpp_ — соглашение с фронтом.
type providerOut struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Ownership  string  `json:"ownership"`
	SMPPHost   *string `json:"smpp_host"`
	SMPPPort   *int    `json:"smpp_port"`
	SystemID   *string `json:"system_id"`
	SystemType *string `json:"system_type"`
	Active     bool    `json:"active"`
}

// providerCreateReq — тело POST/PUT.
type providerCreateReq struct {
	Name       string `json:"name"`
	SMPPHost   string `json:"smpp_host"`
	SMPPPort   int    `json:"smpp_port"`
	SystemID   string `json:"system_id"`
	Password   string `json:"password"`
	SystemType string `json:"system_type"`
}

// List GET /portal/v1/reseller/network/providers
//
// Возвращает все платформенные провайдеры + private текущего агрегатора.
func (h *NetworkProvidersHandlers) List(w http.ResponseWriter, r *http.Request) {
	resellerID, appErr := h.reseller(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, name,
		        COALESCE(ownership, 'platform') AS ownership,
		        host, port, system_id, system_type, active
		   FROM providers
		  WHERE COALESCE(ownership, 'platform') = 'platform'
		     OR (ownership = 'private' AND source_client_id = $1)
		  ORDER BY ownership, name`, resellerID)
	if err != nil {
		log.Error().Err(err).Msg("network providers list")
		respondError(w, shared.ErrInternalServer("ошибка получения провайдеров"))
		return
	}
	defer rows.Close()

	out := []providerOut{}
	for rows.Next() {
		var (
			p          providerOut
			host       string
			port       int
			systemID   *string
			systemType *string
		)
		if err := rows.Scan(&p.ID, &p.Name, &p.Ownership, &host, &port, &systemID, &systemType, &p.Active); err != nil {
			log.Error().Err(err).Msg("network providers scan")
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		p.SMPPHost = &host
		p.SMPPPort = &port
		p.SystemID = systemID
		p.SystemType = systemType
		out = append(out, p)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"providers": out})
}

// Create POST /portal/v1/reseller/network/providers
//
// Создаёт private-провайдера, принадлежащего текущему агрегатору.
func (h *NetworkProvidersHandlers) Create(w http.ResponseWriter, r *http.Request) {
	resellerID, appErr := h.reseller(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}
	var req providerCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Невалидный JSON"))
		return
	}
	if req.Name == "" || req.SMPPHost == "" || req.SystemID == "" {
		respondError(w, shared.ErrInvalidInput("name/smpp_host/system_id обязательны"))
		return
	}
	if req.SMPPPort <= 0 {
		req.SMPPPort = 2775
	}

	var id uuid.UUID
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO providers
		     (name, ownership, source_client_id, host, port, system_id, password, system_type, bind_type, active)
		 VALUES ($1, 'private', $2, $3, $4, $5, $6, $7, 'transceiver', true)
		 RETURNING id`,
		req.Name, resellerID, req.SMPPHost, req.SMPPPort, req.SystemID, req.Password, req.SystemType,
	).Scan(&id)
	if err != nil {
		log.Error().Err(err).Msg("create private provider")
		respondError(w, shared.ErrInternalServer("ошибка создания провайдера"))
		return
	}
	respondJSON(w, http.StatusCreated, providerOut{
		ID:         id.String(),
		Name:       req.Name,
		Ownership:  "private",
		SMPPHost:   &req.SMPPHost,
		SMPPPort:   &req.SMPPPort,
		SystemID:   &req.SystemID,
		SystemType: &req.SystemType,
		Active:     true,
	})
}

// Update PUT /portal/v1/reseller/network/providers/{id}
//
// Редактирует private-провайдера агрегатора. Платформенные — 403. Чужие
// private — 404 (не светим существование).
func (h *NetworkProvidersHandlers) Update(w http.ResponseWriter, r *http.Request) {
	resellerID, appErr := h.reseller(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}
	id, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("id invalid"))
		return
	}

	var (
		ownership      string
		sourceClientID *uuid.UUID
	)
	err := h.pool.QueryRow(r.Context(),
		`SELECT COALESCE(ownership,'platform'), source_client_id FROM providers WHERE id=$1`, id,
	).Scan(&ownership, &sourceClientID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, shared.ErrNotFound("провайдер"))
			return
		}
		log.Error().Err(err).Msg("network providers update select")
		respondError(w, shared.ErrInternalServer("ошибка чтения провайдера"))
		return
	}
	if ownership == "platform" {
		respondError(w, shared.ErrForbidden("нельзя редактировать платформенный провайдер"))
		return
	}
	if sourceClientID == nil || *sourceClientID != resellerID {
		respondError(w, shared.ErrNotFound("провайдер"))
		return
	}

	var req providerCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid JSON"))
		return
	}
	if req.Name == "" || req.SMPPHost == "" || req.SystemID == "" {
		respondError(w, shared.ErrInvalidInput("name/smpp_host/system_id обязательны"))
		return
	}
	if req.SMPPPort <= 0 {
		req.SMPPPort = 2775
	}

	_, err = h.pool.Exec(r.Context(),
		`UPDATE providers
		    SET name=$1,
		        host=$2,
		        port=$3,
		        system_id=$4,
		        password=COALESCE(NULLIF($5,''), password),
		        system_type=$6
		  WHERE id=$7`,
		req.Name, req.SMPPHost, req.SMPPPort, req.SystemID, req.Password, req.SystemType, id)
	if err != nil {
		log.Error().Err(err).Msg("network providers update")
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"id": id.String()})
}

// Delete DELETE /portal/v1/reseller/network/providers/{id}
//
// Удаляет private-провайдера, если он не используется в provider-set'ах
// агрегатора и не выставлен override'ом ни одному суб-аккаунту. Иначе — 409.
// Платформенные — 403.
func (h *NetworkProvidersHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	resellerID, appErr := h.reseller(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}
	id, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("id invalid"))
		return
	}

	var (
		ownership      string
		sourceClientID *uuid.UUID
	)
	err := h.pool.QueryRow(r.Context(),
		`SELECT COALESCE(ownership,'platform'), source_client_id FROM providers WHERE id=$1`, id,
	).Scan(&ownership, &sourceClientID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, shared.ErrNotFound("провайдер"))
			return
		}
		log.Error().Err(err).Msg("network providers delete select")
		respondError(w, shared.ErrInternalServer("ошибка чтения провайдера"))
		return
	}
	if ownership != "private" || sourceClientID == nil || *sourceClientID != resellerID {
		// Платформенный или чужой private — оба запрещены, отдаём 403.
		respondError(w, shared.ErrForbidden("нельзя удалить"))
		return
	}

	// Проверка использования в provider-set'ах агрегатора.
	var usedInSets int
	if err := h.pool.QueryRow(r.Context(),
		`SELECT count(*)
		   FROM reseller_provider_set_items i
		   JOIN reseller_provider_sets s ON s.id = i.set_id
		  WHERE s.reseller_id = $1 AND i.provider_id = $2`,
		resellerID, id).Scan(&usedInSets); err != nil {
		log.Error().Err(err).Msg("network providers delete usedInSets")
		respondError(w, shared.ErrInternalServer("ошибка проверки использования"))
		return
	}
	if usedInSets > 0 {
		respondError(w, shared.ErrConflict(fmt.Sprintf("используется в %d provider-set", usedInSets)).
			WithDetails(`{"kind":"used_in_provider_sets"}`))
		return
	}

	// Проверка использования как override у суб-аккаунтов.
	var usedInClients int
	if err := h.pool.QueryRow(r.Context(),
		`SELECT count(*)
		   FROM client_providers cp
		   JOIN clients c ON c.id = cp.client_id
		  WHERE c.parent_client_id = $1 AND cp.provider_id = $2 AND cp.ownership = 'private'`,
		resellerID, id).Scan(&usedInClients); err != nil {
		log.Error().Err(err).Msg("network providers delete usedInClients")
		respondError(w, shared.ErrInternalServer("ошибка проверки overrides"))
		return
	}
	if usedInClients > 0 {
		respondError(w, shared.ErrConflict(fmt.Sprintf("используется как override у %d суб-аккаунта(ов)", usedInClients)).
			WithDetails(`{"kind":"used_in_overrides"}`))
		return
	}

	if _, err := h.pool.Exec(r.Context(), `DELETE FROM providers WHERE id=$1`, id); err != nil {
		log.Error().Err(err).Msg("network providers delete")
		respondError(w, shared.ErrInternalServer("ошибка удаления"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
