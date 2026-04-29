# Spec №8: Справочник MCC/MNC — миграция схемы и seed-загрузка

**Дата:** 2026-04-26
**Источник:** MVP-таблица, вкладка «Админка Платформы», строки «Справочник → Операторы / Страны»
**Статус:** Не сделано + рассинхрон фронта и бэка по схеме.

## Контекст

Спека: «Загрузить mcc mnc» — массовая загрузка справочника MCC (Mobile Country Code) и MNC (Mobile Network Code) для стран и операторов.

Аудит:
- БД (`migrations/000012`):
  - `countries(id, name, iso_code, phone_code, currency)` — **MCC отсутствует**.
  - `operators(id, country_id, name, code UNIQUE, supports_paid_sender, supports_free_sender, active)` — **полей `mcc`/`mnc` нет**, только generic `code`.
  - `operator_prefixes(operator_id, prefix, priority)` — это телефонные префиксы (+7921...), не MCC/MNC.
- Frontend ([admin.ts:165](portal-frontend/src/api/admin.ts#L165)): `OperatorInfo` имеет **обязательные** поля `mcc: string; mnc: string`. Wizard ([CountriesPage.tsx:515](portal-frontend/src/pages/admin/CountriesPage.tsx#L515)) принимает MCC и MNC отдельными полями и шлёт `operatorsApi.create({mcc, mnc, ...})`.
- Это уже **баг рассинхрона** — фронт думает, что схема поддерживает MCC/MNC, бэк это игнорирует или мапит на `code`.
- Импорта (CSV/JSON/любого) в коде нет ни на бэке, ни на фронте.

## Решение

### Подход

1. Сначала **исправить рассинхрон схемы**: добавить колонки `mcc`/`mnc` в `operators` и `mcc` в `countries`.
2. Потом **seed-миграция** с данными по СНГ + мажорным странам (≈100 операторов). Источник — community-данные mcc-mnc.com (под лицензией ODbL) с курированием для основных регионов.
3. **Bulk-импорт через UI** — отдельный спек, в MVP не нужен. Для дополнения справочника в проде — admin вручную через wizard.

### Что меняем

**1. Миграция `000122_add_mcc_mnc_to_operators`**

```sql
-- Countries
ALTER TABLE countries
  ADD COLUMN mcc VARCHAR(3) NULL;
CREATE INDEX idx_countries_mcc ON countries(mcc) WHERE mcc IS NOT NULL;

-- Operators
ALTER TABLE operators
  ADD COLUMN mcc VARCHAR(3) NULL,
  ADD COLUMN mnc VARCHAR(3) NULL;
CREATE UNIQUE INDEX idx_operators_mcc_mnc ON operators(mcc, mnc) WHERE mcc IS NOT NULL AND mnc IS NOT NULL;
```

`mcc` и `mnc` оставляем NULL-able: операторы без MCC/MNC возможны (виртуальные/MVNO в процессе выделения, тестовые записи). UNIQUE — частичный, только для записей с обоими заполненными.

`down`: `DROP INDEX`, `DROP COLUMN` — обратимо.

**2. Backend: API enforce MCC/MNC**

В `internal/services/routing/application/operator_service.go`:
- `Create`/`Update` принимают `mcc`, `mnc` (опциональные, `*string`).
- Валидация: оба либо пусты, либо оба заполнены (xor запрещён). MCC = ровно 3 цифры, MNC = 1–3 цифры.
- Если оба заполнены — проверять UNIQUE(mcc, mnc) и возвращать `409 Conflict` при дубле.

В `internal/services/routing/application/country_service.go`:
- `Create`/`Update` принимают опциональный `mcc` (3 цифры или null).

**3. Seed-миграция `000123_seed_mcc_mnc_basic`**

Содержит UPSERT (через `INSERT ... ON CONFLICT DO UPDATE`) ~100 строк по реальным MCC/MNC основных операторов СНГ + EU + крупных международных:

```sql
-- Россия (MCC=250)
INSERT INTO countries(name, iso_code, phone_code, currency, mcc) VALUES
  ('Россия', 'RU', '7', 'RUB', '250')
ON CONFLICT (iso_code) DO UPDATE SET mcc = EXCLUDED.mcc;

INSERT INTO operators(country_id, name, code, mcc, mnc, active) VALUES
  ((SELECT id FROM countries WHERE iso_code='RU'), 'МТС',     'MTS',     '250', '01', true),
  ((SELECT id FROM countries WHERE iso_code='RU'), 'МегаФон', 'MEGAFON', '250', '02', true),
  ((SELECT id FROM countries WHERE iso_code='RU'), 'Билайн',  'BEELINE', '250', '99', true),
  ((SELECT id FROM countries WHERE iso_code='RU'), 'Tele2',   'TELE2',   '250', '20', true),
  ((SELECT id FROM countries WHERE iso_code='RU'), 'Yota',    'YOTA',    '250', '11', true)
ON CONFLICT (mcc, mnc) DO UPDATE SET name = EXCLUDED.name;

-- Казахстан (MCC=401)
-- Беларусь (MCC=257)
-- Узбекистан (MCC=434)
-- ... (полный список — отдельный CSV в migrations/seed-data/mcc-mnc.csv,
-- из которого миграция генерируется скриптом)
```

Объём seed — отдельный файл `migrations/seed-data/mcc-mnc-2026-04-26.csv` с курированными данными. Миграция пишется один раз на основе этого CSV.

`down`: только `DELETE` строк, добавленных этой миграцией (по `mcc IN (...)`). Существующие данные не трогаем.

**4. Frontend**

Уже корректно ожидает `mcc`/`mnc` — после миграции бэкенд отдаёт реальные значения.

В админке `/admin/countries`:
- В hierarchical-режиме рядом с названием страны показывать MCC: `«Россия (MCC: 250)»`.
- В описании оператора: `«МТС (250 01)»`.
- В flat-режиме фильтры по MCC/MNC уже есть в UI — после миграции они начнут работать.

**Кнопку «Загрузить MCC/MNC» в UI пока не делаем.** Spec MVP-таблицы говорит «Загрузить» в роадмап-смысле — мы загружаем seed-миграцией. Если позже понадобится bulk-CSV upload — отдельный спек.

### Что НЕ делаем

- **CSV upload через UI.** Объём задачи — день фронта + день бэка + парсер + валидация. Для MVP избыточно. Админ может пользоваться существующим wizard'ом для добавления новых операторов вручную.
- **Импорт из mcc-mnc.com через cron.** Источник community, без SLA, лицензия ODbL — нужно правильно атрибутировать. Для MVP — статический seed достаточен.
- **Замена `operator_prefixes` на MCC/MNC.** Это разные сущности: префиксы (+7921) определяют MNP, MCC/MNC — идентификация сети в SMPP-протоколе. Обе нужны параллельно.
- **Виртуальные операторы и MVNO с подробной таксономией.** Если нужно — позже отдельным спеком.

### Открытые вопросы (закрываю сам)

- **Источник данных:** community-курированный CSV (mcc-mnc.com + ITU TSB Operational Bulletin как сверка), хранится в `migrations/seed-data/mcc-mnc-YYYY-MM-DD.csv`.
- **Формат:** CSV с колонками `mcc, mnc, country_iso, operator_name, status (active/reserved/national)`.
- **Политика обновления:** UPSERT по `(mcc, mnc)`. Имя оператора при конфликте — обновляем (вдруг ребрендинг). Существующие пользовательские правки не теряем — `description`, `supports_*sender` поля seed не трогает.
- **`operators.code`:** остаётся UNIQUE. При импорте из CSV `code` генерируется из `operator_name` (uppercase, без пробелов: «МТС» → `MTS`). Если конфликт по `code` (например два оператора с одинаковым именем в разных странах) — добавляется суффикс `_<country_iso>` (`MTS_RU`).
- **MCC/MNC доступен только админу платформы.** Партнёры/клиенты не загружают свои справочники.

## Acceptance criteria

1. Миграция `000122` накатывается, добавляет колонки `mcc`/`mnc` в operators и `mcc` в countries без поломки FK.
2. UNIQUE-индекс `(mcc, mnc)` срабатывает только для записей с обоими заполненными.
3. Миграция `000123` (seed) добавляет ≥ 30 операторов СНГ с корректными MCC/MNC.
4. UI `/admin/countries` отображает MCC/MNC в обоих режимах (hierarchical/flat).
5. Создание нового оператора через wizard принимает MCC/MNC и валидирует формат.
6. Существующая логика routing (использующая `operator.code`) не сломана: smoke-тест отправки сообщения с разрешением оператора по префиксу проходит.
7. `down` обоих миграций откатывается чисто.

## Размер задачи

M (2-3 рабочих дня: 2 миграции + правки сервиса + ручное курирование CSV для seed + проверка UI).
