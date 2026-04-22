package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	billingDomain "github.com/smpp-server/smpp-server/internal/services/billing/domain"
	tarDomain "github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// DualChargeDeps — зависимости, нужные ChargeMessageDual поверх обычного BillingService.
// Собирается на уровне wiring'а (cmd/billing) из существующих репозиториев + *sqlx.DB.
type DualChargeDeps struct {
	DB              *sqlx.DB
	Guard           billingDomain.CommitIdempotencyGuard // idempotency guard на message_id
	QuotaRepo       AggregatorQuotaTxRepo
	MarginLogRepo   AggregatorMarginLogTxRepo
	AccountRepo     billingDomain.AccountRepository     // используется только для nil-checks
	TransactionRepo billingDomain.TransactionRepository // используется только для nil-checks
	EventPublisher  billingDomain.EventPublisher        // может быть nil
}

// AggregatorQuotaTxRepo — набор tx-методов, нужных dual-charge flow.
// Реальная реализация: *repository.AggregatorQuotaRepository.
type AggregatorQuotaTxRepo interface {
	GetActiveTx(ctx context.Context, tx *sqlx.Tx, aggregatorID uuid.UUID, now time.Time) (*tarDomain.AggregatorQuota, error)
	GetLatestTx(ctx context.Context, tx *sqlx.Tx, aggregatorID uuid.UUID) (*tarDomain.AggregatorQuota, error)
	CreateTx(ctx context.Context, tx *sqlx.Tx, q *tarDomain.AggregatorQuota) error
	IncrementUsageTx(ctx context.Context, tx *sqlx.Tx, quotaID uuid.UUID, segments int) (int64, int64, error)
}

// AggregatorMarginLogTxRepo — tx-методы margin log.
// Реальная реализация: *repository.AggregatorMarginLogRepository.
type AggregatorMarginLogTxRepo interface {
	CreateTx(ctx context.Context, tx *sqlx.Tx, entry *tarDomain.AggregatorMarginLog) (bool, error)
}

// ChargeMessageDualInput — вход атомарного двойного списания.
// Все money-поля — decimal-строки (NUMERIC в БД), чтобы избежать потерь float-арифметики.
type ChargeMessageDualInput struct {
	MessageID       uuid.UUID
	SubAccountID    uuid.UUID
	AggregatorID    uuid.UUID
	OperatorID      uuid.UUID
	SubAccountPrice string // per segment
	AggregatorPrice string // per segment (effective for charge_mode)
	SubAccountTotal string // общая сумма с субаккаунта
	AggregatorTotal string // общая сумма с агрегатора
	Currency        string
	SegmentCount    int
	PoolSegments    int
	OverageSegments int
	ChargeMode      string // "pool" | "overage" | "split"
}

// ChargeMessageDualResult — результат. Флаги взаимоисключающие с Committed.
type ChargeMessageDualResult struct {
	Committed        bool
	AlreadyCommitted bool
	QuotaMissing     bool
	SubInsufficient  bool
	AggInsufficient  bool
	SubTxID          uuid.UUID
	AggTxID          uuid.UUID
	MarginLogID      uuid.UUID
}

// ErrCurrencyMismatch — валюта аккаунта не совпадает с запросом.
var ErrCurrencyMismatch = errors.New("currency mismatch")

