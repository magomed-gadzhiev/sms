import { useState, useEffect } from 'react';
import { Button } from '../../../components/ui/Button';
import { Modal } from '../../../components/ui/Modal';
import { useToast } from '../../../components/ui/Toast';
import { resellerTariffApi, subAccountsApi, ApiError } from '../../../api/client';

interface SubAccountOption {
  id: string;
  name: string;
}

interface TemplateAssignModalProps {
  open: boolean;
  onClose: () => void;
  templateId: string;
  templateName: string;
  onAssigned: () => void;
}

export function TemplateAssignModal({
  open,
  onClose,
  templateId,
  templateName,
  onAssigned,
}: TemplateAssignModalProps) {
  const toast = useToast();
  const [subAccounts, setSubAccounts] = useState<SubAccountOption[]>([]);
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [assigning, setAssigning] = useState(false);

  useEffect(() => {
    if (!open) return;
    setLoading(true);
    setSelected(new Set());
    subAccountsApi
      .list()
      .then((r: any) => {
        const list = (r.sub_accounts || []).map((sa: any) => ({
          id: sa.id,
          name: sa.name || sa.email,
        }));
        setSubAccounts(list);
      })
      .catch(() => toast.error('Ошибка загрузки субаккаунтов'))
      .finally(() => setLoading(false));
  }, [open]);

  function toggleSA(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }

  async function handleAssign() {
    if (selected.size === 0) return;
    setAssigning(true);
    try {
      await resellerTariffApi.assignTemplate(templateId, [...selected]);
      toast.success(`Привязано субаккаунтов: ${selected.size}`);
      onAssigned();
      onClose();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка привязки');
    } finally {
      setAssigning(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={`Привязать к шаблону: ${templateName}`}
    >
      {loading ? (
        <div className="space-y-2">
          <div className="h-8 bg-gray-100 dark:bg-slate-800 rounded animate-pulse" />
          <div className="h-8 bg-gray-100 dark:bg-slate-800 rounded animate-pulse" />
          <div className="h-8 bg-gray-100 dark:bg-slate-800 rounded animate-pulse" />
        </div>
      ) : subAccounts.length === 0 ? (
        <div className="text-sm text-gray-400 dark:text-slate-500 py-4 text-center">Нет доступных субаккаунтов</div>
      ) : (
        <>
          <div className="max-h-60 overflow-y-auto border border-gray-200 dark:border-slate-700 rounded mb-4">
            {subAccounts.map((sa) => (
              <label
                key={sa.id}
                className="flex items-center gap-3 px-3 py-2 hover:bg-gray-50 dark:hover:bg-slate-800 cursor-pointer border-b border-gray-100 dark:border-slate-800 last:border-b-0"
              >
                <input
                  type="checkbox"
                  checked={selected.has(sa.id)}
                  onChange={() => toggleSA(sa.id)}
                  className="rounded border-gray-300 dark:border-slate-600"
                />
                <span className="text-sm">{sa.name}</span>
              </label>
            ))}
          </div>
          <div className="flex gap-2">
            <Button onClick={handleAssign} disabled={assigning || selected.size === 0}>
              {assigning
                ? 'Привязка...'
                : `Привязать (${selected.size})`}
            </Button>
            <Button variant="secondary" onClick={onClose}>
              Отмена
            </Button>
          </div>
        </>
      )}
    </Modal>
  );
}
