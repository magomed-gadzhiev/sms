import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { providersApi, ApiError, type Provider } from '../../api/client';

const BIND_LABELS: Record<number, string> = { 0: 'TRX', 1: 'TX', 2: 'RX' };

export function ProvidersPage() {
  const navigate = useNavigate();
  const [providers, setProviders] = useState<Provider[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  async function load() {
    setLoading(true);
    setError('');
    try {
      const resp = await providersApi.list();
      setProviders(resp.providers ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to load providers');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, []);

  async function handleDelete(id: string) {
    if (!confirm('Delete this provider?')) return;
    try {
      await providersApi.remove(id);
      setProviders(prev => prev.filter(p => p.id !== id));
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'Failed to delete provider');
    }
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h2 style={{ margin: 0 }}>SMPP Providers</h2>
        <button
          onClick={() => navigate('/providers/new')}
          style={{ padding: '8px 20px', background: '#1976d2', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer' }}
        >
          + Add Provider
        </button>
      </div>

      {loading && <p>Loading...</p>}
      {error && <p style={{ color: '#d32f2f' }}>{error}</p>}

      {!loading && !error && providers.length === 0 && (
        <div style={{ textAlign: 'center', padding: 48, color: '#666' }}>
          <p>No providers yet.</p>
          <button onClick={() => navigate('/providers/new')} style={{ cursor: 'pointer' }}>
            Add your first provider
          </button>
        </div>
      )}

      {providers.length > 0 && (
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ borderBottom: '2px solid #e0e0e0', textAlign: 'left' }}>
              <th style={{ padding: '8px' }}>Name</th>
              <th style={{ padding: '8px' }}>Host</th>
              <th style={{ padding: '8px' }}>Bind</th>
              <th style={{ padding: '8px' }}>Conns</th>
              <th style={{ padding: '8px' }}>TPS</th>
              <th style={{ padding: '8px' }}>Status</th>
              <th style={{ padding: '8px' }}></th>
            </tr>
          </thead>
          <tbody>
            {providers.map(p => (
              <tr key={p.id} style={{ borderBottom: '1px solid #e0e0e0' }}>
                <td style={{ padding: '8px' }}>
                  <div>{p.name}</div>
                  {p.description && <div style={{ fontSize: 12, color: '#666' }}>{p.description}</div>}
                </td>
                <td style={{ padding: '8px', fontFamily: 'monospace' }}>{p.host}:{p.port}</td>
                <td style={{ padding: '8px' }}>{BIND_LABELS[p.bind_type] ?? '-'}</td>
                <td style={{ padding: '8px' }}>{p.max_connections}</td>
                <td style={{ padding: '8px' }}>{p.tps_limit}</td>
                <td style={{ padding: '8px' }}>
                  <span style={{ color: p.active ? '#2e7d32' : '#b71c1c', fontWeight: 'bold' }}>
                    {p.active ? 'Active' : 'Inactive'}
                  </span>
                </td>
                <td style={{ padding: '8px' }}>
                  <button
                    onClick={() => handleDelete(p.id)}
                    style={{ color: '#d32f2f', border: 'none', background: 'none', cursor: 'pointer' }}
                  >
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
