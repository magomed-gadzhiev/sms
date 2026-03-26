import { useState, useEffect, useCallback, type FormEvent } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { subAccountsApi, ApiError } from '../../api/client';

type TabName = 'overview' | 'messages' | 'analytics' | 'api-keys' | 'webhooks';

const TABS: { key: TabName; label: string }[] = [
  { key: 'overview', label: 'Overview' },
  { key: 'messages', label: 'Messages' },
  { key: 'analytics', label: 'Analytics' },
  { key: 'api-keys', label: 'API Keys' },
  { key: 'webhooks', label: 'Webhooks' },
];

interface SubAccountDetail {
  id: string;
  name: string;
  email: string;
  contact_person: string;
  active: boolean;
  balance: string;
  daily_limit: number;
  monthly_limit: number;
  messages_today: number;
  messages_this_month: number;
  api_keys: APIKeyItem[];
  webhooks: WebhookItem[];
}

interface APIKeyItem {
  id: string;
  name: string;
  prefix: string;
  active: boolean;
  created_at: string;
}

interface WebhookItem {
  id: string;
  url: string;
  event_types: string[];
  active: boolean;
  created_at?: string;
}

interface MessageItem {
  message_id: string;
  source: string;
  destination: string;
  text: string;
  status: string;
  segment_count: number;
  created_at?: string;
}

interface MessagesResponse {
  messages: MessageItem[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}

interface AnalyticsSummary {
  total_sent: number;
  total_delivered: number;
  total_failed: number;
  delivery_rate: number;
  total_cost: string;
  currency: string;
}

interface TimelineEntry {
  period: string;
  sent: number;
  delivered: number;
  failed: number;
  delivery_rate: number;
}

interface AnalyticsData {
  summary: AnalyticsSummary;
  timeline: TimelineEntry[];
}

export function SubAccountDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<TabName>('overview');
  const [detail, setDetail] = useState<SubAccountDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  async function loadDetail() {
    if (!id) return;
    setLoading(true);
    setError('');
    try {
      const resp = await subAccountsApi.get(id);
      setDetail(resp as SubAccountDetail);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to load sub-account');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadDetail();
  }, [id]);

  if (loading) return <div role="status">Loading...</div>;
  if (error) return <div style={{ color: 'red' }}>{error}</div>;
  if (!detail) return <div>Sub-account not found</div>;

  return (
    <div style={{ maxWidth: 1000 }}>
      <div style={{ marginBottom: 16 }}>
        <button onClick={() => navigate('/sub-accounts')} style={{ marginRight: 16 }}>
          &larr; Back
        </button>
        <span style={{ fontSize: 20, fontWeight: 'bold' }}>{detail.name}</span>
        <span
          aria-label={`Status: ${detail.active ? 'Active' : 'Inactive'}`}
          style={{ marginLeft: 12, color: detail.active ? '#4caf50' : '#d32f2f', fontWeight: 'bold' }}
        >
          {detail.active ? 'Active' : 'Inactive'}
        </span>
      </div>

      {/* Tabs */}
      <div style={{ display: 'flex', gap: 0, borderBottom: '2px solid #ddd', marginBottom: 16 }}>
        {TABS.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            style={{
              padding: '8px 20px',
              border: 'none',
              borderBottom: activeTab === tab.key ? '2px solid #1976d2' : '2px solid transparent',
              background: 'none',
              cursor: 'pointer',
              fontWeight: activeTab === tab.key ? 'bold' : 'normal',
              color: activeTab === tab.key ? '#1976d2' : '#333',
              marginBottom: -2,
            }}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {activeTab === 'overview' && (
        <OverviewTab detail={detail} onUpdate={loadDetail} onDeleted={() => navigate('/sub-accounts')} />
      )}
      {activeTab === 'messages' && id && <MessagesTab subAccountId={id} />}
      {activeTab === 'analytics' && id && <AnalyticsTab subAccountId={id} />}
      {activeTab === 'api-keys' && <APIKeysTab keys={detail.api_keys || []} />}
      {activeTab === 'webhooks' && <WebhooksTab webhooks={detail.webhooks || []} />}
    </div>
  );
}

// ---- Overview Tab ----

function OverviewTab({
  detail,
  onUpdate,
  onDeleted,
}: {
  detail: SubAccountDetail;
  onUpdate: () => void;
  onDeleted: () => void;
}) {
  const [error, setError] = useState('');

  // Edit limits
  const [dailyLimit, setDailyLimit] = useState(String(detail.daily_limit));
  const [monthlyLimit, setMonthlyLimit] = useState(String(detail.monthly_limit));
  const [savingLimits, setSavingLimits] = useState(false);

  // Balance transfer
  const [transferAmount, setTransferAmount] = useState('');
  const [transferring, setTransferring] = useState(false);

  // Delete confirmation
  const [confirmDelete, setConfirmDelete] = useState(false);

  async function handleSaveLimits(e: FormEvent) {
    e.preventDefault();
    setSavingLimits(true);
    setError('');
    try {
      await subAccountsApi.updateLimits(detail.id, {
        daily_limit: Number(dailyLimit),
        monthly_limit: Number(monthlyLimit),
      });
      onUpdate();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to update limits');
    } finally {
      setSavingLimits(false);
    }
  }

  async function handleTransfer(e: FormEvent) {
    e.preventDefault();
    setTransferring(true);
    setError('');
    try {
      await subAccountsApi.transfer(detail.id, transferAmount);
      setTransferAmount('');
      onUpdate();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to transfer balance');
    } finally {
      setTransferring(false);
    }
  }

  async function handleDelete() {
    setError('');
    try {
      await subAccountsApi.remove(detail.id);
      onDeleted();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to delete sub-account');
    }
  }

  return (
    <div>
      {error && <p style={{ color: '#d32f2f' }}>{error}</p>}

      {/* Info cards */}
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 16, marginBottom: 24 }}>
        {[
          { label: 'Balance', value: detail.balance },
          { label: 'Daily Limit', value: `${detail.messages_today} / ${detail.daily_limit}` },
          { label: 'Monthly Limit', value: `${detail.messages_this_month} / ${detail.monthly_limit}` },
          { label: 'Email', value: detail.email },
          { label: 'Contact', value: detail.contact_person || '-' },
        ].map((card) => (
          <div
            key={card.label}
            style={{
              border: '1px solid #ddd',
              borderRadius: 8,
              padding: 16,
              minWidth: 160,
              textAlign: 'center',
            }}
          >
            <div style={{ fontSize: 14, color: '#666', marginBottom: 8 }}>{card.label}</div>
            <div style={{ fontSize: 18, fontWeight: 'bold' }}>{card.value}</div>
          </div>
        ))}
      </div>

