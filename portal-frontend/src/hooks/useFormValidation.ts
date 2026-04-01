import { useCallback, useRef, useState } from 'react';

export interface ValidationRule {
  required?: boolean;
  minLength?: number;
  maxLength?: number;
  pattern?: RegExp;
  patternMessage?: string;
}

export type ValidationRules<K extends string> = Record<K, ValidationRule>;

function validate(value: string, rule: ValidationRule): string {
  if (rule.required && !value.trim()) return 'Поле обязательно для заполнения';
  if (rule.minLength !== undefined && value.trim().length < rule.minLength)
    return `Минимум ${rule.minLength} символов`;
  if (rule.maxLength !== undefined && value.length > rule.maxLength)
    return `Максимум ${rule.maxLength} символов`;
  if (rule.pattern && !rule.pattern.test(value))
    return rule.patternMessage ?? 'Неверный формат';
  return '';
}

export function useFormValidation<K extends string>(rules: ValidationRules<K>) {
  const [errors, setErrors] = useState<Partial<Record<K, string>>>({});
  const [touched, setTouched] = useState<Partial<Record<K, boolean>>>({});
  const valuesRef = useRef<Partial<Record<K, string>>>({});

  const onChange = useCallback(
    (name: K, value: string) => {
      valuesRef.current[name] = value;
      if (touched[name]) {
        const err = validate(value, rules[name]);
        setErrors((prev) => ({ ...prev, [name]: err }));
      }
    },
    [touched, rules],
  );

  const onBlur = useCallback(
    (name: K, value: string) => {
      valuesRef.current[name] = value;
      setTouched((prev) => ({ ...prev, [name]: true }));
      const err = validate(value, rules[name]);
      setErrors((prev) => ({ ...prev, [name]: err }));
    },
    [rules],
  );

  const validateAll = useCallback(
    (values: Record<K, string>): boolean => {
      const newErrors: Partial<Record<K, string>> = {};
      const newTouched: Partial<Record<K, boolean>> = {};
      let valid = true;
      for (const name of Object.keys(rules) as K[]) {
        newTouched[name] = true;
        const err = validate(values[name] ?? '', rules[name]);
        newErrors[name] = err;
        if (err) valid = false;
      }
      setTouched(newTouched);
      setErrors(newErrors);
      return valid;
    },
    [rules],
  );

  const scrollToFirstError = useCallback(() => {
    const el = document.querySelector('[aria-invalid="true"]');
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'center' });
      (el as HTMLElement).focus?.();
    }
  }, []);

  const fieldProps = useCallback(
    (name: K) => ({
      onChange: (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
        onChange(name, e.target.value),
      onBlur: (e: React.FocusEvent<HTMLInputElement | HTMLTextAreaElement>) =>
        onBlur(name, e.target.value),
      'aria-invalid': errors[name] ? ('true' as const) : undefined,
      'aria-describedby': errors[name] ? `${name}-error` : undefined,
    }),
    [errors, onChange, onBlur],
  );

  return { errors, touched, fieldProps, validateAll, scrollToFirstError };
}
