import type { RouteSchedule } from '../../api/client';

const WEEKDAYS: Array<{ mask: number; label: string }> = [
  { mask: 1, label: 'Пн' },
  { mask: 2, label: 'Вт' },
  { mask: 4, label: 'Ср' },
  { mask: 8, label: 'Чт' },
  { mask: 16, label: 'Пт' },
  { mask: 32, label: 'Сб' },
  { mask: 64, label: 'Вс' },
];

interface Props {
  schedules: RouteSchedule[];
  onChange: (schedules: RouteSchedule[]) => void;
}

export function RouteSchedulesEditor({ schedules, onChange }: Props) {
  const add = (): void => {
    onChange([...schedules, { weekdays: 127, timezone: 'Europe/Moscow' }]);
  };
  const remove = (idx: number): void => {
    onChange(schedules.filter((_, i) => i !== idx));
  };
  const update = (idx: number, patch: Partial<RouteSchedule>): void => {
    onChange(schedules.map((s, i) => (i === idx ? { ...s, ...patch } : s)));
  };
  const toggleWeekday = (idx: number, mask: number): void => {
    const cur = schedules[idx].weekdays;
    const next = (cur & mask) ? (cur & ~mask) : (cur | mask);
    update(idx, { weekdays: next });
  };

  return (
    <div className="space-y-3">
      <div className="text-sm font-medium text-gray-700">Расписания</div>
      {schedules.length === 0 && (
        <div className="text-xs text-gray-400">Без расписания — правило активно постоянно</div>
      )}
      {schedules.map((s, idx) => (
        <div key={idx} className="border border-gray-200 rounded p-3 space-y-2">
          <div className="flex items-center gap-2">
            <label className="text-xs text-gray-500">с</label>
            <input
              type="date"
              value={s.date_from || ''}
              onChange={(e) => update(idx, { date_from: e.target.value || null })}
              className="text-xs border rounded px-2 py-1"
            />
            <label className="text-xs text-gray-500">по</label>
            <input
              type="date"
              value={s.date_to || ''}
              onChange={(e) => update(idx, { date_to: e.target.value || null })}
              className="text-xs border rounded px-2 py-1"
            />
            <button
              type="button"
              onClick={() => remove(idx)}
              className="text-xs text-red-600 hover:underline ml-auto"
            >
              Удалить
            </button>
          </div>
          <div className="flex items-center gap-2">
            <label className="text-xs text-gray-500">время с</label>
            <input
              type="time"
              value={s.time_from || ''}
              onChange={(e) => update(idx, { time_from: e.target.value || null })}
              className="text-xs border rounded px-2 py-1"
            />
            <label className="text-xs text-gray-500">по</label>
            <input
              type="time"
              value={s.time_to || ''}
              onChange={(e) => update(idx, { time_to: e.target.value || null })}
              className="text-xs border rounded px-2 py-1"
            />
          </div>
          <div className="flex items-center gap-1">
            {WEEKDAYS.map((d) => (
              <label key={d.mask} className="flex items-center gap-1 text-xs">
                <input
                  type="checkbox"
                  checked={(s.weekdays & d.mask) !== 0}
                  onChange={() => toggleWeekday(idx, d.mask)}
                />
                {d.label}
              </label>
            ))}
          </div>
          <input
            type="text"
            value={s.timezone}
            onChange={(e) => update(idx, { timezone: e.target.value })}
            placeholder="Europe/Moscow"
            className="text-xs border rounded px-2 py-1 w-48"
          />
        </div>
      ))}
      <button
        type="button"
        onClick={add}
        className="text-sm text-blue-600 hover:underline"
      >
        + Расписание
      </button>
    </div>
  );
}
