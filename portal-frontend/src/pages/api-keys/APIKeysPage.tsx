import { useState, useEffect, type FormEvent } from 'react';
import { apiKeysApi, ApiError, type APIKeyInfo, type CreateAPIKeyResponse } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
import { DataTable, type Column } from '../../components/data/DataTable';
import { StatusBadge } from '../../components/ui/Badge';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';

const AVAILABLE_SCOPES = [
  'messages:send',
  'messages:read',
  'webhooks:manage',
  'analytics:read',
  'sub-accounts:manage',
];

const EDIT_SCOPES = [
  'sms:send',
  'sms:read',
  'billing:read',
  'billing:write',
  'webhooks:manage',
];

export function APIKeysPage() {
  const [keys, setKeys] = useState<APIKeyInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Create form state
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [name, setName] = useState('');
  const [allowedIpsInput, setAllowedIpsInput] = useState('');
  const [selectedScopes, setSelectedScopes] = useState<string[]>([]);
  const [expiresAt, setExpiresAt] = useState('');
  const [creating, setCreating] = useState(false);

  // Created key (shown once)
  const [createdKey, setCreatedKey] = useState<CreateAPIKeyResponse | null>(null);
  const [copied, setCopied] = useState(false);

  // Revoke confirmation
  const [revokeId, setRevokeId] = useState<string | null>(null);

  // Detail/Edit modal
  const [detailKey, setDetailKey] = useState<APIKeyInfo | null>(null);
  const [editMode, setEditMode] = useState(false);
  const [editName, setEditName] = useState('');
  const [editScopes, setEditScopes] = useState<string[]>([]);
  const [editAllowedIps, setEditAllowedIps] = useState('');
  const [editExpiresAt, setEditExpiresAt] = useState('');
  const [editSaving, setEditSaving] = useState(false);
  const [editError, setEditError] = useState('');

  async function loadKeys() {
    setLoading(true);
    setError('');
    try {
      const resp = await apiKeysApi.list();
      setKeys(resp.keys || []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить API ключи');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadKeys();
  }, []);

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    setCreating(true);
    setError('');
    try {
      const allowedIps = allowedIpsInput
        .split(/[,\n]/)
        .map((s) => s.trim())
        .filter(Boolean);

      const resp = await apiKeysApi.create({
        name,
        allowed_ips: allowedIps.length > 0 ? allowedIps : undefined,
        scopes: selectedScopes.length > 0 ? selectedScopes : undefined,
        expires_at: expiresAt || undefined,
      });

      setCreatedKey(resp);
      setShowCreateForm(false);
      setName('');
      setAllowedIpsInput('');
      setSelectedScopes([]);
      setExpiresAt('');
      await loadKeys();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось создать API ключ');
    } finally {
      setCreating(false);
    }
  }

  async function handleRevoke(id: string) {
    setError('');
    try {
      await apiKeysApi.revoke(id);
      setRevokeId(null);
      await loadKeys();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось отозвать API ключ');
    }
  }

  function handleCopyKey() {
    if (createdKey) {
      navigator.clipboard.writeText(createdKey.api_key);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }

  function toggleScope(scope: string) {
    setSelectedScopes((prev) =>
      prev.includes(scope) ? prev.filter((s) => s !== scope) : [...prev, scope]
    );
  }

  function toggleEditScope(scope: string) {
    setEditScopes((prev) =>
      prev.includes(scope) ? prev.filter((s) => s !== scope) : [...prev, scope]
    );
  }

  function openDetailModal(key: APIKeyInfo) {
    setDetailKey(key);
    setEditMode(false);
    setEditError('');
  }

  function startEdit() {
    if (!detailKey) return;
    setEditMode(true);
    setEditName(detailKey.name);
    setEditScopes(detailKey.scopes || []);
    setEditAllowedIps(detailKey.allowed_ips?.join('\n') || '');
    setEditExpiresAt(detailKey.expires_at || '');
    setEditError('');
  }

  function closeDetailModal() {
    setDetailKey(null);
    setEditMode(false);
    setEditError('');
  }

  async function handleEditSave(e: FormEvent) {
    e.preventDefault();
    if (!detailKey) return;
    setEditSaving(true);
    setEditError('');
    try {
      const allowedIps = editAllowedIps
        .split(/[,\n]/)
        .map((s) => s.trim())
        .filter(Boolean);

      const updated = await apiKeysApi.update(detailKey.id, {
        name: editName,
        scopes: editScopes,
        allowed_ips: allowedIps,
        expires_at: editExpiresAt || undefined,
      });
      setDetailKey(updated);
      setEditMode(false);
      await loadKeys();
    } catch (err) {
      setEditError(err instanceof ApiError ? err.message : 'Ошибка при сохранении');
    } finally {
      setEditSaving(false);
    }
  }

  const columns: Column<APIKeyInfo>[] = [
    { key: 'name', header: 'Название' },
    {
      key: 'prefix',
      header: 'Префикс',
      render: (key) => <code className="text-sm bg-gray-100 px-1.5 py-0.5 rounded">{key.prefix}...</code>,
    },
    {
      key: 'status',
      header: 'Статус',
      render: (key) => <StatusBadge status={key.active ? 'active' : 'inactive'} />,
    },
    {
      key: 'allowed_ips',
      header: 'Разрешённые IP',
      render: (key) =>
        key.allowed_ips && key.allowed_ips.length > 0 ? key.allowed_ips.join(', ') : 'Любые',
    },
    {
      key: 'scopes',
      header: 'Области доступа',
      render: (key) =>
        key.scopes && key.scopes.length > 0 ? key.scopes.join(', ') : 'Полный доступ',
    },
    {
      key: 'created_at',
      header: 'Создан',
      render: (key) => new Date(key.created_at).toLocaleDateString(),
    },
    {
      key: 'last_used_at',
      header: 'Последнее использование',
      render: (key) =>
        key.last_used_at ? new Date(key.last_used_at).toLocaleDateString() : 'Никогда',
    },
  ];

  if (loading) return <div role="status">Загрузка API ключей...</div>;

  return (
    <div className="max-w-[900px]">
      <PageHeader
        title="API Ключи"
        actions={
          <Button
            onClick={() => {
              setShowCreateForm(true);
              setCreatedKey(null);
            }}
          >
            Создать API ключ
          </Button>
        }
      />

      {error && <p className="text-red-600">{error}</p>}

      {/* Created key banner - shown once */}
      {createdKey && (
        <div role="alert" className="bg-green-50 border border-green-500 rounded p-4 mb-4">
          <p className="mb-2 font-bold">
            API ключ создан успешно. Скопируйте его сейчас — он не будет показан снова.
          </p>
          <div className="flex items-center gap-2">
            <code className="bg-white px-2 py-1 rounded break-all flex-1">
              {createdKey.api_key}
            </code>
            <Button variant="secondary" size="sm" onClick={handleCopyKey}>
              {copied ? 'Скопировано!' : 'Скопировать'}
            </Button>
          </div>
          <button
            onClick={() => setCreatedKey(null)}
            className="mt-2 text-sm text-gray-600 underline hover:text-gray-800 bg-transparent border-none cursor-pointer"
          >
            Скрыть
          </button>
        </div>
      )}

      {/* Create form modal */}
      <Modal open={showCreateForm} onClose={() => setShowCreateForm(false)} title="Создать API ключ">
        <form onSubmit={handleCreate}>
          <div className="mb-3">
            <Input
              label="Название *"
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              placeholder="Например: Production Key"
              className="w-full max-w-[300px]"
            />
          </div>

          <div className="mb-3">
            <label className="text-sm font-medium text-gray-700">
              Разрешённые IP (через запятую или по одному на строку, поддержка CIDR)
            </label>
            <textarea
              value={allowedIpsInput}
              onChange={(e) => setAllowedIpsInput(e.target.value)}
              placeholder="Например: 192.168.1.1, 10.0.0.0/8"
              rows={3}
              className="mt-1 w-full max-w-[400px] rounded border border-gray-300 px-3 py-2 text-sm transition-colors focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary"
            />
          </div>

          <div className="mb-3">
            <label className="text-sm font-medium text-gray-700">Области доступа</label>
            <div className="flex flex-wrap gap-2 mt-1">
              {AVAILABLE_SCOPES.map((scope) => (
                <label key={scope} className="flex items-center gap-1">
                  <input
                    type="checkbox"
                    checked={selectedScopes.includes(scope)}
                    onChange={() => toggleScope(scope)}
                  />
                  <span className="text-sm">{scope}</span>
                </label>
              ))}
            </div>
            <small className="text-gray-500">Оставьте пустым для полного доступа</small>
          </div>

          <div className="mb-3">
            <Input
              label="Срок действия (опционально)"
              type="datetime-local"
              value={expiresAt}
              onChange={(e) => {
                const val = e.target.value;
                setExpiresAt(val ? new Date(val).toISOString() : '');
              }}
            />
          </div>

          <div className="flex gap-2">
            <Button type="submit" disabled={creating}>
              {creating ? 'Создание...' : 'Создать'}
            </Button>
            <Button type="button" variant="secondary" onClick={() => setShowCreateForm(false)}>
              Отмена
            </Button>
          </div>
        </form>
      </Modal>

      {/* Revoke confirmation dialog */}
      <ConfirmDialog
        open={revokeId !== null}
        onConfirm={() => revokeId && handleRevoke(revokeId)}
        onCancel={() => setRevokeId(null)}
        title="Отозвать API ключ"
        description="Вы уверены, что хотите отозвать этот API ключ?"
        variant="danger"
        confirmLabel="Да, отозвать"
      />

      {/* Detail / Edit modal */}
      <Modal
        open={detailKey !== null}
        onClose={closeDetailModal}
        title={editMode ? 'Редактировать API ключ' : 'Детали API ключа'}
        wide
      >
        {detailKey && !editMode && (
          <div>
            <dl className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm mb-6">
              <div>
                <dt className="text-gray-500">Имя</dt>
                <dd className="font-medium text-gray-900">{detailKey.name}</dd>
              </div>
              <div>
                <dt className="text-gray-500">Префикс</dt>
                <dd className="font-medium text-gray-900">
                  <code className="bg-gray-100 px-1.5 py-0.5 rounded">{detailKey.prefix}...</code>
                </dd>
              </div>
              <div>
                <dt className="text-gray-500">Области доступа</dt>
                <dd className="font-medium text-gray-900">
                  {detailKey.scopes && detailKey.scopes.length > 0
                    ? detailKey.scopes.join(', ')
                    : 'Полный доступ'}
                </dd>
              </div>
              <div>
                <dt className="text-gray-500">Разрешенные IP</dt>
                <dd className="font-medium text-gray-900">
                  {detailKey.allowed_ips && detailKey.allowed_ips.length > 0
                    ? detailKey.allowed_ips.join(', ')
                    : 'Любые'}
                </dd>
              </div>
              <div>
                <dt className="text-gray-500">Истекает</dt>
                <dd className="font-medium text-gray-900">
                  {detailKey.expires_at
                    ? new Date(detailKey.expires_at).toLocaleString()
                    : 'Бессрочно'}
                </dd>
              </div>
              <div>
                <dt className="text-gray-500">Последнее использование</dt>
                <dd className="font-medium text-gray-900">
                  {detailKey.last_used_at
                    ? new Date(detailKey.last_used_at).toLocaleString()
                    : 'Никогда'}
                </dd>
              </div>
            </dl>
            <div className="flex gap-2">
              <Button onClick={startEdit}>Редактировать</Button>
              <Button variant="secondary" onClick={closeDetailModal}>Закрыть</Button>
            </div>
          </div>
        )}

        {detailKey && editMode && (
          <form onSubmit={handleEditSave}>
            {editError && (
              <p role="alert" className="text-red-600 mb-3">{editError}</p>
            )}

            <div className="mb-3">
              <Input
                label="Имя"
                type="text"
                value={editName}
                onChange={(e) => setEditName(e.target.value)}
                required
                className="w-full max-w-[300px]"
              />
            </div>

            <div className="mb-3">
              <label className="text-sm font-medium text-gray-700">Области доступа</label>
              <div className="flex flex-wrap gap-2 mt-1">
                {EDIT_SCOPES.map((scope) => (
                  <label key={scope} className="flex items-center gap-1">
                    <input
                      type="checkbox"
                      checked={editScopes.includes(scope)}
                      onChange={() => toggleEditScope(scope)}
                    />
                    <span className="text-sm">{scope}</span>
                  </label>
                ))}
              </div>
            </div>

            <div className="mb-3">
              <label className="text-sm font-medium text-gray-700">
                Разрешенные IP (по одному на строку или через запятую)
              </label>
              <textarea
                value={editAllowedIps}
                onChange={(e) => setEditAllowedIps(e.target.value)}
                placeholder="192.168.1.1&#10;10.0.0.0/8"
                rows={3}
                className="mt-1 w-full max-w-[400px] rounded border border-gray-300 px-3 py-2 text-sm transition-colors focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary"
              />
            </div>

            <div className="mb-4">
              <Input
                label="Срок действия"
                type="datetime-local"
                value={editExpiresAt ? editExpiresAt.slice(0, 16) : ''}
                onChange={(e) => {
                  const val = e.target.value;
                  setEditExpiresAt(val ? new Date(val).toISOString() : '');
                }}
              />
            </div>

            <div className="flex gap-2">
              <Button type="submit" disabled={editSaving}>
                {editSaving ? 'Сохранение...' : 'Сохранить'}
              </Button>
              <Button type="button" variant="secondary" onClick={() => setEditMode(false)}>
                Отмена
              </Button>
            </div>
          </form>
        )}
      </Modal>

      {/* Keys table */}
      <DataTable
        columns={columns}
        data={keys}
        total={keys.length}
        page={1}
        pageSize={keys.length || 1}
        onPageChange={() => {}}
        keyField="id"
        onRowClick={openDetailModal}
        rowActions={(key) =>
          key.active ? (
            <Button
              variant="danger"
              size="sm"
              aria-label={`Revoke ${key.name}`}
              onClick={() => setRevokeId(key.id)}
            >
              Отозвать
            </Button>
          ) : null
        }
      />
    </div>
  );
}
