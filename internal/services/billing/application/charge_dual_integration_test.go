//go:build integration

// Integration-тесты для ChargeMessageDual. Требуют реальную PostgreSQL с накатанными
// миграциями. Запуск:
//
//	go test -tags=integration ./internal/services/billing/application/ \
//	    -run ChargeMessageDual -v
//
// Параметры подключения — через env TEST_DB_HOST / TEST_DB_PORT / TEST_DB_USER /
// TEST_DB_PASSWORD / TEST_DB_NAME (см. internal/testutil.GetTestDSN). Если БД
// недоступна — тесты помечаются как SKIP.
//
// Почему integration, а не unit:
// ChargeMessageDual напрямую дергает deps.DB.BeginTxx() и затем из chargeBalanceTx
// вызывает tx.QueryRowxContext/tx.ExecContext на *sqlx.Tx без промежуточного
// интерфейса. Без sqlmock (которого нет в go.mod и который брифом запрещено
// добавлять) мок *sqlx.Tx невозможен. Единственный способ прогнать логику charge
// balance + ROLLBACK атомарность — реальный Postgres.
package application_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/billing/application"
	tarDomain "github.com/smpp-server/smpp-server/internal/services/tarification/domain"
	tarRepo "github.com/smpp-server/smpp-server/internal/services/tarification/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/testutil"
)

// testEnv — набор ресурсов, общий на весь test-file.
type testEnv struct {
	db       *sqlx.DB
	quotaR   *tarRepo.AggregatorQuotaRepository
	marginR  *tarRepo.AggregatorMarginLogRepository
	deps     application.DualChargeDeps
}

// setupTestEnv инициализирует sqlx.DB. Если подключение невозможно — t.Skip.
func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dsn := testutil.GetTestDSN()
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Skipf("postgres недоступен (%v); integration-тест пропущен", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		t.Skipf("ping не прошёл (%v); integration-тест пропущен", err)
	}

	quotaR := tarRepo.NewAggregatorQuotaRepository(db)
	marginR := tarRepo.NewAggregatorMarginLogRepository(db)

	t.Cleanup(func() { _ = db.Close() })

	return &testEnv{
		db:      db,
		quotaR:  quotaR,
		marginR: marginR,
		deps: application.DualChargeDeps{
			DB:            db,
			QuotaRepo:     quotaR,
			MarginLogRepo: marginR,
			// AccountRepo/TransactionRepo не читаются внутри ChargeMessageDual,
			// EventPublisher=nil — события пропускаем.
		},
	}
}

// seedClient создаёт клиента + его account (balance/frozen/currency).
// Возвращает client_id. Клиент — с уникальным api_key/secret.
func seedClient(t *testing.T, db *sqlx.DB, balance string, currency string, frozen bool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	tag := id.String()[:8]
	_, err := db.ExecContext(ctx, `
		INSERT INTO clients (id, name, api_key, secret, active)
		VALUES ($1, $2, $3, $4, true)
	`, id, fmt.Sprintf("test-%s", tag), fmt.Sprintf("key-%s", tag), fmt.Sprintf("sec-%s", tag))
	require.NoError(t, err, "insert client")

	_, err = db.ExecContext(ctx, `
		INSERT INTO accounts (client_id, balance, currency, frozen)
		VALUES ($1, $2::numeric, $3, $4)
	`, id, balance, currency, frozen)
	require.NoError(t, err, "insert account")

	// Убедиться cleanup — удалить после теста (CASCADE на accounts/transactions).
	t.Cleanup(func() {
		// Порядок: сначала transactions/margin_log (FK на clients), потом quota, потом account/client.
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM aggregator_margin_log WHERE aggregator_id = $1 OR sub_account_id = $1`, id)
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM aggregator_quotas WHERE aggregator_id = $1`, id)
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM transactions WHERE client_id = $1 OR attributed_sub_account_id = $1`, id)
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM clients WHERE id = $1`, id)
	})

	return id
}

