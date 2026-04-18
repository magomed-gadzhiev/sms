package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

// AggregatorResolver разрешает subaccount_id → aggregator_id (parent_client_id)
// с Redis-кешем. Нужен PriceResolver-у для формирования WHERE clause в lookup.
// Read-through cache: miss → DB → записать в Redis.
type AggregatorResolver struct {
	db     *sqlx.DB
	redis  *redis.Client
	ttlSec int
}

func NewAggregatorResolver(db *sqlx.DB, rdb *redis.Client, ttlSeconds int) *AggregatorResolver {
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	return &AggregatorResolver{db: db, redis: rdb, ttlSec: ttlSeconds}
}

const aggregatorLookupSQL = `
SELECT parent_client_id
FROM clients
WHERE id = $1;
`

// AggregatorFor возвращает aggregator_id (parent_client_id). Если у клиента
// нет родителя — это top-level клиент (сам агрегатор), возвращаем его же id.
// Это разумно для fallback-поиска: тарифы для top-level агрегатора хранятся как
// price_rules.owner_type='aggregator' с owner_id=этот id.
func (r *AggregatorResolver) AggregatorFor(ctx context.Context, subID uuid.UUID) (uuid.UUID, error) {
	key := fmt.Sprintf("client:%s:aggregator_id", subID)

	if r.redis != nil {
		if v, err := r.redis.Get(ctx, key).Result(); err == nil {
			if id, perr := uuid.Parse(v); perr == nil {
				return id, nil
			}
		} else if !errors.Is(err, redis.Nil) {
			// transient Redis-ошибка — не критично, идём в DB
		}
	}

	var parent uuid.NullUUID
	err := r.db.GetContext(ctx, &parent, aggregatorLookupSQL, subID)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("client %s not found", subID)
	}
	if err != nil {
		return uuid.Nil, err
	}
	aggID := subID
	if parent.Valid {
		aggID = parent.UUID
	}

	if r.redis != nil {
		// best-effort write; ошибка записи не должна ломать горячий путь
		_ = r.redis.Set(ctx, key, aggID.String(), time.Duration(r.ttlSec)*time.Second).Err()
	}
	return aggID, nil
}
