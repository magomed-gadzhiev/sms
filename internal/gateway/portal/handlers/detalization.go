package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// ClientMessage is a single row returned by ListMessages.
type ClientMessage struct {
	ID           string     `json:"id"`
	Source       string     `json:"source"`
	Destination  string     `json:"destination"`
	TextPreview  string     `json:"text_preview"`
	Status       string     `json:"status"`
	SegmentCount int        `json:"segment_count"`
	CreatedAt    time.Time  `json:"created_at"`
	ProviderName string     `json:"provider_name"`
	DeliveredAt  *time.Time `json:"delivered_at,omitempty"`
	FailedAt     *time.Time `json:"failed_at,omitempty"`
}

// listMessagesResponse is the typed response for ListMessages.
type listMessagesResponse struct {
	Messages []ClientMessage `json:"messages"`
	Total    int64           `json:"total"`
	Limit    int             `json:"limit"`
	Offset   int             `json:"offset"`
}

// DetalizationHandlers handles client-scoped message detalization (log) requests.
type DetalizationHandlers struct {
	db *pgxpool.Pool
}

// NewDetalizationHandlers creates a new DetalizationHandlers backed by a pgxpool.Pool.
func NewDetalizationHandlers(db *pgxpool.Pool) *DetalizationHandlers {
	return &DetalizationHandlers{db: db}
}

// ListMessages handles GET /portal/v1/detalization
// Query params: status, source, destination, date_from (YYYY-MM-DD), date_to (YYYY-MM-DD), limit, offset
func (h *DetalizationHandlers) ListMessages(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	q := r.URL.Query()
	status := q.Get("status")
	source := q.Get("source")
	destination := q.Get("destination")
	dateFrom := q.Get("date_from")
	dateTo := q.Get("date_to")

	limit := 50
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 && v <= 200 {
		limit = v
	}
	offset := 0
	if v, err := strconv.Atoi(q.Get("offset")); err == nil && v >= 0 {
		offset = v
	}

	args := []interface{}{clientID.String()}
	conditions := " AND m.client_id = $1::uuid"
	nextArg := func(v interface{}) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if status != "" {
		conditions += " AND m.status::text = " + nextArg(status)
	}
	if source != "" {
		conditions += " AND m.source ILIKE " + nextArg("%"+source+"%")
	}
	if destination != "" {
		conditions += " AND m.destination ILIKE " + nextArg("%"+destination+"%")
	}
	if dateFrom != "" {
		conditions += " AND m.created_at >= " + nextArg(dateFrom) + "::timestamptz"
	}
	if dateTo != "" {
		conditions += " AND m.created_at < (" + nextArg(dateTo) + "::timestamptz + INTERVAL '1 day')"
	}

	countQuery := `
		SELECT COUNT(*)
		FROM messages m
		WHERE 1=1` + conditions

	listQuery := `
		SELECT
			m.id::text,
			COALESCE(m.source, '')           AS source,
			COALESCE(m.destination, '')      AS destination,
			LEFT(COALESCE(m.text, ''), 100)  AS text_preview,
			m.status::text,
			COALESCE(m.segment_count, 0)     AS segment_count,
			m.created_at,
			m.delivered_at,
			m.failed_at,
			COALESCE(p.name, '')             AS provider_name
		FROM messages m
		LEFT JOIN providers p ON p.id = m.provider_id
		WHERE 1=1` + conditions + `
		ORDER BY m.created_at DESC
		LIMIT ` + strconv.Itoa(limit) + ` OFFSET ` + strconv.Itoa(offset)

	ctx := r.Context()

	var total int64
	if err := h.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка подсчёта сообщений"))
		return
	}

	rows, err := h.db.Query(ctx, listQuery, args...)
	if err != nil {
		log.Error().Err(err).Msg("detalization: ошибка запроса сообщений")
		respondError(w, shared.ErrInternalServer("Ошибка получения сообщений"))
		return
	}
	defer rows.Close()

	messages := []ClientMessage{}
	for rows.Next() {
		var (
			id, src, dst, textPreview, st, providerName string
			segmentCount                                  int
			createdAt                                     time.Time
			deliveredAt, failedAt                         *time.Time
		)
		if err := rows.Scan(
			&id, &src, &dst, &textPreview, &st,
			&segmentCount, &createdAt, &deliveredAt, &failedAt,
			&providerName,
		); err != nil {
			log.Error().Err(err).Msg("detalization: ошибка сканирования строки")
			continue
		}
		msg := ClientMessage{
			ID:           id,
			Source:       src,
			Destination:  dst,
			TextPreview:  textPreview,
			Status:       st,
			SegmentCount: segmentCount,
			CreatedAt:    createdAt,
			ProviderName: providerName,
			DeliveredAt:  deliveredAt,
			FailedAt:     failedAt,
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка итерации строк"))
		return
	}

	respondJSON(w, http.StatusOK, listMessagesResponse{
		Messages: messages,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	})
}

