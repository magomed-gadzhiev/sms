import { useState, useRef, useEffect } from 'react';
import { Calendar, Plus, Pause, Play } from 'lucide-react';
import type { SharedFilter } from '../../api/networkStats';

type Mode = 'stats' | 'analytics' | 'monitoring';

interface PollingInfo {
  isPaused: boolean;
  toggle: () => void;
  lastUpdated: Date | null;
}

interface OperatorOption {
  id: string;
  name: string;
}

interface StatisticsFilterBarProps {
  mode: Mode;
  filters: SharedFilter;
  onFiltersChange: (partial: Partial<SharedFilter>) => void;
  onApply: () => void;
  loading: boolean;
  polling?: PollingInfo;
  operators?: OperatorOption[];
}

const PERIOD_PRESETS = [
  { value: 'today', label: 'Сегодня' },
  { value: 'yesterday', label: 'Вчера' },
  { value: '7d', label: '7 дней' },
  { value: '30d', label: '30 дней' },
  { value: 'month', label: 'Месяц' },
  { value: 'prev_month', label: 'Пред. месяц' },
  { value: 'year', label: 'Год' },
];

const MONITORING_WINDOWS = [
  { value: '15m', label: '15 мин' },
  { value: '60m', label: '60 мин' },
  { value: '24h', label: '24 часа' },
  { value: '7d', label: '7 дней' },
];

const CHANNELS = [
  { value: 'sms', label: 'SMS' },
  { value: 'viber', label: 'Viber' },
  { value: 'whatsapp', label: 'WhatsApp' },
  { value: 'max_messenger', label: 'MAX Messenger' },
];

const SERVICE_TYPES = [
  { value: 'sms', label: 'SMS' },
  { value: 'hlr', label: 'HLR' },
  { value: 'max', label: 'MAX' },
];

const GROUPINGS = [
  { value: '5min', label: 'по 5 минут' },
  { value: '15min', label: 'по 15 минут' },
  { value: 'hour', label: 'по часам' },
  { value: 'day', label: 'по дням' },
  { value: 'month', label: 'по месяцам' },
  { value: 'year', label: 'по годам' },
  { value: 'provider', label: 'по провайдерам' },
  { value: 'channel', label: 'по каналам' },
  { value: 'login', label: 'по логинам' },
  { value: 'operator', label: 'по операторам' },
  { value: 'country', label: 'по странам' },
  { value: 'manager', label: 'по менеджерам' },
];

