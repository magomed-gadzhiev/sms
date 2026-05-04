package handlers

// /reseller/network/provider-sets/{id}/items — содержимое provider-set'а агрегатора.
//
// PUT — атомарный replace items с pre-валидацией ownership провайдеров и
// последующим вызовом ProviderSetMaterializer.ApplyToAllSubscribers, чтобы
// все суб-аккаунты-подписчики получили обновлённый каталог в client_providers.
// GET — список items с join на providers (provider_name).

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

const maxProviderSetItems = 100

// NetworkProviderSetItemsHandlers обрабатывает /portal/v1/reseller/network/provider-sets/{id}/items.
type NetworkProviderSetItemsHandlers struct {
	pool         *pgxpool.Pool
	setRepo      *storage.ResellerProviderSetRepository
	itemsRepo    *storage.ResellerProviderSetItemsRepository
	materializer *network.ProviderSetMaterializer
}

// NewNetworkProviderSetItemsHandlers конструирует handler.
func NewNetworkProviderSetItemsHandlers(pool *pgxpool.Pool, mat *network.ProviderSetMaterializer) *NetworkProviderSetItemsHandlers {
	return &NetworkProviderSetItemsHandlers{
		pool:         pool,
		setRepo:      storage.NewResellerProviderSetRepository(pool),
		itemsRepo:    storage.NewResellerProviderSetItemsRepository(pool),
		materializer: mat,
	}
}

// reseller извлекает client_id из контекста.
func (h *NetworkProviderSetItemsHandlers) reseller(r *http.Request) (uuid.UUID, *shared.AppError) {
	cid, ok := middleware.GetClientID(r.Context())
	if !ok || cid == uuid.Nil {
		return uuid.Nil, shared.ErrUnauthorized("Клиент не найден")
	}
	return cid, nil
}

// verifyOwnership загружает set и проверяет, что он принадлежит resellerID.
// Чужой set → 404 (не светим существование).
func (h *NetworkProviderSetItemsHandlers) verifyOwnership(r *http.Request, resellerID, setID uuid.UUID) *shared.AppError {
	set, err := h.setRepo.GetByID(r.Context(), setID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return shared.ErrNotFound("provider-set")
		}
		log.Error().Err(err).Msg("provider-set-items ownership lookup")
		return shared.ErrInternalServer("ошибка чтения provider-set")
	}
	if set.ResellerID != resellerID {
		return shared.ErrNotFound("provider-set")
	}
	return nil
}

// providerSetItemOut — JSON-форма item'а с join'ом на providers.name.
type providerSetItemOut struct {
	ID                 string `json:"id"`
	ProviderID         string `json:"provider_id"`
	ProviderName       string `json:"provider_name"`
	Priority           int    `json:"priority"`
	ExposeCost         bool   `json:"expose_cost"`
	ExposeProviderName bool   `json:"expose_provider_name"`
}

// providerSetItemReq — элемент тела PUT.
type providerSetItemReq struct {
	ProviderID         string `json:"provider_id"`
	Priority           int    `json:"priority"`
	ExposeCost         bool   `json:"expose_cost"`
	ExposeProviderName bool   `json:"expose_provider_name"`
}

// putItemsReq — тело PUT.
type putItemsReq struct {
	Items []providerSetItemReq `json:"items"`
}

