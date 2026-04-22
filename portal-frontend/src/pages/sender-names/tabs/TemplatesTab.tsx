import { useState, useEffect, useCallback, type ReactNode } from 'react';
import { templatesApi, ApiError, type SenderNameInfo, type TemplateInfo } from '../../../api/client';
import { Button } from '../../../components/ui/Button';
import { Badge } from '../../../components/ui/Badge';
import { DataTable, type Column } from '../../../components/data/DataTable';
import type { BulkAction } from '../../../components/data/BulkActionBar';
import { ConfirmDialog } from '../../../components/ui/ConfirmDialog';
import { TemplateFormModal } from '../../../components/templates/TemplateFormModal';
import { TemplatePreviewModal } from '../../../components/templates/TemplatePreviewModal';

const PAGE_SIZE = 200;

const STATUS_BADGE: Record<string, { variant: 'warning' | 'success' | 'danger' | 'default'; label: string }> = {
  draft: { variant: 'default', label: 'Черновик' },
  pending: { variant: 'warning', label: 'На модерации' },
  review: { variant: 'warning', label: 'На ревью' },
  revision_requested: { variant: 'danger', label: 'Доработка' },
  approved: { variant: 'success', label: 'Одобрен' },
  rejected: { variant: 'danger', label: 'Отклонён' },
};

const TRAFFIC_LABELS: Record<string, string> = {
  transactional: 'Транзакционный',
  authorization: 'Авторизационный',
  service: 'Сервисный',
};

function truncate(s: string, n: number): string {
  return s.length > n ? s.slice(0, n) + '...' : s;
}

