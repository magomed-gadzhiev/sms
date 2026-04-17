import { Suspense, lazy, useState, useEffect } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { Table2, TrendingUp, Activity } from 'lucide-react';
import { useNetworkStats } from '../../hooks/useNetworkStats';
import { StatisticsFilterBar } from '../../components/network-stats/StatisticsFilterBar';
import { referencesApi } from '../../api/client';

// Lazy-load heavy components
const StatisticsKPIStrip = lazy(() => import('../../components/network-stats/StatisticsKPIStrip').then(m => ({ default: m.StatisticsKPIStrip })));
const StatisticsTable = lazy(() => import('../../components/network-stats/StatisticsTable').then(m => ({ default: m.StatisticsTable })));
const MonitoringKPIGrid = lazy(() => import('../../components/network-stats/MonitoringKPIGrid').then(m => ({ default: m.MonitoringKPIGrid })));
const MonitoringTable = lazy(() => import('../../components/network-stats/MonitoringTable').then(m => ({ default: m.MonitoringTable })));
const DrillDownDrawer = lazy(() => import('../../components/network-stats/DrillDownDrawer').then(m => ({ default: m.DrillDownDrawer })));
const ExportButton = lazy(() => import('../../components/network-stats/ExportButton').then(m => ({ default: m.ExportButton })));

