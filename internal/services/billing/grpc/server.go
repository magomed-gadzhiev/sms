package grpc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
	"github.com/smpp-server/smpp-server/internal/services/billing/application"
	"github.com/smpp-server/smpp-server/internal/services/billing/domain"
)

// Server реализует gRPC сервис для биллинга
type Server struct {
	billingv1.UnimplementedBillingServiceServer
	billingService *application.BillingService
	pricingService *application.PricingService
	logger         zerolog.Logger
}

// NewServer создает новый gRPC сервер для Billing Service
func NewServer(
	billingService *application.BillingService,
	pricingService *application.PricingService,
) *Server {
	return &Server{
		billingService: billingService,
		pricingService: pricingService,
		logger:         log.With().Str("component", "billing-grpc-server").Logger(),
	}
}

// GetBalance получает баланс клиента
func (s *Server) GetBalance(ctx context.Context, req *billingv1.GetBalanceRequest) (*billingv1.GetBalanceResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	account, err := s.billingService.GetBalance(ctx, clientID)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			// Возвращаем баланс 0, если счет не найден
			return &billingv1.GetBalanceResponse{
				ClientId:  req.ClientId,
				Balance:   "0",
				Currency:  "RUB",
				UpdatedAt: timestamppb.Now(),
			}, nil
		}
		s.logger.Error().Err(err).Str("client_id", req.ClientId).Msg("ошибка получения баланса")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &billingv1.GetBalanceResponse{
		ClientId:    account.ClientID.String(),
		Balance:     account.Balance,
		Currency:    account.Currency,
		UpdatedAt:   timestamppb.New(account.UpdatedAt),
		Frozen:      account.Frozen,
		CreditLimit: account.CreditLimit,
	}, nil
}

