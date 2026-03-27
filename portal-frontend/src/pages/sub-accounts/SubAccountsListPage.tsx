import { useState, useEffect, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { subAccountsApi, ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
import { DataTable, type Column } from '../../components/data/DataTable';
import { StatusBadge } from '../../components/ui/Badge';

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

const columns: Column<SubAccount>[] = [
  {
    key: 'name',
    header: 'Name',
    render: (sa) => (
      <Link to={`/sub-accounts/${sa.id}`} className="text-primary hover:underline">
        {sa.name}
      </Link>
    ),
  },
  { key: 'email', header: 'Email' },
  {
    key: 'active',
    header: 'Active',
    render: (sa) => <StatusBadge status={sa.active ? 'active' : 'inactive'} />,
  },
  { key: 'balance', header: 'Balance' },
  { key: 'daily_limit', header: 'Daily Limit' },
  { key: 'monthly_limit', header: 'Monthly Limit' },
  { key: 'messages_today', header: 'Today' },
  { key: 'messages_this_month', header: 'This Month' },
];

export function SubAccountsListPage() {
  const [data, setData] = useState<SubAccountsListResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Create form state
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [creating, setCreating] = useState(false);
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

  const subAccounts = data?.sub_accounts ?? [];

  return (
    <div className="max-w-5xl">
      <PageHeader
        title="Sub-accounts"
        subtitle={data ? `${data.current_count} / ${data.max_sub_accounts}` : undefined}
        actions={<Button onClick={() => setShowCreateForm(true)}>Create Sub-account</Button>}
      />

      {error && <p className="text-red-600">{error}</p>}

      {/* Create form modal */}
      <Modal open={showCreateForm} onClose={() => setShowCreateForm(false)} title="Create Sub-account">
        <form onSubmit={handleCreate} className="flex flex-col gap-4">
          <Input
            label="Name"
            value={formName}
            onChange={(e) => setFormName(e.target.value)}
            required
            placeholder="Sub-account name"
          />
          <Input
            label="Email"
            type="email"
            value={formEmail}
            onChange={(e) => setFormEmail(e.target.value)}
            required
            placeholder="email@example.com"
          />
          <Input
            label="Contact Person"
            value={formContactPerson}
            onChange={(e) => setFormContactPerson(e.target.value)}
            placeholder="Contact person name"
          />
          <Input
            label="Initial Balance"
            value={formInitialBalance}
            onChange={(e) => setFormInitialBalance(e.target.value)}
            placeholder="0.00"
          />
          <div className="flex gap-4">
            <Input
              label="Daily Limit"
              type="number"
              value={formDailyLimit}
              onChange={(e) => setFormDailyLimit(e.target.value)}
              placeholder="e.g. 1000"
            />
            <Input
              label="Monthly Limit"
              type="number"
              value={formMonthlyLimit}
              onChange={(e) => setFormMonthlyLimit(e.target.value)}
              placeholder="e.g. 30000"
            />
          </div>
          <div className="flex gap-2 pt-2">
            <Button type="submit" disabled={creating}>
              {creating ? 'Creating...' : 'Create'}
            </Button>
            <Button type="button" variant="secondary" onClick={() => setShowCreateForm(false)}>
              Cancel
            </Button>
          </div>
        </form>
      </Modal>

      {/* Sub-accounts table */}
      <DataTable
        columns={columns}
        data={subAccounts}
        total={subAccounts.length}
        page={1}
        pageSize={subAccounts.length}
        onPageChange={() => {}}
        keyField="id"
      />
    </div>
  );
}
