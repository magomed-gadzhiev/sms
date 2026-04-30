package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// DetalizationHandlers обрабатывает запросы для раздела "Детализация" (лог сообщений).
type DetalizationHandlers struct {
	db *storage.DB
}

// NewDetalizationHandlers создаёт обработчик с прямым доступом к БД.
func NewDetalizationHandlers(db *storage.DB) *DetalizationHandlers {
	return &DetalizationHandlers{db: db}
}

// markDeprecated добавляет HTTP-заголовки RFC 8594 (Sunset) и draft Deprecation,
// чтобы внешние клиенты узнавали об устаревании /admin/v1/messages*.
// Раздел "Детализация" в UI заменён на /messages (см. MVP-feedback №10/11, 5424ee2);
// endpoint оставлен функциональным до полного удаления отдельным cleanup-PR.
// BUG-58.
func markDeprecated(w http.ResponseWriter) {
	w.Header().Set("Deprecation", "true")
	w.Header().Set("Link", "</admin/v1/messages>; rel=\"deprecation\", </messages>; rel=\"successor-version\"")
}

// ListMessages обрабатывает GET /admin/v1/messages
// Query params: client_id, status, source, destination, provider_id, date_from, date_to, limit, offset
func (h *DetalizationHandlers) ListMessages(w http.ResponseWriter, r *http.Request) {
	markDeprecated(w)
	q := r.URL.Query()
	clientID := q.Get("client_id")
	status := q.Get("status")
	source := q.Get("source")
	destination := q.Get("destination")
	providerID := q.Get("provider_id")
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

	args := []interface{}{}
	conditions := ""
	nextArg := func(v interface{}) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if clientID != "" {
		conditions += " AND m.client_id = " + nextArg(clientID) + "::uuid"
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
	if providerID != "" {
		conditions += " AND m.provider_id = " + nextArg(providerID) + "::uuid"
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
			COALESCE(p.name, '')             AS provider_name,
			COALESCE(c.name, '')             AS client_name
		FROM messages m
		LEFT JOIN providers p ON p.id = m.provider_id
		LEFT JOIN clients c ON c.id = m.client_id
		WHERE 1=1` + conditions + `
		ORDER BY m.created_at DESC
		LIMIT ` + strconv.Itoa(limit) + ` OFFSET ` + strconv.Itoa(offset)

	ctx := r.Context()

	var total int64
	_ = h.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)

	rows, err := h.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		log.Error().Err(err).Msg("detalization: ошибка запроса сообщений")
		respondError(w, shared.ErrInternalServer("Ошибка получения сообщений"))
		return
	}
	defer rows.Close()

	items := []map[string]interface{}{}
	for rows.Next() {
		var (
			id, src, dst, textPreview, st, providerName, clientName string
			segmentCount                                              int
			createdAt                                                 time.Time
			deliveredAt, failedAt                                     sql.NullTime
		)
		if err := rows.Scan(
			&id, &src, &dst, &textPreview, &st,
			&segmentCount, &createdAt, &deliveredAt, &failedAt,
			&providerName, &clientName,
		); err != nil {
			log.Error().Err(err).Msg("detalization: ошибка сканирования строки")
			continue
		}
		item := map[string]interface{}{
			"id":            id,
			"source":        src,
			"destination":   dst,
			"text_preview":  textPreview,
			"status":        st,
			"segment_count": segmentCount,
			"created_at":    createdAt,
			"provider_name": providerName,
			"client_name":   clientName,
		}
		if deliveredAt.Valid {
			item["delivered_at"] = deliveredAt.Time
		}
		if failedAt.Valid {
			item["failed_at"] = failedAt.Time
		}
		items = append(items, item)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"messages": items,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// GetMessage обрабатывает GET /admin/v1/messages/{id}
func (h *DetalizationHandlers) GetMessage(w http.ResponseWriter, r *http.Request) {
	markDeprecated(w)
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
			COALESCE(p.name, '')   AS provider_name,
			COALESCE(r.name, '')   AS route_name,
			COALESCE(c.name, '')   AS client_name,
			COALESCE(m.client_id::text, '') AS client_id
		FROM messages m
		LEFT JOIN providers p ON p.id = m.provider_id
		LEFT JOIN client_routes r ON r.id = m.route_id
		LEFT JOIN clients c ON c.id = m.client_id
		WHERE m.id = $1::uuid
		LIMIT 1
	`

	var (
		msgID, source, destination, text, encoding   string
		status, statusMessage, externalID             string
		providerID, routeID, smppMessageID            string
		providerName, routeName, clientName, clientID string
		segmentCount, retryCount, maxRetries           int
		createdAt                                      time.Time
		submittedAt, deliveredAt, failedAt             sql.NullTime
		scheduledAt, expiredAt                         sql.NullTime
	)

	err := h.db.QueryRowContext(ctx, msgQuery, id).Scan(
		&msgID, &source, &destination, &text, &encoding,
		&status, &statusMessage, &externalID,
		&segmentCount, &retryCount, &maxRetries,
		&providerID, &routeID, &smppMessageID,
		&createdAt, &submittedAt, &deliveredAt, &failedAt,
		&scheduledAt, &expiredAt,
		&providerName, &routeName, &clientName, &clientID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, shared.ErrNotFound("Сообщение не найдено"))
			return
		}
		log.Error().Err(err).Str("id", id).Msg("detalization: ошибка запроса сообщения")
		respondError(w, shared.ErrInternalServer("Ошибка получения сообщения"))
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
		"created_at":    createdAt,
		"client_id":     clientID,
		"client_name":   clientName,
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
	if submittedAt.Valid {
		result["submitted_at"] = submittedAt.Time
	}
	if deliveredAt.Valid {
		result["delivered_at"] = deliveredAt.Time
	}
	if failedAt.Valid {
		result["failed_at"] = failedAt.Time
	}
	if scheduledAt.Valid {
		result["scheduled_at"] = scheduledAt.Time
	}
	if expiredAt.Valid {
		result["expired_at"] = expiredAt.Time
	}

	// DLR receipt
	const dlrQuery = `
		SELECT stat, COALESCE(err, 0), COALESCE(text, ''),
		       submit_date, done_date,
		       COALESCE(receipted_message_id, '')
		FROM dlr_receipts
		WHERE message_id = $1::uuid
		ORDER BY created_at DESC
		LIMIT 1
	`
	var dlrStat, dlrText, dlrReceiptedID string
	var dlrErr int
	var dlrSubmitDate, dlrDoneDate sql.NullTime
	if err := h.db.QueryRowContext(ctx, dlrQuery, id).Scan(
		&dlrStat, &dlrErr, &dlrText, &dlrSubmitDate, &dlrDoneDate, &dlrReceiptedID,
	); err == nil {
		dlr := map[string]interface{}{
			"stat": dlrStat,
			"err":  dlrErr,
			"text": dlrText,
		}
		if dlrSubmitDate.Valid {
			dlr["submit_date"] = dlrSubmitDate.Time
		}
		if dlrDoneDate.Valid {
			dlr["done_date"] = dlrDoneDate.Time
		}
		if dlrReceiptedID != "" {
			dlr["receipted_message_id"] = dlrReceiptedID
		}
		result["dlr"] = dlr
	}

	// Billing
	const billingQuery = `
		SELECT segment_count, price_per_segment::text, total_amount::text,
		       tariff_plan_id::text, source_rule_id::text, created_at
		FROM tarification_log
		WHERE message_id = $1::uuid
		LIMIT 1
	`
	var bSegments int
	var bPricePerSeg, bTotal string
	var bPlanID, bSourceRuleID *string
	var bCreatedAt time.Time
	if err := h.db.QueryRowContext(ctx, billingQuery, id).Scan(
		&bSegments, &bPricePerSeg, &bTotal,
		&bPlanID, &bSourceRuleID, &bCreatedAt,
	); err == nil {
		result["billing"] = map[string]interface{}{
			"segment_count":     bSegments,
			"price_per_segment": bPricePerSeg,
			"total_amount":      bTotal,
			"tariff_plan_id":    bPlanID,    // *string → nil serialises as JSON null
			"source_rule_id":    bSourceRuleID, // new; client shows "Unified (rule: …)" when plan_id is null
			"billed_at":         bCreatedAt,
		}
	}

	respondJSON(w, http.StatusOK, result)
}
