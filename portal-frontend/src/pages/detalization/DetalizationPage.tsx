import { useEffect, useState, useCallback } from 'react';
import { detalizationApi, type DetalizationMessage, type DetalizationMessageDetail } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';

const PAGE_SIZE = 50;

const STATUS_LABELS: Record<string, string> = {
  pending: 'Ожидает',
  queued: 'В очереди',
  sent: 'Отправлено',
  delivered: 'Доставлено',
  failed: 'Ошибка',
  expired: 'Истекло',
  rejected: 'Отклонено',
};

const DLR_STAT_LABELS: Record<string, string> = {
  DELIVRD: 'Доставлено',
  UNDELIV: 'Не доставлено',
  EXPIRED: 'Истекло',
  REJECTD: 'Отклонено',
  ACCEPTD: 'Принято',
  DELETED: 'Удалено',
  UNKNOWN: 'Неизвестно',
};

const STATUS_COLOR_MAP: Record<string, string> = {
  delivered: 'bg-green-100 text-green-800',
  sent: 'bg-blue-100 text-blue-800',
  failed: 'bg-red-100 text-red-800',
  rejected: 'bg-red-100 text-red-800',
  expired: 'bg-yellow-100 text-yellow-800',
  queued: 'bg-yellow-100 text-yellow-800',
  pending: 'bg-gray-100 text-gray-700',
};

function StatusBadge({ status }: { status: string }) {
  const label = STATUS_LABELS[status] || status;
  const cls = STATUS_COLOR_MAP[status] ?? 'bg-gray-100 text-gray-700';
  return (
    <span className={`inline-block rounded px-2 py-0.5 text-xs font-medium ${cls}`}>
      {label}
    </span>
  );
}

function formatDate(s: string | undefined | null): string {
  if (!s) return '—';
  return new Date(s).toLocaleString('ru-RU');
}

const DETALIZATION_FILTERS: FilterDef[] = [
  {
    key: 'status',
    label: 'Статус',
    type: 'select',
    options: [
      { value: 'pending', label: 'Ожидает' },
      { value: 'queued', label: 'В очереди' },
      { value: 'sent', label: 'Отправлено' },
      { value: 'delivered', label: 'Доставлено' },
      { value: 'failed', label: 'Ошибка' },
      { value: 'expired', label: 'Истекло' },
      { value: 'rejected', label: 'Отклонено' },
    ],
  },
  { key: 'date_from', label: 'С даты', type: 'date' },
  { key: 'date_to', label: 'По дату', type: 'date' },
  { key: 'destination', label: 'Получатель', type: 'text', placeholder: '+7...' },
];

const INITIAL_FILTERS: Record<string, string> = {
  status: '',
  date_from: '',
  date_to: '',
  destination: '',
};

const COLUMNS: Column<DetalizationMessage>[] = [
  {
    key: 'created_at',
    header: 'Дата',
    render: (msg) => (
      <span className="text-xs whitespace-nowrap">{formatDate(msg.created_at)}</span>
    ),
  },
  { key: 'source', header: 'Отправитель' },
  { key: 'destination', header: 'Получатель' },
  {
    key: 'text_preview',
    header: 'Текст',
    render: (msg) => (
      <span className="block max-w-[200px] truncate text-sm" title={msg.text_preview}>
        {msg.text_preview}
      </span>
    ),
  },
  {
    key: 'status',
    header: 'Статус',
    render: (msg) => <StatusBadge status={msg.status} />,
  },
  { key: 'segment_count', header: 'Сегменты' },
];

interface DetailRowProps {
  label: string;
  value: React.ReactNode;
}

function DetailRow({ label, value }: DetailRowProps) {
  return (
    <div className="flex gap-2 py-1 border-b border-gray-100 last:border-0">
      <span className="w-44 shrink-0 text-sm text-gray-500">{label}</span>
      <span className="text-sm text-gray-900 break-all">{value ?? '—'}</span>
    </div>
  );
}

