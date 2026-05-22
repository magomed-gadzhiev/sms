import { type ReactNode, useEffect, useRef, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';

export interface NavItem {
  path: string;
  label: string;
  icon?: ReactNode;
}

export interface NavGroup {
  label: string;
  items: NavItem[];
  accent?: 'network';
  icon?: ReactNode;
}

interface SidebarProps {
  title: string;
  items?: NavItem[];
  groups?: NavGroup[];
  footer?: ReactNode;
  isOpen?: boolean;
  onClose?: () => void;
  /** When true, sidebar shows only group icons; clicking opens a flyout popover.
   *  Driven by media query in UserLayout: md range (768-1023). */
  collapsed?: boolean;
}

function NavLink({ item, isActive, onClose, iconOnly }: { item: NavItem; isActive: boolean; onClose?: () => void; iconOnly?: boolean }) {
  return (
    <li>
      <Link
        to={item.path}
        aria-current={isActive ? 'page' : undefined}
        aria-label={iconOnly ? item.label : undefined}
        title={iconOnly ? item.label : undefined}
        onClick={onClose}
        className={`flex items-center gap-2 px-3 py-2 rounded text-sm transition-colors
          ${isActive
            ? 'bg-primary text-white font-semibold'
            : 'text-gray-700 dark:text-slate-300 hover:bg-gray-100 dark:hover:bg-slate-800'
          }
          ${iconOnly ? 'justify-center' : ''}`}
      >
        {item.icon}
        {!iconOnly && item.label}
      </Link>
    </li>
  );
}

function ExpandedGroup({ group, location, onClose }: { group: NavGroup; location: ReturnType<typeof useLocation>; onClose?: () => void }) {
  const hasActiveChild = group.items.some((item) => location.pathname.startsWith(item.path));
  const [isExpanded, setIsExpanded] = useState(hasActiveChild);
  const isAccent = group.accent === 'network';

  useEffect(() => {
    if (hasActiveChild) setIsExpanded(true);
  }, [hasActiveChild]);

  return (
    <li className={isAccent ? 'mt-3 pt-3 border-t border-gray-200 dark:border-slate-700 bg-indigo-50/40 dark:bg-indigo-950/30 -mx-2 px-2 rounded-md' : undefined}>
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        aria-expanded={isExpanded}
        className={`flex items-center justify-between w-full px-3 py-2 text-xs font-semibold uppercase tracking-wider transition-colors ${
          isAccent
            ? 'text-indigo-700 dark:text-indigo-300 hover:text-indigo-900 dark:hover:text-indigo-100'
            : 'text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-200'
        }`}
      >
        <span className="flex items-center gap-2">
          {group.icon}
          {group.label}
        </span>
        <svg className={`w-3.5 h-3.5 transition-transform ${isExpanded ? 'rotate-90' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
        </svg>
      </button>
      {isExpanded && (
        <ul className="mt-0.5 space-y-0.5 pb-1">
          {group.items.map((item) => (
            <NavLink key={item.path} item={item} isActive={location.pathname.startsWith(item.path)} onClose={onClose} />
          ))}
        </ul>
      )}
    </li>
  );
}

function CollapsedGroup({ group, location, onClose }: { group: NavGroup; location: ReturnType<typeof useLocation>; onClose?: () => void }) {
  const hasActiveChild = group.items.some((item) => location.pathname.startsWith(item.path));
  const [isFlyoutOpen, setIsFlyoutOpen] = useState(false);
  const wrapperRef = useRef<HTMLLIElement>(null);
  const isAccent = group.accent === 'network';

  // Close flyout on outside click.
  useEffect(() => {
    if (!isFlyoutOpen) return;
    const handler = (e: MouseEvent) => {
      if (!wrapperRef.current?.contains(e.target as Node)) setIsFlyoutOpen(false);
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [isFlyoutOpen]);

  // Close flyout on navigation.
  useEffect(() => {
    setIsFlyoutOpen(false);
  }, [location.pathname]);

  return (
    <li ref={wrapperRef} className="relative">
      <button
        onClick={() => setIsFlyoutOpen((v) => !v)}
        aria-expanded={isFlyoutOpen}
        aria-haspopup="menu"
        aria-label={group.label}
        title={group.label}
        className={`flex items-center justify-center w-full p-2 rounded transition-colors ${
          hasActiveChild
            ? 'bg-primary/10 text-primary'
            : isAccent
            ? 'text-indigo-700 dark:text-indigo-300 hover:bg-indigo-50 dark:hover:bg-indigo-950/40'
            : 'text-gray-600 dark:text-slate-300 hover:bg-gray-100 dark:hover:bg-slate-800'
        }`}
      >
        {group.icon ?? <span className="text-xs font-semibold">{group.label[0]}</span>}
      </button>
      {isFlyoutOpen && (
        <div
          role="menu"
          aria-label={group.label}
          className="absolute left-full top-0 ml-1 z-50 w-56 bg-white dark:bg-slate-900 border border-gray-200 dark:border-slate-700 rounded-md shadow-lg py-1"
        >
          <div className={`px-3 py-1 text-[10px] font-semibold uppercase tracking-wider ${isAccent ? 'text-indigo-700 dark:text-indigo-300' : 'text-gray-500 dark:text-slate-400'}`}>
            {group.label}
          </div>
          <ul>
            {group.items.map((item) => (
              <li key={item.path}>
                <Link
                  to={item.path}
                  onClick={() => {
                    setIsFlyoutOpen(false);
                    onClose?.();
                  }}
                  aria-current={location.pathname.startsWith(item.path) ? 'page' : undefined}
                  className={`block px-3 py-2 text-sm transition-colors ${
                    location.pathname.startsWith(item.path)
                      ? 'bg-primary text-white font-semibold'
                      : 'text-gray-700 dark:text-slate-300 hover:bg-gray-100 dark:hover:bg-slate-800'
                  }`}
                  role="menuitem"
                >
                  {item.label}
                </Link>
              </li>
            ))}
          </ul>
        </div>
      )}
    </li>
  );
}

export function Sidebar({ title, items, groups, footer, isOpen, onClose, collapsed = false }: SidebarProps) {
  const location = useLocation();
  // Mobile drawer (isOpen=true) always uses full width regardless of collapsed.
  // Desktop: w-16 in collapsed (md range), w-56 expanded (lg+).
  const widthClass = isOpen ? 'w-56' : collapsed ? 'w-16' : 'w-56';
  // Effective collapsed state for rendering: never collapsed when drawer is open on mobile.
  const renderCollapsed = collapsed && !isOpen;

  return (
    <>
      {isOpen && (
        <div className="fixed inset-0 bg-black/40 z-40 md:hidden" onClick={onClose} aria-hidden="true" />
      )}

      <aside
        aria-label="Боковая навигация"
        className={`
          ${widthClass} border-r border-gray-200 dark:border-slate-700 bg-gray-50 dark:bg-slate-900 flex flex-col min-h-screen
          fixed z-50 top-0 left-0 transition-transform duration-200 ease-in-out
          md:static md:translate-x-0 md:z-auto
          ${isOpen ? 'translate-x-0' : '-translate-x-full'}
        `}
      >
        <div className={`p-4 border-b border-gray-200 dark:border-slate-700 flex items-center ${renderCollapsed ? 'justify-center' : 'justify-between'}`}>
          {!renderCollapsed && <h2 className="text-base font-semibold text-gray-900 dark:text-slate-100">{title}</h2>}
          {renderCollapsed && <span className="text-base font-semibold text-gray-900 dark:text-slate-100" title={title}>S</span>}
          {onClose && (
            <button className="md:hidden text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-200" onClick={onClose} aria-label="Закрыть меню">
              ✕
            </button>
          )}
        </div>
        <nav
          aria-label="Главное меню"
          className={`flex-1 py-2 px-2 ${renderCollapsed ? 'overflow-visible' : 'overflow-y-auto'}`}
        >
          <ul className="space-y-1">
            {items?.map((item) => (
              <NavLink
                key={item.path}
                item={item}
                isActive={location.pathname.startsWith(item.path)}
                onClose={onClose}
                iconOnly={renderCollapsed}
              />
            ))}
            {groups?.map((group) =>
              renderCollapsed ? (
                <CollapsedGroup key={group.label} group={group} location={location} onClose={onClose} />
              ) : (
                <ExpandedGroup key={group.label} group={group} location={location} onClose={onClose} />
              ),
            )}
          </ul>
        </nav>
        {footer && (
          <div className={`border-t border-gray-200 dark:border-slate-700 ${renderCollapsed ? 'p-2' : 'p-4'}`}>{footer}</div>
        )}
      </aside>
    </>
  );
}
