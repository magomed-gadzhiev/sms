import { useState, useEffect, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import { messagesApi, ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Badge } from '../../components/ui/Badge';
import { Button } from '../../components/ui/Button';

interface MessageDetail {
  message_id: string;
  source: string;
  destination: string;
  text: string;
  status: string;
  segment_count: number;
  created_at?: string;
  submitted_at?: string;
  delivered_at?: string;
  failed_at?: string;
  error_code?: string;
  error_message?: string;
}

const STATUS_VARIANT: Record<string, 'default' | 'info' | 'warning' | 'success' | 'danger'> = {
  pending: 'warning',
  queued: 'info',
  sent: 'info',
  submitted: 'info',
  delivered: 'success',
  failed: 'danger',
  expired: 'danger',
  rejected: 'danger',
};

const STATUS_LABEL: Record<string, string> = {
  pending: 'Ожидает',
  queued: 'В очереди',
  sent: 'Отправлено',
  submitted: 'Передано',
  delivered: 'Доставлено',
  failed: 'Ошибка',
  expired: 'Истекло',
  rejected: 'Отклонено',
};

interface TimelineStep {
  key: string;
  label: string;
  timestamp?: string;
  done: boolean;
  failed: boolean;
}

function buildTimeline(msg: MessageDetail): TimelineStep[] {
  const isFailed = ['failed', 'rejected', 'expired'].includes(msg.status);
  const isDelivered = msg.status === 'delivered';

  return [
    {
      key: 'created',
      label: 'Создано',
      timestamp: msg.created_at,
      done: true,
      failed: false,
    },
    {
      key: 'submitted',
      label: 'Отправлено',
      timestamp: msg.submitted_at,
      done: !!msg.submitted_at || isDelivered || isFailed,
      failed: false,
    },
    {
      key: 'result',
      label: isDelivered ? 'Доставлено' : isFailed ? 'Ошибка' : 'Доставка',
      timestamp: msg.delivered_at || msg.failed_at,
      done: isDelivered || isFailed,
      failed: isFailed,
    },
  ];
}

function formatTimestamp(ts?: string): string {
  if (!ts) return '--';
  return new Date(ts).toLocaleString('ru-RU');
}

export function MessageDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [message, setMessage] = useState<MessageDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const loadMessage = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError('');
    try {
      const resp = await messagesApi.get(id);
      setMessage(resp as MessageDetail);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить сообщение');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    loadMessage();
  }, [loadMessage]);

  if (loading) {
    return <div className="animate-pulse p-8 text-center text-gray-500">Загрузка...</div>;
  }

  if (error) {
    return (
      <div className="p-8">
        <p className="text-red-600 mb-4">{error}</p>
        <Link to="/messages">
          <Button variant="secondary">Назад к сообщениям</Button>
        </Link>
      </div>
    );
  }

  if (!message) return null;

  const timeline = buildTimeline(message);
  const statusVariant = STATUS_VARIANT[message.status] ?? 'default';
  const statusLabel = STATUS_LABEL[message.status] ?? message.status;

  return (
    <div className="max-w-3xl">
      <PageHeader
        title={`Сообщение #${message.message_id.substring(0, 8)}`}
        breadcrumbs={[
          { label: 'Сообщения', href: '/messages' },
          { label: `Сообщение #${message.message_id.substring(0, 8)}` },
        ]}
        actions={
          <Link to="/messages">
            <Button variant="secondary">Назад</Button>
          </Link>
        }
      />

      {/* Detail card */}
      <div className="bg-white border border-gray-200 rounded-lg p-6 mb-6">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <span className="text-sm text-gray-500">ID сообщения</span>
            <p className="font-mono text-sm mt-0.5 break-all">{message.message_id}</p>
          </div>
          <div>
            <span className="text-sm text-gray-500">Статус</span>
            <div className="mt-0.5">
              <Badge variant={statusVariant}>{statusLabel}</Badge>
            </div>
          </div>
          <div>
            <span className="text-sm text-gray-500">Отправитель</span>
            <p className="text-sm mt-0.5">{message.source || '--'}</p>
          </div>
          <div>
            <span className="text-sm text-gray-500">Получатель</span>
            <p className="text-sm mt-0.5">{message.destination}</p>
          </div>
          <div>
            <span className="text-sm text-gray-500">Сегменты</span>
            <p className="text-sm mt-0.5">{message.segment_count}</p>
          </div>
          <div>
            <span className="text-sm text-gray-500">Создано</span>
            <p className="text-sm mt-0.5">{formatTimestamp(message.created_at)}</p>
          </div>
        </div>
        {message.text && (
          <div className="mt-4 pt-4 border-t border-gray-100">
            <span className="text-sm text-gray-500">Текст сообщения</span>
            <p className="text-sm mt-1 whitespace-pre-wrap bg-gray-50 rounded p-3">{message.text}</p>
          </div>
        )}
        {message.error_message && (
          <div className="mt-4 pt-4 border-t border-gray-100">
            <span className="text-sm text-red-500">Ошибка</span>
            <p className="text-sm mt-1 text-red-700">
              {message.error_code && <span className="font-mono mr-2">[{message.error_code}]</span>}
              {message.error_message}
            </p>
          </div>
        )}
      </div>

      {/* Delivery timeline */}
      <div className="bg-white border border-gray-200 rounded-lg p-6">
        <h2 className="text-base font-semibold text-gray-900 mb-5">Статус доставки</h2>
        <div className="flex items-start">
          {timeline.map((step, i) => (
            <div key={step.key} className="flex-1 flex flex-col items-center relative">
              {/* Connector line */}
              {i > 0 && (
                <div
                  className={`absolute top-4 right-1/2 w-full h-0.5 -translate-y-1/2 ${
                    step.done
                      ? step.failed
                        ? 'bg-red-400'
                        : 'bg-green-400'
                      : 'bg-gray-200'
                  }`}
                />
              )}
              {/* Circle */}
              <div
                className={`relative z-10 w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold ${
                  step.done
                    ? step.failed
                      ? 'bg-red-100 text-red-600 ring-2 ring-red-400'
                      : 'bg-green-100 text-green-600 ring-2 ring-green-400'
                    : 'bg-gray-100 text-gray-400 ring-2 ring-gray-200'
                }`}
              >
                {step.done ? (step.failed ? '!' : '\u2713') : (i + 1)}
              </div>
              {/* Label */}
              <span
                className={`mt-2 text-xs font-medium ${
                  step.done ? (step.failed ? 'text-red-600' : 'text-green-700') : 'text-gray-400'
                }`}
              >
                {step.label}
              </span>
              {/* Timestamp */}
              <span className="mt-0.5 text-[11px] text-gray-400">
                {formatTimestamp(step.timestamp)}
              </span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
