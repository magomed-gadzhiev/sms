import { useState } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { PageHeader } from '../../../components/layout/PageHeader';
import { BalancesTab } from './BalancesTab';
import { TransactionsTab } from './TransactionsTab';
import { CreditLimitsTab } from './CreditLimitsTab';
import { PricingTab } from './PricingTab';

const tabs = [
  { value: 'balances', label: 'Балансы' },
  { value: 'transactions', label: 'Транзакции' },
  { value: 'credit-limits', label: 'Кредитные лимиты' },
  { value: 'pricing', label: 'Правила цен' },
] as const;

export function BillingPage() {
  const [tab, setTab] = useState<string>('balances');

  const tabCls = (value: string) =>
    `px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
      tab === value
        ? 'border-primary text-primary'
        : 'border-transparent text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-200'
    }`;

  return (
    <>
      <PageHeader
        title="Биллинг"
        breadcrumbs={[
          { label: 'Админ', href: '/admin/dashboard' },
          { label: 'Биллинг' },
        ]}
      />
      <Tabs.Root value={tab} onValueChange={setTab}>
        <Tabs.List className="flex border-b border-gray-200 dark:border-slate-700 mb-4">
          {tabs.map((t) => (
            <Tabs.Trigger key={t.value} value={t.value} className={tabCls(t.value)}>
              {t.label}
            </Tabs.Trigger>
          ))}
        </Tabs.List>
        <Tabs.Content value="balances">
          <BalancesTab />
        </Tabs.Content>
        <Tabs.Content value="transactions">
          <TransactionsTab />
        </Tabs.Content>
        <Tabs.Content value="credit-limits">
          <CreditLimitsTab />
        </Tabs.Content>
        <Tabs.Content value="pricing">
          <PricingTab />
        </Tabs.Content>
      </Tabs.Root>
    </>
  );
}
