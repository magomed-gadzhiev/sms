import { useEffect, useState, useCallback } from 'react';
import { messagesApi } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';

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

const MESSAGE_FILTERS: FilterDef[] = [
  { key: 'status', label: 'Status', type: 'select', options: [
    { value: '', label: 'All' },
    { value: 'pending', label: 'Pending' },
    { value: 'queued', label: 'Queued' },
    { value: 'sent', label: 'Sent' },
    { value: 'delivered', label: 'Delivered' },
    { value: 'failed', label: 'Failed' },
    { value: 'expired', label: 'Expired' },
    { value: 'rejected', label: 'Rejected' },
  ]},
  { key: 'date_from', label: 'From', type: 'date' },
  { key: 'date_to', label: 'To', type: 'date' },
  { key: 'destination', label: 'Destination', type: 'text', placeholder: '+7...' },
];

const columns: Column<MessageItem>[] = [
  { key: 'message_id', header: 'ID', render: (msg) => <span className="font-mono text-xs">{msg.message_id.substring(0, 8)}...</span> },
  { key: 'source', header: 'Source' },
  { key: 'destination', header: 'Destination' },
  { key: 'text', header: 'Text', render: (msg) => <span className="block max-w-[200px] truncate" title={msg.text}>{msg.text}</span> },
  { key: 'status', header: 'Status' },
  { key: 'segment_count', header: 'Segments' },
  { key: 'created_at', header: 'Created', render: (msg) => <span className="text-xs">{msg.created_at ? new Date(msg.created_at).toLocaleString() : '-'}</span> },
];

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
    setSending(true);
    setSendError('');
    try {
      await messagesApi.send({ destination: sendDest, text: sendText, source: sendSource || undefined });
      setShowSendModal(false);
      setSendDest('');
      setSendText('');
      setSendSource('');
      fetchMessages();
    } catch (err) {
      setSendError(err instanceof Error ? err.message : 'Ошибка отправки');
    } finally {
      setSending(false);
    }
  };

  return (
    <div>
      <PageHeader
        title="Messages"
        actions={<Button onClick={() => setShowSendModal(true)}>Отправить SMS</Button>}
      />

      <Modal open={showSendModal} onClose={() => setShowSendModal(false)} title="Отправить SMS" description="Отправка тестового SMS сообщения">
        <div className="space-y-3">
          {sendError && <p role="alert" className="text-red-600 text-sm">{sendError}</p>}
          <Input label="Номер получателя" value={sendDest} onChange={(e) => setSendDest(e.target.value)} placeholder="+79001234567" required />
          <Input label="Sender ID" value={sendSource} onChange={(e) => setSendSource(e.target.value)} placeholder="MyCompany" />
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Текст сообщения</label>
            <textarea className="w-full rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary" rows={3} value={sendText} onChange={(e) => setSendText(e.target.value)} required />
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

      {error && <div className="text-red-600 mb-3">Error: {error}</div>}

      <DataTable
        columns={columns}
        data={data?.messages ?? []}
        total={data?.total ?? 0}
        page={page}
        pageSize={20}
        onPageChange={setPage}
        keyField="message_id"
        loading={loading}
      />
    </div>
  );
}
