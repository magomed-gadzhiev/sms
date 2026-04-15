package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
	routingv1 "github.com/smpp-server/smpp-server/api/proto/routingv1"
	sendernamev1 "github.com/smpp-server/smpp-server/api/proto/sendernamev1"
	tarificationv1 "github.com/smpp-server/smpp-server/api/proto/tarificationv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type SenderNameHandlers struct {
	client        sendernamev1.SenderNameServiceClient
	routingClient routingv1.RoutingServiceClient
	tariffClient  tarificationv1.TarificationServiceClient
	billingClient billingv1.BillingServiceClient
	pool          *pgxpool.Pool
}

func NewSenderNameHandlers(client sendernamev1.SenderNameServiceClient) *SenderNameHandlers {
	return &SenderNameHandlers{client: client}
}

// SetPool устанавливает пул соединений для прямых SQL-запросов
func (h *SenderNameHandlers) SetPool(pool *pgxpool.Pool) {
	h.pool = pool
}

// SetBillingClients устанавливает gRPC клиенты для тарификации
func (h *SenderNameHandlers) SetBillingClients(
	routingClient routingv1.RoutingServiceClient,
	tariffClient tarificationv1.TarificationServiceClient,
	billingClient billingv1.BillingServiceClient,
) {
	h.routingClient = routingClient
	h.tariffClient = tariffClient
	h.billingClient = billingClient
}

type createSenderNameRequest struct {
	Name      string `json:"name"`
	CompanyID string `json:"company_id"`
}

func (h *SenderNameHandlers) CreateSenderName(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	var req createSenderNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("Поле name обязательно"))
		return
	}

	companyID := req.CompanyID
	if companyID == "" {
		if h.pool == nil {
			respondError(w, shared.ErrInternalServer("database pool недоступен"))
			return
		}
		if err := h.pool.QueryRow(r.Context(),
			`SELECT company_id::text FROM client_companies WHERE client_id = $1 AND is_default = TRUE LIMIT 1`,
			clientID,
		).Scan(&companyID); err != nil {
			log.Error().Err(err).Str("client_id", clientID.String()).Msg("не найдена дефолтная компания клиента")
			respondError(w, shared.ErrInvalidInput("у клиента не найдена компания по умолчанию"))
			return
		}
	}

	log.Info().Str("client_id", clientID.String()).Str("name", req.Name).Str("company_id", companyID).Msg("создание имени отправителя: отправка gRPC")

	resp, err := h.client.CreateSenderName(r.Context(), &sendernamev1.CreateSenderNameRequest{
		ClientId:  clientID.String(),
		Name:      req.Name,
		CompanyId: companyID,
	})
	if err != nil {
		log.Error().Err(err).Str("company_id", companyID).Msg("ошибка создания имени отправителя")
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, senderNameToJSON(resp.SenderName))
}

func (h *SenderNameHandlers) ListSenderNames(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	page, perPage := parsePagination(r)
	offset := (page - 1) * perPage
	statusFilter := r.URL.Query().Get("status")

	resp, err := h.client.ListSenderNames(r.Context(), &sendernamev1.ListSenderNamesRequest{
		ClientId: clientID.String(),
		Status:   statusFilter,
		Limit:    perPage,
		Offset:   offset,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка имён отправителей")
		respondGRPCError(w, err)
		return
	}

	items := make([]map[string]interface{}, 0, len(resp.SenderNames))
	for _, sn := range resp.SenderNames {
		items = append(items, senderNameToJSON(sn))
	}
	totalPages := int32(0)
	if perPage > 0 && resp.Total > 0 {
		totalPages = (resp.Total + perPage - 1) / perPage
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sender_names": items,
		"total":        resp.Total,
		"page":         page,
		"per_page":     perPage,
		"total_pages":  totalPages,
	})
}

