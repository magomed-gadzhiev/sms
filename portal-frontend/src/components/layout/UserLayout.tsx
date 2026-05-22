import { useEffect, useState } from 'react';
import { Outlet, useLocation } from 'react-router-dom';
import { Sidebar, type NavItem, type NavGroup } from './Sidebar';
import { SkipLink } from '../SkipLink';
import { useAuth } from '../../contexts/AuthContext';
import { PageTitleProvider } from '../../contexts/PageTitleContext';
import { CommandPalette } from '../ui/CommandPalette';
import { AppHeader } from './AppHeader';
import { apiFetch } from '../../api/client';
import {
  HomeIcon,
  PaperPlaneIcon,
  MagnifyingGlassIcon,
  ChartBarIcon,
  UsersIcon,
  CreditCardIcon,
  CodeBracketIcon,
  CogIcon,
  NetworkIcon,
} from '../icons/SidebarIcons';

const HOME_ITEMS: NavItem[] = [
  { path: '/command-center', label: 'Командный центр', icon: <HomeIcon /> },
];

function buildOwnNavGroups(isSubAccount: boolean): NavGroup[] {
  const integrations: NavItem[] = [
    { path: '/api-keys', label: 'API Ключи' },
    { path: '/webhooks', label: 'Вебхуки' },
    { path: '/lookup', label: 'Lookup' },
  ];
  if (!isSubAccount) {
    integrations.splice(2, 0, { path: '/providers', label: 'Подключения' });
    integrations.splice(3, 0, { path: '/routing', label: 'Маршрутизация' });
  }
  return [
    {
      label: 'Отправить',
      icon: <PaperPlaneIcon />,
      items: [
        { path: '/quick-send', label: 'Быстрая отправка' },
        { path: '/campaigns', label: 'Кампании' },
        { path: '/templates', label: 'Шаблоны' },
        { path: '/sender-names', label: 'Имена отправителей' },
        { path: '/companies', label: 'Мои компании' },
      ],
    },
    {
      label: 'Отследить',
      icon: <MagnifyingGlassIcon />,
      items: [
        { path: '/messages', label: 'Сообщения' },
        { path: '/cascade/history', label: 'История каскадов' },
      ],
    },
    {
      label: 'Аналитика',
      icon: <ChartBarIcon />,
      items: [
        { path: '/analytics', label: 'Статистика' },
      ],
    },
    {
      label: 'Контакты',
      icon: <UsersIcon />,
      items: [
        { path: '/contact-lists', label: 'Контактные базы' },
        { path: '/segments', label: 'Сегменты' },
        { path: '/opt-out', label: 'Список отписок' },
      ],
    },
    {
      label: 'Финансы',
      icon: <CreditCardIcon />,
      items: [
        { path: '/billing', label: 'Биллинг' },
        { path: '/tariffs', label: 'Тарифы' },
      ],
    },
    {
      label: 'Интеграции',
      icon: <CodeBracketIcon />,
      items: integrations,
    },
    {
      label: 'Настройки',
      icon: <CogIcon />,
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

function buildNetworkGroup(moderationCount: number): NavGroup {
  return {
    label: 'Сеть',
    accent: 'network',
    icon: <NetworkIcon />,
    items: [
      { path: '/network/dashboard', label: 'Дашборд сети' },
      { path: '/network/sub-accounts', label: 'Суб-аккаунты' },
      {
        path: '/network/moderation',
        label: moderationCount > 0 ? `Модерация (${moderationCount})` : 'Модерация',
      },
      { path: '/network/providers', label: 'Провайдеры' },
      { path: '/network/provider-sets', label: 'Provider-sets' },
      { path: '/network/route-sets', label: 'Route-sets' },
      { path: '/network/assignments', label: 'Назначения' },
      { path: '/network/tariffs', label: 'Тарифы сети' },
      { path: '/network/statistics', label: 'Статистика сети' },
      { path: '/network/audit-log', label: 'История изменений' },
    ],
  };
}

export function UserLayout() {
  const { user } = useAuth();
  const location = useLocation();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [isPaletteOpen, setIsPaletteOpen] = useState(false);
  const [moderationCount, setModerationCount] = useState(0);

  // Sidebar auto-collapses on md range (768-1023px). User can override after mount.
  const [isSidebarCollapsed, setIsSidebarCollapsed] = useState(() => {
    if (typeof window === 'undefined') return false;
    return window.matchMedia('(min-width: 768px) and (max-width: 1023px)').matches;
  });
  useEffect(() => {
    const mq = window.matchMedia('(min-width: 768px) and (max-width: 1023px)');
    const handler = (e: MediaQueryListEvent) => setIsSidebarCollapsed(e.matches);
    mq.addEventListener('change', handler);
    return () => mq.removeEventListener('change', handler);
  }, []);

  const isSubAccount = !!user?.parent_client_id;
  const isReseller = !!user?.is_reseller;

  useEffect(() => {
    if (!isReseller) return;
    apiFetch<{ sender_names: number; templates: number; registrations: number }>('/reseller/moderation/counts')
      .then((data) => setModerationCount(data.sender_names + data.templates + data.registrations))
      .catch(() => {});
  }, [isReseller, location.pathname]);

  const navGroups: NavGroup[] = [
    ...buildOwnNavGroups(isSubAccount),
    ...(isReseller ? [buildNetworkGroup(moderationCount)] : []),
  ];

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
    <PageTitleProvider>
      <div className="flex min-h-screen">
        <SkipLink targetId="main-content" />

        <AppHeader
          onOpenMobileMenu={() => setIsMobileMenuOpen(true)}
          onOpenSearch={() => setIsPaletteOpen(true)}
          sidebarCollapsed={isSidebarCollapsed}
        />

        <Sidebar
          title="SMS Portal"
          items={HOME_ITEMS}
          groups={navGroups}
          isOpen={isMobileMenuOpen}
          onClose={() => setIsMobileMenuOpen(false)}
          collapsed={isSidebarCollapsed}
        />
        <main id="main-content" className="flex-1 p-4 md:p-6 bg-gray-50/50 dark:bg-slate-950 overflow-auto pt-18 md:pt-20 text-gray-900 dark:text-slate-100">
          <Outlet />
        </main>

        <CommandPalette isOpen={isPaletteOpen} onClose={() => setIsPaletteOpen(false)} />

        {/* Support chat */}
        <a
          href="https://t.me/sms_support"
          target="_blank"
          rel="noopener noreferrer"
          className="fixed bottom-22 md:bottom-6 right-6 z-50 w-14 h-14 bg-primary text-white rounded-full shadow-lg flex items-center justify-center hover:opacity-90 transition-opacity"
          aria-label="Поддержка"
          title="Написать в поддержку"
        >
          <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
          </svg>
        </a>
      </div>
    </PageTitleProvider>
  );
}
