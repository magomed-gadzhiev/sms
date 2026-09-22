// Message Status vocabulary — frontend twin of the backend module
// internal/shared/messagestatus. Single owner of the status enum, labels,
// badge variants and terminality; per-page label/color maps are banned.
//
// Domain rule (CONTEXT.md): `unknown` is never a stored status, so it has no
// entry here; unrecognized statuses fall back to a neutral chip via
// messageStatusMeta.

export const MESSAGE_STATUSES = [
  'pending',
  'queued',
  'sent',
  'delivered',
  'failed',
  'expired',
  'rejected',
  'scheduled',
  'cancelled',
] as const;

export type MessageStatus = (typeof MESSAGE_STATUSES)[number];

export type BadgeVariant = 'default' | 'success' | 'warning' | 'danger' | 'info';

export interface MessageStatusMeta {
  label: string;
  badgeVariant: BadgeVariant;
  /** Tailwind chip classes for pages rendering custom status spans. */
  colorClass: string;
  terminal: boolean;
}

export const messageStatus: Record<MessageStatus, MessageStatusMeta> = {
  pending: {
    label: 'Ожидание',
    badgeVariant: 'warning',
    colorClass: 'bg-yellow-100 text-yellow-800',
    terminal: false,
  },
  queued: {
    label: 'В очереди',
    badgeVariant: 'warning',
    colorClass: 'bg-yellow-100 text-yellow-800',
    terminal: false,
  },
  sent: {
    label: 'Отправлено',
    badgeVariant: 'info',
    colorClass: 'bg-blue-100 text-blue-800',
    terminal: false,
  },
  delivered: {
    label: 'Доставлено',
    badgeVariant: 'success',
    colorClass: 'bg-green-100 text-green-800',
    terminal: true,
  },
  failed: {
    label: 'Ошибка',
    badgeVariant: 'danger',
    colorClass: 'bg-red-100 text-red-800',
    terminal: true,
  },
  expired: {
    label: 'Истекло',
    badgeVariant: 'danger',
    colorClass: 'bg-gray-100 text-gray-600',
    terminal: true,
  },
  rejected: {
    label: 'Отклонено',
    badgeVariant: 'danger',
    colorClass: 'bg-red-100 text-red-800',
    terminal: true,
  },
  scheduled: {
    label: 'Запланировано',
    badgeVariant: 'info',
    colorClass: 'bg-purple-100 text-purple-800',
    terminal: false,
  },
  cancelled: {
    label: 'Отменено',
    badgeVariant: 'default',
    colorClass: 'bg-gray-100 text-gray-600',
    terminal: true,
  },
};

export function isMessageStatus(status: string): status is MessageStatus {
  return (MESSAGE_STATUSES as readonly string[]).includes(status);
}

/** Terminal statuses: no further transition expected. */
export function isTerminal(status: string): boolean {
  return isMessageStatus(status) && messageStatus[status].terminal;
}

/** Failure outcomes: the terminal statuses that did not deliver. */
export function isFailureOutcome(status: string): boolean {
  return status === 'failed' || status === 'rejected' || status === 'expired';
}

/** Status metadata with a neutral fallback for unrecognized values. */
export function messageStatusMeta(status: string): MessageStatusMeta {
  if (isMessageStatus(status)) return messageStatus[status];
  return { label: status, badgeVariant: 'default', colorClass: 'bg-gray-100 text-gray-700', terminal: false };
}
