import type { KPI } from '../../api/networkStats';

/**
 * Pure formatter for KPI values.
 *
 * Audit findings driving this implementation:
 *  - F2: backend returns ratio (e.g. 0.16974), UI used to render "0,17"
 *    without any "%" suffix → users read it as 0.17 absolute. Fix: when
 *    `format === 'percent'` multiply by 100 and append "%". Manual "%"
 *    suffix (instead of `Intl.NumberFormat({ style: 'percent' })`) so we
 *    get "17,0%" not "17 %" with a NBSP gap that ru-RU adds by default.
 *  - F3: backend now omits `value` for money KPIs when source data is
 *    unknown (Tasks 1+2 made `Value *float64`, omitempty). UI used to
 *    render `0 ₽` in green-ok which silently lied about absent data.
 *    Fix: when `value` is null/undefined return "—". KPICard then forces
 *    neutral ("unknown") status so the missing-data card never looks "good".
 */

const COUNT_FORMATTER = new Intl.NumberFormat('ru-RU');
const PERCENT_FORMATTER = new Intl.NumberFormat('ru-RU', {
  minimumFractionDigits: 1,
  maximumFractionDigits: 1,
});

const currencyFormatters = new Map<string, Intl.NumberFormat>();
function currencyFormatter(currency: string): Intl.NumberFormat {
  let f = currencyFormatters.get(currency);
  if (!f) {
    f = new Intl.NumberFormat('ru-RU', { style: 'currency', currency });
    currencyFormatters.set(currency, f);
  }
  return f;
}

export function formatKPIValue(kpi: KPI): string {
  const v = kpi.value;
  if (v === undefined || v === null || Number.isNaN(v)) {
    return '—';
  }
  switch (kpi.format) {
    case 'count':
      return COUNT_FORMATTER.format(v);
    case 'percent':
      return `${PERCENT_FORMATTER.format(v * 100)}%`;
    case 'currency':
      return currencyFormatter(kpi.currency || 'RUB').format(v);
    default:
      return String(v);
  }
}
