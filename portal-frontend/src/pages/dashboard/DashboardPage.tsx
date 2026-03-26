import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { dashboardApi } from '../../api/client';

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
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    dashboardApi
      .get()
      .then((resp) => setData(resp as DashboardData))
      .catch((err) => setError(err.message || 'Failed to load dashboard'))
      .finally(() => setLoading(false));
  }, []);

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