func (h *SenderNameHandlers) GetSenderName(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}
	resp, err := h.client.GetSenderName(r.Context(), &sendernamev1.GetSenderNameRequest{
		Id: id, ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, senderNameToJSON(resp.SenderName))
}

type updateSenderNameRequest struct {
	Name string `json:"name"`
}

func (h *SenderNameHandlers) UpdateSenderName(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}
	var req updateSenderNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("Поле name обязательно"))
		return
	}
	resp, err := h.client.UpdateSenderName(r.Context(), &sendernamev1.UpdateSenderNameRequest{
		Id: id, ClientId: clientID.String(), Name: req.Name,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, senderNameToJSON(resp.SenderName))
}

func (h *SenderNameHandlers) ResubmitSenderName(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}
	resp, err := h.client.ResubmitSenderName(r.Context(), &sendernamev1.ResubmitSenderNameRequest{
		Id: id, ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, senderNameToJSON(resp.SenderName))
}

func (h *SenderNameHandlers) GetSenderNameHistory(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}
	page, perPage := parsePagination(r)
	offset := (page - 1) * perPage

	resp, err := h.client.GetSenderNameHistory(r.Context(), &sendernamev1.GetSenderNameHistoryRequest{
		SenderNameId: id,
		ClientId:     clientID.String(),
		Limit:        perPage,
		Offset:       offset,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	entries := make([]map[string]interface{}, 0, len(resp.Entries))
	for _, e := range resp.Entries {
		entries = append(entries, historyEntryToJSON(e))
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"total":   resp.Total,
	})
}

func senderNameToJSON(sn *sendernamev1.SenderNameInfo) map[string]interface{} {
	m := map[string]interface{}{
		"id":               sn.Id,
		"client_id":        sn.ClientId,
		"name":             sn.Name,
		"status":           sn.Status,
		"rejection_reason": sn.RejectionReason,
		"reviewer_id":      sn.ReviewerId,
	}
	if sn.ReviewedAt != nil {
		m["reviewed_at"] = sn.ReviewedAt.AsTime()
	} else {
		m["reviewed_at"] = nil
	}
	if sn.CreatedAt != nil {
		m["created_at"] = sn.CreatedAt.AsTime()
	}
	if sn.UpdatedAt != nil {
		m["updated_at"] = sn.UpdatedAt.AsTime()
	}
	return m
}

// CreateSenderRegistration создаёт регистрацию имени отправителя у оператора.
// POST /api/sender-registrations
// Для платных регистраций (type=paid) автоматически создаёт billing record за текущий месяц.
func (h *SenderNameHandlers) CreateSenderRegistration(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	var req struct {
		OperatorID string `json:"operator_id"`
		SenderName string `json:"sender_name"`
		Type       string `json:"type"` // "paid" | "free"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.OperatorID == "" {
		respondError(w, shared.ErrInvalidInput("operator_id обязателен"))
		return
	}
	if req.SenderName == "" {
		respondError(w, shared.ErrInvalidInput("sender_name обязателен"))
		return
	}
	if req.Type == "" {
		req.Type = "free"
	}

	if h.tariffClient == nil {
		respondError(w, shared.ErrInternalServer("tarification service недоступен"))
		return
	}

	regResp, err := h.tariffClient.CreateSenderRegistration(r.Context(), &tarificationv1.CreateSenderRegistrationRequest{
		ClientId:   clientID.String(),
		OperatorId: req.OperatorID,
		SenderName: req.SenderName,
		Type:       req.Type,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания sender registration")
		respondGRPCError(w, err)
		return
	}

	// Для платных регистраций создаём billing record за текущий месяц
	if req.Type == "paid" && h.routingClient != nil {
		op, err := h.routingClient.GetOperator(r.Context(), &routingv1.GetOperatorRequest{Id: req.OperatorID})
		if err != nil {
			log.Error().Err(err).Str("operator_id", req.OperatorID).Msg("не удалось получить тариф оператора для billing")
		} else if op.MonthlyTariffAmount != "" {
			billingResp, billingErr := h.tariffClient.CreateSenderBillingRecord(r.Context(), &tarificationv1.CreateSenderBillingRecordRequest{
				SenderRegistrationId: regResp.Id,
				ClientId:             clientID.String(),
				OperatorId:           req.OperatorID,
				Amount:               op.MonthlyTariffAmount,
			})
			if billingErr != nil {
				log.Error().Err(billingErr).Str("registration_id", regResp.Id).Msg("не удалось создать billing record")
			} else if !billingResp.GetAlreadyExisted() && h.billingClient != nil {
				// Списываем только при первичном создании monthly billing record,
				// чтобы избежать повторных списаний при ретраях.
				_, chargeErr := h.billingClient.DeductCredits(r.Context(), &billingv1.DeductCreditsRequest{
					ClientId:    clientID.String(),
					Amount:      op.MonthlyTariffAmount,
					Currency:    "RUB",
					Description: "Paid sender name monthly fee",
				})
				if chargeErr != nil {
					log.Error().Err(chargeErr).Str("registration_id", regResp.Id).Msg("не удалось списать оплату за платное имя")
				}
			}
		}
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":          regResp.Id,
		"client_id":   regResp.ClientId,
		"operator_id": regResp.OperatorId,
		"sender_name": regResp.SenderName,
		"type":        regResp.Type,
		"status":      regResp.Status,
	})
}

