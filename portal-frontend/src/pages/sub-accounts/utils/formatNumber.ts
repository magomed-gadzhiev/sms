const numberFmt = new Intl.NumberFormat('ru-RU');

export function formatNumber(n: number): string {
  return numberFmt.format(n);
}

const rubFmt = new Intl.NumberFormat('ru-RU', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

export function formatRub(value: number | string | null | undefined): string {
  if (value === null || value === undefined) return '—';
  const n = typeof value === 'string' ? parseFloat(value) : value;
  if (!Number.isFinite(n)) return '—';
  return `${rubFmt.format(n)} ₽`;
}
