// portal-frontend/src/pages/CommandCenter.tsx
import { useEffect, useState, useRef, useCallback } from 'react';
import { Link } from 'react-router-dom';
import {
  LineChart, Line, ResponsiveContainer, Tooltip,
  PieChart, Pie, Cell, BarChart, Bar, XAxis, YAxis,
} from 'recharts';
import {
  commandCenterApi,
  dashboardApi,
  subAccountsApi,
  type DashboardChartData,
  type DashboardMetrics,
  type ProviderHealth,
  type AlertItem,
  type LiveMessageEvent,
} from '../api/client';
import { useWebSocket } from '../hooks/useWebSocket';
import { usePolling } from '../hooks/usePolling';
import { useAuth } from '../contexts/AuthContext';
import { usePageTitle } from '../contexts/PageTitleContext';
import { RelativeTime } from '../components/ui/RelativeTime';

// ── KPI Card ──────────────────────────────────────────────────────────

function KpiCard({
  title,
  value,
  subtitle,
  trend,
  href,
  children,
}: {
  title: string;
  value: string | number;
  subtitle?: string;
  trend?: number; // positive = up, negative = down
  /** If set, the whole card becomes a Link to this route. */
  href?: string;
  children?: React.ReactNode;
}) {
  const body = (
    <>
      <div className="flex items-center justify-between">
        <span style={{ color: 'var(--cc-text-muted)' }} className="text-xs font-medium uppercase tracking-wider">
          {title}
        </span>
        <span className="flex items-center gap-1.5">
          {trend !== undefined && trend !== 0 && (
            <span
              className="text-xs font-semibold flex items-center gap-0.5"
              style={{ color: trend > 0 ? 'var(--cc-accent-green)' : 'var(--cc-accent-red)' }}
            >
              {trend > 0 ? '↑' : '↓'} {Math.abs(trend)}%
            </span>
          )}
          {href && (
            <span style={{ color: 'var(--cc-text-muted)' }} className="text-xs" aria-hidden="true">
              →
            </span>
          )}
        </span>
      </div>
      <div style={{ color: 'var(--cc-text)' }} className="text-3xl font-bold tabular-nums">
        {value}
      </div>
      {subtitle && (
        <div style={{ color: 'var(--cc-text-muted)' }} className="text-xs">{subtitle}</div>
      )}
      {children}
    </>
  );

  const className = `rounded-xl p-4 flex flex-col gap-3 ${
    href ? 'cursor-pointer hover:opacity-90 transition-opacity focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2' : ''
  }`;
  const style = {
    background: 'var(--cc-surface)',
    border: '1px solid var(--cc-border)',
    ...(href ? { outlineColor: 'var(--cc-accent-blue)' } : {}),
  };

  if (href) {
    return (
      <Link to={href} className={className} style={style} aria-label={`${title} — перейти`}>
        {body}
      </Link>
    );
  }
  return (
    <div className={className} style={style}>
      {body}
    </div>
  );
}

// ── Sparkline ─────────────────────────────────────────────────────────

function Sparkline({ data }: { data: number[] }) {
  const chartData = data.map((v, i) => ({ i, v }));
  return (
    <ResponsiveContainer width="100%" height={40}>
      <LineChart data={chartData} margin={{ top: 4, bottom: 4, left: 0, right: 0 }}>
        <Line
          type="monotone"
          dataKey="v"
          stroke="var(--cc-accent-blue)"
          strokeWidth={1.5}
          dot={false}
          isAnimationActive={false}
        />
        <Tooltip
          contentStyle={{ background: 'var(--cc-surface-2)', border: 'none', borderRadius: 4, fontSize: 11 }}
          labelFormatter={() => ''}
          formatter={(v) => [v ?? 0, 'msg']}
        />
      </LineChart>
    </ResponsiveContainer>
  );
}

// ── Progress Bar ──────────────────────────────────────────────────────

