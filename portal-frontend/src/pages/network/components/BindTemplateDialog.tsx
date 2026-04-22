import { useEffect, useMemo, useState } from 'react';
import {
  ApiError,
  networkTariffsApi,
  type SubAccountTariffSummary,
} from '../../../api/client';
import { Button } from '../../../components/ui/Button';
import { Modal } from '../../../components/ui/Modal';

interface BindTemplateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  template: { id: string; name: string } | null;
  onSuccess?: () => void;
}

export function BindTemplateDialog({
  open,
  onOpenChange,
  template,
  onSuccess,
}: BindTemplateDialogProps) {
  const [subs, setSubs] = useState<SubAccountTariffSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [search, setSearch] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) {
      setSelected(new Set());
      setSearch('');
      setSubmitError(null);
      setSubmitting(false);
      return;
    }
    setLoading(true);
    setLoadError(null);
    networkTariffsApi
      .listSubaccounts()
      .then((d) => setSubs(d))
      .catch((e: unknown) =>
        setLoadError(e instanceof Error ? e.message : String(e)),
      )
      .finally(() => setLoading(false));
  }, [open]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return subs;
    return subs.filter(
      (s) =>
        s.sub_account_name.toLowerCase().includes(q) ||
        s.sub_account_email.toLowerCase().includes(q),
    );
  }, [subs, search]);

  const hasReplacements = useMemo(() => {
    if (!template) return false;
    for (const id of selected) {
      const sa = subs.find((s) => s.sub_account_id === id);
      if (sa && sa.template_id && sa.template_id !== template.id) return true;
    }
    return false;
  }, [selected, subs, template]);

  if (!template) return null;

  function toggle(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function handleSubmit() {
    if (selected.size === 0) return;
    setSubmitting(true);
    setSubmitError(null);
    try {
      await networkTariffsApi.bindTemplate(template!.id, [...selected]);
      onOpenChange(false);
      onSuccess?.();
    } catch (err) {
      setSubmitError(
        err instanceof ApiError
          ? err.message || 'Ошибка привязки'
          : 'Ошибка привязки',
      );
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={() => onOpenChange(false)}
      title={`Привязать шаблон: ${template.name}`}
      wide
    >
      <div className="space-y-3">
        <input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Поиск по имени или email…"
          className="w-full rounded border border-slate-300 px-3 py-2 text-sm focus:border-sky-500 focus:outline-none"
        />
        {loading ? (
          <div className="space-y-2" aria-busy="true">
            <div className="h-10 bg-slate-100 rounded animate-pulse" />
            <div className="h-10 bg-slate-100 rounded animate-pulse" />
            <div className="h-10 bg-slate-100 rounded animate-pulse" />
          </div>
        ) : loadError ? (
          <div
            role="alert"
            className="rounded border border-red-200 bg-red-50 text-red-800 px-3 py-2 text-sm"
          >
            Не удалось загрузить субаккаунты: {loadError}
          </div>
        ) : filtered.length === 0 ? (
          <div className="text-sm text-slate-500 py-6 text-center">
            {subs.length === 0
              ? 'Нет доступных субаккаунтов'
              : 'Ничего не найдено'}
          </div>
        ) : (
          <div className="max-h-80 overflow-y-auto border border-slate-200 rounded">
            {filtered.map((sa) => {
              const checked = selected.has(sa.sub_account_id);
              const willReplace =
                sa.template_id && sa.template_id !== template.id;
              return (
                <label
                  key={sa.sub_account_id}
                  className="flex items-start gap-3 px-3 py-2 hover:bg-slate-50 cursor-pointer border-b border-slate-100 last:border-b-0"
                >
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => toggle(sa.sub_account_id)}
                    className="mt-0.5 rounded border-slate-300"
                  />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="text-sm font-medium text-slate-900 truncate">
                        {sa.sub_account_name}
                      </span>
                      {willReplace && (
                        <span className="inline-flex items-center rounded bg-amber-100 text-amber-800 text-xs px-1.5 py-0.5">
                          Заменит {sa.template_name}
                        </span>
                      )}
                    </div>
                    <div className="text-xs text-slate-500 truncate">
                      {sa.sub_account_email}
                    </div>
                  </div>
                </label>
              );
            })}
          </div>
        )}

        {hasReplacements && (
          <div
            role="note"
            className="rounded border border-amber-200 bg-amber-50 text-amber-900 px-3 py-2 text-xs"
          >
            Переопределения субаккаунтов сохранятся.
          </div>
        )}

        {submitError && (
          <div
            role="alert"
            className="rounded border border-red-200 bg-red-50 text-red-800 px-3 py-2 text-sm"
          >
            {submitError}
          </div>
        )}

        <div className="flex items-center justify-between pt-2">
          <div className="text-sm text-slate-600">
            Выбрано: {selected.size}
          </div>
          <div className="flex gap-2">
            <Button
              type="button"
              variant="secondary"
              onClick={() => onOpenChange(false)}
              disabled={submitting}
            >
              Отмена
            </Button>
            <Button
              type="button"
              onClick={handleSubmit}
              disabled={submitting || selected.size === 0}
            >
              {submitting ? 'Привязка…' : 'Привязать'}
            </Button>
          </div>
        </div>
      </div>
    </Modal>
  );
}
