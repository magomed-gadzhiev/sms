package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	campaignv1 "github.com/smpp-server/smpp-server/api/proto/campaignv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// CampaignHandlers содержит HTTP обработчики для кампаний
type CampaignHandlers struct {
	campaignClient campaignv1.CampaignServiceClient
}

// NewCampaignHandlers создаёт новый экземпляр CampaignHandlers
func NewCampaignHandlers(campaignClient campaignv1.CampaignServiceClient) *CampaignHandlers {
	return &CampaignHandlers{campaignClient: campaignClient}
}

// CreateCampaign обрабатывает POST /campaigns
func (h *CampaignHandlers) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var req campaignv1.CreateCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	req.ClientId = clientID.String()

	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("name обязателен"))
		return
	}

	resp, err := h.campaignClient.CreateCampaign(r.Context(), &req)
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, resp)
}

// ListCampaigns обрабатывает GET /campaigns
func (h *CampaignHandlers) ListCampaigns(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	page, perPage := parsePagination(r)
	offset := (page - 1) * perPage
	status := r.URL.Query().Get("status")

	resp, err := h.campaignClient.ListCampaigns(r.Context(), &campaignv1.ListCampaignsRequest{
		ClientId: clientID.String(),
		Status:   status,
		Limit:    perPage,
		Offset:   offset,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetCampaign обрабатывает GET /campaigns/{id}
func (h *CampaignHandlers) GetCampaign(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	resp, err := h.campaignClient.GetCampaign(r.Context(), &campaignv1.GetCampaignRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// UpdateCampaign обрабатывает PUT /campaigns/{id}
func (h *CampaignHandlers) UpdateCampaign(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var req campaignv1.UpdateCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	req.Id = mux.Vars(r)["id"]
	req.ClientId = clientID.String()

	resp, err := h.campaignClient.UpdateCampaign(r.Context(), &req)
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// DeleteCampaign обрабатывает DELETE /campaigns/{id}
func (h *CampaignHandlers) DeleteCampaign(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	_, err := h.campaignClient.DeleteCampaign(r.Context(), &campaignv1.DeleteCampaignRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// LaunchCampaign обрабатывает POST /campaigns/{id}/launch
func (h *CampaignHandlers) LaunchCampaign(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	resp, err := h.campaignClient.LaunchCampaign(r.Context(), &campaignv1.LaunchCampaignRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// PauseCampaign обрабатывает POST /campaigns/{id}/pause
func (h *CampaignHandlers) PauseCampaign(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	resp, err := h.campaignClient.PauseCampaign(r.Context(), &campaignv1.PauseCampaignRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// ResumeCampaign обрабатывает POST /campaigns/{id}/resume
func (h *CampaignHandlers) ResumeCampaign(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	resp, err := h.campaignClient.ResumeCampaign(r.Context(), &campaignv1.ResumeCampaignRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// CancelCampaign обрабатывает POST /campaigns/{id}/cancel
func (h *CampaignHandlers) CancelCampaign(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	resp, err := h.campaignClient.CancelCampaign(r.Context(), &campaignv1.CancelCampaignRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// SetVariants обрабатывает PUT /campaigns/{id}/variants
func (h *CampaignHandlers) SetVariants(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	var req struct {
		Variants []*campaignv1.VariantInput `json:"variants"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	resp, err := h.campaignClient.SetVariants(r.Context(), &campaignv1.SetVariantsRequest{
		CampaignId: id,
		ClientId:   clientID.String(),
		Variants:   req.Variants,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// SetABConfig обрабатывает PUT /campaigns/{id}/ab-config
func (h *CampaignHandlers) SetABConfig(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	var req struct {
		Metric            string `json:"metric"`
		TestDurationHours int32  `json:"test_duration_hours"`
		AutoSelectWinner  bool   `json:"auto_select_winner"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	resp, err := h.campaignClient.SetABConfig(r.Context(), &campaignv1.SetABConfigRequest{
		CampaignId:        id,
		ClientId:          clientID.String(),
		Metric:            req.Metric,
		TestDurationHours: req.TestDurationHours,
		AutoSelectWinner:  req.AutoSelectWinner,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// SelectWinner обрабатывает POST /campaigns/{id}/select-winner
func (h *CampaignHandlers) SelectWinner(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	var req struct {
		VariantId string `json:"variant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	resp, err := h.campaignClient.SelectWinner(r.Context(), &campaignv1.SelectWinnerRequest{
		CampaignId: id,
		ClientId:   clientID.String(),
		VariantId:  req.VariantId,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// SetRetryConfig обрабатывает PUT /campaigns/{id}/retry-config
func (h *CampaignHandlers) SetRetryConfig(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	var config campaignv1.RetryConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	resp, err := h.campaignClient.SetRetryConfig(r.Context(), &campaignv1.SetRetryConfigRequest{
		CampaignId: id,
		ClientId:   clientID.String(),
		Config:     &config,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// RetryFailed обрабатывает POST /campaigns/{id}/retry
func (h *CampaignHandlers) RetryFailed(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	var req struct {
		AlternativeTemplateId string `json:"alternative_template_id"`
	}
	// Body может быть пустым
	json.NewDecoder(r.Body).Decode(&req)

	resp, err := h.campaignClient.RetryFailed(r.Context(), &campaignv1.RetryFailedRequest{
		CampaignId:            id,
		ClientId:              clientID.String(),
		AlternativeTemplateId: req.AlternativeTemplateId,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// PreviewTemplate обрабатывает POST /campaigns/templates/preview
func (h *CampaignHandlers) PreviewTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var req campaignv1.PreviewTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	req.ClientId = clientID.String()

	if req.TemplateText == "" {
		respondError(w, shared.ErrInvalidInput("template_text обязателен"))
		return
	}

	resp, err := h.campaignClient.PreviewTemplate(r.Context(), &req)
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetCampaignStats обрабатывает GET /campaigns/{id}/stats
func (h *CampaignHandlers) GetCampaignStats(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	resp, err := h.campaignClient.GetCampaignStats(r.Context(), &campaignv1.GetCampaignStatsRequest{
		CampaignId: id,
		ClientId:   clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetTimeline обрабатывает GET /campaigns/{id}/timeline
func (h *CampaignHandlers) GetTimeline(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]
	interval := r.URL.Query().Get("interval")
	metric := r.URL.Query().Get("metric")

	resp, err := h.campaignClient.GetCampaignTimeline(r.Context(), &campaignv1.GetCampaignTimelineRequest{
		CampaignId: id,
		ClientId:   clientID.String(),
		Interval:   interval,
		Metric:     metric,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetVariantComparison обрабатывает GET /campaigns/{id}/variants/compare
func (h *CampaignHandlers) GetVariantComparison(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	resp, err := h.campaignClient.GetVariantComparison(r.Context(), &campaignv1.GetVariantComparisonRequest{
		CampaignId: id,
		ClientId:   clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetHeatmap обрабатывает GET /campaigns/{id}/heatmap
func (h *CampaignHandlers) GetHeatmap(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	resp, err := h.campaignClient.GetDeliveryHeatmap(r.Context(), &campaignv1.GetDeliveryHeatmapRequest{
		CampaignId: id,
		ClientId:   clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetOptimalSendTime обрабатывает GET /campaigns/{id}/optimal-time
func (h *CampaignHandlers) GetOptimalSendTime(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	resp, err := h.campaignClient.GetOptimalSendTime(r.Context(), &campaignv1.GetOptimalSendTimeRequest{
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// ExportReport обрабатывает GET /campaigns/{id}/report
func (h *CampaignHandlers) ExportReport(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "csv"
	}

	resp, err := h.campaignClient.ExportReport(r.Context(), &campaignv1.ExportReportRequest{
		CampaignId: id,
		ClientId:   clientID.String(),
		Format:     format,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	// Устанавливаем заголовки для скачивания файла
	w.Header().Set("Content-Type", resp.ContentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+resp.Filename+"\"")
	w.WriteHeader(http.StatusOK)
	w.Write(resp.Data)
}
