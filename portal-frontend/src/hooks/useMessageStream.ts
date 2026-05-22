import { useEffect, useRef, useState, useCallback } from 'react';

export type StreamStatus = 'connecting' | 'connected' | 'disconnected' | 'error';

export interface MessageStatusEvent {
  message_id: string;
  status: string;
  updated_at: string;
}

export interface UseMessageStreamResult {
  streamStatus: StreamStatus;
  /** Latest status update received from the stream. */
  lastEvent: MessageStatusEvent | null;
  /** All updates keyed by message_id for easy lookup. */
  updates: Record<string, MessageStatusEvent>;
  /** Manually close the SSE connection. */
  close: () => void;
}

const RECONNECT_DELAY_MS = 3000;
const MAX_RECONNECT_DELAY_MS = 30000;

/**
 * useMessageStream opens an SSE connection to /portal/v1/messages/stream
 * and exposes real-time status updates for the current client's messages.
 *
 * @param enabled - Set to false to disable the stream (e.g. when the tab is hidden).
 */
export function useMessageStream(enabled = true): UseMessageStreamResult {
  const [streamStatus, setStreamStatus] = useState<StreamStatus>('disconnected');
  const [lastEvent, setLastEvent] = useState<MessageStatusEvent | null>(null);
  const [updates, setUpdates] = useState<Record<string, MessageStatusEvent>>({});

  const esRef = useRef<EventSource | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reconnectDelayRef = useRef<number>(RECONNECT_DELAY_MS);
  const closedManuallyRef = useRef<boolean>(false);

  const clearReconnectTimer = () => {
    if (reconnectTimerRef.current !== null) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
  };

  const disconnect = useCallback(() => {
    clearReconnectTimer();
    if (esRef.current) {
      esRef.current.close();
      esRef.current = null;
    }
  }, []);

  const close = useCallback(() => {
    closedManuallyRef.current = true;
    disconnect();
    setStreamStatus('disconnected');
  }, [disconnect]);

  useEffect(() => {
    if (!enabled) {
      disconnect();
      setStreamStatus('disconnected');
      return;
    }

    closedManuallyRef.current = false;

    const connect = () => {
      if (closedManuallyRef.current) return;

      setStreamStatus('connecting');

      const es = new EventSource('/portal/v1/messages/stream', { withCredentials: true });
      esRef.current = es;

      es.addEventListener('connected', () => {
        setStreamStatus('connected');
        reconnectDelayRef.current = RECONNECT_DELAY_MS;
      });

      es.addEventListener('message.status', (e: MessageEvent) => {
        try {
          const event: MessageStatusEvent = JSON.parse(e.data as string);
          setLastEvent(event);
          setUpdates((prev) => ({ ...prev, [event.message_id]: event }));
        } catch {
          // Ignore malformed events.
        }
      });

      es.onerror = () => {
        es.close();
        esRef.current = null;
        setStreamStatus('error');

        if (!closedManuallyRef.current) {
          reconnectTimerRef.current = setTimeout(() => {
            reconnectDelayRef.current = Math.min(
              reconnectDelayRef.current * 2,
              MAX_RECONNECT_DELAY_MS,
            );
            connect();
          }, reconnectDelayRef.current);
        }
      };
    };

    connect();

    return () => {
      closedManuallyRef.current = true;
      disconnect();
    };
  }, [enabled, disconnect]);

  return { streamStatus, lastEvent, updates, close };
}
