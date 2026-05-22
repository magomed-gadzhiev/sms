import { useEffect, useId, useState } from 'react';
import { Button } from '../../../components/ui/Button';

export interface FilterOption {
  value: string;
  label: string;
}

export interface FilterDef {
  key: string;
  label: string;
  type: 'text' | 'select' | 'date';
  placeholder?: string;
  options?: FilterOption[];
}

interface Props {
  primary: FilterDef[];
  secondary: FilterDef[];
  values: Record<string, string>;
  onSearch: (values: Record<string, string>) => void;
  onReset: () => void;
}

function Field({ f, value, onChange }: { f: FilterDef; value: string; onChange: (v: string) => void }) {
  const fieldId = useId();
  if (f.type === 'select') {
    return (
      <div className="flex flex-col gap-1">
        <label htmlFor={fieldId} className="text-xs text-gray-500 font-medium">{f.label}</label>
        <select
          id={fieldId}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="rounded border border-gray-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
        >
          <option value="">Все</option>
          {f.options?.map((o) => (
            <option key={o.value} value={o.value}>{o.label}</option>
          ))}
        </select>
      </div>
    );
  }
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={fieldId} className="text-xs text-gray-500 font-medium">{f.label}</label>
      <input
        id={fieldId}
        type={f.type === 'date' ? 'date' : 'text'}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={f.placeholder}
        className="rounded border border-gray-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
      />
    </div>
  );
}

export function MessageFilters({ primary, secondary, values, onSearch, onReset }: Props) {
  const [draft, setDraft] = useState<Record<string, string>>(values);
  const [showExtra, setShowExtra] = useState(false);

  useEffect(() => {
    setDraft(values);
  }, [values]);

  const handleReset = () => {
    const empty: Record<string, string> = {};
    [...primary, ...secondary].forEach((f) => { empty[f.key] = ''; });
    setDraft(empty);
    onReset();
  };

  const set = (key: string, val: string) => setDraft((prev) => ({ ...prev, [key]: val }));

  const renderField = (f: FilterDef) => <Field key={f.key} f={f} value={draft[f.key] ?? ''} onChange={(v) => set(f.key, v)} />;

  return (
    <div className="bg-white border border-gray-200 rounded-lg p-4 mb-4">
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-3 mb-3">
        {primary.map(renderField)}
      </div>
      {showExtra && (
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3 mb-3 pt-3 border-t border-gray-100">
          {secondary.map(renderField)}
        </div>
      )}
      <div className="flex items-center justify-between">
        <button
          type="button"
          onClick={() => setShowExtra((v) => !v)}
          className="text-sm text-gray-500 hover:text-gray-700 flex items-center gap-1"
        >
          <svg
            className={`w-4 h-4 transition-transform ${showExtra ? 'rotate-180' : ''}`}
            fill="none" stroke="currentColor" viewBox="0 0 24 24"
          >
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
          {showExtra ? 'Скрыть дополнительные' : 'Дополнительные фильтры'}
        </button>
        <div className="flex gap-2">
          <Button variant="secondary" onClick={handleReset}>Сбросить</Button>
          <Button onClick={() => onSearch(draft)}>Найти</Button>
        </div>
      </div>
    </div>
  );
}
