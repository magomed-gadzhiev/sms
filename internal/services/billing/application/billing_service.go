package application

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
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
	// commitGuard — канонический владелец идемпотентности списаний за
	// Message (commit_idempotency_guard). ChargeDirect-путь обязан быть
	// сконфигурирован guard'ом (см. SetCommitGuard).
	commitGuard domain.CommitIdempotencyGuard
	logger      zerolog.Logger
}

// SetCommitGuard устанавливает guard идемпотентности для ChargeMessage.
// Вызывается на wiring-уровне (cmd/billing); без guard production-путь
// ChargeMessage отклоняет запросы.
func (s *BillingService) SetCommitGuard(guard domain.CommitIdempotencyGuard) {
	s.commitGuard = guard
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
	// Пробуем использовать транзакцию если репозиторий поддерживает
	type txAccountRepo interface {
		BeginTx(ctx context.Context) (*sqlx.Tx, error)
		GetByClientIDForUpdate(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID) (*domain.Account, error)
		UpdateBalanceTx(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID, newBalance string) error
	}
	type txTransactionRepo interface {
		CreateTx(ctx context.Context, tx *sqlx.Tx, transaction *domain.Transaction) error
	}

	accRepo, accOk := s.accountRepo.(txAccountRepo)
	txnRepo, txnOk := s.transactionRepo.(txTransactionRepo)

	if accOk && txnOk {
		return s.addCreditsWithTx(ctx, accRepo, txnRepo, clientID, amount, currency, description, paymentMethod)
	}

	// Fallback без транзакции (для тестов с моками)
	return s.addCreditsNoTx(ctx, clientID, amount, currency, description, paymentMethod)
}

// addCreditsWithTx добавляет средства атомарно через DB-транзакцию
func (s *BillingService) addCreditsWithTx(
	ctx context.Context,
	accRepo interface {
		BeginTx(ctx context.Context) (*sqlx.Tx, error)
		GetByClientIDForUpdate(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID) (*domain.Account, error)
		UpdateBalanceTx(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID, newBalance string) error
	},
	txnRepo interface {
		CreateTx(ctx context.Context, tx *sqlx.Tx, transaction *domain.Transaction) error
	},
	clientID uuid.UUID,
	amount, currency, description string,
	paymentMethod *string,
) (*domain.Transaction, error) {
	// Создаём счёт если его нет (вне транзакции — идемпотентно)
	account, err := s.accountRepo.GetByClientID(ctx, clientID)
	if err != nil {
		if err != domain.ErrAccountNotFound {
			return nil, fmt.Errorf("failed to get account: %w", err)
		}
		account = domain.NewAccount(clientID, currency)
		if err := account.Validate(); err != nil {
			return nil, fmt.Errorf("invalid account: %w", err)
		}
		if err := s.accountRepo.Create(ctx, account); err != nil {
			return nil, fmt.Errorf("failed to create account: %w", err)
		}
	}

	tx, err := accRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Блокируем счёт для атомарного обновления
	account, err = accRepo.GetByClientIDForUpdate(ctx, tx, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account for update: %w", err)
	}

	if currency == "" {
		currency = account.Currency
	} else if account.Currency != currency {
		return nil, fmt.Errorf("currency mismatch: account has %s, but %s provided", account.Currency, currency)
	}

	newBalance, err := s.add(account.Balance, amount)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate new balance: %w", err)
	}

	if err := accRepo.UpdateBalanceTx(ctx, tx, clientID, newBalance); err != nil {
		return nil, fmt.Errorf("failed to update balance: %w", err)
	}

	transaction := domain.NewTransaction(
		clientID, domain.TransactionTypeCredit, amount,
		account.Balance, newBalance, currency,
	).WithDescription(description)

	if paymentMethod != nil {
		transaction = transaction.WithPaymentMethod(*paymentMethod)
	}

	if err := transaction.Validate(); err != nil {
		return nil, fmt.Errorf("invalid transaction: %w", err)
	}

	if err := txnRepo.CreateTx(ctx, tx, transaction); err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

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