      {/* Edit limits form */}
      <div
        style={{
          background: '#f5f5f5',
          border: '1px solid #ddd',
          borderRadius: 4,
          padding: 16,
          marginBottom: 16,
        }}
      >
        <h3 style={{ marginTop: 0 }}>Edit Limits</h3>
        <form onSubmit={handleSaveLimits} style={{ display: 'flex', gap: 16, alignItems: 'flex-end', flexWrap: 'wrap' }}>
          <label>
            Daily Limit
            <br />
            <input
              type="number"
              value={dailyLimit}
              onChange={(e) => setDailyLimit(e.target.value)}
              style={{ width: 150 }}
            />
          </label>
          <label>
            Monthly Limit
            <br />
            <input
              type="number"
              value={monthlyLimit}
              onChange={(e) => setMonthlyLimit(e.target.value)}
              style={{ width: 150 }}
            />
          </label>
          <button type="submit" disabled={savingLimits}>
            {savingLimits ? 'Saving...' : 'Save Limits'}
          </button>
        </form>
      </div>

      {/* Balance transfer form */}
      <div
        style={{
          background: '#f5f5f5',
          border: '1px solid #ddd',
          borderRadius: 4,
          padding: 16,
          marginBottom: 16,
        }}
      >
        <h3 style={{ marginTop: 0 }}>Balance Transfer</h3>
        <form onSubmit={handleTransfer} style={{ display: 'flex', gap: 16, alignItems: 'flex-end' }}>
          <label>
            Amount
            <br />
            <input
              type="text"
              value={transferAmount}
              onChange={(e) => setTransferAmount(e.target.value)}
              placeholder="0.00"
              required
              style={{ width: 150 }}
            />
          </label>
          <button type="submit" disabled={transferring || !transferAmount}>
            {transferring ? 'Transferring...' : 'Transfer'}
          </button>
        </form>
      </div>

      {/* Delete section */}
      <div
        style={{
          border: '1px solid #d32f2f',
          borderRadius: 4,
          padding: 16,
        }}
      >
        <h3 style={{ marginTop: 0, color: '#d32f2f' }}>Danger Zone</h3>
        {confirmDelete ? (
          <div>
            <p>Are you sure you want to delete this sub-account? This cannot be undone.</p>
            <div style={{ display: 'flex', gap: 8 }}>
              <button onClick={handleDelete} style={{ background: '#d32f2f', color: '#fff', border: 'none', padding: '6px 16px', cursor: 'pointer' }}>
                Yes, Delete
              </button>
              <button onClick={() => setConfirmDelete(false)}>Cancel</button>
            </div>
          </div>
        ) : (
          <button onClick={() => setConfirmDelete(true)} style={{ color: '#d32f2f' }}>
            Delete Sub-account
          </button>
        )}
      </div>
    </div>
  );
}

