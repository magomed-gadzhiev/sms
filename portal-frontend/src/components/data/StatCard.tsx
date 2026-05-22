interface StatCardProps {
  title: string;
  value: string | number;
  subtitle?: string;
  trend?: { value: number; direction: 'up' | 'down' };
}

export function StatCard({ title, value, subtitle, trend }: StatCardProps) {
  return (
    <div className="bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-slate-700 p-5">
      <div className="text-sm font-medium text-gray-500 dark:text-slate-400 mb-1">{title}</div>
      <div className="text-2xl font-semibold text-gray-900 dark:text-slate-100">{value}</div>
      {(subtitle || trend) && (
        <div className="mt-1 text-sm">
          {trend && (
            <span className={trend.direction === 'up' ? 'text-success' : 'text-danger'}>
              {trend.direction === 'up' ? '↑' : '↓'} {trend.value}%
            </span>
          )}
          {subtitle && <span className="text-gray-500 dark:text-slate-400"> {subtitle}</span>}
        </div>
      )}
    </div>
  );
}
