import { useEffect, useState, useCallback } from 'react';
import { messagesApi } from '../../api/client';

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

export function MessagesPage() {
  const [data, setData] = useState<MessagesResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const [page, setPage] = useState(1);
  const [status, setStatus] = useState('');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [destination, setDestination] = useState('');

  const fetchMessages = useCallback(() => {
    setLoading(true);
    setError(null);

    const params: Record<string, string> = {
      page: String(page),
      per_page: '20',
    };
    if (status) params.status = status;
    if (dateFrom) params.date_from = dateFrom;
    if (dateTo) params.date_to = dateTo;
    if (destination) params.destination = destination;

    messagesApi
      .list(params)
      .then((resp) => setData(resp as MessagesResponse))
      .catch((err) => setError(err.message || 'Failed to load messages'))
      .finally(() => setLoading(false));
  }, [page, status, dateFrom, dateTo, destination]);

  useEffect(() => {
    fetchMessages();
  }, [fetchMessages]);

  const handleFilter = (e: React.FormEvent) => {
    e.preventDefault();
    setPage(1);
    fetchMessages();
  };

  return (
    <div>
      <h2>Messages</h2>

      <form onSubmit={handleFilter} style={{ marginBottom: 16, display: 'flex', gap: 12, flexWrap: 'wrap', alignItems: 'flex-end' }}>
        <label>
          Status
          <br />
          <select value={status} onChange={(e) => setStatus(e.target.value)} style={{ padding: 4 }}>
            <option value="">All</option>
            <option value="pending">Pending</option>
            <option value="queued">Queued</option>
            <option value="sent">Sent</option>
            <option value="delivered">Delivered</option>
            <option value="failed">Failed</option>
            <option value="expired">Expired</option>
            <option value="rejected">Rejected</option>
          </select>
        </label>

        <label>
          From
          <br />
          <input type="date" value={dateFrom} onChange={(e) => setDateFrom(e.target.value)} />
        </label>

        <label>
          To
          <br />
          <input type="date" value={dateTo} onChange={(e) => setDateTo(e.target.value)} />
        </label>

        <label>
          Destination
          <br />
          <input
            type="text"
            value={destination}
            onChange={(e) => setDestination(e.target.value)}
            placeholder="+7..."
            style={{ padding: 4 }}
          />
        </label>

        <button type="submit" style={{ padding: '4px 12px' }}>
          Filter
        </button>
      </form>

      {error && <div style={{ color: 'red', marginBottom: 12 }}>Error: {error}</div>}

      {loading ? (
        <div role="status">Loading...</div>
      ) : data ? (
        <>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <caption style={{ textAlign: 'left', marginBottom: 8, fontWeight: 'bold' }}>Messages list</caption>
            <thead>
              <tr>
                {['ID', 'Source', 'Destination', 'Text', 'Status', 'Segments', 'Created'].map((h) => (
                  <th key={h} style={{ borderBottom: '2px solid #ddd', padding: 8, textAlign: 'left' }}>
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {data.messages.length === 0 ? (
                <tr>
                  <td colSpan={7} style={{ padding: 16, textAlign: 'center', color: '#767676' }}>
                    No messages found
                  </td>
                </tr>
              ) : (
                data.messages.map((msg) => (
                  <tr key={msg.message_id}>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8, fontSize: 12, fontFamily: 'monospace' }}>
                      {msg.message_id.substring(0, 8)}...
                    </td>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8 }}>{msg.source}</td>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8 }}>{msg.destination}</td>
                    <td title={msg.text} style={{ borderBottom: '1px solid #eee', padding: 8, maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {msg.text}
                    </td>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8 }}>{msg.status}</td>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8 }}>{msg.segment_count}</td>
                    <td style={{ borderBottom: '1px solid #eee', padding: 8, fontSize: 12 }}>
                      {msg.created_at ? new Date(msg.created_at).toLocaleString() : '-'}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>

          <div style={{ marginTop: 16, display: 'flex', gap: 8, alignItems: 'center' }}>
            <button
              aria-label="Previous page"
              aria-disabled={page <= 1}
              disabled={page <= 1}
              onClick={() => setPage((p) => p - 1)}
              style={{ padding: '8px 12px' }}
            >
              Prev
            </button>
            <span aria-live="polite">
              Page {data.page} of {data.total_pages} (total: {data.total})
            </span>
            <button
              aria-label="Next page"
              aria-disabled={page >= data.total_pages}
              disabled={page >= data.total_pages}
              onClick={() => setPage((p) => p + 1)}
              style={{ padding: '8px 12px' }}
            >
              Next
            </button>
          </div>
        </>
      ) : null}
    </div>
  );
}