// ---- Messages Tab ----

function MessagesTab({ subAccountId }: { subAccountId: string }) {
  const [data, setData] = useState<MessagesResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState('');

  const fetchMessages = useCallback(() => {
    setLoading(true);
    setError('');
    const params: Record<string, string> = { page: String(page), per_page: '20' };
    if (status) params.status = status;

    subAccountsApi
      .messages(subAccountId, params)
      .then((resp) => setData(resp as MessagesResponse))
      .catch((err) => setError(err instanceof ApiError ? err.message : 'Failed to load messages'))
      .finally(() => setLoading(false));
  }, [subAccountId, page, status]);

  useEffect(() => {
    fetchMessages();
  }, [fetchMessages]);

  return (
    <div>
      <div style={{ display: 'flex', gap: 12, marginBottom: 16, alignItems: 'flex-end' }}>
        <label>
          Status
          <br />
          <select value={status} onChange={(e) => { setStatus(e.target.value); setPage(1); }} style={{ padding: 4 }}>
            <option value="">All</option>
            <option value="pending">Pending</option>
            <option value="sent">Sent</option>
            <option value="delivered">Delivered</option>
            <option value="failed">Failed</option>
          </select>
        </label>
      </div>

      {error && <p style={{ color: '#d32f2f' }}>{error}</p>}
      {loading ? (
        <div role="status">Loading messages...</div>
      ) : data ? (
        <>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <caption style={{ textAlign: 'left', marginBottom: 8, fontWeight: 'bold' }}>Messages list</caption>
            <thead>
              <tr>
                {['ID', 'Source', 'Destination', 'Text', 'Status', 'Segments', 'Created'].map((h) => (
                  <th key={h} style={{ borderBottom: '2px solid #ddd', padding: 8, textAlign: 'left' }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {data.messages.length === 0 ? (
                <tr>
                  <td colSpan={7} style={{ padding: 16, textAlign: 'center', color: '#767676' }}>No messages found</td>
                </tr>
              ) : (
                data.messages.map((msg) => (
                  <tr key={msg.message_id}>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8, fontSize: 12, fontFamily: 'monospace' }}>
                      {msg.message_id.substring(0, 8)}...
                    </td>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8 }}>{msg.source}</td>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8 }}>{msg.destination}</td>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8, maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {msg.text}
                    </td>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8 }}>{msg.status}</td>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8 }}>{msg.segment_count}</td>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8, fontSize: 12 }}>
                      {msg.created_at ? new Date(msg.created_at).toLocaleString() : '-'}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>

          {data.total_pages > 1 && (
            <div style={{ marginTop: 16, display: 'flex', gap: 8, alignItems: 'center' }}>
              <button disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>Prev</button>
              <span>Page {data.page} of {data.total_pages} (total: {data.total})</span>
              <button disabled={page >= data.total_pages} onClick={() => setPage((p) => p + 1)}>Next</button>
            </div>
          )}
        </>
      ) : null}
    </div>
  );
}

// ---- Analytics Tab ----

