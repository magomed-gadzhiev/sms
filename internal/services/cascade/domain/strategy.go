package domain

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// StrategyMode представляет режим выполнения стратегии
type StrategyMode string

const (
	ModeSequential StrategyMode = "sequential"
	ModeParallel   StrategyMode = "parallel"
)

// IsValid проверяет, является ли режим допустимым
func (sm StrategyMode) IsValid() bool {
	return sm == ModeSequential || sm == ModeParallel
}

// DeliveryStrategy представляет стратегию каскадной доставки
type DeliveryStrategy struct {
	ID          uuid.UUID
	Name        string
	Description string
	Mode        StrategyMode
	Steps       []StrategyStep
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// StrategyStep представляет шаг стратегии доставки
type StrategyStep struct {
	ID          uuid.UUID
	StrategyID  uuid.UUID
	ChannelID   uuid.UUID
	ChannelType ChannelType
	ChannelName string
	StepOrder   int
	TimeoutS    int
	Billable    bool
	CreatedAt   time.Time
}

// NewDeliveryStrategy создает новую стратегию доставки
func NewDeliveryStrategy(name, description string, mode StrategyMode) *DeliveryStrategy {
	now := time.Now()
	return &DeliveryStrategy{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		Mode:        mode,
		Steps:       make([]StrategyStep, 0),
		Active:      true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// GetStepByOrder возвращает шаг по порядковому номеру
func (ds *DeliveryStrategy) GetStepByOrder(order int) *StrategyStep {
	for i := range ds.Steps {
		if ds.Steps[i].StepOrder == order {
			return &ds.Steps[i]
		}
	}
	return nil
}

// NextStep возвращает следующий шаг после указанного порядкового номера
func (ds *DeliveryStrategy) NextStep(currentOrder int) *StrategyStep {
	sorted := make([]StrategyStep, len(ds.Steps))
	copy(sorted, ds.Steps)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StepOrder < sorted[j].StepOrder
	})

	for i := range sorted {
		if sorted[i].StepOrder > currentOrder {
			return &sorted[i]
		}
	}
	return nil
}

// Validate проверяет корректность стратегии
func (ds *DeliveryStrategy) Validate() error {
	if ds.Name == "" {
		return fmt.Errorf("strategy name is required")
	}
	if !ds.Mode.IsValid() {
		return fmt.Errorf("invalid strategy mode: %s", ds.Mode)
	}
	if len(ds.Steps) == 0 {
		return fmt.Errorf("strategy must have at least one step")
	}

	orders := make(map[int]bool)
	for _, step := range ds.Steps {
		if orders[step.StepOrder] {
			return fmt.Errorf("duplicate step order: %d", step.StepOrder)
		}
		orders[step.StepOrder] = true
	}

	return nil
}

// AddStep добавляет шаг в стратегию
func (ds *DeliveryStrategy) AddStep(channelID uuid.UUID, channelType ChannelType, channelName string, stepOrder, timeoutS int, billable bool) {
	step := StrategyStep{
		ID:          uuid.New(),
		StrategyID:  ds.ID,
		ChannelID:   channelID,
		ChannelType: channelType,
		ChannelName: channelName,
		StepOrder:   stepOrder,
		TimeoutS:    timeoutS,
		Billable:    billable,
		CreatedAt:   time.Now(),
	}
	ds.Steps = append(ds.Steps, step)
	ds.UpdatedAt = time.Now()
}
