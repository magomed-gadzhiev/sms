interface Props {
  current: number;
  max: number;
}

export function CharacterCounter({ current, max }: Props) {
  const remaining = max - current;
  const segments = current <= 160 ? 1 : Math.ceil(current / 153);

  let colorClass = 'text-green-600';
  if (current > 160) colorClass = 'text-red-600';
  else if (current > 140) colorClass = 'text-yellow-600';

  return (
    <span className={`text-xs ${colorClass}`} aria-live="polite">
      {remaining >= 0 ? `${remaining} осталось` : `${Math.abs(remaining)} лишних`}
      {segments > 1 && ` · ${segments} SMS`}
    </span>
  );
}
