package application

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/smpp-server/smpp-server/internal/services/template/domain"
)

// operatorBindingRepo is the port the service depends on. The concrete
// *repository.OperatorBindingRepo satisfies it implicitly.
type operatorBindingRepo interface {
	Create(ctx context.Context, b *domain.OperatorTemplateBinding) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.OperatorTemplateBinding, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status, reason string, reviewer uuid.UUID) error
	ListPendingByOperator(ctx context.Context, opID uuid.UUID, limit, offset int) ([]*domain.OperatorTemplateBinding, error)
}

type OperatorBindingService struct {
	repo operatorBindingRepo
}

func NewOperatorBindingService(repo operatorBindingRepo) *OperatorBindingService {
	return &OperatorBindingService{repo: repo}
}

// SubmitForOperators creates a pending binding per operator. Duplicates from a
// re-submit are treated as idempotent (swallowed) so callers can safely retry.
func (s *OperatorBindingService) SubmitForOperators(ctx context.Context, templateID, senderNameID uuid.UUID, operatorIDs []uuid.UUID) error {
	for _, opID := range operatorIDs {
		b := domain.NewOperatorTemplateBinding(templateID, senderNameID, opID)
		if err := s.repo.Create(ctx, b); err != nil && !errors.Is(err, domain.ErrDuplicateOperatorBinding) {
			return err
		}
	}
	return nil
}

func (s *OperatorBindingService) Approve(ctx context.Context, bindingID, reviewer uuid.UUID) error {
	b, err := s.repo.GetByID(ctx, bindingID)
	if err != nil {
		return err
	}
	if !domain.IsValidOperatorBindingTransition(b.Status, domain.OperatorBindingStatusApproved) {
		return domain.ErrInvalidOperatorBindingTransition
	}
	return s.repo.UpdateStatus(ctx, bindingID, domain.OperatorBindingStatusApproved, "", reviewer)
}

func (s *OperatorBindingService) Reject(ctx context.Context, bindingID, reviewer uuid.UUID, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errors.New("rejection reason is required")
	}
	b, err := s.repo.GetByID(ctx, bindingID)
	if err != nil {
		return err
	}
	if !domain.IsValidOperatorBindingTransition(b.Status, domain.OperatorBindingStatusRejected) {
		return domain.ErrInvalidOperatorBindingTransition
	}
	return s.repo.UpdateStatus(ctx, bindingID, domain.OperatorBindingStatusRejected, reason, reviewer)
}

func (s *OperatorBindingService) ListPendingByOperator(ctx context.Context, opID uuid.UUID, limit, offset int) ([]*domain.OperatorTemplateBinding, error) {
	return s.repo.ListPendingByOperator(ctx, opID, limit, offset)
}
