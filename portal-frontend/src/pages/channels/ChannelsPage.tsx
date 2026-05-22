import { useState, useEffect, useCallback } from 'react';
import { cascadeChannelsApi, type DeliveryChannel } from '../../api/cascade';
import { Button } from '../../components/ui/Button';
import { Badge } from '../../components/ui/Badge';
import { DataTable, type Column } from '../../components/data/DataTable';
import { ChannelFormModal } from './ChannelFormModal';

function formatDate(dt: string | null | undefined) {
  if (!dt) return '—';
  const d = new Date(dt);
  if (isNaN(d.getTime())) return '—';
  return d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
}

const CHANNEL_TYPE_LABELS: Record<string, string> = {
  sms: 'SMS',
  flash_call: 'Flash Call',
  reverse_call: 'Reverse Call',
  messenger: 'Messenger',
};

export function ChannelsPage() {
  const [channels, setChannels] = useState<DeliveryChannel[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [modalOpen, setModalOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<DeliveryChannel | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await cascadeChannelsApi.list();
      setChannels(res.channels ?? []);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleToggle = async (ch: DeliveryChannel) => {
    try {
      await cascadeChannelsApi.toggle(ch.channel_id, !ch.active);
      load();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Ошибка');
    }
  };

  const handleEdit = (ch: DeliveryChannel) => {
    setEditTarget(ch);
    setModalOpen(true);
  };

  const handleAdd = () => {
    setEditTarget(null);
    setModalOpen(true);
  };

  const columns: Column<DeliveryChannel>[] = [
    {
      key: 'channel_type',
      header: 'Тип',
      render: (ch) => CHANNEL_TYPE_LABELS[ch.channel_type] ?? ch.channel_type,
    },
    { key: 'name', header: 'Название' },
    {
      key: 'active',
      header: 'Статус',
      render: (ch) => (
        <Badge variant={ch.active ? 'success' : 'default'}>
          {ch.active ? 'Активен' : 'Отключён'}
        </Badge>
      ),
    },
    { key: 'created_at', header: 'Создан', render: (ch) => formatDate(ch.created_at) },
    {
      key: 'actions' as keyof DeliveryChannel,
      header: 'Действия',
      render: (ch) => (
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={() => handleEdit(ch)}>
            Редактировать
          </Button>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => handleToggle(ch)}
          >
            {ch.active ? 'Отключить' : 'Включить'}
          </Button>
        </div>
      ),
    },
  ];

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-semibold">Каналы доставки</h1>
        <Button onClick={handleAdd}>Добавить канал</Button>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded text-red-700 text-sm">
          {error}
        </div>
      )}

      <DataTable
        columns={columns}
        data={channels}
        loading={loading}
        total={channels.length}
        page={1}
        pageSize={channels.length || 1}
        onPageChange={() => {}}
        keyField="channel_id"
      />

      {modalOpen && (
        <ChannelFormModal
          channel={editTarget}
          onClose={() => setModalOpen(false)}
          onSaved={() => { setModalOpen(false); load(); }}
        />
      )}
    </div>
  );
}
