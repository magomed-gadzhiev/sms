package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// SenderService предоставляет бизнес-логику для управления регистрациями отправителей
type SenderService struct {
	regRepo domain.SenderRegistrationRepository
	logger  zerolog.Logger
}

// NewSenderService создает новый сервис регистраций отправителей
func NewSenderService(regRepo domain.SenderRegistrationRepository) *SenderService {
	return &SenderService{
		regRepo: regRepo,
		logger:  log.With().Str("component", "sender-service").Logger(),
	}
}

// CreateRegistration создает новую регистрацию отправителя
func (s *SenderService) CreateRegistration(ctx context.Context, clientID, operatorID uuid.UUID, senderName string, regType domain.SenderRegistrationType) (*domain.SenderRegistration, error) {
	reg := domain.NewSenderRegistration(clientID, operatorID, senderName, regType)
	if err := reg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid sender registration: %w", err)
	}

	// Проверяем дубликат активной регистрации
	existing, err := s.regRepo.GetActiveByClientOperatorName(ctx, clientID, operatorID, senderName)
	if err == nil && existing != nil {
		return nil, domain.ErrSenderRegistrationDuplicate
	}

	if err := s.regRepo.Create(ctx, reg); err != nil {
		return nil, fmt.Errorf("failed to create sender registration: %w", err)
	}

	s.logger.Info().
		Str("registration_id", reg.ID.String()).
		Str("client_id", clientID.String()).
		Str("operator_id", operatorID.String()).
		Str("sender_name", senderName).
		Str("type", string(regType)).
		Msg("sender registration created")

	return reg, nil
}

// UpdateRegistration обновляет регистрацию отправителя
func (s *SenderService) UpdateRegistration(ctx context.Context, id uuid.UUID, status domain.SenderRegistrationStatus, regType *domain.SenderRegistrationType) (*domain.SenderRegistration, error) {
	reg, err := s.regRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get sender registration: %w", err)
	}

	// Валидируем статус
	switch status {
	case domain.SenderStatusActive, domain.SenderStatusPending, domain.SenderStatusExpired:
	default:
		return nil, domain.ErrSenderRegistrationInvalidStatus
	}

	reg.Status = status
	if regType != nil {
		reg.Type = *regType
	}
	reg.UpdatedAt = time.Now()

	if err := s.regRepo.Update(ctx, reg); err != nil {
		return nil, fmt.Errorf("failed to update sender registration: %w", err)
	}

	s.logger.Info().
		Str("registration_id", id.String()).
		Str("status", string(status)).
		Msg("sender registration updated")

	return reg, nil
}

// DetermineSenderCategory определяет категорию отправителя
func (s *SenderService) DetermineSenderCategory(ctx context.Context, clientID, operatorID uuid.UUID, senderName string) (domain.SenderCategory, error) {
	// Если имя отправителя пустое — shared
	if senderName == "" {
		return domain.CategoryShared, nil
	}

	// Ищем активную регистрацию
	reg, err := s.regRepo.GetActiveByClientOperatorName(ctx, clientID, operatorID, senderName)
	if err != nil {
		// Если регистрация не найдена — shared
		if err == domain.ErrSenderRegistrationNotFound {
			return domain.CategoryShared, nil
		}
		return "", fmt.Errorf("failed to get sender registration: %w", err)
	}

	// Определяем категорию по типу регистрации
	switch reg.Type {
	case domain.SenderTypePaid:
		return domain.CategoryPaidRegistered, nil
	case domain.SenderTypeFree:
		return domain.CategoryFreeRegistered, nil
	default:
		return domain.CategoryShared, nil
	}
}

// ListRegistrations получает список регистраций отправителей
func (s *SenderService) ListRegistrations(ctx context.Context, clientID, operatorID *uuid.UUID, limit, offset int) ([]*domain.SenderRegistration, int, error) {
	regs, total, err := s.regRepo.List(ctx, clientID, operatorID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list sender registrations: %w", err)
	}
	return regs, total, nil
}

// GetRegistration получает регистрацию отправителя по ID
func (s *SenderService) GetRegistration(ctx context.Context, id uuid.UUID) (*domain.SenderRegistration, error) {
	reg, err := s.regRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get sender registration: %w", err)
	}
	return reg, nil
}
