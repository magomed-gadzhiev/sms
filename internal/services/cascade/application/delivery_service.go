package application

import (
	"context"

	"github.com/google/uuid"

	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
)

// DeliveryService предоставляет доступ к доставкам для клиентского API
type DeliveryService struct {
	deliveries domain.DeliveryRepository
	attempts   domain.AttemptRepository
}

func NewDeliveryService(deliveries domain.DeliveryRepository, attempts domain.AttemptRepository) *DeliveryService {
	return &DeliveryService{
		deliveries: deliveries,
		attempts:   attempts,
	}
}

func (s *DeliveryService) GetDelivery(ctx context.Context, deliveryID, clientID uuid.UUID) (*domain.Delivery, error) {
	delivery, err := s.deliveries.GetByClientID(ctx, deliveryID, clientID)
	if err != nil {
		return nil, err
	}

	attempts, err := s.attempts.ListByDelivery(ctx, deliveryID)
	if err != nil {
		return nil, err
	}
	for _, a := range attempts {
		delivery.Attempts = append(delivery.Attempts, *a)
	}
	return delivery, nil
}

func (s *DeliveryService) ListDeliveries(ctx context.Context, filter domain.DeliveryFilter) ([]*domain.Delivery, int, error) {
	return s.deliveries.List(ctx, filter)
}

func (s *DeliveryService) GetStats(ctx context.Context, filter domain.StatsFilter) (*domain.DeliveryStats, error) {
	return s.deliveries.Stats(ctx, filter)
}
