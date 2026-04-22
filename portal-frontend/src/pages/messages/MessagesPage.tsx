import { useCallback, useEffect, useRef, useState } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { detalizationApi, exportApi, referencesApi, senderNamesApi } from '../../api/client';
import type { DetalizationMessage } from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';
import { MessageFilters } from './components/MessageFilters';
import type { FilterDef } from './components/MessageFilters';
import { ActiveFilterChips } from './components/ActiveFilterChips';
import { MessageTable, ALL_COLUMNS } from './components/MessageTable';
import type { SortField, SortState } from './components/MessageTable';
import { MessageModal } from './components/MessageModal';
import { ColumnConfigurator } from './components/ColumnConfigurator';
import type { ColumnDef } from './components/ColumnConfigurator';

const STORAGE_KEY = 'messages_visible_columns';
const DEFAULT_VISIBLE_RESELLER = new Set([
  'login', 'destination', 'operator_name', 'channel',
  'text_preview', 'submitted_at', 'status', 'total_amount',
]);
const DEFAULT_VISIBLE_CLIENT = new Set([
  'destination', 'operator_name', 'channel',
  'text_preview', 'submitted_at', 'status', 'total_amount',
]);

function loadVisibleColumns(isReseller: boolean): Set<string> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return new Set(JSON.parse(raw) as string[]);
  } catch { /* ignore */ }
  return new Set(isReseller ? DEFAULT_VISIBLE_RESELLER : DEFAULT_VISIBLE_CLIENT);
}

function saveVisibleColumns(cols: Set<string>) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify([...cols]));
  } catch { /* ignore */ }
}

const STATUS_OPTIONS = [
  { value: 'pending', label: 'Ожидание' },
  { value: 'queued', label: 'В очереди' },
  { value: 'sent', label: 'Отправлено' },
  { value: 'delivered', label: 'Доставлено' },
  { value: 'failed', label: 'Ошибка' },
  { value: 'expired', label: 'Истекло' },
  { value: 'rejected', label: 'Отклонено' },
  { value: 'scheduled', label: 'Запланировано' },
];

const CHANNEL_OPTIONS = [
  { value: 'SMS', label: 'SMS' },
  { value: 'MAX', label: 'MAX' },
  { value: 'Viber', label: 'Viber' },
];

const SEND_METHOD_OPTIONS = [
  { value: 'PORTAL', label: 'ЛК' },
  { value: 'API', label: 'API' },
  { value: 'SMPP', label: 'SMPP' },
];

function getDefaultDateRange() {
  const to = new Date();
  const from = new Date();
  from.setDate(from.getDate() - 7);
  const fmt = (d: Date) => d.toISOString().slice(0, 10);
  return { date_from: fmt(from), date_to: fmt(to) };
}

const DEFAULT_DATE_RANGE = getDefaultDateRange();

const EMPTY_FILTERS: Record<string, string> = {
  date_from: DEFAULT_DATE_RANGE.date_from, date_to: DEFAULT_DATE_RANGE.date_to, destination: '', login: '',
  status: '', operator: '', sender_name: '', channel: '',
  message_id: '', send_method: '', country: '',
};

