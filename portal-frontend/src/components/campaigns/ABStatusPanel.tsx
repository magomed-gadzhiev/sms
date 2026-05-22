import { useState, useEffect } from 'react';

interface ABStatus {
  test_phase: string;
  strategy: string;
  test_percentage: number;
  winning_metric: string;
  auto_select_winner: boolean;
  time_remaining_seconds: number;
  variants: {
    id: string;
    name: string;
    sent_count: number;
    delivered_count: number;
    failed_count: number;
    click_count: number;
    delivery_rate: number;
    click_rate: number;
    is_winner: boolean;
  }[];
}

interface ABStatusPanelProps {
  campaignId: string;
}

export function ABStatusPanel({ campaignId }: ABStatusPanelProps) {
  const [status, setStatus] = useState<ABStatus | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchStatus = async () => {
    const res = await fetch(`/portal/v1/campaigns/${campaignId}/ab-status`, { credentials: 'include' });
    if (res.ok) setStatus(await res.json());
    setLoading(false);
  };

  useEffect(() => { fetchStatus(); const t = setInterval(fetchStatus, 30000); return () => clearInterval(t); }, [campaignId]);

  const selectWinner = async (variantId: string) => {
    await fetch(`/portal/v1/campaigns/${campaignId}/select-winner`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ variant_id: variantId }),
    });
    fetchStatus();
  };

  if (loading || !status) return null;
  if (status.strategy !== 'test_then_send') return null;

  const phaseLabels: Record<string, string> = {
    none: 'Не запущен',
    testing: 'Тестирование',
    waiting_winner: 'Ожидание выбора победителя',
    rollout: 'Рассылка остальным',
    completed: 'Завершён',
  };

  const phaseColors: Record<string, string> = {
    testing: 'text-blue-600 bg-blue-50',
    waiting_winner: 'text-amber-600 bg-amber-50',
    rollout: 'text-green-600 bg-green-50',
    completed: 'text-gray-600 bg-gray-50',
  };

  return (
    <div className="p-4 border border-gray-200 rounded space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">A/B Тест: Test-Then-Send</h3>
        <span className={`text-xs px-2 py-0.5 rounded ${phaseColors[status.test_phase] || 'text-gray-500'}`}>
          {phaseLabels[status.test_phase] || status.test_phase}
        </span>
      </div>

      <p className="text-xs text-gray-500">
        Тест: {status.test_percentage}% аудитории / Метрика: {status.winning_metric}
      </p>

      {status.time_remaining_seconds > 0 && (
        <p className="text-xs text-gray-500">
          Осталось: {Math.ceil(status.time_remaining_seconds / 60)} мин.
        </p>
      )}

      <div className="space-y-2">
        {status.variants?.map((v) => (
          <div key={v.id} className={`p-3 rounded border ${v.is_winner ? 'border-green-300 bg-green-50' : 'border-gray-200'}`}>
            <div className="flex justify-between items-center">
              <span className="text-sm font-medium">{v.name} {v.is_winner && '(Победитель)'}</span>
              {status.test_phase === 'waiting_winner' && !v.is_winner && (
                <button onClick={() => selectWinner(v.id)}
                  className="text-xs px-2 py-1 bg-primary text-white rounded hover:bg-primary/90">
                  Выбрать
                </button>
              )}
            </div>
            <div className="mt-1 flex gap-4 text-xs text-gray-600">
              <span>Отправлено: {v.sent_count}</span>
              <span>Доставлено: {(v.delivery_rate * 100).toFixed(1)}%</span>
              <span>Клики: {v.click_count}</span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
