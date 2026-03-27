import { Input } from '../ui/Input';
import { Select } from '../ui/Select';
import { Button } from '../ui/Button';

export interface FilterDef {
  key: string;
  label: string;
  type: 'text' | 'select' | 'date';
  options?: { value: string; label: string }[];
  placeholder?: string;
}

interface FilterBarProps {
  filters: FilterDef[];
  values: Record<string, string>;
  onChange: (values: Record<string, string>) => void;
  onReset?: () => void;
}

export function FilterBar({ filters, values, onChange, onReset }: FilterBarProps) {
  const update = (key: string, value: string) => {
    onChange({ ...values, [key]: value });
  };

  const hasValues = Object.values(values).some((v) => v !== '');

  return (
    <div className="flex flex-wrap items-end gap-3 mb-4">
      {filters.map((f) => {
        if (f.type === 'select' && f.options) {
          return (
            <div key={f.key} className="min-w-[160px]">
              <Select
                label={f.label}
                options={f.options}
                value={values[f.key] || ''}
                onChange={(v) => update(f.key, v)}
                placeholder={f.placeholder || 'All'}
              />
            </div>
          );
        }
        return (
          <div key={f.key} className="min-w-[160px]">
            <Input
              label={f.label}
              type={f.type === 'date' ? 'date' : 'text'}
              value={values[f.key] || ''}
              onChange={(e) => update(f.key, e.target.value)}
              placeholder={f.placeholder}
            />
          </div>
        );
      })}
      {hasValues && onReset && (
        <Button variant="ghost" size="sm" onClick={onReset}>
          Reset
        </Button>
      )}
    </div>
  );
}
