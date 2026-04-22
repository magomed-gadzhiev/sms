package application

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
)

var (
	senderNameRegistrationsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "sender_name_registrations_total",
		Help: "Total number of sender name registration attempts",
	})
	senderNameStatusChangesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "sender_name_status_changes_total",
		Help: "Total number of sender name status changes by new status",
	}, []string{"status"})
)

type SenderNameService struct {
	repo   SenderNameRepository
	logger zerolog.Logger
}

func NewSenderNameService(repo SenderNameRepository) *SenderNameService {
	return &SenderNameService{
		repo:   repo,
		logger: log.With().Str("component", "sender-name-service").Logger(),
	}
}

func (s *SenderNameService) RegisterSenderName(ctx context.Context, clientID, companyID uuid.UUID, name, channel string) (*domain.SenderName, error) {
	if err := domain.ValidateSenderName(name); err != nil {
		return nil, err
	}
	if err := domain.ValidateChannel(channel); err != nil {
		return nil, err
	}

	// Duplicate check is channel-aware, matching the 3-col unique constraint (client_id, name, channel).
	existing, err := s.repo.GetByClientNameChannel(ctx, clientID, name, channel)
	if err != nil && !errors.Is(err, domain.ErrSenderNameNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, domain.ErrDuplicateSenderName
	}

	sn := &domain.SenderName{
		ID:        uuid.New(),
		ClientID:  clientID,
		CompanyID: companyID,
		Name:      name,
		Channel:   channel,
		Status:    domain.SenderNameStatusPending,
	}

	created, err := s.repo.Create(ctx, sn)
	if err != nil {
		return nil, err
	}

	senderNameRegistrationsTotal.Inc()
	senderNameStatusChangesTotal.WithLabelValues(domain.SenderNameStatusPending).Inc()

	if err := s.addHistory(ctx, created.ID, nil, domain.SenderNameStatusPending, nil, domain.ActorTypeSystem, ""); err != nil {
		s.logger.Error().Err(err).Str("id", created.ID.String()).Msg("failed to write history on create")
	}

	return created, nil
}

func (s *SenderNameService) UpdateSenderName(ctx context.Context, id, clientID uuid.UUID, name string) (*domain.SenderName, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.ClientID != clientID {
		return nil, domain.ErrSenderNameNotFound
	}
	if existing.Status != domain.SenderNameStatusRejected {
		return nil, domain.ErrInvalidSenderNameStatus
	}
	if err := domain.ValidateSenderName(name); err != nil {
		return nil, err
	}

	existing.Name = name
	return s.repo.Update(ctx, existing)
}

func (s *SenderNameService) ResubmitSenderName(ctx context.Context, id, clientID uuid.UUID) (*domain.SenderName, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.ClientID != clientID {
		return nil, domain.ErrSenderNameNotFound
	}
	if !domain.IsValidTransition(existing.Status, domain.SenderNameStatusPending) {
		return nil, domain.ErrInvalidSenderNameStatus
	}

	oldStatus := existing.Status
	updated, err := s.repo.UpdateStatus(ctx, id, domain.SenderNameStatusPending, "", nil)
	if err != nil {
		return nil, err
	}

	senderNameStatusChangesTotal.WithLabelValues(domain.SenderNameStatusPending).Inc()

	if err := s.addHistory(ctx, id, &oldStatus, domain.SenderNameStatusPending, &clientID, domain.ActorTypeClient, ""); err != nil {
		s.logger.Error().Err(err).Str("id", id.String()).Msg("failed to write history on resubmit")
	}

	return updated, nil
}

func (s *SenderNameService) ApproveSenderName(ctx context.Context, id, actorID uuid.UUID) (*domain.SenderName, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !domain.IsValidTransition(existing.Status, domain.SenderNameStatusApproved) {
		return nil, domain.ErrInvalidSenderNameStatus
	}

	oldStatus := existing.Status
	updated, err := s.repo.UpdateStatus(ctx, id, domain.SenderNameStatusApproved, "", &actorID)
	if err != nil {
		return nil, err
	}

	senderNameStatusChangesTotal.WithLabelValues(domain.SenderNameStatusApproved).Inc()

	if err := s.addHistory(ctx, id, &oldStatus, domain.SenderNameStatusApproved, &actorID, domain.ActorTypeAdmin, ""); err != nil {
		s.logger.Error().Err(err).Str("id", id.String()).Msg("failed to write history on approve")
	}

	return updated, nil
}

