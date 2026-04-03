import { useState } from 'react';
import { NavLink } from 'react-router-dom';
import { useAuth } from '../../contexts/AuthContext';

interface SidebarItem {
  path: string;
  label: string;
  icon: string;
  resource: string;
}

interface SidebarGroup {
  title: string | null;
  items: SidebarItem[];
}

const SIDEBAR_GROUPS: SidebarGroup[] = [
  {
    title: null,
    items: [
      { path: '/admin', label: 'Дашборд', icon: '\u{1F4CA}', resource: 'analytics' },
    ],
  },
  {
    title: '\u041E\u0441\u043D\u043E\u0432\u043D\u043E\u0435',
    items: [
      { path: '/admin/clients', label: '\u041A\u043B\u0438\u0435\u043D\u0442\u044B', icon: '\u{1F465}', resource: 'clients' },
      { path: '/admin/templates', label: '\u0428\u0430\u0431\u043B\u043E\u043D\u044B', icon: '\u{1F4DD}', resource: 'templates' },
      { path: '/admin/sender-names', label: '\u0418\u043C\u0435\u043D\u0430 \u043E\u0442\u043F\u0440\u0430\u0432\u0438\u0442\u0435\u043B\u0435\u0439', icon: '\u{1F4F1}', resource: 'templates' },
      { path: '/admin/users', label: '\u041F\u043E\u043B\u044C\u0437\u043E\u0432\u0430\u0442\u0435\u043B\u0438', icon: '\u{1F511}', resource: 'users' },
    ],
  },
  {
    title: '\u0424\u0438\u043D\u0430\u043D\u0441\u044B',
    items: [
      { path: '/admin/billing', label: '\u0411\u0438\u043B\u043B\u0438\u043D\u0433', icon: '\u{1F4B0}', resource: 'billing' },
      { path: '/admin/tarification', label: '\u0422\u0430\u0440\u0438\u0444\u0438\u043A\u0430\u0446\u0438\u044F', icon: '\u{1F4CB}', resource: 'tarification' },
    ],
  },
  {
    title: '\u0418\u043D\u0444\u0440\u0430\u0441\u0442\u0440\u0443\u043A\u0442\u0443\u0440\u0430',
    items: [
      { path: '/admin/providers', label: '\u041F\u0440\u043E\u0432\u0430\u0439\u0434\u0435\u0440\u044B', icon: '\u{1F50C}', resource: 'providers' },
      { path: '/admin/routes', label: '\u041C\u0430\u0440\u0448\u0440\u0443\u0442\u044B', icon: '\u{1F500}', resource: 'routes' },
      { path: '/admin/channels', label: '\u041A\u0430\u043D\u0430\u043B\u044B', icon: '\u{1F4E6}', resource: 'providers' },
      { path: '/admin/delivery-strategies', label: '\u0421\u0442\u0440\u0430\u0442\u0435\u0433\u0438\u0438 \u0434\u043E\u0441\u0442\u0430\u0432\u043A\u0438', icon: '\u{1F69A}', resource: 'routes' },
      { path: '/admin/hlr', label: 'HLR', icon: '\u{1F4E1}', resource: 'hlr' },
    ],
  },
  {
    title: '\u0421\u043F\u0440\u0430\u0432\u043E\u0447\u043D\u0438\u043A\u0438',
    items: [
      { path: '/admin/countries', label: '\u0421\u0442\u0440\u0430\u043D\u044B', icon: '\u{1F30D}', resource: 'countries' },
      { path: '/admin/webhooks', label: '\u0412\u0435\u0431\u0445\u0443\u043A\u0438', icon: '\u{1F517}', resource: 'webhooks' },
    ],
  },
  {
    title: '\u041D\u0430\u0431\u043B\u044E\u0434\u0435\u043D\u0438\u0435',
    items: [
      { path: '/admin/analytics', label: '\u0410\u043D\u0430\u043B\u0438\u0442\u0438\u043A\u0430', icon: '\u{1F4C8}', resource: 'analytics' },
      { path: '/admin/monitoring', label: '\u041C\u043E\u043D\u0438\u0442\u043E\u0440\u0438\u043D\u0433', icon: '\u26A1', resource: 'analytics' },
      { path: '/admin/audit', label: '\u0410\u0443\u0434\u0438\u0442', icon: '\u{1F4DC}', resource: 'audit' },
    ],
  },
];

export function AdminSidebar() {
  const [hovered, setHovered] = useState(false);
  const { user, hasPermission, logout } = useAuth();

  return (
    <aside
      className={`fixed top-0 left-0 h-screen bg-gray-900 text-gray-300 flex flex-col transition-all duration-200 z-40 ${
        hovered ? 'w-[220px]' : 'w-14'
      }`}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      <div className="flex items-center h-14 px-3 border-b border-gray-800 shrink-0">
        <span className="text-lg font-bold text-white whitespace-nowrap overflow-hidden">
          {hovered ? 'SMS Admin' : 'S'}
        </span>
      </div>

      <nav className="flex-1 overflow-y-auto py-2">
        {SIDEBAR_GROUPS.map((group, gi) => {
          const visibleItems = group.items.filter((item) => hasPermission(item.resource, 'read'));
          if (visibleItems.length === 0) return null;

          return (
            <div key={gi}>
              {group.title && hovered && (
                <div className="px-3 pt-4 pb-1 text-[10px] font-semibold uppercase tracking-wider text-gray-500">
                  {group.title}
                </div>
              )}
              {!group.title && gi > 0 && <div className="my-1 border-t border-gray-800" />}
              {visibleItems.map((item) => (
                <NavLink
                  key={item.path}
                  to={item.path}
                  end={item.path === '/admin'}
                  title={!hovered ? item.label : undefined}
                  className={({ isActive }) =>
                    `flex items-center gap-3 px-3 py-2 text-sm transition-colors whitespace-nowrap ${
                      isActive
                        ? 'bg-primary/20 text-white border-l-2 border-primary'
                        : 'hover:bg-gray-800 hover:text-white border-l-2 border-transparent'
                    }`
                  }
                >
                  <span className="text-base w-5 text-center shrink-0">{item.icon}</span>
                  {hovered && <span className="truncate">{item.label}</span>}
                </NavLink>
              ))}
            </div>
          );
        })}
      </nav>

      <div className="border-t border-gray-800 p-3 shrink-0">
        {hovered ? (
          <div>
            <div className="text-xs text-gray-400 truncate mb-2">{user?.email}</div>
            <button
              onClick={logout}
              className="text-xs text-gray-500 hover:text-white transition-colors"
            >
              Выход
            </button>
          </div>
        ) : (
          <button
            onClick={logout}
            title="Выход"
            className="text-gray-500 hover:text-white transition-colors text-base w-5 text-center"
          >
            &crarr;
          </button>
        )}
      </div>
    </aside>
  );
}
