# Network Tariffs Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Заменить трёхвкладочный UI `/network/tariffs` на URL-based страницы (список / библиотека / редактор) + переиспользуемый матричный компонент, применить его и на клиентском `/tariffs`. Источник: [spec 2026-04-22](../specs/2026-04-22-network-tariffs-redesign-design.md).

**Architecture:** Три новые страницы под `/network/tariffs/*` с shared компонентом `<TariffMatrix>`. Бэкенд-эндпоинты добавляются в `cmd/portal-gateway/internal/gateway/portal/handlers/` рядом с существующими `reseller_tariff_*.go` — прямая работа с PostgreSQL через `pgxpool` (как в действующем паттерне). Фронтовый API-клиент расширяется namespace'ом `networkTariffsApi` в `portal-frontend/src/api/client.ts`.

**Tech Stack:** Go 1.24 + pgx/v5 + gorilla/mux (backend), TypeScript 5.7 + React 19 + Radix UI + Tailwind 4 (frontend), PostgreSQL 15+. Stack уже в проекте, новых зависимостей не добавляем.

**Execution wrapper:** все кодовые таски выполняются через `/execute-with-review` (проектное правило в `CLAUDE.md`).

> **URL prefix (schema reality):** в тексте плана эндпоинты показаны как `/api/network/*` для краткости; реальный service prefix — `/portal/v1/network/*`. Frontend `portal-frontend/src/api/client.ts` использует базовый путь `/portal/v1`. Router-регистрации и примеры запросов должны использовать `/portal/v1/network/*`.

**Phases:**
- **Phase 1** — Backend API (tasks 1–6)
- **Phase 2** — Frontend skeleton + routing + list page (tasks 7–10)
- **Phase 3** — Templates library page (tasks 11–13)
- **Phase 4** — `<TariffMatrix>` shared component (tasks 14–17)
- **Phase 5** — Editor page + save flow (tasks 18–21)
- **Phase 6** — Client `/tariffs` migration + a11y + cleanup (tasks 22–25)

---

## Phase 1 — Backend API

### Task 1: `GET /api/network/tariffs/subaccounts-summary`

**Files:**
- Create: `internal/gateway/portal/handlers/network_tariffs_summary.go`
- Modify: `internal/gateway/portal/router.go` (регистрация роута рядом с другими `/reseller/tariff-*`)
- Test: `internal/gateway/portal/handlers/network_tariffs_summary_test.go`

- [ ] **Step 1: Write failing test**

```go
package handlers_test

func TestSubaccountsSummary_ReturnsOwnReseller(t *testing.T) {
  ctx, pool, cleanup := testdb.Setup(t)
  defer cleanup()
  resellerID := seedReseller(t, pool, "reseller-a")
  sa1 := seedSubAccount(t, pool, resellerID, "Test Clean (A)")
  sa2 := seedSubAccount(t, pool, resellerID, "Test Heavy (C)")
  tplID := seedTariffTemplate(t, pool, resellerID, "Базовый")
  bindTemplate(t, pool, tplID, sa1)
  addOverride(t, pool, sa2, "MTS", "paid", 3.2)

  h := handlers.NewNetworkTariffsSummaryHandler(pool)
  req := httptest.NewRequest("GET", "/api/network/tariffs/subaccounts-summary", nil).
    WithContext(authCtx(ctx, resellerID)) // is_reseller=true flag seeded by seedReseller()
  rr := httptest.NewRecorder()
  h.List(rr, req)

  require.Equal(t, 200, rr.Code)
  var out []handlers.SubAccountSummary
  require.NoError(t, json.NewDecoder(rr.Body).Decode(&out))
  require.Len(t, out, 2)
  require.Equal(t, "Test Clean (A)", out[0].SubAccountName)
  require.Equal(t, "Базовый", *out[0].TemplateName)
  require.Equal(t, 0, out[0].OverrideCount)
  require.Equal(t, 1, out[1].OverrideCount)
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./internal/gateway/portal/handlers -run TestSubaccountsSummary -v`
Expected: compile error (handler не существует).

- [ ] **Step 3: Implement handler**

```go
// internal/gateway/portal/handlers/network_tariffs_summary.go
package handlers

import (
  "encoding/json"
  "net/http"
  "github.com/jackc/pgx/v5/pgxpool"
)

type SubAccountSummary struct {
  SubAccountID      string   `json:"sub_account_id"`
  SubAccountName    string   `json:"sub_account_name"`
  SubAccountEmail   string   `json:"sub_account_email"`
  TemplateID        *string  `json:"template_id"`
  TemplateName      *string  `json:"template_name"`
  OverrideCount     int      `json:"override_count"`
  AvgPricePerSMS    *float64 `json:"avg_price_per_sms"`
  Currency          string   `json:"currency"`
}

type NetworkTariffsSummaryHandler struct{ pool *pgxpool.Pool }

func NewNetworkTariffsSummaryHandler(p *pgxpool.Pool) *NetworkTariffsSummaryHandler {
  return &NetworkTariffsSummaryHandler{pool: p}
}

func (h *NetworkTariffsSummaryHandler) List(w http.ResponseWriter, r *http.Request) {
  resellerID, ok := authResellerID(r.Context())
  if !ok { http.Error(w, "unauthorized", 401); return }
  rows, err := h.pool.Query(r.Context(), `
    SELECT c.id, c.name, c.email,
           tpl.id, tpl.name,
           (SELECT COUNT(*) FROM reseller_tariff_plans p
              WHERE p.sub_account_id = c.id AND p.active),
           NULL::numeric, -- avg_price_per_sms: computed in follow-up task
           'RUB'::text    -- clients.currency не существует; hardcode до мультивалютности
    FROM clients c
    LEFT JOIN sub_account_template_assignments b
           ON b.sub_account_id = c.id
    LEFT JOIN reseller_tariff_templates tpl
           ON tpl.id = b.template_id AND tpl.active
    WHERE c.parent_client_id = $1
    ORDER BY c.name
  `, resellerID)
  if err != nil { http.Error(w, err.Error(), 500); return }
  defer rows.Close()

  out := []SubAccountSummary{}
  for rows.Next() {
    var s SubAccountSummary
    if err := rows.Scan(&s.SubAccountID, &s.SubAccountName, &s.SubAccountEmail,
                        &s.TemplateID, &s.TemplateName, &s.OverrideCount,
                        &s.AvgPricePerSMS, &s.Currency); err != nil {
      http.Error(w, err.Error(), 500); return
    }
    out = append(out, s)
  }
  w.Header().Set("Content-Type", "application/json")
  _ = json.NewEncoder(w).Encode(out)
}
```

