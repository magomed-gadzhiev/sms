# Руководство для контрибьюторов

## Обзор

Спасибо за интерес к проекту! Этот документ описывает процесс разработки и внесения изменений в проект.

## Процесс разработки

### 1. Создание ветки

Создайте ветку от `main` для ваших изменений:

```bash
git checkout -b feature/your-feature-name
# или
git checkout -b fix/your-bug-fix
```

### 2. Разработка

Все разработки выполняются в Docker контейнере:

```bash
# Запустить dev контейнер
docker-compose -f deployments/docker-compose.yml up -d dev

# Войти в контейнер
docker-compose -f deployments/docker-compose.yml exec dev sh

# В контейнере
go mod download
```

### 3. Тестирование

Перед коммитом убедитесь, что все тесты проходят:

```bash
# В dev контейнере
go test ./...
go test -race ./...
go vet ./...
```

### 4. Форматирование кода

Используйте `gofmt` или `goimports`:

```bash
go fmt ./...
goimports -w .
```

### 5. Линтинг

Проверьте код линтером:

```bash
# Если установлен golangci-lint
golangci-lint run
```

### 6. Коммит

Создайте коммит с описательным сообщением:

```bash
git add .
git commit -m "feat: add new feature"
# или
git commit -m "fix: fix bug in router"
```

**Формат сообщений коммитов:**

- `feat:` - новая функциональность
- `fix:` - исправление бага
- `docs:` - изменения в документации
- `refactor:` - рефакторинг кода
- `test:` - добавление тестов
- `chore:` - обновление зависимостей, конфигурации и т.д.

### 7. Push и Pull Request

```bash
git push origin feature/your-feature-name
```

Создайте Pull Request в репозитории с описанием изменений.

## Стандарты кода

### Go код

- Следуйте [Effective Go](https://go.dev/doc/effective_go)
- Используйте `gofmt` для форматирования
- Комментируйте публичные функции и типы
- Используйте понятные имена переменных и функций

### Структура проекта

```
smpp-server/
├── cmd/              # Точки входа приложений
├── internal/         # Внутренние пакеты (не экспортируются)
├── pkg/              # Публичные пакеты
├── api/              # API определения (proto, openapi)
├── docs/             # Документация
├── deployments/      # Docker и конфигурации
├── migrations/       # Миграции БД
└── scripts/          # Утилитные скрипты
```

### Именование

- Пакеты: короткие, строчные, без подчеркиваний
- Функции: CamelCase для экспортируемых, camelCase для внутренних
- Константы: UPPER_SNAKE_CASE
- Переменные: camelCase

### Обработка ошибок

Всегда проверяйте ошибки:

```go
result, err := someFunction()
if err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}
```

Используйте типизированные ошибки из `internal/shared/errors.go`:

```go
if err := validate(); err != nil {
    return shared.ErrInvalidInput("field is required")
}
```

### Логирование

Используйте структурированное логирование через `zerolog`:

```go
import "github.com/rs/zerolog/log"

log.Info().
    Str("message_id", msgID).
    Str("status", "sent").
    Msg("message sent successfully")

log.Error().
    Err(err).
    Str("operation", "send_message").
    Msg("failed to send message")
```

### Тестирование

- Пишите unit тесты для всех публичных функций
- Используйте табличные тесты где возможно
- Мокируйте внешние зависимости
- Цель: coverage > 70%

Пример теста:

```go
func TestFunction(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {
            name:    "valid input",
            input:   "test",
            want:    "result",
            wantErr: false,
        },
        {
            name:    "invalid input",
            input:   "",
            want:    "",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Function(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Function() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("Function() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Документация

### Код

- Комментируйте все экспортируемые функции, типы и константы
- Используйте формат Go doc comments

```go
// ProcessMessage обрабатывает SMS сообщение и отправляет его в очередь.
// Возвращает message_id при успехе или ошибку при неудаче.
func ProcessMessage(ctx context.Context, msg *Message) (string, error) {
    // ...
}
```

### Документация проекта

- Обновляйте документацию при изменении функциональности
- Добавляйте примеры использования
- Обновляйте диаграммы при изменении архитектуры

## Работа с зависимостями

### Добавление новой зависимости

```bash
# В dev контейнере
go get github.com/example/package
go mod tidy
```

### Обновление зависимостей

```bash
go get -u ./...
go mod tidy
```

### Удаление неиспользуемых зависимостей

```bash
go mod tidy
```

## Работа с базой данных

### Создание миграции

```bash
# В dev контейнере
migrate create -ext sql -dir migrations -seq migration_name
```

### Применение миграций

```bash
# В dev контейнере
migrate -path migrations -database "postgres://smpp:${POSTGRES_PASSWORD:?POSTGRES_PASSWORD must be set}@postgres:5432/smpp_db?sslmode=disable" up
```

### Откат миграций

```bash
migrate -path migrations -database "postgres://smpp:${POSTGRES_PASSWORD:?POSTGRES_PASSWORD must be set}@postgres:5432/smpp_db?sslmode=disable" down
```

## Работа с Proto файлами

### Генерация кода из proto

```bash
# Windows
.\scripts\generate-proto.ps1

# Linux/Mac
./scripts/generate-proto.sh
```

### Изменение proto файлов

1. Отредактировать `api/proto/sms.proto`
2. Сгенерировать код
3. Обновить handlers если необходимо

## Code Review

При создании Pull Request убедитесь:

1. ✅ Код следует стандартам проекта
2. ✅ Все тесты проходят
3. ✅ Код отформатирован
4. ✅ Нет линтер ошибок
5. ✅ Документация обновлена
6. ✅ Коммиты имеют понятные сообщения

## Вопросы и поддержка

Если у вас есть вопросы:

1. Проверьте существующую документацию
2. Поищите в issues
3. Создайте новый issue с вопросом

## Лицензия

Внося изменения в проект, вы соглашаетесь с лицензией проекта.

## Дополнительная документация

- [Настройка окружения](setup.md)
- [Тестирование](testing.md)
- [Решение проблем](troubleshooting.md)
- [Kafka интеграция](kafka.md)
- [SMPP протокол](smpp-protocol.md)