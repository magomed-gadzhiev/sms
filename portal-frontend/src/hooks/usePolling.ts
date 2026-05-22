import { useEffect, useRef, useState, useCallback } from 'react';

export function usePolling(fn: () => Promise<void>, intervalMs: number) {
  const [isPaused, setIsPaused] = useState(false);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const fnRef = useRef(fn);
  fnRef.current = fn;

  const execute = useCallback(async () => {
    await fnRef.current();
    setLastUpdated(new Date());
  }, []);

  useEffect(() => {
    execute();
  }, [execute]);

  useEffect(() => {
    if (isPaused) {
      if (intervalRef.current) clearInterval(intervalRef.current);
      return;
    }
    intervalRef.current = setInterval(execute, intervalMs);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [isPaused, intervalMs, execute]);

  const pause = useCallback(() => setIsPaused(true), []);
  const resume = useCallback(() => setIsPaused(false), []);

  return { pause, resume, isPaused, lastUpdated };
}