func (s *SenderNameService) RejectSenderName(ctx context.Context, id, actorID uuid.UUID, reason string) (*domain.SenderName, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !domain.IsValidTransition(existing.Status, domain.SenderNameStatusRejected) {
		return nil, domain.ErrInvalidSenderNameStatus
	}

	oldStatus := existing.Status
	updated, err := s.repo.UpdateStatus(ctx, id, domain.SenderNameStatusRejected, reason, &actorID)
	if err != nil {
		return nil, err
	}

	senderNameStatusChangesTotal.WithLabelValues(domain.SenderNameStatusRejected).Inc()

	if err := s.addHistory(ctx, id, &oldStatus, domain.SenderNameStatusRejected, &actorID, domain.ActorTypeAdmin, reason); err != nil {
		s.logger.Error().Err(err).Str("id", id.String()).Msg("failed to write history on reject")
	}

	return updated, nil
}

func (s *SenderNameService) DeactivateSenderName(ctx context.Context, id, actorID uuid.UUID, reason string) (*domain.SenderName, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !domain.IsValidTransition(existing.Status, domain.SenderNameStatusDeactivated) {
		return nil, domain.ErrInvalidSenderNameStatus
	}

	oldStatus := existing.Status
	updated, err := s.repo.UpdateStatus(ctx, id, domain.SenderNameStatusDeactivated, reason, &actorID)
	if err != nil {
		return nil, err
	}

	senderNameStatusChangesTotal.WithLabelValues(domain.SenderNameStatusDeactivated).Inc()

	if err := s.addHistory(ctx, id, &oldStatus, domain.SenderNameStatusDeactivated, &actorID, domain.ActorTypeAdmin, reason); err != nil {
		s.logger.Error().Err(err).Str("id", id.String()).Msg("failed to write history on deactivate")
	}

	return updated, nil
}

func (s *SenderNameService) GetSenderName(ctx context.Context, id, clientID uuid.UUID) (*domain.SenderName, error) {
	sn, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if sn.ClientID != clientID {
		return nil, domain.ErrSenderNameNotFound
	}
	return sn, nil
}

func (s *SenderNameService) ListSenderNames(ctx context.Context, clientID uuid.UUID, status string, limit, offset int) ([]*domain.SenderName, int, error) {
	return s.repo.ListByClient(ctx, clientID, status, limit, offset)
}

func (s *SenderNameService) ListAllSenderNames(ctx context.Context, clientID *uuid.UUID, status, nameQuery string, limit, offset int) ([]*domain.SenderName, int, error) {
	return s.repo.ListAll(ctx, clientID, status, nameQuery, limit, offset)
}

func (s *SenderNameService) GetSenderNameHistory(ctx context.Context, senderNameID, clientID uuid.UUID, limit, offset int) ([]*domain.SenderNameStatusHistory, int, error) {
	// verify ownership
	sn, err := s.repo.GetByID(ctx, senderNameID)
	if err != nil {
		return nil, 0, err
	}
	if sn.ClientID != clientID {
		return nil, 0, domain.ErrSenderNameNotFound
	}
	return s.repo.GetHistory(ctx, senderNameID, limit, offset)
}

func (s *SenderNameService) GetSenderNameAdmin(ctx context.Context, id uuid.UUID) (*domain.SenderName, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *SenderNameService) GetSenderNameHistoryAdmin(ctx context.Context, senderNameID uuid.UUID, limit, offset int) ([]*domain.SenderNameStatusHistory, int, error) {
	return s.repo.GetHistory(ctx, senderNameID, limit, offset)
}

// ValidateSenderNameForTemplate checks that a sender name exists, belongs to clientID, and is approved.
func (s *SenderNameService) ValidateSenderNameForTemplate(ctx context.Context, senderNameID, clientID uuid.UUID) error {
	sn, err := s.repo.GetByID(ctx, senderNameID)
	if err != nil {
		if errors.Is(err, domain.ErrSenderNameNotFound) {
			return domain.ErrSenderNameNotFound
		}
		return err
	}
	if sn.ClientID != clientID {
		return domain.ErrSenderNameNotFound
	}
	if sn.Status != domain.SenderNameStatusApproved {
		return domain.ErrSenderNameNotApproved
	}
	return nil
}

func (s *SenderNameService) addHistory(ctx context.Context, senderNameID uuid.UUID, oldStatus *string, newStatus string, actorID *uuid.UUID, actorType, comment string) error {
	return s.repo.AddHistoryEntry(ctx, &domain.SenderNameStatusHistory{
		ID:           uuid.New(),
		SenderNameID: senderNameID,
		OldStatus:    oldStatus,
		NewStatus:    newStatus,
		ActorID:      actorID,
		ActorType:    actorType,
		Comment:      comment,
	})
}
