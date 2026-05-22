import { describe, it, expect } from 'vitest';
import { formatNumber, formatRub } from './formatNumber';

// Intl.NumberFormat("ru-RU") uses U+00A0 (non-breaking space) as thousands separator on this Node/ICU version.
// The expected strings below use U+00A0 between digit groups, NOT regular space U+0020.

describe('formatNumber', () => {
  it('formats integers with ru-RU thousands separator', () => {
    expect(formatNumber(1000)).toBe('1 000');
    expect(formatNumber(150000)).toBe('150 000');
    expect(formatNumber(3000000)).toBe('3 000 000');
  });
  it('handles zero', () => {
    expect(formatNumber(0)).toBe('0');
  });
  it('handles negative numbers', () => {
    expect(formatNumber(-1500)).toBe('-1 500');
  });
});

describe('formatRub', () => {
  it('formats balance as "99 975,00 ₽"', () => {
    expect(formatRub(99975)).toBe('99 975,00 ₽');
    expect(formatRub(0)).toBe('0,00 ₽');
    expect(formatRub(99975.5)).toBe('99 975,50 ₽');
  });
  it('accepts string input', () => {
    expect(formatRub('99975.00')).toBe('99 975,00 ₽');
  });
  it('returns em-dash for NaN-like input', () => {
    expect(formatRub('not a number')).toBe('—');
    expect(formatRub(NaN)).toBe('—');
  });
  it('returns em-dash for null/undefined', () => {
    expect(formatRub(null)).toBe('—');
    expect(formatRub(undefined)).toBe('—');
  });
});
