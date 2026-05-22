package domain

import "errors"

var (
	ErrDeliveryNotFound            = errors.New("delivery not found")
	ErrStrategyNotFound            = errors.New("strategy not found")
	ErrChannelNotFound             = errors.New("channel not found")
	ErrAttemptNotFound             = errors.New("attempt not found")
	ErrInvalidTransition           = errors.New("invalid status transition")
	ErrLateDuplicate               = errors.New("late duplicate delivery")
	ErrChannelNotSupported         = errors.New("channel not supported by operator")
	ErrStrategyHasActiveDeliveries = errors.New("strategy has active deliveries")
	ErrDuplicateChannelType        = errors.New("channel type already exists")
	ErrDuplicateStrategyName       = errors.New("strategy name already exists")
	ErrAttemptAlreadyExists        = errors.New("attempt already exists for this step")
	ErrDeliveryConflict            = errors.New("delivery status conflict: concurrent update")
)
