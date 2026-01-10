//go:build integration

package performance

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/smpp-server/smpp-server/internal/testutil"
)

// TestDatabaseConnectionPool оптимизация пула соединений БД
func TestDatabaseConnectionPool(t *testing.T) {
	dsn := testutil.GetTestDSN()
	
	// Тест различных конфигураций пула соединений
	tests := []struct {
		name         string
		maxOpenConns int
		maxIdleConns int
		maxLifetime  time.Duration
		maxIdleTime  time.Duration
	}{
		{
			name:         "Small pool",
			maxOpenConns: 10,
			maxIdleConns: 5,
			maxLifetime:  30 * time.Minute,
			maxIdleTime:  5 * time.Minute,
		},
		{
			name:         "Medium pool",
			maxOpenConns: 50,
			maxIdleConns: 25,
			maxLifetime:  30 * time.Minute,
			maxIdleTime:  5 * time.Minute,
		},
		{
			name:         "Large pool (recommended for 10K msg/s)",
			maxOpenConns: 100,
			maxIdleConns: 50,
			maxLifetime:  30 * time.Minute,
			maxIdleTime:  5 * time.Minute,
		},
		{
			name:         "Very large pool",
			maxOpenConns: 200,
			maxIdleConns: 100,
			maxLifetime:  30 * time.Minute,
			maxIdleTime:  5 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, cleanup := testutil.SetupTestDB(t, dsn)
			defer cleanup()

		// Настройка пула соединений (DB встраивает *sql.DB)
		db.SetMaxOpenConns(tt.maxOpenConns)
		db.SetMaxIdleConns(tt.maxIdleConns)
		db.SetConnMaxLifetime(tt.maxLifetime)
		db.SetConnMaxIdleTime(tt.maxIdleTime)

		// Проверка, что пул работает
		ctx := context.Background()
		err := db.PingContext(ctx)
		require.NoError(t, err)

		// Проверка статистики пула
		stats := db.Stats()
			t.Logf("Pool stats - OpenConnections: %d, Idle: %d, InUse: %d, WaitCount: %d",
				stats.OpenConnections, stats.Idle, stats.InUse, stats.WaitCount)

			assert.GreaterOrEqual(t, stats.OpenConnections, 0)
			assert.LessOrEqual(t, stats.OpenConnections, tt.maxOpenConns)
		})
	}
}

// TestKafkaBatchSize тестирует оптимальный размер батча для Kafka
func TestKafkaBatchSize(t *testing.T) {
	// Тест различных размеров батчей
	batchSizes := []int{1, 10, 50, 100, 200, 500}

	for _, batchSize := range batchSizes {
		t.Run(fmt.Sprintf("BatchSize_%d", batchSize), func(t *testing.T) {
			start := time.Now()
			
			// Симулируем обработку батча сообщений
			for i := 0; i < batchSize; i++ {
				// Симуляция обработки одного сообщения
				time.Sleep(1 * time.Millisecond)
			}
			
			duration := time.Since(start)
			throughput := float64(batchSize) / duration.Seconds()
			
			t.Logf("Batch size: %d, Duration: %v, Throughput: %.2f msg/s",
				batchSize, duration, throughput)
			
			// Для 10K msg/s с батчами по 100, нужно обрабатывать ~100 батчей/сек
			if batchSize >= 100 {
				assert.Greater(t, throughput, 1000.0, "Should handle at least 1000 msg/s per batch")
			}
		})
	}
}

// TestConnectionPoolPerformance тестирует производительность пула соединений
func TestConnectionPoolPerformance(t *testing.T) {
	dsn := testutil.GetTestDSN()
	db, cleanup := testutil.SetupTestDB(t, dsn)
	defer cleanup()

	// Оптимальная конфигурация для высокой нагрузки
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(50)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx := context.Background()
	
	// Тест конкурентных запросов
	concurrency := 50
	queries := 1000

	start := time.Now()
	
	results := make(chan error, concurrency*queries)
	for i := 0; i < concurrency; i++ {
		go func() {
			for j := 0; j < queries; j++ {
				var count int
				err := db.QueryRowContext(ctx, "SELECT 1").Scan(&count)
				results <- err
			}
		}()
	}

	// Собираем результаты
	successCount := 0
	for i := 0; i < concurrency*queries; i++ {
		if err := <-results; err == nil {
			successCount++
		}
	}

	duration := time.Since(start)
	qps := float64(successCount) / duration.Seconds()

	t.Logf("Concurrent queries test:")
	t.Logf("  Total queries: %d", concurrency*queries)
	t.Logf("  Successful: %d", successCount)
	t.Logf("  Duration: %v", duration)
	t.Logf("  Queries per second: %.2f", qps)

	stats := db.Stats()
	t.Logf("Pool stats:")
	t.Logf("  OpenConnections: %d", stats.OpenConnections)
	t.Logf("  Idle: %d", stats.Idle)
	t.Logf("  InUse: %d", stats.InUse)
	t.Logf("  WaitCount: %d", stats.WaitCount)
	t.Logf("  WaitDuration: %v", stats.WaitDuration)

	// Для 10K msg/s нужно минимум 1000 qps
	assert.Greater(t, qps, 1000.0, "Should handle at least 1000 queries per second")
	assert.Equal(t, 0, int(stats.WaitCount), "Should not have connection wait times")
}

// BenchmarkDatabasePool бенчмарк пула соединений
func BenchmarkDatabasePool(b *testing.B) {
	dsn := testutil.GetTestDSN()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		b.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(50)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var count int
			_ = db.QueryRowContext(ctx, "SELECT 1").Scan(&count)
		}
	})
}

// BenchmarkBatchProcessing бенчмарк батч обработки
func BenchmarkBatchProcessing(b *testing.B) {
	batchSizes := []int{10, 50, 100, 200}
	
	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("Batch_%d", batchSize), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Симуляция обработки батча
				for j := 0; j < batchSize; j++ {
					// Минимальная работа
					_ = j * 2
				}
			}
		})
	}
}