// addCreditsNoTx — fallback без транзакции (для тестов с моками)
func (s *BillingService) addCreditsNoTx(
	ctx context.Context,
	clientID uuid.UUID,
	amount, currency, description string,
	paymentMethod *string,
) (*domain.Transaction, error) {
	account, err := s.accountRepo.GetByClientID(ctx, clientID)
	if err != nil {
		if err != domain.ErrAccountNotFound {
			return nil, fmt.Errorf("failed to get account: %w", err)
		}
		account = domain.NewAccount(clientID, currency)
		if err := account.Validate(); err != nil {
			return nil, fmt.Errorf("invalid account: %w", err)
		}
		if err := s.accountRepo.Create(ctx, account); err != nil {
			return nil, fmt.Errorf("failed to create account: %w", err)
		}
	}

	if currency == "" {
		currency = account.Currency
	} else if account.Currency != currency {
		return nil, fmt.Errorf("currency mismatch: account has %s, but %s provided", account.Currency, currency)
	}

	newBalance, err := s.add(account.Balance, amount)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate new balance: %w", err)
	}

	if err := s.accountRepo.UpdateBalance(ctx, clientID, newBalance); err != nil {
		return nil, fmt.Errorf("failed to update balance: %w", err)
	}

	transaction := domain.NewTransaction(
		clientID, domain.TransactionTypeCredit, amount,
		account.Balance, newBalance, currency,
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
	// Пробуем использовать транзакцию если репозиторий поддерживает
	type txRepo interface {
		BeginTx(ctx context.Context) (*sqlx.Tx, error)
		GetByClientIDForUpdate(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID) (*domain.Account, error)
		UpdateBalanceTx(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID, newBalance string) error
	}

	if repo, ok := s.accountRepo.(txRepo); ok {
		return s.deductCreditsWithTx(ctx, repo, clientID, amount, currency, description)
	}

	// Fallback без транзакции (для тестов с моками)
	return s.deductCreditsNoTx(ctx, clientID, amount, currency, description)
}

