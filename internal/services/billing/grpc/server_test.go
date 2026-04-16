package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
	"github.com/smpp-server/smpp-server/internal/services/billing/application"
	"github.com/smpp-server/smpp-server/internal/services/billing/domain"
)

// --- Mocks ---

type mockAccountRepo struct {
	mock.Mock
}

func (m *mockAccountRepo) Create(ctx context.Context, account *domain.Account) error {
	args := m.Called(ctx, account)
	return args.Error(0)
}

func (m *mockAccountRepo) GetByClientID(ctx context.Context, clientID uuid.UUID) (*domain.Account, error) {
	args := m.Called(ctx, clientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Account), args.Error(1)
}

func (m *mockAccountRepo) Update(ctx context.Context, account *domain.Account) error {
	args := m.Called(ctx, account)
	return args.Error(0)
}

func (m *mockAccountRepo) UpdateBalance(ctx context.Context, clientID uuid.UUID, newBalance string) error {
	args := m.Called(ctx, clientID, newBalance)
	return args.Error(0)
}

func (m *mockAccountRepo) FreezeAccount(ctx context.Context, clientID uuid.UUID, adminID uuid.UUID) (time.Time, error) {
	args := m.Called(ctx, clientID, adminID)
	return args.Get(0).(time.Time), args.Error(1)
}

func (m *mockAccountRepo) UnfreezeAccount(ctx context.Context, clientID uuid.UUID) error {
	args := m.Called(ctx, clientID)
	return args.Error(0)
}

func (m *mockAccountRepo) SetCreditLimit(ctx context.Context, clientID uuid.UUID, limit string) error {
	args := m.Called(ctx, clientID, limit)
	return args.Error(0)
}

func (m *mockAccountRepo) SetLowBalanceThreshold(ctx context.Context, clientID uuid.UUID, threshold string) error {
	args := m.Called(ctx, clientID, threshold)
	return args.Error(0)
}

func (m *mockAccountRepo) ListBalances(ctx context.Context, search string, status string, belowThreshold bool, limit, offset int32) ([]domain.BalanceInfo, int32, error) {
	args := m.Called(ctx, search, status, belowThreshold, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int32), args.Error(2)
	}
	return args.Get(0).([]domain.BalanceInfo), args.Get(1).(int32), args.Error(2)
}

type mockTransactionRepo struct {
	mock.Mock
}

func (m *mockTransactionRepo) Create(ctx context.Context, transaction *domain.Transaction) error {
	args := m.Called(ctx, transaction)
	return args.Error(0)
}

func (m *mockTransactionRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Transaction), args.Error(1)
}

func (m *mockTransactionRepo) GetAll(ctx context.Context, limit, offset int) ([]*domain.Transaction, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Transaction), args.Error(1)
}

func (m *mockTransactionRepo) GetByClientID(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*domain.Transaction, error) {
	args := m.Called(ctx, clientID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Transaction), args.Error(1)
}

func (m *mockTransactionRepo) GetByClientIDAndType(ctx context.Context, clientID uuid.UUID, transactionType domain.TransactionType, limit, offset int) ([]*domain.Transaction, error) {
	args := m.Called(ctx, clientID, transactionType, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Transaction), args.Error(1)
}

func (m *mockTransactionRepo) GetByClientIDAndPeriod(ctx context.Context, clientID uuid.UUID, from, to time.Time, limit, offset int) ([]*domain.Transaction, error) {
	args := m.Called(ctx, clientID, from, to, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Transaction), args.Error(1)
}

func (m *mockTransactionRepo) GetByMessageID(ctx context.Context, messageID uuid.UUID) (*domain.Transaction, error) {
	args := m.Called(ctx, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Transaction), args.Error(1)
}

type mockBillingEventPublisher struct {
	mock.Mock
}

func (m *mockBillingEventPublisher) PublishBalanceChanged(ctx context.Context, clientID string, balance, currency string) error {
	args := m.Called(ctx, clientID, balance, currency)
	return args.Error(0)
}

func (m *mockBillingEventPublisher) PublishTransactionCompleted(ctx context.Context, transactionID, clientID, transactionType, amount, currency string) error {
	args := m.Called(ctx, transactionID, clientID, transactionType, amount, currency)
	return args.Error(0)
}

func (m *mockBillingEventPublisher) PublishBalanceLow(ctx context.Context, clientID, balance, threshold, currency string) error {
	args := m.Called(ctx, clientID, balance, threshold, currency)
	return args.Error(0)
}

type mockPricingRuleRepo struct {
	mock.Mock
}

