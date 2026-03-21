import { useEffect, useState, useCallback } from 'react';
import { auditApi } from '../../api/client';

interface AuditEntry {
  id: string;
  tenant_id: string;
  user_id: string;
  action: string;
  resource_type: string;
  resource_id: string;
  details: string;
  ip_address: string;
  created_at?: string;
}

interface AuditLogResponse {
  entries: AuditEntry[];
  total: number;
  page: number;
  total_pages: number;
}

const ACTION_OPTIONS = [
  { value: '', label: 'All actions' },
  { value: 'auth.login', label: 'Login' },
  { value: 'auth.logout', label: 'Logout' },
  { value: 'auth.password_reset', label: 'Password Reset' },
  { value: 'auth.totp_enabled', label: 'TOTP Enabled' },
  { value: 'auth.totp_disabled', label: 'TOTP Disabled' },
  { value: 'api_key.created', label: 'API Key Created' },
  { value: 'api_key.revoked', label: 'API Key Revoked' },
  { value: 'webhook.created', label: 'Webhook Created' },
  { value: 'webhook.updated', label: 'Webhook Updated' },
  { value: 'webhook.deleted', label: 'Webhook Deleted' },
  { value: 'webhook.test_sent', label: 'Webhook Test Sent' },
  { value: 'sub_account.created', label: 'Sub-account Created' },
  { value: 'sub_account.deleted', label: 'Sub-account Deleted' },
  { value: 'sub_account.limit_updated', label: 'Sub-account Limit Updated' },
  { value: 'balance.transfer_out', label: 'Balance Transfer Out' },
  { value: 'balance.transfer_in', label: 'Balance Transfer In' },
  { value: 'profile.updated', label: 'Profile Updated' },
];

export function AuditLogPage() {
  const [data, setData] = useState<AuditLogResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const [page, setPage] = useState(1);
  const [action, setAction] = useState('');
  const [userId, setUserId] = useState('');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');

  const fetchAuditLog = useCallback(() => {
    setLoading(true);
    setError(null);

    const params: Record<string, string> = {
      page: String(page),
      per_page: '20',
    };
    if (action) params.action = action;
    if (userId) params.user_id = userId;
    if (dateFrom) params.date_from = dateFrom;
    if (dateTo) params.date_to = dateTo;

    auditApi
      .list(params)
      .then((resp) => setData(resp as AuditLogResponse))
      .catch((err) => setError(err.message || 'Failed to load audit log'))
      .finally(() => setLoading(false));
  }, [page, action, userId, dateFrom, dateTo]);

  useEffect(() => {
    fetchAuditLog();
  }, [fetchAuditLog]);

  const handleFilter = (e: React.FormEvent) => {
    e.preventDefault();
    setPage(1);
    fetchAuditLog();
  };

  const formatDetails = (details: string): string => {
    if (!details || details === '{}' || details === 'null') return '-';
    try {
      const parsed = JSON.parse(details);
      return Object.entries(parsed)
        .map(([k, v]) => `${k}: ${v}`)
        .join(', ');
    } catch {
      return details;
    }
  };

  return (
    <div>
      <h2>Audit Log</h2>

      <form onSubmit={handleFilter} style={{ marginBottom: 16, display: 'flex', gap: 12, flexWrap: 'wrap', alignItems: 'flex-end' }}>
        <label>
          Action
          <br />
          <select value={action} onChange={(e) => setAction(e.target.value)} style={{ padding: 4 }}>
            {ACTION_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </label>

        <label>
          From
          <br />
          <input type="date" value={dateFrom} onChange={(e) => setDateFrom(e.target.value)} />
        </label>

        <label>
          To
          <br />
          <input type="date" value={dateTo} onChange={(e) => setDateTo(e.target.value)} />
        </label>

        <label>
          User ID
          <br />
          <input
            type="text"
            value={userId}
            onChange={(e) => setUserId(e.target.value)}
            placeholder="Filter by user..."
            style={{ padding: 4 }}
          />
        </label>

        <button type="submit" style={{ padding: '4px 12px' }}>
          Filter
        </button>
      </form>

      {error && <div style={{ color: 'red', marginBottom: 12 }}>Error: {error}</div>}

      {loading ? (
        <div>Loading...</div>
      ) : data ? (
        <>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr>
                {['Timestamp', 'Action', 'Resource Type', 'Resource ID', 'User ID', 'IP Address', 'Details'].map((h) => (
                  <th key={h} style={{ borderBottom: '2px solid #ddd', padding: 8, textAlign: 'left' }}>
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {data.entries.length === 0 ? (
                <tr>
                  <td colSpan={7} style={{ padding: 16, textAlign: 'center', color: '#999' }}>
                    No audit log entries found
                  </td>
                </tr>
              ) : (
                data.entries.map((entry) => (
                  <tr key={entry.id}>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8, fontSize: 12, whiteSpace: 'nowrap' }}>
                      {entry.created_at ? new Date(entry.created_at).toLocaleString() : '-'}
                    </td>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8 }}>{entry.action}</td>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8 }}>{entry.resource_type}</td>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8, fontSize: 12, fontFamily: 'monospace' }}>
                      {entry.resource_id ? `${entry.resource_id.substring(0, 8)}...` : '-'}
                    </td>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8, fontSize: 12, fontFamily: 'monospace' }}>
                      {entry.user_id ? `${entry.user_id.substring(0, 8)}...` : '-'}
                    </td>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8, fontSize: 12 }}>{entry.ip_address || '-'}</td>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8, fontSize: 12, maxWidth: 250, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {formatDetails(entry.details)}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>

          <div style={{ marginTop: 16, display: 'flex', gap: 8, alignItems: 'center' }}>
            <button disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
              Prev
            </button>
            <span>
              Page {data.page} of {data.total_pages} (total: {data.total})
            </span>
            <button disabled={page >= data.total_pages} onClick={() => setPage((p) => p + 1)}>
              Next
            </button>
          </div>
        </>
      ) : null}
    </div>
  );
}
