# Многомерная аналитика — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Расширить страницу аналитики клиентского портала независимыми фильтрами по провайдеру, отправителю, кампании, статусу и drill-down до списка сообщений.

**Architecture:** Добавляем `campaign_id` и `country_code` в таблицу `messages`; расширяем proto двумя новыми методами (`GetMessagesList`, `GetFilterOptions`); расширяем SQL в репозитории; добавляем два новых HTTP-эндпоинта в портальный gateway; переписываем `AnalyticsPage.tsx` с новыми фильтрами, переключаемым графиком и drawer-панелью для drill-down.

**Tech Stack:** Go 1.24 (pgx/sqlx, gRPC, gorilla/mux), protobuf, PostgreSQL 15, TypeScript 5.7 + React 19, Recharts 3, Radix UI, Tailwind CSS 4.2

---

## Карта файлов

| Файл | Действие |
|------|---------|
| `migrations/000083_messages_campaign_country.up.sql` | Создать |
| `migrations/000083_messages_campaign_country.down.sql` | Создать |
| `api/proto/analytics/analytics.proto` | Изменить |
| `api/proto/analyticsv1/analytics.pb.go` | Регенерировать |
| `api/proto/analyticsv1/analytics_grpc.pb.go` | Регенерировать |
| `internal/services/analytics/domain/repository.go` | Изменить |
| `internal/services/analytics/infrastructure/repository/metric_repository.go` | Изменить |
| `internal/services/analytics/grpc/server.go` | Изменить |
| `internal/services/analytics/grpc/server_test.go` | Изменить |
| `internal/gateway/portal/handlers/analytics.go` | Изменить |
| `internal/gateway/portal/handlers/analytics_test.go` | Изменить |
| `internal/gateway/portal/router/router.go` | Изменить |
| `portal-frontend/src/api/client.ts` | Изменить |
| `portal-frontend/src/pages/analytics/AnalyticsPage.tsx` | Изменить |
| `portal-frontend/src/components/analytics/AnalyticsDrawer.tsx` | Создать |

---

## Task 1: DB Migration — campaign_id и country_code на messages

**Files:**
- Create: `migrations/000083_messages_campaign_country.up.sql`
- Create: `migrations/000083_messages_campaign_country.down.sql`

- [ ] **Step 1: Написать up-миграцию**

```sql
-- migrations/000083_messages_campaign_country.up.sql
ALTER TABLE messages
  ADD COLUMN IF NOT EXISTS campaign_id   UUID REFERENCES campaigns(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS country_code  VARCHAR(5);

CREATE INDEX IF NOT EXISTS idx_messages_campaign_id   ON messages (campaign_id)   WHERE campaign_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_messages_country_code  ON messages (country_code)  WHERE country_code IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_messages_source        ON messages (source);
```

- [ ] **Step 2: Написать down-миграцию**

```sql
-- migrations/000083_messages_campaign_country.down.sql
DROP INDEX IF EXISTS idx_messages_source;
DROP INDEX IF EXISTS idx_messages_country_code;
DROP INDEX IF EXISTS idx_messages_campaign_id;

ALTER TABLE messages
  DROP COLUMN IF EXISTS country_code,
  DROP COLUMN IF EXISTS campaign_id;
```

- [ ] **Step 3: Применить миграцию на сервере**

```bash
scripts/server.sh migrate
```

Ожидаемый вывод: `... migration applied successfully`

- [ ] **Step 4: Коммит**

```bash
git add migrations/000083_messages_campaign_country.up.sql migrations/000083_messages_campaign_country.down.sql
git commit -m "feat(analytics): add campaign_id and country_code columns to messages"
```

---

## Task 2: Proto — новые методы и поля

**Files:**
- Modify: `api/proto/analytics/analytics.proto`

- [ ] **Step 1: Добавить поля в GetStatisticsRequest и новые сообщения**

Заменить блок `GetStatisticsRequest` и добавить методы в конце файла:

```protobuf
// GetStatisticsRequest представляет запрос на получение статистики
message GetStatisticsRequest {
  string client_id = 1;
  google.protobuf.Timestamp from = 2;
  google.protobuf.Timestamp to = 3;
  // group_by: day, week, country, provider, sender, campaign, status
  string group_by = 4;
  repeated string provider_ids = 5;
  // Новые фильтры
  repeated string sender_names  = 6;
  repeated string campaign_ids  = 7;
  repeated string statuses      = 8;
}
```

Добавить в service AnalyticsService два новых rpc:

```protobuf
  // GetMessagesList возвращает пагинированный список сообщений для drill-down
  rpc GetMessagesList(GetMessagesListRequest) returns (GetMessagesListResponse);

  // GetFilterOptions возвращает доступные значения фильтров для текущего клиента
  rpc GetFilterOptions(GetFilterOptionsRequest) returns (GetFilterOptionsResponse);
```

Добавить новые message-типы в конец файла:

```protobuf
message GetMessagesListRequest {
  string client_id      = 1;
  google.protobuf.Timestamp from = 2;
  google.protobuf.Timestamp to   = 3;
  repeated string provider_ids  = 4;
  repeated string sender_names  = 5;
  repeated string campaign_ids  = 6;
  repeated string statuses      = 7;
  string dimension_key  = 8;
  string dimension_type = 9;
  int32  page           = 10;
  int32  page_size      = 11;
}

message GetMessagesListResponse {
  repeated MessageRecord messages = 1;
  int32 total = 2;
}

message MessageRecord {
  string message_id                     = 1;
  string phone_masked                   = 2;
  string sender_name                    = 3;
  string status                         = 4;
  google.protobuf.Timestamp sent_at     = 5;
  google.protobuf.Timestamp delivered_at = 6;
  string error_code                     = 7;
  string provider_name                  = 8;
  string campaign_name                  = 9;
}

message GetFilterOptionsRequest {
  string client_id = 1;
}

message GetFilterOptionsResponse {
  repeated FilterOption providers  = 1;
  repeated FilterOption campaigns  = 2;
  repeated string      sender_names = 3;
}

message FilterOption {
  string id   = 1;
  string name = 2;
}
```

- [ ] **Step 2: Регенерировать Go-файлы из proto**

```bash
cd C:/projects/sms
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       api/proto/analytics/analytics.proto
```

Если `protoc` недоступен локально — сгенерированные файлы обновить вручную по образцу существующих (добавить новые поля/методы в `.pb.go` и `_grpc.pb.go`). Убедиться что компилируется:

```bash
go build ./...
```

- [ ] **Step 3: Коммит**

```bash
git add api/proto/analytics/ api/proto/analyticsv1/
git commit -m "feat(analytics): extend proto with GetMessagesList and GetFilterOptions"
```

---

## Task 3: Domain — новые типы и расширение интерфейса

**Files:**
- Modify: `internal/services/analytics/domain/repository.go`

- [ ] **Step 1: Расширить StatisticsFilters и добавить новые типы**

В файле `internal/services/analytics/domain/repository.go` заменить `StatisticsFilters` и добавить новые типы:

```go
// StatisticsFilters содержит фильтры для статистики
type StatisticsFilters struct {
	ClientID    *uuid.UUID
	From        time.Time
	To          time.Time
	// group_by: day, week, country, provider, sender, campaign, status
	GroupBy     string
	ProviderIDs []uuid.UUID
	// Новые фильтры
	SenderNames []string
	CampaignIDs []uuid.UUID
	Statuses    []string
}

// MessagesListFilters содержит фильтры для списка сообщений (drill-down)
type MessagesListFilters struct {
	ClientID      *uuid.UUID
	From          time.Time
	To            time.Time
	ProviderIDs   []uuid.UUID
	SenderNames   []string
	CampaignIDs   []uuid.UUID
	Statuses      []string
	DimensionKey  string
	DimensionType string // provider, sender, campaign, country, status, date
	Page          int
	PageSize      int // макс 50
}

// MessageRecord представляет одно сообщение в drill-down
type MessageRecord struct {
	MessageID    string
	PhoneMasked  string
	SenderName   string
	Status       string
	SentAt       time.Time
	DeliveredAt  *time.Time
	ErrorCode    string
	ProviderName string
	CampaignName string
}

// MessagesList результат drill-down запроса
type MessagesList struct {
	Messages []*MessageRecord
	Total    int32
}

// FilterOption пара id+name для мультиселектов
type FilterOption struct {
	ID   string
	Name string
}

// FilterOptions доступные значения фильтров для клиента
type FilterOptions struct {
	Providers   []*FilterOption
	Campaigns   []*FilterOption
	SenderNames []string
}
```