// ChargeMessage списывает средства за сообщение
func (s *Server) ChargeMessage(ctx context.Context, req *billingv1.ChargeMessageRequest) (*billingv1.ChargeMessageResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	if req.MessageId == "" {
		return nil, status.Error(codes.InvalidArgument, "message_id is required")
	}
	if req.Amount == "" {
		return nil, status.Error(codes.InvalidArgument, "amount is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	messageID, err := uuid.Parse(req.MessageId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid message_id format")
	}

	currency := req.Currency
	if currency == "" {
		currency = "RUB"
	}

	transaction, err := s.billingService.ChargeMessage(ctx, clientID, messageID, req.Amount, currency, req.Description)
	if err != nil {
		if errors.Is(err, domain.ErrInsufficientBalance) {
			return &billingv1.ChargeMessageResponse{
				Success: false,
				Error:   "insufficient balance",
			}, nil
		}
		s.logger.Error().Err(err).
			Str("client_id", req.ClientId).
			Str("message_id", req.MessageId).
			Msg("ошибка списания средств")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &billingv1.ChargeMessageResponse{
		TransactionId: transaction.ID.String(),
		NewBalance:    transaction.BalanceAfter,
		Success:       true,
	}, nil
}

// AddCredits добавляет средства на счет клиента
func (s *Server) AddCredits(ctx context.Context, req *billingv1.AddCreditsRequest) (*billingv1.AddCreditsResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	if req.Amount == "" {
		return nil, status.Error(codes.InvalidArgument, "amount is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	currency := req.Currency
	if currency == "" {
		currency = "RUB"
	}

	var paymentMethod *string
	if req.PaymentMethod != "" {
		paymentMethod = &req.PaymentMethod
	}

	transaction, err := s.billingService.AddCredits(ctx, clientID, req.Amount, currency, req.Description, paymentMethod)
	if err != nil {
		s.logger.Error().Err(err).
			Str("client_id", req.ClientId).
			Msg("ошибка добавления средств")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &billingv1.AddCreditsResponse{
		TransactionId: transaction.ID.String(),
		NewBalance:    transaction.BalanceAfter,
		Success:       true,
	}, nil
}

// DeductCredits списывает средства со счета клиента
func (s *Server) DeductCredits(ctx context.Context, req *billingv1.DeductCreditsRequest) (*billingv1.DeductCreditsResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	if req.Amount == "" {
		return nil, status.Error(codes.InvalidArgument, "amount is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	currency := req.Currency
	if currency == "" {
		currency = "RUB"
	}

	transaction, err := s.billingService.DeductCredits(ctx, clientID, req.Amount, currency, req.Description)
	if err != nil {
		if errors.Is(err, domain.ErrInsufficientBalance) {
			return &billingv1.DeductCreditsResponse{
				Success: false,
				Error:   "insufficient balance",
			}, nil
		}
		s.logger.Error().Err(err).
			Str("client_id", req.ClientId).
			Msg("ошибка списания средств")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &billingv1.DeductCreditsResponse{
		TransactionId: transaction.ID.String(),
		NewBalance:    transaction.BalanceAfter,
		Success:       true,
	}, nil
}

// GetTransactionHistory получает историю транзакций
func (s *Server) GetTransactionHistory(ctx context.Context, req *billingv1.GetTransactionHistoryRequest) (*billingv1.GetTransactionHistoryResponse, error) {
	var clientID *uuid.UUID
	if req.ClientId != "" {
		id, err := uuid.Parse(req.ClientId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
		}
		clientID = &id
	}

	limit := int(req.Limit)
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	offset := int(req.Offset)
	if offset < 0 {
		offset = 0
	}

	transactions, err := s.billingService.GetTransactionHistory(ctx, clientID, limit, offset)
	if err != nil {
		s.logger.Error().Err(err).
			Str("client_id", req.ClientId).
			Msg("ошибка получения истории транзакций")
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoTransactions := make([]*billingv1.Transaction, len(transactions))
	for i, tx := range transactions {
		var messageIDStr string
		if tx.MessageID != nil {
			messageIDStr = tx.MessageID.String()
		}

		protoTransactions[i] = &billingv1.Transaction{
			TransactionId:  tx.ID.String(),
			ClientId:       tx.ClientID.String(),
			Type:           string(tx.Type),
			Amount:         tx.Amount,
			Currency:       tx.Currency,
			BalanceBefore:  tx.BalanceBefore,
			BalanceAfter:   tx.BalanceAfter,
			Description:    tx.Description,
			MessageId:      messageIDStr,
			CreatedAt:      timestamppb.New(tx.CreatedAt),
		}
	}

	return &billingv1.GetTransactionHistoryResponse{
		Transactions: protoTransactions,
		Total:        int32(len(protoTransactions)),
		Limit:        int32(limit),
		Offset:       int32(offset),
	}, nil
}

// GetPricingRules получает правила тарификации для клиента
func (s *Server) GetPricingRules(ctx context.Context, req *billingv1.GetPricingRulesRequest) (*billingv1.GetPricingRulesResponse, error) {
	var clientID *uuid.UUID
	if req.ClientId != "" {
		id, err := uuid.Parse(req.ClientId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
		}
		clientID = &id
	}

	rules, err := s.pricingService.GetPricingRules(ctx, clientID)
	if err != nil {
		s.logger.Error().Err(err).Msg("ошибка получения правил тарификации")
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoRules := make([]*billingv1.PricingRule, len(rules))
	for i, rule := range rules {
		var clientIDStr string
		if rule.ClientID != nil {
			clientIDStr = rule.ClientID.String()
		}

		protoRules[i] = &billingv1.PricingRule{
			RuleId:             rule.ID.String(),
			ClientId:           clientIDStr,
			DestinationPattern: rule.DestinationPattern,
			PricePerMessage:    rule.PricePerMessage,
			Currency:           rule.Currency,
			Priority:           int32(rule.Priority),
			Active:             rule.Active,
			CreatedAt:          timestamppb.New(rule.CreatedAt),
			UpdatedAt:          timestamppb.New(rule.UpdatedAt),
		}
	}

	return &billingv1.GetPricingRulesResponse{
		Rules: protoRules,
	}, nil
}

// TransferBalance переводит средства между клиентами
func (s *Server) TransferBalance(ctx context.Context, req *billingv1.TransferBalanceRequest) (*billingv1.TransferBalanceResponse, error) {
	if req.FromClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "from_client_id is required")
	}
	if req.ToClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "to_client_id is required")
	}
	if req.Amount == "" {
		return nil, status.Error(codes.InvalidArgument, "amount is required")
	}

	fromClientID, err := uuid.Parse(req.FromClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid from_client_id format")
	}

	toClientID, err := uuid.Parse(req.ToClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid to_client_id format")
	}

	currency := req.Currency
	if currency == "" {
		currency = "RUB"
	}

	transferID, fromBalance, toBalance, err := s.billingService.TransferBalance(ctx, fromClientID, toClientID, req.Amount, currency)
	if err != nil {
		if errors.Is(err, domain.ErrInsufficientBalance) {
			return nil, status.Error(codes.FailedPrecondition, "insufficient balance")
		}
		s.logger.Error().Err(err).
			Str("from_client_id", req.FromClientId).
			Str("to_client_id", req.ToClientId).
			Str("amount", req.Amount).
			Msg("ошибка перевода средств")
		return nil, status.Error(codes.Internal, "failed to transfer balance")
	}

	return &billingv1.TransferBalanceResponse{
		TransferId:  transferID,
		FromBalance: fromBalance,
		ToBalance:   toBalance,
	}, nil
}

// CreatePricingRule создает правило тарификации
func (s *Server) CreatePricingRule(ctx context.Context, req *billingv1.CreatePricingRuleRequest) (*billingv1.CreatePricingRuleResponse, error) {
	if req.DestinationPattern == "" {
		return nil, status.Error(codes.InvalidArgument, "destination_pattern is required")
	}
	if req.PricePerMessage == "" {
		return nil, status.Error(codes.InvalidArgument, "price_per_message is required")
	}

	var clientID *uuid.UUID
	if req.ClientId != "" {
		id, err := uuid.Parse(req.ClientId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
		}
		clientID = &id
	}

	currency := req.Currency
	if currency == "" {
		currency = "RUB"
	}

	priority := int(req.Priority)
	if priority < 0 {
		priority = 0
	}

	active := req.Active
	// По умолчанию активно
	if !req.Active {
		active = true
	}

	rule, err := s.pricingService.CreatePricingRule(
		ctx,
		clientID,
		req.DestinationPattern,
		req.PricePerMessage,
		currency,
		priority,
		active,
	)
	if err != nil {
		s.logger.Error().Err(err).Msg("ошибка создания правила тарификации")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &billingv1.CreatePricingRuleResponse{
		RuleId:    rule.ID.String(),
		CreatedAt: timestamppb.New(rule.CreatedAt),
	}, nil
}

// FreezeAccount замораживает счет клиента
func (s *Server) FreezeAccount(ctx context.Context, req *billingv1.FreezeAccountRequest) (*billingv1.FreezeAccountResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	if req.AdminId == "" {
		return nil, status.Error(codes.InvalidArgument, "admin_id is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	adminID, err := uuid.Parse(req.AdminId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid admin_id format")
	}

	frozenAt, err := s.billingService.FreezeAccount(ctx, clientID, adminID)
	if err != nil {
		s.logger.Error().Err(err).
			Str("client_id", req.ClientId).
			Msg("ошибка заморозки счета")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &billingv1.FreezeAccountResponse{
		Success:  true,
		FrozenAt: timestamppb.New(frozenAt),
	}, nil
}

// UnfreezeAccount размораживает счет клиента
func (s *Server) UnfreezeAccount(ctx context.Context, req *billingv1.UnfreezeAccountRequest) (*billingv1.UnfreezeAccountResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	if err := s.billingService.UnfreezeAccount(ctx, clientID); err != nil {
		s.logger.Error().Err(err).
			Str("client_id", req.ClientId).
			Msg("ошибка разморозки счета")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &billingv1.UnfreezeAccountResponse{
		Success: true,
	}, nil
}

// SetCreditLimit устанавливает кредитный лимит для клиента
func (s *Server) SetCreditLimit(ctx context.Context, req *billingv1.SetCreditLimitRequest) (*billingv1.SetCreditLimitResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	if err := s.billingService.SetCreditLimit(ctx, clientID, req.CreditLimit); err != nil {
		s.logger.Error().Err(err).
			Str("client_id", req.ClientId).
			Msg("ошибка установки кредитного лимита")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &billingv1.SetCreditLimitResponse{
		Success:     true,
		CreditLimit: req.CreditLimit,
	}, nil
}

// SetLowBalanceThreshold устанавливает порог низкого баланса
func (s *Server) SetLowBalanceThreshold(ctx context.Context, req *billingv1.SetLowBalanceThresholdRequest) (*billingv1.SetLowBalanceThresholdResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	if err := s.billingService.SetLowBalanceThreshold(ctx, clientID, req.Threshold); err != nil {
		s.logger.Error().Err(err).
			Str("client_id", req.ClientId).
			Msg("ошибка установки порога низкого баланса")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &billingv1.SetLowBalanceThresholdResponse{
		Success:   true,
		Threshold: req.Threshold,
	}, nil
}

// ListBalances получает список балансов с фильтрацией
func (s *Server) ListBalances(ctx context.Context, req *billingv1.ListBalancesRequest) (*billingv1.ListBalancesResponse, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}

	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	balances, total, err := s.billingService.ListBalances(ctx, req.Search, req.Status, req.BelowThreshold, limit, offset)
	if err != nil {
		s.logger.Error().Err(err).Msg("ошибка получения списка балансов")
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoBalances := make([]*billingv1.BalanceInfo, len(balances))
	for i, b := range balances {
		info := &billingv1.BalanceInfo{
			ClientId:            b.ClientID.String(),
			ClientName:          b.ClientName,
			Balance:             b.Balance,
			Currency:            b.Currency,
			Frozen:              b.Frozen,
			CreditLimit:         b.CreditLimit,
			LowBalanceThreshold: b.LowBalanceThreshold,
			UpdatedAt:           timestamppb.New(b.UpdatedAt),
		}
		if b.FrozenAt != nil {
			info.FrozenAt = timestamppb.New(*b.FrozenAt)
		}
		if b.FrozenBy != nil {
			info.FrozenBy = b.FrozenBy.String()
		}
		protoBalances[i] = info
	}

	return &billingv1.ListBalancesResponse{
		Balances: protoBalances,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	}, nil
}
