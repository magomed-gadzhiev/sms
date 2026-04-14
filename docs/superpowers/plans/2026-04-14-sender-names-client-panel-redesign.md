# Sender Names Client Panel Redesign — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign the operator registration flow in the client portal's sender names section — replace modals with dedicated pages and add a table-based bulk operator registration screen.

**Architecture:** Three frontend pages (`/sender-names`, `/sender-names/:id`, `/sender-names/:id/operators`) replace the current modal-in-modal pattern. Three new backend REST endpoints expose operators list with registration types, per-sender-name operator registrations, and bulk registration creation. Backend queries `operators` and `sender_registrations` tables directly via pgx pool.

**Tech Stack:** Go 1.24 + gorilla/mux + pgx/v5 (backend), TypeScript 5.7 + React 19 + React Router + Tailwind CSS 4.2 (frontend)

**Spec:** `docs/superpowers/specs/2026-04-14-sender-names-client-panel-redesign.md`

---

## File Structure

### Backend (modify)

| File | Responsibility |
|------|---------------|
| `internal/gateway/portal/handlers/sender_names.go` | Add `pool` field + 3 new handler methods |
| `internal/gateway/portal/router/router.go` | Add 3 new routes under `/sender-names` and `/operators` |
| `cmd/portal-gateway/main.go` | Pass `dbPool` to `SenderNameHandlers` via new `SetPool` method |

### Frontend (modify)

| File | Responsibility |
|------|---------------|
| `portal-frontend/src/api/client.ts` | Add `operatorsApi` object + `OperatorRegistration` types |
| `portal-frontend/src/App.tsx` | Add 2 new route entries + lazy imports |
| `portal-frontend/src/pages/sender-names/SenderNamesPage.tsx` | Remove detail/register modals, navigate to detail page on row click |

### Frontend (create)

| File | Responsibility |
|------|---------------|
| `portal-frontend/src/pages/sender-names/SenderNameDetailPage.tsx` | Sender name detail page with status, history, operator pills |
| `portal-frontend/src/pages/sender-names/SenderNameOperatorsPage.tsx` | Table of operators with checkboxes, select for type, bulk register button |

---

## Task 1: Backend — Add pool + ListOperators handler

**Files:**
- Modify: `internal/gateway/portal/handlers/sender_names.go`

- [ ] **Step 1: Add pool field and SetPool method to SenderNameHandlers**

Add after the existing `SetBillingClients` method (~line 38):

```go
// SetPool устанавливает пул соединений для прямых SQL-запросов
func (h *SenderNameHandlers) SetPool(pool *pgxpool.Pool) {
	h.pool = pool
}
```

Add `pool` field to the struct and the import:

```go
import (
	// ... existing imports ...
	"github.com/jackc/pgx/v5/pgxpool"
)

type SenderNameHandlers struct {
	client        sendernamev1.SenderNameServiceClient
	routingClient routingv1.RoutingServiceClient
	tariffClient  tarificationv1.TarificationServiceClient
	billingClient billingv1.BillingServiceClient
	pool          *pgxpool.Pool
}
```

- [ ] **Step 2: Add ListOperators handler**

Append to `sender_names.go`:

```go
// ListOperators GET /portal/v1/operators
// Возвращает список активных операторов с доступными типами регистрации.
func (h *SenderNameHandlers) ListOperators(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		respondError(w, shared.ErrInternalServer("database pool недоступен"))
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, name, code, supports_paid_sender, supports_free_sender, monthly_tariff_amount
		 FROM operators WHERE active = true ORDER BY name`)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка операторов")
		respondError(w, shared.ErrInternalServer("ошибка получения операторов"))
		return
	}
	defer rows.Close()

	type operatorJSON struct {
		ID                string   `json:"id"`
		Name              string   `json:"name"`
		Slug              string   `json:"slug"`
		RegistrationTypes []string `json:"registration_types"`
		MonthlyTariff     *string  `json:"monthly_tariff_amount"`
	}

	operators := make([]operatorJSON, 0)
	for rows.Next() {
		var id, name, code string
		var supportsPaid, supportsFree bool
		var tariff *string
		if err := rows.Scan(&id, &name, &code, &supportsPaid, &supportsFree, &tariff); err != nil {
			log.Error().Err(err).Msg("ошибка сканирования оператора")
			continue
		}
		types := make([]string, 0, 2)
		if supportsFree {
			types = append(types, "free")
		}
		if supportsPaid {
			types = append(types, "paid")
		}
		operators = append(operators, operatorJSON{
			ID:                id,
			Name:              name,
			Slug:              code,
			RegistrationTypes: types,
			MonthlyTariff:     tariff,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"operators": operators,
	})
}
```

- [ ] **Step 3: Verify compilation**

Run: `cd /c/projects/sms && go build ./internal/gateway/portal/...`
Expected: compiles with no errors

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/handlers/sender_names.go
git commit -m "feat(portal): add SetPool and ListOperators handler to SenderNameHandlers"
```

