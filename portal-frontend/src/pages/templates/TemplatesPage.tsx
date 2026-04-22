import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { templatesApi, ApiError, type TemplateInfo } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Badge } from '../../components/ui/Badge';
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

function truncate(text: string, max: number): string {
  return text.length > max ? text.slice(0, max) + '...' : text;
}

export function TemplatesPage() {
  const navigate = useNavigate();

  const [templates, setTemplates] = useState<TemplateInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Preview modal
  const [previewTemplate, setPreviewTemplate] = useState<TemplateInfo | null>(null);

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

  return (
    <div className="w-full">
      <PageHeader
        title="Шаблоны"
        subtitle="Справочник всех ваших шаблонов. Создание и редактирование — в карточке имени отправителя."
      />

      {error && <p className="text-red-600 mb-4">{error}</p>}

      {/* Preview modal */}
      <TemplatePreviewModal
        template={previewTemplate}
        onClose={() => setPreviewTemplate(null)}
      />

      {loading && templates.length === 0 && (
        <div role="status">Загрузка шаблонов...</div>
      )}

      {!loading && !error && templates.length === 0 && (
        <div className="text-center py-16 text-gray-500">
          <p className="mb-2">У вас пока нет шаблонов</p>
          <p className="text-sm">
            Откройте{' '}
            <a href="/sender-names" className="text-primary underline hover:no-underline">
              список имён отправителей
            </a>
            , зарегистрируйте имя и создайте шаблон во вкладке «Шаблоны» на его странице.
          </p>
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
        bulkActions={[]}
        rowActions={(tpl) => (
          <div className="flex gap-1">
            <Button variant="ghost" size="sm" onClick={() => setPreviewTemplate(tpl)}>Превью</Button>
            {tpl.sender_name_id ? (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => navigate(`/sender-names/${tpl.sender_name_id}?tab=templates`)}
              >
                Перейти к имени
              </Button>
            ) : (
              <span className="text-xs text-gray-400">без имени</span>
            )}
          </div>
        )}
      />
      )}
    </div>
  );
}
