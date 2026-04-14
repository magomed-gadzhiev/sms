import type { FilterDef } from './MessageFilters';

interface Props {
  filterDefs: FilterDef[];
  values: Record<string, string>;
  onRemove: (key: string) => void;
}

export function ActiveFilterChips({ filterDefs, values, onRemove }: Props) {
  const active = filterDefs.filter((f) => values[f.key] && values[f.key] !== '');
  if (active.length === 0) return null;

  return (
    <div className="flex flex-wrap gap-2 mb-3">
      {active.map((f) => {
        const raw = values[f.key];
        const label = f.options?.find((o) => o.value === raw)?.label ?? raw;
        return (
          <span
            key={f.key}
            className="inline-flex items-center gap-1 rounded-full bg-primary/10 text-primary px-3 py-0.5 text-sm"
          >
            <span className="text-gray-500 text-xs">{f.label}:</span>
            <span>{label}</span>
            <button
              onClick={() => onRemove(f.key)}
              className="ml-1 rounded-full hover:bg-primary/20 p-0.5"
              aria-label={`Убрать фильтр ${f.label}`}
            >
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </span>
        );
      })}
    </div>
  );
}
