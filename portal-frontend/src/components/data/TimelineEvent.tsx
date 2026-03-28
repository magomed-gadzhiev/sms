interface TimelineEventProps {
  icon: string;
  title: string;
  description?: string;
  author?: string;
  date: string;
  color?: 'gray' | 'green' | 'red' | 'yellow' | 'blue';
}

const colorMap = {
  gray: 'bg-gray-200 text-gray-600',
  green: 'bg-green-100 text-green-700',
  red: 'bg-red-100 text-red-700',
  yellow: 'bg-yellow-100 text-yellow-700',
  blue: 'bg-blue-100 text-blue-700',
};

export function TimelineEvent({ icon, title, description, author, date, color = 'gray' }: TimelineEventProps) {
  return (
    <div className="flex gap-3 pb-4 last:pb-0">
      <div className="flex flex-col items-center">
        <div className={`w-8 h-8 rounded-full flex items-center justify-center text-sm ${colorMap[color]}`}>
          {icon}
        </div>
        <div className="w-px flex-1 bg-gray-200 mt-1" />
      </div>
      <div className="flex-1 pt-1">
        <div className="text-sm font-medium text-gray-900">{title}</div>
        {description && <div className="text-sm text-gray-600 mt-0.5">{description}</div>}
        <div className="text-xs text-gray-400 mt-1">
          {author && <span>{author} &middot; </span>}
          {new Date(date).toLocaleString()}
        </div>
      </div>
    </div>
  );
}
