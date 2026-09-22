//go:build functional

package functional_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/billing/application"
	billingDomain "github.com/smpp-server/smpp-server/internal/services/billing/domain"
	billingRepo "github.com/smpp-server/smpp-server/internal/services/billing/infrastructure/repository"
	billingMocks "github.com/smpp-server/smpp-server/internal/services/billing/mocks"

	"github.com/jmoiron/sqlx"
)

func TestBillingChain(t *testing.T) {
	skipIfNoDB(t)
	db := setupTestDB(t)

	t.Run("ChargeAccount", func(t *testing.T) {
		ctx := context.Background()
		clientID := uuid.New()

		// Insert a test client
		_, err := db.ExecContext(ctx,
			`INSERT INTO clients (id, name, api_key, secret)
			 VALUES ($1, 'test-charge', $1, 'secret')
			 ON CONFLICT DO NOTHING`, clientID.String())
		require.NoError(t, err)

		transactionRepo := billingRepo.NewTransactionRepository(db)

		// Create an account with balance 100.00 (see seedAccount note)
		_ = seedAccount(t, db, clientID, "100.000000")

		t.Cleanup(func() {
			_, _ = db.ExecContext(context.Background(),
				`DELETE FROM transactions WHERE client_id = $1`, clientID)
			_, _ = db.ExecContext(context.Background(),
				`DELETE FROM accounts WHERE client_id = $1`, clientID)
			_, _ = db.ExecContext(context.Background(),
				`DELETE FROM clients WHERE id = $1`, clientID)
		})

		accountRepo := billingRepo.NewAccountRepository(db)
		mockPublisher := &billingMocks.MockEventPublisher{}
		mockPublisher.On("PublishBalanceChanged", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockPublisher.On("PublishTransactionCompleted", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		svc := application.NewBillingService(accountRepo, transactionRepo, mockPublisher)

		// DeductCredits with 10.50
		tx, err := svc.DeductCredits(ctx, clientID, "10.500000", "RUB", "test charge")
		require.NoError(t, err)
		require.NotNil(t, tx)

		// Verify balance reduced
		acct, err := accountRepo.GetByClientID(ctx, clientID)
		require.NoError(t, err)
		assert.Equal(t, "89.500000", acct.Balance)

		// Verify transaction record
		fetchedTx, err := transactionRepo.GetByID(ctx, tx.ID)
		require.NoError(t, err)
		assert.Equal(t, billingDomain.TransactionTypeCharge, fetchedTx.Type)
		assert.Equal(t, "10.500000", fetchedTx.Amount)
		assert.Equal(t, "100.000000", fetchedTx.BalanceBefore)
		assert.Equal(t, "89.500000", fetchedTx.BalanceAfter)
		assert.Equal(t, "test charge", fetchedTx.Description)

		// Verify events were published
		mockPublisher.AssertCalled(t, "PublishBalanceChanged",
			mock.Anything, clientID.String(), "89.500000", "RUB")
	})

	t.Run("InsufficientFunds", func(t *testing.T) {
		ctx := context.Background()
		clientID := uuid.New()

		_, err := db.ExecContext(ctx,
			`INSERT INTO clients (id, name, api_key, secret)
			 VALUES ($1, 'test-insuf', $1, 'secret')
			 ON CONFLICT DO NOTHING`, clientID.String())
		require.NoError(t, err)

		transactionRepo := billingRepo.NewTransactionRepository(db)

		// Create account with balance 0.10 (see seedAccount note)
		_ = seedAccount(t, db, clientID, "0.100000")

		t.Cleanup(func() {
			_, _ = db.ExecContext(context.Background(),
				`DELETE FROM transactions WHERE client_id = $1`, clientID)
			_, _ = db.ExecContext(context.Background(),
				`DELETE FROM accounts WHERE client_id = $1`, clientID)
			_, _ = db.ExecContext(context.Background(),
				`DELETE FROM clients WHERE id = $1`, clientID)
		})

		accountRepo := billingRepo.NewAccountRepository(db)
		mockPublisher := &billingMocks.MockEventPublisher{}
		svc := application.NewBillingService(accountRepo, transactionRepo, mockPublisher)

		// Attempt to deduct 0.50 -- should fail with insufficient balance
		_, err = svc.DeductCredits(ctx, clientID, "0.500000", "RUB", "should fail")
		require.Error(t, err)
		assert.ErrorIs(t, err, billingDomain.ErrInsufficientBalance)

		// Verify balance unchanged
		acct, err := accountRepo.GetByClientID(ctx, clientID)
		require.NoError(t, err)
		assert.Equal(t, "0.100000", acct.Balance)

		// Verify no transaction was created
		txns, err := transactionRepo.GetByClientID(ctx, clientID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, txns, 0)
	})

	t.Run("RefundTransaction", func(t *testing.T) {
		ctx := context.Background()
		clientID := uuid.New()

		_, err := db.ExecContext(ctx,
			`INSERT INTO clients (id, name, api_key, secret)
			 VALUES ($1, 'test-refund', $1, 'secret')
			 ON CONFLICT DO NOTHING`, clientID.String())
		require.NoError(t, err)

		accountRepo := billingRepo.NewAccountRepository(db)
		transactionRepo := billingRepo.NewTransactionRepository(db)

		// Create account with balance 100.00 (see seedAccount note)
		_ = seedAccount(t, db, clientID, "100.000000")

		t.Cleanup(func() {
			_, _ = db.ExecContext(context.Background(),
				`DELETE FROM transactions WHERE client_id = $1`, clientID)
			_, _ = db.ExecContext(context.Background(),
				`DELETE FROM accounts WHERE client_id = $1`, clientID)
			_, _ = db.ExecContext(context.Background(),
				`DELETE FROM clients WHERE id = $1`, clientID)
		})

		mockPublisher := &billingMocks.MockEventPublisher{}
		mockPublisher.On("PublishBalanceChanged", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockPublisher.On("PublishTransactionCompleted", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		svc := application.NewBillingService(accountRepo, transactionRepo, mockPublisher)

		// Step 1: Charge 25.00
		chargeTx, err := svc.DeductCredits(ctx, clientID, "25.000000", "RUB", "initial charge")
		require.NoError(t, err)
		require.NotNil(t, chargeTx)

		// Verify balance after charge
		acct, err := accountRepo.GetByClientID(ctx, clientID)
		require.NoError(t, err)
		assert.Equal(t, "75.000000", acct.Balance)

		// Step 2: Add credits (refund) of 25.00
		refundTx, err := svc.AddCredits(ctx, clientID, "25.000000", "RUB", "refund", nil)
		require.NoError(t, err)
		require.NotNil(t, refundTx)

		// Verify balance restored
		acct, err = accountRepo.GetByClientID(ctx, clientID)
		require.NoError(t, err)
		assert.Equal(t, "100.000000", acct.Balance)

		// Verify refund transaction record
		fetchedRefund, err := transactionRepo.GetByID(ctx, refundTx.ID)
		require.NoError(t, err)
		assert.Equal(t, billingDomain.TransactionTypeCredit, fetchedRefund.Type)
		assert.Equal(t, "25.000000", fetchedRefund.Amount)
		assert.Equal(t, "75.000000", fetchedRefund.BalanceBefore)
		assert.Equal(t, "100.000000", fetchedRefund.BalanceAfter)

		// Verify 2 total transactions
		txns, err := transactionRepo.GetByClientID(ctx, clientID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, txns, 2)
	})
}

// seedAccount вставляет компанию + аккаунт напрямую: accounts.company_id
// NOT NULL с миграции компаний, а AccountRepository.Create колонку не пишет
// (известный дрейф — см. PR кандидата 5).
func seedAccount(t *testing.T, db *sqlx.DB, clientID uuid.UUID, balance string) *billingDomain.Account {
	t.Helper()
	ctx := context.Background()
	companyID := uuid.New()
	_, err := db.ExecContext(ctx, `
		INSERT INTO companies (id, name, is_offer, active, created_at, updated_at)
		VALUES ($1, 'test-company', false, true, NOW(), NOW())`, companyID)
	require.NoError(t, err)
	// Триггер trg_create_account_for_new_client уже создал аккаунт клиенту —
	// дополняем его company_id и балансом вместо второго INSERT (UNIQUE client_id).
	result, err := db.ExecContext(ctx, `
		UPDATE accounts SET company_id = $2, balance = $3 WHERE client_id = $1`,
		clientID, companyID, balance)
	require.NoError(t, err)
	rows, err := result.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), rows, "trigger-created account expected")

	account := billingDomain.NewAccount(clientID, "RUB")
	account.Balance = balance
	return account
}
