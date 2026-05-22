import { useEffect, useState } from 'react';

interface Props {
  /** ISO 8601 timestamp. */
  date: string;
  /** How often to re-render the relative label. Default 60s. Set 0 to disable. */
  refreshIntervalMs?: number;
  className?: string;
}

const RTF = new Intl.RelativeTimeFormat('ru-RU', { numeric: 'auto' });
const TZ = Intl.DateTimeFormat().resolvedOptions().timeZone;

function formatRelative(target: Date, now: Date): string {
  const diffMs = target.getTime() - now.getTime();
  const absS = Math.abs(diffMs) / 1000;
  if (absS < 60) return RTF.format(Math.round(diffMs / 1000), 'second');
  if (absS < 3600) return RTF.format(Math.round(diffMs / 60_000), 'minute');
  if (absS < 86_400) return RTF.format(Math.round(diffMs / 3_600_000), 'hour');

  const time = target.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const targetDayStart = new Date(target.getFullYear(), target.getMonth(), target.getDate()).getTime();
  const dayDelta = Math.round((targetDayStart - startOfToday) / 86_400_000);

  if (dayDelta === 0) return `сегодня в ${time}`;
  if (dayDelta === -1) return `вчера в ${time}`;
  if (dayDelta === 1) return `завтра в ${time}`;

  const sameYear = target.getFullYear() === now.getFullYear();
  const dateStr = sameYear
    ? target.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit' })
    : target.toLocaleDateString('ru-RU');
  return `${dateStr} в ${time}`;
}

function formatAbsolute(d: Date): string {
  const s = d.toLocaleString('ru-RU', { dateStyle: 'medium', timeStyle: 'medium' });
  return `${s} (${TZ})`;
}

export function RelativeTime({ date, refreshIntervalMs = 60_000, className }: Props) {
  const target = new Date(date);
  const [, setTick] = useState(0);

  useEffect(() => {
    if (!refreshIntervalMs) return;
    const id = setInterval(() => setTick((t) => t + 1), refreshIntervalMs);
    return () => clearInterval(id);
  }, [refreshIntervalMs]);

  if (isNaN(target.getTime())) return <span className={className}>—</span>;

  return (
    <time dateTime={date} title={formatAbsolute(target)} className={className}>
      {formatRelative(target, new Date())}
    </time>
  );
}
