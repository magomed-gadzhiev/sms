import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { usePageTitle } from '../../contexts/PageTitleContext';
import { Button } from '../../components/ui/Button';
import { Drawer } from '../../components/ui/Drawer';
import { Input } from '../../components/ui/Input';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { useToast } from '../../components/ui/Toast';
import { networkApi, ApiError } from '../../api/client';
import type { NetworkProvider } from '../../api/client';

interface ProviderForm {
  name: string;
  smpp_host: string;
  smpp_port: number;
  system_id: string;
  password: string;
  system_type: string;
}

const emptyForm: ProviderForm = {
  name: '',
  smpp_host: '',
  smpp_port: 2775,
  system_id: '',
  password: '',
  system_type: 'SMPP',
};

export function ProvidersCatalogPage() {
  usePageTitle('Провайдеры');
  const toast = useToast();
  const [list, setList] = useState<NetworkProvider[]>([]);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editing, setEditing] = useState<NetworkProvider | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<NetworkProvider | null>(null);
  const [form, setForm] = useState<ProviderForm>(emptyForm);
  const [submitting, setSubmitting] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const reload = useCallback(() => {
    networkApi
      .listProviders()
      .then((r) => setList(r.providers))
      .catch(() => toast.error('Ошибка загрузки'));
  }, [toast]);

  useEffect(() => {
    reload();
  }, [reload]);

  const openCreate = () => {
    setEditing(null);
    setForm(emptyForm);
    setDrawerOpen(true);
  };

  const openEdit = (p: NetworkProvider) => {
    setEditing(p);
    setForm({
      name: p.name,
      smpp_host: p.smpp_host || '',
      smpp_port: p.smpp_port || 2775,
      system_id: p.system_id || '',
      password: '',
      system_type: p.system_type || 'SMPP',
    });
    setDrawerOpen(true);
  };

  const submit = async () => {
    setSubmitting(true);
    try {
      if (editing) {
        // On edit: send password only if user entered something (otherwise keep current)
        const payload: {
          name: string;
          smpp_host: string;
          smpp_port: number;
          system_id: string;
          password?: string;
          system_type?: string;
        } = {
          name: form.name,
          smpp_host: form.smpp_host,
          smpp_port: form.smpp_port,
          system_id: form.system_id,
          system_type: form.system_type,
        };
        if (form.password) payload.password = form.password;
        await networkApi.updateProvider(editing.id, payload);
      } else {
        await networkApi.createProvider({
          name: form.name,
          smpp_host: form.smpp_host,
          smpp_port: form.smpp_port,
          system_id: form.system_id,
          password: form.password,
          system_type: form.system_type,
        });
      }
      toast.success(editing ? 'Обновлён' : 'Создан');
      setDrawerOpen(false);
      reload();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    } finally {
      setSubmitting(false);
    }
  };

  const doDelete = async () => {
    if (!confirmDelete) return;
    setDeleting(true);
    try {
      await networkApi.deleteProvider(confirmDelete.id);
      toast.success('Удалён');
      setConfirmDelete(null);
      reload();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
      setConfirmDelete(null);
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div className="max-w-6xl">
      <PageHeader
        actions={<Button onClick={openCreate}>+ Добавить своего</Button>}
      />
      <table className="w-full text-sm bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-slate-700">
        <thead>
          <tr className="bg-gray-50 dark:bg-slate-950 text-gray-500 dark:text-slate-400 text-xs uppercase">
            <th className="text-left p-3">Имя</th>
            <th className="text-left p-3">Ownership</th>
            <th className="text-left p-3">SMPP host</th>
            <th className="text-center p-3">Активен</th>
            <th className="text-right p-3">Действия</th>
          </tr>
        </thead>
        <tbody>
          {list.length === 0 ? (
            <tr>
              <td colSpan={5} className="p-6 text-center text-gray-400 dark:text-slate-500">
                Нет провайдеров
              </td>
            </tr>
          ) : (
            list.map((p) => (
              <tr key={p.id} className="border-t border-gray-100 dark:border-slate-800">
                <td className="p-3 font-medium">{p.name}</td>
                <td className="p-3">
                  <span
                    className={`px-2 py-0.5 text-xs rounded ${
                      p.ownership === 'private'
                        ? 'bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-300'
                        : 'bg-gray-100 dark:bg-slate-800 text-gray-600 dark:text-slate-400'
                    }`}
                  >
                    {p.ownership === 'private' ? 'Свой' : 'Платформенный'}
                  </span>
                </td>
                <td className="p-3 text-gray-500 dark:text-slate-400">
                  {p.smpp_host || '—'}
                  {p.smpp_port ? `:${p.smpp_port}` : ''}
                </td>
                <td className="p-3 text-center">
                  <span
                    className={`inline-block w-2 h-2 rounded-full ${
                      p.active ? 'bg-green-500' : 'bg-gray-300'
                    }`}
                  />
                </td>
                <td className="p-3 text-right">
                  {p.ownership === 'private' && (
                    <>
                      <button
                        onClick={() => openEdit(p)}
                        className="text-blue-600 dark:text-blue-400 hover:underline mr-3"
                      >
                        Редактировать
                      </button>
                      <button
                        onClick={() => setConfirmDelete(p)}
                        className="text-red-600 dark:text-red-400 hover:underline"
                      >
                        Удалить
                      </button>
                    </>
                  )}
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>

      <Drawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        title={editing ? 'Редактировать провайдера' : 'Добавить своего провайдера'}
        width="md"
      >
        <div className="space-y-4">
          <Input
            label="Имя"
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
          />
          <Input
            label="SMPP host"
            value={form.smpp_host}
            onChange={(e) => setForm({ ...form, smpp_host: e.target.value })}
          />
          <Input
            label="SMPP port"
            type="number"
            value={String(form.smpp_port)}
            onChange={(e) =>
              setForm({ ...form, smpp_port: parseInt(e.target.value, 10) || 2775 })
            }
          />
          <Input
            label="System ID"
            value={form.system_id}
            onChange={(e) => setForm({ ...form, system_id: e.target.value })}
          />
          <Input
            label="Password"
            type="password"
            value={form.password}
            onChange={(e) => setForm({ ...form, password: e.target.value })}
            placeholder={editing ? 'оставьте пустым, чтобы не менять' : ''}
          />
          <Input
            label="System type"
            value={form.system_type}
            onChange={(e) => setForm({ ...form, system_type: e.target.value })}
          />
          <Button onClick={submit} disabled={submitting}>
            {submitting ? 'Сохранение...' : 'Сохранить'}
          </Button>
        </div>
      </Drawer>

      <ConfirmDialog
        open={!!confirmDelete}
        onCancel={() => setConfirmDelete(null)}
        onConfirm={doDelete}
        title="Удалить провайдера?"
        description={`«${confirmDelete?.name ?? ''}» будет удалён. Если он используется в provider-set'ах или override'ах — операция вернёт 409.`}
        variant="danger"
        confirmLabel="Удалить"
        loading={deleting}
      />
    </div>
  );
}