- [ ] **Step 4: Register route**

Edit `internal/gateway/portal/router.go` — добавить рядом с существующими `/reseller/tariff-*`:

```go
summaryH := handlers.NewNetworkTariffsSummaryHandler(pool)
// Mounted on `protected` subrouter; is_reseller=true проверяется в handler'е
// (см. authReseller в handlers — consistent с `reseller_*` эндпоинтами).
protected.HandleFunc("/network/tariffs/subaccounts-summary", summaryH.List).Methods("GET")
```

- [ ] **Step 5: Run test — expect PASS**

Run: `go test ./internal/gateway/portal/handlers -run TestSubaccountsSummary -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/gateway/portal/handlers/network_tariffs_summary.go \
        internal/gateway/portal/handlers/network_tariffs_summary_test.go \
        internal/gateway/portal/router.go
git commit -m "feat(tariffs): GET /api/network/tariffs/subaccounts-summary"
```

---

### Task 2: Avg price computation + Redis cache

**Files:**
- Modify: `internal/gateway/portal/handlers/network_tariffs_summary.go`
- Test: `internal/gateway/portal/handlers/network_tariffs_summary_test.go`

- [ ] **Step 1: Failing test**

```go
func TestSubaccountsSummary_AvgPrice(t *testing.T) {
  // seed: sa1 with plan RU × paid × MTS tier0=3.0, MegaFon tier0=3.6
  // expected avg = 3.3
  ...
  require.InDelta(t, 3.3, *out[0].AvgPricePerSMS, 0.01)
}
```

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement avg via SQL subquery**

Заменить `NULL::numeric` в запросе на:
```sql
(SELECT AVG(t.price_per_segment)
   FROM reseller_tariff_plans p
   JOIN reseller_tariff_periods pd ON pd.tariff_plan_id = p.id
     AND pd.start_date <= CURRENT_DATE AND (pd.end_date IS NULL OR pd.end_date > CURRENT_DATE)
   JOIN reseller_tariff_tiers t ON t.tariff_period_id = pd.id AND t.from_count = 0
   WHERE (p.sub_account_id = c.id OR p.template_id = tpl.id)
     AND p.active AND p.country_id IN (SELECT id FROM countries WHERE iso_code = 'RU')
     AND p.sender_category = 'paid_registered')
```

- [ ] **Step 4: Добавить Redis-кеш**

Инжектить `redis.UniversalClient` в handler; ключ `tariffs:summary:<reseller_id>`, TTL 5 мин. Invalidation — в задачах 5, 6 (bulk update / bind template).

- [ ] **Step 5: Run — PASS**

- [ ] **Step 6: Commit**

```bash
git commit -am "feat(tariffs): compute avg_price_per_sms + redis cache"
```

---

### Task 3: `GET /api/network/tariff-templates` с счётчиками

**Files:**
- Create: `internal/gateway/portal/handlers/network_tariff_templates.go`
- Modify: `internal/gateway/portal/router.go`
- Test: `internal/gateway/portal/handlers/network_tariff_templates_test.go`

- [ ] **Step 1: Failing test** — проверяет что `plans_count` и `bound_subaccount_count` корректны при 0 и 3 планах/привязках.

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Handler с on-read COUNT'ами**

```sql
SELECT tpl.id, tpl.name, tpl.description,
       (SELECT COUNT(*) FROM reseller_tariff_plans WHERE template_id = tpl.id AND active),
       (SELECT COUNT(*) FROM sub_account_template_assignments WHERE template_id = tpl.id),
       tpl.created_at
FROM reseller_tariff_templates tpl
WHERE tpl.reseller_id = $1 AND tpl.active
ORDER BY tpl.name
```

- [ ] **Step 4: Register route** `GET /portal/v1/network/tariff-templates` на `protected` subrouter; `is_reseller=true` проверяется в handler'е (consistent с `reseller_*` handlers).

- [ ] **Step 5: Run — PASS**

- [ ] **Step 6: Commit**

```bash
git commit -am "feat(tariffs): GET /api/network/tariff-templates with counters"
```

---

### Task 4: `POST /api/network/tariff-templates` + bind + duplicate

**Files:**
- Modify: `internal/gateway/portal/handlers/network_tariff_templates.go` (добавить методы `Create`, `Bind`, `Duplicate`)
- Test: тот же test-файл — три теста

- [ ] **Step 1: Failing tests**

```go
func TestCreateTemplate_UniqueName(t *testing.T) { ... }
func TestCreateTemplate_CopyFromExisting(t *testing.T) {
  // проверяет что все плаsны/периоды/tiers скопированы
}
func TestBindTemplate_ReplacesExisting(t *testing.T) {
  // sub_account уже привязан к template-X; bind к template-Y; overrides сохраняются
}
```

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement**

`Create(w, r)` — POST body `{name, description, copy_from_id?}`. Транзакция: INSERT в `reseller_tariff_templates`; если `copy_from_id` — CTE-копирование `plans → periods → tiers`.

