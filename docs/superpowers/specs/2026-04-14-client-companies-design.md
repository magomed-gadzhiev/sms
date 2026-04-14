# Компании клиентов (Client Companies)

**Дата:** 2026-04-14
**Статус:** Draft

## Цель

Дать клиентам возможность заводить компании с юридическими реквизитами (по ИНН). Компании используются для регистрации sender names и имеют собственные балансы. Списание идёт с первого ненулевого баланса.

## Ключевые решения

- **Связь клиент ↔ компания:** N:N через связующую таблицу. У клиента есть «основная» компания по умолчанию.
- **Баланс:** на уровне компании (не клиента).
- **Поиск по ИНН:** пока без внешнего API — ручной ввод, валидация ИНН по контрольной сумме.
- **Компания «Оферта»:** при регистрации нового клиента автоматически создаётся персональная компания «Оферта» (`is_offer = true`, без ИНН). Означает работу по договору-оферте.
- **Миграция sender names:** существующие sender names привязываются к автоматически созданной компании-оферте клиента.

## Модель данных

### Таблица `companies`

| Поле | Тип | Обязательное | Описание |
|------|-----|:---:|----------|
| id | UUID | да | PK, gen_random_uuid() |
| inn | VARCHAR(12) | нет | ИНН (10 или 12 цифр), nullable для «Оферта» |
| name | VARCHAR(255) | да | Краткое наименование |
| full_name | VARCHAR(500) | нет | Полное наименование |
| kpp | VARCHAR(9) | нет | КПП |
| ogrn | VARCHAR(15) | нет | ОГРН/ОГРНИП |
| legal_address | TEXT | нет | Юридический адрес |
| actual_address | TEXT | нет | Фактический адрес |
| ceo_name | VARCHAR(255) | нет | ФИО руководителя |
| ceo_title | VARCHAR(255) | нет | Должность |
| acting_basis | VARCHAR(255) | нет | На основании (Устав, доверенность №...) |
| bank_name | VARCHAR(255) | нет | Наименование банка |
| bank_bik | VARCHAR(9) | нет | БИК |
| bank_corr_account | VARCHAR(20) | нет | Корреспондентский счёт |
| bank_account | VARCHAR(20) | нет | Расчётный счёт |
| email | VARCHAR(255) | нет | Контактный email |
| phone | VARCHAR(20) | нет | Контактный телефон |
| is_offer | BOOLEAN | да | true = системная компания-оферта |
| active | BOOLEAN | да | default true |
| created_at | TIMESTAMPTZ | да | default now() |
| updated_at | TIMESTAMPTZ | да | default now() |

### Таблица `client_companies`

| Поле | Тип | Описание |
|------|-----|----------|
| client_id | UUID FK → clients | |
| company_id | UUID FK → companies | |
| is_default | BOOLEAN | Основная компания клиента |
| created_at | TIMESTAMPTZ | |

**Ограничения:**
- UNIQUE (client_id, company_id)
- Partial unique index: UNIQUE (client_id) WHERE is_default = true — гарантия ровно одной основной компании

### Изменения существующих таблиц

**`sender_names`:** добавить `company_id UUID NOT NULL FK → companies` (после миграции данных).

**`accounts`:** добавить `company_id UUID FK → companies`. Поле `client_id` остаётся для обратной совместимости, но основной ключ для поиска баланса — `company_id`.

## Логика биллинга

### Списание при отправке SMS

1. Клиент отправляет SMS (указывает sender name или используется дефолтный)
2. Sender name привязан к `company_id` — определяем компанию
3. Проверяем баланс этой компании
4. Если баланс > 0 — списываем
5. Если баланс = 0 — ищем следующую компанию клиента с ненулевым балансом
   - Порядок: `is_default` первой, остальные по `created_at`
6. Если все балансы = 0 — отклоняем отправку

### Биллинг sender names

Ежемесячный биллинг за sender name (`sender_name_billing_records`) списывается с баланса компании, к которой привязан sender name.

## API

### gRPC-сервис `CompanyService`

```protobuf
service CompanyService {
  rpc CreateCompany(CreateCompanyRequest) returns (CompanyResponse);
  rpc UpdateCompany(UpdateCompanyRequest) returns (CompanyResponse);
  rpc GetCompany(GetCompanyRequest) returns (CompanyResponse);
  rpc ListClientCompanies(ListClientCompaniesRequest) returns (ListClientCompaniesResponse);
  rpc SetDefaultCompany(SetDefaultCompanyRequest) returns (Empty);
  rpc AttachCompany(AttachCompanyRequest) returns (Empty);
  rpc DetachCompany(DetachCompanyRequest) returns (Empty);
}
```

- `CreateCompany` — создать компанию и привязать к клиенту
- `UpdateCompany` — обновить реквизиты
- `GetCompany` — получить компанию по ID
- `ListClientCompanies` — список компаний клиента
- `SetDefaultCompany` — сменить основную компанию
- `AttachCompany` — привязать существующую компанию к клиенту
- `DetachCompany` — отвязать (запрещено если `is_default` или есть привязанные sender names)

### Валидация ИНН

- 10 цифр (юр. лицо) или 12 цифр (ИП) — проверка контрольной суммы на бэкенде
- Уникальность ИНН в системе НЕ требуется (разные клиенты могут завести одну и ту же компанию)

## Портал (frontend)

- **Страница «Мои компании»** — список компаний клиента с балансами
- **Детальная страница компании** — реквизиты (редактируемые), баланс, список sender names
- **Форма создания компании** — ИНН + наименование обязательны, остальное опционально
- **Форма регистрации sender name** — добавить выбор компании (dropdown), по умолчанию — основная компания клиента
- **Страница биллинга** — показывать балансы по компаниям

## Миграция данных

Выполняется в одной транзакции:

1. Создать таблицы `companies` и `client_companies`
2. Для каждого существующего клиента:
   - Создать компанию `name = 'Оферта'`, `is_offer = true`, без ИНН
   - Вставить запись в `client_companies` с `is_default = true`
   - Существующий `accounts` клиента привязать к новой компании
3. Добавить `company_id` в `accounts` (nullable), заполнить из `client_companies`

### Создание новой компании (runtime)

При создании новой компании клиентом автоматически создаётся запись `accounts` с нулевым балансом для этой компании. Аналогично, при регистрации нового клиента создаётся компания «Оферта» + `accounts` с нулевым балансом.
4. Добавить `company_id` в `sender_names` (nullable), заполнить из `client_companies` (привязать к дефолтной компании клиента)
5. Сделать `company_id` NOT NULL в обеих таблицах
6. Добавить FK и индексы

### Обратная совместимость

- `accounts.client_id` остаётся, но перестаёт быть основным ключом для поиска баланса
- Существующие API-вызовы (`ChargeMessage`, `GetBalance`) продолжают работать — внутри резолвят компанию через `client_companies.is_default`
- Новые эндпоинты используют `company_id` напрямую
