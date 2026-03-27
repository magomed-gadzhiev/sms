import { type ReactNode } from 'react';
import { Link, useLocation } from 'react-router-dom';

export interface NavItem {
  path: string;
  label: string;
  icon?: ReactNode;
}

interface SidebarProps {
  title: string;
  items: NavItem[];
  footer?: ReactNode;
  isOpen?: boolean;
  onClose?: () => void;
}

export function Sidebar({ title, items, footer, isOpen, onClose }: SidebarProps) {
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
        aria-label="Navigation sidebar"
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
              aria-label="Close menu"
            >
              ✕
            </button>
          )}
        </div>
        <nav aria-label="Main menu" className="flex-1 py-2 space-y-0.5 px-2">
          <ul>
          {items.map((item) => {
            const isActive = location.pathname.startsWith(item.path);
            return (
              <li key={item.path}>
                <Link
                  to={item.path}
                  aria-current={isActive ? 'page' : undefined}
                  onClick={onClose}
                  className={`flex items-center gap-2 px-3 py-2 rounded text-sm transition-colors
                    ${isActive
                      ? 'bg-primary/10 text-primary font-medium'
                      : 'text-gray-700 hover:bg-gray-100'
                    }`}
                >
                  {item.icon}
                  {item.label}
                </Link>
              </li>
            );
          })}
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