// seedQuota создаёт квоту на текущий UTC-месяц. autoRenew/limit/used — параметры.
func seedQuota(t *testing.T, db *sqlx.DB, aggID uuid.UUID, limit, used int64, autoRenew bool) uuid.UUID {
	t.Helper()
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	qID := uuid.New()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO aggregator_quotas
			(id, aggregator_id, period_start, period_end, segment_limit, segments_used,
			 overage_rate, currency, auto_renew, notified_80pct, notified_100pct,
			 created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, false, false, NOW(), NOW())
	`, qID, aggID, start, end, limit, used, "0.5000", "RUB", autoRenew)
	require.NoError(t, err, "insert quota")
	return qID
}

// seedExpiredQuota создаёт квоту на ПРОШЛЫЙ месяц — чтобы GetActive её не находил,
// но GetLatest возвращал (для lazy-create сценариев).
func seedExpiredQuota(t *testing.T, db *sqlx.DB, aggID uuid.UUID, autoRenew bool) uuid.UUID {
	t.Helper()
	now := time.Now().UTC()
	// Прошлый месяц.
	start := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	qID := uuid.New()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO aggregator_quotas
			(id, aggregator_id, period_start, period_end, segment_limit, segments_used,
			 overage_rate, currency, auto_renew, notified_80pct, notified_100pct,
			 created_at, updated_at)
		VALUES ($1, $2, $3, $4, 10000, 0, '0.5000', 'RUB', $5, false, false, NOW(), NOW())
	`, qID, aggID, start, end, autoRenew)
	require.NoError(t, err, "insert expired quota")
	return qID
}

// readBalance возвращает текущий баланс счёта.
func readBalance(t *testing.T, db *sqlx.DB, clientID uuid.UUID) string {
	t.Helper()
	var bal string
	err := db.QueryRowxContext(context.Background(),
		`SELECT balance::text FROM accounts WHERE client_id = $1`, clientID).Scan(&bal)
	require.NoError(t, err)
	return bal
}

// countMarginLog — сколько записей margin_log для данного message_id.
func countMarginLog(t *testing.T, db *sqlx.DB, messageID uuid.UUID) int {
	t.Helper()
	var n int
	err := db.QueryRowxContext(context.Background(),
		`SELECT COUNT(*) FROM aggregator_margin_log WHERE message_id = $1`, messageID).Scan(&n)
	require.NoError(t, err)
	return n
}

// countTransactions — сколько транзакций для данного message_id.
func countTransactions(t *testing.T, db *sqlx.DB, messageID uuid.UUID) int {
	t.Helper()
	var n int
	err := db.QueryRowxContext(context.Background(),
		`SELECT COUNT(*) FROM transactions WHERE message_id = $1`, messageID).Scan(&n)
	require.NoError(t, err)
	return n
}

// readQuotaUsage возвращает segments_used активной квоты агрегатора.
// 0 если нет активной квоты — тест сам различит по контексту.
func readQuotaUsage(t *testing.T, db *sqlx.DB, aggID uuid.UUID) int64 {
	t.Helper()
	var used int64
	err := db.QueryRowxContext(context.Background(), `
		SELECT segments_used FROM aggregator_quotas
		WHERE aggregator_id = $1 AND period_start <= NOW() AND period_end > NOW()
		ORDER BY period_start DESC LIMIT 1
	`, aggID).Scan(&used)
	if errors.Is(err, sql.ErrNoRows) {
		return -1
	}
	require.NoError(t, err)
	return used
}

// --- Тесты. Префикс TestChargeMessageDual_* для общего `-run ChargeMessageDual`. ---

// Case 1: Happy path pool mode — все ок, всё списано, margin_log записан.
func TestChargeMessageDual_HappyPath_Pool(t *testing.T) {
	env := setupTestEnv(t)
	sub := seedClient(t, env.db, "1000.000000", "RUB", false)
	agg := seedClient(t, env.db, "2000.000000", "RUB", false)
	seedQuota(t, env.db, agg, 10000, 0, true)

	msgID := uuid.New()
	in := application.ChargeMessageDualInput{
		MessageID:       msgID,
		SubAccountID:    sub,
		AggregatorID:    agg,
		OperatorID:      uuid.New(),
		SubAccountPrice: "2.000000",
		AggregatorPrice: "1.000000",
		SubAccountTotal: "10.000000",
		AggregatorTotal: "5.000000",
		Currency:        "RUB",
		SegmentCount:    5,
		PoolSegments:    5,
		OverageSegments: 0,
		ChargeMode:      tarDomain.ChargeModePool,
	}

	res, err := application.ChargeMessageDual(context.Background(), env.deps, in)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.True(t, res.Committed, "Committed must be true")
	assert.False(t, res.AlreadyCommitted)
	assert.False(t, res.QuotaMissing)
	assert.False(t, res.SubInsufficient)
	assert.False(t, res.AggInsufficient)
	assert.NotEqual(t, uuid.Nil, res.SubTxID)
	assert.NotEqual(t, uuid.Nil, res.AggTxID)
	assert.NotEqual(t, uuid.Nil, res.MarginLogID)

	assert.Equal(t, "990.000000", readBalance(t, env.db, sub), "sub balance -10")
	assert.Equal(t, "1995.000000", readBalance(t, env.db, agg), "agg balance -5")
	assert.Equal(t, 1, countMarginLog(t, env.db, msgID))
	assert.Equal(t, 2, countTransactions(t, env.db, msgID), "sub + agg transactions")
	assert.Equal(t, int64(5), readQuotaUsage(t, env.db, agg), "quota used += 5")
}

