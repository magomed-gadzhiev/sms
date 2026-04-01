import { type ReactNode, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';

export interface NavItem {
  path: string;
  label: string;
  icon?: ReactNode;
}

export interface NavGroup {
  label: string;
  items: NavItem[];
}

interface SidebarProps {
  title: string;
  items?: NavItem[];
  groups?: NavGroup[];
  footer?: ReactNode;
  isOpen?: boolean;
  onClose?: () => void;
}

function NavLink({ item, isActive, onClose }: { item: NavItem; isActive: boolean; onClose?: () => void }) {
  return (
    <li>
      <Link
        to={item.path}
        aria-current={isActive ? 'page' : undefined}
        onClick={onClose}
        className={`flex items-center gap-2 px-3 py-2 rounded text-sm transition-colors
          ${isActive
            ? 'bg-primary text-white font-semibold'
            : 'text-gray-700 hover:bg-gray-100'
          }`}
      >
        {item.icon}
        {item.label}
      </Link>
    </li>
  );
}

function CollapsibleGroup({ group, location, onClose }: { group: NavGroup; location: ReturnType<typeof useLocation>; onClose?: () => void }) {
  const hasActiveChild = group.items.some((item) => location.pathname.startsWith(item.path));
  const [isExpanded, setIsExpanded] = useState(hasActiveChild);

  return (
    <li>
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        aria-expanded={isExpanded}
        className="flex items-center justify-between w-full px-3 py-2 text-xs font-semibold uppercase tracking-wider text-gray-500 hover:text-gray-700 transition-colors"
      >
        <span>{group.label}</span>
        <svg
          className={`w-3.5 h-3.5 transition-transform ${isExpanded ? 'rotate-90' : ''}`}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          aria-hidden="true"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
        </svg>
      </button>
      {isExpanded && (
        <ul className="mt-0.5 space-y-0.5">
          {group.items.map((item) => (
            <NavLink
              key={item.path}
              item={item}
              isActive={location.pathname.startsWith(item.path)}
              onClose={onClose}
            />
          ))}
        </ul>
      )}
    </li>
  );
}

export function Sidebar({ title, items, groups, footer, isOpen, onClose }: SidebarProps) {
  const location = useLocation();

  return (
    <>
      {/* Mobile backdrop */}
      {isOpen && (
        <div
          className="fixed inset-0 bg-black/40 z-40 md:hidden"
          onClick={onClose}
          aria-hidden="true"
        />
      )}

      <aside
        aria-label="Боковая навигация"
        className={`
          w-56 border-r border-gray-200 bg-gray-50 flex flex-col min-h-screen
          fixed z-50 top-0 left-0 transition-transform duration-200 ease-in-out
          md:static md:translate-x-0 md:z-auto
          ${isOpen ? 'translate-x-0' : '-translate-x-full'}
        `}
      >
        <div className="p-4 border-b border-gray-200 flex items-center justify-between">
          <h2 className="text-base font-semibold text-gray-900">{title}</h2>
          {onClose && (
            <button
              className="md:hidden text-gray-500 hover:text-gray-700"
              onClick={onClose}
              aria-label="Закрыть меню"
            >
              ✕
            </button>
          )}
        </div>
        <nav aria-label="Главное меню" className="flex-1 py-2 px-2 overflow-y-auto">
          <ul className="space-y-1">
            {/* Flat items (backward compatible) */}
            {items?.map((item) => (
              <NavLink
                key={item.path}
                item={item}
                isActive={location.pathname.startsWith(item.path)}
                onClose={onClose}
              />
            ))}
            {/* Grouped items */}
            {groups?.map((group) => (
              <CollapsibleGroup
                key={group.label}
                group={group}
                location={location}
                onClose={onClose}
              />
            ))}
          </ul>
        </nav>
        {footer && (
          <div className="p-4 border-t border-gray-200">
            {footer}
          </div>
        )}
      </aside>
    </>
  );
}