- [ ] **Step 2: Добавить методы в интерфейс MetricRepository**

```go
// MetricRepository определяет интерфейс для работы с метриками
type MetricRepository interface {
	Create(ctx context.Context, metric *Metric) error
	CreateAggregated(ctx context.Context, metric *AggregatedMetric) error
	GetAggregated(ctx context.Context, filters *AggregateFilters) ([]*AggregatedMetric, error)
	GetStatistics(ctx context.Context, filters *StatisticsFilters) (*Statistics, error)
	GetProviderPerformance(ctx context.Context, providerID uuid.UUID, from, to time.Time) (*ProviderPerformance, error)
	UpdateAggregated(ctx context.Context, metric *AggregatedMetric) error
	// Новые методы
	GetMessagesList(ctx context.Context, filters *MessagesListFilters) (*MessagesList, error)
	GetFilterOptions(ctx context.Context, clientID uuid.UUID) (*FilterOptions, error)
}
```

- [ ] **Step 3: Проверить компиляцию**

```bash
go build ./internal/services/analytics/...
```

Ожидается ошибка: `MetricRepository does not implement domain.MetricRepository (missing GetMessagesList, GetFilterOptions)` — это нормально, реализацию добавим в Task 4.

- [ ] **Step 4: Обновить mock**

В файле `internal/services/analytics/mocks/mock_metric_repository.go` добавить заглушки для двух новых методов (по образцу существующих в том же файле):

```go
func (m *MockMetricRepository) GetMessagesList(ctx context.Context, filters *domain.MessagesListFilters) (*domain.MessagesList, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MessagesList), args.Error(1)
}

func (m *MockMetricRepository) GetFilterOptions(ctx context.Context, clientID uuid.UUID) (*domain.FilterOptions, error) {
	args := m.Called(ctx, clientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.FilterOptions), args.Error(1)
}
```

- [ ] **Step 5: Коммит**

```bash
git add internal/services/analytics/domain/repository.go \
        internal/services/analytics/mocks/mock_metric_repository.go
git commit -m "feat(analytics): extend domain types with filters, MessagesList, FilterOptions"
```

---

## Task 4: Repository — SQL для новых фильтров и методов

**Files:**
- Modify: `internal/services/analytics/infrastructure/repository/metric_repository.go`

- [ ] **Step 1: Расширить GetStatistics — новые фильтры и GROUP BY**

В методе `GetStatistics` (строка ~179) найти блок формирования `whereExtra` и добавить обработку новых фильтров после существующих:

```go
// Добавить после блока ProviderIDs (строка ~195):
if len(filters.SenderNames) > 0 {
    whereExtra += fmt.Sprintf(" AND source = ANY($%d)", argIndex)
    args = append(args, filters.SenderNames)
    argIndex++
}
if len(filters.CampaignIDs) > 0 {
    whereExtra += fmt.Sprintf(" AND campaign_id = ANY($%d)", argIndex)
    args = append(args, filters.CampaignIDs)
    argIndex++
}
if len(filters.Statuses) > 0 {
    whereExtra += fmt.Sprintf(" AND status = ANY($%d)", argIndex)
    args = append(args, filters.Statuses)
    argIndex++
}
```

В блоке `switch filters.GroupBy` (строка ~255) добавить новые случаи:

```go
case "week":
    groupByExpr = "DATE_TRUNC('week', created_at)"
    groupKeyExpr = "TO_CHAR(DATE_TRUNC('week', created_at), 'YYYY-MM-DD')"
case "country":
    groupByExpr = "COALESCE(country_code, 'unknown')"
    groupKeyExpr = "COALESCE(country_code, 'unknown')"
case "provider":
    groupByExpr = "COALESCE(provider_id::text, 'unknown')"
    groupKeyExpr = "COALESCE(p.name, provider_id::text, 'unknown')"
case "sender":
    groupByExpr = "COALESCE(source, 'unknown')"
    groupKeyExpr = "COALESCE(source, 'unknown')"
case "campaign":
    groupByExpr = "COALESCE(campaign_id::text, 'no_campaign')"
    groupKeyExpr = "COALESCE(c.name, campaign_id::text, 'без кампании')"
case "status":
    groupByExpr = "status"
    groupKeyExpr = "status"
default: // "day"
    groupByExpr = "DATE_TRUNC('day', created_at)"
    groupKeyExpr = "TO_CHAR(DATE_TRUNC('day', created_at), 'YYYY-MM-DD')"
```

Для группировок `provider` и `campaign` нужны JOIN-ы. Изменить формирование `groupQuery`:

```go
// Заменить строку с FROM messages:
fromClause := "FROM messages m"
if filters.GroupBy == "provider" {
    fromClause += " LEFT JOIN providers p ON m.provider_id = p.id"
}
if filters.GroupBy == "campaign" {
    fromClause += " LEFT JOIN campaigns c ON m.campaign_id = c.id"
}

groupQuery := fmt.Sprintf(`
    SELECT
        %s as group_key,
        COUNT(*) FILTER (WHERE m.status IN ('sent', 'delivered', 'failed', 'expired', 'rejected')) as total_sent,
        COUNT(*) FILTER (WHERE m.status = 'delivered') as total_delivered,
        COUNT(*) FILTER (WHERE m.status IN ('failed', 'expired', 'rejected')) as total_failed,
        AVG(EXTRACT(EPOCH FROM (COALESCE(m.delivered_at, m.updated_at) - m.created_at)) * 1000) as avg_delivery_time_ms
    %s
    WHERE m.created_at >= $1 AND m.created_at <= $2%s
    GROUP BY %s
    ORDER BY %s`, groupKeyExpr, fromClause, whereExtra, groupByExpr, groupByExpr)
```

Также обновить `totalsQuery` чтобы использовать алиас `m`:

```go
totalsQuery := `
    SELECT
        m.status,
        COUNT(*) as count,
        AVG(EXTRACT(EPOCH FROM (COALESCE(m.delivered_at, m.updated_at) - m.created_at)) * 1000) as avg_delivery_time_ms
    FROM messages m
    WHERE m.created_at >= $1 AND m.created_at <= $2` + whereExtra + ` GROUP BY m.status`
```

- [ ] **Step 2: Реализовать GetMessagesList**

Добавить метод после `GetStatistics`:

