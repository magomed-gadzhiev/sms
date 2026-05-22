import { useState, useEffect } from 'react';
import { Modal } from '../../../components/ui/Modal';
import { Button } from '../../../components/ui/Button';
import { useToast } from '../../../components/ui/Toast';
import type { SubAccountTariffSummary, ResellerTemplate } from '../../../api/client';
import { resellerTariffApi, ApiError } from '../../../api/client';

interface Props {
  subAccount: SubAccountTariffSummary;
  onClose: () => void;
  onAssigned: () => void;
}

export function PickTemplateDialog({ subAccount, onClose, onAssigned }: Props) {
  const [templates, setTemplates] = useState<ResellerTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<string | null>(subAccount.template_id);
  const [assigning, setAssigning] = useState(false);
  const toast = useToast();

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    resellerTariffApi
      .listTemplates()
      .then((res) => {
        if (!cancelled) {
          setTemplates(res.templates);
        }
      })
      .catch((e) => {
        if (!cancelled) {
          setError(e instanceof ApiError ? e.message : 'Не удалось загрузить шаблоны');
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const handleAssign = async () => {
    if (!selected) return;
    setAssigning(true);
    try {
      await resellerTariffApi.assignTemplate(selected, [subAccount.sub_account_id]);
      toast.success('Шаблон привязан');
      onAssigned();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка при привязке шаблона');
    } finally {
      setAssigning(false);
    }
  };

  return (
    <Modal open title="Выбрать шаблон тарифов" onClose={onClose}>
      {loading && (
        <div className="space-y-2">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-12 bg-slate-100 dark:bg-slate-800 rounded animate-pulse" />
          ))}
        </div>
      )}

      {error && !loading && (
        <div
          role="alert"
          className="rounded border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-950/40 text-red-800 dark:text-red-300 px-4 py-3 text-sm mb-4"
        >
          {error}
        </div>
      )}

      {!loading && !error && templates.length === 0 && (
        <p className="text-center text-slate-500 dark:text-slate-400 py-8">
          Нет доступных шаблонов. Создайте новый шаблон на странице шаблонов.
        </p>
      )}

      {!loading && !error && templates.length > 0 && (
        <div className="space-y-3 mb-6">
          {templates.map((tpl) => (
            <label
              key={tpl.id}
              className="flex items-start gap-3 p-3 border border-slate-200 dark:border-slate-700 rounded-lg cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors"
            >
              <input
                type="radio"
                name="template"
                value={tpl.id}
                checked={selected === tpl.id}
                onChange={(e) => setSelected(e.target.value)}
                className="mt-1"
              />
              <div className="flex-1 min-w-0">
                <div className="font-medium text-slate-900 dark:text-slate-100">
                  {tpl.name}
                </div>
                {tpl.description && (
                  <div className="text-xs text-slate-500 dark:text-slate-400 mt-1 line-clamp-2">
                    {tpl.description}
                  </div>
                )}
                <div className="text-xs text-slate-500 dark:text-slate-400 mt-2">
                  Привязано к {tpl.assigned_count} субаккаунт{tpl.assigned_count === 1 ? 'у' : tpl.assigned_count > 1 && tpl.assigned_count < 5 ? 'ам' : 'ам'}
                </div>
              </div>
            </label>
          ))}
        </div>
      )}

      {!loading && !error && (
        <div className="flex justify-end gap-3">
          <Button variant="secondary" onClick={onClose} disabled={assigning}>
            Отмена
          </Button>
          <Button
            onClick={handleAssign}
            disabled={assigning || !selected}
          >
            {assigning ? 'Привязка...' : 'Привязать'}
          </Button>
        </div>
      )}
    </Modal>
  );
}
