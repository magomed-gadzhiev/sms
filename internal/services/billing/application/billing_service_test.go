package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/billing/domain"
	"github.com/smpp-server/smpp-server/internal/services/billing/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newTestBillingService создает BillingService с моками для тестов.
func newTestBillingService() (
	*BillingService,
	*mocks.MockAccountRepository,
	*mocks.MockTransactionRepository,
	*mocks.MockTransferRepository,
	*mocks.MockEventPublisher,
) {
	accountRepo := new(mocks.MockAccountRepository)
	transactionRepo := new(mocks.MockTransactionRepository)
	transferRepo := new(mocks.MockTransferRepository)
	eventPublisher := new(mocks.MockEventPublisher)

	svc := NewBillingService(accountRepo, transactionRepo, eventPublisher)
	svc.SetTransferRepo(transferRepo)

	return svc, accountRepo, transactionRepo, transferRepo, eventPublisher
}

func TestBillingService(t *testing.T) {
	t.Run("DeductCredits", func(t *testing.T) {
		t.Run("sufficient_balance", func(t *testing.T) {
			svc, accountRepo, transactionRepo, _, eventPublisher := newTestBillingService()
			ctx := context.Background()
			clientID := uuid.New()

			account := &domain.Account{
				ID:       uuid.New(),
				ClientID: clientID,
				Balance:  "100.000000",
				Currency: "USD",
			}

			accountRepo.On("GetByClientID", ctx, clientID).Return(account, nil)
			accountRepo.On("UpdateBalance", ctx, clientID, "90.000000").Return(nil)
			transactionRepo.On("Create", ctx, mock.AnythingOfType("*domain.Transaction")).Return(nil)
			eventPublisher.On("PublishBalanceChanged", ctx, clientID.String(), "90.000000", "USD").Return(nil)
			eventPublisher.On("PublishTransactionCompleted", ctx, mock.AnythingOfType("string"), clientID.String(), "charge", "10.000000", "USD").Return(nil)

			tx, err := svc.DeductCredits(ctx, clientID, "10.000000", "USD", "SMS charge")

			require.NoError(t, err)
			require.NotNil(t, tx)
			assert.Equal(t, clientID, tx.ClientID)
			assert.Equal(t, domain.TransactionTypeCharge, tx.Type)
			assert.Equal(t, "10.000000", tx.Amount)
			assert.Equal(t, "100.000000", tx.BalanceBefore)
			assert.Equal(t, "90.000000", tx.BalanceAfter)
			assert.Equal(t, "USD", tx.Currency)
			assert.Equal(t, "SMS charge", tx.Description)

			accountRepo.AssertExpectations(t)
			transactionRepo.AssertExpectations(t)
			eventPublisher.AssertExpectations(t)
		})

		t.Run("insufficient_balance", func(t *testing.T) {
			svc, accountRepo, _, _, _ := newTestBillingService()
			ctx := context.Background()
			clientID := uuid.New()

			account := &domain.Account{
				ID:       uuid.New(),
				ClientID: clientID,
				Balance:  "5.000000",
				Currency: "USD",
			}

			accountRepo.On("GetByClientID", ctx, clientID).Return(account, nil)

			tx, err := svc.DeductCredits(ctx, clientID, "10.000000", "USD", "SMS charge")

			require.Error(t, err)
			assert.Nil(t, tx)
			assert.ErrorIs(t, err, domain.ErrInsufficientBalance)

			accountRepo.AssertExpectations(t)
		})

		t.Run("currency_mismatch", func(t *testing.T) {
			svc, accountRepo, _, _, _ := newTestBillingService()
			ctx := context.Background()
			clientID := uuid.New()

			account := &domain.Account{
				ID:       uuid.New(),
				ClientID: clientID,
				Balance:  "100.000000",
				Currency: "USD",
			}

			accountRepo.On("GetByClientID", ctx, clientID).Return(account, nil)

			tx, err := svc.DeductCredits(ctx, clientID, "10.000000", "EUR", "SMS charge")

			require.Error(t, err)
			assert.Nil(t, tx)
			assert.Contains(t, err.Error(), "currency mismatch")

			accountRepo.AssertExpectations(t)
		})

		t.Run("account_not_found", func(t *testing.T) {
			svc, accountRepo, _, _, _ := newTestBillingService()
			ctx := context.Background()
			clientID := uuid.New()

			accountRepo.On("GetByClientID", ctx, clientID).Return(nil, domain.ErrAccountNotFound)

			tx, err := svc.DeductCredits(ctx, clientID, "10.000000", "USD", "SMS charge")

			require.Error(t, err)
			assert.Nil(t, tx)

			accountRepo.AssertExpectations(t)
		})

		t.Run("exact_balance_deduction", func(t *testing.T) {
			svc, accountRepo, transactionRepo, _, eventPublisher := newTestBillingService()
			ctx := context.Background()
			clientID := uuid.New()

			account := &domain.Account{
				ID:       uuid.New(),
				ClientID: clientID,
				Balance:  "10.000000",
				Currency: "USD",
			}

			accountRepo.On("GetByClientID", ctx, clientID).Return(account, nil)
			accountRepo.On("UpdateBalance", ctx, clientID, "0.000000").Return(nil)
			transactionRepo.On("Create", ctx, mock.AnythingOfType("*domain.Transaction")).Return(nil)
			eventPublisher.On("PublishBalanceChanged", ctx, clientID.String(), "0.000000", "USD").Return(nil)
			eventPublisher.On("PublishTransactionCompleted", ctx, mock.AnythingOfType("string"), clientID.String(), "charge", "10.000000", "USD").Return(nil)

			tx, err := svc.DeductCredits(ctx, clientID, "10.000000", "USD", "Full deduction")

			require.NoError(t, err)
			require.NotNil(t, tx)
			assert.Equal(t, "0.000000", tx.BalanceAfter)

			accountRepo.AssertExpectations(t)
			transactionRepo.AssertExpectations(t)
		})
	})

	t.Run("ChargeMessage", func(t *testing.T) {
		t.Run("successful_charge", func(t *testing.T) {
			svc, accountRepo, transactionRepo, _, eventPublisher := newTestBillingService()
			ctx := context.Background()
			clientID := uuid.New()
			messageID := uuid.New()

			account := &domain.Account{
				ID:       uuid.New(),
				ClientID: clientID,
				Balance:  "50.000000",
				Currency: "USD",
			}

			transactionRepo.On("GetByMessageID", ctx, messageID).Return(nil, domain.ErrTransactionNotFound)
			accountRepo.On("GetByClientID", ctx, clientID).Return(account, nil)
			accountRepo.On("UpdateBalance", ctx, clientID, "49.990000").Return(nil)
			transactionRepo.On("Create", ctx, mock.AnythingOfType("*domain.Transaction")).Return(nil)
			eventPublisher.On("PublishBalanceChanged", ctx, clientID.String(), "49.990000", "USD").Return(nil)
			eventPublisher.On("PublishTransactionCompleted", ctx, mock.AnythingOfType("string"), clientID.String(), "charge", "0.010000", "USD").Return(nil)

			tx, err := svc.ChargeMessage(ctx, clientID, messageID, "0.010000", "USD", "Message charge")

			require.NoError(t, err)
			require.NotNil(t, tx)
			assert.Equal(t, domain.TransactionTypeCharge, tx.Type)
			assert.Equal(t, "0.010000", tx.Amount)
			require.NotNil(t, tx.MessageID)
			assert.Equal(t, messageID, *tx.MessageID)

			accountRepo.AssertExpectations(t)
			transactionRepo.AssertExpectations(t)
			eventPublisher.AssertExpectations(t)
		})

		t.Run("idempotent_existing_transaction", func(t *testing.T) {
			svc, _, transactionRepo, _, _ := newTestBillingService()
			ctx := context.Background()
			clientID := uuid.New()
			messageID := uuid.New()

			existingTx := &domain.Transaction{
				ID:       uuid.New(),
				ClientID: clientID,
				Type:     domain.TransactionTypeCharge,
				Amount:   "0.010000",
				Currency: "USD",
			}

			transactionRepo.On("GetByMessageID", ctx, messageID).Return(existingTx, nil)

			tx, err := svc.ChargeMessage(ctx, clientID, messageID, "0.010000", "USD", "Message charge")

			require.NoError(t, err)
			assert.Equal(t, existingTx.ID, tx.ID)

			transactionRepo.AssertExpectations(t)
		})

		t.Run("insufficient_balance", func(t *testing.T) {
			svc, accountRepo, transactionRepo, _, _ := newTestBillingService()
			ctx := context.Background()
			clientID := uuid.New()
			messageID := uuid.New()

			account := &domain.Account{
				ID:       uuid.New(),
				ClientID: clientID,
				Balance:  "0.005000",
				Currency: "USD",
			}

			transactionRepo.On("GetByMessageID", ctx, messageID).Return(nil, domain.ErrTransactionNotFound)
			accountRepo.On("GetByClientID", ctx, clientID).Return(account, nil)

			tx, err := svc.ChargeMessage(ctx, clientID, messageID, "0.010000", "USD", "Message charge")

			require.Error(t, err)
			assert.Nil(t, tx)
			assert.ErrorIs(t, err, domain.ErrInsufficientBalance)

			accountRepo.AssertExpectations(t)
			transactionRepo.AssertExpectations(t)
		})
	})

	t.Run("GetBalance", func(t *testing.T) {
		t.Run("existing_account", func(t *testing.T) {
			svc, accountRepo, _, _, _ := newTestBillingService()
			ctx := context.Background()
			clientID := uuid.New()

			account := &domain.Account{
				ID:       uuid.New(),
				ClientID: clientID,
				Balance:  "250.500000",
				Currency: "USD",
			}

			accountRepo.On("GetByClientID", ctx, clientID).Return(account, nil)

			result, err := svc.GetBalance(ctx, clientID)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, clientID, result.ClientID)
			assert.Equal(t, "250.500000", result.Balance)
			assert.Equal(t, "USD", result.Currency)

			accountRepo.AssertExpectations(t)
		})

		t.Run("account_not_found", func(t *testing.T) {
			svc, accountRepo, _, _, _ := newTestBillingService()
			ctx := context.Background()
			clientID := uuid.New()

			accountRepo.On("GetByClientID", ctx, clientID).Return(nil, domain.ErrAccountNotFound)

			result, err := svc.GetBalance(ctx, clientID)

			require.Error(t, err)
			assert.Nil(t, result)

			accountRepo.AssertExpectations(t)
		})
	})

	t.Run("AddCredits", func(t *testing.T) {
		t.Run("existing_account", func(t *testing.T) {
			svc, accountRepo, transactionRepo, _, eventPublisher := newTestBillingService()
			ctx := context.Background()
			clientID := uuid.New()

			account := &domain.Account{
				ID:       uuid.New(),
				ClientID: clientID,
				Balance:  "100.000000",
				Currency: "USD",
			}

			accountRepo.On("GetByClientID", ctx, clientID).Return(account, nil)
			accountRepo.On("UpdateBalance", ctx, clientID, "150.000000").Return(nil)
			transactionRepo.On("Create", ctx, mock.AnythingOfType("*domain.Transaction")).Return(nil)
			eventPublisher.On("PublishBalanceChanged", ctx, clientID.String(), "150.000000", "USD").Return(nil)
			eventPublisher.On("PublishTransactionCompleted", ctx, mock.AnythingOfType("string"), clientID.String(), "credit", "50.000000", "USD").Return(nil)

			paymentMethod := "credit_card"
			tx, err := svc.AddCredits(ctx, clientID, "50.000000", "USD", "Top-up", &paymentMethod)

			require.NoError(t, err)
			require.NotNil(t, tx)
			assert.Equal(t, domain.TransactionTypeCredit, tx.Type)
			assert.Equal(t, "50.000000", tx.Amount)
			assert.Equal(t, "100.000000", tx.BalanceBefore)
			assert.Equal(t, "150.000000", tx.BalanceAfter)
			require.NotNil(t, tx.PaymentMethod)
			assert.Equal(t, "credit_card", *tx.PaymentMethod)

			accountRepo.AssertExpectations(t)
			transactionRepo.AssertExpectations(t)
			eventPublisher.AssertExpectations(t)
		})

		t.Run("new_account_created", func(t *testing.T) {
			svc, accountRepo, transactionRepo, _, eventPublisher := newTestBillingService()
			ctx := context.Background()
			clientID := uuid.New()

			accountRepo.On("GetByClientID", ctx, clientID).Return(nil, domain.ErrAccountNotFound)
			accountRepo.On("Create", ctx, mock.AnythingOfType("*domain.Account")).Return(nil)
			accountRepo.On("UpdateBalance", ctx, clientID, "50.000000").Return(nil)
			transactionRepo.On("Create", ctx, mock.AnythingOfType("*domain.Transaction")).Return(nil)
			eventPublisher.On("PublishBalanceChanged", ctx, clientID.String(), "50.000000", "USD").Return(nil)
			eventPublisher.On("PublishTransactionCompleted", ctx, mock.AnythingOfType("string"), clientID.String(), "credit", "50.000000", "USD").Return(nil)

			tx, err := svc.AddCredits(ctx, clientID, "50.000000", "USD", "Initial top-up", nil)

			require.NoError(t, err)
			require.NotNil(t, tx)
			assert.Equal(t, domain.TransactionTypeCredit, tx.Type)
			assert.Equal(t, "50.000000", tx.Amount)

			accountRepo.AssertExpectations(t)
			transactionRepo.AssertExpectations(t)
			eventPublisher.AssertExpectations(t)
		})

		t.Run("currency_mismatch", func(t *testing.T) {
			svc, accountRepo, _, _, _ := newTestBillingService()
			ctx := context.Background()
			clientID := uuid.New()

			account := &domain.Account{
				ID:       uuid.New(),
				ClientID: clientID,
				Balance:  "100.000000",
				Currency: "USD",
			}

			accountRepo.On("GetByClientID", ctx, clientID).Return(account, nil)

			tx, err := svc.AddCredits(ctx, clientID, "50.000000", "EUR", "Top-up", nil)

			require.Error(t, err)
			assert.Nil(t, tx)
			assert.Contains(t, err.Error(), "currency mismatch")

			accountRepo.AssertExpectations(t)
		})
	})

	t.Run("TransferBalance", func(t *testing.T) {
		t.Run("valid_transfer", func(t *testing.T) {
			svc, accountRepo, transactionRepo, transferRepo, eventPublisher := newTestBillingService()
			ctx := context.Background()
			fromClientID := uuid.New()
			toClientID := uuid.New()

			fromAccount := &domain.Account{
				ID:       uuid.New(),
				ClientID: fromClientID,
				Balance:  "200.000000",
				Currency: "USD",
			}

			toAccount := &domain.Account{
				ID:       uuid.New(),
				ClientID: toClientID,
				Balance:  "50.000000",
				Currency: "USD",
			}

			accountRepo.On("GetByClientID", ctx, fromClientID).Return(fromAccount, nil)
			accountRepo.On("GetByClientID", ctx, toClientID).Return(toAccount, nil)
			accountRepo.On("UpdateBalance", ctx, fromClientID, "150.000000").Return(nil)
			accountRepo.On("UpdateBalance", ctx, toClientID, "100.000000").Return(nil)
			transactionRepo.On("Create", ctx, mock.AnythingOfType("*domain.Transaction")).Return(nil).Twice()
			transferRepo.On("Create", ctx, mock.AnythingOfType("*domain.BalanceTransfer")).Return(nil)
			eventPublisher.On("PublishBalanceChanged", ctx, fromClientID.String(), "150.000000", "USD").Return(nil)
			eventPublisher.On("PublishBalanceChanged", ctx, toClientID.String(), "100.000000", "USD").Return(nil)

			transferID, fromBalance, toBalance, err := svc.TransferBalance(ctx, fromClientID, toClientID, "50.000000", "USD")

			require.NoError(t, err)
			assert.NotEmpty(t, transferID)
			assert.Equal(t, "150.000000", fromBalance)
			assert.Equal(t, "100.000000", toBalance)

			accountRepo.AssertExpectations(t)
			transactionRepo.AssertExpectations(t)
			transferRepo.AssertExpectations(t)
			eventPublisher.AssertExpectations(t)
		})

		t.Run("insufficient_sender_balance", func(t *testing.T) {
			svc, accountRepo, _, _, _ := newTestBillingService()
			ctx := context.Background()
			fromClientID := uuid.New()
			toClientID := uuid.New()

			fromAccount := &domain.Account{
				ID:       uuid.New(),
				ClientID: fromClientID,
				Balance:  "10.000000",
				Currency: "USD",
			}

			accountRepo.On("GetByClientID", ctx, fromClientID).Return(fromAccount, nil)

			_, _, _, err := svc.TransferBalance(ctx, fromClientID, toClientID, "50.000000", "USD")

			require.Error(t, err)
			assert.ErrorIs(t, err, domain.ErrInsufficientBalance)

			accountRepo.AssertExpectations(t)
		})

		t.Run("receiver_account_created_if_not_exists", func(t *testing.T) {
			svc, accountRepo, transactionRepo, transferRepo, eventPublisher := newTestBillingService()
			ctx := context.Background()
			fromClientID := uuid.New()
			toClientID := uuid.New()

			fromAccount := &domain.Account{
				ID:       uuid.New(),
				ClientID: fromClientID,
				Balance:  "100.000000",
				Currency: "USD",
			}

			accountRepo.On("GetByClientID", ctx, fromClientID).Return(fromAccount, nil)
			accountRepo.On("GetByClientID", ctx, toClientID).Return(nil, domain.ErrAccountNotFound)
			accountRepo.On("Create", ctx, mock.AnythingOfType("*domain.Account")).Return(nil)
			accountRepo.On("UpdateBalance", ctx, fromClientID, "70.000000").Return(nil)
			accountRepo.On("UpdateBalance", ctx, toClientID, "30.000000").Return(nil)
			transactionRepo.On("Create", ctx, mock.AnythingOfType("*domain.Transaction")).Return(nil).Twice()
			transferRepo.On("Create", ctx, mock.AnythingOfType("*domain.BalanceTransfer")).Return(nil)
			eventPublisher.On("PublishBalanceChanged", ctx, fromClientID.String(), "70.000000", "USD").Return(nil)
			eventPublisher.On("PublishBalanceChanged", ctx, toClientID.String(), "30.000000", "USD").Return(nil)

			transferID, fromBalance, toBalance, err := svc.TransferBalance(ctx, fromClientID, toClientID, "30.000000", "USD")

			require.NoError(t, err)
			assert.NotEmpty(t, transferID)
			assert.Equal(t, "70.000000", fromBalance)
			assert.Equal(t, "30.000000", toBalance)

			accountRepo.AssertExpectations(t)
			transactionRepo.AssertExpectations(t)
			transferRepo.AssertExpectations(t)
			eventPublisher.AssertExpectations(t)
		})

		t.Run("sender_currency_mismatch", func(t *testing.T) {
			svc, accountRepo, _, _, _ := newTestBillingService()
			ctx := context.Background()
			fromClientID := uuid.New()
			toClientID := uuid.New()

			fromAccount := &domain.Account{
				ID:       uuid.New(),
				ClientID: fromClientID,
				Balance:  "100.000000",
				Currency: "USD",
			}

			accountRepo.On("GetByClientID", ctx, fromClientID).Return(fromAccount, nil)

			_, _, _, err := svc.TransferBalance(ctx, fromClientID, toClientID, "30.000000", "EUR")

			require.Error(t, err)
			assert.Contains(t, err.Error(), "currency mismatch")

			accountRepo.AssertExpectations(t)
		})

		t.Run("receiver_currency_mismatch", func(t *testing.T) {
			svc, accountRepo, _, _, _ := newTestBillingService()
			ctx := context.Background()
			fromClientID := uuid.New()
			toClientID := uuid.New()

			fromAccount := &domain.Account{
				ID:       uuid.New(),
				ClientID: fromClientID,
				Balance:  "100.000000",
				Currency: "USD",
			}

			toAccount := &domain.Account{
				ID:       uuid.New(),
				ClientID: toClientID,
				Balance:  "50.000000",
				Currency: "EUR",
			}

			accountRepo.On("GetByClientID", ctx, fromClientID).Return(fromAccount, nil)
			accountRepo.On("GetByClientID", ctx, toClientID).Return(toAccount, nil)

			_, _, _, err := svc.TransferBalance(ctx, fromClientID, toClientID, "30.000000", "USD")

			require.Error(t, err)
			assert.Contains(t, err.Error(), "currency mismatch")

			accountRepo.AssertExpectations(t)
		})
	})

	t.Run("GetTransactionHistory", func(t *testing.T) {
		t.Run("returns_transactions", func(t *testing.T) {
			svc, _, transactionRepo, _, _ := newTestBillingService()
			ctx := context.Background()
			clientID := uuid.New()

			expectedTxs := []*domain.Transaction{
				{ID: uuid.New(), ClientID: clientID, Type: domain.TransactionTypeCharge, Amount: "0.010000"},
				{ID: uuid.New(), ClientID: clientID, Type: domain.TransactionTypeCredit, Amount: "100.000000"},
			}

			transactionRepo.On("GetByClientID", ctx, clientID, 10, 0).Return(expectedTxs, nil)

			txs, err := svc.GetTransactionHistory(ctx, clientID, 10, 0)

			require.NoError(t, err)
			assert.Len(t, txs, 2)
			assert.Equal(t, expectedTxs[0].ID, txs[0].ID)
			assert.Equal(t, expectedTxs[1].ID, txs[1].ID)

			transactionRepo.AssertExpectations(t)
		})
	})
}
