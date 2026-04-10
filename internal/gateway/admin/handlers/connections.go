package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/api/proto/providerv1"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// ConnectionsHandlers manages SMPP connections (runtime view).
type ConnectionsHandlers struct {
	db             *storage.DB
	providerClient providerv1.ProviderServiceClient
	redis          *redis.Client
}

// NewConnectionsHandlers creates a new ConnectionsHandlers instance.
func NewConnectionsHandlers(db *storage.DB, providerClient providerv1.ProviderServiceClient, rdb *redis.Client) *ConnectionsHandlers {
	return &ConnectionsHandlers{
		db:             db,
		providerClient: providerClient,
		redis:          rdb,
	}
}

type connectionInfo struct {
	ProviderID        string    `json:"provider_id"`
	Name              string    `json:"name"`
	Host              string    `json:"host"`
	Port              int32     `json:"port"`
	SystemID          string    `json:"system_id"`
	BindType          int32     `json:"bind_type"`
	MaxConnections    int32     `json:"max_connections"`
	Status            string    `json:"status"`
	ActiveConnections int32     `json:"active_connections"`
	SuccessRate       float64   `json:"success_rate"`
	MessagesSent24h   int64     `json:"messages_sent_24h"`
	MessagesFailed24h int64     `json:"messages_failed_24h"`
	LastSuccess       string    `json:"last_success,omitempty"`
	LastFailure       string    `json:"last_failure,omitempty"`
	LastError         string    `json:"last_error,omitempty"`
	Active            bool      `json:"active"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type providerRow struct {
	ID             string
	Name           string
	Host           string
	Port           int32
	SystemID       string
	BindType       int32
	MaxConnections int32
	Active         bool
	UpdatedAt      time.Time
}

func (h *ConnectionsHandlers) fetchHealth(ctx context.Context, providerID string) *providerv1.GetProviderHealthResponse {
	hctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp, err := h.providerClient.GetProviderHealth(hctx, &providerv1.GetProviderHealthRequest{
		ProviderId: providerID,
	})
	if err != nil {
		return nil
	}
	return resp
}

func buildConnectionInfo(p providerRow, health *providerv1.GetProviderHealthResponse) connectionInfo {
	ci := connectionInfo{
		ProviderID:     p.ID,
		Name:           p.Name,
		Host:           p.Host,
		Port:           p.Port,
		SystemID:       p.SystemID,
		BindType:       p.BindType,
		MaxConnections: p.MaxConnections,
		Active:         p.Active,
		UpdatedAt:      p.UpdatedAt,
		Status:         "unknown",
	}
	if health != nil {
		ci.Status = health.Status
		ci.ActiveConnections = health.ActiveConnections
		ci.SuccessRate = float64(health.SuccessRate)
		ci.MessagesSent24h = health.MessagesSent_24H
		ci.MessagesFailed24h = health.MessagesFailed_24H
		if health.LastSuccess != nil {
			ci.LastSuccess = health.LastSuccess.AsTime().Format(time.RFC3339)
		}
		if health.LastFailure != nil {
			ci.LastFailure = health.LastFailure.AsTime().Format(time.RFC3339)
		}
		if health.LastError != "" {
			ci.LastError = health.LastError
		}
	}
	return ci
}

// ListConnections handles GET /admin/v1/connections
func (h *ConnectionsHandlers) ListConnections(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id::text, name, host, port, system_id, bind_type, max_connections, active, updated_at
		FROM providers
		ORDER BY name
	`)
	if err != nil {
		log.Error().Err(err).Msg("connections: error listing providers")
		respondError(w, shared.ErrInternal("Ошибка получения провайдеров"))
		return
	}
	defer rows.Close()

	var providers []providerRow
	for rows.Next() {
		var p providerRow
		if err := rows.Scan(&p.ID, &p.Name, &p.Host, &p.Port, &p.SystemID, &p.BindType, &p.MaxConnections, &p.Active, &p.UpdatedAt); err != nil {
			continue
		}
		providers = append(providers, p)
	}

	connections := make([]connectionInfo, len(providers))
	var wg sync.WaitGroup
	for i, p := range providers {
		wg.Add(1)
		go func(i int, p providerRow) {
			defer wg.Done()
			health := h.fetchHealth(r.Context(), p.ID)
			connections[i] = buildConnectionInfo(p, health)
		}(i, p)
	}
	wg.Wait()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"connections": connections,
		"total":       len(connections),
	})
}

// GetConnection handles GET /admin/v1/connections/{id}
func (h *ConnectionsHandlers) GetConnection(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var p providerRow
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id::text, name, host, port, system_id, bind_type, max_connections, active, updated_at
		FROM providers
		WHERE id = $1::uuid
	`, id).Scan(&p.ID, &p.Name, &p.Host, &p.Port, &p.SystemID, &p.BindType, &p.MaxConnections, &p.Active, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		respondError(w, shared.ErrNotFound("Провайдер не найден"))
		return
	}
	if err != nil {
		log.Error().Err(err).Str("provider_id", id).Msg("connections: error getting provider")
		respondError(w, shared.ErrInternal("Ошибка получения провайдера"))
		return
	}

	health := h.fetchHealth(r.Context(), p.ID)
	respondJSON(w, http.StatusOK, buildConnectionInfo(p, health))
}

// ReconnectConnection handles POST /admin/v1/connections/{id}/reconnect
func (h *ConnectionsHandlers) ReconnectConnection(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	h.publishCommand(w, id, "reconnect")
}

// StopConnection handles POST /admin/v1/connections/{id}/stop
func (h *ConnectionsHandlers) StopConnection(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	h.publishCommand(w, id, "stop")
}

func (h *ConnectionsHandlers) publishCommand(w http.ResponseWriter, providerID, command string) {
	payload, err := json.Marshal(map[string]string{
		"provider_id": providerID,
		"command":     command,
	})
	if err != nil {
		log.Error().Err(err).Msg("connections: error marshaling command")
		respondError(w, shared.ErrInternal("Ошибка формирования команды"))
		return
	}

	// Use a detached context so client disconnect does not cancel the Redis publish.
	publishCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := h.redis.Publish(publishCtx, "smpp:admin:commands", string(payload)).Err(); err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Str("command", command).Msg("connections: error publishing command")
		respondError(w, shared.ErrInternal("Ошибка отправки команды"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"queued":      true,
		"provider_id": providerID,
		"command":     command,
	})
}
