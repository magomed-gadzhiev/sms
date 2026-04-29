import { describe, it, expect } from 'vitest';
import { pluralizeRu, pluralizeRules } from './pluralize';

describe('pluralizeRu', () => {
  const forms: [string, string, string] = ['яблоко', 'яблока', 'яблок'];

  it('picks singular for 1, 21, 101 (mod10==1, mod100!=11)', () => {
    expect(pluralizeRu(1, forms)).toBe('яблоко');
    expect(pluralizeRu(21, forms)).toBe('яблоко');
    expect(pluralizeRu(101, forms)).toBe('яблоко');
  });

  it('picks few (genitive singular) for 2, 3, 4 and their tens variants', () => {
    expect(pluralizeRu(2, forms)).toBe('яблока');
    expect(pluralizeRu(3, forms)).toBe('яблока');
    expect(pluralizeRu(4, forms)).toBe('яблока');
    expect(pluralizeRu(22, forms)).toBe('яблока');
    expect(pluralizeRu(103, forms)).toBe('яблока');
  });

  it('picks plural for 0, 5..20, 25..30', () => {
    expect(pluralizeRu(0, forms)).toBe('яблок');
    expect(pluralizeRu(5, forms)).toBe('яблок');
    expect(pluralizeRu(11, forms)).toBe('яблок');
    expect(pluralizeRu(12, forms)).toBe('яблок');
    expect(pluralizeRu(13, forms)).toBe('яблок');
    expect(pluralizeRu(14, forms)).toBe('яблок');
    expect(pluralizeRu(15, forms)).toBe('яблок');
    expect(pluralizeRu(20, forms)).toBe('яблок');
    expect(pluralizeRu(25, forms)).toBe('яблок');
  });

  it('handles negative numbers like positive (via abs)', () => {
    expect(pluralizeRu(-1, forms)).toBe('яблоко');
    expect(pluralizeRu(-2, forms)).toBe('яблока');
    expect(pluralizeRu(-5, forms)).toBe('яблок');
  });

  it('pluralizeRules matches the специфичная форма', () => {
    expect(pluralizeRules(1)).toBe('правило');
    expect(pluralizeRules(2)).toBe('правила');
    expect(pluralizeRules(5)).toBe('правил');
    expect(pluralizeRules(11)).toBe('правил');
    expect(pluralizeRules(21)).toBe('правило');
  });
});
