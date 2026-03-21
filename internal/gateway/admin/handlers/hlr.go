package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	routingv1 "github.com/smpp-server/smpp-server/api/proto/routingv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// HLRHandlers обрабатывает HTTP запросы для управления HLR-провайдерами и весами маршрутизации
type HLRHandlers struct {
	routingClient routingv1.RoutingServiceClient
}

// NewHLRHandlers создает новый экземпляр HLRHandlers
func NewHLRHandlers(routingClient routingv1.RoutingServiceClient) *HLRHandlers {
	return &HLRHandlers{
		routingClient: routingClient,
	}
}

// === HLR Provider endpoints ===

// CreateProviderRequest — запрос на создание HLR-провайдера
type CreateHLRProviderRequest struct {
	Name             string   `json:"name"`
	AdapterType      string   `json:"adapter_type"`
	ConfigJSON       string   `json:"config_json"`
	Priority         int32    `json:"priority"`
	SupportedRegions []string `json:"supported_regions"`
	CostPerLookup    string   `json:"cost_per_lookup"`
}

// CreateProvider обрабатывает POST /admin/v1/hlr/providers
func (h *HLRHandlers) CreateProvider(w http.ResponseWriter, r *http.Request) {
	var req CreateHLRProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("name обязателен"))
		return
	}
	if req.AdapterType == "" {
		respondError(w, shared.ErrInvalidInput("adapter_type обязателен"))
		return
	}

	resp, err := h.routingClient.CreateHLRProvider(r.Context(), &routingv1.CreateHLRProviderRequest{
		Name:             req.Name,
		AdapterType:      req.AdapterType,
		ConfigJson:       req.ConfigJSON,
		Priority:         req.Priority,
		SupportedRegions: req.SupportedRegions,
		CostPerLookup:    req.CostPerLookup,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания HLR-провайдера")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, hlrProviderToResponse(resp))
}

// GetProvider обрабатывает GET /admin/v1/hlr/providers/{id}
func (h *HLRHandlers) GetProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerID := vars["id"]

	resp, err := h.routingClient.GetHLRProvider(r.Context(), &routingv1.GetHLRProviderRequest{
		Id: providerID,
	})
	if err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Msg("ошибка получения HLR-провайдера")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, hlrProviderToResponse(resp))
}

// ListProviders обрабатывает GET /admin/v1/hlr/providers
func (h *HLRHandlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("active_only") == "true"

	resp, err := h.routingClient.ListHLRProviders(r.Context(), &routingv1.ListHLRProvidersRequest{
		ActiveOnly: activeOnly,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка HLR-провайдеров")
		respondGRPCError(w, err)
		return
	}

	providers := make([]map[string]interface{}, 0, len(resp.Providers))
	for _, p := range resp.Providers {
		providers = append(providers, hlrProviderToResponse(p))
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"providers": providers,
		"total":     len(providers),
	})
}

