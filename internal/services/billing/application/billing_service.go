package application

import (
	"context"
	"fmt"
	"math/big"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/billing/domain"
)

// BillingService предоставляет бизнес-логику для биллинга
type BillingService struct {
	accountRepo     domain.AccountRepository
	transactionRepo domain.TransactionRepository
	eventPublisher  domain.EventPublisher
	logger          zerolog.Logger
}

// NewBillingService создает новый сервис биллинга
func NewBillingService(
	accountRepo domain.AccountRepository,
	transactionRepo domain.TransactionRepository,
	eventPublisher domain.EventPublisher,
) *BillingService {
	return &BillingService{
		accountRepo:     accountRepo,
		transactionRepo: transactionRepo,
		eventPublisher:  eventPublisher,
		logger:          log.With().Str("component", "billing-service").Logger(),
	}
}

// GetBalance получает баланс клиента
func (s *BillingService) GetBalance(ctx context.Context, clientID uuid.UUID) (*domain.Account, error) {
	account, err := s.accountRepo.GetByClientID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}
	return account, nil
}

// AddCredits добавляет средства на счет клиента
func (s *BillingService) AddCredits(
	ctx context.Context,
	clientID uuid.UUID,
	amount, currency, description string,
	paymentMethod *string,
) (*domain.Transaction, error) {
	// Получаем или создаем счет
	account, err := s.accountRepo.GetByClientID(ctx, clientID)
	if err != nil {
		if err != domain.ErrAccountNotFound {
			return nil, fmt.Errorf("failed to get account: %w", err)
		}
		// Создаем новый счет
		account = domain.NewAccount(clientID, currency)
		if err := account.Validate(); err != nil {
			return nil, fmt.Errorf("invalid account: %w", err)
		}
		if err := s.accountRepo.Create(ctx, account); err != nil {
			return nil, fmt.Errorf("failed to create account: %w", err)
		}
	}

	// Проверяем валюту
	if account.Currency != currency {
		return nil, fmt.Errorf("currency mismatch: account has %s, but %s provided", account.Currency, currency)
	}

	// Вычисляем новый баланс
	newBalance, err := s.add(account.Balance, amount)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate new balance: %w", err)
	}

	// Обновляем баланс
	if err := s.accountRepo.UpdateBalance(ctx, clientID, newBalance); err != nil {
		return nil, fmt.Errorf("failed to update balance: %w", err)
	}

	// Создаем транзакцию
	transaction := domain.NewTransaction(
		clientID,
		domain.TransactionTypeCredit,
		amount,
		account.Balance,
		newBalance,
		currency,
	).WithDescription(description)

	if paymentMethod != nil {
		transaction = transaction.WithPaymentMethod(*paymentMethod)
	}

	if err := transaction.Validate(); err != nil {
		return nil, fmt.Errorf("invalid transaction: %w", err)
	}

	if err := s.transactionRepo.Create(ctx, transaction); err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// Публикуем события
	if s.eventPublisher != nil {
		if err := s.eventPublisher.PublishBalanceChanged(ctx, clientID.String(), newBalance, currency); err != nil {
			s.logger.Warn().Err(err).Msg("failed to publish balance changed event")
		}
		if err := s.eventPublisher.PublishTransactionCompleted(ctx, transaction.ID.String(), clientID.String(), string(transaction.Type), amount, currency); err != nil {
			s.logger.Warn().Err(err).Msg("failed to publish transaction completed event")
		}
	}

	return transaction, nil
}

// DeductCredits списывает средства со счета клиента
func (s *BillingService) DeductCredits(
	ctx context.Context,
	clientID uuid.UUID,
	amount, currency, description string,
) (*domain.Transaction, error) {
	account, err := s.accountRepo.GetByClientID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	// Проверяем валюту
	if account.Currency != currency {
		return nil, fmt.Errorf("currency mismatch: account has %s, but %s provided", account.Currency, currency)
	}

	// Проверяем баланс
	newBalance, err := s.subtract(account.Balance, amount)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate new balance: %w", err)
	}

	if s.isNegative(newBalance) {
		return nil, domain.ErrInsufficientBalance
	}

	// Обновляем баланс
	if err := s.accountRepo.UpdateBalance(ctx, clientID, newBalance); err != nil {
		return nil, fmt.Errorf("failed to update balance: %w", err)
	}

	// Создаем транзакцию
	transaction := domain.NewTransaction(
		clientID,
		domain.TransactionTypeCharge,
		amount,
		account.Balance,
		newBalance,
		currency,
	).WithDescription(description)

	if err := transaction.Validate(); err != nil {
		return nil, fmt.Errorf("invalid transaction: %w", err)
	}

	if err := s.transactionRepo.Create(ctx, transaction); err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// Публикуем события
	if s.eventPublisher != nil {
		if err := s.eventPublisher.PublishBalanceChanged(ctx, clientID.String(), newBalance, currency); err != nil {
			s.logger.Warn().Err(err).Msg("failed to publish balance changed event")
		}
		if err := s.eventPublisher.PublishTransactionCompleted(ctx, transaction.ID.String(), clientID.String(), string(transaction.Type), amount, currency); err != nil {
			s.logger.Warn().Err(err).Msg("failed to publish transaction completed event")
		}
	}

	return transaction, nil
}

