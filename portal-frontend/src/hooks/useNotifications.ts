import { useCallback, useEffect, useRef, useState } from 'react';
import { notificationsApi, type NotificationItem } from '../api/client';

export function useNotifications(enabled = true) {
  const [items, setItems] = useState<NotificationItem[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetch = useCallback(async () => {
    if (!enabled) return;
    try {
      const res = await notificationsApi.list();
      setItems(res.items ?? []);
      setUnreadCount(res.unread_count ?? 0);
    } catch {
      // silent — don't break the UI
    }
  }, [enabled]);

  useEffect(() => {
    if (!enabled) {
      setItems([]);
      setUnreadCount(0);
      return;
    }
    fetch();

    intervalRef.current = setInterval(fetch, 30_000);

    const onVisibility = () => {
      if (document.visibilityState === 'visible') fetch();
    };
    document.addEventListener('visibilitychange', onVisibility);

    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, [fetch, enabled]);

  const markRead = useCallback(async (id: string) => {
    await notificationsApi.markRead(id);
    setItems((prev) => prev.map((n) => (n.id === id ? { ...n, is_read: true } : n)));
    setUnreadCount((c) => Math.max(0, c - 1));
  }, []);

  const markAllRead = useCallback(async () => {
    await notificationsApi.markAllRead();
    setItems((prev) => prev.map((n) => ({ ...n, is_read: true })));
    setUnreadCount(0);
  }, []);

  return { items, unreadCount, markRead, markAllRead };
}
