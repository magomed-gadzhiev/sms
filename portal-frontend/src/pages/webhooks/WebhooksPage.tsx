import { useState, useEffect, type FormEvent } from 'react';
import { webhooksApi, ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Badge, StatusBadge } from '../../components/ui/Badge';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';

const AVAILABLE_EVENT_TYPES = [
  'delivered',
  'failed',
  'expired',
  'rejected',
];

interface WebhookInfo {
  id: string;
  client_id: string;
  url: string;
  event_types: string[];
  active: boolean;
  created_at?: string;
  updated_at?: string;
}

interface WebhookListResponse {
  subscriptions: WebhookInfo[];
}

interface CreateWebhookResponse extends WebhookInfo {
  secret: string;
}

export function WebhooksPage() {
  const [webhooks, setWebhooks] = useState<WebhookInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Create form state
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [formUrl, setFormUrl] = useState('');
  const [formEventTypes, setFormEventTypes] = useState<string[]>([]);
  const [creating, setCreating] = useState(false);

  // Edit form state
  const [editId, setEditId] = useState<string | null>(null);
  const [editUrl, setEditUrl] = useState('');
  const [editEventTypes, setEditEventTypes] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);

  // Created webhook secret (shown once)
  const [createdSecret, setCreatedSecret] = useState<string | null>(null);
  const [secretCopied, setSecretCopied] = useState(false);

  // Delete confirmation
  const [deleteId, setDeleteId] = useState<string | null>(null);

  // Test result
  const [testResult, setTestResult] = useState<{ id: string; message: string } | null>(null);

  async function loadWebhooks() {
    setLoading(true);
    setError('');
    try {
      const resp = (await webhooksApi.list()) as WebhookListResponse;
      setWebhooks(resp.subscriptions || []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to load webhooks');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadWebhooks();
  }, []);

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    setCreating(true);
    setError('');
    try {
      const resp = (await webhooksApi.create({
        url: formUrl,
        event_types: formEventTypes,
      })) as CreateWebhookResponse;
      setCreatedSecret(resp.secret);
      setShowCreateForm(false);
      setFormUrl('');
      setFormEventTypes([]);
      await loadWebhooks();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to create webhook');
    } finally {
      setCreating(false);
    }
  }

  function startEdit(wh: WebhookInfo) {
    setEditId(wh.id);
    setEditUrl(wh.url);
    setEditEventTypes([...wh.event_types]);
  }

  async function handleUpdate(e: FormEvent) {
    e.preventDefault();
    if (!editId) return;
    setSaving(true);
    setError('');
    try {
      await webhooksApi.update(editId, {
        url: editUrl,
        event_types: editEventTypes,
      });
      setEditId(null);
      await loadWebhooks();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to update webhook');
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(id: string) {
    setError('');
    try {
      await webhooksApi.remove(id);
      setDeleteId(null);
      await loadWebhooks();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to delete webhook');
    }
  }

  async function handleTest(id: string) {
    setError('');
    setTestResult(null);
    try {
      const resp = (await webhooksApi.test(id)) as { message: string };
      setTestResult({ id, message: resp.message || 'Test event sent' });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to test webhook');
    }
  }

  function handleCopySecret() {
    if (createdSecret) {
      navigator.clipboard.writeText(createdSecret);
      setSecretCopied(true);
      setTimeout(() => setSecretCopied(false), 2000);
    }
  }

  function toggleEventType(list: string[], type: string): string[] {
    return list.includes(type) ? list.filter((t) => t !== type) : [...list, type];
  }

  const deleteWebhook = webhooks.find((wh) => wh.id === deleteId);

  const columns: Column<WebhookInfo>[] = [
    {
      key: 'url',
      header: 'URL',
      render: (wh) => <span className="max-w-[300px] break-all">{wh.url}</span>,
    },
    {
      key: 'event_types',
      header: 'Event Types',
      render: (wh) => (
        <div className="flex flex-wrap gap-1">
          {wh.event_types.map((t) => (
            <Badge key={t}>{t}</Badge>
          ))}
        </div>
      ),
    },
    {
      key: 'active',
      header: 'Status',
      render: (wh) => <StatusBadge status={wh.active ? 'active' : 'inactive'} />,
    },
    {
      key: 'created_at',
      header: 'Created',
      render: (wh) => (wh.created_at ? new Date(wh.created_at).toLocaleDateString() : '-'),
    },
  ];

  if (loading) return <div role="status">Loading webhooks...</div>;

  return (
    <div className="max-w-[900px]">
      <PageHeader
        title="Webhooks"
        actions={
          <Button
            onClick={() => {
              setShowCreateForm(true);
              setCreatedSecret(null);
            }}
          >
            Create Webhook
          </Button>
        }
      />

      {error && <p className="text-red-600">{error}</p>}

      {/* Secret banner - shown once after creation */}
      {createdSecret && (
        <div
          role="alert"
          className="bg-green-50 border border-green-500 rounded p-4 mb-4"
        >
          <p className="mb-2 font-bold">
            Webhook created. Copy the secret now -- it will not be shown again.
          </p>
          <div className="flex items-center gap-2">
            <code className="bg-white px-2 py-1 rounded break-all flex-1">
              {createdSecret}
            </code>
            <Button size="sm" variant="secondary" onClick={handleCopySecret}>
              {secretCopied ? 'Copied!' : 'Copy'}
            </Button>
          </div>
          <button
            onClick={() => setCreatedSecret(null)}
            className="mt-2 bg-transparent border-none cursor-pointer underline text-sm text-gray-600"
          >
            Dismiss
          </button>
        </div>
      )}

      {/* Test result banner */}
      {testResult && (
        <div
          role="status"
          className="bg-blue-50 border border-blue-500 rounded p-3 mb-4 flex items-center justify-between"
        >
          <span>{testResult.message}</span>
          <button
            onClick={() => setTestResult(null)}
            className="ml-3 bg-transparent border-none cursor-pointer underline text-sm text-gray-600"
          >
            Dismiss
          </button>
        </div>
      )}

      {/* Create form modal */}
      <Modal open={showCreateForm} onClose={() => setShowCreateForm(false)} title="Create Webhook">
        <form onSubmit={handleCreate}>
          <div className="mb-4">
            <Input
              label="URL *"
              type="url"
              value={formUrl}
              onChange={(e) => setFormUrl(e.target.value)}
              required
              placeholder="https://example.com/webhook"
              className="w-full max-w-[400px]"
            />
          </div>
          <div className="mb-4">
            <label className="text-sm font-medium text-gray-700">Event Types *</label>
            <div className="flex flex-wrap gap-2 mt-1">
              {AVAILABLE_EVENT_TYPES.map((type) => (
                <label key={type} className="flex items-center gap-1 text-sm">
                  <input
                    type="checkbox"
                    checked={formEventTypes.includes(type)}
                    onChange={() => setFormEventTypes(toggleEventType(formEventTypes, type))}
                  />
                  {type}
                </label>
              ))}
            </div>
          </div>
          <div className="flex gap-2 justify-end">
            <Button variant="secondary" type="button" onClick={() => setShowCreateForm(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={creating || formEventTypes.length === 0}>
              {creating ? 'Creating...' : 'Create'}
            </Button>
          </div>
        </form>
      </Modal>

      {/* Edit form modal */}
      <Modal open={!!editId} onClose={() => setEditId(null)} title="Edit Webhook">
        <form onSubmit={handleUpdate}>
          <div className="mb-4">
            <Input
              label="URL"
              type="url"
              value={editUrl}
              onChange={(e) => setEditUrl(e.target.value)}
              required
              className="w-full max-w-[400px]"
            />
          </div>
          <div className="mb-4">
            <label className="text-sm font-medium text-gray-700">Event Types</label>
            <div className="flex flex-wrap gap-2 mt-1">
              {AVAILABLE_EVENT_TYPES.map((type) => (
                <label key={type} className="flex items-center gap-1 text-sm">
                  <input
                    type="checkbox"
                    checked={editEventTypes.includes(type)}
                    onChange={() => setEditEventTypes(toggleEventType(editEventTypes, type))}
                  />
                  {type}
                </label>
              ))}
            </div>
          </div>
          <div className="flex gap-2 justify-end">
            <Button variant="secondary" type="button" onClick={() => setEditId(null)}>
              Cancel
            </Button>
            <Button type="submit" disabled={saving || editEventTypes.length === 0}>
              {saving ? 'Saving...' : 'Save'}
            </Button>
          </div>
        </form>
      </Modal>

      {/* Delete confirmation */}
      <ConfirmDialog
        open={!!deleteId}
        onCancel={() => setDeleteId(null)}
        onConfirm={() => deleteId && handleDelete(deleteId)}
        title="Delete Webhook"
        description={`Are you sure you want to delete the webhook${deleteWebhook ? ` for ${deleteWebhook.url}` : ''}? This action cannot be undone.`}
        confirmLabel="Delete"
        variant="danger"
      />

      {/* Webhooks table */}
      <DataTable<WebhookInfo>
        columns={columns}
        data={webhooks}
        total={webhooks.length}
        page={1}
        pageSize={webhooks.length || 10}
        onPageChange={() => {}}
        keyField="id"
        rowActions={(wh) => (
          <div className="flex gap-1">
            <Button
              variant="ghost"
              size="sm"
              aria-label={`Edit ${wh.url}`}
              onClick={() => startEdit(wh)}
            >
              Edit
            </Button>
            <Button
              variant="ghost"
              size="sm"
              aria-label={`Test ${wh.url}`}
              onClick={() => handleTest(wh.id)}
            >
              Test
            </Button>
            <Button
              variant="ghost"
              size="sm"
              aria-label={`Delete ${wh.url}`}
              onClick={() => setDeleteId(wh.id)}
            >
              Delete
            </Button>
          </div>
        )}
      />
    </div>
  );
}
