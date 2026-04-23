// Network tariff editor page (Tasks 18–21).
// - Top bar: breadcrumbs, channel tab (SMS only), filter dropdowns, strategy indicator.
// - Matrix: TariffMatrix component in editable mode.
// - Period actions: «+ Добавить период» (NewPeriodDialog).
//   «Удалить период» — omitted until backend endpoint exists. TODO(task-26-delete-period).
// - Unsaved-changes guard: anchor click-capture for in-app nav, beforeunload for tab close.
//   NB: useBlocker requires a data router (createBrowserRouter); app still uses BrowserRouter,
//   so we intercept clicks on <a href> elements manually.
//
// Note re top-bar «Режим правки» toggle and Save/Cancel: TariffMatrix owns its own
// edit-mode controls (Save/Cancel rendered inside the matrix). Duplicating them in
// the top bar would create two sources of truth for edit state, so we surface the
// current unsaved-changes state as a passive badge in the top bar instead of a
// redundant toggle. This is the pragmatic read of the spec; matrix-internal
// controls remain the single interaction surface.

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Link,
  useNavigate,
  useParams,
  useSearchParams,
} from 'react-router-dom';
import {
  networkTariffsApi,
  type TariffEditorData,
} from '../../api/client';
import { TariffMatrix } from '../../components/tariffs/TariffMatrix';
import type {
  TariffBulkSaveBody,
  TariffMatrixSaveError,
  TariffMatrixSaveResult,
  TariffMatrixScope,
} from '../../components/tariffs/TariffMatrix.types';
import { Button } from '../../components/ui/Button';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { useToast } from '../../components/ui/Toast';
import { NewPeriodDialog } from './components/NewPeriodDialog';

type Mode = 'template' | 'override';

interface EditorParams {
  mode: Mode;
  channel: string;
  country: string;
  sender_category: string;
  traffic_type: string;
  period_id?: string;
}

function readParams(search: URLSearchParams): EditorParams {
  const modeRaw = search.get('mode');
  const mode: Mode = modeRaw === 'override' ? 'override' : 'template';
  return {
    mode,
    channel: search.get('channel') || 'sms',
    country: search.get('country') || 'RU',
    sender_category: search.get('sender_category') || 'paid_registered',
    traffic_type: search.get('traffic_type') || 'any',
    period_id: search.get('period_id') || undefined,
  };
}

const SENDER_CATEGORIES: { value: string; label: string }[] = [
  { value: 'paid_registered', label: 'Платный (зарег.)' },
  { value: 'paid_unregistered', label: 'Платный (незарег.)' },
  { value: 'free', label: 'Бесплатный' },
];

const TRAFFIC_TYPES: { value: string; label: string }[] = [
  { value: 'any', label: 'Любой' },
  { value: 'service', label: 'Сервисный' },
  { value: 'advertising', label: 'Рекламный' },
];

const COUNTRIES: { value: string; label: string }[] = [
  { value: 'RU', label: 'Россия' },
  { value: 'KZ', label: 'Казахстан' },
  { value: 'BY', label: 'Беларусь' },
];

