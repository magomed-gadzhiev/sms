# Бенчмарки и профилирование производительности

## Обзор

В проекте используются бенчмарки Go для измерения производительности критичных компонентов и профилирование для выявления узких мест.

## Запуск бенчмарков

### Все бенчмарки

```bash
# Базовый запуск
go test -bench=. -benchmem ./internal/...

# Используя скрипт (Windows PowerShell)
.\scripts\benchmark.ps1

# Используя скрипт (Linux/Mac)
./scripts/profile.sh
```

### Бенчмарки с профилированием

```bash
# CPU профилирование
go test -bench=. -cpuprofile=cpu.prof -benchmem ./internal/...

# Memory профилирование
go test -bench=. -memprofile=mem.prof -benchmem ./internal/...

# Оба профиля
go test -bench=. -cpuprofile=cpu.prof -memprofile=mem.prof -benchmem ./internal/...

# Используя скрипты
.\scripts\benchmark.ps1 -All
./scripts/profile.sh --all
```

### Бенчмарки конкретного пакета

```bash
# SMPP протокол
go test -bench=. ./internal/smpp/protocol/...

# Router
go test -bench=. ./internal/router/...

# Queue
go test -bench=. ./internal/queue/...
```

## Анализ профилей

### CPU профиль

```bash
# Интерактивный режим
go tool pprof cpu.prof

# В браузере
go tool pprof -http=:8080 cpu.prof

# Текстовый отчет
go tool pprof -top cpu.prof
```

### Memory профиль

```bash
# Интерактивный режим
go tool pprof mem.prof

# В браузере
go tool pprof -http=:8080 mem.prof

# Текстовый отчет
go tool pprof -top mem.prof
```

## Доступные бенчмарки

### SMPP Protocol

- `BenchmarkEncodePDU` - кодирование базовой PDU
- `BenchmarkEncodeSubmitSM` - кодирование submit_sm
- `BenchmarkEncodeBind` - кодирование bind
- `BenchmarkDecodePDU` - декодирование базовой PDU
- `BenchmarkDecodeSubmitSM` - декодирование submit_sm

### Router

- `BenchmarkRouteMessage` - роутинг сообщения
- `BenchmarkRouteMessage_WithFailover` - роутинг с failover
- `BenchmarkRouteMessage_ByDestination` - роутинг по назначению

### Queue

- `BenchmarkSerializeKafkaMessage` - сериализация Kafka сообщения
- `BenchmarkDeserializeKafkaMessage` - десериализация Kafka сообщения
- `BenchmarkFromMessage` - преобразование Message в KafkaMessage
- `BenchmarkToMessage` - преобразование KafkaMessage в Message
- `BenchmarkDLRMessage_Serialize` - сериализация DLR сообщения
- `BenchmarkDeserializeDLR` - десериализация DLR сообщения

### SMSC Pool

- `BenchmarkThrottler_Allow` - проверка rate limit
- `BenchmarkConnection_UpdateActivity` - обновление активности соединения
- `BenchmarkConnection_IsExpired` - проверка истечения соединения

## Интерпретация результатов

### Метрики бенчмарков

- **ns/op** - наносекунды на операцию
- **B/op** - байты на операцию (аллокации памяти)
- **allocs/op** - количество аллокаций на операцию

### Пример вывода

```
BenchmarkEncodeSubmitSM-8    100000    12345 ns/op    1024 B/op    5 allocs/op
```

Это означает:
- 100,000 итераций
- 12,345 наносекунд на операцию
- 1,024 байта памяти на операцию
- 5 аллокаций на операцию

## Рекомендации по оптимизации

1. **Высокий ns/op** - проверьте алгоритмы, используйте более эффективные структуры данных
2. **Высокий B/op** - оптимизируйте использование памяти, переиспользуйте буферы
3. **Высокий allocs/op** - уменьшите количество аллокаций, используйте sync.Pool

## CI/CD интеграция

Бенчмарки должны запускаться в CI/CD pipeline для отслеживания регрессий:

```yaml
- name: Run benchmarks
  run: |
    go test -bench=. -benchmem ./internal/... > benchmark.txt
    
- name: Compare benchmarks
  run: |
    # Сравнение с предыдущими результатами
    benchstat old.txt benchmark.txt
```

## Дополнительные инструменты

- **benchstat** - сравнение результатов бенчмарков
- **pprof** - анализ профилей производительности
- **go-torch** - визуализация CPU профилей (deprecated, используйте pprof -http)

## Ссылки

- [Go Benchmarking](https://golang.org/pkg/testing/#hdr-Benchmarks)
- [Profiling Go Programs](https://golang.org/doc/diagnostics.html#profiling)
- [pprof Documentation](https://github.com/google/pprof/blob/main/doc/README.md)
