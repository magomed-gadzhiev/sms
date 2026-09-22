import { describe, expect, it } from 'vitest';

import {
  MESSAGE_STATUSES,
  isFailureOutcome,
  isMessageStatus,
  isTerminal,
  messageStatus,
  messageStatusMeta,
} from './messageStatus';

describe('messageStatus vocabulary', () => {
  it('covers exactly the CONTEXT.md lifecycle statuses', () => {
    expect([...MESSAGE_STATUSES].sort()).toEqual(
      ['cancelled', 'delivered', 'expired', 'failed', 'pending', 'queued', 'rejected', 'scheduled', 'sent'].sort(),
    );
  });

  it('every status has a label, variant, color and terminality', () => {
    for (const s of MESSAGE_STATUSES) {
      const meta = messageStatus[s];
      expect(meta.label.length).toBeGreaterThan(0);
      expect(meta.badgeVariant).toBeTruthy();
      expect(meta.colorClass).toMatch(/^bg-/);
      expect(typeof meta.terminal).toBe('boolean');
    }
  });

  it('terminal set matches the domain rule', () => {
    const terminal = MESSAGE_STATUSES.filter((s) => messageStatus[s].terminal);
    expect(terminal.sort()).toEqual(['cancelled', 'delivered', 'expired', 'failed', 'rejected'].sort());
  });

  it('isTerminal answers for raw strings', () => {
    expect(isTerminal('delivered')).toBe(true);
    expect(isTerminal('failed')).toBe(true);
    expect(isTerminal('sent')).toBe(false);
    expect(isTerminal('unknown')).toBe(false);
    expect(isTerminal('')).toBe(false);
  });

  it('unknown statuses never claim membership (unknown is never stored)', () => {
    expect(isMessageStatus('unknown')).toBe(false);
    expect(isMessageStatus('UNKNOWN')).toBe(false);
    expect(isMessageStatus('Delivered')).toBe(false);
  });

  it('isFailureOutcome covers failed/rejected/expired only', () => {
    expect(isFailureOutcome('failed')).toBe(true);
    expect(isFailureOutcome('rejected')).toBe(true);
    expect(isFailureOutcome('expired')).toBe(true);
    expect(isFailureOutcome('delivered')).toBe(false);
    expect(isFailureOutcome('cancelled')).toBe(false);
    expect(isFailureOutcome('sent')).toBe(false);
  });

  it('messageStatusMeta falls back to a neutral chip', () => {
    const meta = messageStatusMeta('mystery');
    expect(meta.label).toBe('mystery');
    expect(meta.badgeVariant).toBe('default');
    expect(meta.terminal).toBe(false);
  });
});
