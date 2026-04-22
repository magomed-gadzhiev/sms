// Shared price formatter for the tariffs matrix.
// `null` → em-dash placeholder; numeric → `value.toFixed(2) + ' ' + symbol(currency)`.
// RUB → ₽; otherwise falls back to the 3-letter currency code.

const CURRENCY_SYMBOLS: Record<string, string> = {
  RUB: '\u20BD',
  USD: '$',
  EUR: '\u20AC',
  KZT: '\u20B8',
  BYN: 'Br',
  UAH: '\u20B4',
};

export function currencySymbol(currency: string): string {
  const key = currency.toUpperCase();
  return CURRENCY_SYMBOLS[key] ?? key;
}

export function formatPrice(value: number | null | undefined, currency: string): string {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '\u2014';
  }
  return `${value.toFixed(2)} ${currencySymbol(currency)}`;
}