---

## Task 2: Backend — GetSenderNameOperatorRegistrations handler

**Files:**
- Modify: `internal/gateway/portal/handlers/sender_names.go`

- [ ] **Step 1: Add GetSenderNameOperatorRegistrations handler**

Append to `sender_names.go`:

```go
// GetSenderNameOperatorRegistrations GET /portal/v1/sender-names/{id}/operator-registrations
// Возвращает список регистраций данного имени отправителя у операторов.
func (h *SenderNameHandlers) GetSenderNameOperatorRegistrations(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	if h.pool == nil {
		respondError(w, shared.ErrInternalServer("database pool недоступен"))
		return
	}

	senderNameID := mux.Vars(r)["id"]
	if senderNameID == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT sr.operator_id, o.name AS operator_name, sr.type, sr.status
		 FROM sender_registrations sr
		 JOIN operators o ON o.id = sr.operator_id
		 JOIN sender_names sn ON sn.name = sr.sender_name AND sn.client_id = sr.client_id
		 WHERE sn.id = $1 AND sn.client_id = $2`,
		senderNameID, clientID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения operator registrations")
		respondError(w, shared.ErrInternalServer("ошибка получения регистраций"))
		return
	}
	defer rows.Close()

	type regJSON struct {
		OperatorID   string `json:"operator_id"`
		OperatorName string `json:"operator_name"`
		Type         string `json:"type"`
		Status       string `json:"status"`
	}
	regs := make([]regJSON, 0)
	for rows.Next() {
		var r regJSON
		if err := rows.Scan(&r.OperatorID, &r.OperatorName, &r.Type, &r.Status); err != nil {
			log.Error().Err(err).Msg("ошибка сканирования registration")
			continue
		}
		regs = append(regs, r)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"registrations": regs,
	})
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /c/projects/sms && go build ./internal/gateway/portal/...`
Expected: compiles with no errors

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/handlers/sender_names.go
git commit -m "feat(portal): add GetSenderNameOperatorRegistrations handler"
```

---

## Task 3: Backend — BulkCreateOperatorRegistrations handler

**Files:**
- Modify: `internal/gateway/portal/handlers/sender_names.go`

- [ ] **Step 1: Add BulkCreateOperatorRegistrations handler**

Append to `sender_names.go`:

