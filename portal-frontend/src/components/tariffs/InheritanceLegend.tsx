interface ChipProps {
  className: string;
  label: string;
  hint: string;
}

function Chip({ className, label, hint }: ChipProps) {
  return (
    <span
      className="inline-flex items-center gap-1.5 text-xs text-slate-600 dark:text-slate-400"
      title={hint}
    >
      <span className={`inline-block w-3 h-3 rounded border ${className}`} aria-hidden />
      {label}
    </span>
  );
}

export function InheritanceLegend() {
  return (
    <div
      className="flex flex-wrap items-center gap-4"
      role="group"
      aria-label="Легенда наследования цен"
    >
      <Chip
        className="bg-emerald-50 border-emerald-400 dark:bg-emerald-950/30 dark:border-emerald-700"
        label="Из шаблона"
        hint="Цена унаследована из привязанного шаблона"
      />
      <Chip
        className="bg-violet-50 border-violet-400 dark:bg-violet-950/30 dark:border-violet-700"
        label="Переопределено"
        hint="Цена задана только для этого субаккаунта"
      />
      <Chip
        className="bg-white border-slate-300 dark:bg-slate-900 dark:border-slate-700"
        label="Не задано"
        hint="Используется значение по умолчанию"
      />
    </div>
  );
}
