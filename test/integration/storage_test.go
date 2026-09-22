//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageRepository_Create(t *testing.T) {
	// Этот тест требует запущенной PostgreSQL
	// Используется testcontainers или внешняя БД
	dsn := testutil.GetTestDSN()
	db, cleanup := testutil.SetupTestDB(t, dsn)
	defer cleanup()

	testutil.CleanupTestDB(t, db)

	repo := storage.NewMessageRepository(db)
	ctx := context.Background()

	msg := testutil.NewTestMessage()
	msg.ID = uuid.New()
	seededClientID := testutil.SeedTestClient(t, db)
	msg.ClientID = &seededClientID

	err := repo.Create(ctx, msg)
	require.NoError(t, err)
	assert.NotEmpty(t, msg.ID)
}

func TestMessageRepository_GetByID(t *testing.T) {
	dsn := testutil.GetTestDSN()
	db, cleanup := testutil.SetupTestDB(t, dsn)
	defer cleanup()

	testutil.CleanupTestDB(t, db)

	repo := storage.NewMessageRepository(db)
	ctx := context.Background()

	// Создаем сообщение
	msg := testutil.NewTestMessage()
	msg.ID = uuid.New()
	seededClientID := testutil.SeedTestClient(t, db)
	msg.ClientID = &seededClientID
	err := repo.Create(ctx, msg)
	require.NoError(t, err)

	// Получаем сообщение
	retrieved, err := repo.GetByID(ctx, msg.ID)
	require.NoError(t, err)
	assert.Equal(t, msg.ID, retrieved.ID)
	assert.Equal(t, msg.Source, retrieved.Source)
	assert.Equal(t, msg.Destination, retrieved.Destination)
	assert.Equal(t, msg.Text, retrieved.Text)
}

func TestMessageRepository_UpdateStatus(t *testing.T) {
	dsn := testutil.GetTestDSN()
	db, cleanup := testutil.SetupTestDB(t, dsn)
	defer cleanup()

	testutil.CleanupTestDB(t, db)

	repo := storage.NewMessageRepository(db)
	ctx := context.Background()

	// Создаем сообщение
	msg := testutil.NewTestMessage()
	msg.ID = uuid.New()
	seededClientID := testutil.SeedTestClient(t, db)
	msg.ClientID = &seededClientID
	err := repo.Create(ctx, msg)
	require.NoError(t, err)

	// Обновляем статус
	err = repo.UpdateStatus(ctx, msg.ID, shared.MessageStatusSent, "")
	require.NoError(t, err)

	// Проверяем обновление
	updated, err := repo.GetByID(ctx, msg.ID)
	require.NoError(t, err)
	assert.Equal(t, shared.MessageStatusSent, updated.Status)
	assert.NotNil(t, updated.SubmittedAt)
}

func TestClientRepository_CreateAndGet(t *testing.T) {
	dsn := testutil.GetTestDSN()
	db, cleanup := testutil.SetupTestDB(t, dsn)
	defer cleanup()

	testutil.CleanupTestDB(t, db)

	repo := storage.NewClientRepository(db)
	ctx := context.Background()

	client := testutil.NewTestClient()
	client.ID = uuid.New()

	err := repo.Create(ctx, client)
	require.NoError(t, err)

	// Получаем по ID
	retrieved, err := repo.GetByID(ctx, client.ID)
	require.NoError(t, err)
	assert.Equal(t, client.ID, retrieved.ID)
	assert.Equal(t, client.Name, retrieved.Name)
	assert.Equal(t, client.APIKey, retrieved.APIKey)

	// Получаем по API ключу
	retrievedByKey, err := repo.GetByAPIKey(ctx, client.APIKey)
	require.NoError(t, err)
	assert.Equal(t, client.ID, retrievedByKey.ID)
}

func TestProviderRepository_CreateAndGet(t *testing.T) {
	dsn := testutil.GetTestDSN()
	db, cleanup := testutil.SetupTestDB(t, dsn)
	defer cleanup()

	testutil.CleanupTestDB(t, db)

	repo := storage.NewProviderRepository(db)
	ctx := context.Background()

	provider := testutil.NewTestProvider()
	provider.ID = uuid.New()

	err := repo.Create(ctx, provider)
	require.NoError(t, err)

	// Получаем по ID
	retrieved, err := repo.GetByID(ctx, provider.ID)
	require.NoError(t, err)
	assert.Equal(t, provider.ID, retrieved.ID)
	assert.Equal(t, provider.Name, retrieved.Name)
	assert.Equal(t, provider.Host, retrieved.Host)
	assert.Equal(t, provider.Port, retrieved.Port)

	// Получаем по имени
	retrievedByName, err := repo.GetByName(ctx, provider.Name)
	require.NoError(t, err)
	assert.Equal(t, provider.ID, retrievedByName.ID)
}

func TestMessageRepository_GetPendingForRetry(t *testing.T) {
	dsn := testutil.GetTestDSN()
	db, cleanup := testutil.SetupTestDB(t, dsn)
	defer cleanup()

	testutil.CleanupTestDB(t, db)

	repo := storage.NewMessageRepository(db)
	ctx := context.Background()

	// Создаем сообщение со статусом failed и next_retry_at в прошлом
	msg := testutil.NewTestMessage()
	msg.ID = uuid.New()
	seededClientID := testutil.SeedTestClient(t, db)
	msg.ClientID = &seededClientID
	msg.Status = shared.MessageStatusFailed
	msg.RetryCount = 2
	msg.MaxRetries = 5
	now := time.Now()
	msg.NextRetryAt = &now
	err := repo.Create(ctx, msg)
	require.NoError(t, err)

	// Получаем сообщения для retry
	pending, err := repo.GetPendingForRetry(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, msg.ID, pending[0].ID)
}

func TestMessageRepository_IncrementRetryCount(t *testing.T) {
	dsn := testutil.GetTestDSN()
	db, cleanup := testutil.SetupTestDB(t, dsn)
	defer cleanup()

	testutil.CleanupTestDB(t, db)

	repo := storage.NewMessageRepository(db)
	ctx := context.Background()

	// Создаем сообщение
	msg := testutil.NewTestMessage()
	msg.ID = uuid.New()
	seededClientID := testutil.SeedTestClient(t, db)
	msg.ClientID = &seededClientID
	msg.RetryCount = 0
	err := repo.Create(ctx, msg)
	require.NoError(t, err)

	// Увеличиваем счетчик retry
	nextRetryAt := time.Now().Add(time.Minute)
	err = repo.IncrementRetryCount(ctx, msg.ID, nextRetryAt)
	require.NoError(t, err)

	// Проверяем обновление
	updated, err := repo.GetByID(ctx, msg.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, updated.RetryCount)
	assert.NotNil(t, updated.NextRetryAt)
}
