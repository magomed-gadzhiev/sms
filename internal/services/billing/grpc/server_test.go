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
			Currency:  "USD",
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
		assert.Equal(t, "USD", resp.Currency)
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
		assert.Equal(t, "USD", resp.Currency)
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
			Currency: "USD",
		}

		accountRepo.On("GetByClientID", mock.Anything, clientID).Return(account, nil)
		accountRepo.On("UpdateBalance", mock.Anything, clientID, mock.AnythingOfType("string")).Return(nil)
		txRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Transaction")).Return(nil)
		pub.On("PublishBalanceChanged", mock.Anything, clientID.String(), mock.Anything, "USD").Return(nil)
		pub.On("PublishTransactionCompleted", mock.Anything, mock.Anything, clientID.String(), "charge", "100.00", "USD").Return(nil)

		resp, err := srv.DeductCredits(context.Background(), &billingv1.DeductCreditsRequest{
			ClientId:    clientID.String(),
			Amount:      "100.00",
			Currency:    "USD",
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
			Currency: "USD",
		}

		accountRepo.On("GetByClientID", mock.Anything, clientID).Return(account, nil)

		resp, err := srv.DeductCredits(context.Background(), &billingv1.DeductCreditsRequest{
			ClientId:    clientID.String(),
			Amount:      "500.00",
			Currency:    "USD",
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
			Currency: "USD",
		}

		// ChargeMessage first checks if transaction already exists for this message
		txRepo.On("GetByMessageID", mock.Anything, messageID).Return(nil, domain.ErrTransactionNotFound)
		accountRepo.On("GetByClientID", mock.Anything, clientID).Return(account, nil)
		accountRepo.On("UpdateBalance", mock.Anything, clientID, mock.AnythingOfType("string")).Return(nil)
		txRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Transaction")).Return(nil)
		pub.On("PublishBalanceChanged", mock.Anything, clientID.String(), mock.Anything, "USD").Return(nil)
		pub.On("PublishTransactionCompleted", mock.Anything, mock.Anything, clientID.String(), "charge", "5.00", "USD").Return(nil)

		resp, err := srv.ChargeMessage(context.Background(), &billingv1.ChargeMessageRequest{
			ClientId:    clientID.String(),
			MessageId:   messageID.String(),
			Amount:      "5.00",
			Currency:    "USD",
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
			Currency: "USD",
		}

		txRepo.On("GetByMessageID", mock.Anything, messageID).Return(nil, domain.ErrTransactionNotFound)
		accountRepo.On("GetByClientID", mock.Anything, clientID).Return(account, nil)

		resp, err := srv.ChargeMessage(context.Background(), &billingv1.ChargeMessageRequest{
			ClientId:    clientID.String(),
			MessageId:   messageID.String(),
			Amount:      "100.00",
			Currency:    "USD",
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
