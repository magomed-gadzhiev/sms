import { useEffect, useState } from 'react';
import { ApiError, networkTariffsApi } from '../../../api/client';
import { Button } from '../../../components/ui/Button';
import { Modal } from '../../../components/ui/Modal';

interface DuplicateTemplateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  sourceTemplate: { id: string; name: string } | null;
  onSuccess?: () => void;
}

export function DuplicateTemplateDialog({
  open,
  onOpenChange,
  sourceTemplate,
  onSuccess,
}: DuplicateTemplateDialogProps) {
  const [name, setName] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open && sourceTemplate) {
      setName(`Копия ${sourceTemplate.name}`);
      setError(null);
      setSubmitting(false);
    }
  }, [open, sourceTemplate]);

  if (!sourceTemplate) return null;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const trimmed = name.trim();
    if (!trimmed) {
      setError('Имя обязательно');
      return;
    }
    setError(null);
    setSubmitting(true);
    try {
      await networkTariffsApi.duplicateTemplate(sourceTemplate!.id, trimmed);
      onOpenChange(false);
      onSuccess?.();
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 409) {
          setError('Шаблон с таким именем уже существует');
        } else {
          setError(err.message || 'Ошибка дублирования');
        }
      } else {
        setError('Ошибка дублирования');
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={() => onOpenChange(false)}
      title={`Дублировать: ${sourceTemplate.name}`}
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label
            htmlFor="dup-name"
            className="block text-sm font-medium text-slate-700 mb-1"
          >
            Имя <span className="text-red-600 dark:text-red-400">*</span>
          </label>
          <input
            id="dup-name"
            type="text"
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              if (error) setError(null);
            }}
            autoFocus
            className="w-full rounded border border-slate-300 px-3 py-2 text-sm focus:border-sky-500 focus:outline-none"
            aria-invalid={!!error}
            aria-describedby={error ? 'dup-name-error' : undefined}
          />
          {error && (
            <p
              id="dup-name-error"
              role="alert"
              className="mt-1 text-xs text-red-600 dark:text-red-400"
            >
              {error}
            </p>
          )}
        </div>
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
            {submitting ? 'Дублирование…' : 'Дублировать'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
