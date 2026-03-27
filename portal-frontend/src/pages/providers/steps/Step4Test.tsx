import { useState } from 'react';
import { providersApi, ApiError, type CreateProviderRequest, type TestConnectionResult } from '../../../api/client';

interface Props {
  data: Partial<CreateProviderRequest>;
}

export function Step4Test({ data }: Props) {
  const [result, setResult] = useState<TestConnectionResult | null>(null);
  const [testing, setTesting] = useState(false);
  const [error, setError] = useState('');

  async function runTest() {
    if (!data.host || !data.port || !data.system_id || !data.password) {
      setError('Fill in connection details first (Step 2)');
      return;
    }
    setTesting(true);
    setError('');
    setResult(null);
    try {
      const res = await providersApi.testConnection({
        host: data.host,
        port: data.port,
        system_id: data.system_id,
        password: data.password,
        bind_type: data.bind_type ?? 0,
      });
      setResult(res);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Test failed');
    } finally {
      setTesting(false);
    }
  }

  return (
    <div>
      <h3>Test Connection</h3>
      <p style={{ color: '#666' }}>
        Verify your SMPP credentials by performing a live bind/unbind test.
      </p>
      <button
        type="button"
        onClick={runTest}
        disabled={testing}
        style={{
          padding: '8px 20px',
          background: '#1976d2',
          color: '#fff',
          border: 'none',
          borderRadius: 4,
          cursor: testing ? 'not-allowed' : 'pointer',
        }}
      >
        {testing ? 'Testing...' : 'Test Connection'}
      </button>

      {error && <p style={{ color: '#d32f2f', marginTop: 12 }}>{error}</p>}

      {result && (
        <div style={{ marginTop: 16, padding: 12, background: result.success ? '#e8f5e9' : '#ffebee', borderRadius: 4 }}>
          <div style={{ fontWeight: 'bold', color: result.success ? '#2e7d32' : '#c62828', marginBottom: 8 }}>
            {result.success ? `✓ Connected (${result.latency_ms}ms)` : `✗ Failed`}
          </div>
          <pre style={{ fontSize: 12, margin: 0, whiteSpace: 'pre-wrap' }}>
            {(result.log ?? []).join('\n')}
            {result.error ? `\nError: ${result.error}` : ''}
          </pre>
        </div>
      )}
    </div>
  );
}
