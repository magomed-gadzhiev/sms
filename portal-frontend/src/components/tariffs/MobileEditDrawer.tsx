import { useEffect, useState } from 'react';
import { Button } from '../ui/Button';
import { formatQuantity } from './formatQuantity';
import { currencySymbol } from './formatPrice';

interface Tier {
  id: string;
  from_quantity: number;
}

interface Operator {
  id: string;
  name: string;
}

interface DraftEntry {
  tierId: string;
  raw: string;
  valid: boolean;
}

interface Props {
  operator: Operator;
  tiers: Tier[];
  initialValues: Map<string, string>;
  currency: string;
  onCancel: () => void;
  onApply: (drafts: DraftEntry[]) => void;
}

function isValidPrice(raw: string): boolean {
  const trimmed = raw.trim().replace(',', '.');
  if (trimmed === '') return true;
  const n = Number(trimmed);
  return Number.isFinite(n) && n >= 0 && n <= 999;
}

export function MobileEditDrawer({
  operator,
  tiers,
  initialValues,
  currency,
  onCancel,
  onApply,
}: Props) {
  const [drafts, setDrafts] = useState<Map<string, DraftEntry>>(() => {
    const m = new Map<string, DraftEntry>();
    for (const t of tiers) {
      const raw = initialValues.get(t.id) ?? '';
      m.set(t.id, { tierId: t.id, raw, valid: isValidPrice(raw) });
    }
    return m;
  });

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onCancel();
    };
    document.addEventListener('keydown', onKey);
    document.body.style.overflow = 'hidden';
    return () => {
      document.removeEventListener('keydown', onKey);
      document.body.style.overflow = '';
    };
  }, [onCancel]);

  const update = (id: string, raw: string) => {
    setDrafts((prev) => {
      const next = new Map(prev);
      next.set(id, { tierId: id, raw, valid: isValidPrice(raw) });
      return next;
    });
  };

  const anyInvalid = Array.from(drafts.values()).some((d) => !d.valid);

  return (
    <div
      className="fixed inset-0 z-40 flex items-end"
      role="dialog"
      aria-modal="true"
      aria-label={`Тарифы для ${operator.name}`}
    >
      <div className="absolute inset-0 bg-black/50" onClick={onCancel} />
      <div className="relative w-full bg-white dark:bg-slate-900 rounded-t-lg p-4 max-h-[80vh] overflow-y-auto">
        <header className="mb-3">
          <h2 className="font-semibold text-slate-900 dark:text-slate-100">{operator.name}</h2>
          <p className="text-xs text-slate-500">
            Цена за SMS, {currencySymbol(currency)}
          </p>
        </header>
        <div className="space-y-3 mb-4">
          {tiers.map((t) => {
            const d = drafts.get(t.id);
            if (!d) return null;
            return (
              <label key={t.id} className="block">
                <span className="text-sm text-slate-700 dark:text-slate-300">
                  При объёме {formatQuantity(t.from_quantity)} сообщений
                </span>
                <input
                  type="text"
                  inputMode="decimal"
                  className={`mt-1 block w-full border rounded px-3 py-2 text-base bg-white dark:bg-slate-800 dark:text-slate-100 ${
                    d.valid
                      ? 'border-slate-300 dark:border-slate-700'
                      : 'border-red-500'
                  }`}
                  value={d.raw}
                  onChange={(e) => update(t.id, e.target.value)}
                />
                {!d.valid && (
                  <span className="text-xs text-red-600 dark:text-red-400">
                    Некорректное значение (0–999)
                  </span>
                )}
              </label>
            );
          })}
        </div>
        <div className="flex gap-2 justify-end">
          <Button variant="secondary" onClick={onCancel}>
            Отмена
          </Button>
          <Button
            disabled={anyInvalid}
            onClick={() => onApply(Array.from(drafts.values()))}
          >
            Применить
          </Button>
        </div>
      </div>
    </div>
  );
}