```go
// GetMessagesList возвращает пагинированный список сообщений для drill-down
func (r *MetricRepository) GetMessagesList(ctx context.Context, filters *domain.MessagesListFilters) (*domain.MessagesList, error) {
	if filters.PageSize <= 0 || filters.PageSize > 50 {
		filters.PageSize = 50
	}
	if filters.Page <= 0 {
		filters.Page = 1
	}
	offset := (filters.Page - 1) * filters.PageSize

	args := []interface{}{filters.From, filters.To}
	argIndex := 3
	where := ""

	if filters.ClientID != nil {
		where += fmt.Sprintf(" AND m.client_id = $%d", argIndex)
		args = append(args, *filters.ClientID)
		argIndex++
	}
	if len(filters.ProviderIDs) > 0 {
		where += fmt.Sprintf(" AND m.provider_id = ANY($%d)", argIndex)
		args = append(args, filters.ProviderIDs)
		argIndex++
	}
	if len(filters.SenderNames) > 0 {
		where += fmt.Sprintf(" AND m.source = ANY($%d)", argIndex)
		args = append(args, filters.SenderNames)
		argIndex++
	}
	if len(filters.CampaignIDs) > 0 {
		where += fmt.Sprintf(" AND m.campaign_id = ANY($%d)", argIndex)
		args = append(args, filters.CampaignIDs)
		argIndex++
	}
	if len(filters.Statuses) > 0 {
		where += fmt.Sprintf(" AND m.status = ANY($%d)", argIndex)
		args = append(args, filters.Statuses)
		argIndex++
	}

	// Drill-down по конкретному значению измерения
	switch filters.DimensionType {
	case "provider":
		where += fmt.Sprintf(" AND p.name = $%d", argIndex)
		args = append(args, filters.DimensionKey)
		argIndex++
	case "sender":
		where += fmt.Sprintf(" AND m.source = $%d", argIndex)
		args = append(args, filters.DimensionKey)
		argIndex++
	case "campaign":
		where += fmt.Sprintf(" AND c.name = $%d", argIndex)
		args = append(args, filters.DimensionKey)
		argIndex++
	case "country":
		where += fmt.Sprintf(" AND COALESCE(m.country_code, 'unknown') = $%d", argIndex)
		args = append(args, filters.DimensionKey)
		argIndex++
	case "status":
		where += fmt.Sprintf(" AND m.status = $%d", argIndex)
		args = append(args, filters.DimensionKey)
		argIndex++
	case "date":
		where += fmt.Sprintf(" AND DATE_TRUNC('day', m.created_at) = $%d::date", argIndex)
		args = append(args, filters.DimensionKey)
		argIndex++
	}

	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM messages m
		LEFT JOIN providers p ON m.provider_id = p.id
		LEFT JOIN campaigns c ON m.campaign_id = c.id
		WHERE m.created_at >= $1 AND m.created_at <= $2%s`, where)

	var total int32
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	limitIdx := argIndex
	offsetIdx := argIndex + 1
	dataArgs := append(args, filters.PageSize, offset)

	dataQuery := fmt.Sprintf(`
		SELECT
			m.id::text                                   AS message_id,
			regexp_replace(m.destination, '(^\+?\d{1,4})\d+(\d{4}$)', '\1***\2') AS phone_masked,
			m.source                                     AS sender_name,
			m.status,
			m.created_at                                 AS sent_at,
			m.delivered_at,
			COALESCE(m.status_message, '')               AS error_code,
			COALESCE(p.name, '')                         AS provider_name,
			COALESCE(c.name, '')                         AS campaign_name
		FROM messages m
		LEFT JOIN providers p ON m.provider_id = p.id
		LEFT JOIN campaigns c ON m.campaign_id = c.id
		WHERE m.created_at >= $1 AND m.created_at <= $2%s
		ORDER BY m.created_at DESC
		LIMIT $%d OFFSET $%d`, where, limitIdx, offsetIdx)

	type msgRow struct {
		MessageID    string     `db:"message_id"`
		PhoneMasked  string     `db:"phone_masked"`
		SenderName   string     `db:"sender_name"`
		Status       string     `db:"status"`
		SentAt       time.Time  `db:"sent_at"`
		DeliveredAt  *time.Time `db:"delivered_at"`
		ErrorCode    string     `db:"error_code"`
		ProviderName string     `db:"provider_name"`
		CampaignName string     `db:"campaign_name"`
	}

	var rows []msgRow
	if err := r.db.SelectContext(ctx, &rows, dataQuery, dataArgs...); err != nil {
		return nil, err
	}

	records := make([]*domain.MessageRecord, len(rows))
	for i, row := range rows {
		records[i] = &domain.MessageRecord{
			MessageID:    row.MessageID,
			PhoneMasked:  row.PhoneMasked,
			SenderName:   row.SenderName,
			Status:       row.Status,
			SentAt:       row.SentAt,
			DeliveredAt:  row.DeliveredAt,
			ErrorCode:    row.ErrorCode,
			ProviderName: row.ProviderName,
			CampaignName: row.CampaignName,
		}
	}

	return &domain.MessagesList{Messages: records, Total: total}, nil
}
```

- [ ] **Step 3: Реализовать GetFilterOptions**

```go
// GetFilterOptions возвращает доступные фильтры для клиента
func (r *MetricRepository) GetFilterOptions(ctx context.Context, clientID uuid.UUID) (*domain.FilterOptions, error) {
	// Провайдеры: те, через которые клиент отправлял сообщения
	providerQuery := `
		SELECT DISTINCT p.id::text, p.name
		FROM messages m
		JOIN providers p ON m.provider_id = p.id
		WHERE m.client_id = $1
		ORDER BY p.name`

	type optRow struct {
		ID   string `db:"id"`
		Name string `db:"name"`
	}

	var provRows []optRow
	if err := r.db.SelectContext(ctx, &provRows, providerQuery, clientID); err != nil {
		return nil, err
	}
	providers := make([]*domain.FilterOption, len(provRows))
	for i, r := range provRows {
		providers[i] = &domain.FilterOption{ID: r.ID, Name: r.Name}
	}

	// Кампании клиента
	campQuery := `
		SELECT id::text, name
		FROM campaigns
		WHERE client_id = $1
		ORDER BY name`

	var campRows []optRow
	if err := r.db.SelectContext(ctx, &campRows, campQuery, clientID); err != nil {
		return nil, err
	}
	campaigns := make([]*domain.FilterOption, len(campRows))
	for i, r := range campRows {
		campaigns[i] = &domain.FilterOption{ID: r.ID, Name: r.Name}
	}

	// Отправители (уникальные source клиента)
	senderQuery := `
		SELECT DISTINCT source FROM messages
		WHERE client_id = $1 AND source != ''
		ORDER BY source`

	var senders []string
	if err := r.db.SelectContext(ctx, &senders, senderQuery, clientID); err != nil {
		return nil, err
	}

	return &domain.FilterOptions{
		Providers:   providers,
		Campaigns:   campaigns,
		SenderNames: senders,
	}, nil
}
```

- [ ] **Step 4: Проверить компиляцию**

```bash
go build ./internal/services/analytics/...
```

Ожидается: успешная сборка.

- [ ] **Step 5: Коммит**

```bash
git add internal/services/analytics/infrastructure/repository/metric_repository.go
git commit -m "feat(analytics): extend repository with new filters, GetMessagesList, GetFilterOptions"
```

---

## Task 5: gRPC Server — реализация новых методов

**Files:**
- Modify: `internal/services/analytics/grpc/server.go`
- Modify: `internal/services/analytics/grpc/server_test.go`

- [ ] **Step 1: Добавить метод GetMessagesList в AnalyticsService**

В `internal/services/analytics/application/analytics_service.go` добавить:

```go
// GetMessagesList возвращает пагинированный список сообщений
func (s *AnalyticsService) GetMessagesList(ctx context.Context, filters *domain.MessagesListFilters) (*domain.MessagesList, error) {
	return s.metricRepo.GetMessagesList(ctx, filters)
}

