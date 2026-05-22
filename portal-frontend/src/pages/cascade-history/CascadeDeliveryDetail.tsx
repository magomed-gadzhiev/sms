import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { cascadeDeliveriesApi, type Delivery, type AttemptInfo } from '../../api/cascade';
import { Badge } from '../../components/ui/Badge';
import { Button } from '../../components/ui/Button';

const DELIVERY_STATUS_BADGE: Record<string, { variant: 'warning' | 'success' | 'danger' | 'default'; label: string }> = {
  pending:     { variant: 'warning', label: 'Ожидание' },
  in_progress: { variant: 'warning', label: 'В процессе' },
  delivered:   { variant: 'success', label: 'Доставлено' },
  failed:      { variant: 'danger',  label: 'Не доставлено' },
  cancelled:   { variant: 'default', label: 'Отменено' },
};

const ATTEMPT_STATUS_BADGE: Record<string, { variant: 'warning' | 'success' | 'danger' | 'default'; label: string }> = {
  pending:        { variant: 'default', label: 'Ожидание' },
  sent:           { variant: 'warning', label: 'Отправлен' },
  delivered:      { variant: 'success', label: 'Доставлен' },
  failed:         { variant: 'danger',  label: 'Неудача' },
  timeout:        { variant: 'danger',  label: 'Таймаут' },
  skipped:        { variant: 'default', label: 'Пропущен' },
  late_duplicate: { variant: 'default', label: 'Поздний дубль' },
};

const CHANNEL_ICONS: Record<string, string> = {
  sms: '💬',
  flash_call: '📞',
  reverse_call: '↩️',
  messenger: '📱',
};

function formatDateTime(dt?: string) {
  if (!dt) return '—';
  return new Date(dt).toLocaleString('ru-RU', {
    day: '2-digit', month: '2-digit', year: 'numeric',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  });
}

function AttemptCard({ attempt }: { attempt: AttemptInfo }) {
  const badge = ATTEMPT_STATUS_BADGE[attempt.status] ?? { variant: 'default' as const, label: attempt.status };
  const icon = CHANNEL_ICONS[attempt.channel_type] ?? '📡';

  return (
    <div className="border border-gray-200 dark:border-slate-700 rounded-lg p-4">
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <span>{icon}</span>
          <span className="font-medium text-sm">{attempt.channel_type.replace(/_/g, ' ').toUpperCase()}</span>
          <span className="text-gray-400 dark:text-slate-500 text-xs">шаг {attempt.step_order}</span>
        </div>
        <Badge variant={badge.variant}>{badge.label}</Badge>
      </div>

      <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-sm">
        <div className="text-gray-500 dark:text-slate-400">Отправлен:</div>
        <div>{formatDateTime(attempt.sent_at)}</div>
        <div className="text-gray-500 dark:text-slate-400">Результат:</div>
        <div>{formatDateTime(attempt.result_at)}</div>
        <div className="text-gray-500 dark:text-slate-400">Стоимость:</div>
        <div>{attempt.cost} {attempt.currency}</div>
        {attempt.error_message && (
          <>
            <div className="text-gray-500 dark:text-slate-400">Ошибка:</div>
            <div className="text-red-600 dark:text-red-400">{attempt.error_message}</div>
          </>
        )}
      </div>
    </div>
  );
}

export function CascadeDeliveryDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [delivery, setDelivery] = useState<Delivery | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!id) return;
    cascadeDeliveriesApi.get(id)
      .then(setDelivery)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) {
    return (
      <div className="p-6">
        <div className="animate-pulse space-y-4">
          <div className="h-8 bg-gray-200 dark:bg-slate-700 rounded w-1/3" />
          <div className="h-32 bg-gray-200 dark:bg-slate-700 rounded" />
          <div className="h-48 bg-gray-200 dark:bg-slate-700 rounded" />
        </div>
      </div>
    );
  }

  if (error || !delivery) {
    return (
      <div className="p-6">
        <div className="text-red-600 dark:text-red-400 mb-4">{error || 'Доставка не найдена'}</div>
        <Button variant="secondary" onClick={() => navigate(-1)}>Назад</Button>
      </div>
    );
  }

  const statusBadge = DELIVERY_STATUS_BADGE[delivery.status] ?? { variant: 'default' as const, label: delivery.status };
  const attempts = delivery.attempts ?? [];

  return (
    <div className="p-6 max-w-3xl">
      <div className="flex items-center gap-3 mb-6">
        <Button variant="secondary" size="sm" onClick={() => navigate(-1)}>← Назад</Button>
        <h1 className="text-xl font-semibold">Детали доставки</h1>
      </div>

      {/* Header card */}
      <div className="border border-gray-200 dark:border-slate-700 rounded-lg p-5 mb-6">
        <div className="grid grid-cols-2 gap-x-8 gap-y-3 text-sm">
          <div>
            <p className="text-gray-500 dark:text-slate-400 text-xs mb-0.5">Статус</p>
            <Badge variant={statusBadge.variant}>{statusBadge.label}</Badge>
          </div>
          <div>
            <p className="text-gray-500 dark:text-slate-400 text-xs mb-0.5">Доставлено через</p>
            <p className="font-medium">{delivery.delivered_via || '—'}</p>
          </div>
          <div>
            <p className="text-gray-500 dark:text-slate-400 text-xs mb-0.5">Получатель</p>
            <p className="font-medium">{delivery.recipient}</p>
          </div>
          <div>
            <p className="text-gray-500 dark:text-slate-400 text-xs mb-0.5">Итоговая стоимость</p>
            <p className="font-medium">{delivery.total_cost} {delivery.currency}</p>
          </div>
          <div className="col-span-2">
            <p className="text-gray-500 dark:text-slate-400 text-xs mb-0.5">ID</p>
            <p className="font-mono text-xs text-gray-600 dark:text-slate-400">{delivery.id}</p>
          </div>
          <div>
            <p className="text-gray-500 dark:text-slate-400 text-xs mb-0.5">Создано</p>
            <p>{formatDateTime(delivery.created_at)}</p>
          </div>
          <div>
            <p className="text-gray-500 dark:text-slate-400 text-xs mb-0.5">Обновлено</p>
            <p>{formatDateTime(delivery.updated_at)}</p>
          </div>
        </div>
      </div>

      {/* Attempts timeline */}
      <h2 className="text-base font-medium text-gray-700 dark:text-slate-300 mb-3">
        Попытки доставки ({attempts.length})
      </h2>
      {attempts.length === 0 ? (
        <p className="text-gray-400 dark:text-slate-500 text-sm">Нет попыток</p>
      ) : (
        <div className="space-y-3">
          {[...attempts]
            .sort((a, b) => a.step_order - b.step_order)
            .map((attempt) => (
              <AttemptCard key={attempt.id} attempt={attempt} />
            ))}
        </div>
      )}
    </div>
  );
}
