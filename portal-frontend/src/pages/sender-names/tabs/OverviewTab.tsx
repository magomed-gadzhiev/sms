import type { SenderNameInfo } from '../../../api/client';
import { Badge } from '../../../components/ui/Badge';
import { STATUS_BADGE } from '../senderNameUtils';

const CHANNEL_LABEL: Record<string, string> = {
  sms:   'SMS',
  voice: 'Голос',
  viber: 'Viber',
};

function formatDate(dt: string) {
  return new Date(dt).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
}

export function OverviewTab({ senderName }: { senderName: SenderNameInfo }) {
  const s = STATUS_BADGE[senderName.status] ?? { variant: 'default' as const, label: senderName.status };

  return (
    <div>
      {/* Status + channel chips */}
      <div className="flex items-center gap-3 mb-6">
        <Badge variant={s.variant}>{s.label}</Badge>
        {senderName.channel && (
          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
            {CHANNEL_LABEL[senderName.channel] ?? senderName.channel}
          </span>
        )}
      </div>

      {/* Metadata grid */}
      <div className="grid grid-cols-2 gap-4 text-sm mb-6">
        <div>
          <span className="text-gray-400">Создано</span>
          <div className="text-gray-900">{formatDate(senderName.created_at)}</div>
        </div>
        {senderName.reviewed_at && (
          <div>
            <span className="text-gray-400">
              {senderName.status === 'approved' ? 'Одобрено' : 'Отклонено'}
            </span>
            <div className="text-gray-900">{formatDate(senderName.reviewed_at)}</div>
          </div>
        )}
      </div>

      {/* Pending info */}
      {senderName.status === 'pending' && (
        <div className="border-t pt-4">
          <p className="text-sm text-gray-500">
            Имя находится на модерации. Мы уведомим вас о результате.
          </p>
        </div>
      )}

      {/* Rejection reason */}
      {senderName.status === 'rejected' && senderName.rejection_reason && (
        <div className="border-t pt-4">
          <div className="p-3 bg-red-50 border border-red-200 rounded-md">
            <p className="text-sm font-medium text-red-800">Причина отклонения</p>
            <p className="text-sm text-red-700 mt-1">{senderName.rejection_reason}</p>
          </div>
        </div>
      )}
    </div>
  );
}
