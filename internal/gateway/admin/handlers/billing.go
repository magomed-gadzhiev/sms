package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/billingv1"
	"github.com/smpp-server/smpp-server/internal/gateway/admin/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// BillingHandlers обрабатывает HTTP запросы для биллинга
type BillingHandlers struct {
	billingClient billingv1.BillingServiceClient
}

// NewBillingHandlers создает новый экземпляр BillingHandlers
func NewBillingHandlers(billingClient billingv1.BillingServiceClient) *BillingHandlers {
	return &BillingHandlers{
		billingClient: billingClient,
	}
}

// GetBalance обрабатывает GET /admin/v1/billing/clients/:id/balance
func (h *BillingHandlers) GetBalance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["id"]
	if clientID == "" {
		// Попробуем из query параметра для обратной совместимости
		clientID = r.URL.Query().Get("client_id")
		if clientID == "" {
			respondError(w, shared.ErrInvalidInput("client_id обязателен"))
			return
		}
	}

	resp, err := h.billingClient.GetBalance(r.Context(), &billingv1.GetBalanceRequest{
		ClientId: clientID,
	})
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("ошибка получения баланса")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, BalanceResponse{
		ClientID:  resp.ClientId,
		Balance:   resp.Balance,
		Currency:  resp.Currency,
		UpdatedAt: resp.UpdatedAt.AsTime(),
	})
}

// AddCredits обрабатывает POST /admin/v1/billing/clients/:id/credits
func (h *BillingHandlers) AddCredits(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["id"]
	if clientID == "" {
		// Попробуем из query параметра для обратной совместимости
		clientID = r.URL.Query().Get("client_id")
		if clientID == "" {
			respondError(w, shared.ErrInvalidInput("client_id обязателен"))
			return
		}
	}

	var req AddCreditsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if err := req.Validate(); err != nil {
		respondError(w, err.(*shared.AppError))
		return
	}

	grpcReq := &billingv1.AddCreditsRequest{
		ClientId:      clientID,
		Amount:        req.Amount,
		Currency:      req.Currency,
		Description:   req.Description,
		PaymentMethod: req.PaymentMethod,
	}

	resp, err := h.billingClient.AddCredits(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("ошибка добавления средств")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, AddCreditsResponse{
		TransactionID: resp.TransactionId,
		NewBalance:    resp.NewBalance,
		Success:       resp.Success,
		Error:         resp.Error,
	})
}

// GetTransactionHistory обрабатывает GET /admin/v1/billing/transactions
func (h *BillingHandlers) GetTransactionHistory(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	transactionType := r.URL.Query().Get("transaction_type")
	limit := parseInt(r.URL.Query().Get("limit"), 50)
	offset := parseInt(r.URL.Query().Get("offset"), 0)

	// client_id опционален для админа — без него возвращаем все транзакции

	var from, to *time.Time
	if fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат параметра from"))
			return
		}
		from = &t
	}
	if toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат параметра to"))
			return
		}
		to = &t
	}

	grpcReq := &billingv1.GetTransactionHistoryRequest{
		ClientId:        clientID,
		TransactionType: transactionType,
		Limit:           int32(limit),
		Offset:          int32(offset),
	}
	if from != nil {
		grpcReq.From = timestamppb.New(*from)
	}
	if to != nil {
		grpcReq.To = timestamppb.New(*to)
	}

	resp, err := h.billingClient.GetTransactionHistory(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("ошибка получения истории транзакций")
		respondGRPCError(w, err)
		return
	}

	transactions := make([]Transaction, len(resp.Transactions))
	for i, t := range resp.Transactions {
		transactions[i] = transactionToResponse(t)
	}

	respondJSON(w, http.StatusOK, TransactionHistoryResponse{
		Transactions: transactions,
		Total:        int(resp.Total),
		Limit:        limit,
		Offset:       offset,
	})
}

// GetPricingRules обрабатывает GET /admin/v1/billing/pricing-rules
func (h *BillingHandlers) GetPricingRules(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")

	resp, err := h.billingClient.GetPricingRules(r.Context(), &billingv1.GetPricingRulesRequest{
		ClientId: clientID,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения правил тарификации")
		respondGRPCError(w, err)
		return
	}

	rules := make([]PricingRule, len(resp.Rules))
	for i, rule := range resp.Rules {
		rules[i] = pricingRuleToResponse(rule)
	}

	respondJSON(w, http.StatusOK, PricingRulesResponse{
		Rules: rules,
	})
}

// CreatePricingRule обрабатывает POST /admin/v1/billing/pricing-rules
func (h *BillingHandlers) CreatePricingRule(w http.ResponseWriter, r *http.Request) {
	var req CreatePricingRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if err := req.Validate(); err != nil {
		respondError(w, err.(*shared.AppError))
		return
	}

	grpcReq := &billingv1.CreatePricingRuleRequest{
		ClientId:           req.ClientID,
		DestinationPattern: req.DestinationPattern,
		PricePerMessage:    req.PricePerMessage,
		Currency:           req.Currency,
		Priority:           req.Priority,
		Active:             req.Active,
	}

	resp, err := h.billingClient.CreatePricingRule(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания правила тарификации")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, CreatePricingRuleResponse{
		RuleID:    resp.RuleId,
		CreatedAt: resp.CreatedAt.AsTime(),
	})
}

