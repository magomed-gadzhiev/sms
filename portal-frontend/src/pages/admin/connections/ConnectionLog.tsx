import React, { useEffect, useRef, useState } from 'react';

interface LogLine {
  timestamp: string;
  level: string;
  message: string;
}

interface ConnectionLogProps {
  connectionId: string;
}

// Set to true ONLY if GET /admin/v1/connections/{id}/logs endpoint exists in router
const LOG_ENDPOINT_EXISTS = false;

export const ConnectionLog: React.FC<ConnectionLogProps> = ({ connectionId }) => {
  const [lines, setLines] = useState<LogLine[]>([]);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!LOG_ENDPOINT_EXISTS) return;
    let cancelled = false;

    async function fetchLogs() {
      try {
        const res = await fetch(`/admin/v1/connections/${connectionId}/logs`);
        if (!res.ok) return;
        const data: LogLine[] = await res.json();
        if (!cancelled) setLines(data.slice(-100));
      } catch { /* ignore */ }
    }

    fetchLogs();
    const interval = setInterval(fetchLogs, 5000);
    return () => { cancelled = true; clearInterval(interval); };
  }, [connectionId]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [lines]);

  if (!LOG_ENDPOINT_EXISTS) {
    return (
      <div className="bg-gray-900 rounded-lg p-4 text-gray-400 text-sm font-mono h-48 flex items-center justify-center">
        Логи недоступны — endpoint не реализован
      </div>
    );
  }

  return (
    <div className="bg-gray-900 rounded-lg p-3 h-48 overflow-y-auto font-mono text-xs">
      {lines.length === 0 ? (
        <span className="text-gray-500">Нет логов</span>
      ) : (
        lines.map((line, i) => (
          <div key={`${line.timestamp}-${i}`} className="leading-5">
            <span className="text-gray-500">[{line.timestamp}]</span>{' '}
            <span className={
              line.level === 'error' ? 'text-red-400' :
              line.level === 'warn'  ? 'text-yellow-400' :
              'text-green-400'
            }>{line.level}:</span>{' '}
            <span className="text-gray-200">{line.message}</span>
          </div>
        ))
      )}
      <div ref={bottomRef} />
    </div>
  );
};
