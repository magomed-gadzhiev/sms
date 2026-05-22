import { describe, it, expect } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useFormValidation } from './useFormValidation';

describe('useFormValidation', () => {
  it('does not set an error before the field is touched', () => {
    const { result } = renderHook(() =>
      useFormValidation({ email: { required: true } }),
    );
    act(() => result.current.fieldProps('email').onChange({ target: { value: '' } } as React.ChangeEvent<HTMLInputElement>));
    expect(result.current.errors.email).toBeUndefined();
  });

  it('surfaces "required" error on blur with empty value', () => {
    const { result } = renderHook(() =>
      useFormValidation({ email: { required: true } }),
    );
    act(() => result.current.fieldProps('email').onBlur({ target: { value: '' } } as React.FocusEvent<HTMLInputElement>));
    expect(result.current.errors.email).toBe('Поле обязательно для заполнения');
  });

  it('enforces minLength on blur', () => {
    const { result } = renderHook(() =>
      useFormValidation({ password: { minLength: 8 } }),
    );
    act(() => result.current.fieldProps('password').onBlur({ target: { value: 'short' } } as React.FocusEvent<HTMLInputElement>));
    expect(result.current.errors.password).toBe('Минимум 8 символов');
  });

  it('enforces maxLength', () => {
    const { result } = renderHook(() =>
      useFormValidation({ name: { maxLength: 3 } }),
    );
    act(() => result.current.fieldProps('name').onBlur({ target: { value: 'long' } } as React.FocusEvent<HTMLInputElement>));
    expect(result.current.errors.name).toBe('Максимум 3 символов');
  });

  it('uses patternMessage when pattern does not match', () => {
    const { result } = renderHook(() =>
      useFormValidation({
        email: {
          pattern: /^\S+@\S+\.\S+$/,
          patternMessage: 'Неверный e-mail',
        },
      }),
    );
    act(() =>
      result.current.fieldProps('email').onBlur({
        target: { value: 'no-at-sign' },
      } as React.FocusEvent<HTMLInputElement>),
    );
    expect(result.current.errors.email).toBe('Неверный e-mail');
  });

  it('clears error once a touched field becomes valid on change', () => {
    const { result } = renderHook(() =>
      useFormValidation({ email: { required: true } }),
    );
    act(() => result.current.fieldProps('email').onBlur({ target: { value: '' } } as React.FocusEvent<HTMLInputElement>));
    expect(result.current.errors.email).toBeTruthy();
    act(() => result.current.fieldProps('email').onChange({ target: { value: 'ok@example.com' } } as React.ChangeEvent<HTMLInputElement>));
    expect(result.current.errors.email).toBe('');
  });

  it('validateAll returns false and fills all errors+touched for invalid form', () => {
    const { result } = renderHook(() =>
      useFormValidation({
        email: { required: true },
        password: { minLength: 8 },
      }),
    );
    let ok = true;
    act(() => {
      ok = result.current.validateAll({ email: '', password: 'x' });
    });
    expect(ok).toBe(false);
    expect(result.current.errors.email).toBe('Поле обязательно для заполнения');
    expect(result.current.errors.password).toBe('Минимум 8 символов');
    expect(result.current.touched.email).toBe(true);
    expect(result.current.touched.password).toBe(true);
  });

  it('validateAll returns true for a valid form', () => {
    const { result } = renderHook(() =>
      useFormValidation({
        email: { required: true },
        password: { minLength: 8 },
      }),
    );
    let ok = false;
    act(() => {
      ok = result.current.validateAll({
        email: 'user@example.com',
        password: 'longenough',
      });
    });
    expect(ok).toBe(true);
    expect(result.current.errors.email).toBe('');
    expect(result.current.errors.password).toBe('');
  });

  it('fieldProps exposes aria-invalid and aria-describedby when in error', () => {
    const { result } = renderHook(() =>
      useFormValidation({ email: { required: true } }),
    );
    act(() =>
      result.current.fieldProps('email').onBlur({
        target: { value: '' },
      } as React.FocusEvent<HTMLInputElement>),
    );
    const props = result.current.fieldProps('email');
    expect(props['aria-invalid']).toBe('true');
    expect(props['aria-describedby']).toBe('email-error');
  });

  it('reset clears errors and touched state', () => {
    const { result } = renderHook(() =>
      useFormValidation({ email: { required: true } }),
    );
    act(() =>
      result.current.fieldProps('email').onBlur({
        target: { value: '' },
      } as React.FocusEvent<HTMLInputElement>),
    );
    expect(result.current.errors.email).toBeTruthy();
    act(() => result.current.reset());
    expect(result.current.errors).toEqual({});
    expect(result.current.touched).toEqual({});
  });
});
