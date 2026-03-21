package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	routingv1 "github.com/smpp-server/smpp-server/api/proto/routingv1"
	"github.com/smpp-server/smpp-server/internal/gateway/client/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// LookupHandlers содержит handlers для операций Number Lookup (HLR/MNP)
type LookupHandlers struct {
	routingClient routingv1.RoutingServiceClient
}

// NewLookupHandlers создает новый LookupHandlers
func NewLookupHandlers(routingClient routingv1.RoutingServiceClient) *LookupHandlers {
	return &LookupHandlers{routingClient: routingClient}
}

// SingleLookupRequest представляет запрос на проверку одного номера
type SingleLookupRequest struct {
	MSISDN       string `json:"msisdn"`
	ForceRefresh bool   `json:"force_refresh"`
}

// BulkLookupRequest представляет запрос на массовую проверку номеров
type BulkLookupRequest struct {
	MSISDNs      []string `json:"msisdns"`
	ForceRefresh bool     `json:"force_refresh"`
}

// SingleLookup обрабатывает POST /api/v1/lookup
func (h *LookupHandlers) SingleLookup(w http.ResponseWriter, r *http.Request) {
	var req SingleLookupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	// Получаем client_id из контекста
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	// Валидация
	if req.MSISDN == "" {
		respondError(w, shared.ErrInvalidInput("Поле msisdn обязательно"))
		return
	}

	// Генерируем request_id
	requestID := uuid.New().String()

	// Вызываем Routing Service
	resp, err := h.routingClient.NumberLookup(r.Context(), &routingv1.NumberLookupRequest{
		Msisdn:       req.MSISDN,
		ClientId:     clientID.String(),
		ForceRefresh: req.ForceRefresh,
		RequestId:    requestID,
	})
	if err != nil {
		log.Error().Err(err).Str("msisdn", req.MSISDN).Msg("ошибка выполнения number lookup")
		respondGRPCError(w, err)
		return
	}

	// Формируем ответ
	response := map[string]interface{}{
		"msisdn":          resp.Msisdn,
		"operator_mccmnc": resp.OperatorMccmnc,
		"operator_name":   resp.OperatorName,
		"number_status":   resp.NumberStatus.String(),
		"country_code":    resp.CountryCode,
		"number_type":     resp.NumberType.String(),
		"is_ported":       resp.IsPorted,
		"cached":          resp.Cached,
	}

	if resp.OriginalOperatorMccmnc != "" {
		response["original_operator_mccmnc"] = resp.OriginalOperatorMccmnc
	}
	if resp.QueriedAt != nil {
		response["queried_at"] = resp.QueriedAt.AsTime()
	}

	respondJSON(w, http.StatusOK, response)
}

// BulkLookup обрабатывает POST /api/v1/lookup/bulk
func (h *LookupHandlers) BulkLookup(w http.ResponseWriter, r *http.Request) {
	var req BulkLookupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	// Получаем client_id из контекста
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	// Валидация
	if len(req.MSISDNs) == 0 {
		respondError(w, shared.ErrInvalidInput("Список номеров не может быть пустым"))
		return
	}
	if len(req.MSISDNs) > 1000 {
		respondError(w, shared.ErrInvalidInput("Максимальное количество номеров — 1000"))
		return
	}

	// Генерируем request_id
	requestID := uuid.New().String()

	// Вызываем Routing Service
	resp, err := h.routingClient.BulkNumberLookup(r.Context(), &routingv1.BulkNumberLookupRequest{
		Msisdns:      req.MSISDNs,
		ClientId:     clientID.String(),
		ForceRefresh: req.ForceRefresh,
		RequestId:    requestID,
	})
	if err != nil {
		log.Error().Err(err).Int("count", len(req.MSISDNs)).Msg("ошибка выполнения bulk number lookup")
		respondGRPCError(w, err)
		return
	}

	// Формируем результаты
	results := make([]map[string]interface{}, 0, len(resp.Results))
	for _, result := range resp.Results {
		item := map[string]interface{}{
			"msisdn":          result.Msisdn,
			"operator_mccmnc": result.OperatorMccmnc,
			"operator_name":   result.OperatorName,
			"number_status":   result.NumberStatus.String(),
			"country_code":    result.CountryCode,
			"number_type":     result.NumberType.String(),
			"is_ported":       result.IsPorted,
			"cached":          result.Cached,
		}
		if result.OriginalOperatorMccmnc != "" {
			item["original_operator_mccmnc"] = result.OriginalOperatorMccmnc
		}
		if result.QueriedAt != nil {
			item["queried_at"] = result.QueriedAt.AsTime()
		}
		results = append(results, item)
	}

	response := map[string]interface{}{
		"results":       results,
		"total_count":   resp.TotalCount,
		"success_count": resp.SuccessCount,
		"failed_count":  resp.FailedCount,
	}

	respondJSON(w, http.StatusOK, response)
}

// GetLookupHistory обрабатывает GET /api/v1/lookup/history
func (h *LookupHandlers) GetLookupHistory(w http.ResponseWriter, r *http.Request) {
	// Получаем client_id из контекста
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	// Парсим query параметры
	query := r.URL.Query()

	protoReq := &routingv1.GetLookupHistoryRequest{
		ClientId: clientID.String(),
	}

	// Парсим from/to
	if fromStr := query.Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			protoReq.FromDate = timestamppb.New(t)
		}
	}
	if toStr := query.Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			protoReq.ToDate = timestamppb.New(t)
		}
	}

	// Фильтры
	if msisdn := query.Get("msisdn"); msisdn != "" {
		protoReq.MsisdnFilter = msisdn
	}
	if source := query.Get("source"); source != "" {
		protoReq.SourceFilter = source
	}

	// Пагинация
	page := int32(1)
	if pageStr := query.Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = int32(p)
		}
	}
	protoReq.Page = page

	pageSize := int32(50)
	if pageSizeStr := query.Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = int32(ps)
		}
	}
	protoReq.PageSize = pageSize

	// Вызываем Routing Service
	resp, err := h.routingClient.GetLookupHistory(r.Context(), protoReq)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения истории lookup запросов")
		respondGRPCError(w, err)
		return
	}

	// Формируем ответ
	items := make([]map[string]interface{}, 0, len(resp.Items))
	for _, item := range resp.Items {
		entry := map[string]interface{}{
			"id":              item.Id,
			"msisdn":          item.Msisdn,
			"operator_mccmnc": item.OperatorMccmnc,
			"operator_name":   item.OperatorName,
			"number_status":   item.NumberStatus.String(),
			"country_code":    item.CountryCode,
			"number_type":     item.NumberType.String(),
			"is_ported":       item.IsPorted,
			"source":          item.Source,
			"client_id":       item.ClientId,
			"cached":          item.Cached,
			"latency_ms":      item.LatencyMs,
		}
		if item.RequestId != "" {
			entry["request_id"] = item.RequestId
		}
		if item.MessageId != "" {
			entry["message_id"] = item.MessageId
		}
		if item.CreatedAt != nil {
			entry["created_at"] = item.CreatedAt.AsTime()
		}
		items = append(items, entry)
	}

	response := map[string]interface{}{
		"items":       items,
		"total_count": resp.TotalCount,
		"page":        resp.Page,
		"page_size":   resp.PageSize,
	}

	respondJSON(w, http.StatusOK, response)
}