// deductCreditsWithTx списывает средства с использованием транзакции и SELECT FOR UPDATE
func (s *BillingService) deductCreditsWithTx(
	ctx context.Context,
	repo interface {
		BeginTx(ctx context.Context) (*sqlx.Tx, error)
		GetByClientIDForUpdate(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID) (*domain.Account, error)
		UpdateBalanceTx(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID, newBalance string) error
	},
	clientID uuid.UUID,
	amount, currency, description string,
) (*domain.Transaction, error) {
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	account, err := repo.GetByClientIDForUpdate(ctx, tx, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account for update: %w", err)
	}

	// Проверяем заморозку
	if account.Frozen {
		return nil, domain.ErrAccountFrozen
	}

	// Проверяем валюту: если не указана — используем валюту аккаунта
	if currency == "" {
		currency = account.Currency
	} else if account.Currency != currency {
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

	// Обновляем баланс в рамках транзакции
	if err := repo.UpdateBalanceTx(ctx, tx, clientID, newBalance); err != nil {
		return nil, fmt.Errorf("failed to update balance: %w", err)
	}

	// Создаём запись транзакции ВНУТРИ DB-транзакции (до коммита)
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

	// Записываем транзакцию через tx, если репозиторий поддерживает
	type txTransactionRepo interface {
		CreateTx(ctx context.Context, tx *sqlx.Tx, transaction *domain.Transaction) error
	}
	if txRepo, ok := s.transactionRepo.(txTransactionRepo); ok {
		if err := txRepo.CreateTx(ctx, tx, transaction); err != nil {
			return nil, fmt.Errorf("failed to create transaction: %w", err)
		}
	} else {
		// Fallback: создаём вне транзакции (для старых реализаций)
		if err := s.transactionRepo.Create(ctx, transaction); err != nil {
			return nil, fmt.Errorf("failed to create transaction: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Публикуем события (после коммита — идемпотентно)
	if s.eventPublisher != nil {
		if err := s.eventPublisher.PublishBalanceChanged(ctx, clientID.String(), newBalance, currency); err != nil {
			s.logger.Warn().Err(err).Msg("failed to publish balance changed event")
		}
		if err := s.eventPublisher.PublishTransactionCompleted(ctx, transaction.ID.String(), clientID.String(), string(transaction.Type), amount, currency); err != nil {
			s.logger.Warn().Err(err).Msg("failed to publish transaction completed event")
		}
		s.publishLowBalanceAlertIfNeeded(ctx, clientID.String(), newBalance, account.LowBalanceThreshold, currency)
	}

	return transaction, nil
}

// deductCreditsNoTx списывает средства без транзакции (fallback для тестов)
func (s *BillingService) deductCreditsNoTx(
	ctx context.Context,
	clientID uuid.UUID,
	amount, currency, description string,
) (*domain.Transaction, error) {
	account, err := s.accountRepo.GetByClientID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	// Проверяем валюту: если не указана — используем валюту аккаунта
	if currency == "" {
		currency = account.Currency
	} else if account.Currency != currency {
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
		s.publishLowBalanceAlertIfNeeded(ctx, clientID.String(), newBalance, account.LowBalanceThreshold, currency)
	}

	return transaction, nil
}

// ChargeMessage списывает средства за сообщение
func (s *BillingService) ChargeMessage(
	ctx context.Context,
	clientID, messageID uuid.UUID,
	amount, currency, description string,
) (*domain.Transaction, error) {
	// Идемпотентность — commit_idempotency_guard.ClaimTx внутри транзакции
	// (см. chargeMessageWithTx). Прежний check-before-tx (GetByMessageID до
	// BeginTx) удалён: он был race-prone (два конкурентных вызова проходили
	// проверку и списывали дважды) и дублировал guard-механизм.

	// Пробуем использовать транзакцию если репозиторий поддерживает
	type txRepo interface {
		BeginTx(ctx context.Context) (*sqlx.Tx, error)
		GetByClientIDForUpdate(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID) (*domain.Account, error)
		UpdateBalanceTx(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID, newBalance string) error
	}

	if repo, ok := s.accountRepo.(txRepo); ok {
		return s.chargeMessageWithTx(ctx, repo, clientID, messageID, amount, currency, description)
	}

	// Fallback без транзакции (для тестов с моками)
	return s.chargeMessageNoTx(ctx, clientID, messageID, amount, currency, description)
}

// chargeMessageWithTx списывает средства за сообщение с использованием транзакции и SELECT FOR UPDATE
func (s *BillingService) chargeMessageWithTx(
	ctx context.Context,
	repo interface {
		BeginTx(ctx context.Context) (*sqlx.Tx, error)
		GetByClientIDForUpdate(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID) (*domain.Account, error)
		UpdateBalanceTx(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID, newBalance string) error
	},
	clientID, messageID uuid.UUID,
	amount, currency, description string,
) (*domain.Transaction, error) {
	if s.commitGuard == nil {
		return nil, fmt.Errorf("charge message %s: commit guard not configured (SetCommitGuard)", messageID)
	}

	tx, err := repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Канонический guard: claim в той же tx, что и сдвиг баланса. Повторный
	// вызов с тем же message_id получает already-committed поведение —
	// guard выживает только если исходная tx закоммитилась.
	claimed, err := s.commitGuard.ClaimTx(ctx, tx, messageID)
	if err != nil {
		return nil, fmt.Errorf("idempotency guard: %w", err)
	}
	if !claimed {
		// Replay: списание уже было — возвращаем существующую транзакцию
		// (read-only, после отката текущей tx).
		_ = tx.Rollback()
		existing, err := s.transactionRepo.GetByMessageID(ctx, messageID)
		if err != nil || existing == nil {
			s.logger.Warn().Err(err).
				Str("message_id", messageID.String()).
				Msg("guard says committed but transaction row not found")
			return nil, fmt.Errorf("charge message %s: already committed but transaction row unavailable", messageID)
		}
		s.logger.Debug().
			Str("message_id", messageID.String()).
			Str("transaction_id", existing.ID.String()).
			Msg("transaction already exists for message (guard replay)")
		return existing, nil
	}

	account, err := repo.GetByClientIDForUpdate(ctx, tx, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account for update: %w", err)
	}

	// Проверяем заморозку
	if account.Frozen {
		return nil, domain.ErrAccountFrozen
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

	// Обновляем баланс в рамках транзакции
	if err := repo.UpdateBalanceTx(ctx, tx, clientID, newBalance); err != nil {
		return nil, fmt.Errorf("failed to update balance: %w", err)
	}

	// Создаём запись транзакции ВНУТРИ DB-транзакции (до коммита)
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

	type txTransactionRepo interface {
		CreateTx(ctx context.Context, tx *sqlx.Tx, transaction *domain.Transaction) error
	}
	if txRepo, ok := s.transactionRepo.(txTransactionRepo); ok {
		if err := txRepo.CreateTx(ctx, tx, transaction); err != nil {
			return nil, fmt.Errorf("failed to create transaction: %w", err)
		}
	} else {
		if err := s.transactionRepo.Create(ctx, transaction); err != nil {
			return nil, fmt.Errorf("failed to create transaction: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Публикуем события (после коммита)
	if s.eventPublisher != nil {
		if err := s.eventPublisher.PublishBalanceChanged(ctx, clientID.String(), newBalance, currency); err != nil {
			s.logger.Warn().Err(err).Msg("failed to publish balance changed event")
		}
		if err := s.eventPublisher.PublishTransactionCompleted(ctx, transaction.ID.String(), clientID.String(), string(transaction.Type), amount, currency); err != nil {
			s.logger.Warn().Err(err).Msg("failed to publish transaction completed event")
		}
		s.publishLowBalanceAlertIfNeeded(ctx, clientID.String(), newBalance, account.LowBalanceThreshold, currency)
	}

	return transaction, nil
}

// chargeMessageNoTx списывает средства за сообщение без транзакции (fallback для тестов)
func (s *BillingService) chargeMessageNoTx(
	ctx context.Context,
	clientID, messageID uuid.UUID,
	amount, currency, description string,
) (*domain.Transaction, error) {
	// Mock-мир без *sqlx.Tx: guard неприменим, replay-проверка остаётся здесь.
	// Production-путь (chargeMessageWithTx) идемпотентен через commit guard.
	if existing, err := s.transactionRepo.GetByMessageID(ctx, messageID); err == nil && existing != nil {
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
		s.publishLowBalanceAlertIfNeeded(ctx, clientID.String(), newBalance, account.LowBalanceThreshold, currency)
	}

	return transaction, nil
}

// GetTransactionHistory получает историю транзакций
func (s *BillingService) GetTransactionHistory(
	ctx context.Context,
	clientID *uuid.UUID,
	limit, offset int,
) ([]*domain.Transaction, error) {
	if clientID == nil {
		return s.transactionRepo.GetAll(ctx, limit, offset)
	}
	return s.transactionRepo.GetByClientID(ctx, *clientID, limit, offset)
}

// TransferBalance переводит средства между клиентами (атомарная операция)
func (s *BillingService) TransferBalance(
	ctx context.Context,
	fromClientID, toClientID uuid.UUID,
	amount, currency string,
) (transferID, fromBalance, toBalance string, err error) {
	// Пробуем использовать транзакцию если репозиторий поддерживает
	type txAccountRepo interface {
		BeginTx(ctx context.Context) (*sqlx.Tx, error)
		GetByClientIDForUpdate(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID) (*domain.Account, error)
		UpdateBalanceTx(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID, newBalance string) error
	}
	type txTransactionRepo interface {
		CreateTx(ctx context.Context, tx *sqlx.Tx, transaction *domain.Transaction) error
	}

	accRepo, accOk := s.accountRepo.(txAccountRepo)
	txnRepo, txnOk := s.transactionRepo.(txTransactionRepo)

	if accOk && txnOk {
		return s.transferBalanceWithTx(ctx, accRepo, txnRepo, fromClientID, toClientID, amount, currency)
	}

	// Fallback без транзакции (для тестов с моками)
	return s.transferBalanceNoTx(ctx, fromClientID, toClientID, amount, currency)
}

// transferBalanceWithTx выполняет перевод в рамках DB-транзакции с SELECT FOR UPDATE
func (s *BillingService) transferBalanceWithTx(
	ctx context.Context,
	accRepo interface {
		BeginTx(ctx context.Context) (*sqlx.Tx, error)
		GetByClientIDForUpdate(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID) (*domain.Account, error)
		UpdateBalanceTx(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID, newBalance string) error
	},
	txnRepo interface {
		CreateTx(ctx context.Context, tx *sqlx.Tx, transaction *domain.Transaction) error
	},
	fromClientID, toClientID uuid.UUID,
	amount, currency string,
) (string, string, string, error) {
	tx, err := accRepo.BeginTx(ctx)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Блокируем оба счёта в стабильном порядке (по UUID) для предотвращения deadlock
	var firstID, secondID uuid.UUID
	if fromClientID.String() < toClientID.String() {
		firstID, secondID = fromClientID, toClientID
	} else {
		firstID, secondID = toClientID, fromClientID
	}

	first, err := accRepo.GetByClientIDForUpdate(ctx, tx, firstID)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) && firstID == toClientID {
			// Создаем счёт получателя внутри транзакции
			first = domain.NewAccount(toClientID, currency)
			createQuery := `INSERT INTO accounts (id, client_id, balance, currency, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6)`
			if _, execErr := tx.ExecContext(ctx, createQuery,
				first.ID, first.ClientID, first.Balance, first.Currency, first.CreatedAt, first.UpdatedAt,
			); execErr != nil {
				return "", "", "", fmt.Errorf("failed to create receiver account: %w", execErr)
			}
		} else {
			return "", "", "", fmt.Errorf("failed to lock account %s: %w", firstID, err)
		}
	}

	second, err := accRepo.GetByClientIDForUpdate(ctx, tx, secondID)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) && secondID == toClientID {
			second = domain.NewAccount(toClientID, currency)
			createQuery := `INSERT INTO accounts (id, client_id, balance, currency, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6)`
			if _, execErr := tx.ExecContext(ctx, createQuery,
				second.ID, second.ClientID, second.Balance, second.Currency, second.CreatedAt, second.UpdatedAt,
			); execErr != nil {
				return "", "", "", fmt.Errorf("failed to create receiver account: %w", execErr)
			}
		} else {
			return "", "", "", fmt.Errorf("failed to lock account %s: %w", secondID, err)
		}
	}

	// Определяем fromAccount и toAccount
	var fromAccount, toAccount *domain.Account
	if firstID == fromClientID {
		fromAccount, toAccount = first, second
	} else {
		fromAccount, toAccount = second, first
	}

	// Проверяем валюту отправителя
	if fromAccount.Currency != currency {
		return "", "", "", fmt.Errorf("currency mismatch: sender account has %s, but %s provided", fromAccount.Currency, currency)
	}
	if toAccount.Currency != currency {
		return "", "", "", fmt.Errorf("currency mismatch: receiver account has %s, but %s provided", toAccount.Currency, currency)
	}

	// Вычисляем новые балансы
	newFromBalance, err := s.subtract(fromAccount.Balance, amount)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to calculate sender balance: %w", err)
	}
	if s.isNegative(newFromBalance) {
		return "", "", "", domain.ErrInsufficientBalance
	}

	newToBalance, err := s.add(toAccount.Balance, amount)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to calculate receiver balance: %w", err)
	}

	// Обновляем балансы в рамках транзакции
	if err := accRepo.UpdateBalanceTx(ctx, tx, fromClientID, newFromBalance); err != nil {
		return "", "", "", fmt.Errorf("failed to debit sender: %w", err)
	}
	if err := accRepo.UpdateBalanceTx(ctx, tx, toClientID, newToBalance); err != nil {
		return "", "", "", fmt.Errorf("failed to credit receiver: %w", err)
	}

	// Создаём записи транзакций внутри DB-транзакции
	fromTx := domain.NewTransaction(
		fromClientID, domain.TransactionTypeTransferOut, amount,
		fromAccount.Balance, newFromBalance, currency,
	).WithDescription(fmt.Sprintf("Transfer to %s", toClientID.String()))

	if err := txnRepo.CreateTx(ctx, tx, fromTx); err != nil {
		return "", "", "", fmt.Errorf("failed to create transfer_out transaction: %w", err)
	}

	toTx := domain.NewTransaction(
		toClientID, domain.TransactionTypeTransferIn, amount,
		toAccount.Balance, newToBalance, currency,
	).WithDescription(fmt.Sprintf("Transfer from %s", fromClientID.String()))

	if err := txnRepo.CreateTx(ctx, tx, toTx); err != nil {
		return "", "", "", fmt.Errorf("failed to create transfer_in transaction: %w", err)
	}

	// Коммитим всё атомарно
	if err := tx.Commit(); err != nil {
		return "", "", "", fmt.Errorf("failed to commit transfer transaction: %w", err)
	}

	// Создаем запись о переводе (вне транзакции — не критично)
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

// transferBalanceNoTx — fallback для тестов с моками (без DB-транзакции)
func (s *BillingService) transferBalanceNoTx(
	ctx context.Context,
	fromClientID, toClientID uuid.UUID,
	amount, currency string,
) (string, string, string, error) {
	fromAccount, err := s.accountRepo.GetByClientID(ctx, fromClientID)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get sender account: %w", err)
	}
	if fromAccount.Currency != currency {
		return "", "", "", fmt.Errorf("currency mismatch: sender account has %s, but %s provided", fromAccount.Currency, currency)
	}

	newFromBalance, err := s.subtract(fromAccount.Balance, amount)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to calculate sender balance: %w", err)
	}
	if s.isNegative(newFromBalance) {
		return "", "", "", domain.ErrInsufficientBalance
	}

	toAccount, err := s.accountRepo.GetByClientID(ctx, toClientID)
	if err != nil {
		if err != domain.ErrAccountNotFound {
			return "", "", "", fmt.Errorf("failed to get receiver account: %w", err)
		}
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

	if err := s.accountRepo.UpdateBalance(ctx, fromClientID, newFromBalance); err != nil {
		return "", "", "", fmt.Errorf("failed to debit sender: %w", err)
	}
	if err := s.accountRepo.UpdateBalance(ctx, toClientID, newToBalance); err != nil {
		return "", "", "", fmt.Errorf("failed to credit receiver: %w", err)
	}

	fromTx := domain.NewTransaction(
		fromClientID, domain.TransactionTypeTransferOut, amount,
		fromAccount.Balance, newFromBalance, currency,
	).WithDescription(fmt.Sprintf("Transfer to %s", toClientID.String()))
	if err := s.transactionRepo.Create(ctx, fromTx); err != nil {
		return "", "", "", fmt.Errorf("failed to create transfer_out transaction: %w", err)
	}

	toTx := domain.NewTransaction(
		toClientID, domain.TransactionTypeTransferIn, amount,
		toAccount.Balance, newToBalance, currency,
	).WithDescription(fmt.Sprintf("Transfer from %s", fromClientID.String()))
	if err := s.transactionRepo.Create(ctx, toTx); err != nil {
		return "", "", "", fmt.Errorf("failed to create transfer_in transaction: %w", err)
	}

	transfer := domain.NewBalanceTransfer(fromClientID, toClientID, amount, currency, fromTx.ID, toTx.ID)
	if s.transferRepo != nil {
		if err := s.transferRepo.Create(ctx, transfer); err != nil {
			s.logger.Warn().Err(err).Msg("failed to save balance transfer record")
		}
	}

	if s.eventPublisher != nil {
		if err := s.eventPublisher.PublishBalanceChanged(ctx, fromClientID.String(), newFromBalance, currency); err != nil {
			s.logger.Warn().Err(err).Msg("failed to publish sender balance changed event")
		}
		if err := s.eventPublisher.PublishBalanceChanged(ctx, toClientID.String(), newToBalance, currency); err != nil {
			s.logger.Warn().Err(err).Msg("failed to publish receiver balance changed event")
		}
	}

	s.logger.Info().
		Str("from_client_id", fromClientID.String()).
		Str("to_client_id", toClientID.String()).
		Str("amount", amount).
		Str("currency", currency).
		Msg("balance transfer completed (no-tx fallback)")

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

// isBelowThreshold проверяет, упал ли баланс ниже порогового значения
func (s *BillingService) isBelowThreshold(balance, threshold string) bool {
	if threshold == "" || threshold == "0" {
		return false
	}
	balanceBig := new(big.Float)
	thresholdBig := new(big.Float)
	if _, ok := balanceBig.SetString(balance); !ok {
		return false
	}
	if _, ok := thresholdBig.SetString(threshold); !ok {
		return false
	}
	return balanceBig.Cmp(thresholdBig) < 0
}

// publishLowBalanceAlertIfNeeded проверяет и публикует уведомление о низком балансе
func (s *BillingService) publishLowBalanceAlertIfNeeded(ctx context.Context, clientID, newBalance, threshold, currency string) {
	if s.eventPublisher == nil {
		return
	}
	if !s.isBelowThreshold(newBalance, threshold) {
		return
	}
	if err := s.eventPublisher.PublishBalanceLow(ctx, clientID, newBalance, threshold, currency); err != nil {
		s.logger.Warn().Err(err).
			Str("client_id", clientID).
			Msg("failed to publish balance.low event")
	}
}
