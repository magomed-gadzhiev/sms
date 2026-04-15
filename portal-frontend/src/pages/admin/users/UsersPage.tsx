import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../../components/layout/PageHeader';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../../components/data/FilterBar';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import { Modal } from '../../../components/ui/Modal';
import { ConfirmDialog } from '../../../components/ui/ConfirmDialog';
import { Badge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import {
  usersApi,
  rolesApi,
  clientsApi,
  type UserDetailInfo,
  type RoleDetail,
  type ClientInfo,
} from '../../../api/admin';

const PAGE_SIZE = 20;

const breadcrumbs = [
  { label: 'Админ', href: '/admin/dashboard' },
  { label: 'Пользователи' },
];

export function UsersPage() {
  const toast = useToast();

  const [data, setData] = useState<UserDetailInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [filterValues, setFilterValues] = useState<Record<string, string>>({});

  const [roles, setRoles] = useState<RoleDetail[]>([]);
  const [clients, setClients] = useState<ClientInfo[]>([]);

  const [showCreate, setShowCreate] = useState(false);
  const [editUser, setEditUser] = useState<UserDetailInfo | null>(null);
  const [deactivateUser, setDeactivateUser] = useState<UserDetailInfo | null>(null);
  const [reset2faUser, setReset2faUser] = useState<UserDetailInfo | null>(null);
  const [saving, setSaving] = useState(false);

  // Reset password result
  const [resetPasswordResult, setResetPasswordResult] = useState<{ username: string; password: string } | null>(null);
  const [resetPasswordCopied, setResetPasswordCopied] = useState(false);

  const [createForm, setCreateForm] = useState({
    username: '',
    email: '',
    password: '',
    role_id: '',
    active: true,
    client_id: '',
  });

  const [editForm, setEditForm] = useState({
    email: '',
    role_id: '',
    active: true,
  });

  // Fetch roles and clients for selects
  useEffect(() => {
    rolesApi.list().then((res) => setRoles(res.roles || [])).catch(() => {});
    clientsApi.list({ limit: 500 }).then((res) => setClients(res.clients || [])).catch(() => {});
  }, []);

  const roleOptions = roles.map((r) => ({ value: r.id, label: r.name }));
  const clientOptions = clients.map((c) => ({ value: c.client_id, label: c.name }));

  const filters: FilterDef[] = [
    { key: 'search', label: 'Поиск', type: 'text', placeholder: 'Имя или email...' },
    {
      key: 'role_id',
      label: 'Роль',
      type: 'select',
      options: roleOptions,
      placeholder: 'Все',
    },
    {
      key: 'active_only',
      label: 'Статус',
      type: 'select',
      options: [
        { value: 'true', label: 'Активные' },
      ],
    },
  ];

  const columns: Column<UserDetailInfo>[] = [
    {
      key: 'email',
      header: 'Пользователь',
      sortable: true,
      render: (u) => (
        <div>
          <div className="font-medium text-sm">{u.email}</div>
          {u.username && u.username !== u.email && (
            <div className="text-xs text-gray-400">{u.username}</div>
          )}
        </div>
      ),
    },
    {
      key: 'role',
      header: 'Роль',
      render: (u) => <Badge variant="info">{u.role.name}</Badge>,
    },
    {
      key: 'active',
      header: 'Статус',
      render: (u) => (
        <Badge variant={u.active ? 'success' : 'default'}>
          {u.active ? 'Активен' : 'Неактивен'}
        </Badge>
      ),
    },
    {
      key: 'totp_enabled',
      header: '2FA',
      render: (u) => (
        <Badge variant={u.totp_enabled ? 'success' : 'default'}>
          {u.totp_enabled ? 'Включен' : 'Отключен'}
        </Badge>
      ),
    },
    {
      key: 'last_login_at',
      header: 'Последний вход',
      responsive: true,
      render: (u) =>
        u.last_login_at
          ? new Date(u.last_login_at).toLocaleString()
          : '-',
    },
    {
      key: 'created_at',
      header: 'Создан',
      responsive: true,
      render: (u) => new Date(u.created_at).toLocaleDateString(),
    },
  ];

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await usersApi.list({
        search: filterValues.search || undefined,
        role_id: filterValues.role_id || undefined,
        active_only: filterValues.active_only === 'true' ? true : undefined,
        limit: PAGE_SIZE,
        offset: (page - 1) * PAGE_SIZE,
      });
      setData(res.users || []);
      setTotal(res.total);
    } catch {
      toast.error('Не удалось загрузить пользователей');
    } finally {
      setLoading(false);
    }
  }, [page, filterValues, toast]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // ── Create ──

  const openCreate = () => {
    setCreateForm({ username: '', email: '', password: '', role_id: '', active: true, client_id: '' });
    setShowCreate(true);
  };

  const handleCreate = async () => {
    setSaving(true);
    try {
      await usersApi.create(createForm);
      toast.success('Пользователь создан');
      setShowCreate(false);
      fetchData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Не удалось создать пользователя');
    } finally {
      setSaving(false);
    }
  };

  // ── Edit ──

  const openEdit = (user: UserDetailInfo) => {
    setEditForm({ email: user.email, role_id: user.role.id, active: user.active });
    setEditUser(user);
  };

  const handleEdit = async () => {
    if (!editUser) return;
    setSaving(true);
    try {
      await usersApi.update(editUser.id, editForm);
      toast.success('Пользователь обновлен');
      setEditUser(null);
      fetchData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Не удалось обновить пользователя');
    } finally {
      setSaving(false);
    }
  };

  // ── Deactivate ──

  const handleDeactivate = async () => {
    if (!deactivateUser) return;
    setSaving(true);
    try {
      await usersApi.deactivate(deactivateUser.id);
      toast.success('Пользователь деактивирован');
      setDeactivateUser(null);
      fetchData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Не удалось деактивировать пользователя');
    } finally {
      setSaving(false);
    }
  };

  // ── Reset 2FA ──

  const handleReset2fa = async () => {
    if (!reset2faUser) return;
    setSaving(true);
    try {
      await usersApi.reset2fa(reset2faUser.id);
      toast.success('2FA сброшен');
      setReset2faUser(null);
      fetchData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Не удалось сбросить 2FA');
    } finally {
      setSaving(false);
    }
  };

  // ── Reset Password ──

  const handleResetPassword = async (user: UserDetailInfo) => {
    try {
      const res = await usersApi.resetPassword(user.id);
      setResetPasswordResult({ username: user.username, password: res.temporary_password });
      setResetPasswordCopied(false);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Не удалось сбросить пароль');
    }
  };

  const handleCopyPassword = () => {
    if (resetPasswordResult) {
      navigator.clipboard.writeText(resetPasswordResult.password);
      setResetPasswordCopied(true);
      setTimeout(() => setResetPasswordCopied(false), 2000);
    }
  };

  return (
    <>
      <PageHeader
        title="Пользователи"
        subtitle={`${total} пользователей`}
        breadcrumbs={breadcrumbs}
        actions={<Button onClick={openCreate}>Создать пользователя</Button>}
      />

      <FilterBar
        filters={filters}
        values={filterValues}
        onChange={(v) => { setFilterValues(v); setPage(1); }}
        onReset={() => { setFilterValues({}); setPage(1); }}
      />

      <DataTable
        columns={columns}
        data={data}
        total={total}
        page={page}
        pageSize={PAGE_SIZE}
        onPageChange={setPage}
        loading={loading}
        rowActions={(user) => (
          <div className="flex gap-1">
            <Button size="sm" variant="ghost" onClick={() => openEdit(user)}>
              Редактировать
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setDeactivateUser(user)}>
              Деактивировать
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setReset2faUser(user)}>
              Сбросить 2FA
            </Button>
            <Button size="sm" variant="ghost" onClick={() => handleResetPassword(user)}>
              Сбросить пароль
            </Button>
          </div>
        )}
      />

      {/* Create Modal */}
      <Modal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        title="Создать пользователя"
      >
        <div className="space-y-4">
          <Input
            label="Имя пользователя"
            value={createForm.username}
            onChange={(e) => setCreateForm({ ...createForm, username: e.target.value })}
            required
          />
          <Input
            label="Email"
            type="email"
            value={createForm.email}
            onChange={(e) => setCreateForm({ ...createForm, email: e.target.value })}
            required
          />
          <Input
            label="Пароль"
            type="password"
            value={createForm.password}
            onChange={(e) => setCreateForm({ ...createForm, password: e.target.value })}
            required
          />
          <Select
            label="Роль"
            options={roleOptions}
            value={createForm.role_id}
            onChange={(v) => setCreateForm({ ...createForm, role_id: v })}
            placeholder="Выберите роль"
          />
          <Select
            label="Статус"
            options={[
              { value: 'true', label: 'Активен' },
              { value: 'false', label: 'Неактивен' },
            ]}
            value={String(createForm.active)}
            onChange={(v) => setCreateForm({ ...createForm, active: v === 'true' })}
          />
          <Select
            label="Клиент (опционально)"
            options={clientOptions}
            value={createForm.client_id}
            onChange={(v) => setCreateForm({ ...createForm, client_id: v })}
            placeholder="Без клиента"
          />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowCreate(false)}>
              Отмена
            </Button>
            <Button
              onClick={handleCreate}
              disabled={saving || !createForm.username || !createForm.email || !createForm.password || !createForm.role_id}
            >
              {saving ? 'Сохранение...' : 'Создать'}
            </Button>
          </div>
        </div>
      </Modal>

      {/* Edit Modal */}
      <Modal
        open={!!editUser}
        onClose={() => setEditUser(null)}
        title="Редактировать пользователя"
      >
        <div className="space-y-4">
          <Input
            label="Email"
            type="email"
            value={editForm.email}
            onChange={(e) => setEditForm({ ...editForm, email: e.target.value })}
            required
          />
          <Select
            label="Роль"
            options={roleOptions}
            value={editForm.role_id}
            onChange={(v) => setEditForm({ ...editForm, role_id: v })}
            placeholder="Выберите роль"
          />
          <Select
            label="Статус"
            options={[
              { value: 'true', label: 'Активен' },
              { value: 'false', label: 'Неактивен' },
            ]}
            value={String(editForm.active)}
            onChange={(v) => setEditForm({ ...editForm, active: v === 'true' })}
          />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setEditUser(null)}>
              Отмена
            </Button>
            <Button
              onClick={handleEdit}
              disabled={saving || !editForm.email || !editForm.role_id}
            >
              {saving ? 'Сохранение...' : 'Сохранить'}
            </Button>
          </div>
        </div>
      </Modal>

      {/* Deactivate Confirm */}
      <ConfirmDialog
        open={!!deactivateUser}
        onConfirm={handleDeactivate}
        onCancel={() => setDeactivateUser(null)}
        title="Деактивировать пользователя"
        description={`Деактивировать пользователя "${deactivateUser?.username}"? Пользователь потеряет доступ к системе.`}
        confirmLabel="Деактивировать"
        variant="danger"
        loading={saving}
      />

      {/* Reset 2FA Confirm */}
      <ConfirmDialog
        open={!!reset2faUser}
        onConfirm={handleReset2fa}
        onCancel={() => setReset2faUser(null)}
        title="Сбросить 2FA"
        description={`Сбросить двухфакторную аутентификацию для "${reset2faUser?.username}"? Пользователю потребуется настроить 2FA заново.`}
        confirmLabel="Сбросить"
        variant="danger"
        loading={saving}
      />

      {/* Reset Password Result Modal */}
      <Modal
        open={!!resetPasswordResult}
        onClose={() => setResetPasswordResult(null)}
        title="Пароль сброшен"
        description="Новый временный пароль для пользователя"
      >
        {resetPasswordResult && (
          <div className="space-y-4">
            <p className="text-sm text-gray-600">
              Новый временный пароль для пользователя <strong>{resetPasswordResult.username}</strong>:
            </p>
            <div className="flex items-center gap-2">
              <code className="flex-1 bg-gray-100 px-3 py-2 rounded text-sm font-mono break-all select-all">
                {resetPasswordResult.password}
              </code>
              <Button variant="secondary" size="sm" onClick={handleCopyPassword}>
                {resetPasswordCopied ? 'Скопировано!' : 'Скопировать'}
              </Button>
            </div>
            <p className="text-xs text-gray-500">
              Передайте этот пароль пользователю. Он должен сменить его при следующем входе.
            </p>
            <div className="flex justify-end pt-2">
              <Button onClick={() => setResetPasswordResult(null)}>
                Закрыть
              </Button>
            </div>
          </div>
        )}
      </Modal>
    </>
  );
}