// ChargeMessage списывает средства за сообщение
func (s *BillingService) ChargeMessage(
	ctx context.Context,
	clientID, messageID uuid.UUID,
	amount, currency, description string,
) (*domain.Transaction, error) {
	// Проверяем, не была ли уже создана транзакция для этого сообщения
	existing, err := s.transactionRepo.GetByMessageID(ctx, messageID)
	if err == nil && existing != nil {
		// Транзакция уже существует
		s.logger.Debug().
			Str("message_id", messageID.String()).
			Str("transaction_id", existing.ID.String()).
			Msg("transaction already exists for message")
		return existing, nil
	}

	account, err := s.accountRepo.GetByClientID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	// Проверяем валюту
	if account.Currency != currency {
		return nil, fmt.Errorf("currency mismatch: account has %s, but %s provided", account.Currency, currency)
	}

	// Проверяем баланс
	newBalance, err := s.subtract(account.Balance, amount)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate new balance: %w", err)
	}

	if s.isNegative(newBalance) {
		return nil, domain.ErrInsufficientBalance
	}

	// Обновляем баланс
	if err := s.accountRepo.UpdateBalance(ctx, clientID, newBalance); err != nil {
		return nil, fmt.Errorf("failed to update balance: %w", err)
	}

	// Создаем транзакцию
	transaction := domain.NewTransaction(
		clientID,
		domain.TransactionTypeCharge,
		amount,
		account.Balance,
		newBalance,
		currency,
	).WithMessageID(messageID).WithDescription(description)

	if err := transaction.Validate(); err != nil {
		return nil, fmt.Errorf("invalid transaction: %w", err)
	}

	if err := s.transactionRepo.Create(ctx, transaction); err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// Публикуем события
	if s.eventPublisher != nil {
		if err := s.eventPublisher.PublishBalanceChanged(ctx, clientID.String(), newBalance, currency); err != nil {
			s.logger.Warn().Err(err).Msg("failed to publish balance changed event")
		}
		if err := s.eventPublisher.PublishTransactionCompleted(ctx, transaction.ID.String(), clientID.String(), string(transaction.Type), amount, currency); err != nil {
			s.logger.Warn().Err(err).Msg("failed to publish transaction completed event")
		}
	}

	return transaction, nil
}

// GetTransactionHistory получает историю транзакций
func (s *BillingService) GetTransactionHistory(
	ctx context.Context,
	clientID uuid.UUID,
	limit, offset int,
) ([]*domain.Transaction, error) {
	return s.transactionRepo.GetByClientID(ctx, clientID, limit, offset)
}

// add складывает два числа в строковом формате
func (s *BillingService) add(a, b string) (string, error) {
	aBig := new(big.Float)
	bBig := new(big.Float)

	if _, ok := aBig.SetString(a); !ok {
		return "", fmt.Errorf("invalid number: %s", a)
	}
	if _, ok := bBig.SetString(b); !ok {
		return "", fmt.Errorf("invalid number: %s", b)
	}

	result := new(big.Float).Add(aBig, bBig)
	return result.Text('f', 6), nil
}

// subtract вычитает b из a
func (s *BillingService) subtract(a, b string) (string, error) {
	aBig := new(big.Float)
	bBig := new(big.Float)

	if _, ok := aBig.SetString(a); !ok {
		return "", fmt.Errorf("invalid number: %s", a)
	}
	if _, ok := bBig.SetString(b); !ok {
		return "", fmt.Errorf("invalid number: %s", b)
	}

	result := new(big.Float).Sub(aBig, bBig)
	return result.Text('f', 6), nil
}

// isNegative проверяет, является ли число отрицательным
func (s *BillingService) isNegative(value string) bool {
	bigFloat := new(big.Float)
	if _, ok := bigFloat.SetString(value); !ok {
		return false
	}
	return bigFloat.Sign() < 0
}
