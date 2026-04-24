import { describe, it, expect, afterEach } from 'vitest';
import { getCookie } from './cookies';

describe('getCookie', () => {
  afterEach(() => {
    // Wipe cookies set during test.
    document.cookie.split(';').forEach((c) => {
      const eq = c.indexOf('=');
      const name = eq > -1 ? c.substring(0, eq).trim() : c.trim();
      document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`;
    });
  });

  it('returns null when cookie is absent', () => {
    expect(getCookie('missing')).toBeNull();
  });

  it('returns the value when cookie exists', () => {
    document.cookie = 'csrf_token=abc123; path=/';
    expect(getCookie('csrf_token')).toBe('abc123');
  });

  it('does not return a prefix-matching cookie', () => {
    document.cookie = 'csrf_token_other=wrong; path=/';
    expect(getCookie('csrf_token')).toBeNull();
  });

  it('finds cookie when it is not first in the jar', () => {
    document.cookie = 'first=one; path=/';
    document.cookie = 'second=two; path=/';
    document.cookie = 'csrf_token=found; path=/';
    expect(getCookie('csrf_token')).toBe('found');
  });
});
