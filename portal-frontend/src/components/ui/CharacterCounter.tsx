interface Props {
  current: number;
  max: number;
}

export function CharacterCounter({ current, max }: Props) {
  const remaining = max - current;
  const segments = current <= 160 ? 1 : Math.ceil(current / 153);

  let colorClass = 'text-green-600 dark:text-green-400';
  if (current > 160) colorClass = 'text-red-600 dark:text-red-400';
  else if (current > 140) colorClass = 'text-yellow-600 dark:text-yellow-400';

  return (
    <span className={`text-xs ${colorClass}`} aria-live="polite">
      {remaining >= 0 ? `${remaining} осталось` : `+${Math.abs(remaining)} сверх`}
      {segments > 1 && ` · ${segments} SMS`}
    </span>
  );
}