// ChargeMessageDual атомарно выполняет двойное списание: с субаккаунта (по тарифу агрегатора)
// и с агрегатора (по pool/overage/split цене). Порядок шагов в одной PG-транзакции:
//  1. INSERT aggregator_margin_log (idempotency-guard по message_id).
//     Если запись уже была — ROLLBACK, return AlreadyCommitted.
//  2. SELECT active quota FOR UPDATE; если нет — lazy-create из latest (при auto_renew=true),
//     иначе QuotaMissing.
//  3. UPDATE segments_used += N.
//  4. Charge sub-account.
//  5. Charge aggregator (с attributed_sub_account_id).
//  6. COMMIT.
//  7. Публикация событий (non-fatal, вне tx).
func ChargeMessageDual(
	ctx context.Context,
	deps DualChargeDeps,
	in ChargeMessageDualInput,
) (*ChargeMessageDualResult, error) {
	if in.Currency == "" {
		in.Currency = "RUB"
	}
	if in.SegmentCount <= 0 {
		return nil, fmt.Errorf("segment_count must be positive, got %d", in.SegmentCount)
	}

	tx, err := deps.DB.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	// Rollback идемпотентен — после Commit это no-op.
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()

	// 1. Idempotency guard — non-partitioned таблица с PK=message_id.
	// Причина: aggregator_margin_log partitioned by created_at, композитный
	// unique (idempotency_key, created_at) не работает для solo ON CONFLICT.
	// Отдельная guard-таблица с PK=message_id — чистый binary-guard до любых
	// shifts балансов. Rollback при любом последующем error откатывает guard
	// тоже — повторный вызов пройдёт заново.
	claimed, err := deps.Guard.ClaimTx(ctx, tx, in.MessageID)
	if err != nil {
		return nil, fmt.Errorf("idempotency guard: %w", err)
	}
	if !claimed {
		// Уже закоммичено — быстрый выход, балансы не трогаем.
		return &ChargeMessageDualResult{AlreadyCommitted: true}, nil
	}

	// 2. Lazy-create квоты при необходимости.
	quota, err := deps.QuotaRepo.GetActiveTx(ctx, tx, in.AggregatorID, now)
	if err != nil {
		return nil, fmt.Errorf("quota get active: %w", err)
	}
	if quota == nil {
		prev, err := deps.QuotaRepo.GetLatestTx(ctx, tx, in.AggregatorID)
		if err != nil {
			return nil, fmt.Errorf("quota get latest: %w", err)
		}
		if prev == nil || !prev.AutoRenew {
			return &ChargeMessageDualResult{QuotaMissing: true}, nil
		}
		// Создать квоту на текущий календарный месяц UTC, наследуя параметры.
		periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		periodEnd := periodStart.AddDate(0, 1, 0) // exclusive upper bound (period_end > now)
		newQuota := &tarDomain.AggregatorQuota{
			ID:           uuid.New(),
			AggregatorID: in.AggregatorID,
			PeriodStart:  periodStart,
			PeriodEnd:    periodEnd,
			SegmentLimit: prev.SegmentLimit,
			SegmentsUsed: 0,
			OverageRate:  prev.OverageRate,
			Currency:     prev.Currency,
			AutoRenew:    prev.AutoRenew,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := deps.QuotaRepo.CreateTx(ctx, tx, newQuota); err != nil {
			return nil, fmt.Errorf("quota lazy-create: %w", err)
		}
		// Перечитать: ON CONFLICT DO NOTHING мог ничего не вставить (concurrent first-call
		// с другим message_id) — тогда возьмём уже существующую строку.
		quota, err = deps.QuotaRepo.GetActiveTx(ctx, tx, in.AggregatorID, now)
		if err != nil {
			return nil, fmt.Errorf("quota re-read after create: %w", err)
		}
		if quota == nil {
			return nil, fmt.Errorf("quota re-read after create: row not found (aggregator=%s)", in.AggregatorID)
		}
	}

	// 3. Инкремент квоты.
	if _, _, err := deps.QuotaRepo.IncrementUsageTx(ctx, tx, quota.ID, in.SegmentCount); err != nil {
		return nil, fmt.Errorf("quota increment: %w", err)
	}

	// 4. Списание с субаккаунта.
	subTxID, subErr := chargeBalanceTx(ctx, tx, in.SubAccountID, in.MessageID, in.SubAccountTotal, in.Currency, "SMS subaccount", nil)
	if errors.Is(subErr, billingDomain.ErrInsufficientBalance) {
		return &ChargeMessageDualResult{SubInsufficient: true}, nil
	}
	if subErr != nil {
		return nil, fmt.Errorf("charge subaccount: %w", subErr)
	}

	// 5. Списание с агрегатора (с attributed_sub_account_id = subaccount).
	attributed := in.SubAccountID
	aggTxID, aggErr := chargeBalanceTx(ctx, tx, in.AggregatorID, in.MessageID, in.AggregatorTotal, in.Currency, "SMS aggregator", &attributed)
	if errors.Is(aggErr, billingDomain.ErrInsufficientBalance) {
		return &ChargeMessageDualResult{AggInsufficient: true}, nil
	}
	if aggErr != nil {
		return nil, fmt.Errorf("charge aggregator: %w", aggErr)
	}

	// 5a. Margin log — analytics запись, ON CONFLICT не нужен (guard уже
	// отсёк дубли выше). Вставляется в той же tx — атомарно с балансами.
	margin, err := billingDomain.SubtractAmount(in.SubAccountTotal, in.AggregatorTotal)
	if err != nil {
		return nil, fmt.Errorf("compute margin: %w", err)
	}
	marginID := uuid.New()
	marginEntry := &tarDomain.AggregatorMarginLog{
		ID:              marginID,
		AggregatorID:    in.AggregatorID,
		SubAccountID:    in.SubAccountID,
		MessageID:       in.MessageID,
		OperatorID:      in.OperatorID,
		SegmentCount:    in.SegmentCount,
		SubAccountPrice: in.SubAccountPrice,
		AggregatorPrice: in.AggregatorPrice,
		SubAccountTotal: in.SubAccountTotal,
		AggregatorTotal: in.AggregatorTotal,
		Margin:          margin,
		IdempotencyKey:  in.MessageID.String(),
		CreatedAt:       now,
		ChargeMode:      in.ChargeMode,
		PoolSegments:    in.PoolSegments,
		OverageSegments: in.OverageSegments,
	}
	if _, err := deps.MarginLogRepo.CreateTx(ctx, tx, marginEntry); err != nil {
		return nil, fmt.Errorf("margin_log insert: %w", err)
	}

	// 6. COMMIT.
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// 6a. Метрика маржи по charge_mode. Инкремент на фактический margin-value
	// (sub_total - agg_total). Parse ошибки — log warn, метрика пропускается:
	// билд-блокер из-за non-numeric строки здесь был бы хуже, чем потерянная метрика.
	if marginF, err := parseDecimal(margin); err == nil {
		billingAggregatorMarginByMode.
			WithLabelValues(in.AggregatorID.String(), in.ChargeMode).
			Add(marginF)
	} else {
		log.Warn().Err(err).Str("margin", margin).Msg("aggregator_margin metric: parse decimal failed")
	}

	// 7. События — non-fatal, вне транзакции.
	if deps.EventPublisher != nil {
		if err := deps.EventPublisher.PublishTransactionCompleted(ctx, subTxID.String(), in.SubAccountID.String(), string(billingDomain.TransactionTypeCharge), in.SubAccountTotal, in.Currency); err != nil {
			log.Warn().Err(err).Str("tx_id", subTxID.String()).Msg("publish sub transaction completed failed")
		}
		if err := deps.EventPublisher.PublishTransactionCompleted(ctx, aggTxID.String(), in.AggregatorID.String(), string(billingDomain.TransactionTypeCharge), in.AggregatorTotal, in.Currency); err != nil {
			log.Warn().Err(err).Str("tx_id", aggTxID.String()).Msg("publish agg transaction completed failed")
		}
	}

	return &ChargeMessageDualResult{
		Committed:   true,
		SubTxID:     subTxID,
		AggTxID:     aggTxID,
		MarginLogID: marginID,
	}, nil
}

// chargeBalanceTx атомарно списывает amount с баланса client_id и создаёт запись в transactions.
// Логика:
//  1. SELECT ... FOR UPDATE — читаем balance/currency/frozen под блокировкой.
//  2. Если frozen или currency mismatch — возвращаем соответствующую ошибку.
//  3. UPDATE accounts SET balance = balance - amount WHERE client_id = $1 AND balance >= amount
//     RETURNING balance. 0 rows → ErrInsufficientBalance.
//  4. INSERT transactions с attributed_sub_account_id (может быть nil).
//
// attributedSubAccount != nil только для записи агрегатора (атрибуция списания к конкретному
// субаккаунту для отчётов). Для субаккаунтной записи — nil.
func chargeBalanceTx(
	ctx context.Context,
	tx *sqlx.Tx,
	clientID, messageID uuid.UUID,
	amount, currency, description string,
	attributedSubAccount *uuid.UUID,
) (uuid.UUID, error) {
	// 1. Read with row lock для frozen/currency чек.
	var (
		oldBalance string
		accCurrency string
		frozen     bool
	)
	err := tx.QueryRowxContext(ctx, `
		SELECT balance::text, currency, frozen
		FROM accounts
		WHERE client_id = $1
		FOR UPDATE
	`, clientID).Scan(&oldBalance, &accCurrency, &frozen)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, billingDomain.ErrAccountNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("select account for update: %w", err)
	}
	if frozen {
		return uuid.Nil, billingDomain.ErrAccountFrozen
	}
	if accCurrency != currency {
		return uuid.Nil, fmt.Errorf("%w: account has %s, requested %s", ErrCurrencyMismatch, accCurrency, currency)
	}

	// 2. Атомарное списание — UPDATE с проверкой balance >= amount через WHERE.
	// CHECK (balance >= 0) в схеме тоже страхует, но мы хотим понятную ошибку вместо constraint violation.
	var newBalance string
	err = tx.QueryRowxContext(ctx, `
		UPDATE accounts
		SET balance = balance - $2::numeric,
		    updated_at = NOW()
		WHERE client_id = $1 AND balance >= $2::numeric
		RETURNING balance::text
	`, clientID, amount).Scan(&newBalance)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, billingDomain.ErrInsufficientBalance
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("update account balance: %w", err)
	}

	// 3. Запись транзакции.
	txID := uuid.New()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO transactions
			(id, client_id, type, amount, balance_before, balance_after, currency,
			 message_id, description, attributed_sub_account_id, created_at)
		VALUES ($1, $2, $3, $4::numeric, $5::numeric, $6::numeric, $7, $8, $9, $10, NOW())
	`, txID, clientID, string(billingDomain.TransactionTypeCharge), amount, oldBalance, newBalance, currency, messageID, description, attributedSubAccount)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert transaction: %w", err)
	}
	return txID, nil
}
