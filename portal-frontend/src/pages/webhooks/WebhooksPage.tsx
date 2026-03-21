import { useState, useEffect, type FormEvent } from 'react';
import { webhooksApi, ApiError } from '../../api/client';

const AVAILABLE_EVENT_TYPES = [
  'message.sent',
  'message.delivered',
  'message.failed',
  'message.expired',
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

  if (loading) return <div>Loading webhooks...</div>;

  return (
    <div style={{ maxWidth: 900 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2>Webhooks</h2>
        <button
          onClick={() => {
            setShowCreateForm(true);
            setCreatedSecret(null);
          }}
        >
          Create Webhook
        </button>
      </div>

      {error && <p style={{ color: 'red' }}>{error}</p>}

      {/* Secret banner - shown once after creation */}
      {createdSecret && (
        <div
          style={{
            background: '#e8f5e9',
            border: '1px solid #4caf50',
            borderRadius: 4,
            padding: 16,
            marginBottom: 16,
          }}
        >
          <p style={{ margin: '0 0 8px', fontWeight: 'bold' }}>
            Webhook created. Copy the secret now -- it will not be shown again.
          </p>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <code
              style={{
                background: '#fff',
                padding: '4px 8px',
                borderRadius: 4,
                wordBreak: 'break-all',
                flex: 1,
              }}
            >
              {createdSecret}
            </code>
            <button onClick={handleCopySecret}>{secretCopied ? 'Copied!' : 'Copy'}</button>
          </div>
          <button
            onClick={() => setCreatedSecret(null)}
            style={{
              marginTop: 8,
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              textDecoration: 'underline',
            }}
          >
            Dismiss
          </button>
        </div>
      )}

      {/* Test result banner */}
      {testResult && (
        <div
          style={{
            background: '#e3f2fd',
            border: '1px solid #2196f3',
            borderRadius: 4,
            padding: 12,
            marginBottom: 16,
          }}
        >
          <span>{testResult.message}</span>
          <button
            onClick={() => setTestResult(null)}
            style={{
              marginLeft: 12,
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              textDecoration: 'underline',
            }}
          >
            Dismiss
          </button>
        </div>
      )}

      {/* Create form */}
      {showCreateForm && (
        <div
          style={{
            background: '#f5f5f5',
            border: '1px solid #ddd',
            borderRadius: 4,
            padding: 16,
            marginBottom: 16,
          }}
        >
          <h3 style={{ marginTop: 0 }}>Create Webhook</h3>
          <form onSubmit={handleCreate}>
            <div style={{ marginBottom: 12 }}>
              <label>
                URL *
                <br />
                <input
                  type="url"
                  value={formUrl}
                  onChange={(e) => setFormUrl(e.target.value)}
                  required
                  placeholder="https://example.com/webhook"
                  style={{ width: '100%', maxWidth: 400 }}
                />
              </label>
            </div>
            <div style={{ marginBottom: 12 }}>
              <label>Event Types *</label>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginTop: 4 }}>
                {AVAILABLE_EVENT_TYPES.map((type) => (
                  <label key={type} style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
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
            <div style={{ display: 'flex', gap: 8 }}>
              <button type="submit" disabled={creating || formEventTypes.length === 0}>
                {creating ? 'Creating...' : 'Create'}
              </button>
              <button type="button" onClick={() => setShowCreateForm(false)}>
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Edit form */}
      {editId && (
        <div
          style={{
            background: '#fff8e1',
            border: '1px solid #ffc107',
            borderRadius: 4,
            padding: 16,
            marginBottom: 16,
          }}
        >
          <h3 style={{ marginTop: 0 }}>Edit Webhook</h3>
          <form onSubmit={handleUpdate}>
            <div style={{ marginBottom: 12 }}>
              <label>
                URL
                <br />
                <input
                  type="url"
                  value={editUrl}
                  onChange={(e) => setEditUrl(e.target.value)}
                  required
                  style={{ width: '100%', maxWidth: 400 }}
                />
              </label>
            </div>
            <div style={{ marginBottom: 12 }}>
              <label>Event Types</label>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginTop: 4 }}>
                {AVAILABLE_EVENT_TYPES.map((type) => (
                  <label key={type} style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
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
            <div style={{ display: 'flex', gap: 8 }}>
              <button type="submit" disabled={saving || editEventTypes.length === 0}>
                {saving ? 'Saving...' : 'Save'}
              </button>
              <button type="button" onClick={() => setEditId(null)}>
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Webhooks table */}
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr style={{ borderBottom: '2px solid #ddd', textAlign: 'left' }}>
            <th style={{ padding: 8 }}>URL</th>
            <th style={{ padding: 8 }}>Event Types</th>
            <th style={{ padding: 8 }}>Status</th>
            <th style={{ padding: 8 }}>Created</th>
            <th style={{ padding: 8 }}>Actions</th>
          </tr>
        </thead>
        <tbody>
          {webhooks.length === 0 && (
            <tr>
              <td colSpan={5} style={{ padding: 16, textAlign: 'center', color: '#999' }}>
                No webhooks yet. Create one to get started.
              </td>
            </tr>
          )}
          {webhooks.map((wh) => (
            <tr key={wh.id} style={{ borderBottom: '1px solid #eee' }}>
              <td style={{ padding: 8, maxWidth: 300, wordBreak: 'break-all' }}>{wh.url}</td>
              <td style={{ padding: 8 }}>
                {wh.event_types.map((t) => (
                  <span
                    key={t}
                    style={{
                      display: 'inline-block',
                      background: '#e0e0e0',
                      borderRadius: 4,
                      padding: '2px 6px',
                      marginRight: 4,
                      marginBottom: 2,
                      fontSize: 12,
                    }}
                  >
                    {t}
                  </span>
                ))}
              </td>
              <td style={{ padding: 8 }}>
                <span
                  style={{
                    color: wh.active ? '#4caf50' : '#f44336',
                    fontWeight: 'bold',
                  }}
                >
                  {wh.active ? 'Active' : 'Inactive'}
                </span>
              </td>
              <td style={{ padding: 8 }}>
                {wh.created_at ? new Date(wh.created_at).toLocaleDateString() : '-'}
              </td>
              <td style={{ padding: 8 }}>
                <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
                  <button onClick={() => startEdit(wh)}>Edit</button>
                  <button onClick={() => handleTest(wh.id)}>Test</button>
                  {deleteId === wh.id ? (
                    <span>
                      Sure?{' '}
                      <button
                        onClick={() => handleDelete(wh.id)}
                        style={{ color: 'red', marginRight: 4 }}
                      >
                        Yes
                      </button>
                      <button onClick={() => setDeleteId(null)}>No</button>
                    </span>
                  ) : (
                    <button onClick={() => setDeleteId(wh.id)}>Delete</button>
                  )}
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
