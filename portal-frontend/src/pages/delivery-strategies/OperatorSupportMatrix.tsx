import { useState, useEffect } from 'react';
import { cascadeOCSApi, cascadeChannelsApi, type OCSEntry, type DeliveryChannel } from '../../api/cascade';
import { Modal } from '../../components/ui/Modal';
import { Button } from '../../components/ui/Button';

interface Props {
  onClose: () => void;
}

type MatrixRow = {
  operator_id: string;
  operator_name: string;
  channels: Record<string, boolean>;
  notes: Record<string, string>;
};

export function OperatorSupportMatrix({ onClose }: Props) {
  const [entries, setEntries] = useState<OCSEntry[]>([]);
  const [channels, setChannels] = useState<DeliveryChannel[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  // Local editable state: operator_id -> channel_type -> supported
  const [matrix, setMatrix] = useState<Record<string, Record<string, boolean>>>({});

  useEffect(() => {
    Promise.all([cascadeOCSApi.list(), cascadeChannelsApi.list()])
      .then(([ocsRes, chRes]) => {
        setEntries(ocsRes.entries ?? []);
        setChannels(chRes.channels ?? []);

        // Build editable matrix
        const m: Record<string, Record<string, boolean>> = {};
        for (const e of ocsRes.entries ?? []) {
          if (!m[e.operator_id]) m[e.operator_id] = {};
          m[e.operator_id][e.channel_type] = e.supported;
        }
        setMatrix(m);
      })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  // Build rows from entries
  const operatorMap: Record<string, string> = {};
  for (const e of entries) {
    operatorMap[e.operator_id] = e.operator_name || e.operator_id.slice(0, 8);
  }
  const operatorIds = Object.keys(operatorMap);

  const channelTypes = channels.map((ch) => ch.channel_type);

  const toggle = (operatorId: string, channelType: string) => {
    setMatrix((prev) => ({
      ...prev,
      [operatorId]: {
        ...(prev[operatorId] ?? {}),
        [channelType]: !(prev[operatorId]?.[channelType] ?? false),
      },
    }));
  };

  const saveRow = async (operatorId: string) => {
    setSaving(true);
    setError('');
    try {
      for (const ct of channelTypes) {
        await cascadeOCSApi.update({
          operator_id: operatorId,
          channel_type: ct,
          supported: matrix[operatorId]?.[ct] ?? false,
        });
      }
      setSuccess(`Оператор ${operatorMap[operatorId]} сохранён`);
      setTimeout(() => setSuccess(''), 2000);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Ошибка сохранения');
    } finally {
      setSaving(false);
    }
  };

  const rows: MatrixRow[] = operatorIds.map((id) => ({
    operator_id: id,
    operator_name: operatorMap[id],
    channels: matrix[id] ?? {},
    notes: {},
  }));

  return (
    <Modal open={true} title="Матрица поддержки каналов операторами" onClose={onClose}>
      {loading ? (
        <div className="py-8 text-center text-gray-400 text-sm">Загрузка...</div>
      ) : (
        <div className="overflow-x-auto">
          {error && (
            <div className="mb-3 p-2 bg-red-50 border border-red-200 rounded text-red-700 text-sm">{error}</div>
          )}
          {success && (
            <div className="mb-3 p-2 bg-green-50 border border-green-200 rounded text-green-700 text-sm">{success}</div>
          )}

          <table className="w-full text-sm">
            <thead>
              <tr className="border-b">
                <th className="text-left py-2 pr-4 font-medium text-gray-600">Оператор</th>
                {channelTypes.map((ct) => (
                  <th key={ct} className="text-center py-2 px-3 font-medium text-gray-600 uppercase text-xs">
                    {ct}
                  </th>
                ))}
                <th className="py-2 pl-4" />
              </tr>
            </thead>
            <tbody>
              {rows.length === 0 && (
                <tr>
                  <td colSpan={channelTypes.length + 2} className="py-8 text-center text-gray-400">
                    Нет данных
                  </td>
                </tr>
              )}
              {rows.map((row) => (
                <tr key={row.operator_id} className="border-b hover:bg-gray-50">
                  <td className="py-2 pr-4 font-medium">{row.operator_name}</td>
                  {channelTypes.map((ct) => (
                    <td key={ct} className="text-center py-2 px-3">
                      <input
                        type="checkbox"
                        checked={row.channels[ct] ?? false}
                        onChange={() => toggle(row.operator_id, ct)}
                        className="cursor-pointer"
                      />
                    </td>
                  ))}
                  <td className="py-2 pl-4">
                    <Button
                      variant="secondary"
                      size="sm"
                      disabled={saving}
                      onClick={() => saveRow(row.operator_id)}
                    >
                      Сохранить
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Modal>
  );
}
