import { type ReactNode } from 'react';

const variantStyles = {
  default: 'bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-slate-300',
  success: 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-200',
  warning: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-200',
  danger: 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-200',
  info: 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-200',
} as const;

interface BadgeProps {
  variant?: keyof typeof variantStyles;
  children: ReactNode;
}

export function Badge({ variant = 'default', children }: BadgeProps) {
  return (
    <span className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${variantStyles[variant]}`}>
      {children}
    </span>
  );
}

const statusMap: Record<string, { variant: keyof typeof variantStyles; label: string }> = {
  active: { variant: 'success', label: 'Активен' },
  inactive: { variant: 'default', label: 'Неактивен' },
  delivered: { variant: 'success', label: 'Доставлено' },
  failed: { variant: 'danger', label: 'Ошибка' },
  pending: { variant: 'warning', label: 'Ожидание' },
  blocked: { variant: 'danger', label: 'Заблокирован' },
  approved: { variant: 'success', label: 'Одобрен' },
  rejected: { variant: 'danger', label: 'Отклонён' },
  draft: { variant: 'default', label: 'Черновик' },
  review: { variant: 'info', label: 'На модерации' },
  revision_requested: { variant: 'warning', label: 'Требуется доработка' },
  healthy: { variant: 'success', label: 'Работает' },
  unhealthy: { variant: 'danger', label: 'Не работает' },
  degraded: { variant: 'warning', label: 'Деградация' },
};

export function StatusBadge({ status }: { status: string }) {
  const config = statusMap[status.toLowerCase()] || { variant: 'default' as const, label: status };
  return <Badge variant={config.variant}>{config.label}</Badge>;
}
