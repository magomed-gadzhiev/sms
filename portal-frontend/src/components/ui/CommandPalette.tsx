import * as Dialog from '@radix-ui/react-dialog';
import { useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import type { CommandItem } from '../../api/client';
import { useCommandPalette } from '../../hooks/useCommandPalette';

const STATIC_ITEMS: CommandItem[] = [
  { id: 'nav-dashboard', type: 'nav', category: 'Навигация', title: 'Дашборд', url: '/dashboard' },
  { id: 'nav-analytics', type: 'nav', category: 'Навигация', title: 'Аналитика', url: '/analytics' },
  { id: 'nav-messages', type: 'nav', category: 'Навигация', title: 'Сообщения', url: '/messages' },
  { id: 'nav-campaigns', type: 'nav', category: 'Навигация', title: 'Рассылки', url: '/campaigns' },
  { id: 'nav-templates', type: 'nav', category: 'Навигация', title: 'Шаблоны', url: '/templates' },
  { id: 'nav-sender-names', type: 'nav', category: 'Навигация', title: 'Имена отправителей', url: '/sender-names' },
  { id: 'nav-contact-lists', type: 'nav', category: 'Навигация', title: 'Контактные базы', url: '/contact-lists' },
  { id: 'nav-billing', type: 'nav', category: 'Навигация', title: 'Биллинг', url: '/billing' },
  { id: 'nav-api-keys', type: 'nav', category: 'Навигация', title: 'API Ключи', url: '/api-keys' },
  { id: 'action-new-campaign', type: 'action', category: 'Действия', title: 'Создать рассылку', url: '/campaigns/new' },
  { id: 'action-new-template', type: 'action', category: 'Действия', title: 'Добавить шаблон', url: '/templates' },
];

function filterStatic(q: string): CommandItem[] {
  if (!q) return STATIC_ITEMS;
  const lower = q.toLowerCase();
  return STATIC_ITEMS.filter(
    (i) => i.title.toLowerCase().includes(lower) || i.category.toLowerCase().includes(lower),
  );
}

interface Props {
  isOpen: boolean;
  onClose: () => void;
}

export function CommandPalette({ isOpen, onClose }: Props) {
  const navigate = useNavigate();
  const { query, setQuery, results, activeIndex, navigate: navKbd } = useCommandPalette();
  const inputRef = useRef<HTMLInputElement>(null);

  const staticMatches = filterStatic(query);
  const allItems: CommandItem[] = [...staticMatches, ...results];

  useEffect(() => {
    if (isOpen) {
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [isOpen]);

  const activate = (item: CommandItem) => {
    navigate(item.url);
    onClose();
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') { e.preventDefault(); navKbd('down', allItems.length || 1); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); navKbd('up', allItems.length || 1); }
    else if (e.key === 'Enter' && allItems[activeIndex]) { activate(allItems[activeIndex]); }
    else if (e.key === 'Escape') onClose();
  };

  // Group items by category
  const grouped = allItems.reduce<Record<string, CommandItem[]>>((acc, item) => {
    (acc[item.category] ??= []).push(item);
    return acc;
  }, {});

  let globalIdx = 0;

  return (
    <Dialog.Root open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 bg-black/40 dark:bg-black/70 z-50" />
        <Dialog.Content
          className="fixed top-[20%] left-1/2 -translate-x-1/2 w-full max-w-lg bg-white dark:bg-slate-900 text-gray-900 dark:text-slate-100 rounded-xl shadow-2xl z-50 overflow-hidden border border-transparent dark:border-slate-700"
          onKeyDown={handleKeyDown}
          aria-label="Палитра команд"
        >
          <Dialog.Title className="sr-only">Поиск и навигация</Dialog.Title>
          <div className="flex items-center gap-3 px-4 py-3 border-b border-gray-200 dark:border-slate-700">
            <svg className="w-4 h-4 text-gray-400 dark:text-slate-500 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            <input
              ref={inputRef}
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Поиск или переход..."
              className="flex-1 outline-none text-sm bg-transparent text-gray-900 dark:text-slate-100 placeholder:text-gray-400 dark:placeholder:text-slate-500"
              aria-autocomplete="list"
              aria-controls="command-palette-results"
            />
            <kbd className="text-xs text-gray-400 dark:text-slate-400 bg-gray-100 dark:bg-slate-800 px-1.5 py-0.5 rounded">Esc</kbd>
          </div>

          <div id="command-palette-results" className="max-h-72 overflow-y-auto py-1" role="listbox">
            {allItems.length === 0 && query.length >= 2 && (
              <div className="px-4 py-6 text-sm text-gray-500 dark:text-slate-400 text-center">Ничего не найдено</div>
            )}
            {Object.entries(grouped).map(([category, items]) => (
              <div key={category}>
                <div className="px-4 py-1.5 text-xs font-medium text-gray-500 dark:text-slate-400 uppercase tracking-wider">
                  {category}
                </div>
                {items.map((item) => {
                  const idx = globalIdx++;
                  return (
                    <button
                      key={item.id}
                      role="option"
                      aria-selected={activeIndex === idx}
                      className={`w-full flex items-center gap-3 px-4 py-2.5 text-sm text-left transition-colors ${
                        activeIndex === idx
                          ? 'bg-blue-50 dark:bg-blue-950/40 text-blue-700 dark:text-blue-300'
                          : 'text-gray-800 dark:text-slate-200 hover:bg-gray-50 dark:hover:bg-slate-800'
                      }`}
                      onClick={() => activate(item)}
                    >
                      <span className="flex-1">{item.title}</span>
                      {item.subtitle && (
                        <span className="text-xs text-gray-400 dark:text-slate-500 truncate max-w-[120px]">{item.subtitle}</span>
                      )}
                    </button>
                  );
                })}
              </div>
            ))}
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
