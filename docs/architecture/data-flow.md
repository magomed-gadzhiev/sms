# Потоки данных

## Обзор

Документ описывает потоки данных в системе SMPP сервера, включая обработку исходящих сообщений, delivery receipts и обработку ошибок.

## Исходящее сообщение через HTTP/gRPC API

### Поток данных

```
Клиент → HAProxy → API Gateway → PostgreSQL → Kafka → Worker → SMSC Provider
```

### Детальное описание

1. **Клиент отправляет запрос**
   - HTTP: `POST /api/v1/sms/send` или gRPC: `SendSMS`
   - Заголовок `X-API-Key` для аутентификации
   - Тело запроса содержит source, destination, text

2. **HAProxy балансирует запрос**
   - Round-robin между инстансами API Gateway
   - Health check перед маршрутизацией

3. **API Gateway обрабатывает запрос**
   - Middleware: Recovery → Logging → CORS → Authentication → Rate Limiting
   - Аутентификация клиента по API ключу
   - Проверка rate limit (Redis)
   - Валидация запроса

4. **API Gateway сохраняет сообщение**
   - Создание записи в таблице `messages` (статус: `pending`)
   - Сохранение в PostgreSQL
   - Получение message_id

5. **API Gateway публикует в Kafka**
   - Создание KafkaMessage структуры
   - Публикация в topic `sms.outgoing`
   - Асинхронная операция (не блокирует ответ)

6. **API Gateway возвращает ответ**
   - HTTP 200 / gRPC OK
   - Body содержит message_id и status: "queued"

7. **Worker потребляет сообщение**
   - Consumer читает из `sms.outgoing`
   - Обработка в отдельной горутине

8. **Worker маршрутизирует сообщение**
   - Определение провайдера через Router
   - Проверка правил маршрутизации (routes)
   - Выбор провайдера (load balancing / failover)

9. **Worker получает соединение**
   - Получение SMPP соединения из пула провайдера
   - Проверка здоровья соединения
   - Переподключение при необходимости

10. **Worker отправляет в SMSC**
    - Создание submit_sm PDU
    - Отправка через SMPP соединение
    - Ожидание submit_sm_resp

11. **Worker обновляет статус**
    - Обновление статуса в PostgreSQL (статус: `sent`)
    - Сохранение smpp_message_id
    - Обновление submitted_at

12. **SMSC обрабатывает сообщение**
    - Доставка сообщения получателю
    - Генерация delivery receipt (DLR)

## Delivery Receipt (DLR) поток

### Поток данных

```
SMSC Provider → Worker → Kafka → Worker → PostgreSQL → SMPP Server → SMPP Client
```

### Детальное описание

1. **SMSC отправляет DLR**
   - deliver_sm PDU через SMPP соединение Worker
   - Содержит stat (DELIVRD, UNDELIV, EXPIRED и т.д.)
   - Содержит receipted_message_id

2. **Worker обрабатывает DLR**
   - Парсинг deliver_sm PDU
   - Извлечение данных DLR
   - Создание DLRMessage структуры

3. **Worker публикует DLR в Kafka**
   - Публикация в topic `sms.dlr`
   - Асинхронная операция

4. **Worker обновляет статус сообщения**
   - Поиск сообщения по smpp_message_id
   - Обновление статуса в PostgreSQL
   - Обновление delivered_at или failed_at
   - Сохранение DLR в таблицу `dlr_receipts`

5. **Worker отправляет DLR клиенту (если подключен)**
   - Проверка подключения SMPP клиента
   - Отправка deliver_sm через SMPP Server
   - Уведомление клиента о статусе доставки

## Исходящее сообщение через SMPP

### Поток данных

```
SMPP Client → SMPP Server → PostgreSQL → Kafka → Worker → SMSC Provider
```

### Детальное описание

1. **SMPP клиент подключается**
   - TCP соединение к SMPP Server (порт 2775)
   - bind_receiver / bind_transmitter / bind_transceiver
   - Аутентификация (system_id, password)

2. **SMPP Server создает сессию**
   - Создание Session объекта
   - Сохранение информации о клиенте
   - Отправка bind_resp

3. **SMPP клиент отправляет submit_sm**
   - PDU с данными сообщения
   - Rate limiting проверка

4. **SMPP Server обрабатывает submit_sm**
   - Валидация PDU
   - Сохранение сообщения в PostgreSQL (статус: `pending`)
   - Публикация в Kafka topic `sms.outgoing`
   - Отправка submit_sm_resp с message_id

5. **Дальнейшая обработка**
   - Аналогична HTTP/gRPC потоку (шаги 7-12)

## Обработка ошибок

