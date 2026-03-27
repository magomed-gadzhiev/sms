import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { dashboardApi, profileApi, ProfileData } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';

interface DashboardData {
  balance: string;
  currency: string;
  messages_today: number;
  messages_delivered_today: number;
  delivery_rate_today: number;
  active_api_keys: number;
  active_webhooks: number;
}

export function DashboardPage() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [profile, setProfile] = useState<ProfileData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [sandboxToggling, setSandboxToggling] = useState(false);

  useEffect(() => {
    Promise.all([
      dashboardApi.get().then((resp) => setData(resp as DashboardData)),
      profileApi.get().then((resp) => setProfile(resp)),
    ])
      .catch((err) => setError(err.message || 'Failed to load dashboard'))
      .finally(() => setLoading(false));
  }, []);

  const handleDisableSandbox = async () => {
    setSandboxToggling(true);
    try {
      const resp = await profileApi.toggleSandbox(false);
      setProfile((prev) => prev ? { ...prev, is_sandbox: resp.is_sandbox } : prev);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Ошибка переключения режима';
      setError(msg);
    } finally {
      setSandboxToggling(false);
    }
  };

  if (loading) return <div role="status">Loading dashboard...</div>;
  if (error) return <div className="text-red-600">Error: {error}</div>;
  if (!data) return <div>No data</div>;

  const cards: { label: string; value: string | number; href?: string }[] = [
    { label: 'Balance', value: `${data.balance} ${data.currency}` },
    { label: 'Messages Today', value: data.messages_today, href: '/messages' },
    { label: 'Delivered Today', value: data.messages_delivered_today, href: '/messages' },
    { label: 'Delivery Rate', value: `${data.delivery_rate_today}%` },
    { label: 'Active API Keys', value: data.active_api_keys, href: '/api-keys' },
    { label: 'Active Webhooks', value: data.active_webhooks, href: '/webhooks' },
  ];

  return (
    <div>
      {profile?.is_sandbox && (
        <div
          role="alert"
          className="bg-amber-50 border border-amber-400 rounded-lg px-4 py-3 mb-4 flex items-center justify-between gap-4"
        >
          <span className="font-medium">Sandbox Mode — SMS не отправляются реально</span>
          <Button
            variant="secondary"
            onClick={handleDisableSandbox}
            disabled={sandboxToggling}
          >
            {sandboxToggling ? 'Переключение...' : 'Перейти в Production'}
          </Button>
        </div>
      )}
      <PageHeader title="Dashboard" />
      <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
        {cards.map((card) => {
          const isInteractive = !!card.href;
          const cardClassName = `border border-gray-200 rounded-lg p-4 text-center block no-underline text-inherit ${
            isInteractive ? 'hover:bg-gray-50 transition-colors group relative pb-10' : ''
          }`;
          
          const inner = (
            <>
              <div className="text-sm text-gray-500 mb-2">{card.label}</div>
              <div className="text-2xl font-bold">{card.value}</div>
              {isInteractive && (
                <div className="absolute bottom-4 left-0 right-0 text-sm text-primary font-medium group-hover:underline">
                  View details &rarr;
                </div>
              )}
            </>
          );

          return card.href ? (
            <Link key={card.label} to={card.href} className={cardClassName} aria-label={`${card.label}: ${card.value}`}>
              {inner}
            </Link>
          ) : (
            <div key={card.label} className={cardClassName}>
              {inner}
            </div>
          );
        })}
      </div>
    </div>
  );
}
