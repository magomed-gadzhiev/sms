import { useState, useEffect, useCallback } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { PageHeader } from '../../../components/layout/PageHeader';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import { Modal } from '../../../components/ui/Modal';
import { ConfirmDialog } from '../../../components/ui/ConfirmDialog';
import { Badge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import {
  clientsApi,
  tarificationApi,
  type ClientInfo,
  type HierarchicalPeriod,
  type TariffTier,
  type CreateHierarchicalPeriodRequest,
} from '../../../api/admin';

const STRATEGY_OPTIONS = [
  { value: 'fixed', label: 'fixed' },
  { value: 'threshold', label: 'threshold' },
  { value: 'threshold_recalc', label: 'threshold_recalc' },
  { value: 'prepaid_threshold', label: 'prepaid_threshold' },
];

function periodStatus(p: HierarchicalPeriod): { label: string; variant: 'success' | 'warning' | 'default' } {
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const start = new Date(p.start_date);
  start.setHours(0, 0, 0, 0);
  if (start > today) return { label: 'Запланирован', variant: 'warning' };
  if (p.end_date) {
    const end = new Date(p.end_date);
    end.setHours(0, 0, 0, 0);
    if (end < today) return { label: 'Истёк', variant: 'default' };
  }
  return { label: 'Активен', variant: 'success' };
}

export function IndividualTariffsPage() {
  const toast = useToast();

  const [clients, setClients] = useState<ClientInfo[]>([]);
  const [selectedClientId, setSelectedClientId] = useState('');
  const [tab, setTab] = useState('periods');

  const [periods, setPeriods] = useState<HierarchicalPeriod[]>([]);
  const [periodsLoading, setPeriodsLoading] = useState(false);
  const [showPeriodForm, setShowPeriodForm] = useState(false);
  const [deletePeriod, setDeletePeriod] = useState<HierarchicalPeriod | null>(null);
  const [savingPeriod, setSavingPeriod] = useState(false);
  const [periodForm, setPeriodForm] = useState({ strategy: 'fixed', start_date: '', end_date: '' });

  const [tiers, setTiers] = useState<TariffTier[]>([]);
  const [tiersLoading, setTiersLoading] = useState(false);
  const [selectedPeriodId, setSelectedPeriodId] = useState('');
  const [showTierForm, setShowTierForm] = useState(false);
  const [editTier, setEditTier] = useState<TariffTier | null>(null);
  const [savingTier, setSavingTier] = useState(false);
  const [tierForm, setTierForm] = useState({ tariff_period_id: '', from_count: 0, price_per_segment: '' });

  const clientOptions = clients.map((c) => ({ value: c.client_id, label: c.name }));
  const periodOptions = periods.map((p) => ({
    value: p.id,
    label: `${p.start_date} — ${p.end_date ?? '∞'} (${p.strategy})`,
  }));

  useEffect(() => {
    clientsApi.list({ limit: 500 }).then((r) => setClients(r.clients || [])).catch(() => {});
  }, []);

  const fetchPeriods = useCallback(async () => {
    if (!selectedClientId) return;
    setPeriodsLoading(true);
    try {
      const res = await tarificationApi.listPeriods({ client_id: selectedClientId });
      setPeriods(res.periods || []);
    } catch {
      toast.error('Не удалось загрузить периоды');
    } finally {
      setPeriodsLoading(false);
    }
  }, [selectedClientId, toast]);

  const fetchTiers = useCallback(async () => {
    if (!selectedPeriodId) return;
    setTiersLoading(true);
    try {
      const res = await tarificationApi.listTariffTiers({ tariff_period_id: selectedPeriodId });
      setTiers(res.tiers || []);
    } catch {
      toast.error('Не удалось загрузить тиры');
    } finally {
      setTiersLoading(false);
    }
  }, [selectedPeriodId, toast]);

  useEffect(() => { fetchPeriods(); }, [fetchPeriods]);
  useEffect(() => { fetchTiers(); }, [fetchTiers]);

  async function handleCreatePeriod() {
    if (!periodForm.strategy || !periodForm.start_date) {
      toast.error('Заполните стратегию и дату начала');
      return;
    }
    setSavingPeriod(true);
    try {
      const req: CreateHierarchicalPeriodRequest = {
        client_id: selectedClientId,
        strategy: periodForm.strategy,
        start_date: periodForm.start_date,
        end_date: periodForm.end_date || null,
      };
      await tarificationApi.createPeriod(req);
      toast.success('Период создан');
      setShowPeriodForm(false);
      setPeriodForm({ strategy: 'fixed', start_date: '', end_date: '' });
      fetchPeriods();
    } catch {
      toast.error('Не удалось создать период');
    } finally {
      setSavingPeriod(false);
    }
  }

  async function handleDeletePeriod() {
    if (!deletePeriod) return;
    try {
      await tarificationApi.deletePeriod(deletePeriod.id);
      toast.success('Период удалён');
      setDeletePeriod(null);
      fetchPeriods();
    } catch {
      toast.error('Не удалось удалить период');
    }
  }

  function openCreateTier() {
    setEditTier(null);
    setTierForm({ tariff_period_id: selectedPeriodId, from_count: 0, price_per_segment: '' });
    setShowTierForm(true);
  }

  function openEditTier(t: TariffTier) {
    setEditTier(t);
    setTierForm({ tariff_period_id: t.tariff_period_id, from_count: t.from_count, price_per_segment: t.price_per_segment });
    setShowTierForm(true);
  }

  async function handleSaveTier() {
    if (!tierForm.tariff_period_id || !tierForm.price_per_segment) {
      toast.error('Выберите период и укажите цену');
      return;
    }
    setSavingTier(true);
    try {
      if (editTier) {
        await tarificationApi.updateTariffTier(editTier.id, { from_count: tierForm.from_count, price_per_segment: tierForm.price_per_segment });
        toast.success('Тир обновлён');
      } else {
        await tarificationApi.createTariffTier({ tariff_period_id: tierForm.tariff_period_id, from_count: tierForm.from_count, price_per_segment: tierForm.price_per_segment });
        toast.success('Тир создан');
      }
      setShowTierForm(false);
      fetchTiers();
    } catch {
      toast.error('Не удалось сохранить тир');
    } finally {
      setSavingTier(false);
    }
  }

  const selectedClient = clients.find((c) => c.client_id === selectedClientId);

  const periodColumns: Column<HierarchicalPeriod>[] = [
    { key: 'start_date', header: 'Начало', sortable: true },
    { key: 'end_date', header: 'Конец', render: (r) => r.end_date ?? '∞' },
    { key: 'strategy', header: 'Стратегия' },
    { key: 'scope_priority', header: 'Приоритет', sortable: true },
    {
      key: 'id',
      header: 'Статус',
      render: (r) => {
        const s = periodStatus(r);
        return <Badge variant={s.variant}>{s.label}</Badge>;
      },
    },
  ];

  const tierColumns: Column<TariffTier>[] = [
    { key: 'from_count', header: 'От (сегментов)', sortable: true },
    { key: 'price_per_segment', header: 'Цена за сегмент' },
  ];

  const tabCls = (value: string) =>
    `px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
      tab === value
        ? 'border-primary text-primary'
        : 'border-transparent text-gray-500 hover:text-gray-700'
    }`;

  return (
    <>
      <PageHeader
        title="Индивидуальные тарифы"
        breadcrumbs={[
          { label: 'Админ', href: '/admin/dashboard' },
          { label: 'Тарификация', href: '/admin/tarification' },
          { label: 'Индивидуальные тарифы' },
        ]}
      />

      <div className="mb-4 w-80">
        <label className="block text-sm font-medium text-gray-700 mb-1">Клиент</label>
        <Select
          value={selectedClientId}
          onChange={(v) => {
            setSelectedClientId(v);
            setSelectedPeriodId('');
            setPeriods([]);
            setTiers([]);
          }}
          options={clientOptions}
          placeholder="Выберите клиента..."
        />
      </div>

      {!selectedClientId && (
        <div className="text-center py-16 text-gray-400">
          Выберите клиента для просмотра его тарифов
        </div>
      )}

      {selectedClientId && (
        <>
          <div className="mb-4 p-3 bg-blue-50 border border-blue-200 rounded-md text-sm text-blue-800">
            Индивидуальный тариф для: <strong>{selectedClient?.name}</strong>
          </div>

          <Tabs.Root value={tab} onValueChange={setTab}>
            <Tabs.List className="flex border-b border-gray-200 mb-4">
              <Tabs.Trigger value="periods" className={tabCls('periods')}>Периоды</Tabs.Trigger>
              <Tabs.Trigger value="tiers" className={tabCls('tiers')}>Тиры</Tabs.Trigger>
            </Tabs.List>

            <Tabs.Content value="periods">
              <div className="mb-3 flex justify-end">
                <Button onClick={() => { setPeriodForm({ strategy: 'fixed', start_date: '', end_date: '' }); setShowPeriodForm(true); }}>
                  + Добавить период
                </Button>
              </div>
              <DataTable
                columns={periodColumns}
                data={periods}
                total={periods.length}
                page={1}
                pageSize={periods.length || 1}
                onPageChange={() => {}}
                loading={periodsLoading}
                rowActions={(r) => (
                  <Button size="sm" variant="ghost" onClick={() => setDeletePeriod(r)}>Удалить</Button>
                )}
              />
            </Tabs.Content>

            <Tabs.Content value="tiers">
              <div className="mb-3 flex items-end gap-4">
                <div className="w-80">
                  <label className="block text-sm font-medium text-gray-700 mb-1">Период</label>
                  <Select
                    value={selectedPeriodId}
                    onChange={setSelectedPeriodId}
                    options={periodOptions}
                    placeholder="Выберите период..."
                  />
                </div>
                {selectedPeriodId && (
                  <Button onClick={openCreateTier}>+ Добавить тир</Button>
                )}
              </div>
              {selectedPeriodId ? (
                <DataTable
                  columns={tierColumns}
                  data={tiers}
                  total={tiers.length}
                  page={1}
                  pageSize={tiers.length || 1}
                  onPageChange={() => {}}
                  loading={tiersLoading}
                  rowActions={(t) => (
                    <Button size="sm" variant="ghost" onClick={() => openEditTier(t)}>Изменить</Button>
                  )}
                />
              ) : (
                <div className="text-center py-8 text-gray-400 text-sm">
                  Выберите период для просмотра тиров
                </div>
              )}
            </Tabs.Content>
          </Tabs.Root>
        </>
      )}

      <Modal open={showPeriodForm} onClose={() => setShowPeriodForm(false)} title="Добавить период">
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Стратегия *</label>
            <Select
              value={periodForm.strategy}
              onChange={(v) => setPeriodForm((f) => ({ ...f, strategy: v }))}
              options={STRATEGY_OPTIONS}
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Дата начала *</label>
              <Input
                type="date"
                value={periodForm.start_date}
                onChange={(e) => setPeriodForm((f) => ({ ...f, start_date: e.target.value }))}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Дата окончания</label>
              <Input
                type="date"
                value={periodForm.end_date}
                onChange={(e) => setPeriodForm((f) => ({ ...f, end_date: e.target.value }))}
              />
            </div>
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="ghost" onClick={() => setShowPeriodForm(false)}>Отмена</Button>
            <Button onClick={handleCreatePeriod} disabled={savingPeriod}>
              {savingPeriod ? 'Создание...' : 'Создать'}
            </Button>
          </div>
        </div>
      </Modal>

      <Modal open={showTierForm} onClose={() => setShowTierForm(false)} title={editTier ? 'Изменить тир' : 'Добавить тир'}>
        <div className="space-y-4">
          {!editTier && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Период *</label>
              <Select
                value={tierForm.tariff_period_id}
                onChange={(v) => setTierForm((f) => ({ ...f, tariff_period_id: v }))}
                options={periodOptions}
                placeholder="Выберите период..."
              />
            </div>
          )}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">От (сегментов)</label>
              <Input
                type="number"
                min={0}
                value={String(tierForm.from_count)}
                onChange={(e) => setTierForm((f) => ({ ...f, from_count: Number(e.target.value) }))}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Цена за сегмент *</label>
              <Input
                type="text"
                placeholder="0.05"
                value={tierForm.price_per_segment}
                onChange={(e) => setTierForm((f) => ({ ...f, price_per_segment: e.target.value }))}
              />
            </div>
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="ghost" onClick={() => setShowTierForm(false)}>Отмена</Button>
            <Button onClick={handleSaveTier} disabled={savingTier}>
              {savingTier ? 'Сохранение...' : 'Сохранить'}
            </Button>
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!deletePeriod}
        title="Удалить период?"
        description={`Период с ${deletePeriod?.start_date ?? ''} будет удалён вместе со всеми тирами.`}
        onConfirm={handleDeletePeriod}
        onCancel={() => setDeletePeriod(null)}
      />
    </>
  );
}
