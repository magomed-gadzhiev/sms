package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AccountRepository определяет интерфейс репозитория счетов
type AccountRepository interface {
	Create(ctx context.Context, account *Account) error
	GetByClientID(ctx context.Context, clientID uuid.UUID) (*Account, error)
	Update(ctx context.Context, account *Account) error
	UpdateBalance(ctx context.Context, clientID uuid.UUID, newBalance string) error
}

// TransactionRepository определяет интерфейс репозитория транзакций
type TransactionRepository interface {
	Create(ctx context.Context, transaction *Transaction) error
	GetByID(ctx context.Context, id uuid.UUID) (*Transaction, error)
	GetByClientID(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*Transaction, error)
	GetByClientIDAndType(ctx context.Context, clientID uuid.UUID, transactionType TransactionType, limit, offset int) ([]*Transaction, error)
	GetByClientIDAndPeriod(ctx context.Context, clientID uuid.UUID, from, to time.Time, limit, offset int) ([]*Transaction, error)
	GetByMessageID(ctx context.Context, messageID uuid.UUID) (*Transaction, error)
}

// TransferRepository определяет интерфейс репозитория переводов
type TransferRepository interface {
	Create(ctx context.Context, transfer *BalanceTransfer) error
	GetByID(ctx context.Context, id uuid.UUID) (*BalanceTransfer, error)
}

// PricingRuleRepository определяет интерфейс репозитория правил тарификации
type PricingRuleRepository interface {
	Create(ctx context.Context, rule *PricingRule) error
	GetByID(ctx context.Context, id uuid.UUID) (*PricingRule, error)
	GetByClientID(ctx context.Context, clientID *uuid.UUID, activeOnly bool) ([]*PricingRule, error)
	GetGlobalRules(ctx context.Context, activeOnly bool) ([]*PricingRule, error)
	GetMatchingRule(ctx context.Context, clientID *uuid.UUID, destination string) (*PricingRule, error)
	Update(ctx context.Context, rule *PricingRule) error
	Delete(ctx context.Context, id uuid.UUID) error
}