// FreezeAccount обрабатывает POST /admin/v1/billing/clients/:id/freeze
func (h *BillingHandlers) FreezeAccount(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["id"]
	if clientID == "" {
		respondError(w, shared.ErrInvalidInput("client_id обязателен"))
		return
	}

	adminID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("не удалось определить пользователя"))
		return
	}

	resp, err := h.billingClient.FreezeAccount(r.Context(), &billingv1.FreezeAccountRequest{
		ClientId: clientID,
		AdminId:  adminID.String(),
	})
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("ошибка заморозки аккаунта")
		respondGRPCError(w, err)
		return
	}

	result := map[string]interface{}{
		"success": resp.Success,
	}
	if resp.FrozenAt != nil {
		result["frozen_at"] = resp.FrozenAt.AsTime()
	}
	respondJSON(w, http.StatusOK, result)
}

// UnfreezeAccount обрабатывает POST /admin/v1/billing/clients/:id/unfreeze
func (h *BillingHandlers) UnfreezeAccount(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["id"]
	if clientID == "" {
		respondError(w, shared.ErrInvalidInput("client_id обязателен"))
		return
	}

	adminID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("не удалось определить пользователя"))
		return
	}

	resp, err := h.billingClient.UnfreezeAccount(r.Context(), &billingv1.UnfreezeAccountRequest{
		ClientId: clientID,
		AdminId:  adminID.String(),
	})
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("ошибка разморозки аккаунта")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": resp.Success,
	})
}

// SetCreditLimit обрабатывает PUT /admin/v1/billing/clients/:id/credit-limit
func (h *BillingHandlers) SetCreditLimit(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["id"]
	if clientID == "" {
		respondError(w, shared.ErrInvalidInput("client_id обязателен"))
		return
	}

	var req struct {
		CreditLimit string `json:"credit_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.CreditLimit == "" {
		respondError(w, shared.ErrInvalidInput("credit_limit обязателен"))
		return
	}

	resp, err := h.billingClient.SetCreditLimit(r.Context(), &billingv1.SetCreditLimitRequest{
		ClientId:    clientID,
		CreditLimit: req.CreditLimit,
	})
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("ошибка установки кредитного лимита")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":      resp.Success,
		"credit_limit": resp.CreditLimit,
	})
}

// SetLowBalanceThreshold обрабатывает PUT /admin/v1/billing/clients/:id/low-balance-threshold
func (h *BillingHandlers) SetLowBalanceThreshold(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["id"]
	if clientID == "" {
		respondError(w, shared.ErrInvalidInput("client_id обязателен"))
		return
	}

	var req struct {
		Threshold string `json:"threshold"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Threshold == "" {
		respondError(w, shared.ErrInvalidInput("threshold обязателен"))
		return
	}

	resp, err := h.billingClient.SetLowBalanceThreshold(r.Context(), &billingv1.SetLowBalanceThresholdRequest{
		ClientId:  clientID,
		Threshold: req.Threshold,
	})
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("ошибка установки порога низкого баланса")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":   resp.Success,
		"threshold": resp.Threshold,
	})
}

// ListBalances обрабатывает GET /admin/v1/billing/balances
func (h *BillingHandlers) ListBalances(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")
	belowThreshold := r.URL.Query().Get("below_threshold") == "true"
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)

	resp, err := h.billingClient.ListBalances(r.Context(), &billingv1.ListBalancesRequest{
		Search:         search,
		Status:         status,
		BelowThreshold: belowThreshold,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка балансов")
		respondGRPCError(w, err)
		return
	}

	balances := make([]map[string]interface{}, len(resp.Balances))
	for i, b := range resp.Balances {
		balances[i] = balanceInfoToMap(b)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"balances": balances,
		"total":    resp.Total,
		"limit":    resp.Limit,
		"offset":   resp.Offset,
	})
}

