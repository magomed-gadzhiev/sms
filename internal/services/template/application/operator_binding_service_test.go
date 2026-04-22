package application_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/template/application"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
	"github.com/smpp-server/smpp-server/internal/services/template/mocks"
)

func TestSubmitForOperators_CreatesBindingPerOperator(t *testing.T) {
	repo := new(mocks.MockOperatorBindingRepo)
	svc := application.NewOperatorBindingService(repo)
	ctx := context.Background()

	tmplID, snID := uuid.New(), uuid.New()
	ops := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	repo.On("Create", mock.Anything, mock.MatchedBy(func(b *domain.OperatorTemplateBinding) bool {
		return b.TemplateID == tmplID && b.SenderNameID == snID && b.Status == domain.OperatorBindingStatusPending
	})).Return(nil).Times(3)

	err := svc.SubmitForOperators(ctx, tmplID, snID, ops)
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestSubmitForOperators_IdempotentOnDuplicate(t *testing.T) {
	repo := new(mocks.MockOperatorBindingRepo)
	svc := application.NewOperatorBindingService(repo)
	ctx := context.Background()

	tmplID, snID, opID := uuid.New(), uuid.New(), uuid.New()
	repo.On("Create", mock.Anything, mock.Anything).Return(domain.ErrDuplicateOperatorBinding)

	err := svc.SubmitForOperators(ctx, tmplID, snID, []uuid.UUID{opID})
	require.NoError(t, err) // duplicate is swallowed
}

func TestApprove_InvalidTransitionFromApproved(t *testing.T) {
	repo := new(mocks.MockOperatorBindingRepo)
	svc := application.NewOperatorBindingService(repo)
	ctx := context.Background()

	b := &domain.OperatorTemplateBinding{ID: uuid.New(), Status: domain.OperatorBindingStatusApproved}
	repo.On("GetByID", mock.Anything, b.ID).Return(b, nil)

	err := svc.Approve(ctx, b.ID, uuid.New())
	assert.ErrorIs(t, err, domain.ErrInvalidOperatorBindingTransition)
}

func TestApprove_HappyPath(t *testing.T) {
	repo := new(mocks.MockOperatorBindingRepo)
	svc := application.NewOperatorBindingService(repo)
	ctx := context.Background()

	id, reviewer := uuid.New(), uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(&domain.OperatorTemplateBinding{ID: id, Status: domain.OperatorBindingStatusPending}, nil)
	repo.On("UpdateStatus", mock.Anything, id, domain.OperatorBindingStatusApproved, "", reviewer).Return(nil)

	assert.NoError(t, svc.Approve(ctx, id, reviewer))
	repo.AssertExpectations(t)
}

func TestReject_RequiresReason(t *testing.T) {
	repo := new(mocks.MockOperatorBindingRepo)
	svc := application.NewOperatorBindingService(repo)
	err := svc.Reject(context.Background(), uuid.New(), uuid.New(), "   ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reason")
}

func TestReject_HappyPath(t *testing.T) {
	repo := new(mocks.MockOperatorBindingRepo)
	svc := application.NewOperatorBindingService(repo)
	ctx := context.Background()

	id, reviewer := uuid.New(), uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(&domain.OperatorTemplateBinding{ID: id, Status: domain.OperatorBindingStatusPending}, nil)
	repo.On("UpdateStatus", mock.Anything, id, domain.OperatorBindingStatusRejected, "not aligned with operator policy", reviewer).Return(nil)

	assert.NoError(t, svc.Reject(ctx, id, reviewer, "not aligned with operator policy"))
	repo.AssertExpectations(t)
}
