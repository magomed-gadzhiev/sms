package domain

import (
	"context"

	"github.com/google/uuid"
)

// ChargeOutcome — типизированный исход попытки списания за Message.
// Ровно один исход на вызов; заменяет наборы boolean-флагов, в которых
// «ничего не сработало» значило «неизвестно что».
type ChargeOutcome string

const (
	// ChargeOutcomeCommitted — списание закоммичено этим вызовом.
	ChargeOutcomeCommitted ChargeOutcome = "committed"
	// ChargeOutcomeAlreadyCommitted — replay: commit_idempotency_guard уже
	// закрыт более ранним вызовом; балансы не менялись.
	ChargeOutcomeAlreadyCommitted ChargeOutcome = "already_committed"
	// ChargeOutcomeRejected — бизнес-отказ; retry не изменит результат
	// (баланс/квота/тариф). Причина — в Rejection.
	ChargeOutcomeRejected ChargeOutcome = "rejected"
	// ChargeOutcomeTransient — инфраструктурный сбой; безопасно повторить.
	// Детали — в Err.
	ChargeOutcomeTransient ChargeOutcome = "transient"
)

// RejectionReason уточняет ChargeOutcomeRejected.
type RejectionReason string

const (
	RejectionInsufficientBalanceSub    RejectionReason = "insufficient_balance_subaccount"
	RejectionInsufficientBalanceAgg    RejectionReason = "insufficient_balance_aggregator"
	RejectionInsufficientBalanceDirect RejectionReason = "insufficient_balance"
	RejectionQuotaMissing              RejectionReason = "quota_missing"
	RejectionNoTariff                  RejectionReason = "no_tariff"
)

// ChargeResult — результат попытки списания через Charger.
type ChargeResult struct {
	Outcome   ChargeOutcome
	Rejection RejectionReason // только при Outcome=Rejected
	Err       error           // только при Outcome=Transient

	// Чеки закоммиченного списания (нулевые UUID при не-Committed исходах).
	TransactionID  uuid.UUID // direct-списание
	SubAccountTxID uuid.UUID // dual: транзакция субаккаунта
	AggregatorTxID uuid.UUID // dual: транзакция агрегатора
	MarginLogID    uuid.UUID // dual: запись aggregator_margin_log
}

// Charger — единый порт списания за Message. Идемпотентность каждого
// метода — на commit_idempotency_guard (ClaimTx внутри той же транзакции,
// что и сдвиги балансов): повторный вызов с тем же MessageID даёт
// AlreadyCommitted, а не второе списание.
type Charger interface {
	// ChargeDirect списывает amount с одного клиента (direct, без реселлера).
	ChargeDirect(ctx context.Context, clientID, messageID uuid.UUID, amount, currency, description string, segmentCount int32) ChargeResult
	// ChargeDual атомарно списывает с субаккаунта и агрегатора
	// (pool/overage/split) в одной транзакции.
	ChargeDual(ctx context.Context, in ChargeDualInput) ChargeResult
}

// ChargeDualInput — параметры двойного списания; зеркалирует
// application.ChargeMessageDualInput, принадлежность — домену биллинга.
type ChargeDualInput struct {
	MessageID       uuid.UUID
	SubAccountID    uuid.UUID
	AggregatorID    uuid.UUID
	OperatorID      uuid.UUID
	SubAccountPrice string // за сегмент
	AggregatorPrice string // за сегмент (эффективная для charge_mode)
	SubAccountTotal string // итог с субаккаунта
	AggregatorTotal string // итог с агрегатора
	Currency        string
	SegmentCount    int
	PoolSegments    int
	OverageSegments int
	ChargeMode      string // "pool" | "overage" | "split"
}

// FormatTxID рендерит чек-UUID для прото/логов; нулевой UUID — пустая строка.
func FormatTxID(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}
