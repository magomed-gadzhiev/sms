import { useState, useEffect, useRef, type FormEvent } from 'react';
import { apiKeysApi, ApiError, type APIKeyInfo, type CreateAPIKeyResponse } from '../../api/client';
import { useFocusTrap } from '../../hooks/useFocusTrap';

const AVAILABLE_SCOPES = [
  'messages:send',
  'messages:read',
  'webhooks:manage',
  'analytics:read',
  'sub-accounts:manage',
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

  // Focus trap for create form
  const createFormRef = useRef<HTMLDivElement>(null);
  useFocusTrap(createFormRef, showCreateForm);

  async function loadKeys() {
    setLoading(true);
    setError('');
    try {
      const resp = await apiKeysApi.list();
      setKeys(resp.keys || []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to load API keys');
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
      setError(err instanceof ApiError ? err.message : 'Failed to create API key');
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
      setError(err instanceof ApiError ? err.message : 'Failed to revoke API key');
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

  if (loading) return <div role="status">Loading API keys...</div>;

  return (
    <div style={{ maxWidth: 900 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2>API Keys</h2>
        <button
          onClick={() => {
            setShowCreateForm(true);
            setCreatedKey(null);
          }}
        >
          Create API Key
        </button>
      </div>

      {error && <p style={{ color: '#d32f2f' }}>{error}</p>}

      {/* Created key banner - shown once */}
      {createdKey && (
        <div
          role="alert"
          style={{
            background: '#e8f5e9',
            border: '1px solid #4caf50',
            borderRadius: 4,
            padding: 16,
            marginBottom: 16,
          }}
        >
          <p style={{ margin: '0 0 8px', fontWeight: 'bold' }}>
            API Key created successfully. Copy it now -- it will not be shown again.
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
              {createdKey.api_key}
            </code>
            <button onClick={handleCopyKey}>{copied ? 'Copied!' : 'Copy'}</button>
          </div>
          <button
            onClick={() => setCreatedKey(null)}
            style={{ marginTop: 8, background: 'none', border: 'none', cursor: 'pointer', textDecoration: 'underline' }}
          >
            Dismiss
          </button>
        </div>
      )}

      {/* Create form dialog */}
      {showCreateForm && (
        <div
          ref={createFormRef}
          style={{
            background: '#f5f5f5',
            border: '1px solid #ddd',
            borderRadius: 4,
            padding: 16,
            marginBottom: 16,
          }}
        >
          <h3 style={{ marginTop: 0 }}>Create New API Key</h3>
          <form onSubmit={handleCreate}>
            <div style={{ marginBottom: 12 }}>
              <label>
                Name *
                <br />
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required
                  placeholder="e.g. Production Key"
                  style={{ width: '100%', maxWidth: 300 }}
                />
              </label>
            </div>

            <div style={{ marginBottom: 12 }}>
              <label>
                Allowed IPs (comma or newline separated, supports CIDR)
                <br />
                <textarea
                  value={allowedIpsInput}
                  onChange={(e) => setAllowedIpsInput(e.target.value)}
                  placeholder="e.g. 192.168.1.1, 10.0.0.0/8"
                  rows={3}
                  style={{ width: '100%', maxWidth: 400 }}
                />
              </label>
            </div>

            <div style={{ marginBottom: 12 }}>
              <label>Scopes</label>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginTop: 4 }}>
                {AVAILABLE_SCOPES.map((scope) => (
                  <label key={scope} style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                    <input
                      type="checkbox"
                      checked={selectedScopes.includes(scope)}
                      onChange={() => toggleScope(scope)}
                    />
                    {scope}
                  </label>
                ))}
              </div>
              <small style={{ color: '#666' }}>Leave empty for full access</small>
            </div>

            <div style={{ marginBottom: 12 }}>
              <label>
                Expires At (optional)
                <br />
                <input
                  type="datetime-local"
                  value={expiresAt}
                  onChange={(e) => {
                    const val = e.target.value;
                    setExpiresAt(val ? new Date(val).toISOString() : '');
                  }}
                />
              </label>
            </div>

            <div style={{ display: 'flex', gap: 8 }}>
              <button type="submit" disabled={creating}>
                {creating ? 'Creating...' : 'Create'}
              </button>
              <button type="button" onClick={() => setShowCreateForm(false)}>
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Keys table */}
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <caption style={{ textAlign: 'left', marginBottom: 8, fontWeight: 'bold' }}>API Keys</caption>
        <thead>
          <tr style={{ borderBottom: '2px solid #ddd', textAlign: 'left' }}>
            <th style={{ padding: 8 }}>Name</th>
            <th style={{ padding: 8 }}>Prefix</th>
            <th style={{ padding: 8 }}>Status</th>
            <th style={{ padding: 8 }}>Allowed IPs</th>
            <th style={{ padding: 8 }}>Scopes</th>
            <th style={{ padding: 8 }}>Created</th>
            <th style={{ padding: 8 }}>Last Used</th>
            <th style={{ padding: 8 }}>Actions</th>
          </tr>
        </thead>
        <tbody>
          {keys.length === 0 && (
            <tr>
              <td colSpan={8} style={{ padding: 16, textAlign: 'center', color: '#767676' }}>
                No API keys yet. Create one to get started.
              </td>
            </tr>
          )}
          {keys.map((key) => (
            <tr key={key.id} style={{ borderBottom: '1px solid #eee' }}>
              <td style={{ padding: 8 }}>{key.name}</td>
              <td style={{ padding: 8 }}>
                <code>{key.prefix}...</code>
              </td>
              <td style={{ padding: 8 }}>
                <span
                  aria-label={`Status: ${key.active ? 'Active' : 'Revoked'}`}
                  style={{
                    color: key.active ? '#4caf50' : '#d32f2f',
                    fontWeight: 'bold',
                  }}
                >
                  {key.active ? 'Active' : 'Revoked'}
                </span>
              </td>
              <td style={{ padding: 8 }}>
                {key.allowed_ips && key.allowed_ips.length > 0
                  ? key.allowed_ips.join(', ')
                  : 'Any'}
              </td>
              <td style={{ padding: 8 }}>
                {key.scopes && key.scopes.length > 0 ? key.scopes.join(', ') : 'Full access'}
              </td>
              <td style={{ padding: 8 }}>{new Date(key.created_at).toLocaleDateString()}</td>
              <td style={{ padding: 8 }}>
                {key.last_used_at ? new Date(key.last_used_at).toLocaleDateString() : 'Never'}
              </td>
              <td style={{ padding: 8 }}>
                {key.active &&
                  (revokeId === key.id ? (
                    <span>
                      Sure?{' '}
                      <button
                        aria-label={`Confirm revoke ${key.name}`}
                        onClick={() => handleRevoke(key.id)}
                        style={{ color: '#d32f2f', marginRight: 4, padding: '8px 12px' }}
                      >
                        Yes, revoke
                      </button>
                      <button onClick={() => setRevokeId(null)} style={{ padding: '8px 12px' }}>Cancel</button>
                    </span>
                  ) : (
                    <button
                      aria-label={`Revoke ${key.name}`}
                      onClick={() => setRevokeId(key.id)}
                      style={{ padding: '8px 12px' }}
                    >
                      Revoke
                    </button>
                  ))}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
