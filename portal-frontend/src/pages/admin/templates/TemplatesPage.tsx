import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../../components/layout/PageHeader';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../../components/data/FilterBar';
import { StatusBadge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import { templatesApi, clientsApi, type TemplateInfo, type ClientInfo } from '../../../api/admin';
import { TemplateReviewModal } from './TemplateReviewModal';

const PAGE_SIZE = 20;

const statusLabelMap: Record<string, string> = {
  draft: 'Черновик',
  pending: 'Ожидание',
  review: 'На ревью',
  revision_requested: 'Доработка',
  approved: 'Одобрен',
  rejected: 'Отклонён',
};

const columns: Column<TemplateInfo>[] = [
  { key: 'name', header: 'Название', sortable: true },
  // TODO: Shows truncated client_id UUID; replace with client_email once the
  // admin templates endpoint is pool-migrated (currently returns client_id only).
  { key: 'client_id', header: 'Клиент', render: (t) => t.client_id.slice(0, 8) + '...' },
  {
    key: 'body',
    header: 'Текст',
    render: (t) => <span className="truncate max-w-[200px] inline-block">{t.body}</span>,
  },
  {
    key: 'status',
    header: 'Статус',
    render: (t) => {
      const label = statusLabelMap[t.status];
      return label ? <StatusBadge status={t.status} /> : <StatusBadge status={t.status} />;
    },
  },
  {
    key: 'reviewer_id',
    header: 'Ревьюер',
    render: (t) => (t.reviewer_id ? t.reviewer_id.slice(0, 8) + '...' : '—'),
  },
  {
    key: 'reviewed_at',
    header: 'Дата ревью',
    render: (t) => (t.reviewed_at ? new Date(t.reviewed_at).toLocaleDateString() : '—'),
  },
  {
    key: 'created_at',
    header: 'Создан',
    render: (t) => new Date(t.created_at).toLocaleDateString(),
  },
];

export function TemplatesPage() {
  const toast = useToast();
  const [data, setData] = useState<TemplateInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [filterValues, setFilterValues] = useState<Record<string, string>>({});
  const [selectedTemplate, setSelectedTemplate] = useState<TemplateInfo | null>(null);
  const [clients, setClients] = useState<ClientInfo[]>([]);

  const filters: FilterDef[] = [
    {
      key: 'client_id',
      label: 'Клиент',
      type: 'select',
      options: clients.map((c) => ({ value: c.client_id, label: c.name })),
      placeholder: 'Все клиенты',
    },
    {
      key: 'status',
      label: 'Статус',
      type: 'select',
      options: [
        { value: 'pending', label: 'Ожидание' },
        { value: 'review', label: 'На ревью' },
        { value: 'revision_requested', label: 'Доработка' },
        { value: 'approved', label: 'Одобрен' },
        { value: 'rejected', label: 'Отклонён' },
      ],
    },
  ];

  useEffect(() => {
    clientsApi.list({ limit: 500 }).then((res) => setClients(res.clients || [])).catch(() => {});
  }, []);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await templatesApi.list({
        ...filterValues,
        limit: PAGE_SIZE,
        offset: (page - 1) * PAGE_SIZE,
      });
      setData(res.templates || []);
      setTotal(res.total);
    } catch {
      toast.error('Failed to load templates');
    } finally {
      setLoading(false);
    }
  }, [page, filterValues, toast]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleRowClick = (template: TemplateInfo) => {
    setSelectedTemplate(template);
  };

  const handleModalUpdate = () => {
    setSelectedTemplate(null);
    fetchData();
  };

  return (
    <>
      <PageHeader
        title="Шаблоны"
        subtitle={`${total} шаблонов`}
        breadcrumbs={[
          { label: 'Админ', href: '/admin/dashboard' },
          { label: 'Шаблоны' },
        ]}
      />
      <FilterBar
        filters={filters}
        values={filterValues}
        onChange={(v) => {
          setFilterValues(v);
          setPage(1);
        }}
        onReset={() => {
          setFilterValues({});
          setPage(1);
        }}
      />
      <DataTable
        columns={columns}
        data={data}
        total={total}
        page={page}
        pageSize={PAGE_SIZE}
        onPageChange={setPage}
        loading={loading}
        keyField="template_id"
        onRowClick={handleRowClick}
      />
      <TemplateReviewModal
        template={selectedTemplate}
        open={!!selectedTemplate}
        onClose={() => setSelectedTemplate(null)}
        onUpdate={handleModalUpdate}
      />
    </>
  );
}
