import { useEffect, useState } from 'react';
import {
  ApiError,
  networkTariffsApi,
} from '../../../api/client';
import { Button } from '../../../components/ui/Button';
import { Modal } from '../../../components/ui/Modal';

export interface NewPeriodDialogPeriod {
  id: string;
  from: string;
  to: string | null;
}

interface NewPeriodDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  planId: string;
  existingPeriods: NewPeriodDialogPeriod[];
  onSuccess: (newPeriodId: string) => void;
}

function todayISO(): string {
  const d = new Date();
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

// Treat null end_date as +Infinity for overlap math.
function overlaps(
  aFrom: string,
  aTo: string | null,
  bFrom: string,
  bTo: string | null,
): boolean {
  const aEnd = aTo ?? '9999-12-31';
  const bEnd = bTo ?? '9999-12-31';
  return aFrom <= bEnd && bFrom <= aEnd;
}

export function NewPeriodDialog({
  open,
  onOpenChange,
  planId,
  existingPeriods,
  onSuccess,
}: NewPeriodDialogProps) {
  const [from, setFrom] = useState(todayISO());
  const [to, setTo] = useState('');
  const [copyFromId, setCopyFromId] = useState('');
  const [keepTiers, setKeepTiers] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) {
      setFrom(todayISO());
      setTo('');
      setCopyFromId('');
      setKeepTiers(true);
      setSubmitting(false);
      setError(null);
    }
  }, [open]);

  function validate(): string | null {
    if (!from) return 'Начало обязательно';
    if (to && to <= from) return 'Конец должен быть позже начала';
    const newTo = to || null;
    for (const p of existingPeriods) {
      if (overlaps(from, newTo, p.from, p.to)) {
        return 'Период пересекается с существующим';
      }
    }
    return null;
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const v = validate();
    if (v) {
      setError(v);
      return;
    }
    setError(null);
    setSubmitting(true);
    try {
      const body: {
        from: string;
        to?: string;
        copy_from_period_id?: string;
        keep_tiers: boolean;
      } = { from, keep_tiers: keepTiers };
      if (to) body.to = to;
      if (copyFromId) body.copy_from_period_id = copyFromId;
      const { id } = await networkTariffsApi.createPeriod(planId, body);
      onSuccess(id);
      onOpenChange(false);
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setError('Период пересекается с существующим (server)');
      } else if (err instanceof ApiError) {
        setError(err.message || 'Ошибка создания периода');
      } else {
        setError('Ошибка создания периода');
      }
    } finally {
      setSubmitting(false);
    }
  }

  const showTiersChoice = copyFromId !== '';

  return (
    <Modal open={open} onClose={() => onOpenChange(false)} title="Новый период">
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label htmlFor="np-from" className="block text-sm font-medium text-slate-700 mb-1">
            Начало <span className="text-red-600">*</span>
          </label>
          <input
            id="np-from"
            type="date"
            value={from}
            onChange={(e) => {
              setFrom(e.target.value);
              if (error) setError(null);
            }}
            required
            className="w-full rounded border border-slate-300 px-3 py-2 text-sm focus:border-sky-500 focus:outline-none"
          />
        </div>
        <div>
          <label htmlFor="np-to" className="block text-sm font-medium text-slate-700 mb-1">
            Конец <span className="text-slate-400 text-xs">(пусто = бессрочно)</span>
          </label>
          <input
            id="np-to"
            type="date"
            value={to}
            onChange={(e) => {
              setTo(e.target.value);
              if (error) setError(null);
            }}
            className="w-full rounded border border-slate-300 px-3 py-2 text-sm focus:border-sky-500 focus:outline-none"
          />
        </div>
        <div>
          <label htmlFor="np-copy" className="block text-sm font-medium text-slate-700 mb-1">
            Скопировать цены из периода
          </label>
          <select
            id="np-copy"
            value={copyFromId}
            onChange={(e) => setCopyFromId(e.target.value)}
            className="w-full rounded border border-slate-300 px-3 py-2 text-sm focus:border-sky-500 focus:outline-none"
          >
            <option value="">— не копировать —</option>
            {existingPeriods.map((p) => (
              <option key={p.id} value={p.id}>
                {p.from} — {p.to ?? 'бессрочно'}
              </option>
            ))}
          </select>
        </div>
        {showTiersChoice && (
          <fieldset className="space-y-2">
            <legend className="text-sm font-medium text-slate-700">Tiers</legend>
            <label className="flex items-center gap-2 text-sm text-slate-700">
              <input
                type="radio"
                name="np-tiers"
                checked={keepTiers}
                onChange={() => setKeepTiers(true)}
              />
              Оставить tiers выбранного периода
            </label>
            <label className="flex items-center gap-2 text-sm text-slate-700">
              <input
                type="radio"
                name="np-tiers"
                checked={!keepTiers}
                onChange={() => setKeepTiers(false)}
              />
              Новые tiers
            </label>
          </fieldset>
        )}
        {error && (
          <p role="alert" className="text-xs text-red-600">
            {error}
          </p>
        )}
        <div className="flex gap-2 justify-end pt-2">
          <Button
            type="button"
            variant="secondary"
            onClick={() => onOpenChange(false)}
            disabled={submitting}
          >
            Отмена
          </Button>
          <Button type="submit" disabled={submitting}>
            {submitting ? 'Создание…' : 'Создать'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