export function NetworkTariffEditorPage() {
  const { id = '' } = useParams<{ id: string }>();
  const [search, setSearch] = useSearchParams();
  const toast = useToast();
  const navigate = useNavigate();

  const params = useMemo(() => readParams(search), [search]);

  const [data, setData] = useState<TariffEditorData | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false);
  const [newPeriodOpen, setNewPeriodOpen] = useState(false);
  const [pendingHref, setPendingHref] = useState<string | null>(null);
  const [creatingPlan, setCreatingPlan] = useState(false);
  const [createPlanError, setCreatePlanError] = useState<string | null>(null);

  // fetch editor data whenever id/params change
  useEffect(() => {
    if (!id) return;
    let cancelled = false;
    setLoading(true);
    setLoadError(null);
    networkTariffsApi
      .getEditor(id, params)
      .then((d) => {
        if (!cancelled) setData(d);
      })
      .catch((e: unknown) => {
        if (!cancelled) {
          setLoadError(e instanceof Error ? e.message : String(e));
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [id, params]);

  // Task 21: block in-app navigation when there are unsaved changes.
  // BrowserRouter doesn't support useBlocker, so we capture clicks on internal
  // anchors at document level and prompt before allowing navigation.
  useEffect(() => {
    if (!hasUnsavedChanges) return;
    const handler = (e: MouseEvent) => {
      if (e.defaultPrevented) return;
      if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
      const anchor = (e.target as HTMLElement | null)?.closest('a');
      if (!anchor) return;
      const href = anchor.getAttribute('href');
      if (!href || href.startsWith('#')) return;
      if (anchor.target && anchor.target !== '_self') return;
      // Only intercept same-origin internal nav.
      const url = new URL(anchor.href, window.location.href);
      if (url.origin !== window.location.origin) return;
      if (url.pathname === window.location.pathname && url.search === window.location.search) return;
      e.preventDefault();
      setPendingHref(url.pathname + url.search + url.hash);
    };
    document.addEventListener('click', handler, true);
    return () => document.removeEventListener('click', handler, true);
  }, [hasUnsavedChanges]);

  // Task 21: native tab-close guard.
  useEffect(() => {
    if (!hasUnsavedChanges) return;
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = '';
    };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, [hasUnsavedChanges]);

  const updateParam = useCallback(
    (key: keyof EditorParams, value: string) => {
      const next = new URLSearchParams(search);
      next.set(key, value);
      // clear period_id when other filters change — backend picks active period
      if (key !== 'period_id') {
        next.delete('period_id');
      }
      setSearch(next, { replace: false });
    },
    [search, setSearch],
  );

  const matrixScope = useMemo<TariffMatrixScope | null>(() => {
    if (!data) return null;
    if (data.scope.kind === 'template') {
      return { kind: 'template', templateId: data.scope.template_id ?? id };
    }
    return { kind: 'override', subAccountId: data.scope.sub_account_id ?? id };
  }, [data, id]);

  const refetch = useCallback(() => {
    if (!id) return;
    networkTariffsApi
      .getEditor(id, params)
      .then((d) => setData(d))
      .catch((e: unknown) => {
        toast.error(e instanceof Error ? e.message : 'Ошибка загрузки');
      });
  }, [id, params, toast]);

  const handleSave = useCallback(
    async (batch: TariffBulkSaveBody): Promise<TariffMatrixSaveResult> => {
      if (!data || !data.plan) return { ok: false };
      try {
        const res = await networkTariffsApi.bulkPatchPlan(data.plan.id, batch);
        if (res.ok) {
          toast.success('Изменения сохранены');
          refetch();
          return { ok: true };
        }
        // Pass errors through; matrix will highlight cells and stay in edit mode.
        // Best-effort shape match — backend returns unknown[], matrix expects
        // {operator_id, tier_id, reason}[].
        const errs = (res.errors ?? []) as TariffMatrixSaveError[];
        return { ok: false, errors: errs };
      } catch (e) {
        toast.error(e instanceof Error ? e.message : 'Ошибка сохранения');
        return { ok: false };
      }
    },
    [data, refetch, toast],
  );

  // --- render ---

  if (loading && !data) {
    return <div className="p-6 text-sm text-slate-500">Загрузка…</div>;
  }

  if (loadError && !data) {
    return (
      <div className="p-6 text-sm text-red-600">
        Ошибка загрузки: {loadError}
      </div>
    );
  }

  if (!data || !matrixScope) {
    return null;
  }

  // Backend returns plan=null when no plan matches the selected filters
  // (template exists, but no row in reseller_tariff_plans for this
  // country/sender_category/traffic_type combination). Render an empty
  // state with filters still switchable + a CTA to create the missing plan.
  if (!data.plan) {
    const handleCreatePlan = async () => {
      setCreatingPlan(true);
      setCreatePlanError(null);
      try {
        const body: Parameters<typeof networkTariffsApi.createPlan>[0] = {
          country: params.country,
          sender_category: params.sender_category,
          traffic_type: params.traffic_type,
          strategy: 'fixed',
        };
        if (matrixScope.kind === 'template') {
          body.template_id = matrixScope.templateId;
        } else if (matrixScope.kind === 'override') {
          body.sub_account_id = matrixScope.subAccountId;
        } else {
          return;
        }
        await networkTariffsApi.createPlan(body);
        toast.success('Тарифный план создан');
        refetch();
      } catch (e) {
        const msg = e instanceof Error ? e.message : String(e);
        setCreatePlanError(msg);
        toast.error(msg);
      } finally {
        setCreatingPlan(false);
      }
    };
    return (
      <div className="flex flex-col">
        <div className="sticky top-0 z-10 bg-white border-b border-slate-200">
          <div className="px-6 py-3 space-y-3">
            <div className="flex items-center gap-2 text-sm">
              <Link to="/network/tariffs" className="text-sky-600 hover:underline">
                Тарифы
              </Link>
              <span className="text-slate-400">›</span>
              <span className="font-medium text-slate-800">
                {data.scope.kind === 'template' ? data.scope.template_name : data.scope.sub_account_name}
              </span>
            </div>
            <div className="flex flex-wrap items-center gap-3">
              <FilterSelect
                label="Страна"
                value={params.country}
                options={COUNTRIES}
                onChange={(v) => updateParam('country', v)}
              />
              <FilterSelect
                label="Тип имени"
                value={params.sender_category}
                options={SENDER_CATEGORIES}
                onChange={(v) => updateParam('sender_category', v)}
              />
              <FilterSelect
                label="Тип трафика"
                value={params.traffic_type}
                options={TRAFFIC_TYPES}
                onChange={(v) => updateParam('traffic_type', v)}
              />
            </div>
          </div>
        </div>
        <div className="px-6 py-10 text-center text-sm text-slate-600">
          <p className="mb-1 font-medium text-slate-800">
            Для выбранной комбинации фильтров тарифный план ещё не создан.
          </p>
          <p className="mb-4 text-slate-500">
            Создайте план для текущего сочетания «страна × тип имени × тип
            трафика» — далее можно будет задать ступени цен и периоды.
          </p>
          <Button onClick={handleCreatePlan} disabled={creatingPlan}>
            {creatingPlan ? 'Создаём…' : '+ Создать тарифный план'}
          </Button>
          {createPlanError && (
            <p role="alert" className="mt-3 text-xs text-red-600">
              {createPlanError}
            </p>
          )}
        </div>
      </div>
    );
  }

  const plan = data.plan;

  const activePeriod =
    data.periods.find((p) => p.id === data.active_period_id) ?? data.periods[0];

  return (
    <div className="flex flex-col">
      {/* sticky top bar */}
      <div className="sticky top-0 z-10 bg-white border-b border-slate-200">
        <div className="px-6 py-3 space-y-3">
          {/* breadcrumbs */}
          <div className="flex items-center gap-2 text-sm">
            <Link to="/network/tariffs" className="text-sky-600 hover:underline">
              Тарифы
            </Link>
            <span className="text-slate-400">›</span>
            <span className="font-medium text-slate-800">
              {data.scope.kind === 'template' ? data.scope.template_name : data.scope.sub_account_name}
            </span>
            {params.mode === 'override' && data.template && (
              <span className="ml-3 text-xs text-slate-500">
                Шаблон: {data.template.name}
              </span>
            )}
            {hasUnsavedChanges && (
              <span className="ml-3 px-2 py-0.5 rounded bg-amber-100 text-amber-800 text-xs">
                Несохранённые изменения
              </span>
            )}
          </div>

          {/* channel tabs (SMS only active) */}
          <div className="flex items-center gap-1 border-b border-slate-100 -mx-6 px-6">
            <button
              type="button"
              className="px-3 py-1.5 text-sm font-medium border-b-2 border-sky-500 text-sky-700"
              aria-current="page"
            >
              SMS
            </button>
          </div>

          {/* filters + strategy */}
          <div className="flex flex-wrap items-center gap-3">
            <FilterSelect
              label="Страна"
              value={params.country}
              options={COUNTRIES}
              onChange={(v) => updateParam('country', v)}
            />
            <FilterSelect
              label="Тип имени"
              value={params.sender_category}
              options={SENDER_CATEGORIES}
              onChange={(v) => updateParam('sender_category', v)}
            />
            <FilterSelect
              label="Тип трафика"
              value={params.traffic_type}
              options={TRAFFIC_TYPES}
              onChange={(v) => updateParam('traffic_type', v)}
            />
            <PeriodSelect
              value={activePeriod?.id ?? ''}
              periods={data.periods}
              onChange={(v) => updateParam('period_id', v)}
              onNewPeriod={() => setNewPeriodOpen(true)}
            />
            <div className="ml-auto text-xs text-slate-500">
              Стратегия: <span className="font-medium text-slate-700">{plan.strategy}</span>
              {/* TODO(out-of-scope): «Редактировать стратегию плана». */}
            </div>
          </div>
        </div>
      </div>

      {/* matrix body */}
      <div className="px-6 py-4">
        {data.active_period_id ? (
          <TariffMatrix
            data={{ ...data, plan, active_period_id: data.active_period_id }}
            scope={matrixScope}
            editable={true}
            showInheritance={params.mode === 'override'}
            currency={plan.currency}
            onSaveBatch={handleSave}
            onUnsavedChange={setHasUnsavedChanges}
          />
        ) : (
          <div className="py-8 text-center text-sm text-slate-600">
            У плана пока нет периодов тарификации. Создайте первый, чтобы
            начать редактирование матрицы.
          </div>
        )}

        {/* period-level actions */}
        <div className="mt-4 flex items-center gap-2">
          <Button variant="secondary" size="sm" onClick={() => setNewPeriodOpen(true)}>
            + Добавить период
          </Button>
          {/* TODO(task-26-delete-period): wire delete-period flow once backend endpoint
              `DELETE /network/tariff-periods/:id` is available. Intentionally omitted here. */}
        </div>
      </div>

      {/* new period dialog */}
      <NewPeriodDialog
        open={newPeriodOpen}
        onOpenChange={setNewPeriodOpen}
        planId={plan.id}
        existingPeriods={data.periods.map((p) => ({ id: p.id, from: p.from, to: p.to }))}
        onSuccess={(newPeriodId) => {
          updateParam('period_id', newPeriodId);
          toast.success('Период создан');
        }}
      />

      {/* unsaved-changes confirm dialog for in-app nav */}
      <ConfirmDialog
        open={pendingHref !== null}
        title="Несохранённые изменения"
        description="Вы уверены, что хотите уйти со страницы? Все несохранённые изменения будут потеряны."
        confirmLabel="Уйти"
        variant="danger"
        onCancel={() => setPendingHref(null)}
        onConfirm={() => {
          const href = pendingHref;
          setPendingHref(null);
          setHasUnsavedChanges(false);
          if (href) navigate(href);
        }}
      />

    </div>
  );
}

// --- small helpers ---

interface FilterSelectProps {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
}

function FilterSelect({ label, value, options, onChange }: FilterSelectProps) {
  return (
    <label className="flex items-center gap-2 text-sm">
      <span className="text-slate-600">{label}:</span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="rounded border border-slate-300 px-2 py-1 text-sm focus:border-sky-500 focus:outline-none"
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </label>
  );
}

interface PeriodSelectProps {
  value: string;
  periods: { id: string; from: string; to: string | null; active: boolean }[];
  onChange: (value: string) => void;
  onNewPeriod: () => void;
}

function PeriodSelect({ value, periods, onChange, onNewPeriod }: PeriodSelectProps) {
  const NEW_SENTINEL = '__new__';
  return (
    <label className="flex items-center gap-2 text-sm">
      <span className="text-slate-600">Период:</span>
      <select
        value={value}
        onChange={(e) => {
          const v = e.target.value;
          if (v === NEW_SENTINEL) {
            onNewPeriod();
            return;
          }
          onChange(v);
        }}
        className="rounded border border-slate-300 px-2 py-1 text-sm focus:border-sky-500 focus:outline-none"
      >
        {periods.map((p) => (
          <option key={p.id} value={p.id}>
            {p.from} — {p.to ?? 'бессрочно'}
            {p.active ? ' (активный)' : ''}
          </option>
        ))}
        <option value={NEW_SENTINEL}>+ Новый период…</option>
      </select>
    </label>
  );
}
