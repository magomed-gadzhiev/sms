import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../../components/layout/PageHeader';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Modal } from '../../../components/ui/Modal';
import { ConfirmDialog } from '../../../components/ui/ConfirmDialog';
import { Badge } from '../../../components/ui/Badge';
import { PermissionMatrix } from '../../../components/data/PermissionMatrix';
import { useToast } from '../../../components/ui/Toast';
import {
  rolesApi,
  permissionsApi,
  type RoleDetail,
  type PermissionInfo,
} from '../../../api/admin';

const breadcrumbs = [
  { label: 'Админ', href: '/admin/dashboard' },
  { label: 'Пользователи', href: '/admin/users' },
  { label: 'Роли' },
];

export function RolesPage() {
  const toast = useToast();

  const [data, setData] = useState<RoleDetail[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);

  const [allPermissions, setAllPermissions] = useState<PermissionInfo[]>([]);

  const [showCreate, setShowCreate] = useState(false);
  const [editRole, setEditRole] = useState<RoleDetail | null>(null);
  const [deleteRole, setDeleteRole] = useState<RoleDetail | null>(null);
  const [saving, setSaving] = useState(false);

  const [form, setForm] = useState({
    name: '',
    description: '',
    permission_ids: new Set<string>(),
  });

  // Fetch all permissions on mount
  useEffect(() => {
    permissionsApi.list().then((res) => setAllPermissions(res.permissions || [])).catch(() => {});
  }, []);

  const columns: Column<RoleDetail>[] = [
    { key: 'name', header: 'Название', sortable: true },
    { key: 'description', header: 'Описание' },
    {
      key: 'user_count',
      header: 'Пользователи',
      render: (r) => String(r.user_count),
    },
    {
      key: 'builtin',
      header: 'Встроенная',
      render: (r) => (
        <Badge variant={r.builtin ? 'info' : 'default'}>
          {r.builtin ? 'Да' : 'Нет'}
        </Badge>
      ),
    },
  ];

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await rolesApi.list();
      setData(res.roles || []);
      setTotal(res.total);
    } catch {
      toast.error('Не удалось загрузить роли');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // ── Create ──

  const openCreate = () => {
    setForm({ name: '', description: '', permission_ids: new Set() });
    setShowCreate(true);
  };

  const handleCreate = async () => {
    setSaving(true);
    try {
      await rolesApi.create({
        name: form.name,
        description: form.description,
        permission_ids: Array.from(form.permission_ids),
      });
      toast.success('Роль создана');
      setShowCreate(false);
      fetchData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Не удалось создать роль');
    } finally {
      setSaving(false);
    }
  };

  // ── Edit ──

  const openEdit = (role: RoleDetail) => {
    setForm({
      name: role.name,
      description: role.description,
      permission_ids: new Set(role.permissions.map((p) => p.id)),
    });
    setEditRole(role);
  };

  const handleEdit = async () => {
    if (!editRole) return;
    setSaving(true);
    try {
      await rolesApi.update(editRole.id, {
        name: form.name,
        description: form.description,
        permission_ids: Array.from(form.permission_ids),
      });
      toast.success('Роль обновлена');
      setEditRole(null);
      fetchData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Не удалось обновить роль');
    } finally {
      setSaving(false);
    }
  };

  // ── Delete ──

  const handleDelete = async () => {
    if (!deleteRole) return;
    setSaving(true);
    try {
      await rolesApi.delete(deleteRole.id);
      toast.success('Роль удалена');
      setDeleteRole(null);
      fetchData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Не удалось удалить роль');
    } finally {
      setSaving(false);
    }
  };

  const isEditing = !!editRole;
  const isBuiltin = editRole?.builtin ?? false;

  return (
    <>
      <PageHeader
        title="Роли"
        subtitle={`${total} ролей`}
        breadcrumbs={breadcrumbs}
        actions={<Button onClick={openCreate}>Создать роль</Button>}
      />

      <DataTable
        columns={columns}
        data={data}
        total={total}
        page={page}
        pageSize={data.length || 20}
        onPageChange={setPage}
        loading={loading}
        rowActions={(role) => (
          <div className="flex gap-1">
            <Button size="sm" variant="ghost" onClick={() => openEdit(role)}>
              Редактировать
            </Button>
            <Button
              size="sm"
              variant="ghost"
              disabled={role.builtin}
              onClick={() => setDeleteRole(role)}
            >
              Удалить
            </Button>
          </div>
        )}
      />

      {/* Create / Edit Modal */}
      <Modal
        open={showCreate || isEditing}
        onClose={() => { setShowCreate(false); setEditRole(null); }}
        title={isEditing ? 'Редактировать роль' : 'Создать роль'}
        wide
      >
        <div className="space-y-4">
          <Input
            label="Название"
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
            disabled={isEditing && isBuiltin}
            required
          />
          <Input
            label="Описание"
            value={form.description}
            onChange={(e) => setForm({ ...form, description: e.target.value })}
          />
          <div>
            <label className="text-sm font-medium text-gray-700 mb-2 block">
              Разрешения
            </label>
            <PermissionMatrix
              allPermissions={allPermissions}
              selectedIds={form.permission_ids}
              onChange={(ids) => setForm({ ...form, permission_ids: ids })}
            />
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <Button
              variant="secondary"
              onClick={() => { setShowCreate(false); setEditRole(null); }}
            >
              Отмена
            </Button>
            <Button
              onClick={isEditing ? handleEdit : handleCreate}
              disabled={saving || !form.name}
            >
              {saving ? 'Сохранение...' : 'Сохранить'}
            </Button>
          </div>
        </div>
      </Modal>

      {/* Delete Confirm */}
      <ConfirmDialog
        open={!!deleteRole}
        onConfirm={handleDelete}
        onCancel={() => setDeleteRole(null)}
        title="Удалить роль"
        description={`Удалить роль "${deleteRole?.name}"? Это действие необратимо.`}
        confirmLabel="Удалить"
        variant="danger"
        loading={saving}
      />
    </>
  );
}
