package handlers

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"

	"github.com/rs/zerolog/log"

	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
	contactv1 "github.com/smpp-server/smpp-server/api/proto/contactv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// CostEstimateHandlers handles campaign cost estimation requests.
type CostEstimateHandlers struct {
	billingClient billingv1.BillingServiceClient
	contactClient contactv1.ContactServiceClient
}

// NewCostEstimateHandlers creates a new CostEstimateHandlers.
func NewCostEstimateHandlers(
	billingClient billingv1.BillingServiceClient,
	contactClient contactv1.ContactServiceClient,
) *CostEstimateHandlers {
	return &CostEstimateHandlers{
		billingClient: billingClient,
		contactClient: contactClient,
	}
}

// costEstimateRequest is the request body for POST /portal/v1/campaigns/estimate-cost.
type costEstimateRequest struct {
	ContactListID    string   `json:"contact_list_id"`
	Text             string   `json:"text"`
	Source           string   `json:"source"`
	ExcludeCountries []string `json:"exclude_countries,omitempty"`
	ExcludeOperators []string `json:"exclude_operators,omitempty"`
}

// CostEstimateResponse is the response for cost estimation.
type CostEstimateResponse struct {
	Recipients        int    `json:"recipients"`
	SegmentsPerMsg    int    `json:"segments_per_msg"`
	TotalSegments     int    `json:"total_segments"`
	PricePerSegment   string `json:"price_per_segment"`
	EstimatedCost     string `json:"estimated_cost"`
	CurrentBalance    string `json:"current_balance"`
	BalanceSufficient bool   `json:"balance_sufficient"`
}

// countSegments returns the number of SMS segments for a given text.
// GSM-7: 160 chars single, 153 per segment multipart.
// Unicode: 70 chars single, 67 per segment multipart.
func countSegments(text string) int {
	runes := []rune(text)
	length := len(runes)
	if length == 0 {
		return 1
	}
	gsm7 := true
	for _, r := range runes {
		if r > 127 {
			gsm7 = false
			break
		}
	}
	if gsm7 {
		if length <= 160 {
			return 1
		}
		return int(math.Ceil(float64(length) / 153.0))
	}
	if length <= 70 {
		return 1
	}
	return int(math.Ceil(float64(length) / 67.0))
}

// Estimate handles POST /portal/v1/campaigns/estimate-cost.
func (h *CostEstimateHandlers) Estimate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	var req costEstimateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Некорректное тело запроса"))
		return
	}

	ctx := r.Context()

	// Get recipient count from contact list.
	recipients := 0
	if req.ContactListID != "" {
		listResp, err := h.contactClient.GetContactList(ctx, &contactv1.GetContactListRequest{
			Id:       req.ContactListID,
			ClientId: clientID.String(),
		})
		if err != nil {
			log.Warn().Err(err).Str("contact_list_id", req.ContactListID).Msg("cost-estimate: failed to get contact list")
		} else {
			recipients = int(listResp.GetContactsCount())
		}
	}

	// Get pricing rules for this client.
	pricePerSegment := "0.00"
	pricingResp, err := h.billingClient.GetPricingRules(ctx, &billingv1.GetPricingRulesRequest{
		ClientId: clientID.String(),
	})
	if err != nil {
		log.Warn().Err(err).Msg("cost-estimate: failed to get pricing rules")
	} else if len(pricingResp.GetRules()) > 0 {
		pricePerSegment = pricingResp.GetRules()[0].GetPricePerMessage()
	}

	// Get current balance.
	currentBalance := "0.00"
	balanceResp, err := h.billingClient.GetBalance(ctx, &billingv1.GetBalanceRequest{
		ClientId: clientID.String(),
	})
	if err != nil {
		log.Warn().Err(err).Msg("cost-estimate: failed to get balance")
	} else {
		currentBalance = balanceResp.GetBalance()
	}

	segments := countSegments(req.Text)
	totalSegments := segments * recipients

	// Multiply price × total segments using float arithmetic.
	// TODO: replace with decimal arithmetic when shopspring/decimal is added.
	var estimatedCost float64
	if _, scanErr := fmt.Sscanf(pricePerSegment, "%f", &estimatedCost); scanErr == nil {
		estimatedCost *= float64(totalSegments)
	}

	var balance float64
	fmt.Sscanf(currentBalance, "%f", &balance)
	balanceSufficient := balance >= estimatedCost

	respondJSON(w, http.StatusOK, CostEstimateResponse{
		Recipients:        recipients,
		SegmentsPerMsg:    segments,
		TotalSegments:     totalSegments,
		PricePerSegment:   pricePerSegment,
		EstimatedCost:     fmt.Sprintf("%.2f", estimatedCost),
		CurrentBalance:    currentBalance,
		BalanceSufficient: balanceSufficient,
	})
}
