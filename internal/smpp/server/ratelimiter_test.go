package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRateLimiter_DefaultsForZero(t *testing.T) {
	// perSecond <= 0 должен использовать дефолтное значение 100
	rl := NewRateLimiter(0)
	require.NotNil(t, rl)
	assert.Equal(t, 100, rl.maxTokens)
	assert.Equal(t, 100, rl.tokens)
}

func TestNewRateLimiter_DefaultsForNegative(t *testing.T) {
	rl := NewRateLimiter(-5)
	require.NotNil(t, rl)
	assert.Equal(t, 100, rl.maxTokens)
}

func TestNewRateLimiter_PositiveRate(t *testing.T) {
	rl := NewRateLimiter(10)
	require.NotNil(t, rl)
	assert.Equal(t, 10, rl.maxTokens)
	assert.Equal(t, 10, rl.tokens)
	// rate = time.Second / 10 = 100ms
	assert.Equal(t, 100*time.Millisecond, rl.rate)
}

func TestRateLimiter_AllowUpToLimit(t *testing.T) {
	const limit = 5
	rl := NewRateLimiter(limit)

	// Должны пройти ровно limit запросов
	for i := 0; i < limit; i++ {
		err := rl.Allow()
		assert.NoError(t, err, "запрос %d должен быть разрешён", i+1)
	}
}

func TestRateLimiter_BlockWhenExhausted(t *testing.T) {
	const limit = 3
	rl := NewRateLimiter(limit)

	// Исчерпываем токены
	for i := 0; i < limit; i++ {
		require.NoError(t, rl.Allow())
	}

	// Следующий запрос должен быть заблокирован
	err := rl.Allow()
	assert.ErrorIs(t, err, ErrRateLimitExceeded)
}

func TestRateLimiter_RefillOverTime(t *testing.T) {
	// Устанавливаем 2 токена/с, rate = 500ms
	rl := NewRateLimiter(2)

	// Исчерпываем оба токена
	require.NoError(t, rl.Allow())
	require.NoError(t, rl.Allow())

	// Токены закончились
	assert.ErrorIs(t, rl.Allow(), ErrRateLimitExceeded)

	// Ждём, чтобы появился хотя бы один токен (rate = 500ms)
	time.Sleep(600 * time.Millisecond)

	// Теперь должен пройти хотя бы один запрос
	err := rl.Allow()
	assert.NoError(t, err, "после рефилла токен должен быть доступен")
}

func TestRateLimiter_RefillCappedAtMax(t *testing.T) {
	const limit = 3
	rl := NewRateLimiter(limit)

	// Исчерпываем все токены
	for i := 0; i < limit; i++ {
		require.NoError(t, rl.Allow())
	}

	// Ждём достаточно долго, чтобы заполниться сверх лимита (проверяем кэпинг)
	time.Sleep(time.Duration(limit+2) * (time.Second / time.Duration(limit)))

	// После рефилла — максимум limit токенов, не больше
	for i := 0; i < limit; i++ {
		err := rl.Allow()
		assert.NoError(t, err, "запрос %d после рефилла должен пройти", i+1)
	}

	// (limit+1)-й запрос должен быть заблокирован
	assert.ErrorIs(t, rl.Allow(), ErrRateLimitExceeded)
}

func TestRateLimiter_SetRate(t *testing.T) {
	rl := NewRateLimiter(10)

	// Исчерпываем несколько токенов
	for i := 0; i < 5; i++ {
		require.NoError(t, rl.Allow())
	}

	// Меняем лимит — должны сброситься токены
	rl.SetRate(3)
	assert.Equal(t, 3, rl.maxTokens)
	assert.Equal(t, 3, rl.tokens)

	// Теперь работаем с новым лимитом
	for i := 0; i < 3; i++ {
		assert.NoError(t, rl.Allow())
	}
	assert.ErrorIs(t, rl.Allow(), ErrRateLimitExceeded)
}

func TestRateLimiter_SetRateZeroUsesDefault(t *testing.T) {
	rl := NewRateLimiter(5)
	rl.SetRate(0)
	assert.Equal(t, 100, rl.maxTokens)
	assert.Equal(t, 100, rl.tokens)
}

func TestRateLimiter_ConcurrentAllow(t *testing.T) {
	const limit = 50
	rl := NewRateLimiter(limit)

	allowed := make(chan struct{}, limit*2)

	// Запускаем 2*limit горутин одновременно
	done := make(chan struct{})
	for i := 0; i < limit*2; i++ {
		go func() {
			if err := rl.Allow(); err == nil {
				allowed <- struct{}{}
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < limit*2; i++ {
		<-done
	}
	close(allowed)

	count := len(allowed)
	// Разрешено не больше limit запросов
	assert.LessOrEqual(t, count, limit, "разрешено не должно превышать лимит")
}
