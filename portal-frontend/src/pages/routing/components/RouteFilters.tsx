interface RouteFiltersProps {
  search: string;
  onSearch: (value: string) => void;
  routeType: string;
  status: string;
  onFilterChange: (key: 'route_type' | 'status', value: string) => void;
}

const ROUTE_TYPES = [
  { value: '', label: 'Все' },
  { value: 'sms', label: 'SMS' },
  { value: 'hlr', label: 'HLR' },
  { value: 'max', label: 'MAX' },
];

const STATUSES = [
  { value: '', label: 'Все' },
  { value: 'active', label: 'Активные' },
  { value: 'draft', label: 'Черновики' },
];

export function RouteFilters({ search, onSearch, routeType, status, onFilterChange }: RouteFiltersProps) {
  return (
    <div className="flex flex-wrap items-center gap-4 mb-6">
      <input
        type="text"
        placeholder="Поиск по названию..."
        value={search}
        onChange={(e) => onSearch(e.target.value)}
        className="rounded border border-gray-300 px-3 py-2 text-sm min-w-[260px] focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:border-primary"
      />

      <div className="flex items-center gap-1">
        <span className="text-sm text-gray-500 mr-1">Тип:</span>
        {ROUTE_TYPES.map((rt) => (
          <button
            key={rt.value}
            onClick={() => onFilterChange('route_type', rt.value)}
            className={`px-3 py-1.5 text-xs font-medium rounded-full transition-colors ${
              routeType === rt.value
                ? 'bg-primary text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
            }`}
          >
            {rt.label}
          </button>
        ))}
      </div>

      <div className="flex items-center gap-1">
        <span className="text-sm text-gray-500 mr-1">Статус:</span>
        {STATUSES.map((s) => (
          <button
            key={s.value}
            onClick={() => onFilterChange('status', s.value)}
            className={`px-3 py-1.5 text-xs font-medium rounded-full transition-colors ${
              status === s.value
                ? 'bg-primary text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
            }`}
          >
            {s.label}
          </button>
        ))}
      </div>
    </div>
  );
}