export function TemplatesTab({ senderName }: { senderName: SenderNameInfo }) {
  const [templates, setTemplates] = useState<TemplateInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [showForm, setShowForm] = useState(false);
  const [editingTemplate, setEditingTemplate] = useState<TemplateInfo | null>(null);
  const [previewTemplate, setPreviewTemplate] = useState<TemplateInfo | null>(null);
  const [submittingId, setSubmittingId] = useState<string | null>(null);
  const [deleteId, setDeleteId] = useState<string | null>(null);

  // Client-side filter: fetch a large page and keep only templates for this sender name.
  // Server-side sender_name_id filter is not yet implemented in /templates API (only status filter).
  // Phase 3 will add the server-side filter. 200 templates per sender name is the current soft cap.
  const fetchTemplates = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await templatesApi.list({ page: '1', per_page: String(PAGE_SIZE) });
      const all = res.templates || [];
      const scoped = all.filter(t => t.sender_name_id === senderName.id);
      setTemplates(scoped);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить шаблоны');
    } finally {
      setLoading(false);
    }
  }, [senderName.id]);

  useEffect(() => { fetchTemplates(); }, [fetchTemplates]);

  async function handleSubmitForReview(id: string) {
    if (submittingId) return;
    setSubmittingId(id);
    setError('');
    try {
      await templatesApi.submit(id);
      await fetchTemplates();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось отправить на модерацию');
    } finally {
      setSubmittingId(null);
    }
  }

  async function handleDelete(id: string) {
    setError('');
    try {
      await templatesApi.remove(id);
      setDeleteId(null);
      await fetchTemplates();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось удалить шаблон');
    }
  }

  const columns: Column<TemplateInfo>[] = [
    { key: 'name', header: 'Название' },
    {
      key: 'status',
      header: 'Статус',
      render: (tpl) => {
        const cfg = STATUS_BADGE[tpl.status] ?? { variant: 'default' as const, label: tpl.status };
        return <Badge variant={cfg.variant}>{cfg.label}</Badge>;
      },
    },
    {
      key: 'traffic_type',
      header: 'Тип трафика',
      render: (tpl): ReactNode => (
        <span className="text-sm text-gray-700">
          {TRAFFIC_LABELS[tpl.traffic_type || 'transactional'] ?? tpl.traffic_type}
        </span>
      ),
    },
    {
      key: 'body',
      header: 'Текст',
      render: (tpl): ReactNode => (
        <span className="text-gray-600 max-w-[280px] inline-block truncate" title={tpl.body}>
          {truncate(tpl.body, 60)}
        </span>
      ),
    },
    {
      key: 'created_at',
      header: 'Создан',
      render: (tpl): ReactNode => new Date(tpl.created_at).toLocaleDateString('ru-RU'),
    },
  ];

  const bulkActions: BulkAction<TemplateInfo>[] = [
    {
      label: 'На проверку',
      variant: 'primary',
      requiresConfirmation: true,
      confirmMessage: (n) => `Отправить ${n} шаблонов на проверку?`,
      onAction: async (ids) => {
        const submittable = templates
          .filter(t => ids.includes(t.id) && (t.status === 'draft' || t.status === 'revision_requested'))
          .map(t => t.id);
        await Promise.all(submittable.map(id => templatesApi.submit(id).catch(() => {})));
        await fetchTemplates();
      },
    },
    {
      label: 'Удалить',
      variant: 'danger',
      requiresConfirmation: true,
      confirmMessage: (n) => `Удалить ${n} шаблонов? Это действие необратимо.`,
      onAction: async (ids) => {
        await Promise.all(ids.map(id => templatesApi.remove(id).catch(() => {})));
        await fetchTemplates();
      },
    },
  ];

  const deleteTemplate = templates.find(t => t.id === deleteId);

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <p className="text-sm text-gray-500">
          Шаблоны для имени <span className="font-mono">{senderName.name}</span>
        </p>
        <Button
          onClick={() => { setEditingTemplate(null); setShowForm(true); }}
          disabled={senderName.status !== 'approved'}
          title={senderName.status !== 'approved' ? 'Создание шаблонов доступно только для одобренных имён' : undefined}
        >
          Создать шаблон
        </Button>
      </div>

      {error && <p className="text-red-600 mb-4">{error}</p>}

      {loading && templates.length === 0 && (
        <div role="status">Загрузка шаблонов...</div>
      )}

      {!loading && !error && templates.length === 0 && (
        <div className="text-center py-12 text-gray-500">
          <p className="mb-2">Шаблонов для этого имени пока нет</p>
          <p className="text-sm mb-4">
            {senderName.status === 'approved'
              ? 'Создайте первый шаблон для отправки сообщений с этим именем'
              : 'Создание шаблонов будет доступно после одобрения имени'}
          </p>
          {senderName.status === 'approved' && (
            <Button onClick={() => { setEditingTemplate(null); setShowForm(true); }}>
              Создать шаблон
            </Button>
          )}
        </div>
      )}

      {(loading || templates.length > 0) && (
        <DataTable<TemplateInfo>
          columns={columns}
          data={templates}
          total={templates.length}
          page={1}
          pageSize={PAGE_SIZE}
          onPageChange={() => {}}
          loading={loading}
          keyField="id"
          bulkActions={bulkActions}
          rowActions={(tpl): ReactNode => (
            <div className="flex gap-1 flex-wrap">
              <Button variant="ghost" size="sm" onClick={() => setPreviewTemplate(tpl)}>
                Превью
              </Button>
              {(tpl.status === 'draft' || tpl.status === 'revision_requested') && (
                <Button
                  variant="primary"
                  size="sm"
                  disabled={submittingId === tpl.id}
                  onClick={() => handleSubmitForReview(tpl.id)}
                >
                  {submittingId === tpl.id ? 'Отправка...' : 'На модерацию'}
                </Button>
              )}
              {(tpl.status === 'draft' || tpl.status === 'revision_requested' || tpl.status === 'rejected') && (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => { setEditingTemplate(tpl); setShowForm(true); }}
                >
                  Редактировать
                </Button>
              )}
              <Button variant="ghost" size="sm" onClick={() => setDeleteId(tpl.id)}>
                Удалить
              </Button>
              {tpl.status === 'rejected' && tpl.rejection_reason && (
                <span
                  className="inline-flex items-center text-xs text-red-600 ml-1"
                  title={tpl.rejection_reason}
                >
                  Причина: {truncate(tpl.rejection_reason, 30)}
                </span>
              )}
            </div>
          )}
        />
      )}

      <TemplateFormModal
        open={showForm}
        onClose={() => { setShowForm(false); setEditingTemplate(null); }}
        onSubmitted={() => { fetchTemplates(); setShowForm(false); setEditingTemplate(null); }}
        editingTemplate={editingTemplate}
        preselectedSenderNameId={senderName.id}
        availableSenderNames={[senderName]}
        availableSenderNamesError={false}
        onReloadSenderNames={undefined}
      />

      <TemplatePreviewModal
        template={previewTemplate}
        onClose={() => setPreviewTemplate(null)}
      />

      <ConfirmDialog
        open={!!deleteId}
        onCancel={() => setDeleteId(null)}
        onConfirm={() => { if (deleteId) handleDelete(deleteId); }}
        title="Удалить шаблон"
        description={`Удалить шаблон${deleteTemplate ? ` "${deleteTemplate.name}"` : ''}? Действие необратимо.`}
        confirmLabel="Удалить"
        variant="danger"
      />
    </div>
  );
}