// GetMessage handles GET /portal/v1/detalization/{id}
func (h *DetalizationHandlers) GetMessage(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID сообщения обязателен"))
		return
	}

	ctx := r.Context()

	const msgQuery = `
		SELECT
			m.id::text,
			COALESCE(m.source, '')          AS source,
			COALESCE(m.destination, '')     AS destination,
			COALESCE(m.text, '')            AS text,
			COALESCE(m.encoding, 'GSM7')    AS encoding,
			m.status::text,
			COALESCE(m.status_message, '')  AS status_message,
			COALESCE(m.external_id, '')     AS external_id,
			COALESCE(m.segment_count, 0)    AS segment_count,
			COALESCE(m.retry_count, 0)      AS retry_count,
			COALESCE(m.max_retries, 0)      AS max_retries,
			COALESCE(m.provider_id::text, '') AS provider_id,
			COALESCE(m.route_id::text, '')    AS route_id,
			COALESCE(m.smpp_message_id, '')   AS smpp_message_id,
			m.created_at,
			m.submitted_at,
			m.delivered_at,
			m.failed_at,
			m.scheduled_at,
			m.expired_at,
			COALESCE(p.name, '') AS provider_name,
			COALESCE(r.name, '') AS route_name
		FROM messages m
		LEFT JOIN providers p ON p.id = m.provider_id
		LEFT JOIN client_routes r ON r.id = m.route_id
		WHERE m.id = $1::uuid AND m.client_id = $2::uuid
		LIMIT 1
	`

	var (
		msgID         string
		source        string
		destination   string
		text          string
		encoding      string
		status        string
		statusMessage string
		externalID    string
		segmentCount  int32
		retryCount    int32
		maxRetries    int32
		providerID    string
		routeID       string
		smppMessageID string
		createdAt     *time.Time
		submittedAt   *time.Time
		deliveredAt   *time.Time
		failedAt      *time.Time
		scheduledAt   *time.Time
		expiredAt     *time.Time
		providerName  string
		routeName     string
	)

	err := h.db.QueryRow(ctx, msgQuery, id, clientID.String()).Scan(
		&msgID, &source, &destination, &text, &encoding,
		&status, &statusMessage, &externalID,
		&segmentCount, &retryCount, &maxRetries,
		&providerID, &routeID, &smppMessageID,
		&createdAt, &submittedAt, &deliveredAt, &failedAt, &scheduledAt, &expiredAt,
		&providerName, &routeName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, shared.ErrNotFound("Сообщение не найдено"))
		} else {
			log.Error().Err(err).Str("id", id).Msg("detalization: ошибка запроса сообщения")
			respondError(w, shared.ErrInternalServer("Ошибка получения сообщения"))
		}
		return
	}

	result := map[string]interface{}{
		"id":            msgID,
		"source":        source,
		"destination":   destination,
		"text":          text,
		"encoding":      encoding,
		"status":        status,
		"segment_count": segmentCount,
		"retry_count":   retryCount,
		"max_retries":   maxRetries,
	}
	if createdAt != nil {
		result["created_at"] = *createdAt
	}
	if statusMessage != "" {
		result["status_message"] = statusMessage
	}
	if externalID != "" {
		result["external_id"] = externalID
	}
	if smppMessageID != "" {
		result["smpp_message_id"] = smppMessageID
	}
	if providerID != "" {
		result["provider_id"] = providerID
		result["provider_name"] = providerName
	}
	if routeID != "" {
		result["route_id"] = routeID
		result["route_name"] = routeName
	}
	if submittedAt != nil {
		result["submitted_at"] = *submittedAt
	}
	if deliveredAt != nil {
		result["delivered_at"] = *deliveredAt
	}
	if failedAt != nil {
		result["failed_at"] = *failedAt
	}
	if scheduledAt != nil {
		result["scheduled_at"] = *scheduledAt
	}
	if expiredAt != nil {
		result["expired_at"] = *expiredAt
	}

	// DLR and billing queries use message_id only; client ownership already
	// verified above by AND m.client_id = $2::uuid.

	// DLR receipt (most recent)
	const dlrQuery = `
		SELECT stat, COALESCE(err, 0), COALESCE(text, ''),
		       submit_date, done_date,
		       COALESCE(receipted_message_id, '')
		FROM dlr_receipts
		WHERE message_id = $1::uuid
		ORDER BY created_at DESC
		LIMIT 1
	`
	var (
		dlrStat               string
		dlrErr                int32
		dlrText               string
		dlrSubmitDate         *time.Time
		dlrDoneDate           *time.Time
		dlrReceiptedMessageID string
	)
	dlrScanErr := h.db.QueryRow(ctx, dlrQuery, id).Scan(
		&dlrStat, &dlrErr, &dlrText, &dlrSubmitDate, &dlrDoneDate, &dlrReceiptedMessageID,
	)
	if dlrScanErr == nil {
		dlr := map[string]interface{}{
			"stat": dlrStat,
			"err":  dlrErr,
			"text": dlrText,
		}
		if dlrSubmitDate != nil {
			dlr["submit_date"] = *dlrSubmitDate
		}
		if dlrDoneDate != nil {
			dlr["done_date"] = *dlrDoneDate
		}
		if dlrReceiptedMessageID != "" {
			dlr["receipted_message_id"] = dlrReceiptedMessageID
		}
		result["dlr"] = dlr
	} else if !errors.Is(dlrScanErr, pgx.ErrNoRows) {
		log.Warn().Err(dlrScanErr).Msg("detalization: ошибка получения DLR receipt")
	}

	// Billing from tarification_log
	const billingQuery = `
		SELECT segment_count, price_per_segment, total_amount,
		       tariff_plan_id::text, created_at
		FROM tarification_log
		WHERE message_id = $1::uuid
		ORDER BY created_at DESC
		LIMIT 1
	`
	var (
		billSegmentCount    int32
		billPricePerSegment float64
		billTotalAmount     float64
		billTariffPlanID    string
		billCreatedAt       *time.Time
	)
	billScanErr := h.db.QueryRow(ctx, billingQuery, id).Scan(
		&billSegmentCount, &billPricePerSegment, &billTotalAmount, &billTariffPlanID, &billCreatedAt,
	)
	if billScanErr == nil {
		billing := map[string]interface{}{
			"segment_count":     billSegmentCount,
			"price_per_segment": billPricePerSegment,
			"total_amount":      billTotalAmount,
			"tariff_plan_id":    billTariffPlanID,
		}
		if billCreatedAt != nil {
			billing["billed_at"] = *billCreatedAt
		}
		result["billing"] = billing
	} else if !errors.Is(billScanErr, pgx.ErrNoRows) {
		log.Warn().Err(billScanErr).Msg("detalization: ошибка получения данных тарификации")
	}

	respondJSON(w, http.StatusOK, result)
}
