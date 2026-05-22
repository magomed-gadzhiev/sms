import { useState, useRef, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { MoreVertical } from 'lucide-react';
import type { SubAccountTariffSummary } from '../../../api/client';
import { resellerTariffApi, ApiError } from '../../../api/client';
import { useToast } from '../../../components/ui/Toast';
import { ConfirmDialog } from '../../../components/ui/ConfirmDialog';
import { PickTemplateDialog } from './PickTemplateDialog';

interface Props {
  row: SubAccountTariffSummary;
  onChanged: () => void;
}

export function SubAccountTariffRowMenu({ row, onChanged }: Props) {
  const [open, setOpen] = useState(false);
  const [pickOpen, setPickOpen] = useState(false);
  const [unbindConfirm, setUnbindConfirm] = useState(false);
  const [unbinding, setUnbinding] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const nav = useNavigate();
  const toast = useToast();

  useEffect(() => {
    if (!open) return;
    const onDocClick = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    const onEsc = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onDocClick);
    document.addEventListener('keydown', onEsc);
    return () => {
      document.removeEventListener('mousedown', onDocClick);
      document.removeEventListener('keydown', onEsc);
    };
  }, [open]);

  const handleUnbind = async () => {
    if (!row.template_id) return;
    setUnbinding(true);
    try {
      await resellerTariffApi.unassignTemplate(row.template_id, row.sub_account_id);
      toast.success('Шаблон отвязан');
      onChanged();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка при отвязке шаблона');
    } finally {
      setUnbinding(false);
      setUnbindConfirm(false);
    }
  };

  return (
    <div ref={ref} className="relative" data-no-row-click>
      <button
        type="button"
        onClick={(e) => {
          e.stopPropagation();
          setOpen((v) => !v);
        }}
        aria-label={`Действия с субаккаунтом ${row.sub_account_name}`}
        aria-haspopup="menu"
        aria-expanded={open}
        className="p-1.5 rounded hover:bg-slate-100 dark:hover:bg-slate-800 focus:outline-none focus:ring-2 focus:ring-sky-500"
      >
        <MoreVertical className="w-4 h-4 text-slate-500 dark:text-slate-400" aria-hidden />
      </button>
      {open && (
        <div
          role="menu"
          className="absolute right-0 top-full mt-1 z-20 min-w-[220px] bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-lg shadow-lg py-1 text-sm"
        >
          <button
            role="menuitem"
            className="w-full text-left px-3 py-1.5 hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors"
            onClick={() => {
              setOpen(false);
              nav(`/network/tariffs/editor/${row.sub_account_id}?mode=override`);
            }}
          >
            Редактировать тарифы
          </button>
          <button
            role="menuitem"
            className="w-full text-left px-3 py-1.5 hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors"
            onClick={() => {
              setOpen(false);
              setPickOpen(true);
            }}
          >
            {row.template_id ? 'Сменить шаблон' : 'Привязать шаблон'}
          </button>
          {row.template_id && (
            <button
              role="menuitem"
              className="w-full text-left px-3 py-1.5 hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors text-red-600 dark:text-red-400"
              onClick={() => {
                setOpen(false);
                setUnbindConfirm(true);
              }}
            >
              Отвязать шаблон
            </button>
          )}
        </div>
      )}
      {pickOpen && (
        <PickTemplateDialog
          subAccount={row}
          onClose={() => setPickOpen(false)}
          onAssigned={() => {
            setPickOpen(false);
            onChanged();
          }}
        />
      )}
      {unbindConfirm && (
        <ConfirmDialog
          open={unbindConfirm}
          title="Отвязать шаблон?"
          description={`Шаблон «${row.template_name}» будет отвязан от субаккаунта «${row.sub_account_name}». Переопределения останутся активны.`}
          confirmLabel="Отвязать"
          variant="danger"
          loading={unbinding}
          onConfirm={handleUnbind}
          onCancel={() => setUnbindConfirm(false)}
        />
      )}
    </div>
  );
}