```go
// BulkCreateOperatorRegistrations POST /portal/v1/sender-names/{id}/operator-registrations
// Создаёт регистрации имени отправителя у нескольких операторов.
func (h *SenderNameHandlers) BulkCreateOperatorRegistrations(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	senderNameID := mux.Vars(r)["id"]
	if senderNameID == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}

	// Получаем имя отправителя по ID
	snResp, err := h.client.GetSenderName(r.Context(), &sendernamev1.GetSenderNameRequest{
		Id: senderNameID, ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	if snResp.SenderName.Status != "approved" {
		respondError(w, shared.ErrInvalidInput("имя отправителя должно быть в статусе approved"))
		return
	}

	var req struct {
		Registrations []struct {
			OperatorID string `json:"operator_id"`
			Type       string `json:"type"`
		} `json:"registrations"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if len(req.Registrations) == 0 {
		respondError(w, shared.ErrInvalidInput("registrations не может быть пустым"))
		return
	}

	if h.tariffClient == nil {
		respondError(w, shared.ErrInternalServer("tarification service недоступен"))
		return
	}

	type resultItem struct {
		OperatorID string `json:"operator_id"`
		ID         string `json:"id,omitempty"`
		Status     string `json:"status"`
		Error      string `json:"error,omitempty"`
	}
	results := make([]resultItem, 0, len(req.Registrations))

	for _, reg := range req.Registrations {
		if reg.OperatorID == "" {
			results = append(results, resultItem{OperatorID: reg.OperatorID, Status: "error", Error: "operator_id обязателен"})
			continue
		}
		regType := reg.Type
		if regType == "" {
			regType = "free"
		}

		regResp, err := h.tariffClient.CreateSenderRegistration(r.Context(), &tarificationv1.CreateSenderRegistrationRequest{
			ClientId:   clientID.String(),
			OperatorId: reg.OperatorID,
			SenderName: snResp.SenderName.Name,
			Type:       regType,
		})
		if err != nil {
			log.Error().Err(err).Str("operator_id", reg.OperatorID).Msg("ошибка создания sender registration")
			results = append(results, resultItem{OperatorID: reg.OperatorID, Status: "error", Error: err.Error()})
			continue
		}

		// Для платных регистраций создаём billing record
		if regType == "paid" && h.routingClient != nil {
			op, opErr := h.routingClient.GetOperator(r.Context(), &routingv1.GetOperatorRequest{Id: reg.OperatorID})
			if opErr != nil {
				log.Error().Err(opErr).Str("operator_id", reg.OperatorID).Msg("не удалось получить тариф оператора для billing")
			} else if op.MonthlyTariffAmount != "" {
				billingResp, billingErr := h.tariffClient.CreateSenderBillingRecord(r.Context(), &tarificationv1.CreateSenderBillingRecordRequest{
					SenderRegistrationId: regResp.Id,
					ClientId:             clientID.String(),
					OperatorId:           reg.OperatorID,
					Amount:               op.MonthlyTariffAmount,
				})
				if billingErr != nil {
					log.Error().Err(billingErr).Str("registration_id", regResp.Id).Msg("не удалось создать billing record")
				} else if !billingResp.GetAlreadyExisted() && h.billingClient != nil {
					_, chargeErr := h.billingClient.DeductCredits(r.Context(), &billingv1.DeductCreditsRequest{
						ClientId:    clientID.String(),
						Amount:      op.MonthlyTariffAmount,
						Currency:    "RUB",
						Description: "Paid sender name monthly fee",
					})
					if chargeErr != nil {
						log.Error().Err(chargeErr).Str("registration_id", regResp.Id).Msg("не удалось списать оплату")
					}
				}
			}
		}

		results = append(results, resultItem{OperatorID: reg.OperatorID, ID: regResp.Id, Status: "created"})
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"results": results,
	})
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /c/projects/sms && go build ./internal/gateway/portal/...`
Expected: compiles with no errors

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/handlers/sender_names.go
git commit -m "feat(portal): add BulkCreateOperatorRegistrations handler"
```

---

## Task 4: Backend — Wire routes and pool

**Files:**
- Modify: `internal/gateway/portal/router/router.go`
- Modify: `cmd/portal-gateway/main.go`

- [ ] **Step 1: Add new routes to router.go**

In `router.go`, after the existing sender-names routes block (after line 313 `senderNames.HandleFunc("/{id}/history", ...)`), add:

```go
	senderNames.HandleFunc("/{id}/operator-registrations", senderNameHandlers.GetSenderNameOperatorRegistrations).Methods("GET")
	senderNames.HandleFunc("/{id}/operator-registrations", senderNameHandlers.BulkCreateOperatorRegistrations).Methods("POST")
```

After the existing `protected.HandleFunc("/operators/{id}/sender-tariff", ...)` line (line 321), add:

```go
	// Operators list with registration types
	protected.HandleFunc("/operators", senderNameHandlers.ListOperators).Methods("GET")
```

- [ ] **Step 2: Pass pool to SenderNameHandlers in main.go**

In `cmd/portal-gateway/main.go`, after the existing `senderNameHandlers.SetBillingClients(...)` call (~line 215-218), add:

```go
	senderNameHandlers.SetPool(dbPool)
```

- [ ] **Step 3: Verify compilation**

Run: `cd /c/projects/sms && go build ./...`
Expected: compiles with no errors

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/router/router.go cmd/portal-gateway/main.go
git commit -m "feat(portal): wire operator registration routes and pool"
```

---

## Task 5: Frontend — API client types and methods

**Files:**
- Modify: `portal-frontend/src/api/client.ts`

- [ ] **Step 1: Add OperatorInfo interface and operatorsApi**

After the `senderTariffApi` block (after line 481), add:

```typescript
// Operators API
export interface OperatorInfo {
  id: string;
  name: string;
  slug: string;
  registration_types: string[];
  monthly_tariff_amount: string | null;
}

export interface OperatorRegistration {
  operator_id: string;
  operator_name: string;
  type: string;
  status: string;
}

export const operatorsApi = {
  list: () =>
    apiFetch<{ operators: OperatorInfo[] }>('/operators'),
};