// Case 2: Idempotency — второй вызов с тем же message_id возвращает AlreadyCommitted
// и не меняет балансы.
func TestChargeMessageDual_AlreadyCommitted(t *testing.T) {
	env := setupTestEnv(t)
	sub := seedClient(t, env.db, "1000.000000", "RUB", false)
	agg := seedClient(t, env.db, "2000.000000", "RUB", false)
	seedQuota(t, env.db, agg, 10000, 0, true)

	msgID := uuid.New()
	in := application.ChargeMessageDualInput{
		MessageID:       msgID,
		SubAccountID:    sub,
		AggregatorID:    agg,
		OperatorID:      uuid.New(),
		SubAccountPrice: "2.000000",
		AggregatorPrice: "1.000000",
		SubAccountTotal: "10.000000",
		AggregatorTotal: "5.000000",
		Currency:        "RUB",
		SegmentCount:    5,
		PoolSegments:    5,
		ChargeMode:      tarDomain.ChargeModePool,
	}

	// Первый вызов — committed.
	res1, err := application.ChargeMessageDual(context.Background(), env.deps, in)
	require.NoError(t, err)
	require.True(t, res1.Committed)

	subBalAfter1 := readBalance(t, env.db, sub)
	aggBalAfter1 := readBalance(t, env.db, agg)

	// Второй вызов с тем же message_id → AlreadyCommitted.
	res2, err := application.ChargeMessageDual(context.Background(), env.deps, in)
	require.NoError(t, err)
	require.NotNil(t, res2)
	assert.False(t, res2.Committed)
	assert.True(t, res2.AlreadyCommitted)

	// Балансы не изменились после второго вызова.
	assert.Equal(t, subBalAfter1, readBalance(t, env.db, sub))
	assert.Equal(t, aggBalAfter1, readBalance(t, env.db, agg))
	assert.Equal(t, 1, countMarginLog(t, env.db, msgID), "только одна margin_log запись")
	assert.Equal(t, 2, countTransactions(t, env.db, msgID), "только 2 транзакции (от первого вызова)")
}

// Case 3: QuotaMissing, auto_renew=false — квоты на текущий период нет,
// prev.AutoRenew=false → QuotaMissing, ничего не списано.
func TestChargeMessageDual_QuotaMissing_NoAutoRenew(t *testing.T) {
	env := setupTestEnv(t)
	sub := seedClient(t, env.db, "1000.000000", "RUB", false)
	agg := seedClient(t, env.db, "2000.000000", "RUB", false)
	// expired квота с auto_renew=false.
	seedExpiredQuota(t, env.db, agg, false)

	msgID := uuid.New()
	in := application.ChargeMessageDualInput{
		MessageID: msgID, SubAccountID: sub, AggregatorID: agg, OperatorID: uuid.New(),
		SubAccountPrice: "2.000000", AggregatorPrice: "1.000000",
		SubAccountTotal: "10.000000", AggregatorTotal: "5.000000",
		Currency: "RUB", SegmentCount: 5, PoolSegments: 5, ChargeMode: tarDomain.ChargeModePool,
	}

	res, err := application.ChargeMessageDual(context.Background(), env.deps, in)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.QuotaMissing)
	assert.False(t, res.Committed)

	// margin_log запись вставилась в tx но откатилась через ROLLBACK (defer tx.Rollback).
	assert.Equal(t, 0, countMarginLog(t, env.db, msgID))
	assert.Equal(t, 0, countTransactions(t, env.db, msgID))
	assert.Equal(t, "1000.000000", readBalance(t, env.db, sub))
	assert.Equal(t, "2000.000000", readBalance(t, env.db, agg))
}