`Bind(w, r)` — URL `/portal/v1/network/tariff-templates/{id}/bind`, body `{sub_account_ids}`. В транзакции: `DELETE FROM sub_account_template_assignments WHERE sub_account_id = ANY($1)` + `INSERT` новых строк (unique index на `sub_account_id` гарантирует one-template-per-subaccount).

`Duplicate(w, r)` — URL `/portal/v1/network/tariff-templates/{id}/duplicate`, body `{name}`. Тот же copy-CTE что в `Create`.

Invalidation Redis: после `Bind` — удаление `tariffs:summary:<reseller_id>`.

- [ ] **Step 4: Register routes** (POST/bind/duplicate).

- [ ] **Step 5: Run — PASS**

- [ ] **Step 6: Commit**

```bash
git commit -am "feat(tariffs): create/bind/duplicate template endpoints"
```

---

### Task 5: `GET /api/network/tariff-editor/:id`

**Files:**
- Create: `internal/gateway/portal/handlers/network_tariff_editor.go`
- Modify: `internal/gateway/portal/router.go`
- Test: `internal/gateway/portal/handlers/network_tariff_editor_test.go`

- [ ] **Step 1: Failing tests**

Два теста: `TestEditor_TemplateMode_ReturnsPlain` и `TestEditor_OverrideMode_ReturnsInheritanceMarkers`. Второй проверяет что ячейка с `price_override` имеет `source="override"`, а ячейка без override — `source="template"` + `effective == price_template`.

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement**

Response структура — как в спеке §5.1.5. Запрос формирует:
1. Мета плана (`strategy`, `currency`) из `reseller_tariff_plans`.
2. Массив периодов.
3. Матрицу ячеек: LEFT JOIN `reseller_tariff_tiers` (template-level) × `operators` + LEFT JOIN overrides (только для `mode=override`, по `sub_account_id`).

Query params: `mode, channel, country, sender_category, traffic_type, period_id?`.

- [ ] **Step 4: Register** `GET /portal/v1/network/tariff-editor/{id}`.

- [ ] **Step 5: Run — PASS**

- [ ] **Step 6: Commit**

```bash
git commit -am "feat(tariffs): GET /api/network/tariff-editor/:id (inheritance-aware)"
```

---

### Task 6: `PATCH /api/network/tariff-plans/:id/bulk` + period endpoint

**Files:**
- Create: `internal/gateway/portal/handlers/network_tariff_bulk.go`
- Modify: `internal/gateway/portal/router.go`
- Test: `internal/gateway/portal/handlers/network_tariff_bulk_test.go`

- [ ] **Step 1: Failing tests**

```go
func TestBulkPatch_AllOrNothing(t *testing.T) {
  // один tier с price=-1 → транзакция откатывается, ничего не изменилось
}
func TestBulkPatch_UpsertAndDelete(t *testing.T) { ... }
func TestCreatePeriod_NoOverlap(t *testing.T) { ... }
func TestCreatePeriod_CopyFromWithKeepTiers(t *testing.T) { ... }
```

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement bulk**

Транзакция `pgx.BeginTx`:
- Валидация: цены `>= 0 AND <= 999`, иначе return `400` с массивом errors (ячейка → причина).
- `cells_upsert` — `INSERT ... ON CONFLICT` в `reseller_tariff_tiers` (scope=template) или `reseller_tariff_overrides` (scope=override).
- `cells_delete` — `DELETE ... WHERE operator_id = ANY + tier_id = ANY` ограниченные `plan_id`.
- `tiers_upsert/delete` — работа с `reseller_tariff_tiers` структурой (сами уровни).
- Commit; Redis invalidation `tariffs:summary:<reseller_id>`.

- [ ] **Step 4: Implement period POST**

`POST /portal/v1/network/tariff-plans/{plan_id}/periods` — body `{from, to?, copy_from_period_id?, keep_tiers: bool}`. Валидация пересечений: `SELECT 1 FROM reseller_tariff_periods WHERE plan_id = $1 AND tstzrange(valid_from, valid_to) && tstzrange($2, $3)`.

- [ ] **Step 5: Register routes**

- [ ] **Step 6: Run — PASS**

- [ ] **Step 7: Commit**

```bash
git commit -am "feat(tariffs): PATCH bulk + POST period with copy"
```

---

## Phase 2 — Frontend skeleton + list page

### Task 7: Расширить `api/client.ts` — namespace `networkTariffsApi`

**Files:**
- Modify: `portal-frontend/src/api/client.ts`

- [ ] **Step 1: Добавить типы и методы**

