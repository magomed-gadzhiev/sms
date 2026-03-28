package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/services/contact/application"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
	"github.com/smpp-server/smpp-server/internal/services/contact/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type SegmentHandlers struct {
	service *application.SegmentService
}

func NewSegmentHandlers(pool *pgxpool.Pool) *SegmentHandlers {
	repo := repository.NewSegmentRepository(pool)
	svc := application.NewSegmentService(repo, pool)
	return &SegmentHandlers{service: svc}
}

func (h *SegmentHandlers) CreateSegment(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	var req struct {
		Name           string              `json:"name"`
		Description    string              `json:"description"`
		ContactListIDs []string            `json:"contact_list_ids"`
		Rules          domain.SegmentRules `json:"rules"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат"))
		return
	}

	listIDs := make([]uuid.UUID, len(req.ContactListIDs))
	for i, s := range req.ContactListIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный contact_list_id"))
			return
		}
		listIDs[i] = id
	}

	seg := &domain.SavedSegment{
		ClientID:       clientID,
		Name:           req.Name,
		Description:    req.Description,
		ContactListIDs: listIDs,
		Rules:          req.Rules,
	}
	if err := h.service.Create(r.Context(), seg); err != nil {
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	respondJSON(w, http.StatusCreated, seg)
}

func (h *SegmentHandlers) ListSegments(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	segments, err := h.service.List(r.Context(), clientID)
	if err != nil {
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"segments": segments})
}

func (h *SegmentHandlers) GetSegment(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный ID"))
		return
	}
	seg, err := h.service.Get(r.Context(), id, clientID)
	if err != nil {
		respondError(w, shared.ErrNotFound("Сегмент не найден"))
		return
	}
	respondJSON(w, http.StatusOK, seg)
}

func (h *SegmentHandlers) UpdateSegment(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный ID"))
		return
	}

	var req struct {
		Name           string              `json:"name"`
		Description    string              `json:"description"`
		ContactListIDs []string            `json:"contact_list_ids"`
		Rules          domain.SegmentRules `json:"rules"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат"))
		return
	}

	listIDs := make([]uuid.UUID, len(req.ContactListIDs))
	for i, s := range req.ContactListIDs {
		listIDs[i], _ = uuid.Parse(s)
	}

	seg := &domain.SavedSegment{
		ID: id, ClientID: clientID, Name: req.Name, Description: req.Description,
		ContactListIDs: listIDs, Rules: req.Rules,
	}
	if err := h.service.Update(r.Context(), seg); err != nil {
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	respondJSON(w, http.StatusOK, seg)
}

func (h *SegmentHandlers) DeleteSegment(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	id, _ := uuid.Parse(mux.Vars(r)["id"])
	h.service.Delete(r.Context(), id, clientID)
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *SegmentHandlers) EstimateSegment(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	id, _ := uuid.Parse(mux.Vars(r)["id"])
	seg, err := h.service.Get(r.Context(), id, clientID)
	if err != nil {
		respondError(w, shared.ErrNotFound("Сегмент не найден"))
		return
	}
	count, err := h.service.EstimateCount(r.Context(), seg)
	if err != nil {
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	respondJSON(w, http.StatusOK, map[string]int32{"estimated_count": count})
}
