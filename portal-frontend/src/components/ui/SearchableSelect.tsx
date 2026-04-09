import { useState, useRef, useEffect } from 'react';

export interface SearchableSelectOption {
  value: string;
  label: string;
}

interface SearchableSelectProps {
  label?: string;
  options: SearchableSelectOption[];
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  loading?: boolean;
  error?: string;
  required?: boolean;
  disabled?: boolean;
  className?: string;
}

export function SearchableSelect({
  label,
  options,
  value,
  onChange,
  placeholder = 'Выберите...',
  loading,
  error,
  required,
  disabled,
  className,
}: SearchableSelectProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const ref = useRef<HTMLDivElement>(null);

  const selected = options.find((o) => o.value === value);
  const filtered = search.trim()
    ? options.filter((o) => o.label.toLowerCase().includes(search.toLowerCase()))
    : options;

  useEffect(() => {
    const handleClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, []);

  const handleOpen = () => {
    if (!disabled) { setOpen(!open); setSearch(''); }
  };

  const selectId = label?.toLowerCase().replace(/\s+/g, '-');

  return (
    <div className={`flex flex-col gap-1${className ? ` ${className}` : ''}`} ref={ref}>
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
          className={`w-full text-left rounded border px-3 py-2 text-sm transition-colors
            focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:border-primary
            ${error ? 'border-danger' : 'border-gray-300'}
            ${disabled ? 'bg-gray-50 text-gray-500 cursor-not-allowed' : 'bg-white cursor-pointer hover:border-gray-400'}`}
          aria-haspopup="listbox"
          aria-expanded={open}
          aria-invalid={!!error}
        >
          {loading ? (
            <span className="text-gray-400">Загрузка...</span>
          ) : selected ? (
            selected.label
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
            <div className="max-h-52 overflow-y-auto" role="listbox">
              {filtered.length === 0 ? (
                <div className="px-3 py-2 text-sm text-gray-500">Ничего не найдено</div>
              ) : (
                filtered.map((o) => (
                  <button
                    key={o.value}
                    type="button"
                    role="option"
                    aria-selected={o.value === value}
                    onClick={() => { onChange(o.value); setOpen(false); }}
                    className={`w-full text-left px-3 py-2 text-sm hover:bg-gray-50 transition-colors
                      ${o.value === value ? 'bg-primary/10 font-medium' : ''}`}
                  >
                    {o.label}
                  </button>
                ))
              )}
            </div>
          </div>
        )}
      </div>
      {error && <span className="text-sm text-danger" role="alert">{error}</span>}
    </div>
  );
}
