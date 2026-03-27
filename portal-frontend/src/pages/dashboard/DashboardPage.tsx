import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { dashboardApi, profileApi, ProfileData } from '../../api/client';

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
  if (error) return <div style={{ color: 'red' }}>Error: {error}</div>;
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
          style={{
            background: '#fff3cd',
            border: '1px solid #ffc107',
            borderRadius: 8,
            padding: '12px 16px',
            marginBottom: 16,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 16,
          }}
        >
          <span style={{ fontWeight: 500 }}>Sandbox Mode — SMS не отправляются реально</span>
          <button
            onClick={handleDisableSandbox}
            disabled={sandboxToggling}
            style={{
              background: '#ffc107',
              border: 'none',
              borderRadius: 6,
              padding: '6px 14px',
              cursor: sandboxToggling ? 'not-allowed' : 'pointer',
              fontWeight: 500,
              whiteSpace: 'nowrap',
            }}
          >
            {sandboxToggling ? 'Переключение...' : 'Перейти в Production'}
          </button>
        </div>
      )}
      <h2>Dashboard</h2>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 16 }}>
        {cards.map((card) => {
          const cardStyle = {
            border: '1px solid #ddd',
            borderRadius: 8,
            padding: 16,
            minWidth: 180,
            textAlign: 'center' as const,
            textDecoration: 'none',
            color: 'inherit',
            display: 'block',
          };
          const inner = (
            <>
              <div style={{ fontSize: 14, color: '#666', marginBottom: 8 }}>{card.label}</div>
              <div style={{ fontSize: 24, fontWeight: 'bold' }}>{card.value}</div>
            </>
          );
          return card.href ? (
            <Link key={card.label} to={card.href} style={cardStyle} aria-label={`${card.label}: ${card.value}`}>
              {inner}
            </Link>
          ) : (
            <div key={card.label} style={cardStyle}>
              {inner}
            </div>
          );
        })}
      </div>
    </div>
  );
}
