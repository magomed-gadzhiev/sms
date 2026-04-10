import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../../components/layout/PageHeader';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import { Modal } from '../../../components/ui/Modal';
import { Badge } from '../../../components/ui/Badge';
import {
  messagesApi,
  clientsApi,
  providersApi,
  type AdminMessage,
  type AdminMessageDetail,
  type ClientInfo,
  type ProviderInfo,
} from '../../../api/admin';

const STATUS_LABEL: Record<string, string> = {
  pending: 'Ожидает',
  queued: 'В очереди',
  submitted: 'Отправлено',
  sent: 'Отправлено',
  delivered: 'Доставлено',
  failed: 'Ошибка',
  expired: 'Истекло',
  rejected: 'Отклонено',
};

const STATUS_VARIANT: Record<string, 'success' | 'danger' | 'warning' | 'default'> = {
  delivered: 'success',
  failed: 'danger',
  rejected: 'danger',
  expired: 'danger',
  pending: 'default',
  queued: 'warning',
  submitted: 'warning',
  sent: 'warning',
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

function formatDate(s?: string | null): string {
  if (!s) return '—';
  return new Date(s).toLocaleString('ru-RU');
}

interface Filters {
  client_id: string;
  status: string;
  source: string;
  destination: string;
  provider_id: string;
  date_from: string;
  date_to: string;
}

const EMPTY_FILTERS: Filters = {
  client_id: '',
  status: '',
  source: '',
  destination: '',
  provider_id: '',
  date_from: '',
  date_to: '',
};

const STATUS_OPTIONS = [
  { value: '', label: 'Все статусы' },
  { value: 'pending', label: 'Ожидает' },
  { value: 'queued', label: 'В очереди' },
  { value: 'sent', label: 'Отправлено' },
  { value: 'delivered', label: 'Доставлено' },
  { value: 'failed', label: 'Ошибка' },
  { value: 'expired', label: 'Истекло' },
  { value: 'rejected', label: 'Отклонено' },
];

const PAGE_SIZE = 50;

export function DetalizationPage() {
  const [filters, setFilters] = useState<Filters>(EMPTY_FILTERS);
  const [activeFilters, setActiveFilters] = useState<Filters>(EMPTY_FILTERS);
  const [messages, setMessages] = useState<AdminMessage[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [clients, setClients] = useState<ClientInfo[]>([]);
  const [providers, setProviders] = useState<ProviderInfo[]>([]);
  const [detail, setDetail] = useState<AdminMessageDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  useEffect(() => {
    clientsApi.list({ limit: 500 }).then((r) => setClients(r.clients || [])).catch(() => {});
    providersApi.list({ limit: 500 }).then((r) => setProviders(r.providers || [])).catch(() => {});
  }, []);

  const clientOptions = [
    { value: '', label: 'Все клиенты' },
    ...clients.map((c) => ({ value: c.client_id, label: c.name })),
  ];
  const providerOptions = [
    { value: '', label: 'Все провайдеры' },
    ...providers.map((p) => ({ value: p.provider_id, label: p.name })),
  ];

  const fetchMessages = useCallback(async (f: Filters, p: number) => {
    setLoading(true);
    try {
      const res = await messagesApi.list({ ...f, limit: PAGE_SIZE, offset: (p - 1) * PAGE_SIZE });
      setMessages(res.messages || []);
      setTotal(res.total || 0);
    } catch {
      setMessages([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchMessages(activeFilters, page);
  }, [fetchMessages, activeFilters, page]);

  function applyFilters() {
    setPage(1);
    setActiveFilters({ ...filters });
  }

  function resetFilters() {
    setFilters(EMPTY_FILTERS);
    setPage(1);
    setActiveFilters(EMPTY_FILTERS);
  }

  async function openDetail(id: string) {
    setDetailLoading(true);
    setDetail(null);
    try {
      const res = await messagesApi.get(id);
      setDetail(res);
    } catch {
      // ignore
    } finally {
      setDetailLoading(false);
    }
  }

  const columns: Column<AdminMessage>[] = [
    {
      key: 'created_at',
      header: 'Дата',
      render: (row) => <span className="text-xs text-gray-600 whitespace-nowrap">{formatDate(row.created_at)}</span>,
    },
    {
      key: 'source',
      header: 'Отправитель',
      render: (row) => <span className="font-mono text-xs">{row.source || '—'}</span>,
    },
    {
      key: 'destination',
      header: 'Получатель',
      render: (row) => <span className="font-mono text-xs">{row.destination || '—'}</span>,
    },
    {
      key: 'text_preview',
      header: 'Текст',
      render: (row) => (
        <span className="text-xs text-gray-700 truncate max-w-[200px] block" title={row.text_preview}>
          {row.text_preview || '—'}
        </span>
      ),
    },
    {
      key: 'status',
      header: 'Статус',
      render: (row) => (
        <Badge variant={STATUS_VARIANT[row.status] ?? 'default'}>
          {STATUS_LABEL[row.status] ?? row.status}
        </Badge>
      ),
    },
    {
      key: 'segment_count',
      header: 'Сег.',
      render: (row) => <span className="text-xs">{row.segment_count}</span>,
    },
    {
      key: 'provider_name',
      header: 'Провайдер',
      render: (row) => <span className="text-xs">{row.provider_name || '—'}</span>,
    },
    {
      key: 'client_name',
      header: 'Клиент',
      render: (row) => <span className="text-xs">{row.client_name || '—'}</span>,
    },
  ];

  return (
    <>
      <PageHeader
        title="Детализация"
        breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Детализация' }]}
      />

      {/* Фильтры */}
      <div className="bg-white border border-gray-200 rounded-lg p-4 mb-4">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-3">
          <Select
            label="Клиент"
            value={filters.client_id}
            onChange={(v) => setFilters((f) => ({ ...f, client_id: v }))}
            options={clientOptions}
          />
          <Select
            label="Статус"
            value={filters.status}
            onChange={(v) => setFilters((f) => ({ ...f, status: v }))}
            options={STATUS_OPTIONS}
          />
          <Select
            label="Провайдер"
            value={filters.provider_id}
            onChange={(v) => setFilters((f) => ({ ...f, provider_id: v }))}
            options={providerOptions}
          />
          <Input
            label="Отправитель"
            value={filters.source}
            onChange={(e) => setFilters((f) => ({ ...f, source: e.target.value }))}
            placeholder="Имя или номер"
          />
          <Input
            label="Получатель"
            value={filters.destination}
            onChange={(e) => setFilters((f) => ({ ...f, destination: e.target.value }))}
            placeholder="Номер телефона"
          />
          <Input
            label="Дата с"
            type="date"
            value={filters.date_from}
            onChange={(e) => setFilters((f) => ({ ...f, date_from: e.target.value }))}
          />
          <Input
            label="Дата по"
            type="date"
            value={filters.date_to}
            onChange={(e) => setFilters((f) => ({ ...f, date_to: e.target.value }))}
          />
        </div>
        <div className="flex gap-2">
          <Button onClick={applyFilters}>Применить</Button>
          <Button variant="ghost" onClick={resetFilters}>Сбросить</Button>
        </div>
      </div>

      {/* Таблица */}
      <div className="bg-white border border-gray-200 rounded-lg overflow-hidden">
        <DataTable
          columns={columns}
          data={messages}
          total={total}
          page={page}
          pageSize={PAGE_SIZE}
          onPageChange={setPage}
          loading={loading}
          rowActions={(row) => (
            <Button size="sm" variant="ghost" onClick={() => openDetail(row.id)}>
              Детали
            </Button>
          )}
        />
      </div>

      {/* Модальное окно детализации */}
      <Modal
        open={!!detail || detailLoading}
        onClose={() => setDetail(null)}
        title="Детализация сообщения"
        wide
      >
        {detailLoading && (
          <div className="text-center py-8 text-gray-400">Загрузка...</div>
        )}
        {detail && !detailLoading && (
          <div className="space-y-6">
            {/* Основная информация */}
            <section>
              <h3 className="text-sm font-semibold text-gray-700 mb-3">Основная информация</h3>
              <div className="grid grid-cols-2 gap-3">
                <DetailField label="ID" value={detail.id} mono />
                <DetailField label="Статус">
                  <Badge variant={STATUS_VARIANT[detail.status] ?? 'default'}>
                    {STATUS_LABEL[detail.status] ?? detail.status}
                  </Badge>
                </DetailField>
                <DetailField label="Отправитель" value={detail.source} mono />
                <DetailField label="Получатель" value={detail.destination} mono />
                {detail.client_name && <DetailField label="Клиент" value={detail.client_name} />}
                {detail.provider_name && <DetailField label="Провайдер" value={detail.provider_name} />}
                {detail.route_name && <DetailField label="Маршрут" value={detail.route_name} />}
                <DetailField label="Сегментов" value={String(detail.segment_count)} />
                {(detail.retry_count ?? 0) > 0 && (
                  <DetailField label="Попытки" value={`${detail.retry_count} / ${detail.max_retries ?? '—'}`} />
                )}
                {detail.encoding && <DetailField label="Кодировка" value={detail.encoding} />}
                {detail.external_id && <DetailField label="Внешний ID" value={detail.external_id} mono />}
                {detail.smpp_message_id && <DetailField label="SMPP ID" value={detail.smpp_message_id} mono />}
                {detail.created_at && <DetailField label="Создано" value={formatDate(detail.created_at)} />}
                {detail.submitted_at && <DetailField label="Отправлено" value={formatDate(detail.submitted_at)} />}
                {detail.delivered_at && <DetailField label="Доставлено" value={formatDate(detail.delivered_at)} />}
                {detail.failed_at && <DetailField label="Ошибка" value={formatDate(detail.failed_at)} />}
              </div>
              {detail.text && (
                <div className="mt-3">
                  <span className="text-xs text-gray-500">Текст сообщения</span>
                  <pre className="mt-1 text-sm bg-gray-50 border border-gray-100 rounded p-3 whitespace-pre-wrap break-words font-mono">
                    {detail.text}
                  </pre>
                </div>
              )}
              {detail.status_message && (
                <div className="mt-3">
                  <span className="text-xs text-gray-500">Статус (описание)</span>
                  <p className="mt-1 text-sm text-red-700">{detail.status_message}</p>
                </div>
              )}
            </section>

            {/* DLR от оператора */}
            {detail.dlr && (
              <section>
                <h3 className="text-sm font-semibold text-gray-700 mb-3">Ответ оператора (DLR)</h3>
                <div className="grid grid-cols-2 gap-3">
                  <DetailField label="Статус оператора">
                    <Badge variant={DLR_STAT_VARIANT[detail.dlr.stat] ?? 'default'}>
                      {DLR_STAT_LABEL[detail.dlr.stat] ?? detail.dlr.stat}
                    </Badge>
                  </DetailField>
                  {detail.dlr.err !== 0 && (
                    <DetailField label="Код ошибки" value={String(detail.dlr.err)} mono />
                  )}
                  {detail.dlr.submit_date && (
                    <DetailField label="Принято оператором" value={formatDate(detail.dlr.submit_date)} />
                  )}
                  {detail.dlr.done_date && (
                    <DetailField label="Статус получен" value={formatDate(detail.dlr.done_date)} />
                  )}
                  {detail.dlr.receipted_message_id && (
                    <DetailField label="ID оператора" value={detail.dlr.receipted_message_id} mono />
                  )}
                </div>
                {detail.dlr.text && (
                  <div className="mt-3">
                    <span className="text-xs text-gray-500">Текст от оператора</span>
                    <pre className="mt-1 text-sm bg-gray-50 border border-gray-100 rounded p-3 font-mono">
                      {detail.dlr.text}
                    </pre>
                  </div>
                )}
              </section>
            )}

            {/* Биллинг */}
            {detail.billing && (
              <section>
                <h3 className="text-sm font-semibold text-gray-700 mb-3">Биллинг</h3>
                <div className="grid grid-cols-3 gap-3">
                  <DetailField label="Сегментов" value={String(detail.billing.segment_count)} />
                  <DetailField label="Цена/сегмент" value={`${detail.billing.price_per_segment} ₽`} />
                  <DetailField label="Итого" value={`${detail.billing.total_amount} ₽`} />
                </div>
                <p className="mt-2 text-xs text-gray-400">
                  Списание: {formatDate(detail.billing.billed_at)}
                </p>
              </section>
            )}

            <div className="flex justify-end pt-2">
              <Button variant="ghost" onClick={() => setDetail(null)}>Закрыть</Button>
            </div>
          </div>
        )}
      </Modal>
    </>
  );
}

interface DetailFieldProps {
  label: string;
  value?: string;
  mono?: boolean;
  children?: React.ReactNode;
}

function DetailField({ label, value, mono, children }: DetailFieldProps) {
  return (
    <div>
      <span className="text-xs text-gray-500">{label}</span>
      {children ? (
        <div className="mt-0.5">{children}</div>
      ) : (
        <p className={`text-sm mt-0.5 break-all ${mono ? 'font-mono' : ''}`}>{value || '—'}</p>
      )}
    </div>
  );
}
