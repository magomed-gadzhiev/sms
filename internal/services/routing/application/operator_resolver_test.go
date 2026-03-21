package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/smpp-server/smpp-server/internal/services/routing/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestOperatorResolver(t *testing.T) {

	t.Run("ResolveByNumber", func(t *testing.T) {

		t.Run("known_prefix", func(t *testing.T) {
			prefixRepo := new(mocks.MockOperatorPrefixRepository)
			operatorRepo := new(mocks.MockOperatorRepository)
			countryRepo := new(mocks.MockCountryRepository)

			resolver := NewOperatorResolver(prefixRepo, operatorRepo, countryRepo)

			countryID := uuid.New()
			operatorID := uuid.New()
			prefixID := uuid.New()

			prefix := &domain.OperatorPrefix{
				ID:         prefixID,
				OperatorID: operatorID,
				Prefix:     "+7900",
				Priority:   1,
			}

			operator := &domain.Operator{
				ID:        operatorID,
				CountryID: countryID,
				Name:      "MTS",
				Code:      "mts-ru",
				Active:    true,
			}

			country := &domain.Country{
				ID:        countryID,
				Name:      "Russia",
				ISOCode:   "RU",
				PhoneCode: "+7",
				Currency:  "RUB",
			}

			prefixRepo.On("FindByNumber", mock.Anything, "79001234567").Return(prefix, nil)
			operatorRepo.On("GetByID", mock.Anything, operatorID).Return(operator, nil)
			countryRepo.On("GetByID", mock.Anything, countryID).Return(country, nil)

			result, err := resolver.ResolveByNumber(context.Background(), "79001234567")

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, operatorID, result.OperatorID)
			assert.Equal(t, countryID, result.CountryID)
			assert.Equal(t, "mts-ru", result.OperatorCode)
			assert.Equal(t, "RU", result.CountryCode)
			assert.Equal(t, "RUB", result.Currency)
			assert.Equal(t, "prefix", result.ResolvedBy)

			prefixRepo.AssertExpectations(t)
			operatorRepo.AssertExpectations(t)
			countryRepo.AssertExpectations(t)
		})

		t.Run("unknown_prefix", func(t *testing.T) {
			prefixRepo := new(mocks.MockOperatorPrefixRepository)
			operatorRepo := new(mocks.MockOperatorRepository)
			countryRepo := new(mocks.MockCountryRepository)

			resolver := NewOperatorResolver(prefixRepo, operatorRepo, countryRepo)

			prefixRepo.On("FindByNumber", mock.Anything, "99999999999").
				Return(nil, domain.ErrPrefixNotFound)

			result, err := resolver.ResolveByNumber(context.Background(), "99999999999")

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "ошибка поиска префикса")

			// Operator and country repos should NOT be called
			operatorRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
			countryRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
		})

		t.Run("operator_not_found", func(t *testing.T) {
			prefixRepo := new(mocks.MockOperatorPrefixRepository)
			operatorRepo := new(mocks.MockOperatorRepository)
			countryRepo := new(mocks.MockCountryRepository)

			resolver := NewOperatorResolver(prefixRepo, operatorRepo, countryRepo)

			operatorID := uuid.New()
			prefix := &domain.OperatorPrefix{
				ID:         uuid.New(),
				OperatorID: operatorID,
				Prefix:     "+7900",
				Priority:   1,
			}

			prefixRepo.On("FindByNumber", mock.Anything, "79001234567").Return(prefix, nil)
			operatorRepo.On("GetByID", mock.Anything, operatorID).
				Return(nil, domain.ErrOperatorNotFound)

			result, err := resolver.ResolveByNumber(context.Background(), "79001234567")

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "ошибка получения оператора")

			// Country repo should NOT be called
			countryRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
		})

		t.Run("country_not_found", func(t *testing.T) {
			prefixRepo := new(mocks.MockOperatorPrefixRepository)
			operatorRepo := new(mocks.MockOperatorRepository)
			countryRepo := new(mocks.MockCountryRepository)

			resolver := NewOperatorResolver(prefixRepo, operatorRepo, countryRepo)

			countryID := uuid.New()
			operatorID := uuid.New()

			prefix := &domain.OperatorPrefix{
				ID:         uuid.New(),
				OperatorID: operatorID,
				Prefix:     "+7900",
				Priority:   1,
			}

			operator := &domain.Operator{
				ID:        operatorID,
				CountryID: countryID,
				Name:      "MTS",
				Code:      "mts-ru",
				Active:    true,
			}

			prefixRepo.On("FindByNumber", mock.Anything, "79001234567").Return(prefix, nil)
			operatorRepo.On("GetByID", mock.Anything, operatorID).Return(operator, nil)
			countryRepo.On("GetByID", mock.Anything, countryID).
				Return(nil, domain.ErrCountryNotFound)

			result, err := resolver.ResolveByNumber(context.Background(), "79001234567")

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "ошибка получения страны")
		})

		t.Run("prefix_repo_db_error", func(t *testing.T) {
			prefixRepo := new(mocks.MockOperatorPrefixRepository)
			operatorRepo := new(mocks.MockOperatorRepository)
			countryRepo := new(mocks.MockCountryRepository)

			resolver := NewOperatorResolver(prefixRepo, operatorRepo, countryRepo)

			prefixRepo.On("FindByNumber", mock.Anything, "79001234567").
				Return(nil, errors.New("db connection lost"))

			result, err := resolver.ResolveByNumber(context.Background(), "79001234567")

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "ошибка поиска префикса")
		})
	})
}
