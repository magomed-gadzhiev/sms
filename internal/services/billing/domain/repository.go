package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// BalanceInfo содержит информацию о балансе клиента с именем
type BalanceInfo struct {
	ClientID            uuid.UUID
	ClientName          string
	Balance             string
	Currency            string
	Frozen              bool
	CreditLimit         string
	LowBalanceThreshold string
	FrozenAt            *time.Time
	FrozenBy            *uuid.UUID
	UpdatedAt           time.Time
}

// AccountRepository определяет интерфейс репозитория счетов
type AccountRepository interface {
	Create(ctx context.Context, account *Account) error
	GetByClientID(ctx context.Context, clientID uuid.UUID) (*Account, error)
	Update(ctx context.Context, account *Account) error
	UpdateBalance(ctx context.Context, clientID uuid.UUID, newBalance string) error
	FreezeAccount(ctx context.Context, clientID uuid.UUID, adminID uuid.UUID) (time.Time, error)
	UnfreezeAccount(ctx context.Context, clientID uuid.UUID) error
	SetCreditLimit(ctx context.Context, clientID uuid.UUID, limit string) error
	SetLowBalanceThreshold(ctx context.Context, clientID uuid.UUID, threshold string) error
	ListBalances(ctx context.Context, search string, status string, belowThreshold bool, limit, offset int32) ([]BalanceInfo, int32, error)
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
