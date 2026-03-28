// portal-frontend/src/components/settings/FrequencyCapForm.tsx
import { useState, useEffect } from 'react';

interface Cap {
  id: string;
  cap_type: string;
  max_messages: number;
  period_hours: number;
  enabled: boolean;
}

export function FrequencyCapForm() {
  const [caps, setCaps] = useState<Cap[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchCaps = async () => {
    const res = await fetch('/portal/v1/settings/frequency-caps', { credentials: 'include' });
    if (res.ok) {
      const data = await res.json();
      setCaps(data.caps || []);
    }
    setLoading(false);
  };

  useEffect(() => { fetchCaps(); }, []);

  const saveCap = async (capType: string, maxMessages: number, periodHours: number, enabled: boolean) => {
    await fetch('/portal/v1/settings/frequency-caps', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ cap_type: capType, max_messages: maxMessages, period_hours: periodHours, enabled }),
    });
    fetchCaps();
  };

  const capTypes = [
    { type: 'marketing', label: 'Маркетинговые' },
    { type: 'transactional', label: 'Транзакционные' },
  ];

  if (loading) return <p className="text-sm text-gray-500">Загрузка...</p>;

  return (
    <div className="space-y-4">
      <h3 className="text-sm font-semibold text-gray-900">Ограничения частоты</h3>
      {capTypes.map(({ type, label }) => {
        const cap = caps.find((c) => c.cap_type === type);
        return (
          <div key={type} className="p-4 bg-white border border-gray-200 rounded space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">{label}</span>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={cap?.enabled ?? false}
                  onChange={(e) => saveCap(type, cap?.max_messages ?? 5, cap?.period_hours ?? 24, e.target.checked)}
                />
                Включено
              </label>
            </div>
            <div className="flex gap-4">
              <label className="text-sm text-gray-600">
                Макс. сообщений:
                <input
                  type="number"
                  min={1}
                  defaultValue={cap?.max_messages ?? 5}
                  onBlur={(e) => saveCap(type, parseInt(e.target.value) || 5, cap?.period_hours ?? 24, cap?.enabled ?? false)}
                  className="ml-2 w-20 px-2 py-1 border border-gray-300 rounded text-sm"
                />
              </label>
              <label className="text-sm text-gray-600">
                За период (часов):
                <input
                  type="number"
                  min={1}
                  defaultValue={cap?.period_hours ?? 24}
                  onBlur={(e) => saveCap(type, cap?.max_messages ?? 5, parseInt(e.target.value) || 24, cap?.enabled ?? false)}
                  className="ml-2 w-20 px-2 py-1 border border-gray-300 rounded text-sm"
                />
              </label>
            </div>
          </div>
        );
      })}
    </div>
  );
}
