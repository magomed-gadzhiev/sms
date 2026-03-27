import { useState, useEffect, useCallback } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { Badge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { billingApi, type Transaction, type PricingRule } from '../../api/admin';

const PAGE_SIZE = 20;

const txFilters: FilterDef[] = [
  { key: 'client_id', label: 'Client ID', type: 'text', placeholder: 'UUID...' },
  { key: 'transaction_type', label: 'Type', type: 'select', options: [{ value: 'credit', label: 'Credit' }, { value: 'debit', label: 'Debit' }] },
  { key: 'from', label: 'From', type: 'date' },
  { key: 'to', label: 'To', type: 'date' },
];

const txColumns: Column<Transaction>[] = [
  { key: 'created_at', header: 'Date', render: (t) => new Date(t.created_at).toLocaleString(), sortable: true },
  { key: 'client_id', header: 'Client', render: (t) => t.client_id.slice(0, 8) + '...' },
  { key: 'type', header: 'Type', render: (t) => <Badge variant={t.type === 'credit' ? 'success' : 'danger'}>{t.type}</Badge> },
  { key: 'amount', header: 'Amount', render: (t) => `${t.amount} ${t.currency}` },
  { key: 'balance_after', header: 'Balance After' },
  { key: 'description', header: 'Description' },
];

const ruleColumns: Column<PricingRule>[] = [
  { key: 'client_id', header: 'Client', render: (r) => r.client_id.slice(0, 8) + '...' },
  { key: 'destination_pattern', header: 'Pattern' },
  { key: 'price_per_message', header: 'Price/msg', render: (r) => `${r.price_per_message} ${r.currency}` },
  { key: 'priority', header: 'Priority', sortable: true },
  { key: 'active', header: 'Active', render: (r) => r.active ? 'Yes' : 'No' },
];

export function BillingPage() {
  const toast = useToast();
  const [tab, setTab] = useState('transactions');
  const [txData, setTxData] = useState<Transaction[]>([]);
  const [txTotal, setTxTotal] = useState(0);
  const [txPage, setTxPage] = useState(1);
  const [txFilter, setTxFilter] = useState<Record<string, string>>({});
  const [txLoading, setTxLoading] = useState(true);
  const [rules, setRules] = useState<PricingRule[]>([]);
  const [rulesLoading, setRulesLoading] = useState(true);
  const [showAddCredits, setShowAddCredits] = useState(false);
  const [creditForm, setCreditForm] = useState({ client_id: '', amount: '', description: '' });
  const [saving, setSaving] = useState(false);

  const fetchTransactions = useCallback(async () => {
    setTxLoading(true);
    try { const res = await billingApi.getTransactions({ ...txFilter, limit: PAGE_SIZE, offset: (txPage - 1) * PAGE_SIZE }); setTxData(res.transactions || []); setTxTotal(res.total); }
    catch { toast.error('Failed to load transactions'); }
    finally { setTxLoading(false); }
  }, [txPage, txFilter, toast]);

  const fetchRules = useCallback(async () => {
    setRulesLoading(true);
    try { const res = await billingApi.getPricingRules({}); setRules(res.rules || []); }
    catch { toast.error('Failed to load pricing rules'); }
    finally { setRulesLoading(false); }
  }, [toast]);

  useEffect(() => { if (tab === 'transactions') fetchTransactions(); }, [tab, fetchTransactions]);
  useEffect(() => { if (tab === 'pricing') fetchRules(); }, [tab, fetchRules]);

  const handleAddCredits = async () => {
    setSaving(true);
    try { const res = await billingApi.addCredits(creditForm.client_id, { amount: creditForm.amount, description: creditForm.description }); toast.success(`Credits added. New balance: ${res.new_balance}`); setShowAddCredits(false); fetchTransactions(); }
    catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); }
    finally { setSaving(false); }
  };

  const tabCls = (value: string) => `px-4 py-2 text-sm font-medium border-b-2 transition-colors ${tab === value ? 'border-primary text-primary' : 'border-transparent text-gray-500 hover:text-gray-700'}`;

  return (
    <>
      <PageHeader title="Billing" actions={<Button onClick={() => { setCreditForm({ client_id: '', amount: '', description: '' }); setShowAddCredits(true); }}>Add Credits</Button>} />
      <Tabs.Root value={tab} onValueChange={setTab}>
        <Tabs.List className="flex border-b border-gray-200 mb-4">
          <Tabs.Trigger value="transactions" className={tabCls('transactions')}>Transactions</Tabs.Trigger>
          <Tabs.Trigger value="pricing" className={tabCls('pricing')}>Pricing Rules</Tabs.Trigger>
        </Tabs.List>
        <Tabs.Content value="transactions">
          <FilterBar filters={txFilters} values={txFilter} onChange={(v) => { setTxFilter(v); setTxPage(1); }} onReset={() => { setTxFilter({}); setTxPage(1); }} />
          <DataTable columns={txColumns} data={txData} total={txTotal} page={txPage} pageSize={PAGE_SIZE} onPageChange={setTxPage} loading={txLoading} keyField="transaction_id" />
        </Tabs.Content>
        <Tabs.Content value="pricing">
          <DataTable columns={ruleColumns} data={rules} total={rules.length} page={1} pageSize={100} onPageChange={() => {}} loading={rulesLoading} keyField="rule_id" />
        </Tabs.Content>
      </Tabs.Root>
      <Modal open={showAddCredits} onClose={() => setShowAddCredits(false)} title="Add Credits">
        <div className="space-y-4">
          <Input label="Client ID" value={creditForm.client_id} onChange={(e) => setCreditForm({ ...creditForm, client_id: e.target.value })} required placeholder="UUID" />
          <Input label="Amount" type="number" value={creditForm.amount} onChange={(e) => setCreditForm({ ...creditForm, amount: e.target.value })} required placeholder="0.00" />
          <Input label="Description" value={creditForm.description} onChange={(e) => setCreditForm({ ...creditForm, description: e.target.value })} />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowAddCredits(false)}>Cancel</Button>
            <Button onClick={handleAddCredits} disabled={saving || !creditForm.client_id || !creditForm.amount}>{saving ? 'Adding...' : 'Add Credits'}</Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
