import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { Modal } from '../../../components/ui/Modal';
import { Button } from '../../../components/ui/Button';
import type { DetalizationMessage } from '../../../api/client';

const STATUS_LABELS: Record<string, string> = {
  pending: 'Ожидание', queued: 'В очереди', sent: 'Отправлено',
  delivered: 'Доставлено', failed: 'Ошибка', expired: 'Истекло',
  rejected: 'Отклонено', scheduled: 'Запланировано',
};

const STATUS_COLORS: Record<string, string> = {
  delivered: 'bg-green-100 text-green-800', sent: 'bg-blue-100 text-blue-800',
  failed: 'bg-red-100 text-red-800', rejected: 'bg-red-100 text-red-800',
  expired: 'bg-gray-100 text-gray-600', queued: 'bg-yellow-100 text-yellow-800',
  pending: 'bg-yellow-100 text-yellow-800', scheduled: 'bg-purple-100 text-purple-800',
};

function fmt(s?: string | null) {
  if (!s) return '—';
  return new Date(s).toLocaleString('ru-RU');
}

interface Props {
  message: DetalizationMessage;
  onClose: () => void;
}

export function MessageModal({ message, onClose }: Props) {
  const statusLabel = STATUS_LABELS[message.status] ?? message.status;
  const statusCls = STATUS_COLORS[message.status] ?? 'bg-gray-100 text-gray-700';

  const rows: Array<{ label: string; value: ReactNode }> = [
    { label: 'ID', value: <span className="font-mono text-xs break-all">{message.id}</span> },
    {
      label: 'Статус',
      value: (
        <span className={`inline-block rounded px-2 py-0.5 text-xs font-medium ${statusCls}`}>
          {statusLabel}
        </span>
      ),
    },
    { label: 'Отправитель', value: message.source || '—' },
    { label: 'Получатель', value: message.destination || '—' },
    { label: 'Оператор', value: message.operator_name || '—' },
    { label: 'Канал', value: message.channel || '—' },
    {
      label: 'Текст',
      value: <span className="whitespace-pre-wrap text-sm">{message.text_preview}</span>,
    },
    { label: 'Дата отправки', value: fmt(message.submitted_at) },
    { label: 'Стоимость', value: message.total_amount ? `${message.total_amount} ₽` : '—' },
  ];

  return (
    <Modal open onClose={onClose} title="Сообщение" description="Краткая информация о сообщении">
      <div className="space-y-1 mb-4">
        {rows.map((row) => (
          <div key={row.label} className="flex gap-2 py-1 border-b border-gray-100 last:border-0">
            <span className="w-36 shrink-0 text-sm text-gray-500">{row.label}</span>
            <span className="text-sm text-gray-900">{row.value}</span>
          </div>
        ))}
      </div>
      <div className="flex justify-between items-center pt-2">
        <Button variant="secondary" onClick={onClose}>Закрыть</Button>
        <Link
          to={`/messages/${message.id}`}
          className="inline-flex items-center gap-1 text-sm text-primary hover:underline font-medium"
          onClick={onClose}
        >
          Подробнее
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
          </svg>
        </Link>
      </div>
    </Modal>
  );
}
