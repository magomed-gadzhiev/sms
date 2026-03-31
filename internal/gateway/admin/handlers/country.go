package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/api/proto/routingv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// CountryHandler обрабатывает HTTP запросы для управления странами
type CountryHandler struct {
	routingClient routingv1.RoutingServiceClient
}

// NewCountryHandler создает новый экземпляр CountryHandler
func NewCountryHandler(routingClient routingv1.RoutingServiceClient) *CountryHandler {
	return &CountryHandler{
		routingClient: routingClient,
	}
}

// CreateCountry обрабатывает POST /admin/v1/countries
func (h *CountryHandler) CreateCountry(w http.ResponseWriter, r *http.Request) {
	var req CreateCountryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if err := req.Validate(); err != nil {
		respondError(w, err.(*shared.AppError))
		return
	}

	grpcReq := &routingv1.CreateCountryRequest{
		Name:      req.Name,
		IsoCode:   req.ISOCode,
		PhoneCode: req.PhoneCode,
		Currency:  req.Currency,
	}

	resp, err := h.routingClient.CreateCountry(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания страны")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, countryProtoToResponse(resp))
}

// GetCountry обрабатывает GET /admin/v1/countries/{id}
func (h *CountryHandler) GetCountry(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("id обязателен"))
		return
	}

	resp, err := h.routingClient.GetCountry(r.Context(), &routingv1.GetCountryRequest{
		Id: id,
	})
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("ошибка получения страны")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, countryProtoToResponse(resp))
}

// ListCountries обрабатывает GET /admin/v1/countries
func (h *CountryHandler) ListCountries(w http.ResponseWriter, r *http.Request) {
	limit := parseInt(r.URL.Query().Get("limit"), 50)
	offset := parseInt(r.URL.Query().Get("offset"), 0)

	resp, err := h.routingClient.ListCountries(r.Context(), &routingv1.ListCountriesRequest{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка стран")
		respondGRPCError(w, err)
		return
	}

	countries := make([]CountryResponse, len(resp.Countries))
	for i, c := range resp.Countries {
		countries[i] = countryProtoToResponse(c)
	}

	respondJSON(w, http.StatusOK, ListCountriesResponse{
		Countries: countries,
		Total:     int(resp.Total),
		Limit:     limit,
		Offset:    offset,
	})
}

// UpdateCountry обрабатывает PUT /admin/v1/countries/{id}
func (h *CountryHandler) UpdateCountry(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("id обязателен"))
		return
	}

	var req UpdateCountryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	grpcReq := &routingv1.UpdateCountryRequest{
		Id:        id,
		Name:      req.Name,
		IsoCode:   req.ISOCode,
		PhoneCode: req.PhoneCode,
		Currency:  req.Currency,
	}

	resp, err := h.routingClient.UpdateCountry(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("ошибка обновления страны")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, countryProtoToResponse(resp))
}

// Типы запросов и ответов

type CreateCountryRequest struct {
	Name      string `json:"name"`
	ISOCode   string `json:"iso_code"`
	PhoneCode string `json:"phone_code"`
	Currency  string `json:"currency"`
}

func (r *CreateCountryRequest) Validate() error {
	if r.Name == "" {
		return shared.ErrInvalidInput("name обязателен")
	}
	if r.ISOCode == "" {
		return shared.ErrInvalidInput("iso_code обязателен")
	}
	if r.PhoneCode == "" {
		return shared.ErrInvalidInput("phone_code обязателен")
	}
	if r.Currency == "" {
		r.Currency = "RUB"
	}
	return nil
}

type UpdateCountryRequest struct {
	Name      string `json:"name,omitempty"`
	ISOCode   string `json:"iso_code,omitempty"`
	PhoneCode string `json:"phone_code,omitempty"`
	Currency  string `json:"currency,omitempty"`
}

type CountryResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ISOCode   string    `json:"iso_code"`
	PhoneCode string    `json:"phone_code"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ListCountriesResponse struct {
	Countries []CountryResponse `json:"countries"`
	Total     int               `json:"total"`
	Limit     int               `json:"limit"`
	Offset    int               `json:"offset"`
}

func countryProtoToResponse(c *routingv1.Country) CountryResponse {
	resp := CountryResponse{
		ID:        c.Id,
		Name:      c.Name,
		ISOCode:   c.IsoCode,
		PhoneCode: c.PhoneCode,
		Currency:  c.Currency,
	}
	if c.CreatedAt != nil {
		resp.CreatedAt = c.CreatedAt.AsTime()
	}
	if c.UpdatedAt != nil {
		resp.UpdatedAt = c.UpdatedAt.AsTime()
	}
	return resp
}
