import { useState, useEffect, useCallback } from 'react';
import { templatesApi, senderNamesApi, ApiError, type TemplateInfo, type SenderNameInfo } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { DataTable, type Column } from '../../components/data/DataTable';
import type { BulkAction } from '../../components/data/BulkActionBar';
import { Badge } from '../../components/ui/Badge';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { TemplateFormModal } from '../../components/templates/TemplateFormModal';
import { TemplatePreviewModal } from '../../components/templates/TemplatePreviewModal';

const PAGE_SIZE = 20;

const STATUS_BADGE: Record<string, { variant: 'warning' | 'success' | 'danger' | 'default'; label: string }> = {
  draft: { variant: 'default', label: 'Черновик' },
  pending: { variant: 'warning', label: 'На модерации' },
  review: { variant: 'warning', label: 'На ревью' },
  revision_requested: { variant: 'danger', label: 'Доработка' },
  approved: { variant: 'success', label: 'Одобрен' },
  rejected: { variant: 'danger', label: 'Отклонён' },
};

function pluralize(n: number, one: string, few: string, many: string): string {
  const abs = Math.abs(n);
  const mod10 = abs % 10;
  const mod100 = abs % 100;
  if (mod10 === 1 && mod100 !== 11) return `${n} ${one}`;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return `${n} ${few}`;
  return `${n} ${many}`;
}

function truncate(text: string, max: number): string {
  return text.length > max ? text.slice(0, max) + '...' : text;
}

