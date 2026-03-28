import { useState, useEffect, useCallback } from 'react';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { useToast } from '../../../components/ui/Toast';
import { billingApi, type PricingRule } from '../../../api/admin';

const ruleColumns: Column<PricingRule>[] = [
  {
    key: 'client_id',
    header: 'Клиент',
    render: (r) => r.client_id.slice(0, 8) + '...',
  },
  { key: 'destination_pattern', header: 'Шаблон' },
  {
    key: 'price_per_message',
    header: 'Цена/сообщение',
    render: (r) => `${r.price_per_message} ${r.currency}`,
  },
  { key: 'priority', header: 'Приоритет', sortable: true },
  {
    key: 'active',
    header: 'Активно',
    render: (r) => (r.active ? 'Да' : 'Нет'),
  },
];

export function PricingTab() {
  const toast = useToast();
  const [rules, setRules] = useState<PricingRule[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchRules = useCallback(async () => {
    setLoading(true);
    try {
      const res = await billingApi.getPricingRules({});
      setRules(res.rules || []);
    } catch {
      toast.error('Не удалось загрузить правила цен');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    fetchRules();
  }, [fetchRules]);

  return (
    <DataTable
      columns={ruleColumns}
      data={rules}
      total={rules.length}
      page={1}
      pageSize={100}
      onPageChange={() => {}}
      loading={loading}
      keyField="rule_id"
    />
  );
}
