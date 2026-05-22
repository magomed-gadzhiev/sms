interface Alert {
  id: string;
  type: 'critical' | 'warning' | 'info';
  title: string;
  description: string;
  time: string;
}

interface AlertsFeedProps {
  alerts: Alert[];
}

const alertStyles = {
  critical: {
    dot: 'bg-red-500',
    row: 'bg-red-50 dark:bg-red-950/40 border-red-200 dark:border-red-800',
  },
  warning: {
    dot: 'bg-yellow-500',
    row: 'bg-yellow-50 dark:bg-yellow-950/40 border-yellow-200 dark:border-yellow-800',
  },
  info: {
    dot: 'bg-blue-500',
    row: 'bg-blue-50 dark:bg-blue-950/40 border-blue-200 dark:border-blue-800',
  },
} as const;

export function AlertsFeed({ alerts }: AlertsFeedProps) {
  const sorted = [...alerts]
    .sort((a, b) => new Date(b.time).getTime() - new Date(a.time).getTime())
    .slice(0, 10);

  return (
    <div className="bg-white dark:bg-slate-900 border border-gray-200 dark:border-slate-700 rounded-lg p-4">
      <h3 className="text-sm font-medium text-gray-700 dark:text-slate-300 mb-3">Оповещения</h3>
      {sorted.length === 0 ? (
        <p className="text-sm text-gray-400 dark:text-slate-500">Нет активных оповещений</p>
      ) : (
        <div className="space-y-2">
          {sorted.map((alert) => {
            const style = alertStyles[alert.type];
            return (
              <div
                key={alert.id}
                className={`flex items-start gap-3 border rounded-md px-3 py-2 ${style.row}`}
              >
                <span
                  className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${style.dot}`}
                />
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-medium text-gray-900 dark:text-slate-100">
                    {alert.title}
                  </div>
                  <div className="text-xs text-gray-600 dark:text-slate-400">
                    {alert.description}
                  </div>
                </div>
                <span className="shrink-0 text-xs text-gray-400 dark:text-slate-500 whitespace-nowrap">
                  {alert.time}
                </span>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
