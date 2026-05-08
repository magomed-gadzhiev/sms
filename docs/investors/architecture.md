# Архитектура SMS-платформы — техническое приложение

Этот документ — техническое приложение к инвесторской презентации. Содержит две схемы: **карту межсервисного взаимодействия** (C4-Container уровень) и **последовательность обработки одного сообщения** (sequence-диаграмма с временными бюджетами).

Схемы описаны на языке [Mermaid](https://mermaid.js.org/) — текстовом формате, который автоматически рендерится в SVG. Открыть схемы можно:
- В VS Code с расширением Mermaid Preview
- На [mermaid.live](https://mermaid.live/) — вставить блок кода и получить рендер
- В GitHub-просмотрщике markdown — рендеринг встроенный

---

## 1. Карта межсервисного взаимодействия

Полная схема платформы со всеми ключевыми компонентами и потоками данных. Жирные стрелки — happy-path рассылки сообщения; тонкие — вспомогательные потоки (аудит, метрики, биллинг); пунктир — административная плоскость.

```mermaid
graph TB
    subgraph EXT["Внешние участники"]
        Client["Клиент<br/>(банк, e-com, такси)"]
        Operator["Операторы<br/>МТС, Билайн, Мегафон, Tele2"]
        Aggregator["Партнёры-агрегаторы"]
        Recipient["Получатель<br/>(телефон)"]
        Admin["Администратор<br/>платформы"]
    end

    subgraph PLATFORM["SMS-платформа"]
        subgraph EDGE["Точки входа"]
            ClientGW["Client Gateway<br/>HTTP/gRPC API"]
            PortalGW["Portal Gateway<br/>backend кабинета"]
            AdminGW["Admin Gateway<br/>административный API"]
            SmppSrv["SMPP Server<br/>входящие SMPP"]
        end

        subgraph CORE["Конвейер обработки"]
            Pipeline["Pipeline Worker<br/>тарификация, маршрутизация"]
            SmppGW["SMPP Gateway<br/>исходящие SMPP"]
            DLR["DLR Delivery<br/>отчёты доставки"]
            Worker["Worker<br/>фоновые задачи<br/>(импорт CSV, scheduling)"]
        end

        subgraph DATA["Хранение и обмен"]
            Kafka[("Apache Kafka<br/>sms.outgoing, sms.dlr,<br/>sms.failed, cascade.*")]
            Postgres[("PostgreSQL 15<br/>messages, billing, audit<br/>помесячное партиционирование")]
            Redis[("Redis 7<br/>сессии, rate-limit,<br/>HLR-кеш")]
        end

        subgraph OBS["Мониторинг"]
            Prom["Prometheus"]
            Grafana["Grafana<br/>дашборды"]
        end
    end

    %% Happy path рассылки
    Client ==>|"1. POST /sms/send"| ClientGW
    ClientGW ==>|"2. publish"| Kafka
    Kafka ==>|"3. consume"| Pipeline
    Pipeline ==>|"4. publish dispatch"| Kafka
    Kafka ==>|"5. consume"| SmppGW
    SmppGW ==>|"6. SMPP submit"| Aggregator
    Aggregator ==>|"7. SMPP submit"| Operator
    Operator ==>|"8. доставка"| Recipient
    Operator -.->|"9. DLR"| Aggregator
    Aggregator -.->|"DLR"| DLR
    DLR -.->|"webhook"| Client

    %% Портал клиента
    Client -->|"HTTPS"| PortalGW
    PortalGW --> Postgres
    PortalGW --> Redis

    %% Админ-плоскость (пунктир)
    Admin -.->|"настройка тарифов,<br/>операторов, маршрутов"| AdminGW
    AdminGW -.-> Postgres

    %% Вспомогательные потоки (тонкие)
    Pipeline --> Postgres
    Pipeline --> Redis
    Pipeline -->|"тарификация"| Postgres
    Worker -->|"импорт CSV,<br/>scheduling"| Postgres
    Worker --> Kafka

    %% Входящий SMPP
    Aggregator -.->|"входящий трафик"| SmppSrv
    SmppSrv --> Kafka

    %% Метрики
    ClientGW -.-> Prom
    Pipeline -.-> Prom
    SmppGW -.-> Prom
    DLR -.-> Prom
    Prom --> Grafana

    classDef external fill:#e8f4f8,stroke:#1f4e79,stroke-width:2px
    classDef gateway fill:#fff4e6,stroke:#cc6600,stroke-width:2px
    classDef core fill:#e8f5e8,stroke:#2e7d32,stroke-width:2px
    classDef data fill:#f3e8ff,stroke:#6a1b9a,stroke-width:2px
    classDef obs fill:#f5f5f5,stroke:#616161,stroke-width:1px

    class Client,Operator,Aggregator,Recipient,Admin external
    class ClientGW,PortalGW,AdminGW,SmppSrv,SmppGW gateway
    class Pipeline,DLR,Worker core
    class Kafka,Postgres,Redis data
    class Prom,Grafana obs
```

### Что показывает схема

- **Точки входа** разделены: трафик клиентского API, портала и админки физически не пересекаются. Это безопасность и изоляция нагрузки.
- **Kafka — центральный буфер**: все асинхронные потоки идут через неё. При сбое любого worker'а сообщения копятся в очереди и обрабатываются после восстановления — без потерь.
- **Postgres** хранит транзакционные данные с помесячным партиционированием (messages, audit_log, deliveries, lookup_log) — это позволяет архивировать старые данные без остановки сервиса.
- **Redis** разгружает Postgres на горячих запросах: пользовательские сессии, rate-limiting, HLR-кеш.
- **Prometheus + Grafana** — независимая плоскость наблюдения, не зависящая от работоспособности самого продукта.

---

## 2. Жизнь одного сообщения

Sequence-диаграмма показывает, что физически происходит за время от вызова API клиентом до получения SMS на телефоне. Цифры в миллисекундах — типовые бюджеты при текущей нагрузке 1200 msg/sec.

```mermaid
sequenceDiagram
    autonumber
    participant C as Клиент<br/>(приложение)
    participant GW as Client Gateway
    participant K as Kafka
    participant P as Pipeline Worker
    participant R as Redis<br/>(HLR-кеш)
    participant DB as PostgreSQL
    participant SG as SMPP Gateway
    participant Op as Оператор
    participant Rec as Получатель
    participant DLR as DLR Delivery

    C->>+GW: POST /sms/send (~5 ms сеть)
    GW->>GW: аутентификация API-ключа<br/>валидация (~2 ms)
    GW->>+K: publish sms.outgoing
    GW-->>-C: 202 Accepted + message_id<br/>(всего ~10 ms)

    K->>+P: consume sms.outgoing
    P->>R: lookup оператора по номеру (~1 ms)
    R-->>P: operator_id или miss
    P->>DB: тарификация, списание баланса (~5 ms)
    P->>K: publish dispatch
    deactivate P

    K->>+SG: consume dispatch
    SG->>+Op: SMPP submit (~50–200 ms)
    Op-->>-SG: submit_resp
    SG->>DB: статус "sent"
    deactivate SG

    Op->>Rec: SMS-доставка<br/>(1–30 сек, зависит от оператора)

    Op-->>+DLR: SMPP DELIVER (delivery report)
    DLR->>DB: статус "delivered"
    DLR-->>-C: webhook (если настроен)
```

### Бюджет времени

| Этап | Типовое время | Что определяет |
|---|---|---|
| Приём + валидация + ack клиенту | ~10 мс | Наш код, контролируемо |
| Тарификация + маршрутизация | ~5–10 мс | Наш код, контролируемо |
| Передача SMPP оператору | ~50–200 мс | Сеть до оператора |
| Доставка SMS оператором | 1–30 сек | Оператор, не контролируем |
| Возврат DLR | 1–30 сек после доставки | Оператор |

**Главный вывод:** на долю нашей платформы приходится менее 5% общего времени доставки. Все остальное — операторы и сеть. Это означает, что для роста производительности нет смысла оптимизировать дальше внутренний код — нужно масштабировать SMPP-сессии и работать над контрактами с операторами.

---

## 3. Масштабируемость: где узкие места и как их обходить

| Уровень | Текущая мощность | Узкое место | План при 10× росте |
|---|---|---|---|
| Client Gateway (API) | ~5 000 RPS на узел | CPU на JSON-парсинге | Горизонтально: 2-3 узла за load-balancer |
| Pipeline Worker | ~1 200 msg/sec на узел | I/O в Postgres | 5-6 узлов worker'ов, читают из Kafka параллельно |
| SMPP Gateway | ~500 msg/sec на SMPP-сессию | Лимит оператора на сессию | Несколько сессий на оператора, балансировка между ними |
| PostgreSQL | ~3 000 write/sec | Запись в WAL | Read-replicas для аналитики, логическая репликация. Дальше — шардинг по client_id |
| Kafka | Десятки тысяч msg/sec | Disk I/O брокера | Кластер из 3 брокеров, репликация |
| Redis | Сотни тысяч op/sec | Сеть | Redis Cluster при необходимости |

При горизонте 50 000 msg/sec (2-3 года) ключевое архитектурное изменение — перевод на Kubernetes с автоматическим масштабированием по нагрузке и развёртывание в нескольких ЦОДах для гео-резервирования.