export function MessagesPage() {
  const { user } = useAuth();
  const isReseller = !!user?.is_reseller;
  const [data, setData] = useState<{ messages: DetalizationMessage[]; total: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [appliedFilters, setAppliedFilters] = useState<Record<string, string>>(EMPTY_FILTERS);
  const [sort, setSort] = useState<SortState>({ field: 'submitted_at', order: 'desc' });
  const [selectedMsg, setSelectedMsg] = useState<DetalizationMessage | null>(null);
  const [visibleColumns, setVisibleColumns] = useState<Set<string>>(() => loadVisibleColumns(isReseller));

  const [exportJobId, setExportJobId] = useState<string | null>(null);
  const [exportStatus, setExportStatus] = useState<string | null>(null);
  const [exportError, setExportError] = useState<string | null>(null);
  const exportPollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const [operatorOptions, setOperatorOptions] = useState<Array<{ value: string; label: string }>>([]);
  const [countryOptions, setCountryOptions] = useState<Array<{ value: string; label: string }>>([]);
  const [senderNameOptions, setSenderNameOptions] = useState<Array<{ value: string; label: string }>>([]);

  useEffect(() => {
    return () => {
      if (exportPollRef.current) clearInterval(exportPollRef.current);
    };
  }, []);

  useEffect(() => {
    referencesApi.operators().then((r) =>
      setOperatorOptions(r.operators.map((o) => ({ value: o.name, label: o.name })))
    ).catch(() => {});
    referencesApi.countries().then((r) =>
      setCountryOptions(r.countries.map((c) => ({ value: c.name, label: c.name })))
    ).catch(() => {});
    // Load approved sender names for dropdown
    senderNamesApi.listApproved()
      .then((r) => {
        setSenderNameOptions((r.sender_names ?? []).map((s) => ({ value: s.name, label: s.name })));
      })
      .catch(() => {});
  }, []);

  const primaryFilters: FilterDef[] = [
    { key: 'date_from', label: 'Дата от', type: 'date' },
    { key: 'date_to', label: 'Дата до', type: 'date' },
    { key: 'destination', label: 'Номер', type: 'text', placeholder: '+7...' },
    ...(isReseller ? [{ key: 'login' as const, label: 'Суб-аккаунт', type: 'text' as const, placeholder: 'Имя суб-аккаунта' }] : []),
    { key: 'status', label: 'Статус', type: 'select', options: STATUS_OPTIONS },
    { key: 'operator', label: 'Оператор', type: 'select', options: operatorOptions },
    { key: 'sender_name', label: 'Имя отправителя', type: 'select', options: senderNameOptions },
    { key: 'channel', label: 'Канал', type: 'select', options: CHANNEL_OPTIONS },
  ];

  const secondaryFilters: FilterDef[] = [
    { key: 'message_id', label: 'ID сообщения', type: 'text', placeholder: 'Поиск по ID' },
    { key: 'send_method', label: 'Способ отправки', type: 'select', options: SEND_METHOD_OPTIONS },
    { key: 'country', label: 'Страна', type: 'select', options: countryOptions },
  ];

  const allFilterDefs = [...primaryFilters, ...secondaryFilters];

  const fetchData = useCallback(() => {
    setLoading(true);
    setError(null);
    const activeFilters = Object.fromEntries(Object.entries(appliedFilters).filter(([, v]) => v !== ''));
    detalizationApi.list({
      ...activeFilters,
      limit: pageSize,
      offset: (page - 1) * pageSize,
      sort_by: sort.field,
      sort_order: sort.order,
    }).then((r) => setData({ messages: r.messages, total: r.total }))
      .catch((err) => setError(err instanceof Error ? err.message : 'Ошибка загрузки'))
      .finally(() => setLoading(false));
  }, [appliedFilters, page, pageSize, sort]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const handleSearch = (values: Record<string, string>) => {
    setAppliedFilters(values);
    setPage(1);
  };

  const handleReset = () => {
    setAppliedFilters(EMPTY_FILTERS);
    setPage(1);
  };

  const handleRemoveChip = (key: string) => {
    setAppliedFilters((prev) => ({ ...prev, [key]: '' }));
    setPage(1);
  };

  const handleSort = (field: SortField) => {
    setSort((prev) =>
      prev.field === field
        ? { field, order: prev.order === 'asc' ? 'desc' : 'asc' }
        : { field, order: 'desc' }
    );
    setPage(1);
  };

  const handleColumnsChange = (cols: Set<string>) => {
    setVisibleColumns(cols);
    saveVisibleColumns(cols);
  };

  const handleExport = useCallback(async () => {
    if (exportJobId) return;
    setExportStatus('pending');
    setExportError(null);
    const filters: Record<string, string> = {};
    Object.entries(appliedFilters).forEach(([k, v]) => { if (v) filters[k] = v; });
    try {
      const { job_id } = await exportApi.start(filters);
      setExportJobId(job_id);
      exportPollRef.current = setInterval(async () => {
        try {
          const job = await exportApi.getStatus(job_id);
          setExportStatus(job.status);
          if (job.status === 'ready') {
            clearInterval(exportPollRef.current!);
            const res = await exportApi.download(job_id);
            const blob = await res.blob();
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `messages_export_${job_id.slice(0, 8)}.csv`;
            a.click();
            URL.revokeObjectURL(url);
            setExportStatus(null);
            setExportJobId(null);
          } else if (job.status === 'error') {
            clearInterval(exportPollRef.current!);
            setExportStatus(null);
            setExportJobId(null);
            setExportError('Ошибка при создании экспорта. Попробуйте снова.');
          }
        } catch {
          clearInterval(exportPollRef.current!);
          setExportStatus(null);
          setExportJobId(null);
        }
      }, 2000);
    } catch {
      setExportStatus(null);
      setExportError('Не удалось запустить экспорт. Попробуйте снова.');
    }
  }, [appliedFilters, exportJobId]);

  const defaultVisibleRef = isReseller ? DEFAULT_VISIBLE_RESELLER : DEFAULT_VISIBLE_CLIENT;
  const columnDefs: ColumnDef[] = ALL_COLUMNS.map((c) => ({
    key: c.key,
    label: c.header,
    defaultVisible: defaultVisibleRef.has(c.key),
  }));

  return (
    <div>
      <PageHeader title="Сообщения" />
      <p className="text-sm text-muted-foreground mb-4">
        {isReseller
          ? 'Детализация трафика по всем клиентам и каналам'
          : 'Детализация отправленных вами сообщений по каналам'}
      </p>

      {isReseller && (
        <div className="mb-4 bg-blue-50 border border-blue-200 rounded-lg px-4 py-3 text-sm text-blue-800">
          Отображаются сообщения вашего аккаунта и всех суб-аккаунтов. Используйте фильтр «Суб-аккаунт» для просмотра трафика конкретного суб-аккаунта.
        </div>
      )}

      <MessageFilters
        primary={primaryFilters}
        secondary={secondaryFilters}
        values={appliedFilters}
        onSearch={handleSearch}
        onReset={handleReset}
      />

      <ActiveFilterChips
        filterDefs={allFilterDefs}
        values={appliedFilters}
        onRemove={handleRemoveChip}
      />

      {error && <div role="alert" className="text-red-600 mb-3 text-sm">Ошибка: {error}</div>}

      <div className="flex items-center justify-between mb-2">
        <span className="text-sm text-gray-500" aria-live="polite" aria-atomic="true">
          {loading ? (
            <span role="status" className="inline-flex items-center gap-1.5">
              <svg className="animate-spin w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" aria-hidden="true">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
              </svg>
              Загрузка...
            </span>
          ) : data != null ? (
            <>Найдено: <strong>{data.total}</strong> сообщений</>
          ) : null}
        </span>
        <div className="flex gap-2">
          {exportStatus && exportStatus !== 'ready' && (
            <span role="status" className="text-sm text-gray-400 self-center">Экспорт...</span>
          )}
          <div className="flex flex-col items-end">
            <Button
              variant="secondary"
              onClick={handleExport}
              disabled={!!exportJobId || (data?.total ?? 0) === 0}
            >
              Экспорт
            </Button>
            {exportError && (
              <p role="alert" className="text-sm text-red-600 mt-1">{exportError}</p>
            )}
          </div>
          <ColumnConfigurator
            columns={columnDefs}
            visible={visibleColumns}
            onChange={handleColumnsChange}
          />
        </div>
      </div>

      <MessageTable
        data={data?.messages ?? []}
        total={data?.total ?? 0}
        page={page}
        pageSize={pageSize}
        onPageChange={setPage}
        onPageSizeChange={(s) => { setPageSize(s); setPage(1); }}
        visibleColumns={visibleColumns}
        sort={sort}
        onSort={handleSort}
        onRowClick={setSelectedMsg}
        loading={loading}
      />

      {selectedMsg && (
        <MessageModal message={selectedMsg} onClose={() => setSelectedMsg(null)} />
      )}
    </div>
  );
}