// GetFilterOptions возвращает доступные фильтры для клиента
func (s *AnalyticsService) GetFilterOptions(ctx context.Context, clientID uuid.UUID) (*domain.FilterOptions, error) {
	return s.metricRepo.GetFilterOptions(ctx, clientID)
}
```

- [ ] **Step 2: Расширить gRPC-сервер — обработка новых фильтров в GetStatistics**

В `internal/services/analytics/grpc/server.go` в методе `GetStatistics` после блока `ProviderIDs` добавить:

```go
// Новые фильтры
if len(req.SenderNames) > 0 {
    filters.SenderNames = req.SenderNames
}
if len(req.CampaignIds) > 0 {
    filters.CampaignIDs = make([]uuid.UUID, len(req.CampaignIds))
    for i, cid := range req.CampaignIds {
        campaignID, err := uuid.Parse(cid)
        if err != nil {
            return nil, status.Error(codes.InvalidArgument, "invalid campaign_id format")
        }
        filters.CampaignIDs[i] = campaignID
    }
}
if len(req.Statuses) > 0 {
    filters.Statuses = req.Statuses
}
```

- [ ] **Step 3: Реализовать GetMessagesList в gRPC-сервере**

```go
// GetMessagesList реализует метод gRPC-сервиса
func (s *Server) GetMessagesList(ctx context.Context, req *analyticsv1.GetMessagesListRequest) (*analyticsv1.GetMessagesListResponse, error) {
	var clientID *uuid.UUID
	if req.ClientId != "" {
		id, err := uuid.Parse(req.ClientId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
		}
		clientID = &id
	}

	var from, to time.Time
	if req.From != nil {
		from = req.From.AsTime()
	} else {
		from = time.Now().AddDate(0, 0, -7)
	}
	if req.To != nil {
		to = req.To.AsTime()
	} else {
		to = time.Now()
	}

	pageSize := int(req.PageSize)
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 50
	}
	page := int(req.Page)
	if page <= 0 {
		page = 1
	}

	filters := &domain.MessagesListFilters{
		ClientID:      clientID,
		From:          from,
		To:            to,
		SenderNames:   req.SenderNames,
		DimensionKey:  req.DimensionKey,
		DimensionType: req.DimensionType,
		Page:          page,
		PageSize:      pageSize,
	}

	if len(req.ProviderIds) > 0 {
		filters.ProviderIDs = make([]uuid.UUID, len(req.ProviderIds))
		for i, pid := range req.ProviderIds {
			id, err := uuid.Parse(pid)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, "invalid provider_id format")
			}
			filters.ProviderIDs[i] = id
		}
	}
	if len(req.CampaignIds) > 0 {
		filters.CampaignIDs = make([]uuid.UUID, len(req.CampaignIds))
		for i, cid := range req.CampaignIds {
			id, err := uuid.Parse(cid)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, "invalid campaign_id format")
			}
			filters.CampaignIDs[i] = id
		}
	}
	if len(req.Statuses) > 0 {
		filters.Statuses = req.Statuses
	}

	result, err := s.analyticsService.GetMessagesList(ctx, filters)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка сообщений")
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoMsgs := make([]*analyticsv1.MessageRecord, len(result.Messages))
	for i, m := range result.Messages {
		rec := &analyticsv1.MessageRecord{
			MessageId:    m.MessageID,
			PhoneMasked:  m.PhoneMasked,
			SenderName:   m.SenderName,
			Status:       m.Status,
			ErrorCode:    m.ErrorCode,
			ProviderName: m.ProviderName,
			CampaignName: m.CampaignName,
		}
		if !m.SentAt.IsZero() {
			rec.SentAt = timestamppb.New(m.SentAt)
		}
		if m.DeliveredAt != nil {
			rec.DeliveredAt = timestamppb.New(*m.DeliveredAt)
		}
		protoMsgs[i] = rec
	}

	return &analyticsv1.GetMessagesListResponse{
		Messages: protoMsgs,
		Total:    result.Total,
	}, nil
}
```

- [ ] **Step 4: Реализовать GetFilterOptions в gRPC-сервере**

```go
// GetFilterOptions реализует метод gRPC-сервиса
func (s *Server) GetFilterOptions(ctx context.Context, req *analyticsv1.GetFilterOptionsRequest) (*analyticsv1.GetFilterOptionsResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	opts, err := s.analyticsService.GetFilterOptions(ctx, clientID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения опций фильтров")
		return nil, status.Error(codes.Internal, err.Error())
	}

	providers := make([]*analyticsv1.FilterOption, len(opts.Providers))
	for i, p := range opts.Providers {
		providers[i] = &analyticsv1.FilterOption{Id: p.ID, Name: p.Name}
	}

	campaigns := make([]*analyticsv1.FilterOption, len(opts.Campaigns))
	for i, c := range opts.Campaigns {
		campaigns[i] = &analyticsv1.FilterOption{Id: c.ID, Name: c.Name}
	}

	return &analyticsv1.GetFilterOptionsResponse{
		Providers:   providers,
		Campaigns:   campaigns,
		SenderNames: opts.SenderNames,
	}, nil
}
```

- [ ] **Step 5: Проверить компиляцию**

```bash
go build ./internal/services/analytics/...
```

- [ ] **Step 6: Коммит**

```bash
git add internal/services/analytics/
git commit -m "feat(analytics): implement GetMessagesList and GetFilterOptions in grpc server"
```

---

## Task 6: Portal HTTP Handler — новые эндпоинты

**Files:**
- Modify: `internal/gateway/portal/handlers/analytics.go`

- [ ] **Step 1: Добавить whitelist для новых group_by и helper-парсеры**

В начало файла (в блок `var`) добавить:

```go
allowedAnalyticsGroups = map[string]struct{}{
    "day":      {},
    "week":     {},
    "country":  {},
    "provider": {},
    "sender":   {},
    "campaign": {},
    "status":   {},
}

allowedAnalyticsStatuses = map[string]struct{}{
    "delivered": {},
    "failed":    {},
    "expired":   {},
    "rejected":  {},
    "queued":    {},
    "sent":      {},
    "pending":   {},
}
```

Добавить helper для парсинга comma-separated UUID-списка:

```go
// parseUUIDs разбирает comma-separated строку в []string, проверяя UUID-формат
func parseUUIDs(raw string) ([]string, error) {
    if raw == "" {
        return nil, nil
    }
    parts := strings.Split(raw, ",")
    for _, p := range parts {
        if _, err := uuid.Parse(strings.TrimSpace(p)); err != nil {
            return nil, fmt.Errorf("invalid UUID: %q", p)
        }
    }
    return parts, nil
}

// parseCSV разбирает comma-separated строку в []string
func parseCSV(raw string) []string {
    if raw == "" {
        return nil
    }
    parts := strings.Split(raw, ",")
    result := make([]string, 0, len(parts))
    for _, p := range parts {
        if s := strings.TrimSpace(p); s != "" {
            result = append(result, s)
        }
    }
    return result
}
```

Добавить импорт `"strings"` если его ещё нет.

- [ ] **Step 2: Расширить GetAnalytics — новые query-параметры**

В методе `GetAnalytics` после блока парсинга `groupBy` добавить:

```go
// Новые фильтры
providerIDsRaw, err := parseUUIDs(query.Get("provider_ids"))
if err != nil {
    respondError(w, shared.ErrInvalidInput("Неверный формат provider_ids: "+err.Error()))
    return
}

campaignIDsRaw, err := parseUUIDs(query.Get("campaign_ids"))
if err != nil {
    respondError(w, shared.ErrInvalidInput("Неверный формат campaign_ids: "+err.Error()))
    return
}

senderNames := parseCSV(query.Get("sender_names"))

