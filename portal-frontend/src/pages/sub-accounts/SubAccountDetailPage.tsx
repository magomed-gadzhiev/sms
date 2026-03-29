import { useState, useEffect, useCallback, type FormEvent } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { subAccountsApi, ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { DataTable, type Column } from '../../components/data/DataTable';
import { StatCard } from '../../components/data/StatCard';
import { StatusBadge, Badge } from '../../components/ui/Badge';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { Select } from '../../components/ui/Select';

type TabName = 'overview' | 'messages' | 'analytics' | 'api-keys' | 'webhooks';

const TABS: { key: TabName; label: string }[] = [
  { key: 'overview', label: 'Обзор' },
  { key: 'messages', label: 'Сообщения' },
  { key: 'analytics', label: 'Аналитика' },
  { key: 'api-keys', label: 'API Ключи' },
  { key: 'webhooks', label: 'Вебхуки' },
];

interface SubAccountDetail {
  id: string;
  name: string;
  email: string;
  contact_person: string;
  active: boolean;
  balance: string;
  daily_limit: number;
  monthly_limit: number;
  messages_today: number;
  messages_this_month: number;
  api_keys: APIKeyItem[];
  webhooks: WebhookItem[];
}

interface APIKeyItem {
  id: string;
  name: string;
  prefix: string;
  active: boolean;
  created_at: string;
}

interface WebhookItem {
  id: string;
  url: string;
  event_types: string[];
  active: boolean;
  created_at?: string;
}

interface MessageItem {
  message_id: string;
  source: string;
  destination: string;
  text: string;
  status: string;
  segment_count: number;
  created_at?: string;
}

interface MessagesResponse {
  messages: MessageItem[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}

interface AnalyticsSummary {
  total_sent: number;
  total_delivered: number;
  total_failed: number;
  delivery_rate: number;
  total_cost: string;
  currency: string;
}

interface TimelineEntry {
  period: string;
  sent: number;
  delivered: number;
  failed: number;
  delivery_rate: number;
}

interface AnalyticsData {
  summary: AnalyticsSummary;
  timeline: TimelineEntry[];
}

export function SubAccountDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<TabName>('overview');
  const [detail, setDetail] = useState<SubAccountDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  async function loadDetail() {
    if (!id) return;
    setLoading(true);
    setError('');
    try {
      const resp = await subAccountsApi.get(id);
      setDetail(resp as SubAccountDetail);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить суб-аккаунт');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadDetail();
  }, [id]);

  if (loading) return <div role="status">Загрузка...</div>;
  if (error) return <div className="text-red-600">{error}</div>;
  if (!detail) return <div>Суб-аккаунт не найден</div>;

  return (
    <div className="max-w-5xl">
      <PageHeader
        title={detail.name}
        breadcrumbs={[{ label: 'Суб-аккаунты', href: '/sub-accounts' }, { label: detail.name }]}
        actions={<StatusBadge status={detail.active ? 'active' : 'inactive'} />}
      />

      {/* Tabs */}
      <div className="flex border-b border-gray-200 mb-4">
        {TABS.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`px-5 py-2 -mb-px text-sm font-medium border-b-2 transition-colors ${
              activeTab === tab.key
                ? 'border-primary text-primary'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {activeTab === 'overview' && (
        <OverviewTab detail={detail} onUpdate={loadDetail} onDeleted={() => navigate('/sub-accounts')} />
      )}
      {activeTab === 'messages' && id && <MessagesTab subAccountId={id} />}
      {activeTab === 'analytics' && id && <AnalyticsTab subAccountId={id} />}
      {activeTab === 'api-keys' && <APIKeysTab keys={detail.api_keys || []} />}
      {activeTab === 'webhooks' && <WebhooksTab webhooks={detail.webhooks || []} />}
    </div>
  );
}

// ---- Overview Tab ----

function OverviewTab({
  detail,
  onUpdate,
  onDeleted,
}: {
  detail: SubAccountDetail;
  onUpdate: () => void;
  onDeleted: () => void;
}) {
  const [error, setError] = useState('');

  // Edit limits
  const [dailyLimit, setDailyLimit] = useState(String(detail.daily_limit));
  const [monthlyLimit, setMonthlyLimit] = useState(String(detail.monthly_limit));
  const [savingLimits, setSavingLimits] = useState(false);

  // Balance transfer
  const [transferAmount, setTransferAmount] = useState('');
  const [transferring, setTransferring] = useState(false);

  // Delete confirmation
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);

  async function handleSaveLimits(e: FormEvent) {
    e.preventDefault();
    setSavingLimits(true);
    setError('');
    try {
      await subAccountsApi.updateLimits(detail.id, {
        daily_limit: Number(dailyLimit),
        monthly_limit: Number(monthlyLimit),
      });
      onUpdate();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось обновить лимиты');
    } finally {
      setSavingLimits(false);
    }
  }

  async function handleTransfer(e: FormEvent) {
    e.preventDefault();
    setTransferring(true);
    setError('');
    try {
      await subAccountsApi.transfer(detail.id, transferAmount);
      setTransferAmount('');
      onUpdate();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось перевести средства');
    } finally {
      setTransferring(false);
    }
  }

  async function handleDelete() {
    setDeleting(true);
    setError('');
    try {
      await subAccountsApi.remove(detail.id);
      onDeleted();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось удалить суб-аккаунт');
      setDeleting(false);
    }
  }

  return (
    <div>
      {error && <p className="text-red-600">{error}</p>}

      {/* Info cards */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-6">
        <StatCard title="Баланс" value={detail.balance} />
        <StatCard title="Дневной лимит" value={`${detail.messages_today} / ${detail.daily_limit}`} />
        <StatCard title="Месячный лимит" value={`${detail.messages_this_month} / ${detail.monthly_limit}`} />
        <StatCard title="Email" value={detail.email} />
        <StatCard title="Контакт" value={detail.contact_person || '-'} />
      </div>

      {/* Edit limits form */}
      <div className="bg-gray-50 border border-gray-200 rounded p-4 mb-4">
        <h3 className="mt-0 text-base font-semibold mb-3">Изменить лимиты</h3>
        <form onSubmit={handleSaveLimits} className="flex gap-4 items-end flex-wrap">
          <Input
            label="Дневной лимит"
            type="number"
            value={dailyLimit}
            onChange={(e) => setDailyLimit(e.target.value)}
            className="w-40"
          />
          <Input
            label="Месячный лимит"
            type="number"
            value={monthlyLimit}
            onChange={(e) => setMonthlyLimit(e.target.value)}
            className="w-40"
          />
          <Button type="submit" disabled={savingLimits}>
            {savingLimits ? 'Сохранение...' : 'Сохранить лимиты'}
          </Button>
        </form>
      </div>

      {/* Balance transfer form */}
      <div className="bg-gray-50 border border-gray-200 rounded p-4 mb-4">
        <h3 className="mt-0 text-base font-semibold mb-3">Перевод средств</h3>
        <form onSubmit={handleTransfer} className="flex gap-4 items-end">
          <Input
            label="Сумма"
            type="text"
            value={transferAmount}
            onChange={(e) => setTransferAmount(e.target.value)}
            placeholder="0.00"
            required
            className="w-40"
          />
          <Button type="submit" disabled={transferring || !transferAmount}>
            {transferring ? 'Перевод...' : 'Перевести'}
          </Button>
        </form>
      </div>

      {/* Delete section */}
      <div className="border border-red-600 rounded p-4">
        <h3 className="mt-0 text-red-600 text-base font-semibold mb-3">Зона риска</h3>
        <Button variant="danger" onClick={() => setConfirmDelete(true)}>
          Удалить суб-аккаунт
        </Button>
      </div>

      <ConfirmDialog
        open={confirmDelete}
        onConfirm={handleDelete}
        onCancel={() => setConfirmDelete(false)}
        title="Удалить суб-аккаунт"
        description="Вы уверены, что хотите удалить этот суб-аккаунт? Это действие нельзя отменить."
        confirmLabel="Да, удалить"
        variant="danger"
        loading={deleting}
      />
    </div>
  );
}

// ---- Messages Tab ----

const messageColumns: Column<MessageItem>[] = [
  {
    key: 'message_id',
    header: 'ID',
    render: (msg) => (
      <span className="text-xs font-mono">{msg.message_id.substring(0, 8)}...</span>
    ),
  },
  { key: 'source', header: 'Отправитель' },
  { key: 'destination', header: 'Получатель' },
  {
    key: 'text',
    header: 'Текст',
    render: (msg) => (
      <span className="block max-w-[200px] overflow-hidden text-ellipsis whitespace-nowrap">
        {msg.text}
      </span>
    ),
  },
  {
    key: 'status',
    header: 'Статус',
    render: (msg) => <StatusBadge status={msg.status} />,
  },
  { key: 'segment_count', header: 'Сегменты' },
  {
    key: 'created_at',
    header: 'Создан',
    render: (msg) => (
      <span className="text-xs">
        {msg.created_at ? new Date(msg.created_at).toLocaleString() : '-'}
      </span>
    ),
  },
];

const statusOptions = [
  { value: '', label: 'Все' },
  { value: 'pending', label: 'Ожидание' },
  { value: 'sent', label: 'Отправлено' },
  { value: 'delivered', label: 'Доставлено' },
  { value: 'failed', label: 'Ошибка' },
];

function MessagesTab({ subAccountId }: { subAccountId: string }) {
  const [data, setData] = useState<MessagesResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState('');

  const fetchMessages = useCallback(() => {
    setLoading(true);
    setError('');
    const params: Record<string, string> = { page: String(page), per_page: '20' };
    if (status) params.status = status;

    subAccountsApi
      .messages(subAccountId, params)
      .then((resp) => setData(resp as MessagesResponse))
      .catch((err) => setError(err instanceof ApiError ? err.message : 'Не удалось загрузить сообщения'))
      .finally(() => setLoading(false));
  }, [subAccountId, page, status]);

  useEffect(() => {
    fetchMessages();
  }, [fetchMessages]);

  return (
    <div>
      <div className="flex gap-3 mb-4 items-end">
        <Select
          label="Статус"
          value={status}
          options={statusOptions}
          onChange={(val) => { setStatus(val); setPage(1); }}
        />
      </div>

      {error && <p className="text-red-600">{error}</p>}

      <DataTable<MessageItem>
        columns={messageColumns}
        data={data?.messages ?? []}
        total={data?.total ?? 0}
        page={page}
        pageSize={20}
        onPageChange={setPage}
        loading={loading}
        keyField="message_id"
      />
    </div>
  );
}

// ---- Analytics Tab ----

const PERIOD_OPTIONS = ['7d', '30d', '90d'] as const;

const timelineColumns: Column<TimelineEntry>[] = [
  { key: 'period', header: 'Период' },
  { key: 'sent', header: 'Отправлено' },
  { key: 'delivered', header: 'Доставлено' },
  { key: 'failed', header: 'Ошибки' },
  {
    key: 'delivery_rate',
    header: 'Доставляемость',
    render: (row) => <>{row.delivery_rate}%</>,
  },
];

function AnalyticsTab({ subAccountId }: { subAccountId: string }) {
  const [data, setData] = useState<AnalyticsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [period, setPeriod] = useState('7d');

  useEffect(() => {
    setLoading(true);
    setError('');
    subAccountsApi
      .analytics(subAccountId, { period, group_by: 'day' })
      .then((resp) => setData(resp as AnalyticsData))
      .catch((err) => setError(err instanceof ApiError ? err.message : 'Не удалось загрузить аналитику'))
      .finally(() => setLoading(false));
  }, [subAccountId, period]);

  return (
    <div>
      <div className="flex gap-1 mb-4">
        {PERIOD_OPTIONS.map((p) => (
          <button
            key={p}
            onClick={() => setPeriod(p)}
            className={`px-4 py-1.5 text-sm font-medium rounded transition-colors ${
              period === p
                ? 'bg-primary text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
            }`}
          >
            {p}
          </button>
        ))}
      </div>

      {error && <p className="text-red-600">{error}</p>}
      {loading && <div role="status">Загрузка аналитики...</div>}

      {data && !loading && (
        <>
          <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-6">
            <StatCard title="Отправлено" value={data.summary.total_sent} />
            <StatCard title="Доставлено" value={data.summary.total_delivered} />
            <StatCard title="Ошибки" value={data.summary.total_failed} />
            <StatCard title="Доставляемость" value={`${data.summary.delivery_rate}%`} />
            <StatCard
              title="Стоимость"
              value={data.summary.total_cost ? `${data.summary.total_cost} ${data.summary.currency}` : 'N/A'}
            />
          </div>

          {data.timeline.length > 0 && (
            <DataTable<TimelineEntry>
              columns={timelineColumns}
              data={data.timeline}
              total={data.timeline.length}
              page={1}
              pageSize={data.timeline.length}
              onPageChange={() => {}}
              keyField="period"
            />
          )}
        </>
      )}
    </div>
  );
}

// ---- API Keys Tab ----

const apiKeyColumns: Column<APIKeyItem>[] = [
  { key: 'name', header: 'Название' },
  {
    key: 'prefix',
    header: 'Префикс',
    render: (k) => <code className="text-xs bg-gray-100 px-1.5 py-0.5 rounded">{k.prefix}...</code>,
  },
  {
    key: 'active',
    header: 'Статус',
    render: (k) => <StatusBadge status={k.active ? 'active' : 'inactive'} />,
  },
  {
    key: 'created_at',
    header: 'Создан',
    render: (k) => <>{new Date(k.created_at).toLocaleDateString()}</>,
  },
];

function APIKeysTab({ keys }: { keys: APIKeyItem[] }) {
  return (
    <DataTable<APIKeyItem>
      columns={apiKeyColumns}
      data={keys}
      total={keys.length}
      page={1}
      pageSize={keys.length || 1}
      onPageChange={() => {}}
    />
  );
}

// ---- Webhooks Tab ----

const webhookColumns: Column<WebhookItem>[] = [
  {
    key: 'url',
    header: 'URL',
    render: (wh) => <span className="break-all max-w-xs block">{wh.url}</span>,
  },
  {
    key: 'event_types',
    header: 'Типы событий',
    render: (wh) => (
      <div className="flex flex-wrap gap-1">
        {wh.event_types.map((t) => (
          <Badge key={t}>{t}</Badge>
        ))}
      </div>
    ),
  },
  {
    key: 'active',
    header: 'Статус',
    render: (wh) => <StatusBadge status={wh.active ? 'active' : 'inactive'} />,
  },
  {
    key: 'created_at',
    header: 'Создан',
    render: (wh) => <>{wh.created_at ? new Date(wh.created_at).toLocaleDateString() : '-'}</>,
  },
];

function WebhooksTab({ webhooks }: { webhooks: WebhookItem[] }) {
  return (
    <DataTable<WebhookItem>
      columns={webhookColumns}
      data={webhooks}
      total={webhooks.length}
      page={1}
      pageSize={webhooks.length || 1}
      onPageChange={() => {}}
    />
  );
}
