import { useState } from 'react';
import { operatorTemplatesApi, type OperatorTemplate } from '../../api/client';

const STATUS_LABELS: Record<string, { label: string; className: string }> = {
  draft: { label: 'Черновик', className: 'bg-gray-100 text-gray-700' },
  submitted: { label: 'На модерации', className: 'bg-yellow-100 text-yellow-800' },
  approved: { label: 'Одобрен', className: 'bg-green-100 text-green-800' },
  rejected: { label: 'Отклонён', className: 'bg-red-100 text-red-800' },
  revision_requested: { label: 'Требует доработки', className: 'bg-orange-100 text-orange-800' },
};

interface Props {
  senderNameId: string;
  templates: OperatorTemplate[];
  onRefresh: () => void;
}

export function OperatorTemplatesSection({ senderNameId, templates, onRefresh }: Props) {
  const [expanded, setExpanded] = useState(false);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (tid: string) => {
    setLoading(true);
    try {
      await operatorTemplatesApi.submit(senderNameId, tid);
      onRefresh();
    } finally {
      setLoading(false);
    }
  };

  const handleResubmit = async (tid: string) => {
    setLoading(true);
    try {
      await operatorTemplatesApi.resubmit(senderNameId, tid);
      onRefresh();
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (tid: string) => {
    if (!confirm('Удалить шаблон?')) return;
    setLoading(true);
    try {
      await operatorTemplatesApi.delete(senderNameId, tid);
      onRefresh();
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="mt-6 border-t pt-4">
      <button
        className="flex items-center gap-2 text-sm font-medium text-gray-700 hover:text-gray-900"
        onClick={() => setExpanded(!expanded)}
      >
        <span>{expanded ? '▾' : '▸'}</span>
        Шаблоны операторов ({templates.length})
      </button>

      {expanded && (
        <div className="mt-3 space-y-2">
          {templates.length === 0 && (
            <p className="text-sm text-gray-500">Шаблоны не добавлены</p>
          )}
          {templates.map((t) => {
            const statusStyle = STATUS_LABELS[t.moderation_status] ?? STATUS_LABELS.draft;
            return (
              <div key={t.id} className="border rounded-lg p-3 bg-white">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <p className="font-medium text-sm truncate">{t.name}</p>
                    <p className="text-xs text-gray-500">{t.operator_name}</p>
                    <p className="text-xs text-gray-700 mt-1 whitespace-pre-wrap break-words">{t.body}</p>
                    {t.moderator_note && (
                      <p className="text-xs text-red-600 mt-1">Комментарий: {t.moderator_note}</p>
                    )}
                  </div>
                  <div className="flex flex-col items-end gap-1 shrink-0">
                    <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${statusStyle.className}`}>
                      {statusStyle.label}
                    </span>
                    {t.moderation_status === 'draft' && (
                      <div className="flex gap-1">
                        <button
                          onClick={() => handleSubmit(t.id)}
                          disabled={loading}
                          className="text-xs px-2 py-0.5 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50"
                        >
                          Подать
                        </button>
                        <button
                          onClick={() => handleDelete(t.id)}
                          disabled={loading}
                          className="text-xs px-2 py-0.5 bg-red-50 text-red-600 border border-red-200 rounded hover:bg-red-100 disabled:opacity-50"
                        >
                          Удалить
                        </button>
                      </div>
                    )}
                    {t.moderation_status === 'revision_requested' && (
                      <button
                        onClick={() => handleResubmit(t.id)}
                        disabled={loading}
                        className="text-xs px-2 py-0.5 bg-orange-600 text-white rounded hover:bg-orange-700 disabled:opacity-50"
                      >
                        Повторно подать
                      </button>
                    )}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
