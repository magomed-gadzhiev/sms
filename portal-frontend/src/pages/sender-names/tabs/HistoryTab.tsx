import { useState, useEffect } from 'react';
import { senderNamesApi, ApiError } from '../../../api/client';
import type { SenderNameInfo, SenderNameHistoryEntry } from '../../../api/client';

const STATUS_LABEL: Record<string, string> = {
  pending: 'На модерации',
  approved: 'Одобрено',
  rejected: 'Отклонено',
  deactivated: 'Деактивировано',
};

function labelActor(t: string): string {
  return ({ client: 'Клиент', admin: 'Администратор', system: 'Система' } as Record<string, string>)[t] ?? t;
}

export function HistoryTab({ senderName }: { senderName: SenderNameInfo }) {
  const [items, setItems] = useState<SenderNameHistoryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError('');
    senderNamesApi
      .getHistory(senderName.id)
      .then((res) => {
        if (!cancelled) setItems(res.entries ?? []);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(e instanceof ApiError ? e.message : 'Ошибка загрузки');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [senderName.id]);

  if (loading) {
    return (
      <div role="status" className="py-8 text-center text-gray-500">
        Загрузка...
      </div>
    );
  }

  if (error) {
    return <p className="text-red-600">{error}</p>;
  }

  if (items.length === 0) {
    return (
      <div className="text-center py-12 text-gray-500">
        История изменений пока пуста
      </div>
    );
  }

  const sorted = [...items].sort(
    (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
  );

  return (
    <ol className="space-y-4">
      {sorted.map((item) => (
        <li key={item.id} className="border-l-2 border-gray-200 pl-4 pb-4">
          <div className="text-sm font-medium text-gray-700">
            {item.old_status && STATUS_LABEL[item.old_status]
              ? `${STATUS_LABEL[item.old_status]} → `
              : ''}
            {STATUS_LABEL[item.new_status] ?? item.new_status}
          </div>
          <div className="text-xs text-gray-500 mt-1">
            {new Date(item.created_at).toLocaleString('ru-RU')}
            {item.actor_type && ` · ${labelActor(item.actor_type)}`}
          </div>
          {item.comment && (
            <p className="text-sm text-gray-600 mt-2 bg-gray-50 rounded p-2 border border-gray-100">
              {item.comment}
            </p>
          )}
        </li>
      ))}
    </ol>
  );
}
