import type { TariffEditorCell } from './TariffMatrix.types';
import { formatPrice } from './formatPrice';
import { formatQuantity } from './formatQuantity';

interface Tier {
  id: string;
  from_quantity: number;
}

interface Operator {
  id: string;
  name: string;
}

interface Props {
  operator: Operator;
  tiers: Tier[];
  cellsByTier: Map<string, TariffEditorCell | undefined>;
  currency: string;
  showInheritance: boolean;
  onEdit: () => void;
}

export function MobileOperatorCard({
  operator,
  tiers,
  cellsByTier,
  currency,
  showInheritance,
  onEdit,
}: Props) {
  return (
    <button
      type="button"
      onClick={onEdit}
      className="w-full text-left bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800
                 rounded-lg p-3 hover:bg-slate-50 dark:hover:bg-slate-800/50
                 focus:outline-none focus:ring-2 focus:ring-sky-500"
    >
      <header className="flex items-center gap-2 mb-2">
        <span className="inline-flex items-center justify-center w-7 h-7 rounded-full bg-slate-100 dark:bg-slate-800 text-sm font-medium text-slate-700 dark:text-slate-300">
          {operator.name.charAt(0).toUpperCase()}
        </span>
        <span className="font-medium text-slate-900 dark:text-slate-100">{operator.name}</span>
      </header>
      <div className="grid grid-cols-2 gap-2 text-sm">
        {tiers.map((t) => {
          const cell = cellsByTier.get(t.id);
          const source = cell?.source ?? 'unset';
          const effective = cell?.effective ?? null;
          let tint = '';
          if (showInheritance) {
            if (source === 'override') tint = 'bg-violet-50 dark:bg-violet-950/30';
            else if (source === 'template') tint = 'bg-emerald-50 dark:bg-emerald-950/30';
          }
          return (
            <div key={t.id} className={`rounded px-2 py-1 ${tint}`}>
              <div className="text-xs text-slate-500">{formatQuantity(t.from_quantity)}</div>
              <div className="tabular-nums text-slate-900 dark:text-slate-100">
                {effective === null ? '—' : formatPrice(effective, currency)}
              </div>
            </div>
          );
        })}
      </div>
    </button>
  );
}
