import { useEffect, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import * as DropdownMenu from '@radix-ui/react-dropdown-menu';
import { useAuth } from '../../contexts/AuthContext';
import { usePageTitleValue } from '../../contexts/PageTitleContext';
import { useTheme, type ThemeMode } from '../../contexts/ThemeContext';
import { NotificationBell } from '../ui/NotificationBell';
import { billingApi, type ProfileData } from '../../api/client';

interface AppHeaderProps {
  onOpenMobileMenu: () => void;
  onOpenSearch: () => void;
  /** When true, sidebar is in icon-rail (64px) mode; header offset shrinks. */
  sidebarCollapsed?: boolean;
}

// AppHeader — global sticky header on all breakpoints.
// Page title + search + balance pill + notification bell + user popover.

function getInitials(email: string, companyName: string): string {
  const parts = companyName.trim().split(/\s+/).filter(Boolean);
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase();
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  const local = email.split('@')[0] || email;
  return local.slice(0, 2).toUpperCase();
}

const THEME_LABELS: Record<ThemeMode, string> = {
  system: 'Системная',
  light: 'Светлая',
  dark: 'Тёмная',
};

function UserPopover({ user, onLogout }: { user: ProfileData; onLogout: () => void }) {
  const initials = getInitials(user.email, user.company_name);
  const { mode, setMode } = useTheme();
  const itemBase =
    'block px-3 py-2 text-sm text-gray-700 dark:text-slate-200 hover:bg-gray-50 dark:hover:bg-slate-800 outline-none cursor-pointer';
  const subTriggerClass =
    'flex items-center justify-between px-3 py-2 text-sm text-gray-700 dark:text-slate-200 hover:bg-gray-50 dark:hover:bg-slate-800 outline-none cursor-pointer';
  const radioItemClass =
    'relative flex items-center pl-7 pr-3 py-2 text-sm text-gray-700 dark:text-slate-200 hover:bg-gray-50 dark:hover:bg-slate-800 outline-none cursor-pointer';
  const surfaceClass =
    'z-50 bg-white dark:bg-slate-900 rounded-lg shadow-lg border border-gray-200 dark:border-slate-700 overflow-hidden py-1';
  return (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger asChild>
        <button
          type="button"
          className="w-8 h-8 rounded-full bg-primary text-white text-xs font-semibold flex items-center justify-center hover:opacity-90 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
          aria-label={`Аккаунт: ${user.email}`}
        >
          {initials}
        </button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content
          align="end"
          sideOffset={8}
          className={`${surfaceClass} min-w-[220px]`}
        >
          <div className="px-3 py-2 border-b border-gray-100 dark:border-slate-800">
            <div className="text-sm font-medium text-gray-900 dark:text-slate-100 truncate" title={user.email}>
              {user.email}
            </div>
            {user.company_name && (
              <div className="text-xs text-gray-500 dark:text-slate-400 truncate" title={user.company_name}>
                {user.company_name}
              </div>
            )}
          </div>
          <DropdownMenu.Item asChild>
            <Link to="/profile" className={itemBase}>
              Профиль
            </Link>
          </DropdownMenu.Item>
          <DropdownMenu.Sub>
            <DropdownMenu.SubTrigger className={subTriggerClass}>
              <span>Тема</span>
              <span className="text-gray-400 dark:text-slate-500 text-xs">{THEME_LABELS[mode]} ▸</span>
            </DropdownMenu.SubTrigger>
            <DropdownMenu.Portal>
              <DropdownMenu.SubContent
                sideOffset={4}
                className={`${surfaceClass} min-w-[160px]`}
              >
                <DropdownMenu.RadioGroup
                  value={mode}
                  onValueChange={(v) => setMode(v as ThemeMode)}
                >
                  {(['system', 'light', 'dark'] as const).map((m) => (
                    <DropdownMenu.RadioItem key={m} value={m} className={radioItemClass}>
                      <DropdownMenu.ItemIndicator className="absolute left-2 text-primary">
                        ✓
                      </DropdownMenu.ItemIndicator>
                      {THEME_LABELS[m]}
                    </DropdownMenu.RadioItem>
                  ))}
                </DropdownMenu.RadioGroup>
              </DropdownMenu.SubContent>
            </DropdownMenu.Portal>
          </DropdownMenu.Sub>
          <div className="border-t border-gray-100 dark:border-slate-800" />
          <DropdownMenu.Item onSelect={onLogout} className={itemBase}>
            Выйти
          </DropdownMenu.Item>
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  );
}

function formatBalance(balance: string, currency: string): string {
  const n = parseFloat(balance);
  if (isNaN(n)) return `— ${currency}`;
  return `${n.toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ${currency}`;
}

function routeTitleFallback(pathname: string): string {
  const map: Record<string, string> = {
    '/command-center': 'Командный центр',
    '/quick-send': 'Быстрая отправка',
    '/campaigns': 'Кампании',
    '/templates': 'Шаблоны',
    '/sender-names': 'Имена отправителей',
    '/companies': 'Мои компании',
    '/messages': 'Сообщения',
    '/cascade/history': 'История каскадов',
    '/analytics': 'Аналитика',
    '/contact-lists': 'Контактные базы',
    '/segments': 'Сегменты',
    '/opt-out': 'Список отписок',
    '/billing': 'Биллинг',
    '/tariffs': 'Тарифы',
    '/api-keys': 'API Ключи',
    '/webhooks': 'Вебхуки',
    '/providers': 'Подключения',
    '/routing': 'Маршрутизация',
    '/lookup': 'Lookup',
    '/profile': 'Профиль',
    '/audit-log': 'Журнал аудита',
    '/notifications': 'Уведомления',
    '/network/dashboard': 'Дашборд сети',
    '/network/sub-accounts': 'Суб-аккаунты',
    '/network/moderation': 'Модерация',
    '/network/providers': 'Провайдеры',
    '/network/provider-sets': 'Provider-sets',
    '/network/route-sets': 'Route-sets',
    '/network/assignments': 'Назначения',
    '/network/tariffs': 'Тарифы сети',
    '/network/statistics': 'Статистика сети',
    '/network/audit-log': 'История изменений',
  };
  // Try exact, then longest-prefix.
  if (map[pathname]) return map[pathname];
  const prefixes = Object.keys(map).sort((a, b) => b.length - a.length);
  for (const p of prefixes) {
    if (pathname.startsWith(p + '/') || pathname === p) return map[p];
  }
  return '';
}

export function AppHeader({ onOpenMobileMenu, onOpenSearch, sidebarCollapsed = false }: AppHeaderProps) {
  const { isAuthenticated, user, logout } = useAuth();
  const location = useLocation();
  const titleFromContext = usePageTitleValue();
  const title = titleFromContext || routeTitleFallback(location.pathname);

  const [balance, setBalance] = useState<{ balance: string; currency: string } | null>(null);

  useEffect(() => {
    if (!isAuthenticated) {
      setBalance(null);
      return;
    }
    let cancelled = false;
    billingApi
      .getBalance()
      .then((b) => {
        if (!cancelled) setBalance({ balance: b.balance, currency: b.currency });
      })
      .catch(() => {
        if (!cancelled) setBalance(null);
      });
    return () => {
      cancelled = true;
    };
  }, [isAuthenticated]);

  return (
    <header
      className={`fixed top-0 left-0 right-0 h-14 bg-white dark:bg-slate-900 border-b border-gray-200 dark:border-slate-700 flex items-center px-4 z-30 ${
        sidebarCollapsed ? 'md:left-16' : 'md:left-56'
      }`}
      role="banner"
    >
      {/* Mobile burger */}
      <button
        onClick={onOpenMobileMenu}
        className="md:hidden text-gray-600 dark:text-slate-300 hover:text-gray-900 dark:hover:text-slate-100 mr-3"
        aria-label="Открыть меню"
      >
        <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
        </svg>
      </button>

      {/* Page title */}
      <h1 className="text-base md:text-lg font-semibold text-gray-900 dark:text-slate-100 truncate">{title}</h1>

      <div className="ml-auto flex items-center gap-1 md:gap-2">
        {/* Search trigger (Ctrl+K) */}
        <button
          onClick={onOpenSearch}
          className="flex items-center gap-1.5 text-gray-500 dark:text-slate-400 hover:text-gray-800 dark:hover:text-slate-200 hover:bg-gray-100 dark:hover:bg-slate-800 rounded px-2 py-1 transition-colors"
          aria-label="Поиск (Ctrl+K)"
          title="Поиск (Ctrl+K)"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <kbd className="hidden md:inline text-[11px] font-mono text-gray-400 dark:text-slate-500 border border-gray-200 dark:border-slate-700 rounded px-1">Ctrl+K</kbd>
        </button>

        {/* Balance pill */}
        {balance && (
          <Link
            to="/billing"
            className="hidden sm:flex items-center gap-1 text-sm px-2.5 py-1 rounded-md border border-gray-200 dark:border-slate-700 text-gray-700 dark:text-slate-200 hover:border-gray-400 dark:hover:border-slate-500 hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors"
            title="Перейти к биллингу"
          >
            <span className="font-medium tabular-nums">{formatBalance(balance.balance, balance.currency)}</span>
            <span className="text-gray-400 dark:text-slate-500 text-xs" aria-hidden="true">→</span>
          </Link>
        )}

        {/* Bell */}
        <NotificationBell />

        {/* User popover */}
        {user && <UserPopover user={user} onLogout={() => { void logout(); }} />}
      </div>
    </header>
  );
}
