// Format a tier's `from_quantity` as a short volume label: `0+`, `1000+`, `10k+`, `50k+`.
// For values ≥ 1000 we collapse to `{k-thousands}k+` when cleanly divisible by 1000, else emit raw N+.

export function formatQuantity(n: number): string {
  if (!Number.isFinite(n) || n < 0) return `${n}+`;
  if (n < 1000) return `${n}+`;
  if (n % 1000 === 0) return `${n / 1000}k+`;
  return `${n}+`;
}
