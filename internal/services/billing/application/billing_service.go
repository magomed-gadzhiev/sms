package application

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/billing/domain"
)

// BillingService предоставляет бизнес-логику для биллинга
type BillingService struct {
	accountRepo     domain.AccountRepository
	transactionRepo domain.TransactionRepository
	transferRepo    domain.TransferRepository
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

// SetTransferRepo устанавливает репозиторий переводов
func (s *BillingService) SetTransferRepo(transferRepo domain.TransferRepository) {
	s.transferRepo = transferRepo
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

// TransferBalance переводит средства между клиентами (атомарная операция)
func (s *BillingService) TransferBalance(
	ctx context.Context,
	fromClientID, toClientID uuid.UUID,
	amount, currency string,
) (transferID, fromBalance, toBalance string, err error) {
	// Получаем счёт отправителя
	fromAccount, err := s.accountRepo.GetByClientID(ctx, fromClientID)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get sender account: %w", err)
	}

	// Проверяем валюту
	if fromAccount.Currency != currency {
		return "", "", "", fmt.Errorf("currency mismatch: sender account has %s, but %s provided", fromAccount.Currency, currency)
	}

	// Проверяем баланс отправителя
	newFromBalance, err := s.subtract(fromAccount.Balance, amount)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to calculate sender balance: %w", err)
	}

	if s.isNegative(newFromBalance) {
		return "", "", "", domain.ErrInsufficientBalance
	}

	// Получаем или создаем счёт получателя
	toAccount, err := s.accountRepo.GetByClientID(ctx, toClientID)
	if err != nil {
		if err != domain.ErrAccountNotFound {
			return "", "", "", fmt.Errorf("failed to get receiver account: %w", err)
		}
		// Создаем новый счёт для получателя
		toAccount = domain.NewAccount(toClientID, currency)
		if err := s.accountRepo.Create(ctx, toAccount); err != nil {
			return "", "", "", fmt.Errorf("failed to create receiver account: %w", err)
		}
	}

	if toAccount.Currency != currency {
		return "", "", "", fmt.Errorf("currency mismatch: receiver account has %s, but %s provided", toAccount.Currency, currency)
	}

	newToBalance, err := s.add(toAccount.Balance, amount)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to calculate receiver balance: %w", err)
	}

	// Списываем со счёта отправителя
	if err := s.accountRepo.UpdateBalance(ctx, fromClientID, newFromBalance); err != nil {
		return "", "", "", fmt.Errorf("failed to debit sender: %w", err)
	}

	// Зачисляем на счёт получателя
	if err := s.accountRepo.UpdateBalance(ctx, toClientID, newToBalance); err != nil {
		return "", "", "", fmt.Errorf("failed to credit receiver: %w", err)
	}

	// Создаем транзакцию списания (transfer_out)
	fromTx := domain.NewTransaction(
		fromClientID,
		domain.TransactionTypeTransferOut,
		amount,
		fromAccount.Balance,
		newFromBalance,
		currency,
	).WithDescription(fmt.Sprintf("Transfer to %s", toClientID.String()))

	if err := s.transactionRepo.Create(ctx, fromTx); err != nil {
		return "", "", "", fmt.Errorf("failed to create transfer_out transaction: %w", err)
	}

	// Создаем транзакцию зачисления (transfer_in)
	toTx := domain.NewTransaction(
		toClientID,
		domain.TransactionTypeTransferIn,
		amount,
		toAccount.Balance,
		newToBalance,
		currency,
	).WithDescription(fmt.Sprintf("Transfer from %s", fromClientID.String()))

	if err := s.transactionRepo.Create(ctx, toTx); err != nil {
		return "", "", "", fmt.Errorf("failed to create transfer_in transaction: %w", err)
	}

	// Создаем запись о переводе
	transfer := domain.NewBalanceTransfer(fromClientID, toClientID, amount, currency, fromTx.ID, toTx.ID)

	if s.transferRepo != nil {
		if err := s.transferRepo.Create(ctx, transfer); err != nil {
			s.logger.Warn().Err(err).Msg("failed to save balance transfer record")
		}
	}

	// Публикуем события
	if s.eventPublisher != nil {
		if err := s.eventPublisher.PublishBalanceChanged(ctx, fromClientID.String(), newFromBalance, currency); err != nil {
			s.logger.Warn().Err(err).Msg("failed to publish sender balance changed event")
		}
		if err := s.eventPublisher.PublishBalanceChanged(ctx, toClientID.String(), newToBalance, currency); err != nil {
			s.logger.Warn().Err(err).Msg("failed to publish receiver balance changed event")
		}
	}

	s.logger.Info().
		Str("transfer_id", transfer.ID.String()).
		Str("from_client_id", fromClientID.String()).
		Str("to_client_id", toClientID.String()).
		Str("amount", amount).
		Str("currency", currency).
		Msg("balance transfer completed")

	return transfer.ID.String(), newFromBalance, newToBalance, nil
}

// FreezeAccount замораживает счет клиента
func (s *BillingService) FreezeAccount(ctx context.Context, clientID uuid.UUID, adminID uuid.UUID) (time.Time, error) {
	frozenAt, err := s.accountRepo.FreezeAccount(ctx, clientID, adminID)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to freeze account: %w", err)
	}
	return frozenAt, nil
}

// UnfreezeAccount размораживает счет клиента
func (s *BillingService) UnfreezeAccount(ctx context.Context, clientID uuid.UUID) error {
	if err := s.accountRepo.UnfreezeAccount(ctx, clientID); err != nil {
		return fmt.Errorf("failed to unfreeze account: %w", err)
	}
	return nil
}

// SetCreditLimit устанавливает кредитный лимит для клиента
func (s *BillingService) SetCreditLimit(ctx context.Context, clientID uuid.UUID, limit string) error {
	if err := s.accountRepo.SetCreditLimit(ctx, clientID, limit); err != nil {
		return fmt.Errorf("failed to set credit limit: %w", err)
	}
	return nil
}

// SetLowBalanceThreshold устанавливает порог низкого баланса
func (s *BillingService) SetLowBalanceThreshold(ctx context.Context, clientID uuid.UUID, threshold string) error {
	if err := s.accountRepo.SetLowBalanceThreshold(ctx, clientID, threshold); err != nil {
		return fmt.Errorf("failed to set low balance threshold: %w", err)
	}
	return nil
}

// ListBalances получает список балансов с фильтрацией
func (s *BillingService) ListBalances(ctx context.Context, search string, status string, belowThreshold bool, limit, offset int32) ([]domain.BalanceInfo, int32, error) {
	balances, total, err := s.accountRepo.ListBalances(ctx, search, status, belowThreshold, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list balances: %w", err)
	}
	return balances, total, nil
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