export const senderNameRegistrationsApi = {
  list: (senderNameId: string) =>
    apiFetch<{ registrations: OperatorRegistration[] }>(
      `/sender-names/${senderNameId}/operator-registrations`,
    ),
  bulkCreate: (senderNameId: string, registrations: { operator_id: string; type: string }[]) =>
    apiFetch<{ results: Array<{ operator_id: string; id?: string; status: string; error?: string }> }>(
      `/sender-names/${senderNameId}/operator-registrations`,
      { method: 'POST', body: JSON.stringify({ registrations }) },
    ),
};
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /c/projects/sms/portal-frontend && npx tsc --noEmit --pretty 2>&1 | head -20`
Expected: no new errors

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/api/client.ts
git commit -m "feat(portal): add operatorsApi and senderNameRegistrationsApi to API client"
```

---

## Task 6: Frontend — SenderNameDetailPage

**Files:**
- Create: `portal-frontend/src/pages/sender-names/SenderNameDetailPage.tsx`

- [ ] **Step 1: Create SenderNameDetailPage component**

```tsx
import { useState, useEffect, useCallback, type FormEvent } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import {
  senderNamesApi,
  senderNameRegistrationsApi,
  operatorsApi,
  ApiError,
  type SenderNameInfo,
  type SenderNameHistoryEntry,
  type OperatorRegistration,
  type OperatorInfo,
} from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Badge } from '../../components/ui/Badge';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { useToast } from '../../components/ui/Toast';

const STATUS_BADGE: Record<string, { variant: 'warning' | 'success' | 'danger' | 'default'; label: string }> = {
  pending: { variant: 'warning', label: 'На модерации' },
  approved: { variant: 'success', label: 'Одобрено' },
  rejected: { variant: 'danger', label: 'Отклонено' },
  deactivated: { variant: 'default', label: 'Деактивировано' },
};

function formatDate(dt: string) {
  return new Date(dt).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
}

export function SenderNameDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const toast = useToast();

  const [senderName, setSenderName] = useState<SenderNameInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Operator registrations
  const [registrations, setRegistrations] = useState<OperatorRegistration[]>([]);
  const [operators, setOperators] = useState<OperatorInfo[]>([]);

  // History
  const [history, setHistory] = useState<SenderNameHistoryEntry[]>([]);
  const [showHistory, setShowHistory] = useState(false);
  const [historyLoading, setHistoryLoading] = useState(false);

  // Resubmit
  const [editName, setEditName] = useState('');
  const [editError, setEditError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError('');
    try {
      const sn = await senderNamesApi.get(id);
      setSenderName(sn);
      setEditName(sn.name);

      if (sn.status === 'approved') {
        const [regsRes, opsRes] = await Promise.all([
          senderNameRegistrationsApi.list(id),
          operatorsApi.list(),
        ]);
        setRegistrations(regsRes.registrations ?? []);
        setOperators(opsRes.operators ?? []);
      }
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { load(); }, [load]);

  const loadHistory = async () => {
    if (!id) return;
    setHistoryLoading(true);
    try {
      const res = await senderNamesApi.getHistory(id);
      setHistory(res.entries ?? []);
      setShowHistory(true);
    } catch {
      // ignore
    } finally {
      setHistoryLoading(false);
    }
  };

  const handleResubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!id || !senderName) return;
    const trimmed = editName.trim();
    if (!trimmed) { setEditError('Имя обязательно'); return; }
    setSubmitting(true);
    setEditError('');
    try {
      if (trimmed !== senderName.name) {
        await senderNamesApi.update(id, trimmed);
      }
      await senderNamesApi.resubmit(id);
      toast.success('Имя отправлено на повторное рассмотрение');
      load();
    } catch (e) {
      setEditError(e instanceof ApiError ? e.message : 'Ошибка');
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
      </div>
    );
  }

  if (error || !senderName) {
    return (
      <div>
        <Link to="/sender-names" className="text-primary text-sm hover:underline">← Имена отправителей</Link>
        <div className="mt-4 p-3 bg-red-50 text-red-700 rounded-md text-sm">{error || 'Не найдено'}</div>
      </div>
    );
  }

  const s = STATUS_BADGE[senderName.status] ?? { variant: 'default' as const, label: senderName.status };
  const registeredIds = new Set(registrations.map((r) => r.operator_id));

  return (
    <div>
      <Link to="/sender-names" className="text-primary text-sm hover:underline mb-4 inline-block">← Имена отправителей</Link>

      <div className="flex items-center gap-3 mb-6">
        <h1 className="text-2xl font-bold font-mono">{senderName.name}</h1>
        <Badge variant={s.variant}>{s.label}</Badge>
      </div>

      {/* Metadata */}
      <div className="grid grid-cols-2 gap-4 text-sm mb-6">
        <div>
          <span className="text-gray-400">Создано</span>
          <div className="text-gray-900">{formatDate(senderName.created_at)}</div>
        </div>
        {senderName.reviewed_at && (
          <div>
            <span className="text-gray-400">{senderName.status === 'approved' ? 'Одобрено' : 'Отклонено'}</span>
            <div className="text-gray-900">{formatDate(senderName.reviewed_at)}</div>
          </div>
        )}
      </div>

      {/* Approved: operator registrations */}
      {senderName.status === 'approved' && (
        <div className="border-t pt-4 mb-6">
          <h3 className="text-sm font-medium text-gray-700 mb-3">Регистрация у операторов</h3>
          <div className="flex gap-2 flex-wrap mb-3">
            {operators.map((op) => {
              const registered = registeredIds.has(op.id);
              return (
                <span
                  key={op.id}
                  className={`px-3 py-1 rounded-full text-xs font-medium ${
                    registered ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-500'
                  }`}
                >
                  {registered ? '✓ ' : ''}{op.name}
                </span>
              );
            })}
          </div>
          <Button onClick={() => navigate(`/sender-names/${id}/operators`)}>
            Зарегистрировать у операторов →
          </Button>
        </div>
      )}

      {/* Rejected: reason + resubmit form */}
      {senderName.status === 'rejected' && (
        <div className="border-t pt-4 mb-6">
          {senderName.rejection_reason && (
            <div className="p-3 bg-red-50 border border-red-200 rounded-md mb-4">
              <p className="text-sm font-medium text-red-800">Причина отклонения</p>
              <p className="text-sm text-red-700 mt-1">{senderName.rejection_reason}</p>
            </div>
          )}
          <h3 className="text-sm font-medium text-gray-700 mb-2">Исправить и отправить повторно</h3>
          <form onSubmit={handleResubmit} className="flex gap-2 items-start">
            <div>
              <Input
                value={editName}
                onChange={(e) => { setEditName(e.target.value); setEditError(''); }}
                maxLength={15}
                className="font-mono w-48"
              />
              <p className="text-xs text-gray-400 mt-1">1–11 латинских букв/цифр или 1–15 цифр</p>
              {editError && <p className="text-sm text-red-600 mt-1">{editError}</p>}
            </div>
            <Button type="submit" disabled={submitting}>Отправить</Button>
          </form>
        </div>
      )}

      {/* Pending info */}
      {senderName.status === 'pending' && (
        <div className="border-t pt-4 mb-6">
          <p className="text-sm text-gray-500">Имя находится на модерации. Мы уведомим вас о результате.</p>
        </div>
      )}

      {/* History */}
      <div className="border-t pt-4">
        {!showHistory ? (
          <button
            onClick={loadHistory}
            disabled={historyLoading}
            className="text-primary text-sm hover:underline"
          >
            ▸ Показать историю статусов
          </button>
        ) : (
          <div>
            <p className="text-sm font-medium text-gray-700 mb-2">История статусов</p>
            <div className="space-y-2">
              {history.map((e) => (
                <div key={e.id} className="flex items-start gap-2 text-sm">
                  <span className="text-gray-400 shrink-0">{formatDate(e.created_at)}</span>
                  <span>
                    {e.old_status ? `${e.old_status} → ` : ''}<strong>{e.new_status}</strong>
                    {e.comment && <span className="text-gray-500 ml-1">({e.comment})</span>}
                    <span className="text-xs text-gray-400 ml-1">({e.actor_type})</span>
                  </span>
                </div>
              ))}
              {history.length === 0 && <p className="text-sm text-gray-400">История пуста</p>}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /c/projects/sms/portal-frontend && npx tsc --noEmit --pretty 2>&1 | head -20`
