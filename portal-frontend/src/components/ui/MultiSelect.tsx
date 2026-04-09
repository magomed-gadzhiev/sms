import { useState, useRef, useEffect } from 'react';

export interface MultiSelectOption {
  value: string;
  label: string;
}

interface MultiSelectProps {
  label?: string;
  options: MultiSelectOption[];
  values: string[];
  onChange: (values: string[]) => void;
  placeholder?: string;
  loading?: boolean;
  error?: string;
  required?: boolean;
  disabled?: boolean;
}

export function MultiSelect({
  label,
  options,
  values,
  onChange,
  placeholder = 'Выберите...',
  loading,
  error,
  required,
  disabled,
}: MultiSelectProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const ref = useRef<HTMLDivElement>(null);

  const filtered = search.trim()
    ? options.filter((o) => o.label.toLowerCase().includes(search.toLowerCase()))
    : options;

  const selectedOptions = options.filter((o) => values.includes(o.value));

  useEffect(() => {
    const handleClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, []);

  const toggle = (value: string) => {
    if (values.includes(value)) onChange(values.filter((v) => v !== value));
    else onChange([...values, value]);
  };

  const handleOpen = () => {
    if (!disabled) { setOpen(!open); setSearch(''); }
  };

  const selectId = label?.toLowerCase().replace(/\s+/g, '-');

  return (
    <div className="flex flex-col gap-1" ref={ref}>
      {label && (
        <label htmlFor={selectId} className="text-sm font-medium text-gray-700">
          {label}{required && ' *'}
        </label>
      )}
      <div className="relative">
        <button
          id={selectId}
          type="button"
          onClick={handleOpen}
          disabled={disabled}
          className={`w-full text-left rounded border px-3 py-2 text-sm min-h-[38px] transition-colors
            focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:border-primary
            ${error ? 'border-danger' : 'border-gray-300'}
            ${disabled ? 'bg-gray-50 text-gray-500 cursor-not-allowed' : 'bg-white cursor-pointer hover:border-gray-400'}`}
          aria-haspopup="listbox"
          aria-expanded={open}
          aria-invalid={!!error}
        >
          {loading ? (
            <span className="text-gray-400">Загрузка...</span>
          ) : selectedOptions.length > 0 ? (
            <span className="flex flex-wrap gap-1">
              {selectedOptions.map((o) => (
                <span
                  key={o.value}
                  className="inline-flex items-center rounded bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary"
                >
                  {o.label}
                  <span
                    role="button"
                    aria-label={`Убрать ${o.label}`}
                    tabIndex={0}
                    className="ml-1 cursor-pointer opacity-60 hover:opacity-100"
                    onClick={(e) => { e.stopPropagation(); toggle(o.value); }}
                    onKeyDown={(e) => { if (e.key === 'Enter') { e.stopPropagation(); toggle(o.value); } }}
                  >
                    ×
                  </span>
                </span>
              ))}
            </span>
          ) : (
            <span className="text-gray-400">{placeholder}</span>
          )}
        </button>

        {open && (
          <div className="absolute z-50 mt-1 w-full rounded border border-gray-200 bg-white shadow-lg">
            <div className="p-2 border-b border-gray-100">
              <input
                autoFocus
                className="w-full rounded border border-gray-200 px-2 py-1.5 text-sm outline-none focus:border-primary"
                placeholder="Поиск..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </div>
            <div className="max-h-52 overflow-y-auto" role="listbox" aria-multiselectable="true">
              {filtered.length === 0 ? (
                <div className="px-3 py-2 text-sm text-gray-500">Ничего не найдено</div>
              ) : (
                filtered.map((o) => (
                  <label
                    key={o.value}
                    className="flex items-center gap-2 px-3 py-2 text-sm hover:bg-gray-50 cursor-pointer transition-colors"
                  >
                    <input
                      type="checkbox"
                      checked={values.includes(o.value)}
                      onChange={() => toggle(o.value)}
                      className="rounded border-gray-300 text-primary focus:ring-primary/50"
                    />
                    {o.label}
                  </label>
                ))
              )}
            </div>
            {selectedOptions.length > 0 && (
              <div className="border-t border-gray-100 px-3 py-2">
                <button
                  type="button"
                  className="text-xs text-gray-500 hover:text-gray-700"
                  onClick={() => onChange([])}
                >
                  Очистить выбор
                </button>
              </div>
            )}
          </div>
        )}
      </div>
      {error && <span className="text-sm text-danger" role="alert">{error}</span>}
    </div>
  );
}
