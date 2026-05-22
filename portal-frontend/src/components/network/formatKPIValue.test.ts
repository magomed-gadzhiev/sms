import { describe, it, expect } from 'vitest';
import { formatKPIValue } from './formatKPIValue';
import type { KPI } from '../../api/networkStats';

function k(partial: Partial<KPI>): KPI {
  return { name: 't', ...partial };
}

describe('formatKPIValue', () => {
  it('returns em-dash when value is undefined', () => {
    expect(formatKPIValue(k({ value: undefined, format: 'count' }))).toBe('—');
  });

  it('returns em-dash when value is null (defensive)', () => {
    // value typed as number | undefined, but defensively handle null at runtime
    expect(formatKPIValue(k({ value: null as unknown as number, format: 'count' }))).toBe('—');
  });

  it('renders zero as "0" for count (zero is valid, not no-data)', () => {
    expect(formatKPIValue(k({ value: 0, format: 'count' }))).toBe('0');
  });

  it('renders 12345 as ru-RU grouped (NBSP/space separator)', () => {
    expect(formatKPIValue(k({ value: 12345, format: 'count' }))).toMatch(/^12[\s ]345$/);
  });

  it('renders 0.7084 as "70,8%" for percent', () => {
    expect(formatKPIValue(k({ value: 0.7084, format: 'percent' }))).toBe('70,8%');
  });

  it('renders 0.16974 as "17,0%" for percent (F2 fix)', () => {
    expect(formatKPIValue(k({ value: 0.16974, format: 'percent' }))).toBe('17,0%');
  });

  it('renders 1500.5 as ru-RU currency RUB', () => {
    expect(formatKPIValue(k({ value: 1500.5, format: 'currency', currency: 'RUB' }))).toMatch(
      /1[\s ]500[,.]50?\s?₽/,
    );
  });

  it('falls back to toString when no format', () => {
    expect(formatKPIValue(k({ value: 42 }))).toBe('42');
  });
});