// GetOperatorSenderTariff возвращает тариф оператора для отображения перед регистрацией платного имени.
// GET /api/operators/:id/sender-tariff
func (h *SenderNameHandlers) GetOperatorSenderTariff(w http.ResponseWriter, r *http.Request) {
	if h.routingClient == nil {
		respondError(w, shared.ErrInternalServer("routing service недоступен"))
		return
	}
	operatorID := mux.Vars(r)["id"]
	if operatorID == "" {
		respondError(w, shared.ErrInvalidInput("operator id обязателен"))
		return
	}

	op, err := h.routingClient.GetOperator(r.Context(), &routingv1.GetOperatorRequest{Id: operatorID})
	if err != nil {
		log.Error().Err(err).Str("operator_id", operatorID).Msg("ошибка получения оператора")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"operator_id":           op.Id,
		"monthly_tariff_amount": op.MonthlyTariffAmount,
		"currency":              "RUB",
		"current_month_amount":  op.MonthlyTariffAmount,
	})
}

// GetSenderRegistrationBilling возвращает историю начислений по регистрации.
// GET /api/sender-registrations/:id/billing
func (h *SenderNameHandlers) GetSenderRegistrationBilling(w http.ResponseWriter, r *http.Request) {
	if h.tariffClient == nil {
		respondError(w, shared.ErrInternalServer("tarification service недоступен"))
		return
	}
	regID := mux.Vars(r)["id"]
	if regID == "" {
		respondError(w, shared.ErrInvalidInput("registration id обязателен"))
		return
	}

	limit := 50
	offset := 0

	resp, err := h.tariffClient.ListSenderBillingRecords(r.Context(), &tarificationv1.ListSenderBillingRecordsRequest{
		SenderRegistrationId: regID,
		Limit:                int32(limit),
		Offset:               int32(offset),
	})
	if err != nil {
		log.Error().Err(err).Str("registration_id", regID).Msg("ошибка получения billing records")
		respondGRPCError(w, err)
		return
	}

	records := make([]map[string]interface{}, len(resp.Records))
	for i, rec := range resp.Records {
		r := map[string]interface{}{
			"id":            rec.Id,
			"billing_month": rec.BillingMonth,
			"amount":        rec.Amount,
		}
		if rec.CreatedAt != nil {
			r["created_at"] = rec.CreatedAt.AsTime()
		}
		records[i] = r
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"records": records,
		"total":   resp.Total,
	})
}

