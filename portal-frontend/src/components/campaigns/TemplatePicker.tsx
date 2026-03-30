import { useState, useEffect, useRef } from 'react';
import { templatesApi, type TemplateInfo } from '../../api/client';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';

interface TemplatePickerProps {
  value: string; // template id
  selectedTemplate: TemplateInfo | null;
  onChange: (id: string, template: TemplateInfo | null) => void;
}

export function TemplatePicker({ value, selectedTemplate, onChange }: TemplatePickerProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [templates, setTemplates] = useState<TemplateInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const searchRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!open) return;
    setLoading(true);
    const params: Record<string, string> = { status: 'approved', per_page: '100' };
    if (search.trim()) params.search = search.trim();
    templatesApi
      .list(params)
      .then((res) => setTemplates(res.templates ?? []))
      .catch(() => setTemplates([]))
      .finally(() => setLoading(false));
  }, [open, search]);

  useEffect(() => {
    if (open) {
      setTimeout(() => searchRef.current?.focus(), 50);
    }
  }, [open]);

  function handleSelect(t: TemplateInfo) {
    onChange(t.id, t);
    setOpen(false);
    setSearch('');
  }

  function handleClear(e: React.MouseEvent) {
    e.stopPropagation();
    onChange('', null);
  }

  return (
    <>
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">Шаблон</label>
        <div
          role="button"
          tabIndex={0}
          onClick={() => setOpen(true)}
          onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && setOpen(true)}
          className="flex items-center justify-between rounded border border-gray-300 px-3 py-2 text-sm cursor-pointer hover:border-blue-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400 bg-white min-h-[40px]"
          aria-label="Выбрать шаблон"
        >
          {selectedTemplate ? (
            <div className="flex-1 min-w-0">
              <span className="font-medium text-gray-900 truncate block">{selectedTemplate.name}</span>
              <span className="text-xs text-gray-500 truncate block">{selectedTemplate.body.slice(0, 80)}{selectedTemplate.body.length > 80 ? '…' : ''}</span>
            </div>
          ) : (
            <span className="text-gray-400">Выберите шаблон из библиотеки</span>
          )}
          <div className="flex items-center gap-2 ml-2 shrink-0">
            {value && (
              <button
                type="button"
                onClick={handleClear}
                className="text-gray-400 hover:text-gray-600 text-xs"
                aria-label="Очистить выбор"
              >
                ✕
              </button>
            )}
            <svg className="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 9l4-4 4 4m0 6l-4 4-4-4" />
            </svg>
          </div>
        </div>
        <p className="text-xs text-gray-500 mt-1">Показываются только одобренные шаблоны</p>
      </div>

      <Modal open={open} onClose={() => { setOpen(false); setSearch(''); }} title="Выбор шаблона" wide>
        <div className="space-y-3">
          <input
            ref={searchRef}
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Поиск по названию или тексту..."
            className="w-full rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
          />

          {loading ? (
            <div className="py-8 text-center text-sm text-gray-500">Загрузка...</div>
          ) : templates.length === 0 ? (
            <div className="py-8 text-center text-sm text-gray-500">
              {search ? 'Ничего не найдено' : 'Нет одобренных шаблонов'}
            </div>
          ) : (
            <ul className="divide-y divide-gray-100 max-h-[50vh] overflow-y-auto -mx-6 px-6">
              {templates.map((t) => (
                <li key={t.id}>
                  <button
                    type="button"
                    onClick={() => handleSelect(t)}
                    className={`w-full text-left px-3 py-3 rounded hover:bg-blue-50 transition-colors ${t.id === value ? 'bg-blue-50 ring-1 ring-blue-300' : ''}`}
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0">
                        <p className="text-sm font-medium text-gray-900 truncate">{t.name}</p>
                        <p className="text-xs text-gray-500 mt-0.5 line-clamp-2">{t.body}</p>
                        {t.variables.length > 0 && (
                          <p className="text-xs text-blue-600 mt-1">
                            Переменные: {t.variables.join(', ')}
                          </p>
                        )}
                      </div>
                      {t.id === value && (
                        <span className="shrink-0 text-blue-600 text-xs font-medium">✓ Выбран</span>
                      )}
                    </div>
                  </button>
                </li>
              ))}
            </ul>
          )}

          <div className="flex justify-end pt-2 border-t border-gray-100">
            <Button variant="secondary" onClick={() => { setOpen(false); setSearch(''); }}>
              Отмена
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
