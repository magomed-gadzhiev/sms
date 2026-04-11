package handlers

import (
	"net/http"

	cpv1 "github.com/smpp-server/smpp-server/api/proto/clientproviderv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/rs/zerolog/log"
)

// HealthHandlers содержит handlers для проверки здоровья системы
type HealthHandlers struct {
	providerClient cpv1.ClientProviderServiceClient
}

// NewHealthHandlers создаёт HealthHandlers
func NewHealthHandlers(providerClient cpv1.ClientProviderServiceClient) *HealthHandlers {
	return &HealthHandlers{providerClient: providerClient}
}

// GetProviderHealth обрабатывает GET /portal/v1/providers/health
func (h *HealthHandlers) GetProviderHealth(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	if h.providerClient == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"providers": []interface{}{}})
		return
	}

	resp, err := h.providerClient.ListClientProviders(r.Context(), &cpv1.ListClientProvidersRequest{
		ClientId: clientID.String(),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения провайдеров клиента")
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}

	type providerHealth struct {
		ID             string  `json:"id"`
		Name           string  `json:"name"`
		ConnectionType string  `json:"connection_type"`
		MaxConnections int32   `json:"max_connections"`
		TpsLimit       int32   `json:"tps_limit"`
		Active         bool    `json:"active"`
		SuccessRate    float32 `json:"success_rate"` // TODO: wire to real metrics
		MsgPerSec      float32 `json:"msg_per_sec"`  // TODO: wire to real metrics
		IsDegraded     bool    `json:"is_degraded"`
	}

	providers := make([]providerHealth, 0, len(resp.Providers))
	for _, p := range resp.Providers {
		ph := providerHealth{
			ID:             p.Id,
			Name:           p.Name,
			ConnectionType: "SMPP", // all client providers use SMPP
			MaxConnections: p.MaxConnections,
			TpsLimit:       p.TpsLimit,
			Active:         p.Active,
			SuccessRate:    0.0, // TODO: wire to real metrics
			MsgPerSec:      0.0, // TODO: wire to real metrics
			IsDegraded:     !p.Active,
		}
		providers = append(providers, ph)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"providers": providers})
}
