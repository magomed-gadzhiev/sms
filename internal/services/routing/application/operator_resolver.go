package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

// ResolveResult содержит результат определения оператора по номеру
type ResolveResult struct {
	OperatorID   uuid.UUID
	CountryID    uuid.UUID
	OperatorCode string
	CountryCode  string
	Currency     string
	ResolvedBy   string
}

// OperatorResolver определяет оператора по номеру телефона
type OperatorResolver struct {
	prefixRepo  domain.OperatorPrefixRepository
	operatorRepo domain.OperatorRepository
	countryRepo  domain.CountryRepository
	logger       zerolog.Logger
}

// NewOperatorResolver создает новый резолвер операторов
func NewOperatorResolver(
	prefixRepo domain.OperatorPrefixRepository,
	operatorRepo domain.OperatorRepository,
	countryRepo domain.CountryRepository,
) *OperatorResolver {
	return &OperatorResolver{
		prefixRepo:   prefixRepo,
		operatorRepo: operatorRepo,
		countryRepo:  countryRepo,
		logger:       log.With().Str("component", "operator-resolver").Logger(),
	}
}

// ResolveByNumber определяет оператора по номеру телефона
func (r *OperatorResolver) ResolveByNumber(ctx context.Context, phoneNumber string) (*ResolveResult, error) {
	// Находим подходящий префикс
	prefix, err := r.prefixRepo.FindByNumber(ctx, phoneNumber)
	if err != nil {
		return nil, fmt.Errorf("ошибка поиска префикса: %w", err)
	}

	// Получаем оператора
	operator, err := r.operatorRepo.GetByID(ctx, prefix.OperatorID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения оператора: %w", err)
	}

	// Получаем страну
	country, err := r.countryRepo.GetByID(ctx, operator.CountryID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения страны: %w", err)
	}

	r.logger.Debug().
		Str("phone_number", phoneNumber).
		Str("operator_code", operator.Code).
		Str("country_code", country.ISOCode).
		Msg("оператор определен по префиксу")

	return &ResolveResult{
		OperatorID:   operator.ID,
		CountryID:    country.ID,
		OperatorCode: operator.Code,
		CountryCode:  country.ISOCode,
		Currency:     country.Currency,
		ResolvedBy:   "prefix",
	}, nil
}
