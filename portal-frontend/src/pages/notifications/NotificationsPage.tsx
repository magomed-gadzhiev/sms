// portal-frontend/src/pages/notifications/NotificationsPage.tsx
import { useAuth } from '../../contexts/AuthContext';
import { useNotifications } from '../../hooks/useNotifications';
import { PageHeader } from '../../components/layout/PageHeader';
import { usePageTitle } from '../../contexts/PageTitleContext';
import { Badge } from '../../components/ui/Badge';

const TYPE_LABELS: Record<string, string> = {
  info: 'Инфо',
  warning: 'Предупреждение',
  critical: 'Критическое',
  success: 'Успех',
  low_balance: 'Низкий баланс',
  provider_degraded: 'Проблемы провайдера',
  campaign_completed: 'Кампания завершена',
  template_approved: 'Шаблон одобрен',
  template_rejected: 'Шаблон отклонён',
};

const TYPE_VARIANTS: Record<string, 'default' | 'info' | 'success' | 'warning' | 'danger'> = {
  info: 'info',
  warning: 'warning',
  critical: 'danger',
  success: 'success',
  low_balance: 'warning',
  provider_degraded: 'danger',
  campaign_completed: 'success',
  template_approved: 'success',
  template_rejected: 'danger',
};

export function NotificationsPage() {
  usePageTitle('Уведомления');
  const { isAuthenticated } = useAuth();
  const { items, unreadCount, markRead, markAllRead } = useNotifications(isAuthenticated);

  return (
    <div>
      <PageHeader
        subtitle={unreadCount > 0 ? `${unreadCount} непрочитанных` : 'Все прочитаны'}
        actions={
          unreadCount > 0 ? (
            <button
              onClick={markAllRead}
              className="text-sm text-primary hover:underline"
            >
              Прочитать все
            </button>
          ) : undefined
        }
      />

      <div className="max-w-2xl">
        {items.length === 0 && (
          <div className="text-center py-16 text-gray-500 dark:text-slate-400 text-sm">
            Нет уведомлений
          </div>
        )}
        <ul className="divide-y divide-gray-100 dark:divide-slate-800">
          {items.map((item) => (
            <li
              key={item.id}
              className={`flex items-start gap-3 py-3 px-2 rounded-lg transition-colors ${
                !item.is_read ? 'bg-blue-50 dark:bg-blue-950/40' : 'hover:bg-gray-50 dark:hover:bg-slate-800'
              }`}
            >
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 mb-0.5">
                  <Badge variant={TYPE_VARIANTS[item.type] ?? 'default'}>
                    {TYPE_LABELS[item.type] ?? item.type}
                  </Badge>
                  <span className="text-xs text-gray-400 dark:text-slate-500 tabular-nums">
                    {new Date(item.created_at).toLocaleString('ru', {
                      day: '2-digit', month: '2-digit', year: 'numeric',
                      hour: '2-digit', minute: '2-digit',
                    })}
                  </span>
                </div>
                <p className="text-sm text-gray-700 dark:text-slate-300">{item.body}</p>
              </div>
              {!item.is_read && (
                <button
                  onClick={() => markRead(item.id)}
                  className="shrink-0 text-xs text-gray-400 dark:text-slate-500 hover:text-gray-700 dark:hover:text-slate-200"
                  aria-label="Отметить как прочитанное"
                >
                  ✓
                </button>
              )}
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