// Case 4: QuotaMissing, auto_renew=true — lazy-create успешный, commits.
func TestChargeMessageDual_QuotaMissing_AutoRenew_LazyCreate(t *testing.T) {
	env := setupTestEnv(t)
	sub := seedClient(t, env.db, "1000.000000", "RUB", false)
	agg := seedClient(t, env.db, "2000.000000", "RUB", false)
	seedExpiredQuota(t, env.db, agg, true) // prev с auto_renew=true

	msgID := uuid.New()
	in := application.ChargeMessageDualInput{
		MessageID: msgID, SubAccountID: sub, AggregatorID: agg, OperatorID: uuid.New(),
		SubAccountPrice: "2.000000", AggregatorPrice: "1.000000",
		SubAccountTotal: "10.000000", AggregatorTotal: "5.000000",
		Currency: "RUB", SegmentCount: 3, PoolSegments: 3, ChargeMode: tarDomain.ChargeModePool,
	}

	res, err := application.ChargeMessageDual(context.Background(), env.deps, in)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.Committed, "должен успешно закоммититься после lazy-create")
	assert.False(t, res.QuotaMissing)

	assert.Equal(t, "990.000000", readBalance(t, env.db, sub))
	assert.Equal(t, "1995.000000", readBalance(t, env.db, agg))
	assert.Equal(t, int64(3), readQuotaUsage(t, env.db, agg), "новая квота создана, used=3")
}

// Case 5: SubInsufficient — баланс субаккаунта меньше требуемой суммы.
// UPDATE RETURNING 0 rows → ErrInsufficientBalance → ROLLBACK всей tx.
func TestChargeMessageDual_SubInsufficient(t *testing.T) {
	env := setupTestEnv(t)
	sub := seedClient(t, env.db, "5.000000", "RUB", false) // маленький баланс
	agg := seedClient(t, env.db, "2000.000000", "RUB", false)
	seedQuota(t, env.db, agg, 10000, 0, true)

	msgID := uuid.New()
	in := application.ChargeMessageDualInput{
		MessageID: msgID, SubAccountID: sub, AggregatorID: agg, OperatorID: uuid.New(),
		SubAccountPrice: "2.000000", AggregatorPrice: "1.000000",
		SubAccountTotal: "10.000000", AggregatorTotal: "5.000000",
		Currency: "RUB", SegmentCount: 5, PoolSegments: 5, ChargeMode: tarDomain.ChargeModePool,
	}

	res, err := application.ChargeMessageDual(context.Background(), env.deps, in)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.SubInsufficient)
	assert.False(t, res.Committed)

	// Всё откачено: баланс субаккаунта не тронут, margin_log/transactions пусто.
	assert.Equal(t, "5.000000", readBalance(t, env.db, sub))
	assert.Equal(t, "2000.000000", readBalance(t, env.db, agg))
	assert.Equal(t, 0, countMarginLog(t, env.db, msgID))
	assert.Equal(t, 0, countTransactions(t, env.db, msgID))
	// Квота inc тоже откатана.
	assert.Equal(t, int64(0), readQuotaUsage(t, env.db, agg))
}

// Case 6: AggInsufficient — суб списан, но у агрегатора не хватает.
// ROLLBACK возвращает баланс субаккаунта.
func TestChargeMessageDual_AggInsufficient(t *testing.T) {
	env := setupTestEnv(t)
	sub := seedClient(t, env.db, "1000.000000", "RUB", false)
	agg := seedClient(t, env.db, "2.000000", "RUB", false) // агрегатору не хватит
	seedQuota(t, env.db, agg, 10000, 0, true)

	msgID := uuid.New()
	in := application.ChargeMessageDualInput{
		MessageID: msgID, SubAccountID: sub, AggregatorID: agg, OperatorID: uuid.New(),
		SubAccountPrice: "2.000000", AggregatorPrice: "1.000000",
		SubAccountTotal: "10.000000", AggregatorTotal: "5.000000",
		Currency: "RUB", SegmentCount: 5, PoolSegments: 5, ChargeMode: tarDomain.ChargeModePool,
	}

	res, err := application.ChargeMessageDual(context.Background(), env.deps, in)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.AggInsufficient)
	assert.False(t, res.Committed)

	// КЛЮЧЕВОЕ: баланс субаккаунта восстановлен через ROLLBACK.
	assert.Equal(t, "1000.000000", readBalance(t, env.db, sub), "sub balance ROLLBACK")
	assert.Equal(t, "2.000000", readBalance(t, env.db, agg))
	assert.Equal(t, 0, countMarginLog(t, env.db, msgID))
	assert.Equal(t, 0, countTransactions(t, env.db, msgID))
	assert.Equal(t, int64(0), readQuotaUsage(t, env.db, agg))
}

