import { useEffect, useState } from 'react';
import { Outlet } from 'react-router-dom';
import { Sidebar, type NavItem, type NavGroup } from './Sidebar';
import { SkipLink } from '../SkipLink';
import { useAuth } from '../../contexts/AuthContext';
import { NotificationBell } from '../ui/NotificationBell';
import { CommandPalette } from '../ui/CommandPalette';

const DASHBOARD_NAV: NavItem[] = [
  { path: '/dashboard', label: 'Дашборд' },
  { path: '/analytics', label: 'Аналитика' },
];

const NAV_GROUPS: NavGroup[] = [
  {
    label: 'Рассылки',
    items: [
      { path: '/messages', label: 'Сообщения' },
      { path: '/campaigns', label: 'Рассылки' },
      { path: '/templates', label: 'Шаблоны' },
      { path: '/sender-names', label: 'Имена отправителей' },
    ],
  },
  {
    label: 'Контакты',
    items: [
      { path: '/contact-lists', label: 'Контактные базы' },
      { path: '/segments', label: 'Сегменты' },
      { path: '/opt-out', label: 'Список отписок' },
    ],
  },
  {
    label: 'Финансы',
    items: [
      { path: '/billing', label: 'Биллинг' },
      { path: '/tariffs', label: 'Тарифы' },
    ],
  },
  {
    label: 'Интеграции',
    items: [
      { path: '/api-keys', label: 'API Ключи' },
      { path: '/webhooks', label: 'Вебхуки' },
      { path: '/providers', label: 'Провайдеры' },
      { path: '/lookup', label: 'Lookup' },
    ],
  },
  {
    label: 'Настройки',
    items: [
      { path: '/profile', label: 'Профиль' },
      { path: '/sub-accounts', label: 'Суб-аккаунты' },
      { path: '/settings/domains', label: 'Домены' },
      { path: '/audit-log', label: 'Журнал аудита' },
    ],
  },
];

export function UserLayout() {
  const { user, logout } = useAuth();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [isPaletteOpen, setIsPaletteOpen] = useState(false);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
        e.preventDefault();
        setIsPaletteOpen(true);
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, []);

  return (
    <div className="flex min-h-screen">
      <SkipLink targetId="main-content" />

      {/* Mobile header with hamburger */}
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
        <span className="ml-3 font-semibold text-gray-900">SMS Portal</span>
        <div className="ml-auto flex items-center gap-2">
          <button
            onClick={() => setIsPaletteOpen(true)}
            className="text-gray-400 hover:text-gray-600 p-1"
            aria-label="Поиск (Ctrl+K)"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
          </button>
          <NotificationBell />
        </div>
      </div>

      <Sidebar
        title="SMS Portal"
        items={DASHBOARD_NAV}
        groups={NAV_GROUPS}
        isOpen={isMobileMenuOpen}
        onClose={() => setIsMobileMenuOpen(false)}
        footer={
          <div>
            <div className="flex items-center justify-between mb-2">
              <div className="text-sm text-gray-600 truncate">{user?.email}</div>
              <div className="hidden md:flex items-center gap-1">
                <button
                  onClick={() => setIsPaletteOpen(true)}
                  className="text-gray-400 hover:text-gray-600 p-1 rounded"
                  aria-label="Поиск (Ctrl+K)"
                  title="Ctrl+K"
                >
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                  </svg>
                </button>
                <NotificationBell />
              </div>
            </div>
            <button onClick={logout} className="text-sm text-gray-500 hover:text-gray-700">
              Выйти
            </button>
          </div>
        }
      />
      <main id="main-content" className="flex-1 p-4 md:p-6 bg-gray-50/50 overflow-auto pt-18 md:pt-6">
        <Outlet />
      </main>

      <CommandPalette isOpen={isPaletteOpen} onClose={() => setIsPaletteOpen(false)} />
    </div>
  );
}
