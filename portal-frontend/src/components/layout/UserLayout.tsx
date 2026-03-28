import { useState } from 'react';
import { Outlet } from 'react-router-dom';
import { Sidebar, type NavItem } from './Sidebar';
import { SkipLink } from '../SkipLink';
import { useAuth } from '../../contexts/AuthContext';

const USER_NAV: NavItem[] = [
  { path: '/dashboard', label: 'Дашборд' },
  { path: '/messages', label: 'Сообщения' },
  { path: '/templates', label: 'Шаблоны' },
  { path: '/contact-lists', label: 'Контакты' },
  { path: '/segments', label: 'Сегменты' },
  { path: '/campaigns', label: 'Рассылки' },
  { path: '/lookup', label: 'Lookup' },
  { path: '/providers', label: 'Провайдеры' },
  { path: '/billing', label: 'Биллинг' },
  { path: '/tariffs', label: 'Тарифы' },
  { path: '/api-keys', label: 'API Ключи' },
  { path: '/webhooks', label: 'Вебхуки' },
  { path: '/analytics', label: 'Аналитика' },
  { path: '/sub-accounts', label: 'Суб-аккаунты' },
  { path: '/profile', label: 'Профиль' },
  { path: '/audit-log', label: 'Журнал аудита' },
];

export function UserLayout() {
  const { user, logout } = useAuth();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);

  return (
    <div className="flex min-h-screen">
      <SkipLink targetId="main-content" />

      {/* Mobile header with hamburger */}
      <div className="fixed top-0 left-0 right-0 h-14 bg-white border-b border-gray-200 flex items-center px-4 z-30 md:hidden">
        <button
          onClick={() => setIsMobileMenuOpen(true)}
          className="text-gray-600 hover:text-gray-900"
          aria-label="Open menu"
        >
          <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>
        <span className="ml-3 font-semibold text-gray-900">SMS Portal</span>
      </div>

      <Sidebar
        title="SMS Portal"
        items={USER_NAV}
        isOpen={isMobileMenuOpen}
        onClose={() => setIsMobileMenuOpen(false)}
        footer={
          <div>
            <div className="text-sm text-gray-600 truncate mb-2">{user?.email}</div>
            <button
              onClick={logout}
              className="text-sm text-gray-500 hover:text-gray-700"
            >
              Выйти
            </button>
          </div>
        }
      />
      <main id="main-content" className="flex-1 p-4 md:p-6 bg-gray-50/50 overflow-auto pt-18 md:pt-6">
        <Outlet />
      </main>
    </div>
  );
}
