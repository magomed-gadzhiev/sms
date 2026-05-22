import { useState } from 'react';
import { Button } from '../../../components/ui/Button';

export interface ColumnDef {
  key: string;
  label: string;
  defaultVisible: boolean;
}

interface Props {
  columns: ColumnDef[];
  visible: Set<string>;
  onChange: (visible: Set<string>) => void;
}

export function ColumnConfigurator({ columns, visible, onChange }: Props) {
  const [open, setOpen] = useState(false);

  const toggle = (key: string) => {
    const next = new Set(visible);
    if (next.has(key)) {
      next.delete(key);
    } else {
      next.add(key);
    }
    onChange(next);
  };

  return (
    <div className="relative">
      <Button variant="secondary" onClick={() => setOpen((v) => !v)}>
        <svg className="w-4 h-4 mr-1 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
        </svg>
        Настройка таблицы
      </Button>

      {open && (
        <>
          <div className="fixed inset-0 z-10" onClick={() => setOpen(false)} />
          <div className="absolute right-0 top-full mt-1 z-20 w-56 bg-white border border-gray-200 rounded-lg shadow-lg p-3">
            <p className="text-xs font-medium text-gray-500 mb-2 uppercase tracking-wide">Столбцы</p>
            <div className="space-y-1">
              {columns.map((col) => (
                <label key={col.key} className="flex items-center gap-2 text-sm cursor-pointer hover:bg-gray-50 rounded px-1 py-0.5">
                  <input
                    type="checkbox"
                    checked={visible.has(col.key)}
                    onChange={() => toggle(col.key)}
                    className="rounded"
                  />
                  {col.label}
                </label>
              ))}
            </div>
          </div>
        </>
      )}
    </div>
  );
}
