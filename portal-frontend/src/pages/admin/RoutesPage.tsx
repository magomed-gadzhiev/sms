import { useState, useEffect, useCallback, useRef } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Select } from '../../components/ui/Select';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import {
  platformRoutesApi,
  providersApi,
  operatorsApi,
  legalEntitiesApi,
  type PlatformRoute,
  type ProviderInfo,
  type OperatorInfo,
  type LegalEntity,
} from '../../api/admin';

const CHANNEL_OPTIONS = [
  { value: 'sms', label: 'SMS' },
  { value: 'flash', label: 'Flash' },
  { value: 'viber', label: 'Viber' },
  { value: 'whatsapp', label: 'WhatsApp' },
];

interface RouteForm {
  operator_id: string;
  channel_type: string;
  provider_id: string;
  legal_entity_id: string;
}

const DEFAULT_FORM: RouteForm = {
  operator_id: '',
  channel_type: 'sms',
  provider_id: '',
  legal_entity_id: '',
};

export function RoutesPage() {
  const toast = useToast();

  const [routes, setRoutes] = useState<PlatformRoute[]>([]);
  const [providers, setProviders] = useState<ProviderInfo[]>([]);
  const [operators, setOperators] = useState<OperatorInfo[]>([]);
  const [legalEntities, setLegalEntities] = useState<LegalEntity[]>([]);
  const [loading, setLoading] = useState(true);

  const [showForm, setShowForm] = useState(false);
  const [editRoute, setEditRoute] = useState<PlatformRoute | null>(null);
  const [deleteRoute, setDeleteRoute] = useState<PlatformRoute | null>(null);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState<RouteForm>(DEFAULT_FORM);
  const [formErrors, setFormErrors] = useState<Partial<Record<keyof RouteForm, string>>>({});

  const dragItemRef = useRef<number | null>(null);
  const dragOverItemRef = useRef<number | null>(null);

  const fetchRoutes = useCallback(async () => {
    setLoading(true);
    try {
      const res = await platformRoutesApi.list({ limit: 500 });
      setRoutes(res.routes || []);
    } catch {
      toast.error('Не удалось загрузить маршруты');
    } finally {
      setLoading(false);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    fetchRoutes();
    providersApi.list({ limit: 500 }).then((res) => setProviders(res.providers || [])).catch(() => {});
    operatorsApi.list({ limit: 500 }).then((res) => setOperators(res.operators || [])).catch(() => {});
    legalEntitiesApi.list({ limit: 500 }).then((res) => setLegalEntities(res.legal_entities || [])).catch(() => {});
  }, [fetchRoutes]);

  const openCreate = () => {
    setForm(DEFAULT_FORM);
    setFormErrors({});
    setEditRoute(null);
    setShowForm(true);
  };

  const openEdit = (route: PlatformRoute) => {
    setForm({
      operator_id: route.operator_id ?? '',
      channel_type: route.channel_type,
      provider_id: route.provider_id,
      legal_entity_id: route.legal_entity_id ?? '',
    });
    setFormErrors({});
    setEditRoute(route);
    setShowForm(true);
  };

  const handleSave = async () => {
    const errors: Partial<Record<keyof RouteForm, string>> = {};
    if (!form.provider_id) {
      toast.error('Выберите провайдера');
      return;
    }
    if (isAllNetworks && !form.legal_entity_id) {
      errors.legal_entity_id = 'Обязательное поле';
    }
    if (Object.keys(errors).length > 0) {
      setFormErrors(errors);
      return;
    }
    setSaving(true);
    try {
      if (editRoute) {
        await platformRoutesApi.update(editRoute.id, {
          operator_id: form.operator_id || undefined,
          channel_type: form.channel_type,
          provider_id: form.provider_id,
          legal_entity_id: form.legal_entity_id || undefined,
        });
        toast.success('Маршрут обновлён');
      } else {
        await platformRoutesApi.create({
          operator_id: form.operator_id || undefined,
          channel_type: form.channel_type,
          provider_id: form.provider_id,
          legal_entity_id: form.legal_entity_id || undefined,
          priority: routes.length,
        });
        toast.success('Маршрут создан');
      }
      setShowForm(false);
      setEditRoute(null);
      fetchRoutes();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка сохранения');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteRoute) return;
    setSaving(true);
    try {
      await platformRoutesApi.delete(deleteRoute.id);
      toast.success('Маршрут удалён');
      setDeleteRoute(null);
      fetchRoutes();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка удаления');
    } finally {
      setSaving(false);
    }
  };

  const handleDragEnd = async () => {
    const dragIdx = dragItemRef.current;
    const overIdx = dragOverItemRef.current;
    if (dragIdx === null || overIdx === null || dragIdx === overIdx) {
      dragItemRef.current = null;
      dragOverItemRef.current = null;
      return;
    }

    const newRoutes = [...routes];
    const [dragged] = newRoutes.splice(dragIdx, 1);
    newRoutes.splice(overIdx, 0, dragged);
    const updated = newRoutes.map((r, i) => ({ ...r, priority: i }));
    setRoutes(updated);

    dragItemRef.current = null;
    dragOverItemRef.current = null;

    try {
      await platformRoutesApi.reorder(updated.map((r) => ({ id: r.id, priority: r.priority })));
      toast.success('Маршрут перемещён');
    } catch {
      toast.error('Не удалось сохранить порядок');
      fetchRoutes();
    }
  };

  const operatorOptions = [
    { value: '', label: '— All Networks —' },
    ...operators.map((o) => ({
      value: o.operator_id,
      label: `${o.name} (MCC${o.mcc}/MNC${o.mnc})`,
    })),
  ];

  const providerOptions = providers.map((p) => ({ value: p.provider_id, label: p.name }));

  const legalEntityOptions = legalEntities.map((le) => ({
    value: le.id,
    label: `${le.inn} — ${le.name}`,
  }));

  const isAllNetworks = form.operator_id === '';

  return (
    <>
      <PageHeader
        title="Маршруты"
        subtitle={`${routes.length} маршрутов`}
        breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Маршруты' }]}
        actions={<Button onClick={openCreate}>Создать маршрут</Button>}
      />

      <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white">
        <table className="min-w-full divide-y divide-gray-200 text-sm">
          <thead className="bg-gray-50">
            <tr>
              <th className="w-8 px-3 py-3" />
              <th className="px-4 py-3 text-left font-medium text-gray-600">Оператор</th>
              <th className="px-4 py-3 text-left font-medium text-gray-600">Канал</th>
              <th className="px-4 py-3 text-left font-medium text-gray-600">Провайдер</th>
              <th className="px-4 py-3 text-left font-medium text-gray-600">Юр. лицо</th>
              <th className="px-4 py-3 text-left font-medium text-gray-600">Статус</th>
              <th className="px-4 py-3 text-right font-medium text-gray-600">Действия</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {loading && (
              <tr>
                <td colSpan={7} className="px-4 py-8 text-center text-gray-400">
                  Загрузка...
                </td>
              </tr>
            )}
            {!loading && routes.length === 0 && (
              <tr>
                <td colSpan={7} className="px-4 py-12">
                  <div className="text-center text-gray-500">
                    <p className="text-lg font-medium">Нет маршрутов</p>
                    <p className="text-sm mt-1">Создайте первый маршрут для начала маршрутизации</p>
                    <Button onClick={openCreate} className="mt-4" size="sm">Создать маршрут</Button>
                  </div>
                </td>
              </tr>
            )}
            {routes.map((route, index) => (
              <tr
                key={route.id}
                draggable
                onDragStart={() => { dragItemRef.current = index; }}
                onDragEnter={() => { dragOverItemRef.current = index; }}
                onDragEnd={handleDragEnd}
                onDragOver={(e) => e.preventDefault()}
                className="hover:bg-gray-50 transition-colors"
              >
                <td className="px-3 py-3 text-gray-400 select-none text-center cursor-grab active:cursor-grabbing">⠿</td>
                <td className="px-4 py-3">
                  {route.operator_id === null ? (
                    <span className="font-bold text-blue-600">All Networks</span>
                  ) : (
                    <span>{route.operator_name}</span>
                  )}
                </td>
                <td className="px-4 py-3 uppercase text-xs font-medium text-gray-700">
                  {route.channel_type}
                </td>
                <td className="px-4 py-3">{route.provider_name}</td>
                <td className="px-4 py-3 text-gray-600">
                  {route.legal_entity_id
                    ? `${route.legal_entity_inn} — ${route.legal_entity_name}`
                    : '—'}
                </td>
                <td className="px-4 py-3">
                  <StatusBadge status={route.active ? 'active' : 'inactive'} />
                </td>
                <td className="px-4 py-3 text-right">
                  <div className="flex justify-end gap-1">
                    <Button size="sm" variant="ghost" onClick={() => openEdit(route)}>
                      Изменить
                    </Button>
                    <Button size="sm" variant="ghost" onClick={() => setDeleteRoute(route)}>
                      Удалить
                    </Button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <Modal
        open={showForm}
        onClose={() => { setShowForm(false); setEditRoute(null); }}
        title={editRoute ? 'Редактирование маршрута' : 'Создание маршрута'}
      >
        <div className="space-y-4">
          <Select
            label="Оператор"
            options={operatorOptions}
            value={form.operator_id}
            onChange={(v) => {
              setForm({ ...form, operator_id: v, legal_entity_id: '' });
              setFormErrors((e) => ({ ...e, legal_entity_id: undefined }));
            }}
          />
          {isAllNetworks && (
            <div>
              <Select
                label="Юр. лицо *"
                options={legalEntityOptions}
                value={form.legal_entity_id}
                onChange={(v) => {
                  setForm({ ...form, legal_entity_id: v });
                  setFormErrors((e) => ({ ...e, legal_entity_id: undefined }));
                }}
                required
              />
              {formErrors.legal_entity_id && (
                <p className="text-xs text-red-500 mt-1">{formErrors.legal_entity_id}</p>
              )}
            </div>
          )}
          <Select
            label="Тип канала"
            options={CHANNEL_OPTIONS}
            value={form.channel_type}
            onChange={(v) => setForm({ ...form, channel_type: v })}
          />
          <Select
            label="Провайдер"
            options={providerOptions}
            value={form.provider_id}
            onChange={(v) => setForm({ ...form, provider_id: v })}
            required
          />
          {!isAllNetworks && (
            <Select
              label="Юр. лицо"
              options={[{ value: '', label: '— не указано —' }, ...legalEntityOptions]}
              value={form.legal_entity_id}
              onChange={(v) => setForm({ ...form, legal_entity_id: v })}
            />
          )}
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => { setShowForm(false); setEditRoute(null); }}>
              Отмена
            </Button>
            <Button onClick={handleSave} disabled={saving || !form.provider_id}>
              {saving ? 'Сохранение...' : 'Сохранить'}
            </Button>
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!deleteRoute}
        onConfirm={handleDelete}
        onCancel={() => setDeleteRoute(null)}
        title="Удаление маршрута"
        description={`Удалить маршрут "${deleteRoute?.operator_name ?? ''} / ${deleteRoute?.channel_type ?? ''}"? Это действие необратимо.`}
        confirmLabel="Удалить"
        variant="danger"
        loading={saving}
      />
    </>
  );
}
