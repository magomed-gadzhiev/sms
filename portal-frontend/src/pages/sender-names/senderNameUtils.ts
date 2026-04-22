export const STATUS_BADGE: Record<string, { variant: 'warning' | 'success' | 'danger' | 'default'; label: string }> = {
  pending:     { variant: 'warning', label: 'На модерации' },
  approved:    { variant: 'success', label: 'Одобрено' },
  rejected:    { variant: 'danger',  label: 'Отклонено' },
  deactivated: { variant: 'default', label: 'Деактивировано' },
};
