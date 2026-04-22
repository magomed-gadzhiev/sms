export type Channel = 'sms' | 'voice' | 'viber';

interface ChannelTabsProps {
  value: Channel;
  onChange: (c: Channel) => void;
  counts?: Partial<Record<Channel, number>>;
}

const CHANNELS: { id: Channel; label: string }[] = [
  { id: 'sms',   label: 'SMS'   },
  { id: 'voice', label: 'Voice' },
  { id: 'viber', label: 'Viber' },
];

export function ChannelTabs({ value, onChange, counts }: ChannelTabsProps) {
  return (
    <div
      role="tablist"
      aria-label="Каналы"
      className="flex border-b border-gray-200"
    >
      {CHANNELS.map(({ id, label }) => {
        const active = id === value;
        const count = counts?.[id];
        return (
          <button
            key={id}
            role="tab"
            id={`channel-tab-${id}`}
            aria-selected={active}
            aria-controls={`channel-panel-${id}`}
            onClick={() => onChange(id)}
            className={[
              'inline-flex items-center gap-1.5 px-4 py-2.5 text-sm font-medium',
              'border-b-2 -mb-px transition-colors',
              'focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50',
              active
                ? 'border-primary text-primary'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300',
            ].join(' ')}
          >
            {label}
            {count !== undefined && (
              <span
                className={[
                  'inline-flex items-center justify-center rounded-full px-2 py-0.5 text-xs font-medium',
                  active
                    ? 'bg-blue-100 text-primary'
                    : 'bg-gray-100 text-gray-600',
                ].join(' ')}
              >
                {count}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
