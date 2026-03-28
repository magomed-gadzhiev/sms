// portal-frontend/src/components/settings/QuietHoursForm.tsx
import { useState, useEffect } from 'react';

interface QuietHoursConfig {
  enabled: boolean;
  start_hour: number;
  end_hour: number;
  action: string;
}

export function QuietHoursForm() {
  const [config, setConfig] = useState<QuietHoursConfig>({
    enabled: false, start_hour: 22, end_hour: 8, action: 'postpone',
  });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch('/portal/v1/settings/quiet-hours', { credentials: 'include' })
      .then((r) => r.json())
      .then((data) => { setConfig(data); setLoading(false); })
      .catch(() => setLoading(false));
  }, []);

  const save = async (updated: Partial<QuietHoursConfig>) => {
    const newConfig = { ...config, ...updated };
    setConfig(newConfig);
    await fetch('/portal/v1/settings/quiet-hours', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(newConfig),
    });
  };

  if (loading) return <p className="text-sm text-gray-500">Загрузка...</p>;

  return (
    <div className="space-y-4">
      <h3 className="text-sm font-semibold text-gray-900">Тихие часы</h3>
      <div className="p-4 bg-white border border-gray-200 rounded space-y-3">
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={config.enabled} onChange={(e) => save({ enabled: e.target.checked })} />
          Включить тихие часы
        </label>
        {config.enabled && (
          <>
            <div className="flex gap-4 items-center">
              <label className="text-sm text-gray-600">
                С:
                <select
                  value={config.start_hour}
                  onChange={(e) => save({ start_hour: parseInt(e.target.value) })}
                  className="ml-2 px-2 py-1 border border-gray-300 rounded text-sm"
                >
                  {Array.from({ length: 24 }, (_, i) => (
                    <option key={i} value={i}>{String(i).padStart(2, '0')}:00</option>
                  ))}
                </select>
              </label>
              <label className="text-sm text-gray-600">
                До:
                <select
                  value={config.end_hour}
                  onChange={(e) => save({ end_hour: parseInt(e.target.value) })}
                  className="ml-2 px-2 py-1 border border-gray-300 rounded text-sm"
                >
                  {Array.from({ length: 24 }, (_, i) => (
                    <option key={i} value={i}>{String(i).padStart(2, '0')}:00</option>
                  ))}
                </select>
              </label>
            </div>
            <div>
              <label className="text-sm text-gray-600">Действие при попадании в тихие часы:</label>
              <div className="mt-1 space-y-1">
                <label className="flex items-center gap-2 text-sm">
                  <input type="radio" name="action" value="postpone" checked={config.action === 'postpone'}
                    onChange={() => save({ action: 'postpone' })} />
                  Отложить до окончания тихих часов
                </label>
                <label className="flex items-center gap-2 text-sm">
                  <input type="radio" name="action" value="skip" checked={config.action === 'skip'}
                    onChange={() => save({ action: 'skip' })} />
                  Пропустить (не отправлять)
                </label>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