statusesRaw := parseCSV(query.Get("statuses"))
for _, s := range statusesRaw {
    if _, ok := allowedAnalyticsStatuses[s]; !ok {
        respondError(w, shared.ErrInvalidInput(fmt.Sprintf("Неизвестный статус: %q", s)))
        return
    }
}
```

Передать в gRPC-запрос:

```go
statsResp, err := h.analyticsClient.GetStatistics(statsCtx, &analyticsv1.GetStatisticsRequest{
    ClientId:    clientID.String(),
    From:        timestamppb.New(dateFrom),
    To:          timestamppb.New(dateTo),
    GroupBy:     groupBy,
    ProviderIds: providerIDsRaw,
    SenderNames: senderNames,
    CampaignIds: campaignIDsRaw,
    Statuses:    statusesRaw,
})
```

- [ ] **Step 3: Добавить GetAnalyticsMessages**

```go
// GetAnalyticsMessages обрабатывает GET /analytics/messages
func (h *AnalyticsHandlers) GetAnalyticsMessages(w http.ResponseWriter, r *http.Request) {
	if h.analyticsClient == nil {
		respondError(w, shared.ErrServiceUnavailable("Сервис аналитики временно недоступен"))
		return
	}

	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	query := r.URL.Query()

	// Период
	now := time.Now().UTC()
	dateFrom, dateTo := now.AddDate(0, 0, -7), now
	if fp := query.Get("date_from"); fp != "" {
		if tp := query.Get("date_to"); tp != "" {
			var err error
			dateFrom, err = time.Parse("2006-01-02", fp)
			if err != nil {
				respondError(w, shared.ErrInvalidInput("Неверный формат date_from"))
				return
			}
			parsedTo, err := time.Parse("2006-01-02", tp)
			if err != nil {
				respondError(w, shared.ErrInvalidInput("Неверный формат date_to"))
				return
			}
			dateTo = parsedTo.Add(24*time.Hour - time.Second)
		}
	}

	page := int32(1)
	if p := query.Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = int32(n)
		}
	}
	pageSize := int32(50)
	if ps := query.Get("page_size"); ps != "" {
		if n, err := strconv.Atoi(ps); err == nil && n > 0 && n <= 50 {
			pageSize = int32(n)
		}
	}

	providerIDs, _ := parseUUIDs(query.Get("provider_ids"))
	campaignIDs, _ := parseUUIDs(query.Get("campaign_ids"))
	senderNames := parseCSV(query.Get("sender_names"))
	statuses := parseCSV(query.Get("statuses"))

	req := &analyticsv1.GetMessagesListRequest{
		ClientId:      clientID.String(),
		From:          timestamppb.New(dateFrom),
		To:            timestamppb.New(dateTo),
		ProviderIds:   providerIDs,
		SenderNames:   senderNames,
		CampaignIds:   campaignIDs,
		Statuses:      statuses,
		DimensionKey:  query.Get("dimension_key"),
		DimensionType: query.Get("dimension_type"),
		Page:          page,
		PageSize:      pageSize,
	}

	ctx, cancel := context.WithTimeout(r.Context(), analyticsTimeout)
	defer cancel()

	resp, err := h.analyticsClient.GetMessagesList(ctx, req)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка сообщений")
		respondGRPCError(w, err)
		return
	}

	messages := make([]map[string]interface{}, len(resp.Messages))
	for i, m := range resp.Messages {
		entry := map[string]interface{}{
			"message_id":    m.MessageId,
			"phone_masked":  m.PhoneMasked,
			"sender_name":   m.SenderName,
			"status":        m.Status,
			"error_code":    m.ErrorCode,
			"provider_name": m.ProviderName,
			"campaign_name": m.CampaignName,
			"sent_at":       nil,
			"delivered_at":  nil,
		}
		if m.SentAt != nil {
			entry["sent_at"] = m.SentAt.AsTime().Format(time.RFC3339)
		}
		if m.DeliveredAt != nil {
			entry["delivered_at"] = m.DeliveredAt.AsTime().Format(time.RFC3339)
		}
		messages[i] = entry
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
		"total":    resp.Total,
		"page":     page,
		"page_size": pageSize,
	})
}
```

- [ ] **Step 4: Добавить GetAnalyticsFilterOptions**

```go
// GetAnalyticsFilterOptions обрабатывает GET /analytics/filter-options
func (h *AnalyticsHandlers) GetAnalyticsFilterOptions(w http.ResponseWriter, r *http.Request) {
	if h.analyticsClient == nil {
		respondError(w, shared.ErrServiceUnavailable("Сервис аналитики временно недоступен"))
		return
	}

	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), analyticsTimeout)
	defer cancel()

	resp, err := h.analyticsClient.GetFilterOptions(ctx, &analyticsv1.GetFilterOptionsRequest{
		ClientId: clientID.String(),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения опций фильтров")
		respondGRPCError(w, err)
		return
	}

	providers := make([]map[string]string, len(resp.Providers))
	for i, p := range resp.Providers {
		providers[i] = map[string]string{"id": p.Id, "name": p.Name}
	}

	campaigns := make([]map[string]string, len(resp.Campaigns))
	for i, c := range resp.Campaigns {
		campaigns[i] = map[string]string{"id": c.Id, "name": c.Name}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"providers":    providers,
		"campaigns":    campaigns,
		"sender_names": resp.SenderNames,
	})
}
```

- [ ] **Step 5: Проверить компиляцию**

```bash
go build ./internal/gateway/portal/...
```

- [ ] **Step 6: Коммит**

```bash
git add internal/gateway/portal/handlers/analytics.go
git commit -m "feat(analytics): add GetAnalyticsMessages and GetAnalyticsFilterOptions handlers"
```

---

## Task 7: Router — зарегистрировать новые маршруты

**Files:**
- Modify: `internal/gateway/portal/router/router.go`

- [ ] **Step 1: Найти строку регистрации /analytics и добавить новые маршруты**

В `router.go` найти (строка ~142):
```go
protected.HandleFunc("/analytics", analyticsHandlers.GetAnalytics).Methods("GET")
```

Добавить после неё:
```go
protected.HandleFunc("/analytics/messages", analyticsHandlers.GetAnalyticsMessages).Methods("GET")
protected.HandleFunc("/analytics/filter-options", analyticsHandlers.GetAnalyticsFilterOptions).Methods("GET")
```

- [ ] **Step 2: Проверить компиляцию и тесты**

```bash
go build ./...
go test ./internal/gateway/portal/... -count=1 -timeout 60s
```

Ожидается: все существующие тесты проходят.

- [ ] **Step 3: Коммит**

```bash
git add internal/gateway/portal/router/router.go
git commit -m "feat(analytics): register /analytics/messages and /analytics/filter-options routes"
```

---

## Task 8: Frontend — API client

**Files:**
- Modify: `portal-frontend/src/api/client.ts`

- [ ] **Step 1: Расширить analyticsApi**

Найти блок `export const analyticsApi` и заменить на:

```typescript
// Analytics API types
export type AnalyticsFilterOptions = {
  providers: { id: string; name: string }[];
  campaigns: { id: string; name: string }[];
  sender_names: string[];
};

export type AnalyticsMessagesResponse = {
  messages: {
    message_id: string;
    phone_masked: string;
    sender_name: string;
    status: string;
    sent_at: string | null;
    delivered_at: string | null;
    error_code: string;
    provider_name: string;
    campaign_name: string;
  }[];
  total: number;
  page: number;
  page_size: number;
};

export const analyticsApi = {
  get: (params: Record<string, string>, options?: RequestInit) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<unknown>(`/analytics?${qs}`, options);
  },
  getFilterOptions: () =>
    apiFetch<AnalyticsFilterOptions>('/analytics/filter-options'),
  getMessages: (params: Record<string, string>) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<AnalyticsMessagesResponse>(`/analytics/messages?${qs}`);
  },
};
```

- [ ] **Step 2: Коммит**

```bash
git add portal-frontend/src/api/client.ts
git commit -m "feat(analytics): extend analytics API client with filter-options and messages"
```

---

## Task 9: Frontend — AnalyticsDrawer компонент

**Files:**
- Create: `portal-frontend/src/components/analytics/AnalyticsDrawer.tsx`

- [ ] **Step 1: Создать компонент drawer**

```tsx
import { useEffect, useState } from 'react';
import { analyticsApi, type AnalyticsMessagesResponse } from '../../api/client';
import { ApiError } from '../../api/client';

type DrawerParams = {
  dimensionKey: string;
  dimensionType: string;
  dateFrom: string;
  dateTo: string;
  providerIds?: string;
  senderNames?: string;
  campaignIds?: string;
  statuses?: string;
};

type Props = {
  open: boolean;
  params: DrawerParams | null;
  onClose: () => void;
};

const STATUS_LABELS: Record<string, string> = {
  delivered: 'Доставлено',
  failed: 'Ошибка',
  expired: 'Истекло',
  rejected: 'Отклонено',
  sent: 'Отправлено',
  pending: 'Ожидание',
  queued: 'В очереди',
};

