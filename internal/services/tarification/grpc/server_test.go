package grpc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tarificationv1 "github.com/smpp-server/smpp-server/api/proto/tarificationv1"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// TestTarificationServer_TarifyMessage tests the TarifyMessage gRPC method validation logic.
// Since TarificationService requires complex dependencies (billing gRPC client, multiple repos),
// we test validation by calling the gRPC handler directly with a server that has nil services,
// which triggers validation errors before reaching the service layer.
func TestTarificationServer_TarifyMessage(t *testing.T) {
	t.Run("invalid client_id returns InvalidArgument", func(t *testing.T) {
		srv := &Server{}

		resp, err := srv.TarifyMessage(context.Background(), &tarificationv1.TarifyMessageRequest{
			ClientId:   "not-a-uuid",
			MessageId:  uuid.New().String(),
			OperatorId: uuid.New().String(),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Contains(t, st.Message(), "invalid client_id")
	})

	t.Run("invalid message_id returns InvalidArgument", func(t *testing.T) {
		srv := &Server{}

		resp, err := srv.TarifyMessage(context.Background(), &tarificationv1.TarifyMessageRequest{
			ClientId:   uuid.New().String(),
			MessageId:  "not-a-uuid",
			OperatorId: uuid.New().String(),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Contains(t, st.Message(), "invalid message_id")
	})

	t.Run("invalid operator_id returns InvalidArgument", func(t *testing.T) {
		srv := &Server{}

		resp, err := srv.TarifyMessage(context.Background(), &tarificationv1.TarifyMessageRequest{
			ClientId:   uuid.New().String(),
			MessageId:  uuid.New().String(),
			OperatorId: "not-a-uuid",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Contains(t, st.Message(), "invalid operator_id")
	})
}

func TestTarificationServer_CreateSenderRegistration(t *testing.T) {
	t.Run("invalid client_id returns InvalidArgument", func(t *testing.T) {
		srv := &Server{}

		resp, err := srv.CreateSenderRegistration(context.Background(), &tarificationv1.CreateSenderRegistrationRequest{
			ClientId:   "not-a-uuid",
			OperatorId: uuid.New().String(),
			SenderName: "MySender",
			Type:       "alphanumeric",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("invalid operator_id returns InvalidArgument", func(t *testing.T) {
		srv := &Server{}

		resp, err := srv.CreateSenderRegistration(context.Background(), &tarificationv1.CreateSenderRegistrationRequest{
			ClientId:   uuid.New().String(),
			OperatorId: "not-a-uuid",
			SenderName: "MySender",
			Type:       "alphanumeric",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestTarificationServer_GetTariffPlan(t *testing.T) {
	t.Run("invalid id returns InvalidArgument", func(t *testing.T) {
		srv := &Server{}

		resp, err := srv.GetTariffPlan(context.Background(), &tarificationv1.GetTariffPlanRequest{
			Id: "not-a-uuid",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestTarificationServer_CreateTariffPeriod(t *testing.T) {
	t.Run("invalid tariff_plan_id returns InvalidArgument", func(t *testing.T) {
		srv := &Server{}

		resp, err := srv.CreateTariffPeriod(context.Background(), &tarificationv1.CreateTariffPeriodRequest{
			TariffPlanId: "not-a-uuid",
			StartDate:    "2025-01-01",
			EndDate:      "2025-12-31",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("invalid start_date format returns InvalidArgument", func(t *testing.T) {
		srv := &Server{}

		resp, err := srv.CreateTariffPeriod(context.Background(), &tarificationv1.CreateTariffPeriodRequest{
			TariffPlanId: uuid.New().String(),
			StartDate:    "invalid-date",
			EndDate:      "2025-12-31",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Contains(t, st.Message(), "start_date")
	})

	t.Run("invalid end_date format returns InvalidArgument", func(t *testing.T) {
		srv := &Server{}

		resp, err := srv.CreateTariffPeriod(context.Background(), &tarificationv1.CreateTariffPeriodRequest{
			TariffPlanId: uuid.New().String(),
			StartDate:    "2025-01-01",
			EndDate:      "bad-date",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Contains(t, st.Message(), "end_date")
	})
}

func TestTarificationServer_MapDomainError(t *testing.T) {
	t.Run("NotFound errors map to codes.NotFound", func(t *testing.T) {
		for _, domainErr := range []error{
			domain.ErrTariffPlanNotFound,
			domain.ErrTariffPeriodNotFound,
			domain.ErrTariffTierNotFound,
			domain.ErrSenderRegistrationNotFound,
		} {
			grpcErr := mapDomainError(domainErr)
			st, ok := status.FromError(grpcErr)
			require.True(t, ok)
			assert.Equal(t, codes.NotFound, st.Code(), "expected NotFound for %v", domainErr)
		}
	})

	t.Run("AlreadyExists errors map to codes.AlreadyExists", func(t *testing.T) {
		for _, domainErr := range []error{
			domain.ErrTariffPlanDuplicate,
			domain.ErrSenderRegistrationDuplicate,
			domain.ErrIdempotencyKeyExists,
		} {
			grpcErr := mapDomainError(domainErr)
			st, ok := status.FromError(grpcErr)
			require.True(t, ok)
			assert.Equal(t, codes.AlreadyExists, st.Code(), "expected AlreadyExists for %v", domainErr)
		}
	})

	t.Run("FailedPrecondition errors map to codes.FailedPrecondition", func(t *testing.T) {
		for _, domainErr := range []error{
			domain.ErrInsufficientBalance,
			domain.ErrTariffPlanHasActivePeriod,
		} {
			grpcErr := mapDomainError(domainErr)
			st, ok := status.FromError(grpcErr)
			require.True(t, ok)
			assert.Equal(t, codes.FailedPrecondition, st.Code(), "expected FailedPrecondition for %v", domainErr)
		}
	})

	t.Run("InvalidArgument errors map to codes.InvalidArgument", func(t *testing.T) {
		for _, domainErr := range []error{
			domain.ErrTariffPlanInvalidStrategy,
			domain.ErrTariffPlanInvalidCategory,
			domain.ErrSenderRegistrationInvalidType,
		} {
			grpcErr := mapDomainError(domainErr)
			st, ok := status.FromError(grpcErr)
			require.True(t, ok)
			assert.Equal(t, codes.InvalidArgument, st.Code(), "expected InvalidArgument for %v", domainErr)
		}
	})
}