function ProgressBar({ value, max, color }: { value: number; max: number; color: string }) {
  const pct = max > 0 ? Math.min(100, (value / max) * 100) : 0;
  return (
    <div style={{ background: 'var(--cc-surface-2)' }} className="w-full h-1.5 rounded-full overflow-hidden">
      <div style={{ width: `${pct}%`, background: color }} className="h-full rounded-full transition-all duration-500" />
    </div>
  );
}

// ── Status Badge (dark-theme) ─────────────────────────────────────────

function StatusBadgeDark({ status }: { status: string }) {
  const map: Record<string, { bg: string; text: string }> = {
    delivered: { bg: '#14532d', text: 'var(--cc-accent-green)' },
    sent:      { bg: '#1e3a5f', text: 'var(--cc-accent-blue)' },
    failed:    { bg: '#450a0a', text: 'var(--cc-accent-red)' },
    pending:   { bg: '#334155', text: 'var(--cc-text-muted)' },
    expired:   { bg: '#431407', text: '#f97316' },
  };
  const s = map[status] ?? map.pending;
  return (
    <span
      className="px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase"
      style={{ background: s.bg, color: s.text }}
    >
      {status}
    </span>
  );
}

// ── Health Map ────────────────────────────────────────────────────────

function HealthMap({ providers, isSubAccount }: { providers: ProviderHealth[]; isSubAccount: boolean }) {
  return (
    <div
      style={{ background: 'var(--cc-surface)', border: '1px solid var(--cc-border)' }}
      className="rounded-xl p-4 flex flex-col gap-3"
    >
      <h3 style={{ color: 'var(--cc-text)' }} className="text-sm font-semibold">
        Провайдеры
      </h3>
      <div className="flex flex-col gap-2">
        {providers.length === 0 && (
          isSubAccount ? (
            <p style={{ color: 'var(--cc-text-muted)' }} className="text-xs leading-relaxed">
              Сообщения отправляются через инфраструктуру агрегатора. Здоровье провайдеров доступно агрегатору.
            </p>
          ) : (
            <p style={{ color: 'var(--cc-text-muted)' }} className="text-xs">Нет провайдеров</p>
          )
        )}
        {providers.map((p) => (
          <div
            key={p.id}
            style={{
              background: 'var(--cc-surface-2)',
              border: `1px solid ${p.is_degraded ? 'var(--cc-accent-red)' : 'var(--cc-border)'}`,
            }}
            className="rounded-lg p-2.5"
          >
            <div className="flex items-center justify-between mb-1">
              <span style={{ color: 'var(--cc-text)' }} className="text-xs font-medium">{p.name}</span>
              <span
                className="text-xs font-bold tabular-nums"
                style={{ color: p.success_rate >= 95 ? 'var(--cc-accent-green)' : p.success_rate >= 85 ? 'var(--cc-accent-yellow)' : 'var(--cc-accent-red)' }}
              >
                {p.success_rate.toFixed(1)}%
              </span>
            </div>
            <div className="flex items-center justify-between">
              <span style={{ color: 'var(--cc-text-muted)' }} className="text-[10px]">
                {p.connection_type} · {p.msg_per_sec.toFixed(0)} msg/s
              </span>
              {p.is_degraded && (
                <span style={{ color: 'var(--cc-accent-red)' }} className="text-[10px] font-semibold">
                  ⚠ деградация
                </span>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Smart Alerts ──────────────────────────────────────────────────────

const ALERT_COLORS: Record<string, string> = {
  critical: 'var(--cc-accent-red)',
  warning:  'var(--cc-accent-yellow)',
  success:  'var(--cc-accent-green)',
  info:     'var(--cc-accent-blue)',
};

function SmartAlerts({ alerts }: { alerts: AlertItem[] }) {
  return (
    <div
      style={{ background: 'var(--cc-surface)', border: '1px solid var(--cc-border)' }}
      className="rounded-xl p-4 flex flex-col gap-3"
    >
      <div className="flex items-center justify-between">
        <h3 style={{ color: 'var(--cc-text)' }} className="text-sm font-semibold">Оповещения</h3>
        <Link to="/notifications" style={{ color: 'var(--cc-accent-blue)' }} className="text-xs hover:underline">
          Все →
        </Link>
      </div>
      <div className="flex flex-col gap-2">
        {alerts.length === 0 && (
          <p style={{ color: 'var(--cc-text-muted)' }} className="text-xs">✓ Нет активных оповещений</p>
        )}
        {alerts.slice(0, 5).map((a) => (
          <div
            key={a.id}
            className="flex gap-2 rounded-lg p-2"
            style={{ background: 'var(--cc-surface-2)', borderLeft: `3px solid ${ALERT_COLORS[a.type] ?? 'var(--cc-accent-blue)'}` }}
          >
            <div className="flex-1 min-w-0">
              <div style={{ color: 'var(--cc-text)' }} className="text-xs font-medium truncate">{a.title}</div>
              <div style={{ color: 'var(--cc-text-muted)' }} className="text-[10px] truncate">{a.description}</div>
              <div style={{ color: 'var(--cc-text-muted)' }} className="text-[10px] mt-0.5 tabular-nums">
                <RelativeTime date={a.created_at} />
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Active Campaigns ──────────────────────────────────────────────────

function ActiveCampaigns({ campaigns }: { campaigns: DashboardMetrics['active_campaigns'] }) {
  return (
    <div
      style={{ background: 'var(--cc-surface)', border: '1px solid var(--cc-border)' }}
      className="rounded-xl p-4 flex flex-col gap-3"
    >
      <div className="flex items-center justify-between">
        <h3 style={{ color: 'var(--cc-text)' }} className="text-sm font-semibold">Активные кампании</h3>
        <Link to="/campaigns/new" style={{ color: 'var(--cc-accent-blue)' }} className="text-xs hover:underline">
          + Новая →
        </Link>
      </div>
      {campaigns.length === 0 && (
        <p style={{ color: 'var(--cc-text-muted)' }} className="text-xs">Нет активных кампаний</p>
      )}
      {campaigns.map((c) => (
        <div key={c.id}>
          <div className="flex items-center justify-between mb-1">
            <Link
              to={`/campaigns/${c.id}`}
              style={{ color: 'var(--cc-text)' }}
              className="text-xs font-medium hover:underline truncate max-w-[60%]"
            >
              {c.name}
            </Link>
            <span style={{ color: 'var(--cc-text-muted)' }} className="text-[10px] tabular-nums">
              {c.sent.toLocaleString()} / {c.total.toLocaleString()}
            </span>
          </div>
          <ProgressBar
            value={c.sent}
            max={c.total}
            color={c.delivery_rate >= 95 ? 'var(--cc-accent-green)' : c.delivery_rate >= 85 ? 'var(--cc-accent-yellow)' : 'var(--cc-accent-red)'}
          />
          <div className="flex items-center justify-between mt-0.5">
            <span style={{ color: 'var(--cc-text-muted)' }} className="text-[10px]">
              {c.delivery_rate.toFixed(1)}% доставлено
            </span>
            {c.eta_minutes > 0 && (
              <span style={{ color: 'var(--cc-text-muted)' }} className="text-[10px]">
                ~{c.eta_minutes} мин
              </span>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}

// ── Live Feed ─────────────────────────────────────────────────────────

function LiveFeed({
  messages,
  msgPerSec,
  isPaused,
  onPause,
  onResume,
  status,
}: {
  messages: LiveMessageEvent[];
  msgPerSec: number;
  isPaused: boolean;
  onPause: () => void;
  onResume: () => void;
  status: string;
}) {
  const [count5min, setCount5min] = useState(0);
  const countRef = useRef(0);
  const prevFirstIdRef = useRef<string | undefined>(undefined);

  // Stable 5-minute reset interval
  useEffect(() => {
    const interval = setInterval(() => {
      setCount5min(countRef.current);
      countRef.current = 0;
    }, 300_000);
    return () => clearInterval(interval);
  }, []);

  // Count newly prepended messages
  useEffect(() => {
    if (messages.length === 0) return;
    const firstId = messages[0].message_id;
    if (firstId === prevFirstIdRef.current) return;
    const newCount = messages.findIndex(m => m.message_id === prevFirstIdRef.current);
    countRef.current += newCount === -1 ? messages.length : newCount;
    prevFirstIdRef.current = firstId;
  }, [messages]);

  return (
    <div
      style={{ background: 'var(--cc-surface)', border: '1px solid var(--cc-border)' }}
      className="rounded-xl p-4 flex flex-col gap-3"
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h3 style={{ color: 'var(--cc-text)' }} className="text-sm font-semibold">Live Feed</h3>
          <span
            className="w-2 h-2 rounded-full"
            style={{ background: status === 'open' ? 'var(--cc-accent-green)' : 'var(--cc-accent-red)' }}
            title={status}
          />
        </div>
        <div className="flex items-center gap-3">
          <span style={{ color: 'var(--cc-text-muted)' }} className="text-[10px] tabular-nums">
            {msgPerSec.toFixed(0)} msg/s · {count5min.toLocaleString()} за 5 мин
          </span>
          <button
            onClick={isPaused ? onResume : onPause}
            style={{
              background: 'var(--cc-surface-2)',
              color: 'var(--cc-text)',
              border: '1px solid var(--cc-border)',
            }}
            className="text-[10px] px-2 py-0.5 rounded hover:opacity-80 transition-opacity"
          >
            {isPaused ? '▶ Продолжить' : '⏸ Пауза'}
          </button>
        </div>
      </div>

      <div className="overflow-hidden" style={{ maxHeight: 260 }}>
        <table className="w-full text-[11px]" aria-label="Живая лента сообщений">
          <tbody>
            {messages.slice(0, 20).map((m) => (
              <tr
                key={m.message_id + m.timestamp}
                style={{ borderBottom: '1px solid var(--cc-border)' }}
                className="hover:opacity-80"
              >
                <td className="py-1 pr-2 tabular-nums" style={{ color: 'var(--cc-text-muted)', whiteSpace: 'nowrap' }}>
                  {new Date(m.timestamp).toLocaleTimeString('ru', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                </td>
                <td className="py-1 pr-2">
                  <StatusBadgeDark status={m.status} />
                </td>
                <td className="py-1 pr-2 tabular-nums" style={{ color: 'var(--cc-text)' }}>
                  {m.phone_masked || '—'}
                </td>
                <td className="py-1 pr-2" style={{ color: 'var(--cc-text-muted)' }}>
                  {m.operator ? `${m.operator} → ${m.provider}` : m.provider}
                </td>
                <td className="py-1 truncate max-w-[120px]" style={{ color: 'var(--cc-text-muted)' }}>
                  {m.text_fragment}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {messages.length === 0 && (
          <p
            role="status"
            style={{
              color: status === 'error'
                ? 'var(--cc-accent-red)'
                : status === 'closed'
                ? 'var(--cc-accent-yellow)'
                : 'var(--cc-text-muted)',
            }}
            className="text-xs text-center py-4"
          >
            {status === 'open' && 'Ожидание сообщений...'}
            {status === 'connecting' && 'Подключение к live-ленте...'}
            {status === 'closed' && 'Соединение потеряно. Переподключение...'}
            {status === 'error' && 'Ошибка подключения. Переподключение...'}
          </p>
        )}
      </div>
    </div>
  );
}

// ── Command Center (main component) ──────────────────────────────────

interface SubAccountSummary {
  count: number;
  activeCount: number;
  maxSubAccounts: number;
  totalBalance: number;
  lowBalanceCount: number;
}

export function CommandCenter() {
  usePageTitle('Командный центр');
  const { user, isAuthenticated } = useAuth();
  const isReseller = !!user?.is_reseller;
  const isSubAccount = !!user?.parent_client_id;

  const [metrics, setMetrics]   = useState<DashboardMetrics | null>(null);
  const [metricsLoading, setMetricsLoading] = useState(true);
  const [providers, setProviders] = useState<ProviderHealth[]>([]);
  const [alerts, setAlerts]     = useState<AlertItem[]>([]);
  const [subSummary, setSubSummary] = useState<SubAccountSummary | null>(null);
  const [charts, setCharts] = useState<DashboardChartData | null>(null);

  const fetchMetrics   = useCallback(() => {
    setMetricsLoading(true);
    return commandCenterApi.getMetrics().then(setMetrics).catch(() => {}).finally(() => setMetricsLoading(false));
  }, []);
  const fetchProviders = useCallback(() => commandCenterApi.getProviderHealth().then(r => setProviders(r.providers)).catch(() => {}), []);
  const fetchAlerts    = useCallback(() => commandCenterApi.getAlerts().then(r => setAlerts(r.items)).catch(() => {}), []);
  const fetchCharts    = useCallback(() => dashboardApi.get().then((d) => setCharts(d.charts ?? null)).catch(() => {}), []);

  useEffect(() => {
    if (!isReseller) return;
    subAccountsApi.list().then((resp) => {
      const data = resp as {
        sub_accounts: Array<{ active: boolean; balance?: string }>;
        max_sub_accounts?: number;
      };
      const subs = data.sub_accounts ?? [];
      const totalBalance = subs.reduce((sum, s) => sum + parseFloat(s.balance ?? '0'), 0);
      const lowBalanceCount = subs.filter(s => parseFloat(s.balance ?? '0') < 100).length;
      setSubSummary({
        count: subs.length,
        activeCount: subs.filter(s => s.active).length,
        maxSubAccounts: data.max_sub_accounts ?? 0,
        totalBalance,
        lowBalanceCount,
      });
    }).catch(() => {});
  }, [isReseller]);

  usePolling(fetchMetrics,   15_000);
  usePolling(fetchProviders, 30_000);
  usePolling(fetchAlerts,    30_000);
  usePolling(fetchCharts,    60_000);

  const wsUrl = commandCenterApi.getLiveFeedUrl();
  const {
    status: wsStatus,
    messages: liveMessages,
    isPaused,
    pause,
    resume,
  } = useWebSocket<LiveMessageEvent>({ url: wsUrl, enabled: isAuthenticated, bufferSize: 100 });

  const degradedCount = providers.filter(p => p.is_degraded).length;
  const systemStatus = degradedCount === 0
    ? '● Все системы в норме'
    : `⚠ ${degradedCount} провайдер${degradedCount === 1 ? '' : 'а'} деградирует`;

  const balanceFormatted = metrics
    ? `${parseFloat(metrics.balance).toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ${metrics.currency}`
    : '—';

  const balanceForecast = metrics && parseFloat(metrics.burn_rate_per_hour) > 0
    ? `₽${parseFloat(metrics.burn_rate_per_hour).toFixed(2)}/ч · хватит на ${metrics.forecast_hours}ч`
    : metrics
    ? 'Расходов нет'
    : '—';

  return (
    <div
      className="theme-dark min-h-screen p-4 md:p-6"
      style={{ background: 'var(--cc-bg)' }}
    >
      {/* Page-level status chip (title and balance/bell now live in AppHeader) */}
      <div className="mb-6">
        <span
          className="text-xs"
          style={{ color: degradedCount === 0 ? 'var(--cc-accent-green)' : 'var(--cc-accent-yellow)' }}
        >
          {systemStatus}
        </span>
      </div>

      {/* Row 1: KPI Cards */}
      {metricsLoading && !metrics ? (
        <div
          role="status"
          aria-label="Загрузка метрик"
          className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mb-4"
        >
          {[0, 1, 2].map((i) => (
            <div
              key={i}
              style={{ background: 'var(--cc-surface)', border: '1px solid var(--cc-border)' }}
              className="rounded-xl p-4 h-28 animate-pulse"
            />
          ))}
        </div>
      ) : (
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mb-4">
        <KpiCard
          title="Скорость (msg/s)"
          value={metrics ? `${metrics.msg_per_sec.toFixed(0)} msg/s` : '—'}
          trend={metrics?.msg_per_sec_trend_pct}
          subtitle="сообщений в секунду за последний час"
          href="/analytics"
        >
          {metrics && <Sparkline data={metrics.sparkline_1h} />}
        </KpiCard>

        <KpiCard
          title="Доставлено за 24ч (%)"
          value={metrics ? `${metrics.delivery_rate_24h}%` : '—'}
          trend={metrics?.delivery_rate_trend_pct}
          subtitle="доля успешно доставленных сообщений"
          href="/analytics"
        >
          {metrics && (
            <ProgressBar
              value={metrics.delivery_rate_24h}
              max={100}
              color={
                metrics.delivery_rate_24h >= 95
                  ? 'var(--cc-accent-green)'
                  : metrics.delivery_rate_24h >= 85
                  ? 'var(--cc-accent-yellow)'
                  : 'var(--cc-accent-red)'
              }
            />
          )}
        </KpiCard>

        <KpiCard
          title="Баланс"
          value={balanceFormatted}
          subtitle={balanceForecast}
          href="/billing"
        >
          {metrics && (
            <ProgressBar
              value={Math.min(metrics.forecast_hours, 72)}
              max={72}
              color={
                metrics.forecast_hours >= 48
                  ? 'var(--cc-accent-green)'
                  : metrics.forecast_hours >= 12
                  ? 'var(--cc-accent-yellow)'
                  : 'var(--cc-accent-red)'
              }
            />
          )}
        </KpiCard>
      </div>
      )}

      {/* Row 1b: Sub-accounts summary (resellers only) */}
      {isReseller && subSummary && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mb-4">
          <KpiCard
            title="Суб-аккаунты"
            value={
              subSummary.maxSubAccounts > 0
                ? `${subSummary.activeCount} / ${subSummary.maxSubAccounts}`
                : `${subSummary.activeCount}`
            }
            subtitle={
              subSummary.maxSubAccounts > 0
                ? `активных из ${subSummary.maxSubAccounts} по лимиту`
                : 'активных суб-аккаунтов'
            }
            href="/network/sub-accounts"
          >
            <ProgressBar
              value={subSummary.activeCount}
              max={subSummary.maxSubAccounts || subSummary.count || 1}
              color="var(--cc-accent-blue)"
            />
          </KpiCard>
          <KpiCard
            title="Суммарный баланс"
            value={`${subSummary.totalBalance.toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ₽`}
            subtitle="сумма балансов всех суб-аккаунтов"
            href="/network/sub-accounts"
          />
          <KpiCard
            title="Низкий баланс"
            value={subSummary.lowBalanceCount}
            subtitle="суб-аккаунтов с балансом < 100 ₽"
            href="/network/sub-accounts?filter=low"
          />
        </div>
      )}

      {/* Row 2: Live Feed (2/3) + Health Map (1/3) */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 mb-4">
        <div className="lg:col-span-2">
          <LiveFeed
            messages={liveMessages}
            msgPerSec={metrics?.msg_per_sec ?? 0}
            isPaused={isPaused}
            onPause={pause}
            onResume={resume}
            status={wsStatus}
          />
        </div>
        <HealthMap providers={providers} isSubAccount={isSubAccount} />
      </div>

      {/* Row 3: Smart Alerts (1/3) + Active Campaigns (2/3) */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <SmartAlerts alerts={alerts} />
        <div className="lg:col-span-2">
          <ActiveCampaigns campaigns={metrics?.active_campaigns ?? []} />
        </div>
      </div>

      {/* Row 4: Status distribution (1/3) + Top countries (2/3) — spec 019 FR-002 */}
      {charts && (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 mt-4">
          <StatusDistributionCard buckets={charts.status_distribution ?? []} />
          <div className="lg:col-span-2">
            <TopCountriesCard buckets={charts.top_countries ?? []} />
          </div>
        </div>
      )}
    </div>
  );
}

// ── Status distribution donut + Top countries bar (spec 019) ─────────

const STATUS_COLORS_DASHBOARD: Record<string, string> = {
  delivered: 'var(--cc-accent-green)',
  failed:    'var(--cc-accent-red)',
  pending:   'var(--cc-accent-yellow)',
  expired:   '#f97316',
  rejected:  'var(--cc-accent-red)',
};

function StatusDistributionCard({ buckets }: { buckets: Array<{ status: string; count: number; label: string }> }) {
  const data = buckets.filter((b) => b.count > 0);
  return (
    <div
      style={{ background: 'var(--cc-surface)', border: '1px solid var(--cc-border)' }}
      className="rounded-xl p-4 flex flex-col gap-3"
    >
      <h3 style={{ color: 'var(--cc-text)' }} className="text-sm font-semibold">Распределение статусов (7 дн.)</h3>
      {data.length === 0 ? (
        <p style={{ color: 'var(--cc-text-muted)' }} className="text-xs">Нет данных</p>
      ) : (
        <ResponsiveContainer width="100%" height={200}>
          <PieChart>
            <Pie data={data} dataKey="count" nameKey="label" innerRadius={45} outerRadius={75} paddingAngle={2}>
              {data.map((b, i) => (
                <Cell key={i} fill={STATUS_COLORS_DASHBOARD[b.status] ?? 'var(--cc-accent-blue)'} />
              ))}
            </Pie>
            <Tooltip
              contentStyle={{ background: 'var(--cc-surface-2)', border: 'none', borderRadius: 4, fontSize: 11 }}
              formatter={(value, _name, item) => [
                Number(value).toLocaleString(),
                (item?.payload as { label?: string } | undefined)?.label ?? '',
              ]}
            />
          </PieChart>
        </ResponsiveContainer>
      )}
    </div>
  );
}

function TopCountriesCard({ buckets }: { buckets: Array<{ country: string; count: number }> }) {
  return (
    <div
      style={{ background: 'var(--cc-surface)', border: '1px solid var(--cc-border)' }}
      className="rounded-xl p-4 flex flex-col gap-3"
    >
      <h3 style={{ color: 'var(--cc-text)' }} className="text-sm font-semibold">Топ-5 стран по трафику (7 дн.)</h3>
      {buckets.length === 0 ? (
        <p style={{ color: 'var(--cc-text-muted)' }} className="text-xs">Нет данных</p>
      ) : (
        <ResponsiveContainer width="100%" height={200}>
          <BarChart data={buckets} layout="vertical" margin={{ top: 4, right: 12, bottom: 4, left: 12 }}>
            <XAxis type="number" tick={{ fontSize: 10, fill: 'var(--cc-text-muted)' }} />
            <YAxis dataKey="country" type="category" tick={{ fontSize: 11, fill: 'var(--cc-text)' }} width={80} />
            <Tooltip
              contentStyle={{ background: 'var(--cc-surface-2)', border: 'none', borderRadius: 4, fontSize: 11 }}
              formatter={(value) => [Number(value).toLocaleString(), 'Отправлено']}
            />
            <Bar dataKey="count" fill="var(--cc-accent-blue)" radius={[0, 4, 4, 0]} />
          </BarChart>
        </ResponsiveContainer>
      )}
    </div>
  );
}
