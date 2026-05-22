import { useState, useEffect, useMemo, useCallback, type FormEvent } from 'react';
import { apiKeysApi, ApiError, type APIKeyInfo, type CreateAPIKeyResponse, type RotateAPIKeyResponse } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { usePageTitle } from '../../contexts/PageTitleContext';
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
  usePageTitle('API Ключи');
  const [keys, setKeys] = useState<APIKeyInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Create form state
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [name, setName] = useState('');
  const [nameError, setNameError] = useState('');
  const [allowedIpsInput, setAllowedIpsInput] = useState('');
  const [selectedScopes, setSelectedScopes] = useState<string[]>([]);
  const [expiresAt, setExpiresAt] = useState('');
  const [creating, setCreating] = useState(false);

  // Created key (shown once)
  const [createdKey, setCreatedKey] = useState<CreateAPIKeyResponse | null>(null);
  const [copied, setCopied] = useState(false);

  // Revoke confirmation
  const [revokeId, setRevokeId] = useState<string | null>(null);

  // Rotate state
  const [rotateId, setRotateId] = useState<string | null>(null);
  const [rotating, setRotating] = useState(false);
  const [rotatedKey, setRotatedKey] = useState<RotateAPIKeyResponse | null>(null);
  const [rotateCopied, setRotateCopied] = useState(false);

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
    setError('');
    setNameError('');

    if (name.length > 100) {
      setNameError('Имя не должно превышать 100 символов');
      return;
    }

    setCreating(true);
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
      if (err instanceof ApiError && err.status === 400) {
        setNameError(err.message);
      } else {
        setError(err instanceof ApiError ? err.message : 'Не удалось создать API ключ');
      }
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

  async function handleRotate(id: string) {
    setError('');
    setRotating(true);
    try {
      const resp = await apiKeysApi.rotate(id);
      setRotateId(null);
      setRotatedKey(resp);
      await loadKeys();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось ротировать API ключ');
      setRotateId(null);
    } finally {
      setRotating(false);
    }
  }

  function handleCopyRotatedKey() {
    if (rotatedKey) {
      navigator.clipboard.writeText(rotatedKey.api_key);
      setRotateCopied(true);
      setTimeout(() => setRotateCopied(false), 2000);
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

  const handleOpenDetail = useCallback((key: APIKeyInfo) => openDetailModal(key), []);
  const handleRevokeClick = useCallback((key: APIKeyInfo) => setRevokeId(key.id), []);
  const handleRotateClick = useCallback((key: APIKeyInfo) => setRotateId(key.id), []);

  const columns = useMemo<Column<APIKeyInfo>[]>(() => [
    { key: 'name', header: 'Название' },
    {
      key: 'prefix',
      header: 'Префикс',
      render: (key) => <code className="text-sm bg-gray-100 dark:bg-slate-800 px-1.5 py-0.5 rounded">{key.prefix}...</code>,
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
  ], []);

  if (loading) return <div role="status">Загрузка API ключей...</div>;

  return (
    <div>
      <PageHeader
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

      {error && <p className="text-red-600 dark:text-red-400">{error}</p>}

      {/* Created key banner - shown once */}
      {createdKey && (
        <div role="alert" className="bg-green-50 dark:bg-green-950/40 border border-green-500 rounded p-4 mb-4">
          <p className="mb-2 font-bold">
            API ключ создан успешно. Скопируйте его сейчас — он не будет показан снова.
          </p>
          <div className="flex items-center gap-2">
            <code className="bg-white dark:bg-slate-900 px-2 py-1 rounded break-all flex-1">
              {createdKey.api_key}
            </code>
            <Button variant="secondary" size="sm" onClick={handleCopyKey}>
              {copied ? 'Скопировано!' : 'Скопировать'}
            </Button>
          </div>
          <button
            onClick={() => setCreatedKey(null)}
            className="mt-2 text-sm text-gray-600 dark:text-slate-400 underline hover:text-gray-800 bg-transparent border-none cursor-pointer"
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
              maxLength={100}
              error={nameError || undefined}
            />
          </div>

          <div className="mb-3">
            <label htmlFor="api-key-allowed-ips" className="text-sm font-medium text-gray-700 dark:text-slate-300">
              Разрешённые IP (через запятую или по одному на строку, поддержка CIDR)
            </label>
            <textarea
              id="api-key-allowed-ips"
              value={allowedIpsInput}
              onChange={(e) => setAllowedIpsInput(e.target.value)}
              placeholder="Например: 192.168.1.1, 10.0.0.0/8"
              rows={3}
              className="mt-1 w-full max-w-[400px] rounded border border-gray-300 dark:border-slate-600 px-3 py-2 text-sm transition-colors focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary"
            />
          </div>

          <div className="mb-3">
            <fieldset>
              <legend className="text-sm font-medium text-gray-700 dark:text-slate-300">Области доступа</legend>
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
            </fieldset>
            <small className="text-gray-500 dark:text-slate-400">Оставьте пустым для полного доступа</small>
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

      {/* Rotate confirmation dialog */}
      <ConfirmDialog
        open={rotateId !== null}
        onConfirm={() => rotateId && handleRotate(rotateId)}
        onCancel={() => setRotateId(null)}
        title="Ротировать API ключ"
        description="Будет создан новый API ключ с теми же scope'ами и настройками. Старый ключ продолжит работать в течение 24 часов, после чего перестанет действовать. Подмените ключ во всех ваших интеграциях."
        confirmLabel={rotating ? 'Ротация...' : 'Да, ротировать'}
      />

      {/* Rotated key shown once */}
      <Modal
        open={rotatedKey !== null}
        onClose={() => setRotatedKey(null)}
        title="Новый API ключ создан"
      >
        {rotatedKey && (
          <div className="space-y-4">
            <p className="text-sm text-gray-700 dark:text-slate-300">
              Скопируйте новый ключ сейчас. Он показывается только один раз. Старый ключ продолжит работать ещё 24 часа для безопасной подмены в интеграциях.
            </p>
            <div className="bg-gray-100 dark:bg-slate-800 p-3 rounded font-mono text-sm break-all">
              {rotatedKey.api_key}
            </div>
            <div className="flex gap-2">
              <Button onClick={handleCopyRotatedKey}>
                {rotateCopied ? 'Скопировано' : 'Скопировать'}
              </Button>
              <Button variant="secondary" onClick={() => setRotatedKey(null)}>
                Готово
              </Button>
            </div>
            {rotatedKey.old_key_revoke_at && (
              <p className="text-xs text-gray-500 dark:text-slate-400">
                Старый ключ перестанет работать: {new Date(rotatedKey.old_key_revoke_at).toLocaleString('ru-RU')}
              </p>
            )}
          </div>
        )}
      </Modal>

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
                <dt className="text-gray-500 dark:text-slate-400">Имя</dt>
                <dd className="font-medium text-gray-900 dark:text-slate-100">{detailKey.name}</dd>
              </div>
              <div>
                <dt className="text-gray-500 dark:text-slate-400">Префикс</dt>
                <dd className="font-medium text-gray-900 dark:text-slate-100">
                  <code className="bg-gray-100 dark:bg-slate-800 px-1.5 py-0.5 rounded">{detailKey.prefix}...</code>
                </dd>
              </div>
              <div>
                <dt className="text-gray-500 dark:text-slate-400">Области доступа</dt>
                <dd className="font-medium text-gray-900 dark:text-slate-100">
                  {detailKey.scopes && detailKey.scopes.length > 0
                    ? detailKey.scopes.join(', ')
                    : 'Полный доступ'}
                </dd>
              </div>
              <div>
                <dt className="text-gray-500 dark:text-slate-400">Разрешенные IP</dt>
                <dd className="font-medium text-gray-900 dark:text-slate-100">
                  {detailKey.allowed_ips && detailKey.allowed_ips.length > 0
                    ? detailKey.allowed_ips.join(', ')
                    : 'Любые'}
                </dd>
              </div>
              <div>
                <dt className="text-gray-500 dark:text-slate-400">Истекает</dt>
                <dd className="font-medium text-gray-900 dark:text-slate-100">
                  {detailKey.expires_at
                    ? new Date(detailKey.expires_at).toLocaleString()
                    : 'Бессрочно'}
                </dd>
              </div>
              <div>
                <dt className="text-gray-500 dark:text-slate-400">Последнее использование</dt>
                <dd className="font-medium text-gray-900 dark:text-slate-100">
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
              <p role="alert" className="text-red-600 dark:text-red-400 mb-3">{editError}</p>
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
              <fieldset>
                <legend className="text-sm font-medium text-gray-700 dark:text-slate-300">Области доступа</legend>
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
              </fieldset>
            </div>

            <div className="mb-3">
              <label htmlFor="edit-api-key-allowed-ips" className="text-sm font-medium text-gray-700 dark:text-slate-300">
                Разрешенные IP (по одному на строку или через запятую)
              </label>
              <textarea
                id="edit-api-key-allowed-ips"
                value={editAllowedIps}
                onChange={(e) => setEditAllowedIps(e.target.value)}
                placeholder="192.168.1.1&#10;10.0.0.0/8"
                rows={3}
                className="mt-1 w-full max-w-[400px] rounded border border-gray-300 dark:border-slate-600 px-3 py-2 text-sm transition-colors focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary"
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
        tableLabel="Список API ключей"
        onRowClick={handleOpenDetail}
        rowActions={(key) =>
          key.active ? (
            <div className="flex gap-2">
              <Button
                variant="secondary"
                size="sm"
                aria-label={`Ротировать API ключ ${key.name}`}
                onClick={() => handleRotateClick(key)}
                disabled={!!key.revoke_at}
                title={key.revoke_at ? 'Ключ уже в процессе ротации' : 'Сгенерировать новый ключ; старый перестанет работать через 24 часа'}
              >
                Ротировать
              </Button>
              <Button
                variant="danger"
                size="sm"
                aria-label={`Отозвать API ключ ${key.name}`}
                onClick={() => handleRevokeClick(key)}
              >
                Отозвать
              </Button>
            </div>
          ) : null
        }
      />
    </div>
  );
}
