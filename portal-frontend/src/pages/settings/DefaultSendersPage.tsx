import { useCallback, useEffect, useState } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { defaultSendersApi, senderNamesApi, type SenderNameInfo } from '../../api/client';

const CHANNELS: Array<{ code: string; label: string }> = [
  { code: 'sms', label: 'SMS' },
  { code: 'viber', label: 'Viber' },
  { code: 'max', label: 'MAX' },
];

export function DefaultSendersPage() {
  const [senderNames, setSenderNames] = useState<SenderNameInfo[]>([]);
  const [defaults, setDefaults] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [success, setSuccess] = useState('');
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      const [namesRes, defaultsRes] = await Promise.all([
        senderNamesApi.listApproved(),
        defaultSendersApi.get(),
      ]);
      setSenderNames(namesRes.sender_names);
      setDefaults(defaultsRes);
    } catch {
      setError('Не удалось загрузить данные');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleChange = (channel: string, value: string) => {
    setDefaults((prev) => ({ ...prev, [channel]: value }));
  };

  const save = async () => {
    setSaving(true);
    setError('');
    setSuccess('');
    // Only send channels that have a value set
    const payload: Record<string, string> = {};
    for (const { code } of CHANNELS) {
      if (defaults[code]) payload[code] = defaults[code];
    }
    try {
      await defaultSendersApi.set(payload);
      setSuccess('Настройки сохранены');
    } catch {
      setError('Ошибка сохранения');
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <p className="p-6 text-gray-500">Загрузка...</p>;

  return (
    <div>
      <PageHeader
        title="Имена отправителей по умолчанию"
        subtitle="Выберите имя отправителя по умолчанию для каждого канала"
      />

      <div className="bg-white rounded-lg border p-6 max-w-lg">
        {senderNames.length === 0 ? (
          <p className="text-sm text-gray-500">
            У вас нет одобренных имён отправителей. Добавьте их в разделе{' '}
            <a href="/sender-names" className="text-primary underline">
              Имена отправителей
            </a>
            .
          </p>
        ) : (
          <div className="space-y-5">
            {CHANNELS.map(({ code, label }) => {
              const options = senderNames;
              return (
                <div key={code}>
                  <label
                    htmlFor={`default-sender-${code}`}
                    className="block text-sm font-medium text-gray-700 mb-1"
                  >
                    {label}
                  </label>
                  <select
                    id={`default-sender-${code}`}
                    value={defaults[code] ?? ''}
                    onChange={(e) => handleChange(code, e.target.value)}
                    className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50"
                  >
                    <option value="">— Не выбрано —</option>
                    {options.map((sn) => (
                      <option key={sn.id} value={sn.id}>
                        {sn.name}
                      </option>
                    ))}
                  </select>
                </div>
              );
            })}
          </div>
        )}

        <div className="mt-6 flex items-center gap-4">
          <Button onClick={save} disabled={saving || senderNames.length === 0}>
            {saving ? 'Сохранение...' : 'Сохранить'}
          </Button>
          {success && <span className="text-sm text-green-600">{success}</span>}
          {error && <span className="text-sm text-red-600">{error}</span>}
        </div>
      </div>
    </div>
  );
}
