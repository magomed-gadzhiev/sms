import { useState, useEffect, type FormEvent } from 'react';
import { Modal } from '../../../components/ui/Modal';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { useToast } from '../../../components/ui/Toast';
import { subAccountsApi, ApiError } from '../../../api/client';

interface EditLimitsDialogProps {
  open: boolean;
  subAccountId: string;
  subAccountName: string;
  currentDaily: number;
  currentMonthly: number;
  onClose: () => void;
  onSuccess: () => void;
}

export function EditLimitsDialog({
  open, subAccountId, subAccountName, currentDaily, currentMonthly, onClose, onSuccess,
}: EditLimitsDialogProps) {
  const toast = useToast();
  const [daily, setDaily] = useState('');
  const [monthly, setMonthly] = useState('');
  const [dailyUnlimited, setDailyUnlimited] = useState(false);
  const [monthlyUnlimited, setMonthlyUnlimited] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!open) return;
    setDaily(currentDaily === 0 ? '' : String(currentDaily));
    setMonthly(currentMonthly === 0 ? '' : String(currentMonthly));
    setDailyUnlimited(currentDaily === 0);
    setMonthlyUnlimited(currentMonthly === 0);
    setError('');
  }, [open, currentDaily, currentMonthly]);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const dailyVal = dailyUnlimited ? 0 : Number(daily || 0);
    const monthlyVal = monthlyUnlimited ? 0 : Number(monthly || 0);
    if ((!dailyUnlimited && daily.trim() === '') || (!monthlyUnlimited && monthly.trim() === '')) {
      setError('Введите значение лимита или включите «Без лимита»');
      return;
    }
    if (dailyVal < 0 || monthlyVal < 0) {
      setError('Лимиты не могут быть отрицательными');
      return;
    }
    setSubmitting(true);
    setError('');
    try {
      await subAccountsApi.updateLimits(subAccountId, {
        daily_limit: dailyVal,
        monthly_limit: monthlyVal,
      });
      toast.success(`Лимиты для ${subAccountName} обновлены`);
      onSuccess();
      onClose();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось обновить лимиты');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={`Лимиты — ${subAccountName}`}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div>
          <Input
            label="Дневной лимит (msg)"
            type="number"
            min="0"
            value={daily}
            onChange={(e) => { setDaily(e.target.value); setError(''); }}
            disabled={dailyUnlimited}
            placeholder="напр. 1000"
          />
          <label className="mt-1 inline-flex items-center gap-2 text-sm text-gray-600 dark:text-slate-400 cursor-pointer">
            <input
              type="checkbox"
              checked={dailyUnlimited}
              onChange={(e) => setDailyUnlimited(e.target.checked)}
              className="rounded"
            />
            Без лимита
          </label>
        </div>
        <div>
          <Input
            label="Месячный лимит (msg)"
            type="number"
            min="0"
            value={monthly}
            onChange={(e) => { setMonthly(e.target.value); setError(''); }}
            disabled={monthlyUnlimited}
            placeholder="напр. 30000"
          />
          <label className="mt-1 inline-flex items-center gap-2 text-sm text-gray-600 dark:text-slate-400 cursor-pointer">
            <input
              type="checkbox"
              checked={monthlyUnlimited}
              onChange={(e) => setMonthlyUnlimited(e.target.checked)}
              className="rounded"
            />
            Без лимита
          </label>
        </div>
        {error && <p role="alert" className="text-xs text-red-600 dark:text-red-400">{error}</p>}
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>Отмена</Button>
          <Button type="submit" disabled={submitting}>
            {submitting ? 'Сохранение...' : 'Сохранить'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
