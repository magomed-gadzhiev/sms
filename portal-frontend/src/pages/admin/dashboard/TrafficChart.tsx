import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts';

interface TrafficDataPoint {
  time: string;
  delivered: number;
  failed: number;
}

interface TrafficChartProps {
  data: TrafficDataPoint[];
}

export function TrafficChart({ data }: TrafficChartProps) {
  return (
    <div className="bg-white dark:bg-slate-900 border border-gray-200 dark:border-slate-700 rounded-lg p-4 mb-6">
      <h3 className="text-sm font-medium text-gray-700 dark:text-slate-300 mb-3">
        Трафик сообщений (последний час)
      </h3>
      <ResponsiveContainer width="100%" height={300}>
        <BarChart data={data}>
          <XAxis dataKey="time" tick={{ fontSize: 12 }} />
          <YAxis tick={{ fontSize: 12 }} />
          <Tooltip />
          <Legend />
          <Bar dataKey="delivered" fill="#7c3aed" name="Доставлено" />
          <Bar dataKey="failed" fill="#dc2626" name="Ошибки" />
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