func historyEntryToJSON(e *sendernamev1.SenderNameHistoryEntry) map[string]interface{} {
	m := map[string]interface{}{
		"id":             e.Id,
		"sender_name_id": e.SenderNameId,
		"old_status":     e.OldStatus,
		"new_status":     e.NewStatus,
		"actor_id":       e.ActorId,
		"actor_type":     e.ActorType,
		"comment":        e.Comment,
	}
	if e.CreatedAt != nil {
		m["created_at"] = e.CreatedAt.AsTime()
	}
	return m
}

// ListOperators GET /portal/v1/operators
// Возвращает список активных операторов с доступными типами регистрации.
func (h *SenderNameHandlers) ListOperators(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		respondError(w, shared.ErrInternalServer("database pool недоступен"))
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, name, code, supports_paid_sender, supports_free_sender, monthly_tariff_amount::text
		 FROM operators WHERE active = true ORDER BY name`)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка операторов")
		respondError(w, shared.ErrInternalServer("ошибка получения операторов"))
		return
	}
	defer rows.Close()

	type operatorJSON struct {
		ID                string   `json:"id"`
		Name              string   `json:"name"`
		Slug              string   `json:"slug"`
		RegistrationTypes []string `json:"registration_types"`
		MonthlyTariff     *string  `json:"monthly_tariff_amount"`
	}

	operators := make([]operatorJSON, 0)
	for rows.Next() {
		var id, name, code string
		var supportsPaid, supportsFree bool
		var tariff *string
		if err := rows.Scan(&id, &name, &code, &supportsPaid, &supportsFree, &tariff); err != nil {
			log.Error().Err(err).Msg("ошибка сканирования оператора")
			respondError(w, shared.ErrInternalServer("ошибка получения операторов"))
			return
		}
		types := make([]string, 0, 2)
		if supportsFree {
			types = append(types, "free")
		}
		if supportsPaid {
			types = append(types, "paid")
		}
		operators = append(operators, operatorJSON{
			ID:                id,
			Name:              name,
			Slug:              code,
			RegistrationTypes: types,
			MonthlyTariff:     tariff,
		})
	}

	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("ошибка итерации операторов")
		respondError(w, shared.ErrInternalServer("ошибка получения операторов"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"operators": operators,
	})
}

// GetSenderNameOperatorRegistrations GET /portal/v1/sender-names/{id}/operator-registrations
// Возвращает список регистраций данного имени отправителя у операторов.
func (h *SenderNameHandlers) GetSenderNameOperatorRegistrations(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	if h.pool == nil {
		respondError(w, shared.ErrInternalServer("database pool недоступен"))
		return
	}

	senderNameID := mux.Vars(r)["id"]
	if senderNameID == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT sr.operator_id, o.name AS operator_name, sr.type, sr.status
		 FROM sender_registrations sr
		 JOIN operators o ON o.id = sr.operator_id
		 JOIN sender_names sn ON sn.name = sr.sender_name AND sn.client_id = sr.client_id
		 WHERE sn.id = $1 AND sn.client_id = $2`,
		senderNameID, clientID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения operator registrations")
		respondError(w, shared.ErrInternalServer("ошибка получения регистраций"))
		return
	}
	defer rows.Close()

	type regJSON struct {
		OperatorID   string `json:"operator_id"`
		OperatorName string `json:"operator_name"`
		Type         string `json:"type"`
		Status       string `json:"status"`
	}
	regs := make([]regJSON, 0)
	for rows.Next() {
		var reg regJSON
		if err := rows.Scan(&reg.OperatorID, &reg.OperatorName, &reg.Type, &reg.Status); err != nil {
			log.Error().Err(err).Msg("ошибка сканирования registration")
			respondError(w, shared.ErrInternalServer("ошибка получения регистраций"))
			return
		}
		regs = append(regs, reg)
	}
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("ошибка итерации registrations")
		respondError(w, shared.ErrInternalServer("ошибка получения регистраций"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"registrations": regs,
	})
}

