// portal-frontend/src/pages/admin/detalization/StatusTimeline.tsx
import React from 'react';

interface StatusEvent {
  status: string;
  timestamp: string;
  details?: string;
}

interface StatusTimelineProps {
  statuses: StatusEvent[];
}

const STATUS_COLORS: Record<string, string> = {
  pending:   'bg-gray-400',
  queued:    'bg-yellow-400',
  sent:      'bg-blue-500',
  delivered: 'bg-green-500',
  failed:    'bg-red-500',
  rejected:  'bg-red-400',
  expired:   'bg-orange-400',
};

const STATUS_LABELS: Record<string, string> = {
  pending:   'Ожидание',
  queued:    'В очереди',
  sent:      'Отправлено',
  delivered: 'Доставлено',
  failed:    'Ошибка',
  rejected:  'Отклонено',
  expired:   'Истёк TTL',
};

function formatTime(iso: string): string {
  try {
    return new Date(iso).toLocaleString('ru-RU', {
      day: '2-digit', month: '2-digit', year: 'numeric',
      hour: '2-digit', minute: '2-digit', second: '2-digit',
    });
  } catch {
    return iso;
  }
}

export const StatusTimeline: React.FC<StatusTimelineProps> = ({ statuses }) => {
  if (!statuses || statuses.length === 0) {
    return <p className="text-sm text-gray-500">История статусов недоступна</p>;
  }

  return (
    <div className="relative">
      {statuses.map((event, idx) => {
        const dot = STATUS_COLORS[event.status] ?? 'bg-gray-400';
        const label = STATUS_LABELS[event.status] ?? event.status;
        const isLast = idx === statuses.length - 1;

        return (
          <div key={`${event.status}-${event.timestamp}-${idx}`} className="flex gap-3">
            {/* Dot + line */}
            <div className="flex flex-col items-center">
              <div className={`w-3 h-3 rounded-full flex-shrink-0 mt-1 ${dot}`} />
              {!isLast && <div className="w-px flex-1 min-h-8 bg-gray-200 mt-1" />}
            </div>
            {/* Content */}
            <div className="pb-4">
              <p className="text-sm font-medium text-gray-800">{label}</p>
              <p className="text-xs text-gray-500">{formatTime(event.timestamp)}</p>
              {event.details && (
                <p className="text-xs text-gray-600 mt-0.5">{event.details}</p>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
};