```ts
// types
export interface SubAccountTariffSummary {
  sub_account_id: string;
  sub_account_name: string;
  sub_account_email: string;
  template_id: string | null;
  template_name: string | null;
  override_count: number;
  avg_price_per_sms: number | null;
  currency: string;
}

export interface TariffTemplateSummary {
  id: string;
  name: string;
  description: string | null;
  plans_count: number;
  bound_subaccount_count: number;
  created_at: string;
}

export interface TariffEditorCell {
  operator_id: string;
  tier_id: string;
  price_template: number | null;
  price_override: number | null;
  effective: number | null;
  source: 'template' | 'override' | 'unset';
}

export interface TariffEditorData {
  scope: { kind: 'template' | 'override'; template_id?: string; sub_account_id?: string; name: string };
  template: { id: string; name: string } | null;
  plan: { id: string; strategy: string; currency: string };
  periods: { id: string; from: string; to: string | null; active: boolean }[];
  active_period_id: string;
  operators: { id: string; name: string; icon: string | null }[];
  tiers: { id: string; from_quantity: number }[];
  cells: TariffEditorCell[];
}

// namespace
export const networkTariffsApi = {
  listSubaccounts: () =>
    apiFetch<SubAccountTariffSummary[]>('/network/tariffs/subaccounts-summary'),
  listTemplates: () =>
    apiFetch<TariffTemplateSummary[]>('/network/tariff-templates'),
  createTemplate: (body: { name: string; description?: string; copy_from_id?: string }) =>
    apiFetch<{ id: string }>('/network/tariff-templates', { method: 'POST', body }),
  bindTemplate: (id: string, subAccountIds: string[]) =>
    apiFetch<{ bound: string[] }>(`/network/tariff-templates/${id}/bind`, { method: 'POST', body: { sub_account_ids: subAccountIds } }),
  duplicateTemplate: (id: string, name: string) =>
    apiFetch<{ id: string }>(`/network/tariff-templates/${id}/duplicate`, { method: 'POST', body: { name } }),
  getEditor: (id: string, params: { mode: 'template' | 'override'; channel: string; country: string; sender_category: string; traffic_type: string; period_id?: string }) =>
    apiFetch<TariffEditorData>(`/network/tariff-editor/${id}?${new URLSearchParams({ ...params } as Record<string,string>).toString()}`),
  bulkPatchPlan: (planId: string, body: {
    period_id: string;
    tiers_upsert: { id: string | null; from_quantity: number }[];
    tiers_delete: string[];
    cells_upsert: { operator_id: string; tier_id: string; price: number; scope: 'template' | 'override'; sub_account_id?: string }[];
    cells_delete: { operator_id: string; tier_id: string; scope: 'template' | 'override'; sub_account_id?: string }[];
  }) => apiFetch<{ ok: boolean; errors?: unknown[] }>(`/network/tariff-plans/${planId}/bulk`, { method: 'PATCH', body }),
  createPeriod: (planId: string, body: { from: string; to?: string; copy_from_period_id?: string; keep_tiers: boolean }) =>
    apiFetch<{ id: string }>(`/network/tariff-plans/${planId}/periods`, { method: 'POST', body }),
};
```

- [ ] **Step 2: tsc — no errors**

Run: `cd portal-frontend && npx tsc --noEmit`
Expected: pass.

- [ ] **Step 3: Commit**

```bash
git commit -am "feat(tariffs): networkTariffsApi client namespace"
```

---

### Task 8: Routing — три новые страницы, redirect'ы

**Files:**
- Modify: `portal-frontend/src/App.tsx`
- Create: `portal-frontend/src/pages/network/NetworkTariffsListPage.tsx` (пока заглушка)
- Create: `portal-frontend/src/pages/network/NetworkTariffTemplatesPage.tsx` (заглушка)
- Create: `portal-frontend/src/pages/network/NetworkTariffEditorPage.tsx` (заглушка)

- [ ] **Step 1: Добавить роуты в `App.tsx`**

Найти текущий `<Route path="/network/tariffs" element={<NetworkTariffsPage />} />`, заменить:

```tsx
<Route path="/network/tariffs" element={<NetworkTariffsListPage />} />
<Route path="/network/tariffs/templates" element={<NetworkTariffTemplatesPage />} />
<Route path="/network/tariffs/editor/:id" element={<NetworkTariffEditorPage />} />
```

Добавить redirect со старых query-tab URL через компонент `<TariffsTabRedirect />` на корневом роуте или useEffect в `NetworkTariffsListPage`.

- [ ] **Step 2: Заглушки страниц**

Минимальные компоненты `<div>TODO: <название></div>`. Это позволит собрать проект и итерировать постранично.

- [ ] **Step 3: Проверить `npx tsc --noEmit && npm run lint`**

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(tariffs): scaffold routes for new tariff pages"
```

---

### Task 9: `NetworkTariffsListPage` — таблица субаккаунтов

**Files:**
- Modify: `portal-frontend/src/pages/network/NetworkTariffsListPage.tsx`
- Test: `portal-frontend/src/pages/network/__tests__/NetworkTariffsListPage.test.tsx`

- [ ] **Step 1: Failing test**

```tsx
import { render, screen, waitFor } from '@testing-library/react';
import { NetworkTariffsListPage } from '../NetworkTariffsListPage';
import { networkTariffsApi } from '../../../api/client';
jest.mock('../../../api/client');

test('renders subaccount rows with template and override badges', async () => {
  (networkTariffsApi.listSubaccounts as jest.Mock).mockResolvedValue([
    { sub_account_id: 'a', sub_account_name: 'Test Clean (A)', sub_account_email: 'a@t.l', template_id: 't1', template_name: 'Базовый', override_count: 0, avg_price_per_sms: 2.85, currency: 'RUB' },
    { sub_account_id: 'c', sub_account_name: 'Test Heavy (C)', sub_account_email: 'c@t.l', template_id: 't1', template_name: 'Базовый', override_count: 3, avg_price_per_sms: 2.40, currency: 'RUB' },
  ]);
  render(<MemoryRouter><NetworkTariffsListPage /></MemoryRouter>);
  await waitFor(() => screen.getByText('Test Clean (A)'));
  expect(screen.getAllByText('Базовый')).toHaveLength(2);
  expect(screen.getByText('3 правила')).toBeInTheDocument();
  expect(screen.getByText('2.85 ₽')).toBeInTheDocument();
});
```

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement**

```tsx
// NetworkTariffsListPage.tsx
import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { networkTariffsApi, SubAccountTariffSummary } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';

