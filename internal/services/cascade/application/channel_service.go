package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
)

// ChannelService управляет каналами доставки
type ChannelService struct {
	channels domain.ChannelRepository
}

func NewChannelService(channels domain.ChannelRepository) *ChannelService {
	return &ChannelService{channels: channels}
}

func (s *ChannelService) List(ctx context.Context) ([]*domain.ChannelConfig, error) {
	return s.channels.List(ctx)
}

func (s *ChannelService) Get(ctx context.Context, id uuid.UUID) (*domain.ChannelConfig, error) {
	return s.channels.Get(ctx, id)
}

func (s *ChannelService) Create(ctx context.Context, channelType domain.ChannelType, name, description string, config map[string]interface{}) (*domain.ChannelConfig, error) {
	if !channelType.IsValid() {
		return nil, fmt.Errorf("invalid channel type: %s", channelType)
	}
	if name == "" {
		return nil, fmt.Errorf("channel name is required")
	}

	ch := domain.NewChannelConfig(channelType, name, description, config)
	if err := s.channels.Create(ctx, ch); err != nil {
		return nil, fmt.Errorf("create channel: %w", err)
	}
	return ch, nil
}

func (s *ChannelService) Update(ctx context.Context, id uuid.UUID, name, description string, config map[string]interface{}) (*domain.ChannelConfig, error) {
	ch, err := s.channels.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if name != "" {
		ch.Name = name
	}
	if description != "" {
		ch.Description = description
	}
	if config != nil {
		ch.Config = config
	}
	if err := s.channels.Update(ctx, ch); err != nil {
		return nil, fmt.Errorf("update channel: %w", err)
	}
	return ch, nil
}

func (s *ChannelService) Toggle(ctx context.Context, id uuid.UUID, active bool) (*domain.ChannelConfig, error) {
	if err := s.channels.Toggle(ctx, id, active); err != nil {
		return nil, fmt.Errorf("toggle channel: %w", err)
	}
	return s.channels.Get(ctx, id)
}
