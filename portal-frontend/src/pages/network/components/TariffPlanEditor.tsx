import { useState, useEffect, useCallback } from 'react';
import { Button } from '../../../components/ui/Button';
import { Select } from '../../../components/ui/Select';
import { Modal } from '../../../components/ui/Modal';
import { Badge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import {
  resellerTariffApi,
  referencesApi,
  type ResellerTariffPlan,
  type ResellerTariffPeriod,
  type ResellerTariffTier,
  type OperatorRef,
  type CountryRef,
  ApiError,
} from '../../../api/client';

// --- Constants ---

const CATEGORY_OPTIONS = [
  { value: 'standard', label: 'Стандартная' },
  { value: 'shared', label: 'Общая' },
  { value: 'paid_registered', label: 'Платная регистрация' },
  { value: 'free_registered', label: 'Бесплатная регистрация' },
];

const TRAFFIC_TYPE_OPTIONS = [
  { value: 'any', label: 'Любой' },
  { value: 'authorization', label: 'Авторизация' },
  { value: 'transactional', label: 'Транзакционный' },
  { value: 'service', label: 'Сервисный' },
];

const STRATEGY_OPTIONS = [
  { value: 'fixed', label: 'Фиксированная' },
  { value: 'threshold', label: 'Пороговая' },
  { value: 'threshold_recalc', label: 'Пороговая с пересчётом' },
  { value: 'prepaid_threshold', label: 'Предоплатная пороговая' },
];

const STRATEGY_LABELS: Record<string, string> = {
  fixed: 'Фикс.',
  threshold: 'Порог.',
  threshold_recalc: 'Порог. пересч.',
  prepaid_threshold: 'Предопл. порог.',
};

const CATEGORY_LABELS: Record<string, string> = {
  standard: 'Стандартная',
  shared: 'Общая',
  paid_registered: 'Платная рег.',
  free_registered: 'Бесплатная рег.',
};

const TRAFFIC_LABELS: Record<string, string> = {
  any: 'Любой',
  authorization: 'Авторизация',
  transactional: 'Транзакционный',
  service: 'Сервисный',
};

// --- Sub-components ---

interface TierRow {
  from_count: number;
  price_per_segment: string;
}

function PeriodTiersSection({
  periodId,
  strategy,
}: {
  periodId: string;
  strategy: string;
}) {
  const toast = useToast();
  const [tiers, setTiers] = useState<ResellerTariffTier[]>([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState(false);
  const [editRows, setEditRows] = useState<TierRow[]>([]);
  const [saving, setSaving] = useState(false);

  const loadTiers = useCallback(() => {
    setLoading(true);
    resellerTariffApi
      .listTiers(periodId)
      .then((r) => setTiers(r.tiers || []))
      .catch(() => toast.error('Ошибка загрузки цен'))
      .finally(() => setLoading(false));
  }, [periodId]);

  useEffect(() => {
    loadTiers();
  }, [loadTiers]);

  function startEdit() {
    if (tiers.length > 0) {
      setEditRows(
        tiers.map((t) => ({ from_count: t.from_count, price_per_segment: t.price_per_segment })),
      );
    } else {
      setEditRows([{ from_count: 0, price_per_segment: '' }]);
    }
    setEditing(true);
  }

  function cancelEdit() {
    setEditing(false);
  }

  async function saveTiers() {
    const invalid = editRows.find((r) => {
      const n = parseFloat(r.price_per_segment);
      return isNaN(n) || n < 0;
    });
    if (invalid) {
      toast.error('Цена должна быть неотрицательным числом');
      return;
    }
    setSaving(true);
    try {
      await resellerTariffApi.upsertTiers(periodId, editRows);
      toast.success('Цены сохранены');
      setEditing(false);
      loadTiers();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка сохранения');
    } finally {
      setSaving(false);
    }
  }

  function updateRow(idx: number, field: keyof TierRow, value: string) {
    setEditRows((prev) => {
      const next = [...prev];
      if (field === 'from_count') {
        next[idx] = { ...next[idx], from_count: parseInt(value) || 0 };
      } else {
        next[idx] = { ...next[idx], price_per_segment: value };
      }
      return next;
    });
  }

  function addRow() {
    setEditRows((prev) => [...prev, { from_count: 0, price_per_segment: '' }]);
  }

  function removeRow(idx: number) {
    setEditRows((prev) => prev.filter((_, i) => i !== idx));
  }

  const isFixed = strategy === 'fixed';

  if (loading) {
    return <div className="h-8 bg-gray-100 rounded animate-pulse" />;
  }

  if (!editing) {
    return (
      <div>
        {tiers.length === 0 ? (
          <div className="text-sm text-gray-400 mb-2">Цены не заданы</div>
        ) : (
          <table className="text-sm mb-2 w-full">
            <thead>
              <tr className="text-xs text-gray-500">
                {!isFixed && <th className="text-left pr-4 pb-1">От (шт.)</th>}
                <th className="text-left pb-1">Цена за сегмент</th>
              </tr>
            </thead>
            <tbody>
              {tiers.map((t) => (
                <tr key={t.id}>
                  {!isFixed && <td className="pr-4 py-0.5">{t.from_count}</td>}
                  <td className="font-mono py-0.5">{parseFloat(t.price_per_segment).toFixed(2)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        <Button variant="secondary" size="sm" onClick={startEdit}>
          {tiers.length === 0 ? 'Задать цены' : 'Редактировать'}
        </Button>
      </div>
    );
  }

  return (
    <div>
      <table className="text-sm mb-2 w-full">
        <thead>
          <tr className="text-xs text-gray-500">
            {!isFixed && <th className="text-left pr-4 pb-1">От (шт.)</th>}
            <th className="text-left pb-1">Цена за сегмент</th>
            {!isFixed && <th className="pb-1 w-8" />}
          </tr>
        </thead>
        <tbody>
          {editRows.map((row, idx) => (
            <tr key={idx}>
              {!isFixed && (
                <td className="pr-2 py-0.5">
                  <input
                    type="number"
                    min={0}
                    value={row.from_count}
                    onChange={(e) => updateRow(idx, 'from_count', e.target.value)}
                    className="border border-gray-300 rounded px-2 py-1 w-24 text-sm focus:ring-2 focus:ring-primary/50 focus:border-primary"
                  />
                </td>
              )}
              <td className="pr-2 py-0.5">
                <input
                  type="text"
                  inputMode="decimal"
                  value={row.price_per_segment}
                  onChange={(e) => updateRow(idx, 'price_per_segment', e.target.value)}
                  placeholder="0.00"
                  className="border border-gray-300 rounded px-2 py-1 w-28 text-sm font-mono focus:ring-2 focus:ring-primary/50 focus:border-primary"
                />
              </td>
              {!isFixed && (
                <td className="py-0.5">
                  {editRows.length > 1 && (
                    <button
                      onClick={() => removeRow(idx)}
                      className="text-red-400 hover:text-red-600 text-xs"
                      title="Удалить"
                    >
                      ✕
                    </button>
                  )}
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
      {!isFixed && (
        <button onClick={addRow} className="text-xs text-primary hover:underline mb-2">
          + Добавить порог
        </button>
      )}
      <div className="flex gap-2 mt-2">
        <Button size="sm" onClick={saveTiers} disabled={saving}>
          {saving ? 'Сохранение...' : 'Сохранить'}
        </Button>
        <Button variant="secondary" size="sm" onClick={cancelEdit}>
          Отмена
        </Button>
      </div>
    </div>
  );
}

function PlanPeriodsSection({ planId, strategy }: { planId: string; strategy: string }) {
  const toast = useToast();
  const [periods, setPeriods] = useState<ResellerTariffPeriod[]>([]);
  const [loading, setLoading] = useState(true);
  const [expandedPeriod, setExpandedPeriod] = useState<string | null>(null);
  const [showAddPeriod, setShowAddPeriod] = useState(false);
  const [newStart, setNewStart] = useState('');
  const [addingPeriod, setAddingPeriod] = useState(false);

  const loadPeriods = useCallback(() => {
    setLoading(true);
    resellerTariffApi
      .listPeriods(planId)
      .then((r) => setPeriods(r.periods || []))
      .catch(() => toast.error('Ошибка загрузки периодов'))
      .finally(() => setLoading(false));
  }, [planId]);

  useEffect(() => {
    loadPeriods();
  }, [loadPeriods]);

  async function handleAddPeriod() {
    if (!newStart) {
      toast.error('Укажите дату начала');
      return;
    }
    setAddingPeriod(true);
    try {
      await resellerTariffApi.createPeriod(planId, {
        start_date: newStart,
      });
      toast.success('Период добавлен');
      setShowAddPeriod(false);
      setNewStart('');
      loadPeriods();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка создания периода');
    } finally {
      setAddingPeriod(false);
    }
  }

  async function handleDeletePeriod(periodId: string) {
    try {
      await resellerTariffApi.deletePeriod(periodId);
      toast.success('Период удалён');
      loadPeriods();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка удаления периода');
    }
  }

  if (loading) {
    return <div className="h-16 bg-gray-100 rounded animate-pulse" />;
  }

  return (
    <div className="mt-3">
      <div className="flex items-center justify-between mb-2">
        <div className="text-xs font-semibold text-gray-500 uppercase">Периоды</div>
        <button
          onClick={() => setShowAddPeriod(!showAddPeriod)}
          className="text-xs text-primary hover:underline"
        >
          + Добавить период
        </button>
      </div>

      {showAddPeriod && (
        <div className="flex items-end gap-2 mb-3 p-3 bg-gray-50 rounded border border-gray-200">
          <div className="flex flex-col gap-1">
            <label className="text-xs text-gray-500">Дата начала</label>
            <input
              type="date"
              value={newStart}
              onChange={(e) => setNewStart(e.target.value)}
              className="border border-gray-300 rounded px-2 py-1 text-sm"
            />
          </div>
          <Button size="sm" onClick={handleAddPeriod} disabled={addingPeriod}>
            {addingPeriod ? '...' : 'Добавить'}
          </Button>
          <Button variant="secondary" size="sm" onClick={() => setShowAddPeriod(false)}>
            Отмена
          </Button>
        </div>
      )}

      {periods.length === 0 ? (
        <div className="text-sm text-gray-400">Нет периодов</div>
      ) : (
        <div className="space-y-2">
          {periods.map((period) => {
            const isExpanded = expandedPeriod === period.id;
            return (
              <div
                key={period.id}
                className="border border-gray-200 rounded bg-white"
              >
                <div
                  className="flex items-center justify-between px-3 py-2 cursor-pointer hover:bg-gray-50"
                  onClick={() => setExpandedPeriod(isExpanded ? null : period.id)}
                >
                  <span className="text-sm">
                    {period.start_date} — {period.end_date ?? 'бессрочно'}
                  </span>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleDeletePeriod(period.id);
                      }}
                      className="text-xs text-red-400 hover:text-red-600"
                    >
                      Удалить
                    </button>
                    <span className="text-gray-400 text-xs">{isExpanded ? '▼' : '▶'}</span>
                  </div>
                </div>
                {isExpanded && (
                  <div className="px-3 pb-3 border-t border-gray-100">
                    <PeriodTiersSection periodId={period.id} strategy={strategy} />
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

// --- Main component ---

interface TariffPlanEditorProps {
  plans: ResellerTariffPlan[];
  ownerId: string;
  ownerType: 'template' | 'sub_account';
  onRefresh: () => void;
}

export function TariffPlanEditor({ plans, ownerId, ownerType, onRefresh }: TariffPlanEditorProps) {
  const toast = useToast();
  const [expandedPlan, setExpandedPlan] = useState<string | null>(null);
  const [showAddPlan, setShowAddPlan] = useState(false);
  const [operators, setOperators] = useState<OperatorRef[]>([]);
  const [countries, setCountries] = useState<CountryRef[]>([]);
  const [refsLoaded, setRefsLoaded] = useState(false);

  // New plan form
  const [newCountry, setNewCountry] = useState('');
  const [newOperator, setNewOperator] = useState('');
  const [newCategory, setNewCategory] = useState('standard');
  const [newTraffic, setNewTraffic] = useState('any');
  const [newStrategy, setNewStrategy] = useState('fixed');
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    if (refsLoaded) return;
    Promise.all([referencesApi.operators(), referencesApi.countries()])
      .then(([opRes, ctRes]) => {
        setOperators(opRes.operators || []);
        setCountries(ctRes.countries || []);
        setRefsLoaded(true);
      })
      .catch(() => toast.error('Ошибка загрузки справочников'));
  }, [refsLoaded]);

  async function handleCreatePlan() {
    setCreating(true);
    try {
      const data: Parameters<typeof resellerTariffApi.createPlan>[0] = {
        sender_category: newCategory,
        traffic_type: newTraffic,
        strategy: newStrategy,
      };
      if (ownerType === 'template') {
        data.template_id = ownerId;
      } else {
        data.sub_account_id = ownerId;
      }
      if (newCountry) data.country_id = newCountry;
      if (newOperator) data.operator_id = newOperator;

      await resellerTariffApi.createPlan(data);
      toast.success('План создан');
      setShowAddPlan(false);
      resetForm();
      onRefresh();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка создания плана');
    } finally {
      setCreating(false);
    }
  }

  async function handleDeletePlan(planId: string) {
    try {
      await resellerTariffApi.deletePlan(planId);
      toast.success('План удалён');
      onRefresh();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка удаления плана');
    }
  }

  function resetForm() {
    setNewCountry('');
    setNewOperator('');
    setNewCategory('standard');
    setNewTraffic('any');
    setNewStrategy('fixed');
  }

  const operatorOptions = [{ value: '', label: 'Все операторы' }, ...operators.map((o) => ({ value: o.id, label: o.name }))];
  const countryOptions = [{ value: '', label: 'Все страны' }, ...countries.map((c) => ({ value: c.id, label: c.name }))];

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div className="text-sm text-gray-500">
          {plans.length} {plans.length === 1 ? 'план' : plans.length < 5 ? 'плана' : 'планов'}
        </div>
        <Button size="sm" onClick={() => setShowAddPlan(true)}>
          Добавить план
        </Button>
      </div>

      {/* Add plan modal */}
      <Modal
        open={showAddPlan}
        onClose={() => setShowAddPlan(false)}
        title="Новый тарифный план"
      >
        <div className="flex flex-col gap-4">
          <Select
            label="Страна"
            options={countryOptions}
            value={newCountry}
            onChange={setNewCountry}
            placeholder="Все страны"
          />
          <Select
            label="Оператор"
            options={operatorOptions}
            value={newOperator}
            onChange={setNewOperator}
            placeholder="Все операторы"
          />
          <Select
            label="Категория"
            options={CATEGORY_OPTIONS}
            value={newCategory}
            onChange={setNewCategory}
          />
          <Select
            label="Тип трафика"
            options={TRAFFIC_TYPE_OPTIONS}
            value={newTraffic}
            onChange={setNewTraffic}
          />
          <Select
            label="Стратегия"
            options={STRATEGY_OPTIONS}
            value={newStrategy}
            onChange={setNewStrategy}
          />
          <div className="flex gap-2">
            <Button onClick={handleCreatePlan} disabled={creating}>
              {creating ? 'Создание...' : 'Создать'}
            </Button>
            <Button variant="secondary" onClick={() => setShowAddPlan(false)}>
              Отмена
            </Button>
          </div>
        </div>
      </Modal>

      {/* Plans list */}
      {plans.length === 0 ? (
        <div className="py-12 text-center border border-gray-200 rounded-lg bg-white">
          <div className="text-gray-400">Нет тарифных планов</div>
          <div className="text-sm text-gray-400 mt-1">Добавьте первый план</div>
        </div>
      ) : (
        <div className="space-y-2">
          {plans.map((plan) => {
            const isExpanded = expandedPlan === plan.id;
            return (
              <div
                key={plan.id}
                className="border border-gray-200 rounded-lg bg-white shadow-sm"
              >
                <div
                  className="flex items-center justify-between px-4 py-3 cursor-pointer hover:bg-gray-50"
                  onClick={() => setExpandedPlan(isExpanded ? null : plan.id)}
                >
                  <div className="flex items-center gap-2 flex-wrap">
                    {plan.country_name && (
                      <Badge variant="info">{plan.country_name}</Badge>
                    )}
                    <Badge variant={plan.operator_name ? 'default' : 'default'}>
                      {plan.operator_name || 'Все операторы'}
                    </Badge>
                    <span className="text-sm text-gray-700">
                      {CATEGORY_LABELS[plan.sender_category] || plan.sender_category}
                    </span>
                    <span className="text-xs text-gray-400">
                      {TRAFFIC_LABELS[plan.traffic_type] || plan.traffic_type}
                    </span>
                    <Badge variant="warning">
                      {STRATEGY_LABELS[plan.strategy] || plan.strategy}
                    </Badge>
                  </div>
                  <div className="flex items-center gap-3">
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleDeletePlan(plan.id);
                      }}
                      className="text-xs text-red-400 hover:text-red-600"
                    >
                      Удалить
                    </button>
                    <span className="text-gray-400 text-xs">{isExpanded ? '▼' : '▶'}</span>
                  </div>
                </div>
                {isExpanded && (
                  <div className="px-4 pb-4 border-t border-gray-100">
                    <PlanPeriodsSection planId={plan.id} strategy={plan.strategy} />
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