export default function NetworkStatisticsPage() {
  const stats = useNetworkStats();
  const [operators, setOperators] = useState<{ id: string; name: string }[]>([]);
  const [operatorsError, setOperatorsError] = useState(false);

  useEffect(() => {
    loadOperators();
  }, []);

  function loadOperators() {
    setOperatorsError(false);
    referencesApi.operators()
      .then(r => setOperators(r.operators || []))
      .catch(() => setOperatorsError(true));
  }

  return (
    <div className="min-h-screen">
      <Tabs.Root value={stats.mode} onValueChange={(v) => stats.setMode(v as any)}>
        {/* Tab bar */}
        <div className="flex items-center border-b border-gray-200 bg-white px-4">
          <Tabs.List className="flex">
            <Tabs.Trigger
              value="stats"
              className="flex items-center gap-1.5 px-4 py-3.5 text-sm font-medium border-b-2 transition-colors data-[state=active]:border-blue-600 data-[state=active]:text-blue-600 text-gray-500 border-transparent hover:text-gray-700"
            >
              <Table2 size={16} />
              Статистика
            </Tabs.Trigger>
            <Tabs.Trigger
              value="analytics"
              className="flex items-center gap-1.5 px-4 py-3.5 text-sm font-medium border-b-2 transition-colors data-[state=active]:border-blue-600 data-[state=active]:text-blue-600 text-gray-500 border-transparent hover:text-gray-700"
            >
              <TrendingUp size={16} />
              Аналитика
            </Tabs.Trigger>
            <Tabs.Trigger
              value="monitoring"
              className="flex items-center gap-1.5 px-4 py-3.5 text-sm font-medium border-b-2 transition-colors data-[state=active]:border-blue-600 data-[state=active]:text-blue-600 text-gray-500 border-transparent hover:text-gray-700"
            >
              <Activity size={16} />
              Мониторинг
            </Tabs.Trigger>
          </Tabs.List>
        </div>

        {/* Operators loading error */}
        {operatorsError && (
          <div className="mx-4 mt-2 px-3 py-2 bg-amber-50 border border-amber-200 rounded-md text-xs text-amber-700 flex items-center gap-2">
            Не удалось загрузить список операторов.
            <button onClick={loadOperators} className="underline font-medium hover:text-amber-900">Повторить</button>
          </div>
        )}

        {/* Filter bar — same for all modes */}
        <StatisticsFilterBar
          mode={stats.mode}
          filters={stats.filters}
          onFiltersChange={stats.setFilters}
          onApply={stats.applyFilters}
          loading={stats.loading}
          polling={stats.mode === 'monitoring' ? {
            isPaused: stats.isPaused,
            toggle: stats.togglePolling,
            lastUpdated: stats.lastUpdated,
          } : undefined}
          operators={operators}
        />

        <Suspense fallback={<div className="p-8 text-center text-gray-400">Загрузка...</div>}>
          {/* Statistics mode */}
          <Tabs.Content value="stats" className="outline-none">
            <StatisticsKPIStrip kpis={stats.data?.kpis ?? []} />
            <div className="flex items-center justify-between px-4 py-2">
              <span className="text-xs text-gray-400">
                {(() => {
                  const p = (stats.data as any)?.pagination;
                  if (!p || !p.page || !p.page_size || !p.total_rows) return '';
                  const from = (p.page - 1) * p.page_size + 1;
                  const to = Math.min(p.page * p.page_size, p.total_rows);
                  if (isNaN(from) || isNaN(to) || isNaN(p.total_rows)) return '';
                  return `Показано ${from}–${to} из ${p.total_rows}`;
                })()}
              </span>
              <ExportButton onExport={stats.startExport} status={stats.exportStatus} />
            </div>
            <StatisticsTable
              rows={(stats.data as any)?.rows ?? []}
              pagination={(stats.data as any)?.pagination}
              filters={stats.filters}
              onFiltersChange={stats.setFilters}
              onApply={stats.applyFilters}
              onRowClick={(row: any) => stats.openDrillDown('provider', row.slice, row.slice)}
              loading={stats.loading}
            />
          </Tabs.Content>

          {/* Analytics mode (Phase 2 — same table for now) */}
          <Tabs.Content value="analytics" className="outline-none">
            <StatisticsKPIStrip kpis={stats.data?.kpis ?? []} />
            <StatisticsTable
              rows={(stats.data as any)?.rows ?? []}
              pagination={(stats.data as any)?.pagination}
              filters={stats.filters}
              onFiltersChange={stats.setFilters}
              onApply={stats.applyFilters}
              onRowClick={(row: any) => stats.openDrillDown('provider', row.slice, row.slice)}
              loading={stats.loading}
            />
          </Tabs.Content>

          {/* Monitoring mode */}
          <Tabs.Content value="monitoring" className="outline-none">
            <MonitoringKPIGrid kpis={stats.data?.kpis ?? []} />
            <div className="flex items-center justify-between px-4 py-2">
              <span className="text-xs text-gray-400">
                {(stats.data as any)?.pagination
                  ? `Показано ${(stats.data as any).pagination.total_rows ?? 0} провайдеров`
                  : ''}
              </span>
              <ExportButton onExport={stats.startExport} status={stats.exportStatus} />
            </div>
            <MonitoringTable
              rows={(stats.data as any)?.rows ?? []}
              pagination={(stats.data as any)?.pagination}
              filters={stats.filters}
              onFiltersChange={stats.setFilters}
              onApply={stats.applyFilters}
              onRowClick={(row: any) => stats.openDrillDown('provider', row.slice, row.slice)}
              loading={stats.loading}
            />
          </Tabs.Content>
        </Suspense>
      </Tabs.Root>

      {/* Drill-down drawer */}
      <Suspense fallback={null}>
        <DrillDownDrawer
          open={stats.drillDown !== null}
          onClose={stats.closeDrillDown}
          data={stats.drillDown}
          stack={stats.drillDownStack}
          activeView={stats.drillDownView}
          onViewChange={stats.setDrillDownView}
          onNavigate={stats.navigateDrillDown}
          onDrillDeeper={stats.drillDeeper}
          loading={stats.drillDownLoading}
        />
      </Suspense>

      {/* Error toast */}
      {stats.error && (
        <div className="fixed bottom-4 right-4 rounded-lg bg-red-50 border border-red-200 px-4 py-3 text-sm text-red-700 shadow-lg">
          {stats.error}
        </div>
      )}
    </div>
  );
}
