// portal-frontend/src/components/layout/NetworkLayout.tsx
import { useState, useEffect } from 'react';
import { Outlet, useLocation } from 'react-router-dom';
import { Sidebar, type NavItem, type NavGroup } from './Sidebar';
import { ModeSwitcher, setResellerMode } from './ModeSwitcher';
import { SkipLink } from '../SkipLink';
import { useAuth } from '../../contexts/AuthContext';
import { NotificationBell } from '../ui/NotificationBell';
import { apiFetch } from '../../api/client';

const NAV_ITEMS: NavItem[] = [
  { path: '/network/sub-accounts', label: 'Суб-аккаунты' },
];

const NAV_GROUPS: NavGroup[] = [
  {
    label: 'Управление',
    items: [
      { path: '/network/moderation', label: 'Модерация' },
    ],
  },
];

export function NetworkLayout() {
  const { user, logout } = useAuth();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [moderationCount, setModerationCount] = useState(0);

  useEffect(() => {
    setResellerMode('network');
  }, []);

  useEffect(() => {
    apiFetch<{ sender_names: number; templates: number; registrations: number }>('/reseller/moderation/counts')
      .then((data) => setModerationCount(data.sender_names + data.templates + data.registrations))
      .catch(() => {});
  }, []);

  // Build nav items with badge for moderation
  const navItemsWithBadge: NavItem[] = [...NAV_ITEMS];
  const navGroupsWithBadge: NavGroup[] = NAV_GROUPS.map((group) => ({
    ...group,
    items: group.items.map((item) => {
      if (item.path === '/network/moderation' && moderationCount > 0) {
        return { ...item, label: `Модерация (${moderationCount})` };
      }
      return item;
    }),
  }));

  return (
    <div className="flex min-h-screen">
      <SkipLink targetId="main-content" />

      {/* Mobile header */}
      <div className="fixed top-0 left-0 right-0 h-14 bg-white border-b border-gray-200 flex items-center px-4 z-30 md:hidden">
        <button
          onClick={() => setIsMobileMenuOpen(true)}
          className="text-gray-600 hover:text-gray-900"
          aria-label="Открыть меню"
        >
          <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>
        <span className="ml-3 font-semibold text-gray-900">Управление сетью</span>
        <div className="ml-auto">
          <NotificationBell />
        </div>
      </div>

      <Sidebar
        title="Управление сетью"
        items={navItemsWithBadge}
        groups={navGroupsWithBadge}
        isOpen={isMobileMenuOpen}
        onClose={() => setIsMobileMenuOpen(false)}
        footer={
          <div className="space-y-3">
            <ModeSwitcher currentMode="network" />
            <div>
              <div className="text-sm text-gray-600 truncate">{user?.email}</div>
              <button onClick={logout} className="text-sm text-gray-500 hover:text-gray-700">
                Выйти
              </button>
            </div>
          </div>
        }
      />
      <main id="main-content" className="flex-1 p-4 md:p-6 bg-gray-50/50 overflow-auto pt-18 md:pt-6">
        <Outlet />
      </main>
    </div>
  );
}