// ListItems GET /portal/v1/reseller/network/provider-sets/{id}/items
func (h *NetworkProviderSetItemsHandlers) ListItems(w http.ResponseWriter, r *http.Request) {
	resellerID, appErr := h.reseller(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("id invalid"))
		return
	}
	if ownErr := h.verifyOwnership(r, resellerID, setID); ownErr != nil {
		respondError(w, ownErr)
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT i.id, i.provider_id, p.name AS provider_name,
		        i.priority, i.expose_cost, i.expose_provider_name
		   FROM reseller_provider_set_items i
		   JOIN providers p ON p.id = i.provider_id
		  WHERE i.set_id = $1
		  ORDER BY i.priority DESC, p.name`, setID)
	if err != nil {
		log.Error().Err(err).Msg("provider-set-items list query")
		respondError(w, shared.ErrInternalServer("ошибка чтения items"))
		return
	}
	defer rows.Close()

	out := []providerSetItemOut{}
	for rows.Next() {
		var (
			id, providerID         uuid.UUID
			providerName           string
			priority               int
			expCost, expProviderNm bool
		)
		if err := rows.Scan(&id, &providerID, &providerName, &priority, &expCost, &expProviderNm); err != nil {
			log.Error().Err(err).Msg("provider-set-items scan")
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		out = append(out, providerSetItemOut{
			ID:                 id.String(),
			ProviderID:         providerID.String(),
			ProviderName:       providerName,
			Priority:           priority,
			ExposeCost:         expCost,
			ExposeProviderName: expProviderNm,
		})
	}
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("provider-set-items rows.Err")
		respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"items": out})
}

// PutItems PUT /portal/v1/reseller/network/provider-sets/{id}/items
//
// Атомарный replace + materialize:
//  1. ownership-check на set (чужой → 404).
//  2. pre-валидация input: дубли provider_id → 400; неизвестный provider → 400;
//     чужой private-провайдер → 403.
//  3. itemsRepo.ReplaceItems (DELETE+INSERT в одной транзакции).
//  4. materializer.ApplyToAllSubscribers — обновить client_providers подписчиков.
//
// Шаги 3 и 4 не атомарны между собой: если materialize упадёт, items уже сохранены,
// caller может повторить PUT. Это явный compromise (см. ApplyToAllSubscribers
// docstring — partial failure возможен и на уровне самого materializer'а).
func (h *NetworkProviderSetItemsHandlers) PutItems(w http.ResponseWriter, r *http.Request) {
	resellerID, appErr := h.reseller(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("id invalid"))
		return
	}
	if ownErr := h.verifyOwnership(r, resellerID, setID); ownErr != nil {
		respondError(w, ownErr)
		return
	}

	var req putItemsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Невалидный JSON"))
		return
	}
	if len(req.Items) > maxProviderSetItems {
		respondError(w, shared.ErrInvalidInput(fmt.Sprintf("too many items (max %d)", maxProviderSetItems)))
		return
	}

	// Шаг 1: распарсить provider_id и поймать дубликаты.
	seen := make(map[uuid.UUID]struct{}, len(req.Items))
	parsed := make([]storage.ProviderSetItemInput, 0, len(req.Items))
	for idx, it := range req.Items {
		pid, err := uuid.Parse(it.ProviderID)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("items["+strconv.Itoa(idx)+"].provider_id invalid"))
			return
		}
		if _, dup := seen[pid]; dup {
			respondError(w, shared.ErrInvalidInput("дубликат provider_id в items"))
			return
		}
		seen[pid] = struct{}{}
		parsed = append(parsed, storage.ProviderSetItemInput{
			ProviderID:         pid,
			Priority:           it.Priority,
			ExposeCost:         it.ExposeCost,
			ExposeProviderName: it.ExposeProviderName,
		})
	}

	// Шаг 2: валидация ownership каждого provider_id.
	for _, it := range parsed {
		var (
			ownership      string
			sourceClientID *uuid.UUID
		)
		err := h.pool.QueryRow(r.Context(),
			`SELECT ownership, source_client_id FROM providers WHERE id = $1`, it.ProviderID,
		).Scan(&ownership, &sourceClientID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				respondError(w, shared.ErrInvalidInput("provider "+it.ProviderID.String()+" не существует"))
				return
			}
			log.Error().Err(err).Msg("provider-set-items provider lookup")
			respondError(w, shared.ErrInternalServer("ошибка проверки провайдера"))
			return
		}
		switch ownership {
		case "platform":
			// разрешено всем агрегаторам
		case "private":
			if sourceClientID == nil || *sourceClientID != resellerID {
				respondError(w, shared.ErrForbidden("провайдер "+it.ProviderID.String()+" принадлежит другому агрегатору"))
				return
			}
		default:
			respondError(w, shared.ErrInvalidInput("провайдер "+it.ProviderID.String()+" имеет неподдерживаемый ownership"))
			return
		}
	}

	// Шаг 3: атомарная замена items.
	if err := h.itemsRepo.ReplaceItems(r.Context(), setID, parsed); err != nil {
		log.Error().Err(err).Str("set_id", setID.String()).Msg("provider-set-items replace")
		respondError(w, shared.ErrInternalServer("ошибка сохранения items"))
		return
	}

	// Шаг 4: materialize в client_providers всех подписчиков.
	if h.materializer != nil {
		if err := h.materializer.ApplyToAllSubscribers(r.Context(), setID); err != nil {
			log.Error().Err(err).Str("set_id", setID.String()).Msg("provider-set-items rematerialize")
			respondError(w, shared.ErrInternalServer("rematerialize"))
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"set_id": setID.String(),
		"count":  len(parsed),
	})
}
