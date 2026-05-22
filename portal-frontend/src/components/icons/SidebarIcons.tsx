// Inline SVG icons for sidebar groups (Heroicons 24x24 outline style).
// PR-3 of portal shell overhaul. No external icon dependency.

const baseProps = {
  fill: 'none',
  stroke: 'currentColor',
  viewBox: '0 0 24 24',
  'aria-hidden': true,
  strokeWidth: 1.8,
} as const;

interface IconProps {
  className?: string;
}

export function HomeIcon({ className = 'w-4 h-4' }: IconProps) {
  return (
    <svg {...baseProps} className={className}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M3 12l9-9 9 9M5 10v10a1 1 0 001 1h3v-6h6v6h3a1 1 0 001-1V10" />
    </svg>
  );
}

export function PaperPlaneIcon({ className = 'w-4 h-4' }: IconProps) {
  return (
    <svg {...baseProps} className={className}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M3.4 20.6l17.2-8.6L3.4 3.4l1.2 7.2L15 12l-10.4 1.4-1.2 7.2z" />
    </svg>
  );
}

export function MagnifyingGlassIcon({ className = 'w-4 h-4' }: IconProps) {
  return (
    <svg {...baseProps} className={className}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-4.35-4.35M11 18a7 7 0 100-14 7 7 0 000 14z" />
    </svg>
  );
}

export function ChartBarIcon({ className = 'w-4 h-4' }: IconProps) {
  return (
    <svg {...baseProps} className={className}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M4 20V10m6 10V4m6 16v-7m6 7V8" />
    </svg>
  );
}

export function UsersIcon({ className = 'w-4 h-4' }: IconProps) {
  return (
    <svg {...baseProps} className={className}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z" />
    </svg>
  );
}

export function CreditCardIcon({ className = 'w-4 h-4' }: IconProps) {
  return (
    <svg {...baseProps} className={className}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M3 10h18M3 7a2 2 0 012-2h14a2 2 0 012 2v10a2 2 0 01-2 2H5a2 2 0 01-2-2V7z" />
    </svg>
  );
}

export function CodeBracketIcon({ className = 'w-4 h-4' }: IconProps) {
  return (
    <svg {...baseProps} className={className}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M8 9l-4 3 4 3M16 9l4 3-4 3M14 5l-4 14" />
    </svg>
  );
}

export function CogIcon({ className = 'w-4 h-4' }: IconProps) {
  return (
    <svg {...baseProps} className={className}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M10.3 3.7a1 1 0 011.4 0l1 1a1 1 0 00.7.3h1.4a1 1 0 011 1v1.4a1 1 0 00.3.7l1 1a1 1 0 010 1.4l-1 1a1 1 0 00-.3.7v1.4a1 1 0 01-1 1h-1.4a1 1 0 00-.7.3l-1 1a1 1 0 01-1.4 0l-1-1a1 1 0 00-.7-.3H7.2a1 1 0 01-1-1v-1.4a1 1 0 00-.3-.7l-1-1a1 1 0 010-1.4l1-1a1 1 0 00.3-.7V7a1 1 0 011-1h1.4a1 1 0 00.7-.3l1-1z" />
      <circle cx="12" cy="12" r="3" />
    </svg>
  );
}

export function NetworkIcon({ className = 'w-4 h-4' }: IconProps) {
  return (
    <svg {...baseProps} className={className}>
      <circle cx="12" cy="5" r="2.5" />
      <circle cx="5" cy="19" r="2.5" />
      <circle cx="19" cy="19" r="2.5" />
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 8v3M7 17l3-4M17 17l-3-4" />
    </svg>
  );
}
