import { useState, useEffect, type FormEvent } from 'react';
import { Modal } from '../../../components/ui/Modal';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { useToast } from '../../../components/ui/Toast';
import { subAccountsApi, billingApi, ApiError } from '../../../api/client';
import { formatRub } from '../utils/formatNumber';

interface TransferDialogProps {
  open: boolean;
  subAccountId: string;
  subAccountName: string;
  subAccountBalance: string;
  onClose: () => void;
  onSuccess: () => void;
}

export function TransferDialog({
  open, subAccountId, subAccountName, subAccountBalance, onClose, onSuccess,
}: TransferDialogProps) {
  const toast = useToast();
  const [amount, setAmount] = useState('');
  const [parentBalance, setParentBalance] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!open) return;
    setAmount('');
    setError('');
    setParentBalance(null);
    billingApi.getBalance()
      .then((res) => setParentBalance(res.balance))
      .catch(() => { /* silent */ });
  }, [open]);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const n = parseFloat(amount);
    if (!Number.isFinite(n) || n <= 0) {
      setError('Введите положительную сумму');
      return;
    }
    setSubmitting(true);
    setError('');
    try {
      await subAccountsApi.transfer(subAccountId, amount);
      toast.success(`Баланс ${subAccountName} пополнен на ${formatRub(n)}`);
      onSuccess();
      onClose();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось выполнить перевод');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={`Пополнить баланс — ${subAccountName}`}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div className="text-sm text-gray-600 dark:text-slate-400">
          Текущий баланс суб-аккаунта: <strong>{formatRub(subAccountBalance)}</strong>
        </div>
        {parentBalance !== null && (
          <div className="text-sm text-gray-600 dark:text-slate-400">
            Доступно у вас: <strong>{formatRub(parentBalance)}</strong>
          </div>
        )}
        <Input
          label="Сумма перевода (₽)"
          type="number"
          step="0.01"
          min="0"
          value={amount}
          onChange={(e) => { setAmount(e.target.value); setError(''); }}
          required
          placeholder="100.00"
          autoFocus
        />
        {error && (
          <p role="alert" className="text-xs text-red-600 dark:text-red-400">{error}</p>
        )}
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>Отмена</Button>
          <Button type="submit" disabled={submitting}>
            {submitting ? 'Пополнение...' : 'Пополнить'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