func (m *mockPricingRuleRepo) Create(ctx context.Context, rule *domain.PricingRule) error {
	args := m.Called(ctx, rule)
	return args.Error(0)
}

func (m *mockPricingRuleRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.PricingRule, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PricingRule), args.Error(1)
}

func (m *mockPricingRuleRepo) GetByClientID(ctx context.Context, clientID *uuid.UUID, activeOnly bool) ([]*domain.PricingRule, error) {
	args := m.Called(ctx, clientID, activeOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.PricingRule), args.Error(1)
}

func (m *mockPricingRuleRepo) GetGlobalRules(ctx context.Context, activeOnly bool) ([]*domain.PricingRule, error) {
	args := m.Called(ctx, activeOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.PricingRule), args.Error(1)
}

func (m *mockPricingRuleRepo) GetMatchingRule(ctx context.Context, clientID *uuid.UUID, destination string) (*domain.PricingRule, error) {
	args := m.Called(ctx, clientID, destination)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PricingRule), args.Error(1)
}

func (m *mockPricingRuleRepo) Update(ctx context.Context, rule *domain.PricingRule) error {
	args := m.Called(ctx, rule)
	return args.Error(0)
}

func (m *mockPricingRuleRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// --- Helper ---

func newTestBillingServer(
	accountRepo *mockAccountRepo,
	transactionRepo *mockTransactionRepo,
	pub *mockBillingEventPublisher,
	pricingRepo *mockPricingRuleRepo,
) *Server {
	billingService := application.NewBillingService(accountRepo, transactionRepo, pub)
	pricingService := application.NewPricingService(pricingRepo)
	return NewServer(billingService, pricingService)
}

// --- Tests ---

func TestBillingServer_GetBalance(t *testing.T) {
	t.Run("existing account returns OK", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		txRepo := new(mockTransactionRepo)
		pub := new(mockBillingEventPublisher)
		pricingRepo := new(mockPricingRuleRepo)
		srv := newTestBillingServer(accountRepo, txRepo, pub, pricingRepo)

		clientID := uuid.New()
		account := &domain.Account{
			ID:        uuid.New(),
			ClientID:  clientID,
			Balance:   "1000.50",
			Currency:  "RUB",
			UpdatedAt: time.Now(),
		}

		accountRepo.On("GetByClientID", mock.Anything, clientID).Return(account, nil)

		resp, err := srv.GetBalance(context.Background(), &billingv1.GetBalanceRequest{
			ClientId: clientID.String(),
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, clientID.String(), resp.ClientId)
		assert.Equal(t, "1000.50", resp.Balance)
		assert.Equal(t, "RUB", resp.Currency)
	})

	t.Run("account not found returns zero balance", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		txRepo := new(mockTransactionRepo)
		pub := new(mockBillingEventPublisher)
		pricingRepo := new(mockPricingRuleRepo)
		srv := newTestBillingServer(accountRepo, txRepo, pub, pricingRepo)

		clientID := uuid.New()
		accountRepo.On("GetByClientID", mock.Anything, clientID).Return(nil, domain.ErrAccountNotFound)

		resp, err := srv.GetBalance(context.Background(), &billingv1.GetBalanceRequest{
			ClientId: clientID.String(),
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, clientID.String(), resp.ClientId)
		assert.Equal(t, "0", resp.Balance)
		assert.Equal(t, "RUB", resp.Currency)
	})

	t.Run("empty client_id returns InvalidArgument", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		txRepo := new(mockTransactionRepo)
		pub := new(mockBillingEventPublisher)
		pricingRepo := new(mockPricingRuleRepo)
		srv := newTestBillingServer(accountRepo, txRepo, pub, pricingRepo)

		resp, err := srv.GetBalance(context.Background(), &billingv1.GetBalanceRequest{
			ClientId: "",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("invalid client_id returns InvalidArgument", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		txRepo := new(mockTransactionRepo)
		pub := new(mockBillingEventPublisher)
		pricingRepo := new(mockPricingRuleRepo)
		srv := newTestBillingServer(accountRepo, txRepo, pub, pricingRepo)

		resp, err := srv.GetBalance(context.Background(), &billingv1.GetBalanceRequest{
			ClientId: "not-a-uuid",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestBillingServer_DeductCredits(t *testing.T) {
	t.Run("sufficient balance returns OK", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		txRepo := new(mockTransactionRepo)
		pub := new(mockBillingEventPublisher)
		pricingRepo := new(mockPricingRuleRepo)
		srv := newTestBillingServer(accountRepo, txRepo, pub, pricingRepo)

		clientID := uuid.New()
		account := &domain.Account{
			ID:       uuid.New(),
			ClientID: clientID,
			Balance:  "500.00",
			Currency: "RUB",
		}

		accountRepo.On("GetByClientID", mock.Anything, clientID).Return(account, nil)
		accountRepo.On("UpdateBalance", mock.Anything, clientID, mock.AnythingOfType("string")).Return(nil)
		txRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Transaction")).Return(nil)
		pub.On("PublishBalanceChanged", mock.Anything, clientID.String(), mock.Anything, "RUB").Return(nil)
		pub.On("PublishTransactionCompleted", mock.Anything, mock.Anything, clientID.String(), "charge", "100.00", "RUB").Return(nil)

		resp, err := srv.DeductCredits(context.Background(), &billingv1.DeductCreditsRequest{
			ClientId:    clientID.String(),
			Amount:      "100.00",
			Currency:    "RUB",
			Description: "test deduction",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)
		assert.NotEmpty(t, resp.TransactionId)
	})

	t.Run("insufficient balance returns success=false", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		txRepo := new(mockTransactionRepo)
		pub := new(mockBillingEventPublisher)
		pricingRepo := new(mockPricingRuleRepo)
		srv := newTestBillingServer(accountRepo, txRepo, pub, pricingRepo)

		clientID := uuid.New()
		account := &domain.Account{
			ID:       uuid.New(),
			ClientID: clientID,
			Balance:  "10.00",
			Currency: "RUB",
		}

		accountRepo.On("GetByClientID", mock.Anything, clientID).Return(account, nil)

		resp, err := srv.DeductCredits(context.Background(), &billingv1.DeductCreditsRequest{
			ClientId:    clientID.String(),
			Amount:      "500.00",
			Currency:    "RUB",
			Description: "test deduction",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.False(t, resp.Success)
		assert.Equal(t, "insufficient balance", resp.Error)
	})

	t.Run("empty client_id returns InvalidArgument", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		txRepo := new(mockTransactionRepo)
		pub := new(mockBillingEventPublisher)
		pricingRepo := new(mockPricingRuleRepo)
		srv := newTestBillingServer(accountRepo, txRepo, pub, pricingRepo)

		resp, err := srv.DeductCredits(context.Background(), &billingv1.DeductCreditsRequest{
			ClientId: "",
			Amount:   "100.00",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("empty amount returns InvalidArgument", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		txRepo := new(mockTransactionRepo)
		pub := new(mockBillingEventPublisher)
		pricingRepo := new(mockPricingRuleRepo)
		srv := newTestBillingServer(accountRepo, txRepo, pub, pricingRepo)

		resp, err := srv.DeductCredits(context.Background(), &billingv1.DeductCreditsRequest{
			ClientId: uuid.New().String(),
			Amount:   "",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestBillingServer_ChargeMessage(t *testing.T) {
	t.Run("sufficient balance returns OK", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		txRepo := new(mockTransactionRepo)
		pub := new(mockBillingEventPublisher)
		pricingRepo := new(mockPricingRuleRepo)
		srv := newTestBillingServer(accountRepo, txRepo, pub, pricingRepo)

		clientID := uuid.New()
		messageID := uuid.New()
		account := &domain.Account{
			ID:       uuid.New(),
			ClientID: clientID,
			Balance:  "500.00",
			Currency: "RUB",
		}

		// ChargeMessage first checks if transaction already exists for this message
		txRepo.On("GetByMessageID", mock.Anything, messageID).Return(nil, domain.ErrTransactionNotFound)
		accountRepo.On("GetByClientID", mock.Anything, clientID).Return(account, nil)
		accountRepo.On("UpdateBalance", mock.Anything, clientID, mock.AnythingOfType("string")).Return(nil)
		txRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Transaction")).Return(nil)
		pub.On("PublishBalanceChanged", mock.Anything, clientID.String(), mock.Anything, "RUB").Return(nil)
		pub.On("PublishTransactionCompleted", mock.Anything, mock.Anything, clientID.String(), "charge", "5.00", "RUB").Return(nil)

		resp, err := srv.ChargeMessage(context.Background(), &billingv1.ChargeMessageRequest{
			ClientId:    clientID.String(),
			MessageId:   messageID.String(),
			Amount:      "5.00",
			Currency:    "RUB",
			Description: "SMS charge",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)
		assert.NotEmpty(t, resp.TransactionId)
	})

	t.Run("insufficient balance returns success=false", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		txRepo := new(mockTransactionRepo)
		pub := new(mockBillingEventPublisher)
		pricingRepo := new(mockPricingRuleRepo)
		srv := newTestBillingServer(accountRepo, txRepo, pub, pricingRepo)

		clientID := uuid.New()
		messageID := uuid.New()
		account := &domain.Account{
			ID:       uuid.New(),
			ClientID: clientID,
			Balance:  "1.00",
			Currency: "RUB",
		}

		txRepo.On("GetByMessageID", mock.Anything, messageID).Return(nil, domain.ErrTransactionNotFound)
		accountRepo.On("GetByClientID", mock.Anything, clientID).Return(account, nil)

		resp, err := srv.ChargeMessage(context.Background(), &billingv1.ChargeMessageRequest{
			ClientId:    clientID.String(),
			MessageId:   messageID.String(),
			Amount:      "100.00",
			Currency:    "RUB",
			Description: "SMS charge",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.False(t, resp.Success)
		assert.Equal(t, "insufficient balance", resp.Error)
	})

	t.Run("missing required fields return InvalidArgument", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		txRepo := new(mockTransactionRepo)
		pub := new(mockBillingEventPublisher)
		pricingRepo := new(mockPricingRuleRepo)
		srv := newTestBillingServer(accountRepo, txRepo, pub, pricingRepo)

		tests := []struct {
			name string
			req  *billingv1.ChargeMessageRequest
		}{
			{"empty client_id", &billingv1.ChargeMessageRequest{ClientId: "", MessageId: uuid.New().String(), Amount: "5.00"}},
			{"empty message_id", &billingv1.ChargeMessageRequest{ClientId: uuid.New().String(), MessageId: "", Amount: "5.00"}},
			{"empty amount", &billingv1.ChargeMessageRequest{ClientId: uuid.New().String(), MessageId: uuid.New().String(), Amount: ""}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp, err := srv.ChargeMessage(context.Background(), tt.req)
				require.Error(t, err)
				assert.Nil(t, resp)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, codes.InvalidArgument, st.Code())
			})
		}
	})
}

func TestBillingServer_AddCredits(t *testing.T) {
	t.Run("success returns transaction", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		txRepo := new(mockTransactionRepo)
		pub := new(mockBillingEventPublisher)
		pricingRepo := new(mockPricingRuleRepo)
		srv := newTestBillingServer(accountRepo, txRepo, pub, pricingRepo)

		clientID := uuid.New()
		account := &domain.Account{
			ID: uuid.New(), ClientID: clientID, Balance: "100.00", Currency: "RUB",
		}

		accountRepo.On("GetByClientID", mock.Anything, clientID).Return(account, nil)
		accountRepo.On("UpdateBalance", mock.Anything, clientID, mock.AnythingOfType("string")).Return(nil)
		txRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Transaction")).Return(nil)
		pub.On("PublishBalanceChanged", mock.Anything, clientID.String(), mock.Anything, "RUB").Return(nil)
		pub.On("PublishTransactionCompleted", mock.Anything, mock.Anything, clientID.String(), "credit", "50.00", "RUB").Return(nil)

		resp, err := srv.AddCredits(context.Background(), &billingv1.AddCreditsRequest{
			ClientId:      clientID.String(),
			Amount:        "50.00",
			Currency:      "RUB",
			Description:   "top-up",
			PaymentMethod: "card",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)
		assert.NotEmpty(t, resp.TransactionId)
		assert.Equal(t, "150.000000", resp.NewBalance)
	})

	t.Run("empty client_id returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.AddCredits(context.Background(), &billingv1.AddCreditsRequest{
			ClientId: "", Amount: "50.00",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("empty amount returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.AddCredits(context.Background(), &billingv1.AddCreditsRequest{
			ClientId: uuid.New().String(), Amount: "",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("invalid client_id returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.AddCredits(context.Background(), &billingv1.AddCreditsRequest{
			ClientId: "not-uuid", Amount: "50.00",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("defaults currency to RUB when empty", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		txRepo := new(mockTransactionRepo)
		pub := new(mockBillingEventPublisher)
		pricingRepo := new(mockPricingRuleRepo)
		srv := newTestBillingServer(accountRepo, txRepo, pub, pricingRepo)

		clientID := uuid.New()
		account := &domain.Account{
			ID: uuid.New(), ClientID: clientID, Balance: "0.00", Currency: "RUB",
		}

		accountRepo.On("GetByClientID", mock.Anything, clientID).Return(account, nil)
		accountRepo.On("UpdateBalance", mock.Anything, clientID, mock.AnythingOfType("string")).Return(nil)
		txRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Transaction")).Return(nil)
		pub.On("PublishBalanceChanged", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		pub.On("PublishTransactionCompleted", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		resp, err := srv.AddCredits(context.Background(), &billingv1.AddCreditsRequest{
			ClientId: clientID.String(), Amount: "10.00", Currency: "",
		})

		require.NoError(t, err)
		assert.True(t, resp.Success)
	})
}

func TestBillingServer_GetTransactionHistory(t *testing.T) {
	t.Run("returns transactions for client", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		txRepo := new(mockTransactionRepo)
		pub := new(mockBillingEventPublisher)
		pricingRepo := new(mockPricingRuleRepo)
		srv := newTestBillingServer(accountRepo, txRepo, pub, pricingRepo)

		clientID := uuid.New()
		msgID := uuid.New()
		txs := []*domain.Transaction{
			{
				ID: uuid.New(), ClientID: clientID, Type: domain.TransactionTypeCharge,
				Amount: "5.00", Currency: "RUB", BalanceBefore: "100.00", BalanceAfter: "95.00",
				MessageID: &msgID, CreatedAt: time.Now(),
			},
		}

		txRepo.On("GetByClientID", mock.Anything, clientID, 100, 0).Return(txs, nil)

		resp, err := srv.GetTransactionHistory(context.Background(), &billingv1.GetTransactionHistoryRequest{
			ClientId: clientID.String(),
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Len(t, resp.Transactions, 1)
		assert.Equal(t, "charge", resp.Transactions[0].Type)
		assert.Equal(t, msgID.String(), resp.Transactions[0].MessageId)
	})

	t.Run("returns all transactions when client_id empty", func(t *testing.T) {
		txRepo := new(mockTransactionRepo)
		srv := newTestBillingServer(new(mockAccountRepo), txRepo, new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		txRepo.On("GetAll", mock.Anything, 100, 0).Return([]*domain.Transaction{}, nil)

		resp, err := srv.GetTransactionHistory(context.Background(), &billingv1.GetTransactionHistoryRequest{})

		require.NoError(t, err)
		assert.Empty(t, resp.Transactions)
	})

	t.Run("invalid client_id returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.GetTransactionHistory(context.Background(), &billingv1.GetTransactionHistoryRequest{
			ClientId: "not-uuid",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("respects limit and offset", func(t *testing.T) {
		txRepo := new(mockTransactionRepo)
		srv := newTestBillingServer(new(mockAccountRepo), txRepo, new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		clientID := uuid.New()
		txRepo.On("GetByClientID", mock.Anything, clientID, 10, 5).Return([]*domain.Transaction{}, nil)

		resp, err := srv.GetTransactionHistory(context.Background(), &billingv1.GetTransactionHistoryRequest{
			ClientId: clientID.String(), Limit: 10, Offset: 5,
		})

		require.NoError(t, err)
		assert.Equal(t, int32(10), resp.Limit)
		assert.Equal(t, int32(5), resp.Offset)
		txRepo.AssertExpectations(t)
	})

	t.Run("clamps negative offset to zero", func(t *testing.T) {
		txRepo := new(mockTransactionRepo)
		srv := newTestBillingServer(new(mockAccountRepo), txRepo, new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		clientID := uuid.New()
		txRepo.On("GetByClientID", mock.Anything, clientID, 100, 0).Return([]*domain.Transaction{}, nil)

		resp, err := srv.GetTransactionHistory(context.Background(), &billingv1.GetTransactionHistoryRequest{
			ClientId: clientID.String(), Offset: -10,
		})

		require.NoError(t, err)
		assert.Equal(t, int32(0), resp.Offset)
	})
}

func TestBillingServer_GetPricingRules(t *testing.T) {
	t.Run("returns rules for client", func(t *testing.T) {
		pricingRepo := new(mockPricingRuleRepo)
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), pricingRepo)

		clientID := uuid.New()
		rules := []*domain.PricingRule{
			{
				ID: uuid.New(), ClientID: &clientID, DestinationPattern: "^\\+7",
				PricePerMessage: "0.05", Currency: "RUB", Priority: 10, Active: true,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			},
		}

		pricingRepo.On("GetByClientID", mock.Anything, &clientID, false).Return(rules, nil)

		resp, err := srv.GetPricingRules(context.Background(), &billingv1.GetPricingRulesRequest{
			ClientId: clientID.String(),
		})

		require.NoError(t, err)
		assert.Len(t, resp.Rules, 1)
		assert.Equal(t, "^\\+7", resp.Rules[0].DestinationPattern)
		assert.Equal(t, clientID.String(), resp.Rules[0].ClientId)
	})

	t.Run("returns global rules when client_id empty", func(t *testing.T) {
		pricingRepo := new(mockPricingRuleRepo)
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), pricingRepo)

		rules := []*domain.PricingRule{
			{
				ID: uuid.New(), ClientID: nil, DestinationPattern: "^\\+",
				PricePerMessage: "0.01", Currency: "RUB", Priority: 1, Active: true,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			},
		}

		pricingRepo.On("GetByClientID", mock.Anything, (*uuid.UUID)(nil), false).Return(rules, nil)

		resp, err := srv.GetPricingRules(context.Background(), &billingv1.GetPricingRulesRequest{})

		require.NoError(t, err)
		assert.Len(t, resp.Rules, 1)
		assert.Empty(t, resp.Rules[0].ClientId)
	})

	t.Run("invalid client_id returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.GetPricingRules(context.Background(), &billingv1.GetPricingRulesRequest{
			ClientId: "not-uuid",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestBillingServer_TransferBalance(t *testing.T) {
	t.Run("successful transfer", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		txRepo := new(mockTransactionRepo)
		pub := new(mockBillingEventPublisher)
		pricingRepo := new(mockPricingRuleRepo)
		srv := newTestBillingServer(accountRepo, txRepo, pub, pricingRepo)

		fromID := uuid.New()
		toID := uuid.New()
		fromAccount := &domain.Account{ID: uuid.New(), ClientID: fromID, Balance: "200.00", Currency: "RUB"}
		toAccount := &domain.Account{ID: uuid.New(), ClientID: toID, Balance: "50.00", Currency: "RUB"}

		accountRepo.On("GetByClientID", mock.Anything, fromID).Return(fromAccount, nil)
		accountRepo.On("GetByClientID", mock.Anything, toID).Return(toAccount, nil)
		accountRepo.On("UpdateBalance", mock.Anything, fromID, mock.AnythingOfType("string")).Return(nil)
		accountRepo.On("UpdateBalance", mock.Anything, toID, mock.AnythingOfType("string")).Return(nil)
		txRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Transaction")).Return(nil).Twice()
		pub.On("PublishBalanceChanged", mock.Anything, fromID.String(), mock.Anything, "RUB").Return(nil)
		pub.On("PublishBalanceChanged", mock.Anything, toID.String(), mock.Anything, "RUB").Return(nil)

		resp, err := srv.TransferBalance(context.Background(), &billingv1.TransferBalanceRequest{
			FromClientId: fromID.String(),
			ToClientId:   toID.String(),
			Amount:       "100.00",
			Currency:     "RUB",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotEmpty(t, resp.TransferId)
		assert.Equal(t, "100.000000", resp.FromBalance)
		assert.Equal(t, "150.000000", resp.ToBalance)
	})

	t.Run("insufficient balance returns FailedPrecondition", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		srv := newTestBillingServer(accountRepo, new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		fromID := uuid.New()
		fromAccount := &domain.Account{ID: uuid.New(), ClientID: fromID, Balance: "10.00", Currency: "RUB"}

		accountRepo.On("GetByClientID", mock.Anything, fromID).Return(fromAccount, nil)

		resp, err := srv.TransferBalance(context.Background(), &billingv1.TransferBalanceRequest{
			FromClientId: fromID.String(),
			ToClientId:   uuid.New().String(),
			Amount:       "500.00",
			Currency:     "RUB",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
	})

	t.Run("missing fields return InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		tests := []struct {
			name string
			req  *billingv1.TransferBalanceRequest
		}{
			{"empty from_client_id", &billingv1.TransferBalanceRequest{FromClientId: "", ToClientId: uuid.New().String(), Amount: "10"}},
			{"empty to_client_id", &billingv1.TransferBalanceRequest{FromClientId: uuid.New().String(), ToClientId: "", Amount: "10"}},
			{"empty amount", &billingv1.TransferBalanceRequest{FromClientId: uuid.New().String(), ToClientId: uuid.New().String(), Amount: ""}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp, err := srv.TransferBalance(context.Background(), tt.req)
				require.Error(t, err)
				assert.Nil(t, resp)
				st, _ := status.FromError(err)
				assert.Equal(t, codes.InvalidArgument, st.Code())
			})
		}
	})

	t.Run("invalid from_client_id returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.TransferBalance(context.Background(), &billingv1.TransferBalanceRequest{
			FromClientId: "bad", ToClientId: uuid.New().String(), Amount: "10",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("invalid to_client_id returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.TransferBalance(context.Background(), &billingv1.TransferBalanceRequest{
			FromClientId: uuid.New().String(), ToClientId: "bad", Amount: "10",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestBillingServer_CreatePricingRule(t *testing.T) {
	t.Run("creates rule successfully", func(t *testing.T) {
		pricingRepo := new(mockPricingRuleRepo)
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), pricingRepo)

		pricingRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.PricingRule")).Return(nil)

		resp, err := srv.CreatePricingRule(context.Background(), &billingv1.CreatePricingRuleRequest{
			DestinationPattern: "^\\+7",
			PricePerMessage:    "0.05",
			Currency:           "RUB",
			Priority:           10,
			Active:             true,
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotEmpty(t, resp.RuleId)
		assert.NotNil(t, resp.CreatedAt)
		pricingRepo.AssertExpectations(t)
	})

	t.Run("creates client-specific rule", func(t *testing.T) {
		pricingRepo := new(mockPricingRuleRepo)
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), pricingRepo)

		clientID := uuid.New()
		pricingRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.PricingRule")).Return(nil)

		resp, err := srv.CreatePricingRule(context.Background(), &billingv1.CreatePricingRuleRequest{
			ClientId:           clientID.String(),
			DestinationPattern: "^\\+1",
			PricePerMessage:    "0.03",
			Currency:           "EUR",
			Priority:           5,
		})

		require.NoError(t, err)
		assert.NotEmpty(t, resp.RuleId)
	})

	t.Run("empty destination_pattern returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.CreatePricingRule(context.Background(), &billingv1.CreatePricingRuleRequest{
			DestinationPattern: "", PricePerMessage: "0.05",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("empty price_per_message returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.CreatePricingRule(context.Background(), &billingv1.CreatePricingRuleRequest{
			DestinationPattern: "^\\+7", PricePerMessage: "",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("invalid client_id returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.CreatePricingRule(context.Background(), &billingv1.CreatePricingRuleRequest{
			ClientId: "bad", DestinationPattern: "^\\+7", PricePerMessage: "0.05",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestBillingServer_FreezeAccount(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		srv := newTestBillingServer(accountRepo, new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		clientID := uuid.New()
		adminID := uuid.New()
		frozenAt := time.Now().UTC().Truncate(time.Second)

		accountRepo.On("FreezeAccount", mock.Anything, clientID, adminID).Return(frozenAt, nil)

		resp, err := srv.FreezeAccount(context.Background(), &billingv1.FreezeAccountRequest{
			ClientId: clientID.String(), AdminId: adminID.String(),
		})

		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.NotNil(t, resp.FrozenAt)
		accountRepo.AssertExpectations(t)
	})

	t.Run("empty client_id returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.FreezeAccount(context.Background(), &billingv1.FreezeAccountRequest{
			ClientId: "", AdminId: uuid.New().String(),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("empty admin_id returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.FreezeAccount(context.Background(), &billingv1.FreezeAccountRequest{
			ClientId: uuid.New().String(), AdminId: "",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("invalid client_id returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.FreezeAccount(context.Background(), &billingv1.FreezeAccountRequest{
			ClientId: "bad", AdminId: uuid.New().String(),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("invalid admin_id returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.FreezeAccount(context.Background(), &billingv1.FreezeAccountRequest{
			ClientId: uuid.New().String(), AdminId: "bad",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestBillingServer_UnfreezeAccount(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		srv := newTestBillingServer(accountRepo, new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		clientID := uuid.New()
		accountRepo.On("UnfreezeAccount", mock.Anything, clientID).Return(nil)

		resp, err := srv.UnfreezeAccount(context.Background(), &billingv1.UnfreezeAccountRequest{
			ClientId: clientID.String(),
		})

		require.NoError(t, err)
		assert.True(t, resp.Success)
		accountRepo.AssertExpectations(t)
	})

	t.Run("empty client_id returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.UnfreezeAccount(context.Background(), &billingv1.UnfreezeAccountRequest{ClientId: ""})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("invalid client_id returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.UnfreezeAccount(context.Background(), &billingv1.UnfreezeAccountRequest{ClientId: "bad"})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("repo error returns Internal", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		srv := newTestBillingServer(accountRepo, new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		clientID := uuid.New()
		accountRepo.On("UnfreezeAccount", mock.Anything, clientID).Return(assert.AnError)

		resp, err := srv.UnfreezeAccount(context.Background(), &billingv1.UnfreezeAccountRequest{
			ClientId: clientID.String(),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.Internal, st.Code())
	})
}

func TestBillingServer_SetCreditLimit(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		srv := newTestBillingServer(accountRepo, new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		clientID := uuid.New()
		accountRepo.On("SetCreditLimit", mock.Anything, clientID, "500.00").Return(nil)

		resp, err := srv.SetCreditLimit(context.Background(), &billingv1.SetCreditLimitRequest{
			ClientId: clientID.String(), CreditLimit: "500.00",
		})

		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, "500.00", resp.CreditLimit)
		accountRepo.AssertExpectations(t)
	})

	t.Run("empty client_id returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.SetCreditLimit(context.Background(), &billingv1.SetCreditLimitRequest{
			ClientId: "", CreditLimit: "500.00",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("invalid client_id returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.SetCreditLimit(context.Background(), &billingv1.SetCreditLimitRequest{
			ClientId: "bad", CreditLimit: "500.00",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("repo error returns Internal", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		srv := newTestBillingServer(accountRepo, new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		clientID := uuid.New()
		accountRepo.On("SetCreditLimit", mock.Anything, clientID, "500.00").Return(assert.AnError)

		resp, err := srv.SetCreditLimit(context.Background(), &billingv1.SetCreditLimitRequest{
			ClientId: clientID.String(), CreditLimit: "500.00",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.Internal, st.Code())
	})
}

func TestBillingServer_SetLowBalanceThreshold(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		srv := newTestBillingServer(accountRepo, new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		clientID := uuid.New()
		accountRepo.On("SetLowBalanceThreshold", mock.Anything, clientID, "100.00").Return(nil)

		resp, err := srv.SetLowBalanceThreshold(context.Background(), &billingv1.SetLowBalanceThresholdRequest{
			ClientId: clientID.String(), Threshold: "100.00",
		})

		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, "100.00", resp.Threshold)
		accountRepo.AssertExpectations(t)
	})

	t.Run("empty client_id returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.SetLowBalanceThreshold(context.Background(), &billingv1.SetLowBalanceThresholdRequest{
			ClientId: "", Threshold: "100.00",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("invalid client_id returns InvalidArgument", func(t *testing.T) {
		srv := newTestBillingServer(new(mockAccountRepo), new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		resp, err := srv.SetLowBalanceThreshold(context.Background(), &billingv1.SetLowBalanceThresholdRequest{
			ClientId: "bad", Threshold: "100.00",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestBillingServer_ListBalances(t *testing.T) {
	t.Run("returns balances with filters", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		srv := newTestBillingServer(accountRepo, new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		frozenAt := time.Now()
		frozenBy := uuid.New()
		balances := []domain.BalanceInfo{
			{
				ClientID: uuid.New(), ClientName: "Alice", Balance: "500.00", Currency: "RUB",
				Frozen: false, CreditLimit: "100.00", LowBalanceThreshold: "50.00",
				UpdatedAt: time.Now(),
			},
			{
				ClientID: uuid.New(), ClientName: "Bob", Balance: "10.00", Currency: "RUB",
				Frozen: true, FrozenAt: &frozenAt, FrozenBy: &frozenBy,
				UpdatedAt: time.Now(),
			},
		}

		accountRepo.On("ListBalances", mock.Anything, "Ali", "active", false, int32(10), int32(0)).
			Return(balances, int32(2), nil)

		resp, err := srv.ListBalances(context.Background(), &billingv1.ListBalancesRequest{
			Search: "Ali", Status: "active", Limit: 10, Offset: 0,
		})

		require.NoError(t, err)
		assert.Len(t, resp.Balances, 2)
		assert.Equal(t, int32(2), resp.Total)
		assert.Equal(t, "Alice", resp.Balances[0].ClientName)
		assert.False(t, resp.Balances[0].Frozen)
		assert.True(t, resp.Balances[1].Frozen)
		assert.NotNil(t, resp.Balances[1].FrozenAt)
		assert.Equal(t, frozenBy.String(), resp.Balances[1].FrozenBy)
		accountRepo.AssertExpectations(t)
	})

	t.Run("defaults limit to 50 when zero", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		srv := newTestBillingServer(accountRepo, new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		accountRepo.On("ListBalances", mock.Anything, "", "", false, int32(50), int32(0)).
			Return([]domain.BalanceInfo(nil), int32(0), nil)

		resp, err := srv.ListBalances(context.Background(), &billingv1.ListBalancesRequest{})

		require.NoError(t, err)
		assert.Equal(t, int32(50), resp.Limit)
		accountRepo.AssertExpectations(t)
	})

	t.Run("repo error returns Internal", func(t *testing.T) {
		accountRepo := new(mockAccountRepo)
		srv := newTestBillingServer(accountRepo, new(mockTransactionRepo), new(mockBillingEventPublisher), new(mockPricingRuleRepo))

		accountRepo.On("ListBalances", mock.Anything, "", "", false, int32(50), int32(0)).
			Return([]domain.BalanceInfo(nil), int32(0), assert.AnError)

		resp, err := srv.ListBalances(context.Background(), &billingv1.ListBalancesRequest{})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.Internal, st.Code())
	})
}