function AnalyticsTab({ subAccountId }: { subAccountId: string }) {
  const [data, setData] = useState<AnalyticsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [period, setPeriod] = useState('7d');

  useEffect(() => {
    setLoading(true);
    setError('');
    subAccountsApi
      .analytics(subAccountId, { period, group_by: 'day' })
      .then((resp) => setData(resp as AnalyticsData))
      .catch((err) => setError(err instanceof ApiError ? err.message : 'Failed to load analytics'))
      .finally(() => setLoading(false));
  }, [subAccountId, period]);

  return (
    <div>
      <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
        {['7d', '30d', '90d'].map((p) => (
          <button
            key={p}
            onClick={() => setPeriod(p)}
            style={{
              padding: '6px 16px',
              fontWeight: period === p ? 'bold' : 'normal',
              background: period === p ? '#1976d2' : '#e0e0e0',
              color: period === p ? '#fff' : '#333',
              border: 'none',
              borderRadius: 4,
              cursor: 'pointer',
            }}
          >
            {p}
          </button>
        ))}
      </div>

      {error && <p style={{ color: '#d32f2f' }}>{error}</p>}
      {loading && <div role="status">Loading analytics...</div>}

      {data && !loading && (
        <>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 16, marginBottom: 24 }}>
            {[
              { label: 'Sent', value: data.summary.total_sent },
              { label: 'Delivered', value: data.summary.total_delivered },
              { label: 'Failed', value: data.summary.total_failed },
              { label: 'Delivery Rate', value: `${data.summary.delivery_rate}%` },
              { label: 'Total Cost', value: data.summary.total_cost ? `${data.summary.total_cost} ${data.summary.currency}` : 'N/A' },
            ].map((card) => (
              <div
                key={card.label}
                style={{
                  border: '1px solid #ddd',
                  borderRadius: 8,
                  padding: 16,
                  minWidth: 140,
                  textAlign: 'center',
                }}
              >
                <div style={{ fontSize: 14, color: '#666', marginBottom: 8 }}>{card.label}</div>
                <div style={{ fontSize: 24, fontWeight: 'bold' }}>{card.value}</div>
              </div>
            ))}
          </div>

          {data.timeline.length > 0 && (
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <caption style={{ textAlign: 'left', marginBottom: 8, fontWeight: 'bold' }}>Message statistics by period</caption>
              <thead>
                <tr style={{ borderBottom: '2px solid #ddd', textAlign: 'left' }}>
                  <th style={{ padding: 8 }}>Period</th>
                  <th style={{ padding: 8 }}>Sent</th>
                  <th style={{ padding: 8 }}>Delivered</th>
                  <th style={{ padding: 8 }}>Failed</th>
                  <th style={{ padding: 8 }}>Delivery Rate</th>
                </tr>
              </thead>
              <tbody>
                {data.timeline.map((row, i) => (
                  <tr key={i} style={{ borderBottom: '1px solid #eee' }}>
                    <td style={{ padding: 8 }}>{row.period}</td>
                    <td style={{ padding: 8 }}>{row.sent}</td>
                    <td style={{ padding: 8 }}>{row.delivered}</td>
                    <td style={{ padding: 8 }}>{row.failed}</td>
                    <td style={{ padding: 8 }}>{row.delivery_rate}%</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </>
      )}
    </div>
  );
}

// ---- API Keys Tab ----

function APIKeysTab({ keys }: { keys: APIKeyItem[] }) {
  return (
    <div>
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <caption style={{ textAlign: 'left', marginBottom: 8, fontWeight: 'bold' }}>API Keys</caption>
        <thead>
          <tr style={{ borderBottom: '2px solid #ddd', textAlign: 'left' }}>
            <th style={{ padding: 8 }}>Name</th>
            <th style={{ padding: 8 }}>Prefix</th>
            <th style={{ padding: 8 }}>Status</th>
            <th style={{ padding: 8 }}>Created</th>
          </tr>
        </thead>
        <tbody>
          {keys.length === 0 ? (
            <tr>
              <td colSpan={4} style={{ padding: 16, textAlign: 'center', color: '#767676' }}>
                No API keys for this sub-account.
              </td>
            </tr>
          ) : (
            keys.map((key) => (
              <tr key={key.id} style={{ borderBottom: '1px solid #eee' }}>
                <td style={{ padding: 8 }}>{key.name}</td>
                <td style={{ padding: 8 }}><code>{key.prefix}...</code></td>
                <td style={{ padding: 8 }}>
                  <span
                    aria-label={`Status: ${key.active ? 'Active' : 'Revoked'}`}
                    style={{ color: key.active ? '#4caf50' : '#d32f2f', fontWeight: 'bold' }}
                  >
                    {key.active ? 'Active' : 'Revoked'}
                  </span>
                </td>
                <td style={{ padding: 8 }}>{new Date(key.created_at).toLocaleDateString()}</td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}

// ---- Webhooks Tab ----

function WebhooksTab({ webhooks }: { webhooks: WebhookItem[] }) {
  return (
    <div>
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <caption style={{ textAlign: 'left', marginBottom: 8, fontWeight: 'bold' }}>Webhook subscriptions</caption>
        <thead>
          <tr style={{ borderBottom: '2px solid #ddd', textAlign: 'left' }}>
            <th style={{ padding: 8 }}>URL</th>
            <th style={{ padding: 8 }}>Event Types</th>
            <th style={{ padding: 8 }}>Status</th>
            <th style={{ padding: 8 }}>Created</th>
          </tr>
        </thead>
        <tbody>
          {webhooks.length === 0 ? (
            <tr>
              <td colSpan={4} style={{ padding: 16, textAlign: 'center', color: '#767676' }}>
                No webhooks for this sub-account.
              </td>
            </tr>
          ) : (
            webhooks.map((wh) => (
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
                        fontSize: 12,
                      }}
                    >
                      {t}
                    </span>
                  ))}
                </td>
                <td style={{ padding: 8 }}>
                  <span
                    aria-label={`Status: ${wh.active ? 'Active' : 'Inactive'}`}
                    style={{ color: wh.active ? '#4caf50' : '#d32f2f', fontWeight: 'bold' }}
                  >
                    {wh.active ? 'Active' : 'Inactive'}
                  </span>
                </td>
                <td style={{ padding: 8 }}>
                  {wh.created_at ? new Date(wh.created_at).toLocaleDateString() : '-'}
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}
