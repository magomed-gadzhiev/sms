package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/payment"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type BillingHandlers struct {
	billingClient   billingv1.BillingServiceClient
	paymentProvider payment.PaymentProvider
}

func NewBillingHandlers(billingClient billingv1.BillingServiceClient, paymentProvider payment.PaymentProvider) *BillingHandlers {
	return &BillingHandlers{billingClient: billingClient, paymentProvider: paymentProvider}
}

func (h *BillingHandlers) GetBalance(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	resp, err := h.billingClient.GetBalance(r.Context(), &billingv1.GetBalanceRequest{ClientId: clientID.String()})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения баланса")
		respondGRPCError(w, err)
		return
	}
	result := map[string]interface{}{"client_id": resp.ClientId, "balance": resp.Balance, "currency": resp.Currency}
	if resp.UpdatedAt != nil {
		result["updated_at"] = resp.UpdatedAt.AsTime()
	}

	// Получаем порог низкого баланса из списка балансов
	listResp, err := h.billingClient.ListBalances(r.Context(), &billingv1.ListBalancesRequest{
		Search: clientID.String(),
		Limit:  1,
		Offset: 0,
	})
	if err == nil && len(listResp.Balances) > 0 {
		result["low_balance_threshold"] = listResp.Balances[0].LowBalanceThreshold
	}

	respondJSON(w, http.StatusOK, result)
}

func (h *BillingHandlers) GetTransactions(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	page, perPage := parsePagination(r)
	offset := (page - 1) * perPage
	query := r.URL.Query()
	txType := query.Get("type")
	var dateFrom, dateTo *timestamppb.Timestamp
	if fromStr := query.Get("date_from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			dateFrom = timestamppb.New(t)
		} else if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			dateFrom = timestamppb.New(t)
		}
	}
	if toStr := query.Get("date_to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			dateTo = timestamppb.New(t)
		} else if t, err := time.Parse("2006-01-02", toStr); err == nil {
			dateTo = timestamppb.New(t.Add(24*time.Hour - time.Second))
		}
	}
	resp, err := h.billingClient.GetTransactionHistory(r.Context(), &billingv1.GetTransactionHistoryRequest{
		ClientId: clientID.String(), From: dateFrom, To: dateTo, TransactionType: txType, Limit: perPage, Offset: offset,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения истории транзакций")
		respondGRPCError(w, err)
		return
	}
	transactions := make([]map[string]interface{}, 0, len(resp.Transactions))
	for _, tx := range resp.Transactions {
		t := map[string]interface{}{
			"transaction_id": tx.TransactionId, "type": tx.Type, "amount": tx.Amount,
			"currency": tx.Currency, "balance_before": tx.BalanceBefore, "balance_after": tx.BalanceAfter,
			"description": tx.Description,
		}
		if tx.MessageId != "" {
			t["message_id"] = tx.MessageId
		}
		if tx.CreatedAt != nil {
			t["created_at"] = tx.CreatedAt.AsTime()
		}
		transactions = append(transactions, t)
	}
	totalPages := int32(0)
	if perPage > 0 && resp.Total > 0 {
		totalPages = (resp.Total + perPage - 1) / perPage
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"transactions": transactions, "total": resp.Total, "page": page, "per_page": perPage, "total_pages": totalPages,
	})
}

type topUpRequest struct {
	Amount    string `json:"amount"`
	Currency  string `json:"currency"`
	ReturnURL string `json:"return_url"`
}

func (h *BillingHandlers) TopUp(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	var req topUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Amount == "" {
		respondError(w, shared.ErrInvalidInput("Поле amount обязательно"))
		return
	}
	if req.Currency == "" {
		req.Currency = "RUB"
	}
	session, err := h.paymentProvider.CreatePayment(r.Context(), payment.CreatePaymentRequest{
		ClientID: clientID.String(), Amount: req.Amount, Currency: req.Currency, ReturnURL: req.ReturnURL,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания платежа")
		respondError(w, shared.ErrInternalServer("Ошибка инициализации платежа"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"payment_id": session.PaymentID, "payment_url": session.PaymentURL, "expires_at": session.ExpiresAt,
	})
}

func (h *BillingHandlers) TopUpCallback(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, shared.ErrInvalidInput("Ошибка чтения тела запроса"))
		return
	}
	signature := r.Header.Get("X-Signature")
	if signature == "" {
		signature = r.URL.RawQuery
	}
	result, err := h.paymentProvider.HandleCallback(r.Context(), body, signature)
	if err != nil {
		log.Error().Err(err).Msg("ошибка обработки callback платежа")
		respondError(w, shared.ErrInvalidInput("Невалидный callback"))
		return
	}
	if result.Status == "success" && result.ClientID != "" {
		_, err = h.billingClient.AddCredits(r.Context(), &billingv1.AddCreditsRequest{
			ClientId: result.ClientID, Amount: result.Amount, Currency: result.Currency,
			Description: "Пополнение баланса (payment: " + result.PaymentID + ")", PaymentMethod: "online",
		})
		if err != nil {
			log.Error().Err(err).Str("payment_id", result.PaymentID).Msg("ошибка зачисления средств")
		}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "ok"})
}

type setLowBalanceThresholdRequest struct {
	Threshold string `json:"threshold"`
}

func (h *BillingHandlers) SetLowBalanceThreshold(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	var req setLowBalanceThresholdRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Threshold == "" {
		respondError(w, shared.ErrInvalidInput("Поле threshold обязательно"))
		return
	}
	_, err := h.billingClient.SetLowBalanceThreshold(r.Context(), &billingv1.SetLowBalanceThresholdRequest{
		ClientId:  clientID.String(),
		Threshold: req.Threshold,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка установки порога низкого баланса")
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "threshold": req.Threshold})
}
