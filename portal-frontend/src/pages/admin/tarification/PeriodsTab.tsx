import { useState, useEffect, useCallback, useMemo } from 'react';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../../components/data/FilterBar';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import { Modal } from '../../../components/ui/Modal';
import { Badge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import {
  tarificationApi,
  countriesApi,
  operatorsApi,
  clientsApi,
  type HierarchicalPeriod,
  type CountryInfo,
  type OperatorInfo,
  type ClientInfo,
} from '../../../api/admin';

const SENDER_CATEGORY_OPTIONS = [
  { value: 'paid_registered', label: 'paid_registered' },
  { value: 'free_registered', label: 'free_registered' },
  { value: 'shared', label: 'shared' },
];

const TRAFFIC_TYPE_OPTIONS = [
  { value: 'authorization', label: 'authorization' },
  { value: 'transactional', label: 'transactional' },
  { value: 'service', label: 'service' },
  { value: 'extensible', label: 'extensible' },
];

const STRATEGY_OPTIONS = [
  { value: 'fixed', label: 'fixed' },
  { value: 'threshold', label: 'threshold' },
  { value: 'threshold_recalc', label: 'threshold_recalc' },
  { value: 'prepaid_threshold', label: 'prepaid_threshold' },
];

function computeStatus(period: HierarchicalPeriod): {
  label: string;
  variant: 'success' | 'warning' | 'default';
} {
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const start = new Date(period.start_date);
  start.setHours(0, 0, 0, 0);

  if (period.end_date === null && start <= today) {
    return { label: 'Активен', variant: 'success' };
  }
  if (start > today) {
    return { label: 'Запланирован', variant: 'warning' };
  }
  if (period.end_date !== null) {
    const end = new Date(period.end_date);
    end.setHours(0, 0, 0, 0);
    if (end < today) return { label: 'Истёк', variant: 'default' };
    return { label: 'Активен', variant: 'success' };
  }
  return { label: 'Активен', variant: 'success' };
}

function buildScopeLabel(
  period: HierarchicalPeriod,
  countryMap: Record<string, string>,
  operatorMap: Record<string, string>,
  clientMap: Record<string, string>,
): string {
  const base = period.scope_priority % 100;
  let parts: string[] = [];

  if (base === 0) {
    parts.push('Глобальный');
  } else if (base >= 10 && period.country_id) {
    parts.push(countryMap[period.country_id] || period.country_id.slice(0, 8));
  }
  if (base >= 20 && period.operator_id) {
    parts.push(operatorMap[period.operator_id] || period.operator_id.slice(0, 8));
  }
  if (base >= 30 && period.sender_category) {
    parts.push(period.sender_category);
  }
  if (base >= 40 && period.traffic_type) {
    parts.push(period.traffic_type);
  }

  let label = parts.join(' / ');
  if (period.client_id) {
    label += ` [Клиент: ${clientMap[period.client_id] || period.client_id.slice(0, 8)}]`;
  }
  return label || 'Глобальный';
}

interface FormState {
  country_id: string;
  operator_id: string;
  sender_category: string;
  traffic_type: string;
  client_id: string;
  strategy: string;
  start_date: string;
  end_date: string;
}

const EMPTY_FORM: FormState = {
  country_id: '',
  operator_id: '',
  sender_category: '',
  traffic_type: '',
  client_id: '',
  strategy: '',
  start_date: '',
  end_date: '',
};

export function PeriodsTab() {
  const toast = useToast();

  const [periods, setPeriods] = useState<HierarchicalPeriod[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<Record<string, string>>({});

  const [countries, setCountries] = useState<CountryInfo[]>([]);
  const [operators, setOperators] = useState<OperatorInfo[]>([]);
  const [clients, setClients] = useState<ClientInfo[]>([]);

  // Operators filtered for the create modal (cascade from country selection)
  const [modalOperators, setModalOperators] = useState<OperatorInfo[]>([]);

  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [form, setForm] = useState<FormState>(EMPTY_FORM);

  const [editingPeriod, setEditingPeriod] = useState<HierarchicalPeriod | null>(null);
  const [editForm, setEditForm] = useState({ strategy: '', end_date: '' });

  const [deletingId, setDeletingId] = useState<string | null>(null);

  // Load reference data once
  useEffect(() => {
    countriesApi
      .list({ limit: 500 })
      .then((r) => setCountries(r.countries || []))
      .catch(() => toast.error('Не удалось загрузить страны'));
    operatorsApi
      .list({ limit: 1000 })
      .then((r) => setOperators(r.operators || []))
      .catch(() => toast.error('Не удалось загрузить операторов'));
    clientsApi
      .list({ limit: 1000 })
      .then((r) => setClients(r.clients || []))
      .catch(() => toast.error('Не удалось загрузить клиентов'));
  }, []);

  // When form country_id changes, load operators for that country
  useEffect(() => {
    if (!form.country_id) {
      setModalOperators(operators);
      return;
    }
    const filtered = operators.filter((o) => o.country_id === form.country_id);
    setModalOperators(filtered);
    // Reset operator if not in filtered list
    if (form.operator_id && !filtered.find((o) => o.operator_id === form.operator_id)) {
      setForm((f) => ({ ...f, operator_id: '' }));
    }
  }, [form.country_id, operators]);

  const fetchPeriods = useCallback(async () => {
    setLoading(true);
    try {
      const params: Record<string, string> = {};
      if (filter.country_id) params.country_id = filter.country_id;
      if (filter.operator_id) params.operator_id = filter.operator_id;
      if (filter.sender_category) params.sender_category = filter.sender_category;
      if (filter.traffic_type) params.traffic_type = filter.traffic_type;
      if (filter.client_id) params.client_id = filter.client_id;
      const res = await tarificationApi.listPeriods(params);
      setPeriods(res.periods || []);
      setTotal(res.total || 0);
    } catch {
      toast.error('Не удалось загрузить периоды');
    } finally {
      setLoading(false);
    }
  }, [filter, toast]);

  useEffect(() => {
    fetchPeriods();
  }, [fetchPeriods]);

  // Lookup maps
  const countryMap = useMemo(() => {
    const m: Record<string, string> = {};
    for (const c of countries) m[c.country_id] = c.name;
    return m;
  }, [countries]);

  const operatorMap = useMemo(() => {
    const m: Record<string, string> = {};
    for (const o of operators) m[o.operator_id] = o.name;
    return m;
  }, [operators]);

  const clientMap = useMemo(() => {
    const m: Record<string, string> = {};
    for (const c of clients) m[c.client_id] = c.name;
    return m;
  }, [clients]);

  // Filter options
  const countryOptions = useMemo(
    () => countries.map((c) => ({ value: c.country_id, label: c.name })),
    [countries],
  );

  const operatorOptions = useMemo(
    () => operators.map((o) => ({ value: o.operator_id, label: o.name })),
    [operators],
  );

  const clientOptions = useMemo(
    () => clients.map((c) => ({ value: c.client_id, label: c.name })),
    [clients],
  );

  const modalOperatorOptions = useMemo(
    () => modalOperators.map((o) => ({ value: o.operator_id, label: o.name })),
    [modalOperators],
  );

  const filters: FilterDef[] = useMemo(
    () => [
      {
        key: 'country_id',
        label: 'Страна',
        type: 'select' as const,
        options: countryOptions,
        placeholder: 'Все страны',
      },
      {
        key: 'operator_id',
        label: 'Оператор',
        type: 'select' as const,
        options: operatorOptions,
        placeholder: 'Все операторы',
      },
      {
        key: 'sender_category',
        label: 'Категория отправителя',
        type: 'select' as const,
        options: SENDER_CATEGORY_OPTIONS,
        placeholder: 'Все категории',
      },
      {
        key: 'traffic_type',
        label: 'Тип трафика',
        type: 'select' as const,
        options: TRAFFIC_TYPE_OPTIONS,
        placeholder: 'Все типы',
      },
      {
        key: 'client_id',
        label: 'Клиент',
        type: 'select' as const,
        options: clientOptions,
        placeholder: 'Все клиенты',
      },
    ],
    [countryOptions, operatorOptions, clientOptions],
  );

  const handleCreate = async () => {
    if (!form.strategy || !form.start_date) {
      toast.error('Заполните обязательные поля: стратегия и дата начала');
      return;
    }
    setCreating(true);
    try {
      const res = await tarificationApi.createPeriod({
        country_id: form.country_id || null,
        operator_id: form.operator_id || null,
        sender_category: form.sender_category || null,
        traffic_type: form.traffic_type || null,
        client_id: form.client_id || null,
        strategy: form.strategy,
        start_date: form.start_date,
        end_date: form.end_date || null,
      });
      if (res.auto_close_warning) {
        toast.success(`Период создан. ${res.auto_close_warning.message}`);
      } else {
        toast.success('Период создан');
      }
      setShowCreate(false);
      setForm(EMPTY_FORM);
      fetchPeriods();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка создания периода');
    } finally {
      setCreating(false);
    }
  };

  const handleUpdate = async () => {
    if (!editingPeriod) return;
    setUpdating(true);
    try {
      await tarificationApi.updatePeriod(editingPeriod.id, {
        strategy: editForm.strategy || undefined,
        end_date: editForm.end_date || null,
      });
      toast.success('Период обновлён');
      setEditingPeriod(null);
      fetchPeriods();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка обновления');
    } finally {
      setUpdating(false);
    }
  };

  const handleDelete = useCallback(async (id: string) => {
    try {
      await tarificationApi.deletePeriod(id);
      toast.success('Период удалён');
      fetchPeriods();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка удаления');
    }
  }, [fetchPeriods, toast]); // eslint-disable-line react-hooks/exhaustive-deps

  const sortedPeriods = useMemo(
    () => [...periods].sort((a, b) => a.scope_priority - b.scope_priority),
    [periods],
  );

  const columns: Column<HierarchicalPeriod>[] = useMemo(() => [
    {
      key: 'scope_key',
      header: 'Область действия',
      render: (p) => {
        const base = p.scope_priority % 100;
        const indent = Math.floor(base / 10);
        return (
          <div style={{ paddingLeft: `${indent * 12}px` }}>
            {buildScopeLabel(p, countryMap, operatorMap, clientMap)}
          </div>
        );
      },
    },
    {
      key: 'strategy',
      header: 'Стратегия',
      render: (p) => p.strategy,
    },
    {
      key: 'start_date',
      header: 'Дата начала',
      render: (p) => new Date(p.start_date).toLocaleDateString('ru-RU'),
      sortable: true,
    },
    {
      key: 'end_date',
      header: 'Дата окончания',
      render: (p) =>
        p.end_date ? new Date(p.end_date).toLocaleDateString('ru-RU') : '—',
    },
    {
      key: 'scope_priority',
      header: 'Статус',
      render: (p) => {
        const s = computeStatus(p);
        return <Badge variant={s.variant}>{s.label}</Badge>;
      },
    },
    {
      key: 'id',
      header: '',
      render: (p) => (
        <div className="flex gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => {
              setEditingPeriod(p);
              setEditForm({ strategy: p.strategy, end_date: p.end_date || '' });
            }}
          >
            Изменить
          </Button>
          <Button
            size="sm"
            variant="secondary"
            onClick={() => setDeletingId(p.id)}
          >
            Удалить
          </Button>
        </div>
      ),
    },
  ], [countryMap, operatorMap, clientMap]);

  return (
    <>
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold">Иерархические периоды тарификации</h2>
        <Button
          size="sm"
          onClick={() => {
            setForm(EMPTY_FORM);
            setShowCreate(true);
          }}
        >
          Создать период
        </Button>
      </div>

      <FilterBar
        filters={filters}
        values={filter}
        onChange={(v) => setFilter(v)}
        onReset={() => setFilter({})}
      />

      <DataTable
        columns={columns}
        data={sortedPeriods}
        total={total}
        page={1}
        pageSize={100}
        onPageChange={() => {}}
        loading={loading}
        keyField="id"
      />

      <Modal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        title="Создать период тарификации"
      >
        <div className="space-y-4">
          <Select
            label="Страна"
            options={countryOptions}
            value={form.country_id}
            onChange={(v) => setForm({ ...form, country_id: v, operator_id: '' })}
            placeholder="Не задано (глобальный)"
          />
          <Select
            label="Оператор"
            options={modalOperatorOptions}
            value={form.operator_id}
            onChange={(v) => setForm({ ...form, operator_id: v })}
            placeholder="Не задано"
          />
          <Select
            label="Категория отправителя"
            options={SENDER_CATEGORY_OPTIONS}
            value={form.sender_category}
            onChange={(v) => setForm({ ...form, sender_category: v })}
            placeholder="Не задано"
          />
          <Select
            label="Тип трафика"
            options={TRAFFIC_TYPE_OPTIONS}
            value={form.traffic_type}
            onChange={(v) => setForm({ ...form, traffic_type: v })}
            placeholder="Не задано"
          />
          <Select
            label="Клиент"
            options={clientOptions}
            value={form.client_id}
            onChange={(v) => setForm({ ...form, client_id: v })}
            placeholder="Не задано"
          />
          {form.client_id && (
            <Badge variant="warning">Индивидуальный (приоритет +100)</Badge>
          )}
          <Select
            label="Стратегия *"
            options={STRATEGY_OPTIONS}
            value={form.strategy}
            onChange={(v) => setForm({ ...form, strategy: v })}
            placeholder="Выберите стратегию"
          />
          <Input
            label="Дата начала *"
            type="date"
            value={form.start_date}
            onChange={(e) => setForm({ ...form, start_date: e.target.value })}
            required
          />
          <Input
            label="Дата окончания (необязательно)"
            type="date"
            value={form.end_date}
            onChange={(e) => setForm({ ...form, end_date: e.target.value })}
          />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowCreate(false)}>
              Отмена
            </Button>
            <Button
              onClick={handleCreate}
              disabled={creating || !form.strategy || !form.start_date}
            >
              {creating ? 'Создание...' : 'Создать'}
            </Button>
          </div>
        </div>
      </Modal>

      <Modal
        open={editingPeriod !== null}
        onClose={() => setEditingPeriod(null)}
        title="Изменить период"
      >
        <div className="space-y-4">
          <Select
            label="Стратегия"
            options={STRATEGY_OPTIONS}
            value={editForm.strategy}
            onChange={(v) => setEditForm({ ...editForm, strategy: v })}
            placeholder="Выберите стратегию"
          />
          <Input
            label="Дата окончания (необязательно)"
            type="date"
            value={editForm.end_date}
            onChange={(e) => setEditForm({ ...editForm, end_date: e.target.value })}
          />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setEditingPeriod(null)}>
              Отмена
            </Button>
            <Button onClick={handleUpdate} disabled={updating}>
              {updating ? 'Сохранение...' : 'Сохранить'}
            </Button>
          </div>
        </div>
      </Modal>

      <Modal
        open={deletingId !== null}
        onClose={() => setDeletingId(null)}
        title="Подтверждение удаления"
      >
        <div className="space-y-4">
          <p>Вы уверены, что хотите удалить этот период?</p>
          <div className="flex justify-end gap-3">
            <Button variant="secondary" onClick={() => setDeletingId(null)}>
              Отмена
            </Button>
            <Button
              onClick={async () => {
                if (!deletingId) return;
                const id = deletingId;
                setDeletingId(null);
                await handleDelete(id);
              }}
            >
              Удалить
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
