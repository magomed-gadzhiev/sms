import { useState, useEffect, useCallback, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import { messagesApi, ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Badge } from '../../components/ui/Badge';
import { Button } from '../../components/ui/Button';
import { useMessageStream } from '../../hooks/useMessageStream';

interface DlrInfo {
  stat: string;
  err: number;
  text: string;
  submit_date?: string;
  done_date?: string;
  receipted_message_id?: string;
}

interface BillingInfo {
  segment_count: number;
  price_per_segment: string;
  total_amount: string;
  tariff_plan_id: string;
  billed_at: string;
}

interface MessageDetail {
  message_id: string;
  source: string;
  destination: string;
  text: string;
  encoding?: string;
  status: string;
  status_message?: string;
  external_id?: string;
  segment_count: number;
  retry_count?: number;
  max_retries?: number;
  provider_id?: string;
  provider_name?: string;
  route_id?: string;
  route_name?: string;
  smpp_message_id?: string;
  created_at?: string;
  submitted_at?: string;
  delivered_at?: string;
  failed_at?: string;
  scheduled_at?: string;
  expired_at?: string;
  dlr?: DlrInfo;
  billing?: BillingInfo;
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

const DLR_STAT_LABEL: Record<string, string> = {
  DELIVRD: 'Доставлено',
  UNDELIV: 'Не доставлено',
  EXPIRED: 'Истекло',
  REJECTD: 'Отклонено',
  ACCEPTD: 'Принято',
  DELETED: 'Удалено',
  UNKNOWN: 'Неизвестно',
};

const DLR_STAT_VARIANT: Record<string, 'success' | 'danger' | 'warning' | 'default'> = {
  DELIVRD: 'success',
  UNDELIV: 'danger',
  EXPIRED: 'danger',
  REJECTD: 'danger',
  ACCEPTD: 'warning',
  DELETED: 'warning',
  UNKNOWN: 'default',
};

const TERMINAL_STATUSES = new Set(['delivered', 'failed', 'expired', 'rejected']);

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

  // SSE: subscribe while message is in a non-terminal state.
  const streamEnabled = !!message && !TERMINAL_STATUSES.has(message.status);
  const { streamStatus, updates, close } = useMessageStream(streamEnabled);

  // Track the last status we acted on to avoid double-fetching.
  const lastHandledStatusRef = useRef<string | undefined>(undefined);

  useEffect(() => {
    if (!id) return;
    const update = updates[id];
    if (!update) return;
    if (update.status === lastHandledStatusRef.current) return;

    lastHandledStatusRef.current = update.status;
    loadMessage();

    if (TERMINAL_STATUSES.has(update.status)) {
      close();
    }
  }, [updates, id, loadMessage, close]);

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
          <div className="flex items-center gap-2">
            {streamEnabled && streamStatus === 'connected' && (
              <span className="flex items-center gap-1 text-xs text-green-600" title="Статус обновляется в реальном времени">
                <span className="inline-block w-2 h-2 rounded-full bg-green-500 animate-pulse" />
                Live
              </span>
            )}
            <Link to="/messages">
              <Button variant="secondary">Назад</Button>
            </Link>
          </div>
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
          {message.encoding && (
            <div>
              <span className="text-sm text-gray-500">Кодировка</span>
              <p className="text-sm mt-0.5 font-mono">{message.encoding}</p>
            </div>
          )}
          {message.external_id && (
            <div>
              <span className="text-sm text-gray-500">Внешний ID</span>
              <p className="text-sm mt-0.5 font-mono break-all">{message.external_id}</p>
            </div>
          )}
          {message.provider_name && (
            <div>
              <span className="text-sm text-gray-500">Провайдер</span>
              <p className="text-sm mt-0.5">{message.provider_name}</p>
            </div>
          )}
          {message.route_name && (
            <div>
              <span className="text-sm text-gray-500">Маршрут</span>
              <p className="text-sm mt-0.5">{message.route_name}</p>
            </div>
          )}
          {message.smpp_message_id && (
            <div>
              <span className="text-sm text-gray-500">SMPP ID</span>
              <p className="text-sm mt-0.5 font-mono break-all">{message.smpp_message_id}</p>
            </div>
          )}
          {(message.retry_count !== undefined && message.retry_count > 0) && (
            <div>
              <span className="text-sm text-gray-500">Попытки</span>
              <p className="text-sm mt-0.5">
                {message.retry_count} / {message.max_retries ?? '—'}
              </p>
            </div>
          )}
          {message.scheduled_at && (
            <div>
              <span className="text-sm text-gray-500">Запланировано</span>
              <p className="text-sm mt-0.5">{formatTimestamp(message.scheduled_at)}</p>
            </div>
          )}
          {message.expired_at && (
            <div>
              <span className="text-sm text-gray-500">Истекает</span>
              <p className="text-sm mt-0.5">{formatTimestamp(message.expired_at)}</p>
            </div>
          )}
        </div>
        {message.text && (
          <div className="mt-4 pt-4 border-t border-gray-100">
            <span className="text-sm text-gray-500">Текст сообщения</span>
            <p className="text-sm mt-1 whitespace-pre-wrap bg-gray-50 rounded p-3">{message.text}</p>
          </div>
        )}
        {message.status_message && (
          <div className="mt-4 pt-4 border-t border-gray-100">
            <span className="text-sm text-red-500">Ошибка</span>
            <p className="text-sm mt-1 text-red-700">{message.status_message}</p>
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

      {/* DLR receipt from operator */}
      {message.dlr && (
        <div className="bg-white border border-gray-200 rounded-lg p-6 mt-6">
          <h2 className="text-base font-semibold text-gray-900 mb-4">Ответ оператора (DLR)</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <span className="text-sm text-gray-500">Статус оператора</span>
              <div className="mt-0.5">
                <Badge variant={DLR_STAT_VARIANT[message.dlr.stat] ?? 'default'}>
                  {DLR_STAT_LABEL[message.dlr.stat] ?? message.dlr.stat}
                </Badge>
              </div>
            </div>
            {message.dlr.err !== 0 && (
              <div>
                <span className="text-sm text-gray-500">Код ошибки</span>
                <p className="text-sm mt-0.5 font-mono text-red-700">{message.dlr.err}</p>
              </div>
            )}
            {message.dlr.submit_date && (
              <div>
                <span className="text-sm text-gray-500">Принято оператором</span>
                <p className="text-sm mt-0.5">{formatTimestamp(message.dlr.submit_date)}</p>
              </div>
            )}
            {message.dlr.done_date && (
              <div>
                <span className="text-sm text-gray-500">Статус от оператора</span>
                <p className="text-sm mt-0.5">{formatTimestamp(message.dlr.done_date)}</p>
              </div>
            )}
            {message.dlr.receipted_message_id && (
              <div className="sm:col-span-2">
                <span className="text-sm text-gray-500">ID оператора</span>
                <p className="text-sm mt-0.5 font-mono break-all">{message.dlr.receipted_message_id}</p>
              </div>
            )}
          </div>
          {message.dlr.text && (
            <div className="mt-4 pt-4 border-t border-gray-100">
              <span className="text-sm text-gray-500">Текст от оператора</span>
              <p className="text-sm mt-1 whitespace-pre-wrap bg-gray-50 rounded p-3 font-mono">{message.dlr.text}</p>
            </div>
          )}
        </div>
      )}

      {/* Billing information */}
      {message.billing && (
        <div className="bg-white border border-gray-200 rounded-lg p-6 mt-6">
          <h2 className="text-base font-semibold text-gray-900 mb-4">Биллинг</h2>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <div>
              <span className="text-sm text-gray-500">Сегментов</span>
              <p className="text-sm mt-0.5 font-semibold">{message.billing.segment_count}</p>
            </div>
            <div>
              <span className="text-sm text-gray-500">Цена за сегмент</span>
              <p className="text-sm mt-0.5 font-semibold">{message.billing.price_per_segment} ₽</p>
            </div>
            <div>
              <span className="text-sm text-gray-500">Итого</span>
              <p className="text-sm mt-0.5 font-semibold text-blue-700">{message.billing.total_amount} ₽</p>
            </div>
          </div>
          <div className="mt-3 text-xs text-gray-400">
            Списание: {formatTimestamp(message.billing.billed_at)}
          </div>
        </div>
      )}
    </div>
  );
}
