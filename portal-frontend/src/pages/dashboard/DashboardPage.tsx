import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { dashboardApi, profileApi } from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';
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

const ZERO_DASHBOARD: DashboardData = {
  balance: '0',
  currency: 'RUB',
  messages_today: 0,
  messages_delivered_today: 0,
  delivery_rate_today: 0,
  active_api_keys: 0,
  active_webhooks: 0,
};

export function DashboardPage() {
  const { user: profile, refreshUser } = useAuth();
  const [data, setData] = useState<DashboardData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [sandboxToggling, setSandboxToggling] = useState(false);

  useEffect(() => {
    dashboardApi
      .get()
      .then((resp) => setData(resp as DashboardData))
      .catch(() => setData(ZERO_DASHBOARD))
      .finally(() => setLoading(false));
  }, []);

  const handleDisableSandbox = async () => {
    setSandboxToggling(true);
    try {
      await profileApi.toggleSandbox(false);
      await refreshUser();
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Ошибка переключения режима';
      setError(msg);
    } finally {
      setSandboxToggling(false);
    }
  };

  if (loading) return <div role="status">Загрузка дашборда...</div>;
  if (!data) return null;

  const formattedBalance = new Intl.NumberFormat('ru-RU', {
    style: 'currency', currency: 'RUB', minimumFractionDigits: 2, maximumFractionDigits: 2,
  }).format(parseFloat(data.balance) || 0);

  const cards: { label: string; value: string | number; href?: string }[] = [
    { label: 'Баланс', value: formattedBalance },
    { label: 'Сообщений сегодня', value: data.messages_today, href: '/messages' },
    { label: 'Доставлено сегодня', value: data.messages_delivered_today, href: '/messages' },
    { label: 'Доставляемость', value: `${data.delivery_rate_today}%` },
    { label: 'Активных API ключей', value: data.active_api_keys, href: '/api-keys' },
    { label: 'Активных вебхуков', value: data.active_webhooks, href: '/webhooks' },
  ];

  return (
    <div>
      {error && <div className="text-red-600 mb-4">{error}</div>}
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
      <PageHeader title="Дашборд" />
      {data.messages_today === 0 && (data.active_api_keys === 0 || data.active_webhooks === 0) && (
        <div className="border border-primary/30 bg-primary/5 rounded-lg p-5 mb-6">
          <h3 className="font-semibold text-gray-900 mb-3">Начните работу с платформой</h3>
          <ol className="list-decimal list-inside space-y-1 text-sm text-gray-700">
            <li><a href="/api-keys" className="text-primary hover:underline">Создайте API ключ</a> для доступа к API</li>
            <li><a href="/providers" className="text-primary hover:underline">Подключите SMPP-провайдера</a> для отправки SMS</li>
            <li><a href="/messages" className="text-primary hover:underline">Отправьте тестовое SMS</a> и отслеживайте статистику</li>
          </ol>
        </div>
      )}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
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
                  Подробнее &rarr;
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
