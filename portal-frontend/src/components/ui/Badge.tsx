import { type ReactNode } from 'react';

const variantStyles = {
  default: 'bg-gray-100 text-gray-700',
  success: 'bg-green-100 text-green-800',
  warning: 'bg-yellow-100 text-yellow-800',
  danger: 'bg-red-100 text-red-800',
  info: 'bg-blue-100 text-blue-800',
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
  active: { variant: 'success', label: 'Active' },
  inactive: { variant: 'default', label: 'Inactive' },
  delivered: { variant: 'success', label: 'Delivered' },
  failed: { variant: 'danger', label: 'Failed' },
  pending: { variant: 'warning', label: 'Pending' },
  blocked: { variant: 'danger', label: 'Blocked' },
  approved: { variant: 'success', label: 'Approved' },
  rejected: { variant: 'danger', label: 'Rejected' },
  healthy: { variant: 'success', label: 'Healthy' },
  unhealthy: { variant: 'danger', label: 'Unhealthy' },
  degraded: { variant: 'warning', label: 'Degraded' },
};

export function StatusBadge({ status }: { status: string }) {
  const config = statusMap[status.toLowerCase()] || { variant: 'default' as const, label: status };
  return <Badge variant={config.variant}>{config.label}</Badge>;
}
