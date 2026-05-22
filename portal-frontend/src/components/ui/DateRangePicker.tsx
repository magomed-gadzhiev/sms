import { useId } from 'react';
import { Button } from './Button';
import { Input } from './Input';

export type DateRangeValue =
  | { type: 'preset'; preset: string }
  | { type: 'custom'; from: string; to: string };

interface Props {
  value: DateRangeValue;
  onChange: (v: DateRangeValue) => void;
  presets?: readonly string[];
  /** Optional label for the "от/до" cluster (a11y). */
  ariaLabel?: string;
}

const DEFAULT_PRESETS = ['7d', '30d', '90d'] as const;

export function DateRangePicker({
  value,
  onChange,
  presets = DEFAULT_PRESETS,
  ariaLabel = 'Период',
}: Props) {
  const isCustom = value.type === 'custom';
  const activePreset = value.type === 'preset' ? value.preset : null;
  const from = isCustom ? value.from : '';
  const to = isCustom ? value.to : '';
  const uid = useId();
  const fromId = `${uid}-from`;
  const toId = `${uid}-to`;

  return (
    <div className="flex gap-2 items-center flex-wrap" role="group" aria-label={ariaLabel}>
      {presets.map((p) => {
        const selected = !isCustom && activePreset === p;
        return (
          <Button
            key={p}
            type="button"
            size="sm"
            variant={selected ? 'primary' : 'secondary'}
            aria-pressed={selected}
            onClick={() => onChange({ type: 'preset', preset: p })}
          >
            {p}
          </Button>
        );
      })}
      <span className="mx-2 text-gray-600">или</span>
      <label htmlFor={fromId} className="flex items-center gap-1 text-sm text-gray-700">
        С:
        <Input
          id={fromId}
          type="date"
          value={from}
          onChange={(e) => onChange({ type: 'custom', from: e.target.value, to })}
          className="px-2 py-1"
        />
      </label>
      <label htmlFor={toId} className="flex items-center gap-1 text-sm text-gray-700">
        По:
        <Input
          id={toId}
          type="date"
          value={to}
          onChange={(e) => onChange({ type: 'custom', from, to: e.target.value })}
          className="px-2 py-1"
        />
      </label>
    </div>
  );
}
