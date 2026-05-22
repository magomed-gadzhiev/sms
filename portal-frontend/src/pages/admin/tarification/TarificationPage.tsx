import { useState } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { PageHeader } from '../../../components/layout/PageHeader';
import { PlansTab } from './PlansTab';
import { PeriodsTab } from './PeriodsTab';
import { TiersTab } from './TiersTab';
import { SenderRegistrationsTab } from './SenderRegistrationsTab';

const tabs = [
  { value: 'plans', label: 'Тарифные планы' },
  { value: 'periods', label: 'Периоды' },
  { value: 'tiers', label: 'Тиры' },
  { value: 'senders', label: 'Регистрация отправителей' },
] as const;

export function TarificationPage() {
  const [tab, setTab] = useState<string>('plans');

  const tabCls = (value: string) =>
    `px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
      tab === value
        ? 'border-primary text-primary'
        : 'border-transparent text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-200'
    }`;

  return (
    <>
      <PageHeader
        title="Тарификация"
        breadcrumbs={[
          { label: 'Админ', href: '/admin/dashboard' },
          { label: 'Тарификация' },
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
        <Tabs.Content value="plans">
          <PlansTab />
        </Tabs.Content>
        <Tabs.Content value="periods">
          <PeriodsTab />
        </Tabs.Content>
        <Tabs.Content value="tiers">
          <TiersTab />
        </Tabs.Content>
        <Tabs.Content value="senders">
          <SenderRegistrationsTab />
        </Tabs.Content>
      </Tabs.Root>
    </>
  );
}