export function StatisticsFilterBar({ mode, filters, onFiltersChange, onApply, loading, polling, operators = [] }: StatisticsFilterBarProps) {
  const [showExtra, setShowExtra] = useState(false);
  const [showDatePicker, setShowDatePicker] = useState(false);
  const datePickerRef = useRef<HTMLDivElement>(null);
  const isMonitoring = mode === 'monitoring';

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (datePickerRef.current && !datePickerRef.current.contains(e.target as Node)) {
        setShowDatePicker(false);
      }
    }
    if (showDatePicker) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [showDatePicker]);

  const presets = isMonitoring ? MONITORING_WINDOWS : PERIOD_PRESETS;
  const activePreset = filters.period_preset || '7d';

  function secondsAgo(date: Date | null): string {
    if (!date) return '';
    const s = Math.round((Date.now() - date.getTime()) / 1000);
    return `Обновлено ${s} сек. назад`;
  }

  return (
    <div className="border-b border-gray-200 bg-white px-4 py-3 space-y-2">
      {/* Row 1: Presets + grouping */}
      <div className="flex items-center gap-1.5 flex-wrap">
        {isMonitoring && (
          <span className="text-xs text-gray-400 mr-1">Окно:</span>
        )}
        {!isMonitoring && (
          <span className="text-xs text-gray-400 mr-1">Период:</span>
        )}

        {presets.map(p => (
          <button
            key={p.value}
            onClick={() => {
              onFiltersChange({ period_preset: p.value, date_from: '', date_to: '' });
              // Auto-apply when selecting a period preset
              setTimeout(() => onApply(), 0);
            }}
            className={`px-2.5 py-1 rounded-md text-xs font-medium border transition-colors ${
              activePreset === p.value
                ? 'bg-blue-50 border-blue-200 text-blue-600'
                : 'bg-white border-gray-200 text-gray-500 hover:border-gray-300'
            }`}
          >
            {p.label}
          </button>
        ))}

        {!isMonitoring && (
          <>
            <div className="h-5 border-l border-gray-200 mx-1" />
            <div className="relative" ref={datePickerRef}>
              <button
                onClick={() => setShowDatePicker(v => !v)}
                className={`flex items-center gap-1 px-2.5 py-1 rounded-md text-xs border transition-colors ${
                  filters.date_from && filters.date_to
                    ? 'bg-blue-50 border-blue-200 text-blue-600'
                    : 'border-gray-200 text-gray-500 hover:border-gray-300'
                }`}
              >
                <Calendar size={12} />
                {filters.date_from && filters.date_to
                  ? `${filters.date_from} — ${filters.date_to}`
                  : 'Произвольный'}
              </button>
              {showDatePicker && (
                <div className="absolute top-full left-0 mt-1 z-50 bg-white border border-gray-200 rounded-lg shadow-lg p-3 space-y-2 min-w-[260px]">
                  <div className="flex items-center gap-2">
                    <label className="text-xs text-gray-500 w-8">От</label>
                    <input
                      type="date"
                      value={filters.date_from || ''}
                      onChange={e => onFiltersChange({ date_from: e.target.value, period_preset: '' })}
                      className="flex-1 px-2 py-1 rounded-md text-xs border border-gray-200 text-gray-700"
                    />
                  </div>
                  <div className="flex items-center gap-2">
                    <label className="text-xs text-gray-500 w-8">До</label>
                    <input
                      type="date"
                      value={filters.date_to || ''}
                      onChange={e => onFiltersChange({ date_to: e.target.value, period_preset: '' })}
                      className="flex-1 px-2 py-1 rounded-md text-xs border border-gray-200 text-gray-700"
                    />
                  </div>
                  <div className="flex items-center gap-2 pt-1">
                    <button
                      onClick={() => {
                        setShowDatePicker(false);
                        onApply();
                      }}
                      className="flex-1 px-3 py-1 rounded-md text-xs font-medium bg-blue-600 text-white hover:bg-blue-700"
                    >
                      Применить
                    </button>
                    {(filters.date_from || filters.date_to) && (
                      <button
                        onClick={() => {
                          onFiltersChange({ date_from: '', date_to: '', period_preset: '7d' });
                          setShowDatePicker(false);
                        }}
                        className="px-3 py-1 rounded-md text-xs border border-gray-200 text-gray-500 hover:border-gray-300"
                      >
                        Сброс
                      </button>
                    )}
                  </div>
                </div>
              )}
            </div>
          </>
        )}

        <div className="h-5 border-l border-gray-200 mx-1" />

        <select
          value={filters.group_by || 'day'}
          onChange={e => onFiltersChange({ group_by: e.target.value })}
          className="px-2.5 py-1 rounded-md text-xs border border-gray-200 text-gray-500 bg-white"
        >
          {GROUPINGS.map(g => (
            <option key={g.value} value={g.value}>Группировка: {g.label}</option>
          ))}
        </select>

        {/* Monitoring live indicator */}
        {isMonitoring && polling && (
          <div className="ml-auto flex items-center gap-2">
            <span className="text-xs text-emerald-600 flex items-center gap-1">
              <span className="inline-block w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse" />
              {secondsAgo(polling.lastUpdated)}
            </span>
            <button
              onClick={polling.toggle}
              className="flex items-center gap-1 px-2 py-1 rounded-md text-xs border border-gray-200 text-gray-500 hover:border-gray-300"
            >
              {polling.isPaused ? <Play size={12} /> : <Pause size={12} />}
              {polling.isPaused ? 'Продолжить' : 'Пауза'}
            </button>
          </div>
        )}
      </div>

      {/* Row 2: Quick filters + extra + apply */}
      <div className="flex items-center gap-1.5 flex-wrap">
        <input
          type="text"
          placeholder="Логин"
          value={filters.login || ''}
          onChange={e => onFiltersChange({ login: e.target.value })}
          className="px-2.5 py-1 rounded-md text-xs border border-gray-200 text-gray-700 w-28 placeholder:text-gray-400"
        />
        <select
          value={filters.operator || ''}
          onChange={e => onFiltersChange({ operator: e.target.value })}
          className="px-2.5 py-1 rounded-md text-xs border border-gray-200 text-gray-500 bg-white"
        >
          <option value="">Оператор</option>
          {operators.map(op => (
            <option key={op.id} value={op.name}>{op.name}</option>
          ))}
        </select>
        <select
          value={filters.channel || ''}
          onChange={e => onFiltersChange({ channel: e.target.value })}
          className="px-2.5 py-1 rounded-md text-xs border border-gray-200 text-gray-500 bg-white"
        >
          <option value="">Канал</option>
          {CHANNELS.map(ch => (
            <option key={ch.value} value={ch.value}>{ch.label}</option>
          ))}
        </select>
        <select
          value={filters.service_type || ''}
          onChange={e => onFiltersChange({ service_type: e.target.value })}
          className="px-2.5 py-1 rounded-md text-xs border border-gray-200 text-gray-500 bg-white"
        >
          <option value="">Тип услуги</option>
          {SERVICE_TYPES.map(st => (
            <option key={st.value} value={st.value}>{st.label}</option>
          ))}
        </select>

        <button
          onClick={() => setShowExtra(v => !v)}
          className="flex items-center gap-1 px-2.5 py-1 rounded-md text-xs border border-gray-200 text-gray-400 hover:border-gray-300"
        >
          <Plus size={12} />
          Ещё фильтры
        </button>

        <button
          onClick={onApply}
          disabled={loading}
          className="ml-auto px-5 py-1.5 rounded-md text-sm font-semibold bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50 transition-colors"
        >
          {loading ? 'Загрузка...' : 'Применить'}
        </button>
      </div>

      {/* Extra filters (expandable) */}
      {showExtra && (
        <div className="flex items-center gap-1.5 flex-wrap pt-1 border-t border-gray-100">
          <input type="text" placeholder="Имя отправителя" value={filters.sender_name || ''} onChange={e => onFiltersChange({ sender_name: e.target.value })} className="px-2.5 py-1 rounded-md text-xs border border-gray-200 text-gray-700 w-36 placeholder:text-gray-400" />
          <select value={filters.traffic_type || ''} onChange={e => onFiltersChange({ traffic_type: e.target.value })} className="px-2.5 py-1 rounded-md text-xs border border-gray-200 text-gray-500 bg-white">
            <option value="">Тип трафика</option>
            <option value="transactional">Транзакционный</option>
            <option value="promotional">Рекламный</option>
            <option value="service">Сервисный</option>
          </select>
          <select value={filters.status || ''} onChange={e => onFiltersChange({ status: e.target.value })} className="px-2.5 py-1 rounded-md text-xs border border-gray-200 text-gray-500 bg-white">
            <option value="">Статус</option>
            <option value="delivered">Доставлено</option>
            <option value="sent">Отправлено</option>
            <option value="failed">Ошибка</option>
            <option value="pending">Ожидание</option>
            <option value="expired">Истекло</option>
          </select>
          <input type="text" placeholder="Провайдер" value={filters.provider || ''} onChange={e => onFiltersChange({ provider: e.target.value })} className="px-2.5 py-1 rounded-md text-xs border border-gray-200 text-gray-700 w-28 placeholder:text-gray-400" />
          <input type="text" placeholder="Страна" value={filters.country || ''} onChange={e => onFiltersChange({ country: e.target.value })} className="px-2.5 py-1 rounded-md text-xs border border-gray-200 text-gray-700 w-24 placeholder:text-gray-400" />
          <input type="text" placeholder="Менеджер" value={filters.manager || ''} onChange={e => onFiltersChange({ manager: e.target.value })} className="px-2.5 py-1 rounded-md text-xs border border-gray-200 text-gray-700 w-28 placeholder:text-gray-400" />
          <input type="text" placeholder="Код ошибки" value={filters.error_code || ''} onChange={e => onFiltersChange({ error_code: e.target.value })} className="px-2.5 py-1 rounded-md text-xs border border-gray-200 text-gray-700 w-28 placeholder:text-gray-400" />
        </div>
      )}
    </div>
  );
}
