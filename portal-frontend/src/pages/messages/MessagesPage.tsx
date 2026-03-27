import { useEffect, useState, useCallback } from 'react';
import { messagesApi } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { DataTable, type Column } from '../../components/data/DataTable';

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

  return (
    <div>
      <PageHeader title="Messages" />

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