func balanceInfoToMap(b *billingv1.BalanceInfo) map[string]interface{} {
	result := map[string]interface{}{
		"client_id":             b.ClientId,
		"client_name":           b.ClientName,
		"balance":               b.Balance,
		"currency":              b.Currency,
		"frozen":                b.Frozen,
		"credit_limit":          b.CreditLimit,
		"low_balance_threshold": b.LowBalanceThreshold,
		"frozen_by":             b.FrozenBy,
	}
	if b.FrozenAt != nil {
		result["frozen_at"] = b.FrozenAt.AsTime()
	}
	if b.UpdatedAt != nil {
		result["updated_at"] = b.UpdatedAt.AsTime()
	}
	return result
}

// Типы запросов и ответов

type BalanceResponse struct {
	ClientID  string    `json:"client_id"`
	Balance   string    `json:"balance"`
	Currency  string    `json:"currency"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AddCreditsRequest struct {
	Amount        string `json:"amount"`
	Currency      string `json:"currency"`
	Description   string `json:"description"`
	PaymentMethod string `json:"payment_method,omitempty"`
}

func (r *AddCreditsRequest) Validate() error {
	if r.Amount == "" {
		return shared.ErrInvalidInput("amount обязателен")
	}
	// Валюту не задаём по умолчанию — сервис биллинга использует валюту аккаунта
	return nil
}

type AddCreditsResponse struct {
	TransactionID string `json:"transaction_id"`
	NewBalance    string `json:"new_balance"`
	Success       bool   `json:"success"`
	Error         string `json:"error,omitempty"`
}

type TransactionHistoryResponse struct {
	Transactions []Transaction `json:"transactions"`
	Total        int           `json:"total"`
	Limit        int           `json:"limit"`
	Offset       int           `json:"offset"`
}

type Transaction struct {
	TransactionID string    `json:"transaction_id"`
	ClientID      string    `json:"client_id"`
	Type          string    `json:"type"`
	Amount        string    `json:"amount"`
	Currency      string    `json:"currency"`
	BalanceBefore string    `json:"balance_before"`
	BalanceAfter  string    `json:"balance_after"`
	Description   string    `json:"description"`
	MessageID     string    `json:"message_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type PricingRulesResponse struct {
	Rules []PricingRule `json:"rules"`
}

type PricingRule struct {
	RuleID             string    `json:"rule_id"`
	ClientID           string    `json:"client_id"`
	DestinationPattern string    `json:"destination_pattern"`
	PricePerMessage    string    `json:"price_per_message"`
	Currency           string    `json:"currency"`
	Priority           int32     `json:"priority"`
	Active             bool      `json:"active"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type CreatePricingRuleRequest struct {
	ClientID           string `json:"client_id,omitempty"`
	DestinationPattern string `json:"destination_pattern"`
	PricePerMessage    string `json:"price_per_message"`
	Currency           string `json:"currency"`
	Priority           int32  `json:"priority"`
	Active             bool   `json:"active"`
}

func (r *CreatePricingRuleRequest) Validate() error {
	if r.DestinationPattern == "" {
		return shared.ErrInvalidInput("destination_pattern обязателен")
	}
	if r.PricePerMessage == "" {
		return shared.ErrInvalidInput("price_per_message обязателен")
	}
	if r.Currency == "" {
		return shared.ErrInvalidInput("currency обязателен")
	}
	return nil
}

type CreatePricingRuleResponse struct {
	RuleID    string    `json:"rule_id"`
	CreatedAt time.Time `json:"created_at"`
}

func transactionToResponse(t *billingv1.Transaction) Transaction {
	trans := Transaction{
		TransactionID: t.TransactionId,
		ClientID:      t.ClientId,
		Type:          t.Type,
		Amount:        t.Amount,
		Currency:      t.Currency,
		BalanceBefore: t.BalanceBefore,
		BalanceAfter:  t.BalanceAfter,
		Description:   t.Description,
		MessageID:     t.MessageId,
	}
	if t.CreatedAt != nil {
		trans.CreatedAt = t.CreatedAt.AsTime()
	}
	return trans
}

func pricingRuleToResponse(rule *billingv1.PricingRule) PricingRule {
	pr := PricingRule{
		RuleID:             rule.RuleId,
		ClientID:           rule.ClientId,
		DestinationPattern: rule.DestinationPattern,
		PricePerMessage:    rule.PricePerMessage,
		Currency:           rule.Currency,
		Priority:           int32(rule.Priority),
		Active:             rule.Active,
	}
	if rule.CreatedAt != nil {
		pr.CreatedAt = rule.CreatedAt.AsTime()
	}
	if rule.UpdatedAt != nil {
		pr.UpdatedAt = rule.UpdatedAt.AsTime()
	}
	return pr
}