function MessageDetailModal({
  messageId,
  onClose,
}: {
  messageId: string;
  onClose: () => void;
}) {
  const [detail, setDetail] = useState<DetalizationMessageDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    setError(null);
    detalizationApi
      .get(messageId)
      .then(setDetail)
      .catch((err) => setError(err.message || 'Ошибка загрузки'))
      .finally(() => setLoading(false));
  }, [messageId]);

  return (
    <Modal open onClose={onClose} title="Детали сообщения" description="Подробная информация о сообщении">
      {loading && <p className="text-sm text-gray-500 py-4 text-center">Загрузка...</p>}
      {error && <p className="text-sm text-red-600 py-4">{error}</p>}
      {detail && (
        <div className="space-y-4">
          {/* Main fields */}
          <section>
            <h3 className="text-sm font-semibold text-gray-700 mb-2">Основное</h3>
            <div className="divide-y divide-gray-100">
              <DetailRow label="ID" value={<span className="font-mono text-xs">{detail.id}</span>} />
              <DetailRow label="Статус" value={<StatusBadge status={detail.status} />} />
              {detail.status_message && (
                <DetailRow label="Статус (сообщение)" value={detail.status_message} />
              )}
              <DetailRow label="Отправитель" value={detail.source} />
              <DetailRow label="Получатель" value={detail.destination} />
              <DetailRow label="Текст" value={detail.text} />
              {detail.encoding && <DetailRow label="Кодировка" value={detail.encoding} />}
              <DetailRow label="Сегменты" value={detail.segment_count} />
              {detail.external_id && (
                <DetailRow label="Внешний ID" value={<span className="font-mono text-xs">{detail.external_id}</span>} />
              )}
              {detail.provider_name && <DetailRow label="Провайдер" value={detail.provider_name} />}
              {detail.route_name && <DetailRow label="Маршрут" value={detail.route_name} />}
            </div>
          </section>

          {/* Timestamps */}
          <section>
            <h3 className="text-sm font-semibold text-gray-700 mb-2">Временны́е метки</h3>
            <div className="divide-y divide-gray-100">
              <DetailRow label="Создано" value={formatDate(detail.created_at)} />
              <DetailRow label="Отправлено" value={formatDate(detail.submitted_at)} />
              <DetailRow label="Доставлено" value={formatDate(detail.delivered_at)} />
              <DetailRow label="Ошибка" value={formatDate(detail.failed_at)} />
              {detail.scheduled_at && (
                <DetailRow label="Запланировано" value={formatDate(detail.scheduled_at)} />
              )}
              {detail.expired_at && (
                <DetailRow label="Истекло" value={formatDate(detail.expired_at)} />
              )}
            </div>
          </section>

          {/* DLR */}
          {detail.dlr && (
            <section>
              <h3 className="text-sm font-semibold text-gray-700 mb-2">DLR</h3>
              <div className="divide-y divide-gray-100">
                <DetailRow
                  label="Статус DLR"
                  value={DLR_STAT_LABELS[detail.dlr.stat] ?? detail.dlr.stat}
                />
                <DetailRow label="Код ошибки" value={detail.dlr.err} />
                {detail.dlr.text && <DetailRow label="Текст DLR" value={detail.dlr.text} />}
                {detail.dlr.submit_date && (
                  <DetailRow label="Дата отправки" value={formatDate(detail.dlr.submit_date)} />
                )}
                {detail.dlr.done_date && (
                  <DetailRow label="Дата завершения" value={formatDate(detail.dlr.done_date)} />
                )}
                {detail.dlr.receipted_message_id && (
                  <DetailRow
                    label="ID квитанции"
                    value={<span className="font-mono text-xs">{detail.dlr.receipted_message_id}</span>}
                  />
                )}
              </div>
            </section>
          )}

          {/* Billing */}
          {detail.billing && (
            <section>
              <h3 className="text-sm font-semibold text-gray-700 mb-2">Биллинг</h3>
              <div className="divide-y divide-gray-100">
                <DetailRow label="Сегменты" value={detail.billing.segment_count} />
                <DetailRow label="Цена за сегмент" value={detail.billing.price_per_segment} />
                <DetailRow label="Итоговая сумма" value={detail.billing.total_amount} />
                <DetailRow
                  label="ID тарифного плана"
                  value={<span className="font-mono text-xs">{detail.billing.tariff_plan_id}</span>}
                />
                <DetailRow label="Дата списания" value={formatDate(detail.billing.billed_at)} />
              </div>
            </section>
          )}

          <div className="flex justify-end pt-2">
            <Button variant="secondary" onClick={onClose}>
              Закрыть
            </Button>
          </div>
        </div>
      )}
    </Modal>
  );
}

export function DetalizationPage() {
  const [data, setData] = useState<{
    messages: DetalizationMessage[];
    total: number;
    limit: number;
    offset: number;
  } | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [filterValues, setFilterValues] = useState<Record<string, string>>(INITIAL_FILTERS);
  const [selectedMessageId, setSelectedMessageId] = useState<string | null>(null);

  const fetchData = useCallback(() => {
    setLoading(true);
    setError(null);
    const offset = (page - 1) * PAGE_SIZE;
    detalizationApi
      .list({
        status: filterValues.status || undefined,
        destination: filterValues.destination || undefined,
        date_from: filterValues.date_from || undefined,
        date_to: filterValues.date_to || undefined,
        limit: PAGE_SIZE,
        offset,
      })
      .then(setData)
      .catch((err) => setError(err.message || 'Ошибка загрузки'))
      .finally(() => setLoading(false));
  }, [page, filterValues]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleFilterChange = (values: Record<string, string>) => {
    setFilterValues(values);
    setPage(1);
  };

  const handleFilterReset = () => {
    setFilterValues(INITIAL_FILTERS);
    setPage(1);
  };

  const columnsWithDetails: Column<DetalizationMessage>[] = [
    ...COLUMNS,
    {
      key: 'id',
      header: 'Детали',
      render: (msg) => (
        <Button
          variant="secondary"
          onClick={(e) => {
            e.stopPropagation();
            setSelectedMessageId(msg.id);
          }}
        >
          Детали
        </Button>
      ),
    },
  ];

  return (
    <div>
      <PageHeader title="Детализация" />

      <FilterBar
        filters={DETALIZATION_FILTERS}
        values={filterValues}
        onChange={handleFilterChange}
        onReset={handleFilterReset}
      />

      {error && <div className="text-red-600 mb-3">Ошибка: {error}</div>}

      {!loading && !error && (data?.messages?.length ?? 0) === 0 && (
        <div className="text-center py-12 text-gray-500">
          <p>Сообщений не найдено</p>
          <p className="text-sm mt-1">Измените параметры фильтра</p>
        </div>
      )}

      {(loading || (data?.messages?.length ?? 0) > 0) && (
        <DataTable<DetalizationMessage>
          columns={columnsWithDetails}
          data={data?.messages ?? []}
          total={data?.total ?? 0}
          page={page}
          pageSize={PAGE_SIZE}
          onPageChange={setPage}
          keyField="id"
          tableLabel="Детализация сообщений"
          loading={loading}
        />
      )}

      {selectedMessageId && (
        <MessageDetailModal
          messageId={selectedMessageId}
          onClose={() => setSelectedMessageId(null)}
        />
      )}
    </div>
  );
}
