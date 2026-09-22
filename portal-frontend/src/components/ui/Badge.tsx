import { type ReactNode } from 'react';

import { isMessageStatus, messageStatusMeta } from '../../utils/messageStatus';

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

// Entity statuses (providers, contacts, templates, health...). Message
// statuses are NOT listed here — they resolve through the messageStatus
// vocabulary so labels/variants have a single owner.
const entityStatusMap: Record<string, { variant: keyof typeof variantStyles; label: string }> = {
  active: { variant: 'success', label: 'Активен' },
  inactive: { variant: 'default', label: 'Неактивен' },
  blocked: { variant: 'danger', label: 'Заблокирован' },
  approved: { variant: 'success', label: 'Одобрен' },
  draft: { variant: 'default', label: 'Черновик' },
  review: { variant: 'info', label: 'На модерации' },
  revision_requested: { variant: 'warning', label: 'Требуется доработка' },
  healthy: { variant: 'success', label: 'Работает' },
  unhealthy: { variant: 'danger', label: 'Не работает' },
  degraded: { variant: 'warning', label: 'Деградация' },
};

export function StatusBadge({ status }: { status: string }) {
  const key = status.toLowerCase();
  if (isMessageStatus(key)) {
    const meta = messageStatusMeta(key);
    return <Badge variant={meta.badgeVariant}>{meta.label}</Badge>;
  }
  const config = entityStatusMap[key] || { variant: 'default' as const, label: status };
  return <Badge variant={config.variant}>{config.label}</Badge>;
}