export function TemplatesPage() {
  const [templates, setTemplates] = useState<TemplateInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Approved sender names for dropdown
  const [approvedSenderNames, setApprovedSenderNames] = useState<SenderNameInfo[]>([]);
  const [sendersError, setSendersError] = useState(false);

  // Create / Edit modal
  const [showForm, setShowForm] = useState(false);
  const [editingTemplate, setEditingTemplate] = useState<TemplateInfo | null>(null);

  // Preview modal
  const [previewTemplate, setPreviewTemplate] = useState<TemplateInfo | null>(null);

  // Submit for review loading
  const [submittingId, setSubmittingId] = useState<string | null>(null);

  // Delete confirmation
  const [deleteId, setDeleteId] = useState<string | null>(null);

  const fetchTemplates = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await templatesApi.list({
        page: String(page),
        per_page: String(PAGE_SIZE),
      });
      setTemplates(res.templates || []);
      setTotal(res.total);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить шаблоны');
    } finally {
      setLoading(false);
    }
  }, [page]);

  useEffect(() => {
    fetchTemplates();
  }, [fetchTemplates]);

  function loadSenderNames() {
    setSendersError(false);
    senderNamesApi.listApproved()
      .then((res) => setApprovedSenderNames(res.sender_names || []))
      .catch(() => setSendersError(true));
  }

  useEffect(() => {
    loadSenderNames();
  }, []);

  // --- Create / Edit ---

  function openCreateForm() {
    setEditingTemplate(null);
    setShowForm(true);
  }

  function openEditForm(tpl: TemplateInfo) {
    setEditingTemplate(tpl);
    setShowForm(true);
  }

  // --- Preview ---

  function openPreview(tpl: TemplateInfo) {
    setPreviewTemplate(tpl);
  }

  // --- Submit for review ---

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

  // --- Delete ---

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

  // --- Table columns ---

  const columns: Column<TemplateInfo>[] = [
    { key: 'name', header: 'Название' },
    {
      key: 'status',
      header: 'Статус',
      render: (tpl) => {
        const cfg = STATUS_BADGE[tpl.status] || { variant: 'default' as const, label: tpl.status };
        return <Badge variant={cfg.variant}>{cfg.label}</Badge>;
      },
    },
    {
      key: 'sender_name',
      header: 'Отправитель',
      render: (tpl) =>
        tpl.sender_name ? (
          <span className="text-sm font-medium">{tpl.sender_name}</span>
        ) : (
          <span className="text-gray-400 text-sm">—</span>
        ),
    },
    {
      key: 'traffic_type',
      header: 'Тип трафика',
      render: (tpl) => {
        const labels: Record<string, string> = {
          transactional: 'Транзакционный',
          authorization: 'Авторизационный',
          service: 'Сервисный',
        };
        const val = tpl.traffic_type || 'transactional';
        return <span className="text-sm text-gray-700">{labels[val] ?? val}</span>;
      },
    },
    {
      key: 'body',
      header: 'Текст',
      render: (tpl) => (
        <span className="text-gray-600 max-w-[250px] inline-block truncate" title={tpl.body}>
          {truncate(tpl.body, 60)}
        </span>
      ),
    },
    {
      key: 'created_at',
      header: 'Создан',
      render: (tpl) => new Date(tpl.created_at).toLocaleDateString('ru-RU'),
    },
  ];

  const deleteTemplate = templates.find((t) => t.id === deleteId);

  return (
    <div className="w-full">
      <PageHeader
        title="Шаблоны"
        subtitle={total > 0 ? pluralize(total, 'шаблон', 'шаблона', 'шаблонов') : undefined}
        actions={
          <Button onClick={openCreateForm}>
            Создать шаблон
          </Button>
        }
      />

      {error && <p className="text-red-600 mb-4">{error}</p>}

      {/* Create / Edit modal */}
      <TemplateFormModal
        open={showForm}
        onClose={() => { setShowForm(false); setEditingTemplate(null); }}
        onSubmitted={() => { fetchTemplates(); setShowForm(false); setEditingTemplate(null); }}
        editingTemplate={editingTemplate}
        availableSenderNames={approvedSenderNames}
        availableSenderNamesError={sendersError}
        onReloadSenderNames={loadSenderNames}
      />

      {/* Preview modal */}
      <TemplatePreviewModal
        template={previewTemplate}
        onClose={() => setPreviewTemplate(null)}
      />

      {/* Delete confirmation */}
      <ConfirmDialog
        open={!!deleteId}
        onCancel={() => setDeleteId(null)}
        onConfirm={() => deleteId && handleDelete(deleteId)}
        title="Удалить шаблон"
        description={`Вы уверены, что хотите удалить шаблон${deleteTemplate ? ` "${deleteTemplate.name}"` : ''}? Это действие необратимо.`}
        confirmLabel="Удалить"
        variant="danger"
      />

      {loading && templates.length === 0 && (
        <div role="status">Загрузка шаблонов...</div>
      )}

      {!loading && !error && templates.length === 0 && (
        <div className="text-center py-12 text-gray-500">
          <p className="mb-2">Шаблоны не созданы</p>
          <p className="text-sm mb-4">Создайте первый шаблон для отправки сообщений</p>
          <Button onClick={openCreateForm}>Создать шаблон</Button>
        </div>
      )}

      {/* Templates table */}
      {(loading || templates.length > 0) && (
      <DataTable<TemplateInfo>
        columns={columns}
        data={templates}
        total={total}
        page={page}
        pageSize={PAGE_SIZE}
        onPageChange={setPage}
        loading={loading}
        keyField="id"
        bulkActions={[
          {
            label: 'На проверку',
            variant: 'primary',
            requiresConfirmation: true,
            confirmMessage: (n) => `Отправить ${n} шаблонов на проверку?`,
            onAction: async (ids) => {
              const submittable = templates
                .filter((t) => ids.includes(t.id) && (t.status === 'draft' || t.status === 'revision_requested'))
                .map((t) => t.id);
              await Promise.all(submittable.map((id) => templatesApi.submit(id).catch(() => {})));
              await fetchTemplates();
            },
          } as BulkAction<TemplateInfo>,
          {
            label: 'Удалить',
            variant: 'danger',
            requiresConfirmation: true,
            confirmMessage: (n) => `Удалить ${n} шаблонов? Это действие необратимо.`,
            onAction: async (ids) => {
              await Promise.all(ids.map((id) => templatesApi.remove(id).catch(() => {})));
              await fetchTemplates();
            },
          } as BulkAction<TemplateInfo>,
        ]}
        rowActions={(tpl) => (
          <div className="flex gap-1 flex-wrap">
            <Button
              variant="ghost"
              size="sm"
              aria-label={`Превью ${tpl.name}`}
              onClick={() => openPreview(tpl)}
            >
              Превью
            </Button>

            {(tpl.status === 'draft' || tpl.status === 'revision_requested') && (
              <Button
                variant="primary"
                size="sm"
                aria-label={`Отправить ${tpl.name} на модерацию`}
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
                aria-label={`Редактировать ${tpl.name}`}
                onClick={() => openEditForm(tpl)}
              >
                Редактировать
              </Button>
            )}

            <Button
              variant="ghost"
              size="sm"
              aria-label={`Удалить ${tpl.name}`}
              onClick={() => setDeleteId(tpl.id)}
            >
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
    </div>
  );
}
