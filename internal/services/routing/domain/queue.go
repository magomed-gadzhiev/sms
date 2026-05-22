package domain

import (
	"context"

	"github.com/google/uuid"
)

// EventPublisher определяет интерфейс для публикации событий
type EventPublisher interface {
	PublishMessageRouted(ctx context.Context, messageID uuid.UUID, routeID uuid.UUID, providerID uuid.UUID) error
}

// EventConsumer определяет интерфейс для подписки на события
type EventConsumer interface {
	ConsumeMessageQueued(ctx context.Context, handler MessageQueuedHandler) error
}

// MessageQueuedHandler обрабатывает событие message.queued
type MessageQueuedHandler func(ctx context.Context, messageID uuid.UUID, destination string, clientID *uuid.UUID, routeID *uuid.UUID, providerID *uuid.UUID) error