// UpdateProvider обрабатывает PUT /admin/v1/hlr/providers/{id}
func (h *HLRHandlers) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerID := vars["id"]

	var req struct {
		Name             string   `json:"name"`
		AdapterType      string   `json:"adapter_type"`
		ConfigJSON       string   `json:"config_json"`
		Priority         int32    `json:"priority"`
		SupportedRegions []string `json:"supported_regions"`
		CostPerLookup    string   `json:"cost_per_lookup"`
		Active           bool     `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	resp, err := h.routingClient.UpdateHLRProvider(r.Context(), &routingv1.UpdateHLRProviderRequest{
		Id:               providerID,
		Name:             req.Name,
		AdapterType:      req.AdapterType,
		ConfigJson:       req.ConfigJSON,
		Priority:         req.Priority,
		SupportedRegions: req.SupportedRegions,
		CostPerLookup:    req.CostPerLookup,
		Active:           req.Active,
	})
	if err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Msg("ошибка обновления HLR-провайдера")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, hlrProviderToResponse(resp))
}

// DeleteProvider обрабатывает DELETE /admin/v1/hlr/providers/{id}
func (h *HLRHandlers) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerID := vars["id"]

	resp, err := h.routingClient.DeleteHLRProvider(r.Context(), &routingv1.DeleteHLRProviderRequest{
		Id: providerID,
	})
	if err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Msg("ошибка удаления HLR-провайдера")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": resp != nil && resp.Success,
	})
}

// GetProviderHealth обрабатывает GET /admin/v1/hlr/providers/{id}/health
func (h *HLRHandlers) GetProviderHealth(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerID := vars["id"]

	// Получаем провайдера — в его данных есть информация о статусе и health
	resp, err := h.routingClient.GetHLRProvider(r.Context(), &routingv1.GetHLRProviderRequest{
		Id: providerID,
	})
	if err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Msg("ошибка получения health HLR-провайдера")
		respondGRPCError(w, err)
		return
	}

	health := map[string]interface{}{
		"provider_id":  resp.Id,
		"name":         resp.Name,
		"status":       resp.Status,
		"success_rate": resp.SuccessRate,
		"active":       resp.Active,
	}
	if resp.LastSuccessAt != nil {
		health["last_success_at"] = resp.LastSuccessAt.AsTime()
	}
	if resp.LastFailureAt != nil {
		health["last_failure_at"] = resp.LastFailureAt.AsTime()
	}

	respondJSON(w, http.StatusOK, health)
}

// === Smart Route Weight endpoints ===

// SetWeightsRequest — запрос на установку весов маршрутизации
type SetWeightsRequest struct {
	OperatorCode  string `json:"operator_code"`
	CountryCode   string `json:"country_code"`
	CostWeight    string `json:"cost_weight"`
	QualityWeight string `json:"quality_weight"`
}

// SetWeights обрабатывает POST /admin/v1/routing/weights
func (h *HLRHandlers) SetWeights(w http.ResponseWriter, r *http.Request) {
	var req SetWeightsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.OperatorCode == "" {
		respondError(w, shared.ErrInvalidInput("operator_code обязателен"))
		return
	}
	if req.CountryCode == "" {
		respondError(w, shared.ErrInvalidInput("country_code обязателен"))
		return
	}

	resp, err := h.routingClient.SetSmartRouteWeights(r.Context(), &routingv1.SetSmartRouteWeightsRequest{
		OperatorCode:  req.OperatorCode,
		CountryCode:   req.CountryCode,
		CostWeight:    req.CostWeight,
		QualityWeight: req.QualityWeight,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка установки весов маршрутизации")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, smartRouteWeightToResponse(resp))
}

// ListWeights обрабатывает GET /admin/v1/routing/weights
func (h *HLRHandlers) ListWeights(w http.ResponseWriter, r *http.Request) {
	countryCode := r.URL.Query().Get("country_code")

	resp, err := h.routingClient.ListSmartRouteWeights(r.Context(), &routingv1.ListSmartRouteWeightsRequest{
		CountryCode: countryCode,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка весов маршрутизации")
		respondGRPCError(w, err)
		return
	}

	weights := make([]map[string]interface{}, 0, len(resp.Weights))
	for _, w := range resp.Weights {
		weights = append(weights, smartRouteWeightToResponse(w))
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"weights": weights,
		"total":   len(weights),
	})
}

// DeleteWeights обрабатывает DELETE /admin/v1/routing/weights/{id}
func (h *HLRHandlers) DeleteWeights(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	weightID := vars["id"]

	resp, err := h.routingClient.DeleteSmartRouteWeights(r.Context(), &routingv1.DeleteSmartRouteWeightsRequest{
		Id: weightID,
	})
	if err != nil {
		log.Error().Err(err).Str("weight_id", weightID).Msg("ошибка удаления весов маршрутизации")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": resp != nil && resp.Success,
	})
}

// hlrProviderToResponse преобразует proto HLRProviderProto в JSON-ответ
func hlrProviderToResponse(p *routingv1.HLRProviderProto) map[string]interface{} {
	result := map[string]interface{}{
		"id":                p.Id,
		"name":              p.Name,
		"adapter_type":      p.AdapterType,
		"config_json":       p.ConfigJson,
		"priority":          p.Priority,
		"supported_regions": p.SupportedRegions,
		"cost_per_lookup":   p.CostPerLookup,
		"status":            p.Status,
		"success_rate":      p.SuccessRate,
		"active":            p.Active,
	}
	if p.LastSuccessAt != nil {
		result["last_success_at"] = p.LastSuccessAt.AsTime()
	}
	if p.LastFailureAt != nil {
		result["last_failure_at"] = p.LastFailureAt.AsTime()
	}
	if p.CreatedAt != nil {
		result["created_at"] = p.CreatedAt.AsTime()
	}
	if p.UpdatedAt != nil {
		result["updated_at"] = p.UpdatedAt.AsTime()
	}
	return result
}

// smartRouteWeightToResponse преобразует proto SmartRouteWeightProto в JSON-ответ
func smartRouteWeightToResponse(w *routingv1.SmartRouteWeightProto) map[string]interface{} {
	result := map[string]interface{}{
		"id":             w.Id,
		"operator_code":  w.OperatorCode,
		"country_code":   w.CountryCode,
		"cost_weight":    w.CostWeight,
		"quality_weight": w.QualityWeight,
		"active":         w.Active,
	}
	if w.CreatedAt != nil {
		result["created_at"] = w.CreatedAt.AsTime()
	}
	if w.UpdatedAt != nil {
		result["updated_at"] = w.UpdatedAt.AsTime()
	}
	return result
}