// BulkCreateOperatorRegistrations POST /portal/v1/sender-names/{id}/operator-registrations
// Создаёт регистрации имени отправителя у нескольких операторов.
func (h *SenderNameHandlers) BulkCreateOperatorRegistrations(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	senderNameID := mux.Vars(r)["id"]
	if senderNameID == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}

	// Получаем имя отправителя по ID
	snResp, err := h.client.GetSenderName(r.Context(), &sendernamev1.GetSenderNameRequest{
		Id: senderNameID, ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	if snResp.SenderName.Status != "approved" {
		respondError(w, shared.ErrInvalidInput("имя отправителя должно быть в статусе approved"))
		return
	}

	var req struct {
		Registrations []struct {
			OperatorID string `json:"operator_id"`
			Type       string `json:"type"`
		} `json:"registrations"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if len(req.Registrations) == 0 {
		respondError(w, shared.ErrInvalidInput("registrations не может быть пустым"))
		return
	}

	if h.tariffClient == nil {
		respondError(w, shared.ErrInternalServer("tarification service недоступен"))
		return
	}

	type resultItem struct {
		OperatorID string `json:"operator_id"`
		ID         string `json:"id,omitempty"`
		Status     string `json:"status"`
		Error      string `json:"error,omitempty"`
	}
	results := make([]resultItem, 0, len(req.Registrations))

	for _, reg := range req.Registrations {
		if reg.OperatorID == "" {
			results = append(results, resultItem{OperatorID: reg.OperatorID, Status: "error", Error: "operator_id обязателен"})
			continue
		}
		regType := reg.Type
		if regType == "" {
			regType = "free"
		}

		regResp, err := h.tariffClient.CreateSenderRegistration(r.Context(), &tarificationv1.CreateSenderRegistrationRequest{
			ClientId:   clientID.String(),
			OperatorId: reg.OperatorID,
			SenderName: snResp.SenderName.Name,
			Type:       regType,
		})
		if err != nil {
			log.Error().Err(err).Str("operator_id", reg.OperatorID).Msg("ошибка создания sender registration")
			results = append(results, resultItem{OperatorID: reg.OperatorID, Status: "error", Error: err.Error()})
			continue
		}

		// Для платных регистраций создаём billing record
		if regType == "paid" && h.routingClient != nil {
			op, opErr := h.routingClient.GetOperator(r.Context(), &routingv1.GetOperatorRequest{Id: reg.OperatorID})
			if opErr != nil {
				log.Error().Err(opErr).Str("operator_id", reg.OperatorID).Msg("не удалось получить тариф оператора для billing")
			} else if op.MonthlyTariffAmount != "" {
				billingResp, billingErr := h.tariffClient.CreateSenderBillingRecord(r.Context(), &tarificationv1.CreateSenderBillingRecordRequest{
					SenderRegistrationId: regResp.Id,
					ClientId:             clientID.String(),
					OperatorId:           reg.OperatorID,
					Amount:               op.MonthlyTariffAmount,
				})
				if billingErr != nil {
					log.Error().Err(billingErr).Str("registration_id", regResp.Id).Msg("не удалось создать billing record")
				} else if !billingResp.GetAlreadyExisted() && h.billingClient != nil {
					_, chargeErr := h.billingClient.DeductCredits(r.Context(), &billingv1.DeductCreditsRequest{
						ClientId:    clientID.String(),
						Amount:      op.MonthlyTariffAmount,
						Currency:    "RUB",
						Description: "Paid sender name monthly fee",
					})
					if chargeErr != nil {
						log.Error().Err(chargeErr).Str("registration_id", regResp.Id).Msg("не удалось списать оплату")
					}
				}
			}
		}

		results = append(results, resultItem{OperatorID: reg.OperatorID, ID: regResp.Id, Status: "created"})
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"results": results,
	})
}