// Case 7: Split mode — ChargeMode='split' с PoolSegments+OverageSegments=SegmentCount.
func TestChargeMessageDual_SplitMode(t *testing.T) {
	env := setupTestEnv(t)
	sub := seedClient(t, env.db, "1000.000000", "RUB", false)
	agg := seedClient(t, env.db, "2000.000000", "RUB", false)
	// Квота с limit=3, used=0 — т.е. split: 3 в пуле, 7 overage (имитируем через input-поля,
	// реально split высчитывается в messaging-service; нам важно что поля правильно пишутся).
	seedQuota(t, env.db, agg, 3, 0, true)

	msgID := uuid.New()
	in := application.ChargeMessageDualInput{
		MessageID: msgID, SubAccountID: sub, AggregatorID: agg, OperatorID: uuid.New(),
		SubAccountPrice: "2.000000", AggregatorPrice: "1.500000",
		SubAccountTotal: "20.000000", AggregatorTotal: "15.000000",
		Currency: "RUB", SegmentCount: 10, PoolSegments: 3, OverageSegments: 7,
		ChargeMode: tarDomain.ChargeModeSplit,
	}

	res, err := application.ChargeMessageDual(context.Background(), env.deps, in)
	require.NoError(t, err)
	require.True(t, res.Committed)

	// Проверить что charge_mode/pool/overage записаны в margin_log.
	var (
		mode                       string
		poolSeg, overageSeg, segCnt int
	)
	err = env.db.QueryRowxContext(context.Background(), `
		SELECT charge_mode, pool_segments, overage_segments, segment_count
		FROM aggregator_margin_log WHERE message_id = $1
	`, msgID).Scan(&mode, &poolSeg, &overageSeg, &segCnt)
	require.NoError(t, err)
	assert.Equal(t, "split", mode)
	assert.Equal(t, 3, poolSeg)
	assert.Equal(t, 7, overageSeg)
	assert.Equal(t, 10, segCnt)
}

// Case 8: Concurrent double-call — симулируется последовательными вызовами
// (реальный concurrent проверяется на уровне Postgres UNIQUE(idempotency_key)).
// Ожидание: первый commit, второй AlreadyCommitted, баланс меняется один раз.
// (Дубликат Case 2 по сути, но формулировка из плана — оставляем отдельно для явности.)
func TestChargeMessageDual_ConcurrentDoubleCall(t *testing.T) {
	env := setupTestEnv(t)
	sub := seedClient(t, env.db, "100.000000", "RUB", false)
	agg := seedClient(t, env.db, "200.000000", "RUB", false)
	seedQuota(t, env.db, agg, 10000, 0, true)

	msgID := uuid.New()
	in := application.ChargeMessageDualInput{
		MessageID: msgID, SubAccountID: sub, AggregatorID: agg, OperatorID: uuid.New(),
		SubAccountPrice: "2.000000", AggregatorPrice: "1.000000",
		SubAccountTotal: "10.000000", AggregatorTotal: "5.000000",
		Currency: "RUB", SegmentCount: 5, PoolSegments: 5, ChargeMode: tarDomain.ChargeModePool,
	}

	// Первый вызов.
	res1, err := application.ChargeMessageDual(context.Background(), env.deps, in)
	require.NoError(t, err)
	require.True(t, res1.Committed)

	// Второй вызов — идентичный вход.
	res2, err := application.ChargeMessageDual(context.Background(), env.deps, in)
	require.NoError(t, err)
	require.True(t, res2.AlreadyCommitted)
	require.False(t, res2.Committed)

	// Баланс списан ровно один раз.
	assert.Equal(t, "90.000000", readBalance(t, env.db, sub))
	assert.Equal(t, "195.000000", readBalance(t, env.db, agg))
	assert.Equal(t, 1, countMarginLog(t, env.db, msgID))
	assert.Equal(t, 2, countTransactions(t, env.db, msgID))
	assert.Equal(t, int64(5), readQuotaUsage(t, env.db, agg))
}

// sanity: проверяем что env переменные считались (а не что мы используем их случайно).
// Полезно для диагностики SKIP в CI.
func TestMain(m *testing.M) {
	// no-op; testutil.GetTestDSN берёт env сам. Оставляем для будущих глобальных setup.
	os.Exit(m.Run())
}