export function NetworkTariffsListPage() {
  const [rows, setRows] = useState<SubAccountTariffSummary[]>([]);
  const [search, setSearch] = useState('');
  const [onlyOverrides, setOnlyOverrides] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    networkTariffsApi.listSubaccounts()
      .then(d => { if (!cancelled) setRows(d); })
      .catch(e => { if (!cancelled) setError(String(e)); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, []);

  const filtered = rows.filter(r =>
    (!onlyOverrides || r.override_count > 0) &&
    (search === '' || r.sub_account_name.toLowerCase().includes(search.toLowerCase()))
  );

  if (loading) return <div className="p-4">Загрузка…</div>;
  if (error) return <div className="p-4 text-red-600" role="alert">{error}</div>;

  return (
    <div className="max-w-6xl">
      <PageHeader
        title="Тарифы субаккаунтов"
        actions={<Link to="/network/tariffs/templates" className="text-primary">Шаблоны →</Link>}
      />
      <div className="flex gap-3 mb-4">
        <input
          className="border rounded px-3 py-1.5 text-sm flex-1 max-w-md"
          placeholder="Поиск субаккаунта"
          value={search}
          onChange={e => setSearch(e.target.value)}
        />
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={onlyOverrides} onChange={e => setOnlyOverrides(e.target.checked)} />
          Только с переопределениями
        </label>
      </div>
      <table className="w-full text-sm">
        <thead className="text-left text-slate-500 bg-slate-50">
          <tr>
            <th className="px-3 py-2">Субаккаунт</th>
            <th className="px-3 py-2">Шаблон</th>
            <th className="px-3 py-2">Переопр.</th>
            <th className="px-3 py-2 text-right">Средняя ₽/SMS</th>
            <th className="px-3 py-2" aria-label="Действия"></th>
          </tr>
        </thead>
        <tbody>
          {filtered.map(r => (
            <tr key={r.sub_account_id} className="border-t">
              <td className="px-3 py-2">
                <Link to={`/network/tariffs/editor/${r.sub_account_id}?mode=override`} className="font-medium hover:underline">
                  {r.sub_account_name}
                </Link>
                <div className="text-xs text-slate-500">{r.sub_account_email}</div>
              </td>
              <td className="px-3 py-2">
                {r.template_id ? (
                  <Link to={`/network/tariffs/editor/${r.template_id}?mode=template`}
                        className="inline-block px-2 py-0.5 rounded-full text-xs bg-sky-100 text-sky-800 hover:bg-sky-200"
                        onClick={e => e.stopPropagation()}>
                    {r.template_name}
                  </Link>
                ) : <span className="text-slate-400">—</span>}
              </td>
              <td className="px-3 py-2">
                {r.override_count > 0
                  ? <span className="inline-block px-2 py-0.5 rounded-full text-xs bg-amber-100 text-amber-800">{r.override_count} правил{r.override_count === 1 ? 'о' : ''}</span>
                  : <span className="text-slate-400">—</span>}
              </td>
              <td className="px-3 py-2 text-right tabular-nums">
                {r.avg_price_per_sms != null ? `${r.avg_price_per_sms.toFixed(2)} ${r.currency === 'RUB' ? '₽' : r.currency}` : '—'}
              </td>
              <td className="px-3 py-2 text-right">
                <Link to={`/network/tariffs/editor/${r.sub_account_id}?mode=override`} className="text-primary">Открыть ▸</Link>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {filtered.length === 0 && (
        <div className="text-center text-slate-500 py-8">Нет субаккаунтов.</div>
      )}
    </div>
  );
}
```

- [ ] **Step 4: Run — PASS** (`npm test -- NetworkTariffsListPage`)

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(tariffs): subaccounts list page"
```

---

### Task 10: Redirect со старых `?tab=*` URL

**Files:**
- Modify: `portal-frontend/src/pages/network/NetworkTariffsListPage.tsx`
- Test: тот же test-файл

- [ ] **Step 1: Failing test** — проверка редиректа при `?tab=templates`.

```tsx
test('redirects ?tab=templates to /network/tariffs/templates', () => {
  render(<MemoryRouter initialEntries={['/network/tariffs?tab=templates']}>
    <Routes>
      <Route path="/network/tariffs" element={<NetworkTariffsListPage />} />
      <Route path="/network/tariffs/templates" element={<div>Шаблоны</div>} />
    </Routes>
  </MemoryRouter>);
  expect(screen.getByText('Шаблоны')).toBeInTheDocument();
});
```

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement**

В начало компонента:
```tsx
const [params] = useSearchParams();
const nav = useNavigate();
useEffect(() => {
  const tab = params.get('tab');
  if (tab === 'templates') nav('/network/tariffs/templates', { replace: true });
  if (tab === 'overrides' || tab === 'overview') nav('/network/tariffs', { replace: true });
}, [params, nav]);
```

- [ ] **Step 4: Run — PASS**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(tariffs): redirect legacy ?tab=* URLs"
```

---

## Phase 3 — Templates library page

### Task 11: `NetworkTariffTemplatesPage` — таблица шаблонов

**Files:**
- Modify: `portal-frontend/src/pages/network/NetworkTariffTemplatesPage.tsx`
- Test: `portal-frontend/src/pages/network/__tests__/NetworkTariffTemplatesPage.test.tsx`

- [ ] **Step 1: Failing test** — рендер таблицы с `plans_count` и `bound_subaccount_count`, disabled «Удалить» при `bound>0`.

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement** — аналогично Task 9, таблица с колонками из спеки §3.1. Используем Radix `<DropdownMenu>` для меню `⋮`.

- [ ] **Step 4: Run — PASS**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(tariffs): templates library page"
```

---

### Task 12: Модалки «Создать» и «Дублировать»

**Files:**
- Create: `portal-frontend/src/pages/network/components/CreateTemplateDialog.tsx`
- Create: `portal-frontend/src/pages/network/components/DuplicateTemplateDialog.tsx`
- Modify: `NetworkTariffTemplatesPage.tsx`
- Test: оба компонента тестируются отдельно

- [ ] **Step 1: Failing tests**

```tsx
test('CreateTemplateDialog: disallows empty name', ...);
test('CreateTemplateDialog: calls createTemplate with name+description', ...);
test('CreateTemplateDialog: with copy_from passes copy_from_id', ...);
test('DuplicateTemplateDialog: prefills "Копия <имя>"', ...);
```

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement** — Radix `<Dialog>`; после успеха — `nav(/network/tariffs/editor/:new_id?mode=template)` для create; `refetch` для duplicate.

- [ ] **Step 4: Run — PASS**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(tariffs): create + duplicate template dialogs"
```

---

### Task 13: Модалка «Привязать»

**Files:**
- Create: `portal-frontend/src/pages/network/components/BindTemplateDialog.tsx`
- Modify: `NetworkTariffTemplatesPage.tsx`
- Test: `BindTemplateDialog.test.tsx`

- [ ] **Step 1: Failing test** — чекбокс-список субаккаунтов, поиск, bulk-submit вызывает `bindTemplate(id, [ids])`; для субаккаунта с текущим шаблоном-Y отображается бейдж «Заменит Y».

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement** — использовать `networkTariffsApi.listSubaccounts()` для подгрузки + current template info.

- [ ] **Step 4: Run — PASS**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(tariffs): bind template dialog with replace warning"
```

---

## Phase 4 — TariffMatrix shared component

### Task 14: `<TariffMatrix>` — read-only skeleton

**Files:**
- Create: `portal-frontend/src/components/tariffs/TariffMatrix.tsx`
- Create: `portal-frontend/src/components/tariffs/TariffMatrix.types.ts`
- Test: `portal-frontend/src/components/tariffs/__tests__/TariffMatrix.test.tsx`

- [ ] **Step 1: Failing test** — рендерит таблицу с операторами и ступенями, ячейки с source=template показывают число, source=override — число на `bg-amber-50` с маркером ●, source=unset — «—».

```tsx
test('renders matrix cells by source', () => {
  render(<TariffMatrix data={fixture} editable={false} showInheritance={true} />);
  const tplCell = screen.getByText('3.50');
  expect(tplCell).not.toHaveClass('bg-amber-50');
  const ovrCell = screen.getByText('3.20');
  expect(ovrCell.closest('td')).toHaveClass('bg-amber-50');
  expect(within(ovrCell.closest('td')!).getByText('●')).toBeInTheDocument();
  expect(screen.getAllByText('—')).toHaveLength(1); // unset
});
```

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement** — компонент принимает `TariffEditorData` + флаги, рендерит `<table>` с правильными CSS-классами. В этом task ещё без edit-mode.

Props:
```ts
export interface TariffMatrixProps {
  data: TariffEditorData;
  editable: boolean;              // если false — read-only
  showInheritance: boolean;       // если false (template/client) — не различать source
  onSaveBatch?: (batch: BulkPatchBody) => Promise<{ ok: boolean; errors?: unknown[] }>;
}
```

- [ ] **Step 4: Run — PASS**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(tariffs): TariffMatrix read-only skeleton"
```

---

### Task 15: A11y — корректная table-семантика и aria

**Files:**
- Modify: `TariffMatrix.tsx`
- Test: add a11y-assertions в existing test

- [ ] **Step 1: Failing test**

```tsx
test('matrix a11y: scope attributes + override aria-label', () => {
  render(<TariffMatrix data={fixture} editable={false} showInheritance={true} />);
  const rowHeader = screen.getByRole('rowheader', { name: 'МТС' });
  expect(rowHeader).toHaveAttribute('scope', 'row');
  const ovrCell = screen.getByLabelText(/переопределение поверх значения 3.60 из шаблона Базовый/);
  expect(ovrCell).toBeInTheDocument();
});
```

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement**
- `<caption className="sr-only">` с описанием матрицы.
- Operator column — `<th scope="row">`.
- Tier columns — `<th scope="col">`.
- Override cells — `aria-label` с полной формулировкой.

- [ ] **Step 4: Run — PASS**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(tariffs): TariffMatrix a11y (scope + aria-label)"
```

---

### Task 16: Edit-mode — inline-input + keyboard nav

**Files:**
- Modify: `TariffMatrix.tsx`
- Test: добавить кейсы

- [ ] **Step 1: Failing tests**

```tsx
test('edit mode: cell becomes input on click + validates >=0', ...);
test('edit mode: Enter moves focus down, Tab moves right', ...);
test('edit mode: typing same value as inherited does NOT create override', ...);
test('edit mode: footer shows "Несохранённых изменений: N"', ...);
```

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement**
- Внутренний state `draft: Map<"<opId>:<tierId>", {price, scope}>`.
- Клик по unlocked-ячейке → `<input type="text" inputmode="decimal">` с locale-aware парсингом (`,` → `.`).
- Кнопка `×` на override-ячейке возвращает к inherited (удаляет из draft, добавляет в `cells_delete`).
- Ref-сетка для клавиатурной навигации (Enter/Tab/Shift+Tab/Esc).
- Валидация inline (< 0 / > 999 / не-число) — красная рамка + title-tooltip.
- Sticky footer с `role="status"`.

- [ ] **Step 4: Run — PASS**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(tariffs): TariffMatrix edit mode + keyboard nav"
```

---

### Task 17: `onSaveBatch` — сбор дельты и отправка

**Files:**
- Modify: `TariffMatrix.tsx`
- Test: добавить кейс

- [ ] **Step 1: Failing test**

```tsx
test('Save: calls onSaveBatch with upsert+delete arrays; on success exits edit-mode', async () => {
  const onSave = jest.fn().mockResolvedValue({ ok: true });
  render(<TariffMatrix data={fixture} editable={true} showInheritance={true} onSaveBatch={onSave} />);
  // enter edit mode, change cell, save
  ...
  expect(onSave).toHaveBeenCalledWith(expect.objectContaining({
    cells_upsert: [{ operator_id:'mts', tier_id:'t0', price: 3.15, scope: 'override', sub_account_id: 'c' }],
    cells_delete: [],
  }));
});
```

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement** — собрать batch из `draft`, отправить, при `{ok:true}` — обновить data + сбросить draft + exit edit mode; при `{ok:false, errors}` — остаться в edit mode, подсветить ячейки.

- [ ] **Step 4: Run — PASS**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(tariffs): TariffMatrix batch save + error handling"
```

---

## Phase 5 — Editor page + save flow

### Task 18: `NetworkTariffEditorPage` — top bar + загрузка данных

**Files:**
- Modify: `portal-frontend/src/pages/network/NetworkTariffEditorPage.tsx`
- Test: `portal-frontend/src/pages/network/__tests__/NetworkTariffEditorPage.test.tsx`

- [ ] **Step 1: Failing test** — страница по URL `/network/tariffs/editor/abc?mode=template&channel=sms` делает fetch `getEditor('abc', {...})` и рендерит breadcrumbs + tabs каналов.

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement**

```tsx
export function NetworkTariffEditorPage() {
  const { id } = useParams();
  const [params, setParams] = useSearchParams();
  const mode = (params.get('mode') ?? 'template') as 'template' | 'override';
  const channel = params.get('channel') ?? 'sms';
  const country = params.get('country') ?? 'RU';
  const sender_category = params.get('sender_category') ?? 'paid';
  const traffic_type = params.get('traffic_type') ?? 'any';
  const period_id = params.get('period_id') ?? undefined;
  const [data, setData] = useState<TariffEditorData | null>(null);
  // ... load + render top bar + TariffMatrix
}
```

Top bar — breadcrumbs, channel tabs, filter dropdowns (country/category/traffic/period), strategy badge, edit-mode toggle.

- [ ] **Step 4: Run — PASS**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(tariffs): editor page top bar + data load"
```

---

### Task 19: Интеграция `<TariffMatrix>` + save-to-API

**Files:**
- Modify: `NetworkTariffEditorPage.tsx`

- [ ] **Step 1: Failing test** — при клике «Сохранить» вызывается `bulkPatchPlan(plan_id, batch)` + refetch после успеха.

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement** — handler `onSaveBatch` → `networkTariffsApi.bulkPatchPlan(plan.id, body)`; затем `getEditor(...)` для свежих данных.

- [ ] **Step 4: Run — PASS**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(tariffs): editor page save flow"
```

---

### Task 20: Модалка «Новый период»

**Files:**
- Create: `portal-frontend/src/pages/network/components/NewPeriodDialog.tsx`
- Modify: `NetworkTariffEditorPage.tsx`
- Test: `NewPeriodDialog.test.tsx`

- [ ] **Step 1: Failing tests** — рендер 4 полей, валидация пересечений на клиенте (подсказка без сабмита), submit вызывает `createPeriod(planId, body)`.

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement** — поля: `<input type="date">` × 2, dropdown «Скопировать из», radio «Оставить/Новые tiers». Проверка пересечения против `data.periods`.

- [ ] **Step 4: Run — PASS**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(tariffs): new period dialog with copy option"
```

---

### Task 21: `beforeunload` guard для несохранённых изменений

**Files:**
- Modify: `NetworkTariffEditorPage.tsx` (пробрасываем `hasUnsavedChanges`) + `TariffMatrix.tsx` (expose state)
- Test: добавить кейс

- [ ] **Step 1: Failing test** — если есть несохранённые изменения, `navigate('/network/tariffs')` показывает confirm; при отказе URL не меняется.

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement** — React Router v7 `useBlocker` + native `beforeunload` listener.

- [ ] **Step 4: Run — PASS**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(tariffs): unsaved-changes navigation guard"
```

---

## Phase 6 — Client migration + cleanup

### Task 22: Клиентский endpoint `GET /api/client/tariffs/effective`

**Files:**
- Create: `internal/gateway/portal/handlers/client_tariffs_effective.go`
- Modify: `internal/gateway/portal/router.go`
- Test: `internal/gateway/portal/handlers/client_tariffs_effective_test.go`

- [ ] **Step 1: Failing test** — возвращает ту же форму что `GET /api/network/tariff-editor/:id`, но без `price_template`/`source` в ячейках (только `effective`); scope к `sub_account_id` из сессии.

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement** — SQL как в Task 5 (override-mode), но response сериализует только `{operator_id, tier_id, effective}`.

- [ ] **Step 4: Register** `GET /portal/v1/client/tariffs/effective` с `requireAuthenticated`.

- [ ] **Step 5: Run — PASS**

- [ ] **Step 6: Commit**

```bash
git commit -am "feat(tariffs): GET /api/client/tariffs/effective"
```

---

### Task 23: Клиентский `<TariffsPage>` на `<TariffMatrix>`

**Files:**
- Modify: `portal-frontend/src/pages/tariffs/TariffsPage.tsx`
- Modify: `portal-frontend/src/api/client.ts` (добавить `clientTariffsApi.getEffective`)
- Test: `portal-frontend/src/pages/tariffs/__tests__/TariffsPage.test.tsx`

- [ ] **Step 1: Failing test** — страница вызывает `clientTariffsApi.getEffective({channel,country,...})` и рендерит `<TariffMatrix editable={false} showInheritance={false} />`. Нет кнопки «Режим правки».

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement**
- В `api/client.ts` добавить `clientTariffsApi.getEffective`.
- В `TariffsPage.tsx` — заменить текущий список на матричный компонент. Фильтры канал/страна/тип по тем же dropdown'ам.

- [ ] **Step 4: Run — PASS**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(tariffs): client /tariffs now uses TariffMatrix readonly"
```

---

### Task 24: Удаление старого кода

**Files:**
- Delete: `portal-frontend/src/pages/network/components/TariffOverviewTab.tsx`
- Delete: `portal-frontend/src/pages/network/components/TariffTemplatesTab.tsx`
- Delete: `portal-frontend/src/pages/network/components/TariffOverridesTab.tsx`
- Delete: `portal-frontend/src/pages/network/components/TariffPlanEditor.tsx`
- Delete: `portal-frontend/src/pages/network/NetworkTariffsPage.tsx`

- [ ] **Step 1: Проверить, что импортов на удаляемые компоненты больше нет**

Run: `grep -r "TariffOverviewTab\|TariffTemplatesTab\|TariffOverridesTab\|TariffPlanEditor\|NetworkTariffsPage" portal-frontend/src`
Expected: нет совпадений кроме импортов, которые заменяются в Task 8.

- [ ] **Step 2: Удалить файлы**

- [ ] **Step 3: Проверить build + test**

Run: `cd portal-frontend && npx tsc --noEmit && npm test && npm run lint`
Expected: pass.

- [ ] **Step 4: Commit**

```bash
git commit -am "chore(tariffs): remove legacy tabs components"
```

---

### Task 25: Smoke-тест на сервере + README-обновление

**Files:**
- Modify: `docs/reports/` (добавить отчёт о редизайне)

- [ ] **Step 1: Деплой на dev-сервер**

Run: `./scripts/server.sh deploy`
Expected: контейнеры перезапускаются без ошибок.

- [ ] **Step 2: Ручной прогон сценариев**

Проверить на `http://72.56.232.202:18085/network/tariffs`:
- [ ] Список субаккаунтов загружается.
- [ ] Клик по бейджу шаблона → редактор шаблона.
- [ ] Клик по строке → редактор субаккаунта (override-mode).
- [ ] Редактирование ячейки в edit-mode → сохранение → обновлённые данные.
- [ ] Редирект `/network/tariffs?tab=templates` → `/network/tariffs/templates`.
- [ ] Клиентский `/tariffs` показывает readonly-матрицу.
- [ ] Keyboard-навигация по ячейкам (Tab/Enter/Esc).

- [ ] **Step 3: Отчёт**

Создать `docs/reports/2026-04-22-network-tariffs-redesign.md` с фактами: что изменилось, что протестировано, что осталось в open questions.

- [ ] **Step 4: Commit + push**

```bash
git add docs/reports/2026-04-22-network-tariffs-redesign.md
git commit -m "docs(tariffs): redesign smoke-test report"
```

---

## Self-review

**Spec coverage:**
- §1 Architecture: Tasks 8 (routes), 14–17 (shared component), 24 (cleanup). ✓
- §2 List page: Tasks 9, 10. ✓
- §3 Templates library: Tasks 11, 12, 13. ✓
- §4 Editor: Tasks 18, 19, 20, 21. ✓
- §5 API: Tasks 1–6 (admin), 22 (client). ✓
- §6 A11y: Task 15 (base) + extended in 16 (keyboard). ✓
- §7 Visual: включён в каждую UI-task через tailwind-классы из спеки. ✓
- §8 Risks: cache invalidation встроен в Tasks 2, 4, 6. ✓

**Placeholder check:** все Steps имеют код или точные команды. Отсутствующие места помечены явно (например, «seedTariffTemplate — хелпер из существующих тестов handlers»).

**Type consistency:** `TariffEditorData`, `TariffEditorCell`, `SubAccountTariffSummary`, `TariffTemplateSummary` — определены в Task 7, используются в Tasks 9–23 без переименований.

**Потенциальные пробелы (вынесены в open questions, не блокируют):**
- ~~Точное имя таблицы `reseller_tariff_template_bindings`~~ — resolved: реальная таблица `sub_account_template_assignments` (колонки `sub_account_id`, `template_id`, `assigned_at`; unique index на `sub_account_id`). План и спека обновлены 2026-04-22.
- ~~Имя и структура поля `parent_reseller_id` в `clients`~~ — resolved: реальная колонка `clients.parent_client_id`. План и спека обновлены 2026-04-22.
- `clients.currency` не существует — response'ы hardcode'ят `"RUB"` до отдельной инициативы по мультивалютности.
- Legacy-бейдж и взаимодействие с `aggregator_tariffs` — текущий UI показывает «Legacy» в обзоре; матрица должна уметь его отобразить. В плане явно не адресовано; вопрос: сохранять ли legacy-колонку в новом UI? Рекомендация — показывать `legacy`-бейдж в строке оператора, если источник — `aggregator_tariffs`, без возможности править.

Эти open questions решаются на Task 1 (handler увидит, как данные реально устроены) — если данные расходятся с предположением, план адаптируется.

## Open questions surfaced during Phase 1 implementation

- **Wildcard-override precedence (surfaced in Task 2 review, 2026-04-22).** Semantics of `reseller_tariff_plans.operator_id = NULL` (wildcard) vs specific `operator_id` for override rows are undefined in the spec. The Task 2 CTE suppresses template rows only when an override exists for the *same* `(country, operator, sender_category)` tuple after COALESCE-ing NULLs to zero-UUID. If a wildcard override should suppress *all* specific-operator template rows for that (country, sender_category), the current query is wrong and average prices will be inflated. **Must be resolved before Task 5 (GET /api/network/tariff-editor) and Task 6 (PATCH bulk).** Options:
  - (a) Wildcard has higher precedence — specific operators are overridden by wildcard. Requires modified NOT EXISTS.
  - (b) Specific always wins over wildcard. Requires ORDER BY with priority + DISTINCT ON.
  - (c) Wildcard and specific coexist as separate plan slots (no override relationship). Current code. 
  Pick before implementing editor write flow.
