import { useEffect, useState } from 'react';
import { Outlet } from 'react-router-dom';
import { Sidebar, type NavItem, type NavGroup } from './Sidebar';
import { SkipLink } from '../SkipLink';
import { useAuth } from '../../contexts/AuthContext';
import { NotificationBell } from '../ui/NotificationBell';
import { CommandPalette } from '../ui/CommandPalette';
import { ModeSwitcher } from './ModeSwitcher';

const NAV_ITEMS: NavItem[] = [
  { path: '/command-center', label: 'Командный центр' },
];

function buildNavGroups(): NavGroup[] {
  return [
    {
      label: 'Отправить',
      items: [
        { path: '/quick-send', label: 'Быстрая отправка' },
        { path: '/campaigns', label: 'Кампании' },
        { path: '/campaign-schedules', label: 'Расписания' },
        { path: '/templates', label: 'Шаблоны' },
        { path: '/sender-names', label: 'Имена отправителей' },
        { path: '/companies', label: 'Мои компании' },
      ],
    },
    {
      label: 'Отследить',
      items: [
        { path: '/messages', label: 'Сообщения' },
        { path: '/cascade/history', label: 'История каскадов' },
      ],
    },
    {
      label: 'Аналитика',
      items: [
        { path: '/analytics', label: 'Статистика' },
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
        { path: '/routing', label: 'Маршрутизация' },
        { path: '/lookup', label: 'Lookup' },
        { path: '/settings/smpp', label: 'SMPP' },
      ],
    },
    {
      label: 'Настройки',
      items: [
        { path: '/profile', label: 'Профиль' },
        { path: '/settings/domains', label: 'Домены' },
        { path: '/settings/notifications', label: 'Уведомления' },
        { path: '/settings/default-senders', label: 'Имена по умолчанию' },
        { path: '/audit-log', label: 'Журнал аудита' },
      ],
    },
  ];
}

export function UserLayout() {
  const { user, logout } = useAuth();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const navGroups = buildNavGroups();
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
        items={NAV_ITEMS}
        groups={navGroups}
        isOpen={isMobileMenuOpen}
        onClose={() => setIsMobileMenuOpen(false)}
        footer={
          <div>
            {!!user?.is_reseller && (
              <div className="mb-3">
                <ModeSwitcher currentMode="own" />
              </div>
            )}
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

      {/* Support chat */}
      <a
        href="https://t.me/sms_support"
        target="_blank"
        rel="noopener noreferrer"
        className="fixed bottom-6 right-6 z-50 w-14 h-14 bg-primary text-white rounded-full shadow-lg flex items-center justify-center hover:opacity-90 transition-opacity"
        aria-label="Поддержка"
        title="Написать в поддержку"
      >
        <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
        </svg>
      </a>
    </div>
  );
}
