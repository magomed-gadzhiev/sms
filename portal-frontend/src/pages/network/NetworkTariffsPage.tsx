import { useState } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { PageHeader } from '../../components/layout/PageHeader';
import { TariffOverviewTab } from './components/TariffOverviewTab';
import { TariffTemplatesTab } from './components/TariffTemplatesTab';
import { TariffOverridesTab } from './components/TariffOverridesTab';

const TAB_VALUES = ['overview', 'templates', 'overrides'] as const;
type TabValue = (typeof TAB_VALUES)[number];

const TAB_LABELS: Record<TabValue, string> = {
  overview: 'Обзор',
  templates: 'Шаблоны',
  overrides: 'Переопределения',
};

export function NetworkTariffsPage() {
  const [tab, setTab] = useState<TabValue>('overview');

  const tabCls = (value: string) =>
    `px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
      tab === value
        ? 'border-primary text-primary'
        : 'border-transparent text-gray-500 hover:text-gray-700'
    }`;

  return (
    <div className="max-w-6xl">
      <PageHeader title="Тарифы субаккаунтов" />

      <Tabs.Root value={tab} onValueChange={(v) => setTab(v as TabValue)}>
        <Tabs.List className="flex border-b border-gray-200 mb-6">
          {TAB_VALUES.map((v) => (
            <Tabs.Trigger key={v} value={v} className={tabCls(v)}>
              {TAB_LABELS[v]}
            </Tabs.Trigger>
          ))}
        </Tabs.List>

        <Tabs.Content value="overview">
          <TariffOverviewTab />
        </Tabs.Content>

        <Tabs.Content value="templates">
          <TariffTemplatesTab />
        </Tabs.Content>

        <Tabs.Content value="overrides">
          <TariffOverridesTab />
        </Tabs.Content>
      </Tabs.Root>
    </div>
  );
}
