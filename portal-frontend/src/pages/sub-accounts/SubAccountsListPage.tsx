import { useState, useEffect, useRef, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { subAccountsApi, ApiError } from '../../api/client';
import { useFocusTrap } from '../../hooks/useFocusTrap';

interface SubAccount {
  id: string;
  name: string;
  email: string;
  active: boolean;
  balance: string;
  daily_limit: number;
  monthly_limit: number;
  messages_today: number;
  messages_this_month: number;
}

interface SubAccountsListResponse {
  sub_accounts: SubAccount[];
  current_count: number;
  max_sub_accounts: number;
}

export function SubAccountsListPage() {
  const [data, setData] = useState<SubAccountsListResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Create form state
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [creating, setCreating] = useState(false);
  const createFormRef = useRef<HTMLDivElement>(null);
  useFocusTrap(createFormRef, showCreateForm);
  const [formName, setFormName] = useState('');
  const [formEmail, setFormEmail] = useState('');
  const [formContactPerson, setFormContactPerson] = useState('');
  const [formInitialBalance, setFormInitialBalance] = useState('');
  const [formDailyLimit, setFormDailyLimit] = useState('');
  const [formMonthlyLimit, setFormMonthlyLimit] = useState('');

  async function loadSubAccounts() {
    setLoading(true);
    setError('');
    try {
      const resp = await subAccountsApi.list();
      setData(resp as SubAccountsListResponse);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to load sub-accounts');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadSubAccounts();
  }, []);

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    setCreating(true);
    setError('');
    try {
      await subAccountsApi.create({
        name: formName,
        email: formEmail,
        contact_person: formContactPerson || undefined,
        initial_balance: formInitialBalance || undefined,
        daily_limit: formDailyLimit ? Number(formDailyLimit) : undefined,
        monthly_limit: formMonthlyLimit ? Number(formMonthlyLimit) : undefined,
      });
      setShowCreateForm(false);
      setFormName('');
      setFormEmail('');
      setFormContactPerson('');
      setFormInitialBalance('');
      setFormDailyLimit('');
      setFormMonthlyLimit('');
      await loadSubAccounts();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to create sub-account');
    } finally {
      setCreating(false);
    }
  }

  if (loading) return <div role="status">Loading sub-accounts...</div>;

  return (
    <div style={{ maxWidth: 1000 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2>Sub-accounts</h2>
        <button onClick={() => setShowCreateForm(true)}>Create Sub-account</button>
      </div>

      {data && (
        <div style={{ marginBottom: 16, color: '#666' }}>
          Sub-accounts: {data.current_count} / {data.max_sub_accounts}
        </div>
      )}

      {error && <p style={{ color: '#d32f2f' }}>{error}</p>}

      {/* Create form */}
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
          <h3 style={{ marginTop: 0 }}>Create Sub-account</h3>
          <form onSubmit={handleCreate}>
            <div style={{ marginBottom: 12 }}>
              <label>
                Name *
                <br />
                <input
                  type="text"
                  value={formName}
                  onChange={(e) => setFormName(e.target.value)}
                  required
                  placeholder="Sub-account name"
                  style={{ width: '100%', maxWidth: 300 }}
                />
              </label>
            </div>
            <div style={{ marginBottom: 12 }}>
              <label>
                Email *
                <br />
                <input
                  type="email"
                  value={formEmail}
                  onChange={(e) => setFormEmail(e.target.value)}
                  required
                  placeholder="email@example.com"
                  style={{ width: '100%', maxWidth: 300 }}
                />
              </label>
            </div>
            <div style={{ marginBottom: 12 }}>
              <label>
                Contact Person
                <br />
                <input
                  type="text"
                  value={formContactPerson}
                  onChange={(e) => setFormContactPerson(e.target.value)}
                  placeholder="Contact person name"
                  style={{ width: '100%', maxWidth: 300 }}
                />
              </label>
            </div>
            <div style={{ marginBottom: 12 }}>
              <label>
                Initial Balance
                <br />
                <input
                  type="text"
                  value={formInitialBalance}
                  onChange={(e) => setFormInitialBalance(e.target.value)}
                  placeholder="0.00"
                  style={{ width: '100%', maxWidth: 150 }}
                />
              </label>
            </div>
            <div style={{ marginBottom: 12, display: 'flex', gap: 16 }}>
              <label>
                Daily Limit
                <br />
                <input
                  type="number"
                  value={formDailyLimit}
                  onChange={(e) => setFormDailyLimit(e.target.value)}
                  placeholder="e.g. 1000"
                  style={{ width: 150 }}
                />
              </label>
              <label>
                Monthly Limit
                <br />
                <input
                  type="number"
                  value={formMonthlyLimit}
                  onChange={(e) => setFormMonthlyLimit(e.target.value)}
                  placeholder="e.g. 30000"
                  style={{ width: 150 }}
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

      {/* Sub-accounts table */}
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <caption style={{ textAlign: 'left', marginBottom: 8, fontWeight: 'bold' }}>Sub-accounts</caption>
        <thead>
          <tr style={{ borderBottom: '2px solid #ddd', textAlign: 'left' }}>
            <th style={{ padding: 8 }}>Name</th>
            <th style={{ padding: 8 }}>Email</th>
            <th style={{ padding: 8 }}>Active</th>
            <th style={{ padding: 8 }}>Balance</th>
            <th style={{ padding: 8 }}>Daily Limit</th>
            <th style={{ padding: 8 }}>Monthly Limit</th>
            <th style={{ padding: 8 }}>Today</th>
            <th style={{ padding: 8 }}>This Month</th>
          </tr>
        </thead>
        <tbody>
          {(!data || data.sub_accounts.length === 0) ? (
            <tr>
              <td colSpan={8} style={{ padding: 16, textAlign: 'center', color: '#767676' }}>
                No sub-accounts yet. Create one to get started.
              </td>
            </tr>
          ) : (
            data.sub_accounts.map((sa) => (
              <tr key={sa.id} style={{ borderBottom: '1px solid #eee' }}>
                <td style={{ padding: 8 }}>
                  <Link to={`/sub-accounts/${sa.id}`}>{sa.name}</Link>
                </td>
                <td style={{ padding: 8 }}>{sa.email}</td>
                <td style={{ padding: 8 }}>
                  <span
                    aria-label={`Status: ${sa.active ? 'Active' : 'Inactive'}`}
                    style={{ color: sa.active ? '#4caf50' : '#d32f2f', fontWeight: 'bold' }}
                  >
                    {sa.active ? 'Active' : 'Inactive'}
                  </span>
                </td>
                <td style={{ padding: 8 }}>{sa.balance}</td>
                <td style={{ padding: 8 }}>{sa.daily_limit}</td>
                <td style={{ padding: 8 }}>{sa.monthly_limit}</td>
                <td style={{ padding: 8 }}>{sa.messages_today}</td>
                <td style={{ padding: 8 }}>{sa.messages_this_month}</td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}
