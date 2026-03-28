package handlers

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	routingv1 "github.com/smpp-server/smpp-server/api/proto/routingv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// LookupHandlers содержит handlers для операций Number Lookup в портале
type LookupHandlers struct {
	routingClient routingv1.RoutingServiceClient
}

// NewLookupHandlers создает новый LookupHandlers
func NewLookupHandlers(routingClient routingv1.RoutingServiceClient) *LookupHandlers {
	return &LookupHandlers{routingClient: routingClient}
}

// GetLookupHistory обрабатывает GET /portal/v1/lookup/history
func (h *LookupHandlers) GetLookupHistory(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

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

// numberLookupRequest представляет запрос на проверку номера
type numberLookupRequest struct {
	Phone string `json:"phone"`
}

// NumberLookup обрабатывает POST /portal/v1/lookup/number
func (h *LookupHandlers) NumberLookup(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	var req numberLookupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Phone == "" {
		respondError(w, shared.ErrInvalidInput("Поле phone обязательно"))
		return
	}

	resp, err := h.routingClient.NumberLookup(r.Context(), &routingv1.NumberLookupRequest{
		Msisdn:   req.Phone,
		ClientId: clientID.String(),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка number lookup")
		respondGRPCError(w, err)
		return
	}

	result := map[string]interface{}{
		"msisdn":           resp.Msisdn,
		"operator_mccmnc":  resp.OperatorMccmnc,
		"operator_name":    resp.OperatorName,
		"number_status":    resp.NumberStatus.String(),
		"country_code":     resp.CountryCode,
		"number_type":      resp.NumberType.String(),
		"is_ported":        resp.IsPorted,
		"cached":           resp.Cached,
	}
	if resp.QueriedAt != nil {
		result["queried_at"] = resp.QueriedAt.AsTime()
	}

	respondJSON(w, http.StatusOK, result)
}

// BulkLookup обрабатывает POST /portal/v1/lookup/bulk
func (h *LookupHandlers) BulkLookup(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		respondError(w, shared.ErrInvalidInput("Ошибка парсинга формы"))
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		respondError(w, shared.ErrInvalidInput("Файл не найден в запросе"))
		return
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	var msisdns []string
	for {
		record, err := csvReader.Read()
		if err != nil {
			break
		}
		if len(record) > 0 && record[0] != "" {
			msisdns = append(msisdns, strings.TrimSpace(record[0]))
		}
	}

	if len(msisdns) == 0 {
		respondError(w, shared.ErrInvalidInput("CSV файл пустой"))
		return
	}
	if len(msisdns) > 10000 {
		respondError(w, shared.ErrInvalidInput("Максимум 10 000 номеров"))
		return
	}

	resp, err := h.routingClient.BulkNumberLookup(r.Context(), &routingv1.BulkNumberLookupRequest{
		Msisdns:  msisdns,
		ClientId: clientID.String(),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка bulk lookup")
		respondGRPCError(w, err)
		return
	}

	results := make([]map[string]interface{}, 0, len(resp.Results))
	for _, lr := range resp.Results {
		results = append(results, map[string]interface{}{
			"msisdn":          lr.Msisdn,
			"operator_mccmnc": lr.OperatorMccmnc,
			"operator_name":   lr.OperatorName,
			"number_status":   lr.NumberStatus.String(),
			"country_code":    lr.CountryCode,
			"number_type":     lr.NumberType.String(),
			"is_ported":       lr.IsPorted,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"results":       results,
		"total_count":   resp.TotalCount,
		"success_count": resp.SuccessCount,
		"failed_count":  resp.FailedCount,
	})
}

// GetLookupStats обрабатывает GET /portal/v1/lookup/stats
func (h *LookupHandlers) GetLookupStats(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	query := r.URL.Query()

	// Определяем период для статистики
	now := time.Now()
	var dateFrom, dateTo time.Time

	period := query.Get("period")
	if period == "" {
		period = "7d"
	}

	dateTo = now
	switch period {
	case "30d":
		dateFrom = now.AddDate(0, 0, -30)
	case "90d":
		dateFrom = now.AddDate(0, 0, -90)
	default: // "7d"
		dateFrom = now.AddDate(0, 0, -7)
	}

	// Получаем историю за период для подсчёта статистики
	resp, err := h.routingClient.GetLookupHistory(r.Context(), &routingv1.GetLookupHistoryRequest{
		ClientId: clientID.String(),
		FromDate: timestamppb.New(dateFrom),
		ToDate:   timestamppb.New(dateTo),
		Page:     1,
		PageSize: 1, // Нам нужен только total_count
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения статистики lookup запросов")
		respondGRPCError(w, err)
		return
	}

	response := map[string]interface{}{
		"period":       period,
		"total_lookups": resp.TotalCount,
		"date_from":    dateFrom,
		"date_to":      dateTo,
	}

	respondJSON(w, http.StatusOK, response)
}