### Ошибка при публикации в Kafka

1. **API Gateway / SMPP Server**
   - Retry механизм (до 3 попыток)
   - Экспоненциальная задержка
   - При неудаче: статус сообщения → `failed`
   - Логирование ошибки

### Ошибка при отправке в SMSC

1. **Worker получает ошибку**
   - submit_sm_resp с error status
   - Или timeout соединения

2. **Worker проверяет retry count**
   - Если retry_count < max_retries:
     - Экспоненциальная задержка
     - Публикация обратно в `sms.outgoing`
     - Увеличение retry_count
   - Если retry_count >= max_retries:
     - Публикация в `sms.failed` (DLQ)
     - Статус сообщения → `failed`
     - Логирование ошибки

3. **Обработка failed сообщений**
   - Отдельный handler для `sms.failed`
   - Уведомление администраторов
   - Возможность ручной обработки

### Ошибка соединения с провайдером

1. **Worker обнаруживает разрыв соединения**
   - Health check провайдера
   - Автоматическое переподключение
   - Failover на резервный провайдер (если настроен)

2. **Сообщения в обработке**
   - Возврат в очередь при разрыве соединения
   - Retry после переподключения

## Пакетная отправка

### HTTP/gRPC Batch API

1. **Клиент отправляет batch запрос**
   - Массив сообщений в одном запросе
   - Обработка каждого сообщения независимо

2. **API Gateway обрабатывает batch**
   - Валидация каждого сообщения
   - Сохранение всех сообщений в PostgreSQL
   - Публикация всех сообщений в Kafka
   - Возврат результатов для каждого сообщения

3. **Worker обрабатывает batch**
   - Каждое сообщение обрабатывается независимо
   - Параллельная обработка через concurrency настройки

## Запрос статуса сообщения

### Поток данных

```
Клиент → HAProxy → API Gateway → PostgreSQL → Клиент
```

### Детальное описание

1. **Клиент запрашивает статус**
   - HTTP: `GET /api/v1/sms/status?id={message_id}`
   - gRPC: `GetStatus(message_id)`

2. **API Gateway обрабатывает запрос**
   - Аутентификация клиента
   - Поиск сообщения в PostgreSQL по message_id
   - Проверка прав доступа (сообщение принадлежит клиенту)

3. **API Gateway возвращает статус**
   - Статус сообщения (pending, queued, sent, delivered, failed)
   - Временные метки (created_at, submitted_at, delivered_at)
   - smpp_message_id (если доступен)

## История сообщений

### Поток данных

```
Клиент → HAProxy → API Gateway → PostgreSQL → Клиент
```

### Детальное описание

1. **Клиент запрашивает историю**
   - HTTP: `GET /api/v1/sms/history?limit=100&offset=0&status=sent`
   - Параметры: limit, offset, status (опционально)

2. **API Gateway обрабатывает запрос**
   - Аутентификация клиента
   - Запрос к PostgreSQL с фильтрацией по client_id
   - Применение фильтров (status, дата)
   - Пагинация (limit, offset)

3. **API Gateway возвращает историю**
   - Массив сообщений
   - Метаданные пагинации (limit, offset, count)

## Диаграммы потоков

### Исходящее сообщение (упрощенная схема)

```
┌─────────┐
│ Клиент  │
└────┬────┘
     │ HTTP/gRPC
     ▼
┌─────────────┐
│  HAProxy    │
└────┬────────┘
     │ балансировка
     ▼
┌─────────────┐      ┌──────────┐      ┌─────────┐
│API Gateway  │─────▶│PostgreSQL│      │  Kafka  │
└────┬────────┘      └──────────┘      └────┬────┘
     │                                      │
     └──────────────────────────────────────┘
                                            │
                                            ▼
                                    ┌─────────────┐
                                    │   Worker    │
                                    └──────┬──────┘
                                           │ SMPP
                                           ▼
                                    ┌─────────────┐
                                    │SMSC Provider│
                                    └─────────────┘
```

### Delivery Receipt поток

```
┌─────────────┐
│SMSC Provider│
└──────┬──────┘
       │ deliver_sm
       ▼
┌─────────────┐      ┌─────────┐      ┌──────────┐
│   Worker    │─────▶│  Kafka  │─────▶│PostgreSQL│
└──────┬──────┘      └─────────┘      └──────────┘
       │
       │ deliver_sm (если клиент подключен)
       ▼
┌─────────────┐
│SMPP Server  │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│SMPP Client  │
└─────────────┘
```

## Дополнительная документация

- [Обзор архитектуры](overview.md)
- [Микросервисы](microservices.md)
- [Kafka интеграция](../development/kafka.md)