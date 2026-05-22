import * as DropdownMenu from '@radix-ui/react-dropdown-menu';
import type { ReactNode } from 'react';
import type { NotificationItem } from '../../api/client';
import { RelativeTime } from './RelativeTime';

const TYPE_ICONS: Record<string, string> = {
  campaign_completed: '✅',
  campaign_failed: '❌',
};

interface Props {
  items: NotificationItem[];
  unreadCount: number;
  onMarkRead: (id: string) => void;
  onMarkAllRead: () => void;
  children: ReactNode;
}

export function NotificationPanel({ items, unreadCount, onMarkRead, onMarkAllRead, children }: Props) {
  return (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger asChild>{children}</DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content
          align="end"
          sideOffset={8}
          className="z-50 w-80 bg-white dark:bg-slate-900 rounded-lg shadow-lg border border-gray-200 dark:border-slate-700 overflow-hidden"
        >
          <div className="flex items-center justify-between px-4 py-3 border-b border-gray-100 dark:border-slate-800">
            <span className="font-semibold text-gray-900 dark:text-slate-100 text-sm">Уведомления</span>
            {unreadCount > 0 && (
              <button
                onClick={onMarkAllRead}
                className="text-xs text-blue-600 dark:text-blue-400 hover:underline"
              >
                Отметить все прочитанными
              </button>
            )}
          </div>

          <div className="max-h-80 overflow-y-auto">
            {items.length === 0 ? (
              <div className="px-4 py-6 text-sm text-gray-500 dark:text-slate-400 text-center">
                Нет уведомлений
              </div>
            ) : (
              items.map((n) => (
                <DropdownMenu.Item
                  key={n.id}
                  className={`flex items-start gap-3 px-4 py-3 text-sm cursor-pointer outline-none hover:bg-gray-50 dark:hover:bg-slate-800 border-b border-gray-50 dark:border-slate-800 last:border-0 ${
                    !n.is_read ? 'bg-blue-50/50 dark:bg-blue-950/30' : ''
                  }`}
                  onSelect={() => {
                    if (!n.is_read) onMarkRead(n.id);
                  }}
                >
                  <span className="text-base mt-0.5 shrink-0">
                    {TYPE_ICONS[n.type] ?? '🔔'}
                  </span>
                  <div className="flex-1 min-w-0">
                    <p className={`leading-snug ${
                      !n.is_read
                        ? 'font-medium text-gray-800 dark:text-slate-100'
                        : 'text-gray-600 dark:text-slate-400'
                    }`}>
                      {n.body}
                    </p>
                    <p className="text-xs text-gray-400 dark:text-slate-500 mt-1">
                      <RelativeTime date={n.created_at} />
                    </p>
                  </div>
                  {!n.is_read && (
                    <span className="w-2 h-2 rounded-full bg-blue-500 mt-1.5 shrink-0" aria-hidden="true" />
                  )}
                </DropdownMenu.Item>
              ))
            )}
          </div>
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  );
}