Expected: no new errors (aside from possibly missing `useToast` export — check step 3)

- [ ] **Step 3: Check useToast export exists**

Run: `grep -n 'export.*useToast' portal-frontend/src/components/ui/Toast.tsx`

If not exported, add after the `ToastProvider` component:

```typescript
export function useToast() {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error('useToast must be used within ToastProvider');
  return ctx;
}
```

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/pages/sender-names/SenderNameDetailPage.tsx
git commit -m "feat(portal): add SenderNameDetailPage component"
```

---

## Task 7: Frontend — SenderNameOperatorsPage

**Files:**
- Create: `portal-frontend/src/pages/sender-names/SenderNameOperatorsPage.tsx`

- [ ] **Step 1: Create SenderNameOperatorsPage component**

```tsx
import { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import {
  senderNamesApi,
  senderNameRegistrationsApi,
  operatorsApi,
  ApiError,
  type SenderNameInfo,
  type OperatorInfo,
  type OperatorRegistration,
} from '../../api/client';
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';

export function SenderNameOperatorsPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const toast = useToast();

  const [senderName, setSenderName] = useState<SenderNameInfo | null>(null);
  const [operators, setOperators] = useState<OperatorInfo[]>([]);
  const [registrations, setRegistrations] = useState<OperatorRegistration[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Selection state: { operatorId: { selected, type } }
  const [selection, setSelection] = useState<Record<string, { selected: boolean; type: string }>>({});
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError('');
    try {
      const [sn, opsRes, regsRes] = await Promise.all([
        senderNamesApi.get(id),
        operatorsApi.list(),
        senderNameRegistrationsApi.list(id),
      ]);
      setSenderName(sn);
      setOperators(opsRes.operators ?? []);
      setRegistrations(regsRes.registrations ?? []);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { load(); }, [load]);

  // Initialize selection defaults once data loads
  useEffect(() => {
    const registeredIds = new Set(registrations.map((r) => r.operator_id));
    const initial: Record<string, { selected: boolean; type: string }> = {};
    for (const op of operators) {
      if (registeredIds.has(op.id)) continue; // skip registered
      if (op.registration_types.length === 0) continue; // skip unavailable
      const defaultType = op.registration_types.includes('free') ? 'free' : op.registration_types[0];
      initial[op.id] = { selected: false, type: defaultType };
    }
    setSelection(initial);
  }, [operators, registrations]);

  const toggleSelect = (opId: string) => {
    setSelection((prev) => ({
      ...prev,
      [opId]: { ...prev[opId], selected: !prev[opId]?.selected },
    }));
  };

  const toggleAll = () => {
    const allSelected = Object.values(selection).every((s) => s.selected);
    setSelection((prev) => {
      const next = { ...prev };
      for (const key of Object.keys(next)) {
        next[key] = { ...next[key], selected: !allSelected };
      }
      return next;
    });
  };

  const setType = (opId: string, type: string) => {
    setSelection((prev) => ({
      ...prev,
      [opId]: { ...prev[opId], type },
    }));
  };

  const selectedCount = Object.values(selection).filter((s) => s.selected).length;

  const handleSubmit = async () => {
    if (!id) return;
    const regs = Object.entries(selection)
      .filter(([, s]) => s.selected)
      .map(([operatorId, s]) => ({ operator_id: operatorId, type: s.type }));
    if (regs.length === 0) return;

    setSubmitting(true);
    try {
      await senderNameRegistrationsApi.bulkCreate(id, regs);
      toast.success('Регистрация отправлена');
      navigate(`/sender-names/${id}`);
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка регистрации');
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
      </div>
    );
  }

  if (error || !senderName) {
    return (
      <div>
        <Link to="/sender-names" className="text-primary text-sm hover:underline">← Имена отправителей</Link>
        <div className="mt-4 p-3 bg-red-50 text-red-700 rounded-md text-sm">{error || 'Не найдено'}</div>
      </div>
    );
  }

  const registeredIds = new Set(registrations.map((r) => r.operator_id));
  const registeredNames = registrations.map((r) => r.operator_name);

  return (
    <div>
      {/* Breadcrumb */}
      <div className="text-sm text-gray-500 mb-4">
        <Link to="/sender-names" className="text-primary hover:underline">Имена отправителей</Link>
        {' › '}
        <Link to={`/sender-names/${id}`} className="text-primary hover:underline">{senderName.name}</Link>
        {' › '}
        <span>Регистрация у операторов</span>
      </div>

      <h1 className="text-xl font-semibold mb-1">
        Регистрация имени «{senderName.name}» у операторов
      </h1>
      <p className="text-sm text-gray-500 mb-6">
        Выберите операторов, укажите тип регистрации и нажмите «Зарегистрировать»
      </p>

      {/* Table */}
      <div className="border rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b-2 border-gray-200 bg-gray-50">
              <th className="w-10 px-3 py-3 text-left">
                <input
                  type="checkbox"
                  checked={Object.keys(selection).length > 0 && Object.values(selection).every((s) => s.selected)}
                  onChange={toggleAll}
                  className="w-4 h-4"
                />
              </th>
              <th className="px-3 py-3 text-left text-gray-500 font-medium">Оператор</th>
              <th className="px-3 py-3 text-left text-gray-500 font-medium">Тип регистрации</th>
              <th className="px-3 py-3 text-left text-gray-500 font-medium">Стоимость</th>
              <th className="px-3 py-3 text-left text-gray-500 font-medium">Статус</th>
            </tr>
          </thead>
          <tbody>
            {operators.map((op) => {
              const isRegistered = registeredIds.has(op.id);
              const hasTypes = op.registration_types.length > 0;
              const isDisabled = isRegistered || !hasTypes;
              const sel = selection[op.id];
              const isSelected = sel?.selected ?? false;
              const currentType = sel?.type ?? '';

              return (
                <tr
                  key={op.id}
                  className={`border-b border-gray-100 ${
                    isRegistered ? 'bg-gray-50' : isSelected ? 'bg-blue-50' : ''
                  }`}
                >
                  <td className="px-3 py-3">
                    <input
                      type="checkbox"
                      checked={isSelected}
                      onChange={() => toggleSelect(op.id)}
                      disabled={isDisabled}
                      className="w-4 h-4 disabled:opacity-40"
                    />
                  </td>
                  <td className="px-3 py-3">
                    <div className={`font-medium ${isDisabled ? 'text-gray-400' : 'text-gray-900'}`}>{op.name}</div>
                    <div className="text-xs text-gray-400">{op.slug.toLowerCase()}</div>
                  </td>
                  <td className="px-3 py-3">
                    {isDisabled ? (
                      <span className="text-gray-400">—</span>
                    ) : (
                      <select
                        value={currentType}
                        onChange={(e) => setType(op.id, e.target.value)}
                        className="border border-gray-300 rounded-md px-2 py-1 text-sm"
                      >
                        {op.registration_types.includes('free') && <option value="free">Бесплатная</option>}
                        {op.registration_types.includes('paid') && <option value="paid">Платная</option>}
                      </select>
                    )}
                  </td>
                  <td className="px-3 py-3">
                    {!isDisabled && currentType === 'paid' && op.monthly_tariff_amount ? (
                      <span className="font-semibold">
                        {parseFloat(op.monthly_tariff_amount).toLocaleString('ru-RU', { style: 'currency', currency: 'RUB' })}/мес
                      </span>
                    ) : (
                      <span className="text-gray-400">—</span>
                    )}
                  </td>
                  <td className="px-3 py-3">
                    {isRegistered ? (
                      <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
                        ✓ Зарегистрировано
                      </span>
                    ) : (
                      <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800">
                        Не зарегистрировано
                      </span>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {/* Footer */}
      <div className="flex items-center gap-3 mt-4">
        <Button onClick={handleSubmit} disabled={submitting || selectedCount === 0}>
          {submitting ? 'Регистрация...' : `Зарегистрировать у выбранных (${selectedCount})`}
        </Button>
        <Button variant="ghost" onClick={() => navigate(`/sender-names/${id}`)}>Отмена</Button>
        {registeredNames.length > 0 && (
          <span className="text-sm text-gray-500 ml-auto">
            Уже зарегистрировано: {registeredNames.join(', ')}
          </span>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /c/projects/sms/portal-frontend && npx tsc --noEmit --pretty 2>&1 | head -20`
Expected: no new errors

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/sender-names/SenderNameOperatorsPage.tsx
git commit -m "feat(portal): add SenderNameOperatorsPage component"
```

---

## Task 8: Frontend — Strip SenderNamesPage modal and add navigation

**Files:**
- Modify: `portal-frontend/src/pages/sender-names/SenderNamesPage.tsx`

- [ ] **Step 1: Add useNavigate import**

Replace the import line 1:

```typescript
import { useState, useEffect, useCallback, type FormEvent } from 'react';
```

with:

```typescript
import { useState, useEffect, useCallback, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
```

- [ ] **Step 2: Remove all detail/edit/history/register state variables**

Remove lines 47–65 (the entire block of state for detail panel, edit, history, register):

```typescript
  // Detail / edit panel
  const [selected, setSelected] = useState<SenderNameInfo | null>(null);
  const [editName, setEditName] = useState('');
  const [editError, setEditError] = useState('');
  const [editing, setEditing] = useState(false);
  const [showEdit, setShowEdit] = useState(false);

  // History
  const [history, setHistory] = useState<SenderNameHistoryEntry[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [showHistory, setShowHistory] = useState(false);

  // Register with operator (paid/free)
  const [showRegister, setShowRegister] = useState(false);
  const [regOperatorId, setRegOperatorId] = useState('');
  const [regType, setRegType] = useState<'free' | 'paid'>('free');
  const [regTariff, setRegTariff] = useState<string | null>(null);
  const [regTariffLoading, setRegTariffLoading] = useState(false);
  const [regError, setRegError] = useState('');
  const [registering, setRegistering] = useState(false);
```

- [ ] **Step 3: Remove all handler functions that depended on removed state**

Remove the following functions entirely:
- `openDetail` (lines 101–108)
- `handleUpdate` (lines 110–127)
- `handleResubmit` (lines 129–142)
- `fetchTariff` (lines 144–155)
- `handleRegister` (lines 157–172)
- `loadHistory` (lines 174–185)
- `nextBillingDate` (lines 187–190)

- [ ] **Step 4: Add navigate and update columns/handlers**

Add inside the component, before `const load`:

```typescript
  const navigate = useNavigate();
```

Replace the `columns` definition to remove `next_billing` column and use navigate for `actions`:

```typescript
  const columns: Column<SenderNameInfo>[] = [
    { key: 'name', header: 'Имя отправителя', render: (sn) => <span className="font-mono font-medium">{sn.name}</span> },
    {
      key: 'status', header: 'Статус', render: (sn) => {
        const s = STATUS_BADGE[sn.status] ?? { variant: 'default' as const, label: sn.status };
        return <Badge variant={s.variant}>{s.label}</Badge>;
      },
    },
    { key: 'created_at', header: 'Создано', render: (sn) => formatDate(sn.created_at) },
    {
      key: 'actions', header: '', render: (sn) => (
        <Button variant="ghost" size="sm" onClick={(e) => { e.stopPropagation(); navigate(`/sender-names/${sn.id}`); }}>
          Подробнее
        </Button>
      ),
    },
  ];
```

Replace `onRowClick={openDetail}` on the DataTable with:

```typescript
        onRowClick={(sn) => navigate(`/sender-names/${sn.id}`)}
```

- [ ] **Step 5: Remove the entire detail modal JSX block**

Remove everything from `{/* Detail modal */}` (line 256) to the closing `</Modal>` with `)}` (line 383), leaving only the create modal and the DataTable.

Also remove the unused imports: `senderTariffApi`, `SenderNameHistoryEntry` from the import line 2.

- [ ] **Step 6: Verify TypeScript compiles**

Run: `cd /c/projects/sms/portal-frontend && npx tsc --noEmit --pretty 2>&1 | head -20`
Expected: no new errors

- [ ] **Step 7: Commit**

```bash
git add portal-frontend/src/pages/sender-names/SenderNamesPage.tsx
git commit -m "refactor(portal): strip detail modal from SenderNamesPage, use page navigation"
```

---

## Task 9: Frontend — Add routes to App.tsx

**Files:**
- Modify: `portal-frontend/src/App.tsx`

- [ ] **Step 1: Add lazy imports**

After the existing `SenderNameBillingHistory` import (line 32), add:

```typescript
import { SenderNameDetailPage } from './pages/sender-names/SenderNameDetailPage';
import { SenderNameOperatorsPage } from './pages/sender-names/SenderNameOperatorsPage';
```

- [ ] **Step 2: Add route entries**

After the existing `<Route path="/sender-names" .../>` line (line 130), add:

```tsx
        <Route path="/sender-names/:id" element={<SenderNameDetailPage />} />
        <Route path="/sender-names/:id/operators" element={<SenderNameOperatorsPage />} />
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /c/projects/sms/portal-frontend && npx tsc --noEmit --pretty 2>&1 | head -20`
Expected: no new errors

- [ ] **Step 4: Verify dev server starts**

Run: `cd /c/projects/sms/portal-frontend && npx vite build 2>&1 | tail -5`
Expected: build succeeds

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/App.tsx
git commit -m "feat(portal): add sender name detail and operators page routes"
```

---

## Self-Review Checklist

- **Spec coverage:** All spec sections covered — list page (Task 8), detail page (Task 6), operators page (Task 7), API endpoints (Tasks 1–4), API client (Task 5), routes (Task 9).
- **Placeholder scan:** No TBD/TODO. All code blocks are complete.
- **Type consistency:** `OperatorInfo`, `OperatorRegistration`, `SenderNameInfo` used consistently across API client and components. `operatorsApi`, `senderNameRegistrationsApi` method names match across Task 5, 6, 7.
- **Missing from spec:** `registration_types` column migration not needed — `supports_paid_sender` / `supports_free_sender` booleans in `operators` table already serve this purpose (migration 000012). Plan uses direct SQL query to derive the array.
