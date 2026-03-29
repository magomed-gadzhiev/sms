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
    row: 'bg-red-50 border-red-200',
  },
  warning: {
    dot: 'bg-yellow-500',
    row: 'bg-yellow-50 border-yellow-200',
  },
  info: {
    dot: 'bg-blue-500',
    row: 'bg-blue-50 border-blue-200',
  },
} as const;

export function AlertsFeed({ alerts }: AlertsFeedProps) {
  const sorted = [...alerts]
    .sort((a, b) => new Date(b.time).getTime() - new Date(a.time).getTime())
    .slice(0, 10);

  return (
    <div className="bg-white border border-gray-200 rounded-lg p-4">
      <h3 className="text-sm font-medium text-gray-700 mb-3">Оповещения</h3>
      {sorted.length === 0 ? (
        <p className="text-sm text-gray-400">Нет активных оповещений</p>
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
                  <div className="text-sm font-medium text-gray-900">
                    {alert.title}
                  </div>
                  <div className="text-xs text-gray-600">
                    {alert.description}
                  </div>
                </div>
                <span className="shrink-0 text-xs text-gray-400 whitespace-nowrap">
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
