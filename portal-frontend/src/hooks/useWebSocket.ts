// portal-frontend/src/hooks/useWebSocket.ts
import { useEffect, useRef, useState, useCallback } from 'react';

export type WsStatus = 'connecting' | 'open' | 'closed' | 'error';

interface UseWebSocketOptions<T> {
  url: string;
  enabled?: boolean;
  /** Max messages to keep in buffer */
  bufferSize?: number;
  onMessage?: (msg: T) => void;
}

interface UseWebSocketResult<T> {
  status: WsStatus;
  messages: T[];
  isPaused: boolean;
  pause: () => void;
  resume: () => void;
  clearBuffer: () => void;
}

const RECONNECT_BASE_MS = 2_000;
const RECONNECT_MAX_MS  = 30_000;

export function useWebSocket<T>({
  url,
  enabled = true,
  bufferSize = 100,
  onMessage,
}: UseWebSocketOptions<T>): UseWebSocketResult<T> {
  const [status, setStatus]   = useState<WsStatus>('closed');
  const [messages, setMessages] = useState<T[]>([]);
  const [isPaused, setIsPaused] = useState(false);

  const wsRef         = useRef<WebSocket | null>(null);
  const pausedRef     = useRef(false);
  const timerRef      = useRef<ReturnType<typeof setTimeout> | null>(null);
  const delayRef      = useRef(RECONNECT_BASE_MS);
  const closedByUs    = useRef(false);
  const onMessageRef  = useRef(onMessage);
  onMessageRef.current = onMessage;

  const clearTimer = () => {
    if (timerRef.current !== null) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  };

  const connect = useCallback(() => {
    if (closedByUs.current) return;
    setStatus('connecting');

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      setStatus('open');
      delayRef.current = RECONNECT_BASE_MS;
    };

    ws.onmessage = (e: MessageEvent) => {
      if (pausedRef.current) return;
      try {
        const msg = JSON.parse(e.data as string) as T;
        onMessageRef.current?.(msg);
        setMessages((prev) => {
          const next = [msg, ...prev];
          return next.length > bufferSize ? next.slice(0, bufferSize) : next;
        });
      } catch {
        // ignore malformed
      }
    };

    ws.onerror = () => setStatus('error');

    ws.onclose = () => {
      wsRef.current = null;
      if (closedByUs.current) return;
      setStatus('closed');
      timerRef.current = setTimeout(() => {
        delayRef.current = Math.min(delayRef.current * 2, RECONNECT_MAX_MS);
        connect();
      }, delayRef.current);
    };
  }, [url, bufferSize]);

  useEffect(() => {
    if (!enabled) return;
    closedByUs.current = false;
    connect();
    return () => {
      closedByUs.current = true;
      clearTimer();
      wsRef.current?.close();
    };
  }, [enabled, connect]);

  const pause  = useCallback(() => { pausedRef.current = true;  setIsPaused(true);  }, []);
  const resume = useCallback(() => { pausedRef.current = false; setIsPaused(false); }, []);
  const clearBuffer = useCallback(() => setMessages([]), []);

  return { status, messages, isPaused, pause, resume, clearBuffer };
}
