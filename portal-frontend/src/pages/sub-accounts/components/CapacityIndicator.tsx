interface CapacityIndicatorProps {
  current: number;
  max: number;
}

export function CapacityIndicator({ current, max }: CapacityIndicatorProps) {
  const free = Math.max(0, max - current);
  const pct = max > 0 ? Math.min(100, (current / max) * 100) : 0;
  const atLimit = current >= max;
  const barColor = atLimit ? 'bg-amber-500' : 'bg-primary';
  const trackColor = atLimit ? 'bg-amber-500/20' : 'bg-primary/20';

  return (
    <div className="flex flex-col gap-1.5 max-w-md">
      <div className="text-sm text-gray-600 dark:text-slate-400">
        <strong className="text-gray-900 dark:text-slate-100">{current} из {max}</strong> активных
        {!atLimit && ` · ${free} свободных слот${free === 1 ? '' : free < 5 ? 'а' : 'ов'}`}
        {atLimit && ' · лимит достигнут'}
      </div>
      <div
        className={`h-1 ${trackColor} rounded-full overflow-hidden`}
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={max}
        aria-valuenow={current}
        aria-label={`Использовано ${current} из ${max} суб-аккаунтов`}
      >
        <div className={`h-full ${barColor} rounded-full transition-all`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}
