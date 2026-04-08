import { useEffect, useState, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { messagesApi, exportApi } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
import type { BulkAction } from '../../components/data/BulkActionBar';
import { useMessageStream } from '../../hooks/useMessageStream';

interface MessageItem {
  message_id: string;
  source: string;
  destination: string;
  text: string;
  status: string;
  segment_count: number;
  created_at?: string;
  delivered_at?: string;
}

interface MessagesResponse {
  messages: MessageItem[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}

const STATUS_LABELS: Record<string, string> = {
  pending: 'Ожидание',
  queued: 'В очереди',
  sent: 'Отправлено',
  delivered: 'Доставлено',
  failed: 'Ошибка',
  expired: 'Истекло',
  rejected: 'Отклонено',
};

const MESSAGE_FILTERS: FilterDef[] = [
  { key: 'status', label: 'Статус', type: 'select', options: [
    { value: 'pending', label: 'Ожидание' },
    { value: 'queued', label: 'В очереди' },
    { value: 'sent', label: 'Отправлено' },
    { value: 'delivered', label: 'Доставлено' },
    { value: 'failed', label: 'Ошибка' },
    { value: 'expired', label: 'Истекло' },
    { value: 'rejected', label: 'Отклонено' },
  ]},
  { key: 'date_from', label: 'С даты', type: 'date' },
  { key: 'date_to', label: 'По дату', type: 'date' },
  { key: 'destination', label: 'Получатель', type: 'text', placeholder: '+7...' },
];

function StatusBadge({ status }: { status: string }) {
  const label = STATUS_LABELS[status] || status;
  const colorMap: Record<string, string> = {
    delivered: 'bg-green-100 text-green-800',
    sent: 'bg-blue-100 text-blue-800',
    failed: 'bg-red-100 text-red-800',
    rejected: 'bg-red-100 text-red-800',
    expired: 'bg-gray-100 text-gray-600',
    queued: 'bg-yellow-100 text-yellow-800',
    pending: 'bg-yellow-100 text-yellow-800',
  };
  const cls = colorMap[status] ?? 'bg-gray-100 text-gray-700';
  return <span className={`inline-block rounded px-2 py-0.5 text-xs font-medium ${cls}`}>{label}</span>;
}

function buildColumns(liveUpdates: Record<string, { status: string }>): Column<MessageItem>[] {
  return [
    { key: 'message_id', header: 'ID', render: (msg) => <span className="font-mono text-xs">{msg.message_id.substring(0, 8)}...</span> },
    { key: 'source', header: 'Отправитель' },
    { key: 'destination', header: 'Получатель' },
    { key: 'text', header: 'Текст', render: (msg) => <span className="block max-w-[200px] truncate" title={msg.text}>{msg.text}</span> },
    {
      key: 'status',
      header: 'Статус',
      render: (msg) => {
        const live = liveUpdates[msg.message_id];
        return <StatusBadge status={live ? live.status : msg.status} />;
      },
    },
    { key: 'segment_count', header: 'Сегменты' },
    { key: 'created_at', header: 'Дата создания', render: (msg) => <span className="text-xs">{msg.created_at ? new Date(msg.created_at).toLocaleString() : '-'}</span> },
  ];
}

const INITIAL_FILTERS: Record<string, string> = {
  status: '',
  date_from: '',
  date_to: '',
  destination: '',
};

export function MessagesPage() {
  const [data, setData] = useState<MessagesResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const [page, setPage] = useState(1);
  const [filterValues, setFilterValues] = useState<Record<string, string>>(INITIAL_FILTERS);

  // Real-time SSE stream for live status updates.
  const { streamStatus, updates: liveUpdates } = useMessageStream();

  const [exportJobId, setExportJobId] = useState<string | null>(null);
  const [exportStatus, setExportStatus] = useState<string | null>(null);
  const exportPollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const handleExportCsv = useCallback(async () => {
    setExportStatus('pending');
    const filters: Record<string, string> = {};
    if (filterValues.status) filters.status = filterValues.status;
    if (filterValues.date_from) filters.date_from = filterValues.date_from;
    if (filterValues.date_to) filters.date_to = filterValues.date_to;
    if (filterValues.destination) filters.destination = filterValues.destination;
    try {
      const { job_id } = await exportApi.start(filters);
      setExportJobId(job_id);
      exportPollRef.current = setInterval(async () => {
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
        }
      }, 2000);
    } catch {
      setExportStatus(null);
    }
  }, [filterValues]);

  const messageBulkActions: BulkAction<MessageItem>[] = [
    {
      label: 'Экспорт CSV',
      variant: 'secondary',
      onAction: () => handleExportCsv(),
    },
  ];

  const [showSendModal, setShowSendModal] = useState(false);
  const [sendDest, setSendDest] = useState('');
  const [sendText, setSendText] = useState('');
  const [sendSource, setSendSource] = useState('');
  const [sending, setSending] = useState(false);
  const [sendError, setSendError] = useState('');

  const fetchMessages = useCallback(() => {
    setLoading(true);
    setError(null);

    const params: Record<string, string> = {
      page: String(page),
      per_page: '20',
    };
    if (filterValues.status) params.status = filterValues.status;
    if (filterValues.date_from) params.date_from = filterValues.date_from;
    if (filterValues.date_to) params.date_to = filterValues.date_to;
    if (filterValues.destination) params.destination = filterValues.destination;

    messagesApi
      .list(params)
      .then((resp) => setData(resp as MessagesResponse))
      .catch((err) => setError(err.message || 'Failed to load messages'))
      .finally(() => setLoading(false));
  }, [page, filterValues]);

  useEffect(() => {
    fetchMessages();
  }, [fetchMessages]);

  const handleFilterChange = (values: Record<string, string>) => {
    setFilterValues(values);
    setPage(1);
  };

  const handleReset = () => {
    setFilterValues(INITIAL_FILTERS);
    setPage(1);
  };

  const handleSendSMS = async () => {
    setSendError('');
    const phoneRegex = /^\+?[0-9]{10,15}$/;
    if (!phoneRegex.test(sendDest.replace(/\s/g, ''))) {
      setSendError('Введите корректный номер телефона (например, +79001234567)');
      return;
    }
    if (!sendSource.trim()) {
      setSendError('Укажите Sender ID');
      return;
    }
    setSending(true);
    try {
      await messagesApi.send({ destination: sendDest, text: sendText, source: sendSource || undefined });
      setShowSendModal(false);
      setSendDest('');
      setSendText('');
      setSendSource('');
      fetchMessages();
    } catch (err) {
      const msg = err instanceof Error ? err.message : '';
      if (msg.includes('source is required')) {
        setSendError('Укажите Sender ID');
      } else if (msg.includes('destination')) {
        setSendError('Некорректный номер получателя');
      } else {
        setSendError(msg || 'Ошибка отправки');
      }
    } finally {
      setSending(false);
    }
  };

  const navigate = useNavigate();
  const columns = buildColumns(liveUpdates);

  return (
    <div>
      <PageHeader
        title="Сообщения"
        actions={
          <div className="flex gap-2 items-center">
            {streamStatus === 'connected' && (
              <span className="flex items-center gap-1 text-xs text-green-600" title="Статусы обновляются в реальном времени">
                <span className="inline-block w-2 h-2 rounded-full bg-green-500 animate-pulse" />
                Live
              </span>
            )}
            {exportStatus && exportStatus !== 'ready' && (
              <span className="text-sm text-gray-500 self-center">Экспорт: {exportStatus}...</span>
            )}
            <Button
              variant="secondary"
              onClick={handleExportCsv}
              disabled={!!exportJobId || (data?.total ?? 0) === 0}
            >
              Экспорт CSV
            </Button>
            <Button onClick={() => setShowSendModal(true)}>Отправить SMS</Button>
          </div>
        }
      />

      <Modal open={showSendModal} onClose={() => setShowSendModal(false)} title="Отправить SMS" description="Отправка тестового SMS сообщения">
        <div className="space-y-3">
          {sendError && <p role="alert" className="text-red-600 text-sm">{sendError}</p>}
          <Input label="Номер получателя *" value={sendDest} onChange={(e) => setSendDest(e.target.value)} placeholder="+79001234567" required />
          <Input label="Sender ID *" value={sendSource} onChange={(e) => setSendSource(e.target.value)} placeholder="MyCompany" required />
          <div>
            <label htmlFor="sms-text" className="block text-sm font-medium text-gray-700 mb-1">Текст сообщения *</label>
            <textarea id="sms-text" className="w-full rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary" rows={3} value={sendText} onChange={(e) => setSendText(e.target.value)} required />
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" onClick={() => setShowSendModal(false)}>Отмена</Button>
            <Button onClick={handleSendSMS} disabled={sending || !sendDest || !sendText}>{sending ? 'Отправка...' : 'Отправить'}</Button>
          </div>
        </div>
      </Modal>

      <FilterBar
        filters={MESSAGE_FILTERS}
        values={filterValues}
        onChange={handleFilterChange}
        onReset={handleReset}
      />

      {error && <div className="text-red-600 mb-3">Ошибка: {error}</div>}

      {!loading && !error && (data?.messages?.length ?? 0) === 0 && (
        <div className="text-center py-12 text-gray-500">
          <p className="mb-2">Сообщений пока нет</p>
          <p className="text-sm mb-4">Отправьте первое SMS-сообщение</p>
          <Button onClick={() => setShowSendModal(true)}>Отправить SMS</Button>
        </div>
      )}

      {(loading || (data?.messages?.length ?? 0) > 0) && (
        <DataTable
          columns={columns}
          data={data?.messages ?? []}
          total={data?.total ?? 0}
          page={page}
          pageSize={20}
          onPageChange={setPage}
          keyField="message_id"
          tableLabel="Список SMS сообщений"
          loading={loading}
          bulkActions={messageBulkActions}
          onRowClick={(msg) => navigate(`/messages/${msg.message_id}`)}
        />
      )}
    </div>
  );
}