export function AnalyticsDrawer({ open, params, onClose }: Props) {
  const [data, setData] = useState<AnalyticsMessagesResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [page, setPage] = useState(1);

  useEffect(() => {
    if (!open || !params) return;
    setPage(1);
    setData(null);
  }, [open, params]);

  useEffect(() => {
    if (!open || !params) return;
    setLoading(true);
    setError('');
    const apiParams: Record<string, string> = {
      date_from: params.dateFrom,
      date_to: params.dateTo,
      dimension_key: params.dimensionKey,
      dimension_type: params.dimensionType,
      page: String(page),
      page_size: '50',
    };
    if (params.providerIds) apiParams.provider_ids = params.providerIds;
    if (params.senderNames) apiParams.sender_names = params.senderNames;
    if (params.campaignIds) apiParams.campaign_ids = params.campaignIds;
    if (params.statuses) apiParams.statuses = params.statuses;

    analyticsApi.getMessages(apiParams)
      .then(setData)
      .catch((err) => setError(err instanceof ApiError ? err.message : 'Ошибка загрузки'))
      .finally(() => setLoading(false));
  }, [open, params, page]);

  if (!open) return null;

  const totalPages = data ? Math.ceil(data.total / 50) : 1;

  return (
    <div className="fixed inset-y-0 right-0 w-[640px] bg-white shadow-2xl z-50 flex flex-col border-l border-gray-200">
      {/* Header */}
      <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200">
        <h2 className="text-base font-semibold text-gray-900 truncate">
          {params?.dimensionKey ?? '—'}
        </h2>
        <button
          type="button"
          onClick={onClose}
          className="text-gray-400 hover:text-gray-600 text-xl font-bold leading-none"
          aria-label="Закрыть"
        >
          ×
        </button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto px-6 py-4">
        {error && <p className="text-red-600 text-sm mb-4">{error}</p>}
        {loading && <p className="text-gray-500 text-sm">Загрузка...</p>}

        {data && (
          <>
            <p className="text-sm text-gray-500 mb-3">
              Всего сообщений: <span className="font-semibold text-gray-800">{data.total}</span>
            </p>
            <div className="overflow-x-auto">
              <table className="w-full text-xs border-collapse">
                <thead>
                  <tr className="bg-gray-50 text-left">
                    <th className="px-2 py-2 font-medium text-gray-600 border-b">Телефон</th>
                    <th className="px-2 py-2 font-medium text-gray-600 border-b">Отправитель</th>
                    <th className="px-2 py-2 font-medium text-gray-600 border-b">Статус</th>
                    <th className="px-2 py-2 font-medium text-gray-600 border-b">Время отправки</th>
                    <th className="px-2 py-2 font-medium text-gray-600 border-b">Время доставки</th>
                    <th className="px-2 py-2 font-medium text-gray-600 border-b">Ошибка</th>
                  </tr>
                </thead>
                <tbody>
                  {data.messages.map((msg) => (
                    <tr key={msg.message_id} className="hover:bg-gray-50 border-b border-gray-100">
                      <td className="px-2 py-2 font-mono">{msg.phone_masked}</td>
                      <td className="px-2 py-2">{msg.sender_name || '—'}</td>
                      <td className="px-2 py-2">
                        <span className={`inline-block px-1.5 py-0.5 rounded text-xs font-medium ${
                          msg.status === 'delivered' ? 'bg-green-100 text-green-700' :
                          msg.status === 'failed' || msg.status === 'rejected' ? 'bg-red-100 text-red-700' :
                          msg.status === 'expired' ? 'bg-orange-100 text-orange-700' :
                          'bg-gray-100 text-gray-600'
                        }`}>
                          {STATUS_LABELS[msg.status] ?? msg.status}
                        </span>
                      </td>
                      <td className="px-2 py-2 whitespace-nowrap">
                        {msg.sent_at ? new Date(msg.sent_at).toLocaleString('ru-RU') : '—'}
                      </td>
                      <td className="px-2 py-2 whitespace-nowrap">
                        {msg.delivered_at ? new Date(msg.delivered_at).toLocaleString('ru-RU') : '—'}
                      </td>
                      <td className="px-2 py-2 text-red-600">{msg.error_code || '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Pagination */}
            {totalPages > 1 && (
              <div className="flex gap-2 items-center mt-4 text-sm">
                <button
                  type="button"
                  disabled={page <= 1}
                  onClick={() => setPage((p) => p - 1)}
                  className="px-3 py-1 rounded border border-gray-300 disabled:opacity-40"
                >
                  ←
                </button>
                <span className="text-gray-600">{page} / {totalPages}</span>
                <button
                  type="button"
                  disabled={page >= totalPages}
                  onClick={() => setPage((p) => p + 1)}
                  className="px-3 py-1 rounded border border-gray-300 disabled:opacity-40"
                >
                  →
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Коммит**

```bash
git add portal-frontend/src/components/analytics/AnalyticsDrawer.tsx
git commit -m "feat(analytics): add AnalyticsDrawer component for drill-down"
```

---

## Task 10: Frontend — AnalyticsPage полная замена

**Files:**
- Modify: `portal-frontend/src/pages/analytics/AnalyticsPage.tsx`

- [ ] **Step 1: Заменить содержимое AnalyticsPage.tsx**

```tsx
import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import {
  LineChart, Line, BarChart, Bar, XAxis, YAxis, Tooltip,
  ResponsiveContainer, Legend, CartesianGrid, PieChart, Pie, Cell,
} from 'recharts';
import * as Tabs from '@radix-ui/react-tabs';
import {
  analyticsApi, ApiError,
  type AnalyticsDataExtended, type AnalyticsFilterOptions,
} from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { StatCard } from '../../components/data/StatCard';
import { DataTable, type Column } from '../../components/data/DataTable';
import { AnalyticsDrawer } from '../../components/analytics/AnalyticsDrawer';

// ─── Types ────────────────────────────────────────────────────────────────────

type GroupEntry = {
  group_key: string;
  sent: number;
  delivered: number;
  failed: number;
  delivery_rate: number;
  avg_delivery_ms?: number;
};

type TimelineEntry = GroupEntry & { period: string };
type CountryEntry = GroupEntry & { country: string };

const PERIODS = ['7d', '30d', '90d'] as const;
type Period = (typeof PERIODS)[number];

const GROUP_BY_OPTIONS = ['day', 'week', 'country', 'provider', 'sender', 'campaign', 'status'] as const;
type GroupBy = (typeof GROUP_BY_OPTIONS)[number];

const GROUP_BY_LABELS: Record<GroupBy, string> = {
  day: 'День',
  week: 'Неделя',
  country: 'Страна',
  provider: 'Провайдер',
  sender: 'Отправитель',
  campaign: 'Кампания',
  status: 'Статус',
};

const TIME_GROUP_BYS: GroupBy[] = ['day', 'week'];

const STATUS_OPTIONS = ['delivered', 'failed', 'expired', 'rejected', 'queued', 'sent'] as const;
const STATUS_LABELS: Record<string, string> = {
  delivered: 'Доставлено',
  failed: 'Ошибка',
  expired: 'Истекло',
  rejected: 'Отклонено',
  queued: 'В очереди',
  sent: 'Отправлено',
  pending: 'Ожидание',
};

const MAX_CUSTOM_RANGE_DAYS = 366;

const DONUT_COLORS: Record<string, string> = {
  delivered: '#10B981',
  failed: '#EF4444',
  expired: '#F59E0B',
  rejected: '#8B5CF6',
  sent: '#3B82F6',
  queued: '#6B7280',
  pending: '#94A3B8',
};

// ─── MultiSelect ──────────────────────────────────────────────────────────────

function MultiSelect({
  label,
  options,
  selected,
  onChange,
}: {
  label: string;
  options: { value: string; label: string }[];
  selected: string[];
  onChange: (v: string[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, []);

  const toggle = (v: string) => {
    onChange(selected.includes(v) ? selected.filter((x) => x !== v) : [...selected, v]);
  };

  const displayText = selected.length === 0
    ? label
    : selected.length === 1
    ? options.find((o) => o.value === selected[0])?.label ?? selected[0]
    : `${label}: ${selected.length}`;

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className={`flex items-center gap-1 px-3 py-1.5 text-sm rounded border ${
          selected.length > 0
            ? 'bg-primary/10 text-primary border-primary'
            : 'bg-white text-gray-700 border-gray-300 hover:bg-gray-50'
        }`}
      >
        {displayText}
        <span className="text-xs opacity-60">▾</span>
      </button>
      {open && (
        <div className="absolute z-20 top-full left-0 mt-1 min-w-[180px] bg-white border border-gray-200 rounded shadow-lg">
          {options.length === 0 ? (
            <div className="px-3 py-2 text-xs text-gray-400">Нет вариантов</div>
          ) : (
            options.map((opt) => (
              <label key={opt.value} className="flex items-center gap-2 px-3 py-1.5 hover:bg-gray-50 cursor-pointer text-sm">
                <input
                  type="checkbox"
                  checked={selected.includes(opt.value)}
                  onChange={() => toggle(opt.value)}
                  className="rounded"
                />
                {opt.label}
              </label>
            ))
          )}
        </div>
      )}
    </div>
  );
}

// ─── Main Component ───────────────────────────────────────────────────────────

export function AnalyticsPage() {
  // Filter state
  const [period, setPeriod] = useState<Period>('7d');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [useCustomDates, setUseCustomDates] = useState(false);
  const [compare, setCompare] = useState(false);
  const [groupBy, setGroupBy] = useState<GroupBy>('day');
  const [selectedProviders, setSelectedProviders] = useState<string[]>([]);
  const [selectedSenders, setSelectedSenders] = useState<string[]>([]);
  const [selectedCampaigns, setSelectedCampaigns] = useState<string[]>([]);
  const [selectedStatuses, setSelectedStatuses] = useState<string[]>([]);

  // Data state
  const [data, setData] = useState<AnalyticsDataExtended | null>(null);
  const [filterOptions, setFilterOptions] = useState<AnalyticsFilterOptions>({
    providers: [],
    campaigns: [],
    sender_names: [],
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Drawer state
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerKey, setDrawerKey] = useState('');
  const [drawerType, setDrawerType] = useState('');

  const abortRef = useRef<AbortController | null>(null);
  const requestIdRef = useRef(0);

  // Load filter options once
  useEffect(() => {
    analyticsApi.getFilterOptions()
      .then(setFilterOptions)
      .catch(() => {}); // non-critical
  }, []);

  // Compute effective date range
  const effectiveDateRange = useMemo(() => {
    if (useCustomDates && dateFrom && dateTo) {
      return { from: dateFrom, to: dateTo };
    }
    const now = new Date();
    const to = now.toISOString().split('T')[0];
    const days = period === '30d' ? 30 : period === '90d' ? 90 : 7;
    const from = new Date(now);
    from.setDate(from.getDate() - days);
    return { from: from.toISOString().split('T')[0], to };
  }, [useCustomDates, dateFrom, dateTo, period]);

  const loadAnalytics = useCallback(async () => {
    const currentId = ++requestIdRef.current;
    setLoading(true);
    setError('');

    if (useCustomDates) {
      if (!dateFrom || !dateTo) {
        setError('Для пользовательского периода укажите обе даты.');
        setLoading(false);
        return;
      }
      if (dateFrom > dateTo) {
        setError('Дата «По» не может быть раньше даты «С».');
        setLoading(false);
        return;
      }
      const rangeDays = Math.ceil(
        (Date.parse(dateTo) - Date.parse(dateFrom)) / 86400000
      );
      if (rangeDays > MAX_CUSTOM_RANGE_DAYS) {
        setError('Максимальный пользовательский диапазон — 366 дней.');
        setLoading(false);
        return;
      }
    }

    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    try {
      const params: Record<string, string> = {
        group_by: groupBy,
        include_cost: 'true',
      };
      if (useCustomDates && dateFrom && dateTo) {
        params.date_from = dateFrom;
        params.date_to = dateTo;
      } else {
        params.period = period;
      }
      if (compare) params.compare = 'true';
      if (selectedProviders.length) params.provider_ids = selectedProviders.join(',');
      if (selectedSenders.length) params.sender_names = selectedSenders.join(',');
      if (selectedCampaigns.length) params.campaign_ids = selectedCampaigns.join(',');
      if (selectedStatuses.length) params.statuses = selectedStatuses.join(',');

      const resp = await analyticsApi.get(params, { signal: controller.signal });
      if (currentId !== requestIdRef.current) return;
      setData(resp as AnalyticsDataExtended);
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') return;
      if (currentId !== requestIdRef.current) return;
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить аналитику');
    } finally {
      if (currentId !== requestIdRef.current) return;
      setLoading(false);
    }
  }, [period, dateFrom, dateTo, groupBy, useCustomDates, compare,
      selectedProviders, selectedSenders, selectedCampaigns, selectedStatuses]);

  useEffect(() => {
    loadAnalytics();
    return () => { abortRef.current?.abort(); };
  }, [loadAnalytics]);

  // Timeline & country data
  const timeline = useMemo(() => (data?.timeline ?? []) as TimelineEntry[], [data]);
  const byCountry = useMemo(() => (data?.by_country ?? []) as CountryEntry[], [data]);
  const prevTimeline = data?.previous_timeline ?? [];

  const mergedTimeline = useMemo(() => {
    const prevByPeriod = new Map(prevTimeline.map((e) => [e.period, e]));
    return timeline.map((entry) => {
      const prev = prevByPeriod.get(entry.period);
      return { ...entry, prev_sent: prev?.sent, prev_delivered: prev?.delivered };
    });
  }, [timeline, prevTimeline]);

  // Donut data from summary
  const donutData = useMemo(() => {
    if (!data) return [];
    const s = data.summary;
    return [
      { name: 'Доставлено', value: s.total_delivered ?? 0, key: 'delivered' },
      { name: 'Ошибки',     value: s.total_failed    ?? 0, key: 'failed' },
    ].filter((d) => d.value > 0);
  }, [data]);

  const isTimeGroupBy = TIME_GROUP_BYS.includes(groupBy);

  // Drawer open helper
  const openDrawer = (key: string) => {
    setDrawerKey(key);
    setDrawerType(groupBy === 'day' || groupBy === 'week' ? 'date' : groupBy);
    setDrawerOpen(true);
  };

  const drawerParams = {
    dimensionKey: drawerKey,
    dimensionType: drawerType,
    dateFrom: effectiveDateRange.from,
    dateTo: effectiveDateRange.to,
    providerIds: selectedProviders.join(',') || undefined,
    senderNames: selectedSenders.join(',') || undefined,
    campaignIds: selectedCampaigns.join(',') || undefined,
    statuses: selectedStatuses.join(',') || undefined,
  };

  const resetFilters = () => {
    setSelectedProviders([]);
    setSelectedSenders([]);
    setSelectedCampaigns([]);
    setSelectedStatuses([]);
  };

  const hasActiveFilters = selectedProviders.length + selectedSenders.length +
    selectedCampaigns.length + selectedStatuses.length > 0;

  // Table columns (dynamic first column)
  const groupColumns: Column<GroupEntry>[] = [
    {
      key: 'group_key',
      header: GROUP_BY_LABELS[groupBy],
      render: (row) => (
        <button
          type="button"
          className="text-primary hover:underline text-left"
          onClick={() => openDrawer(row.group_key)}
        >
          {row.group_key}
        </button>
      ),
    },
    { key: 'sent', header: 'Отправлено', sortable: true },
    { key: 'delivered', header: 'Доставлено', sortable: true },
    { key: 'failed', header: 'Ошибки', sortable: true },
    { key: 'delivery_rate', header: 'Доставляемость', render: (row) => <>{row.delivery_rate}%</>, sortable: true },
  ];

  const formatAvgTime = (ms?: number) => {
    if (!ms) return '—';
    if (ms < 1000) return `${ms} мс`;
    if (ms < 60000) return `${(ms / 1000).toFixed(1)} с`;
    return `${(ms / 60000).toFixed(1)} мин`;
  };

  return (
    <div className="max-w-6xl">
      <PageHeader title="Аналитика" />

      {/* ── Row 1: Period ── */}
      <fieldset className="border-none p-0 mb-3">
        <legend className="font-bold mb-2">Фильтры</legend>
        <div className="flex gap-2 items-center flex-wrap">
          {PERIODS.map((p) => (
            <button
              key={p}
              type="button"
              onClick={() => { setPeriod(p); setUseCustomDates(false); }}
              className={`px-4 py-1.5 text-sm rounded border ${
                !useCustomDates && period === p
                  ? 'bg-primary text-white border-primary'
                  : 'bg-white text-gray-700 border-gray-300 hover:bg-gray-50'
              }`}
            >
              {p}
            </button>
          ))}
          <span className="mx-1 text-gray-400">или</span>
          <label className="flex items-center gap-1 text-sm">
            С:
            <input
              type="date"
              value={dateFrom}
              onChange={(e) => { setDateFrom(e.target.value); setUseCustomDates(true); }}
              className="rounded border border-gray-300 px-2 py-1 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary"
            />
          </label>
          <label className="flex items-center gap-1 text-sm">
            По:
            <input
              type="date"
              value={dateTo}
              onChange={(e) => { setDateTo(e.target.value); setUseCustomDates(true); }}
              className="rounded border border-gray-300 px-2 py-1 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary"
            />
          </label>
          <label className="flex items-center gap-2 ml-2 text-sm cursor-pointer">
            <input
              type="checkbox"
              checked={compare}
              onChange={(e) => setCompare(e.target.checked)}
              className="rounded"
            />
            Сравнить с предыдущим периодом
          </label>
        </div>
      </fieldset>

      {/* ── Row 2: Dimension filters ── */}
      <div className="flex gap-2 items-center flex-wrap mb-4">
        <MultiSelect
          label="Провайдер"
          options={filterOptions.providers.map((p) => ({ value: p.id, label: p.name }))}
          selected={selectedProviders}
          onChange={setSelectedProviders}
        />
        <MultiSelect
          label="Отправитель"
          options={filterOptions.sender_names.map((s) => ({ value: s, label: s }))}
          selected={selectedSenders}
          onChange={setSelectedSenders}
        />
        <MultiSelect
          label="Кампания"
          options={filterOptions.campaigns.map((c) => ({ value: c.id, label: c.name }))}
          selected={selectedCampaigns}
          onChange={setSelectedCampaigns}
        />
        <MultiSelect
          label="Статус"
          options={STATUS_OPTIONS.map((s) => ({ value: s, label: STATUS_LABELS[s] }))}
          selected={selectedStatuses}
          onChange={setSelectedStatuses}
        />

        <span className="mx-1 text-gray-300">|</span>

        <label className="flex items-center gap-1 text-sm text-gray-600">
          Группировка:
          <select
            value={groupBy}
            onChange={(e) => setGroupBy(e.target.value as GroupBy)}
            className="ml-1 rounded border border-gray-300 px-2 py-1 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
          >
            {GROUP_BY_OPTIONS.map((g) => (
              <option key={g} value={g}>{GROUP_BY_LABELS[g]}</option>
            ))}
          </select>
        </label>

        {hasActiveFilters && (
          <button
            type="button"
            onClick={resetFilters}
            className="text-xs text-gray-500 hover:text-red-500 underline"
          >
            Сбросить фильтры
          </button>
        )}
      </div>

      {error && <p role="alert" className="text-red-600 mb-4">{error}</p>}
      {loading && <div role="status" aria-live="polite" className="text-gray-500 text-sm mb-4">Загрузка аналитики...</div>}

      {data && !loading && (
        <>
          {/* ── Summary cards ── */}
          <div className="grid grid-cols-2 md:grid-cols-6 gap-3 mb-6">
            <StatCard title="Отправлено" value={data.summary.total_sent} />
            <StatCard title="Доставлено" value={data.summary.total_delivered} />
            <StatCard title="Ошибки" value={data.summary.total_failed} />
            <StatCard title="Доставляемость" value={`${data.summary.delivery_rate}%`} />
            <StatCard
              title="Стоимость"
              value={
                data.summary.total_cost
                  ? new Intl.NumberFormat('ru-RU', {
                      style: 'currency',
                      currency: 'RUB',
                      minimumFractionDigits: 2,
                    }).format(parseFloat(data.summary.total_cost) || 0)
                  : '—'
              }
            />
            <StatCard title="Ср. время" value={formatAvgTime(data.summary.avg_delivery_ms)} />
          </div>

          {/* ── Charts row ── */}
          <div className="flex gap-4 mb-6">
            {/* Main chart */}
            <div className="flex-1 border border-gray-200 rounded-lg p-4">
              <ResponsiveContainer width="100%" height={220}>
                {isTimeGroupBy ? (
                  <LineChart data={mergedTimeline}>
                    <XAxis dataKey="period" tick={{ fontSize: 11 }}
                      tickFormatter={(v: string) => (typeof v === 'string' && v.length > 5 ? v.slice(5) : v)} />
                    <YAxis tick={{ fontSize: 11 }} />
                    <CartesianGrid strokeDasharray="3 3" />
                    <Tooltip />
                    <Legend />
                    <Line type="monotone" dataKey="sent" stroke="#3B82F6" strokeWidth={2} dot={false} name="Отправлено" />
                    <Line type="monotone" dataKey="delivered" stroke="#10B981" strokeWidth={2} dot={false} name="Доставлено" />
                    {compare && prevTimeline.length > 0 && (
                      <>
                        <Line type="monotone" dataKey="prev_sent" stroke="#93C5FD" strokeWidth={1.5} strokeDasharray="4 2" dot={false} name="Отправлено (пред.)" />
                        <Line type="monotone" dataKey="prev_delivered" stroke="#6EE7B7" strokeWidth={1.5} strokeDasharray="4 2" dot={false} name="Доставлено (пред.)" />
                      </>
                    )}
                  </LineChart>
                ) : (
                  <BarChart data={timeline} layout="vertical">
                    <XAxis type="number" tick={{ fontSize: 11 }} />
                    <YAxis type="category" dataKey="group_key" tick={{ fontSize: 10 }} width={100} />
                    <CartesianGrid strokeDasharray="3 3" />
                    <Tooltip />
                    <Legend />
                    <Bar dataKey="sent" fill="#3B82F6" name="Отправлено" />
                    <Bar dataKey="delivered" fill="#10B981" name="Доставлено" />
                  </BarChart>
                )}
              </ResponsiveContainer>
            </div>

            {/* Donut */}
            {donutData.length > 0 && (
              <div className="w-[180px] border border-gray-200 rounded-lg p-3 flex flex-col items-center">
                <p className="text-xs font-medium text-gray-600 mb-2">Статусы</p>
                <PieChart width={140} height={140}>
                  <Pie
                    data={donutData}
                    cx={65}
                    cy={65}
                    innerRadius={40}
                    outerRadius={65}
                    dataKey="value"
                    onClick={(entry) => {
                      const key = (entry as { key?: string }).key;
                      if (key) {
                        setSelectedStatuses((prev) =>
                          prev.includes(key) ? prev.filter((s) => s !== key) : [...prev, key]
                        );
                      }
                    }}
                  >
                    {donutData.map((entry) => (
                      <Cell key={entry.key} fill={DONUT_COLORS[entry.key] ?? '#9CA3AF'} />
                    ))}
                  </Pie>
                  <Tooltip formatter={(value: number, name: string) => [value, name]} />
                </PieChart>
                <div className="w-full space-y-1">
                  {donutData.map((d) => (
                    <div key={d.key} className="flex items-center gap-1 text-xs">
                      <span
                        className="inline-block w-2.5 h-2.5 rounded-full flex-shrink-0"
                        style={{ background: DONUT_COLORS[d.key] ?? '#9CA3AF' }}
                      />
                      <span className="text-gray-600 truncate">{d.name}</span>
                      <span className="ml-auto font-medium">{d.value.toLocaleString('ru-RU')}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>

          {/* ── Data table ── */}
          <Tabs.Root defaultValue="data">
            <Tabs.List className="flex gap-1 mb-4 border-b border-gray-200">
              <Tabs.Trigger
                value="data"
                className="px-4 py-2 text-sm text-gray-600 hover:text-gray-900 data-[state=active]:border-b-2 data-[state=active]:border-primary data-[state=active]:text-primary -mb-px"
              >
                {GROUP_BY_LABELS[groupBy]}
              </Tabs.Trigger>
            </Tabs.List>

            <Tabs.Content value="data">
              {timeline.length > 0 ? (
                <DataTable<GroupEntry>
                  columns={groupColumns}
                  data={timeline}
                  total={timeline.length}
                  page={1}
                  pageSize={timeline.length}
                  onPageChange={() => {}}
                  keyField="group_key"
                  tableLabel={`Разбивка по: ${GROUP_BY_LABELS[groupBy]}`}
                />
              ) : (
                <div className="py-8 text-center text-gray-500 text-sm">
                  Нет данных за выбранный период
                </div>
              )}
            </Tabs.Content>
          </Tabs.Root>
        </>
      )}

      {/* ── Drawer ── */}
      <AnalyticsDrawer
        open={drawerOpen}
        params={drawerOpen ? drawerParams : null}
        onClose={() => setDrawerOpen(false)}
      />
      {drawerOpen && (
        <div
          className="fixed inset-0 bg-black/20 z-40"
          onClick={() => setDrawerOpen(false)}
        />
      )}
    </div>
  );
}
```

- [ ] **Step 2: Проверить что TypeScript компилируется**

```bash
cd portal-frontend && npx tsc --noEmit
```

Исправить все TypeScript-ошибки (если есть несовпадение типов с `AnalyticsDataExtended` — добавить недостающие поля в тип в `client.ts`).

- [ ] **Step 3: Собрать фронтенд**

```bash
cd portal-frontend && npm run build
```

Ожидается: успешная сборка без ошибок.

- [ ] **Step 4: Коммит**

```bash
git add portal-frontend/src/pages/analytics/AnalyticsPage.tsx
git commit -m "feat(analytics): rewrite AnalyticsPage with multi-dimensional filters and drill-down"
```

---

## Task 11: Деплой и smoke-тест

- [ ] **Step 1: Пушнуть в GitHub и задеплоить**

```bash
git push origin master
scripts/server.sh deploy
```

- [ ] **Step 2: Применить миграцию на сервере (если не была применена в Task 1)**

```bash
scripts/server.sh migrate
```

- [ ] **Step 3: Smoke-тест через браузер**

1. Открыть `http://72.56.232.202:18085/analytics`
2. Убедиться что страница загружается без ошибок
3. Переключить группировку на "Провайдер" → проверить что таблица обновилась
4. Кликнуть на строку таблицы → drawer должен открыться с таблицей сообщений
5. Выбрать фильтр статуса "Доставлено" → проверить что summary пересчиталась
6. Нажать "Сбросить фильтры" → фильтры должны очиститься

- [ ] **Step 4: Финальный коммит (если нужны правки после smoke-теста)**

```bash
git add -p
git commit -m "fix(analytics): post-deploy fixes"
git push origin master
```
