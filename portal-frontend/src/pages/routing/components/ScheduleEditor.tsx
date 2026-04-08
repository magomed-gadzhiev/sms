import type { ScheduleJSON } from '../types';
import { WEEKDAY_LABELS, WEEKDAY_BITS } from '../types';

const TIMEZONES = [
  'Europe/Moscow',
  'Europe/Kiev',
  'Europe/Minsk',
  'Asia/Almaty',
  'Asia/Tashkent',
  'UTC',
];

interface ScheduleEditorProps {
  schedule: ScheduleJSON | null;
  onChange: (schedule: ScheduleJSON | null) => void;
}

export function ScheduleEditor({ schedule, onChange }: ScheduleEditorProps) {
  const enabled = schedule !== null;

  function toggle() {
    if (enabled) {
      onChange(null);
    } else {
      onChange({
        weekdays: 127,
        timezone: 'Europe/Moscow',
      });
    }
  }

  function update(partial: Partial<ScheduleJSON>) {
    if (!schedule) return;
    onChange({ ...schedule, ...partial });
  }

  function toggleWeekday(bit: number) {
    if (!schedule) return;
    onChange({ ...schedule, weekdays: schedule.weekdays ^ bit });
  }

  return (
    <div className="flex flex-col gap-3">
      <label className="flex items-center gap-2 cursor-pointer">
        <div
          role="switch"
          aria-checked={enabled}
          tabIndex={0}
          onClick={toggle}
          onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggle(); } }}
          className={`relative w-10 h-6 rounded-full transition-colors ${enabled ? 'bg-primary' : 'bg-gray-300'}`}
        >
          <span
            className={`absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full shadow transition-transform ${enabled ? 'translate-x-4' : ''}`}
          />
        </div>
        <span className="text-sm font-medium text-gray-700">Ограничить по расписанию</span>
      </label>

      {enabled && schedule && (
        <div className="border rounded-lg p-4 bg-gray-50 flex flex-col gap-3">
          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1">
              <label className="text-xs font-medium text-gray-600">Дата с</label>
              <input
                type="date"
                value={schedule.date_from || ''}
                onChange={(e) => update({ date_from: e.target.value || undefined })}
                className="rounded border border-gray-300 px-3 py-2 text-sm"
              />
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-xs font-medium text-gray-600">Дата по</label>
              <input
                type="date"
                value={schedule.date_to || ''}
                onChange={(e) => update({ date_to: e.target.value || undefined })}
                className="rounded border border-gray-300 px-3 py-2 text-sm"
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1">
              <label className="text-xs font-medium text-gray-600">Время с</label>
              <input
                type="time"
                value={schedule.time_from || ''}
                onChange={(e) => update({ time_from: e.target.value || undefined })}
                className="rounded border border-gray-300 px-3 py-2 text-sm"
              />
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-xs font-medium text-gray-600">Время по</label>
              <input
                type="time"
                value={schedule.time_to || ''}
                onChange={(e) => update({ time_to: e.target.value || undefined })}
                className="rounded border border-gray-300 px-3 py-2 text-sm"
              />
            </div>
          </div>

          <div className="flex flex-col gap-1">
            <label className="text-xs font-medium text-gray-600">Дни недели</label>
            <div className="flex gap-1">
              {WEEKDAY_LABELS.map((label, i) => {
                const active = (schedule.weekdays & WEEKDAY_BITS[i]) !== 0;
                return (
                  <button
                    key={i}
                    type="button"
                    onClick={() => toggleWeekday(WEEKDAY_BITS[i])}
                    className={`w-9 h-9 rounded text-xs font-medium transition-colors ${
                      active
                        ? 'bg-primary text-white'
                        : 'bg-gray-200 text-gray-600 hover:bg-gray-300'
                    }`}
                  >
                    {label}
                  </button>
                );
              })}
            </div>
          </div>

          <div className="flex flex-col gap-1">
            <label className="text-xs font-medium text-gray-600">Часовой пояс</label>
            <select
              value={schedule.timezone}
              onChange={(e) => update({ timezone: e.target.value })}
              className="rounded border border-gray-300 px-3 py-2 text-sm"
            >
              {TIMEZONES.map((tz) => (
                <option key={tz} value={tz}>{tz}</option>
              ))}
            </select>
          </div>
        </div>
      )}
    </div>
  );
}
