import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  ApiError,
  networkTariffsApi,
  type TariffTemplateSummary,
} from '../../../api/client';
import { Button } from '../../../components/ui/Button';
import { Modal } from '../../../components/ui/Modal';

interface CreateTemplateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function CreateTemplateDialog({
  open,
  onOpenChange,
}: CreateTemplateDialogProps) {
  const nav = useNavigate();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [copyEnabled, setCopyEnabled] = useState(false);
  const [copyFromId, setCopyFromId] = useState<string>('');
  const [templates, setTemplates] = useState<TariffTemplateSummary[]>([]);
  const [loadingTemplates, setLoadingTemplates] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [nameError, setNameError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) {
      setName('');
      setDescription('');
      setCopyEnabled(false);
      setCopyFromId('');
      setNameError(null);
      setSubmitting(false);
      return;
    }
  }, [open]);

  useEffect(() => {
    if (!open || !copyEnabled || templates.length > 0) return;
    setLoadingTemplates(true);
    networkTariffsApi
      .listTemplates()
      .then((d) => setTemplates(d))
      .catch(() => {
        // silent — user can still create without copy source
      })
      .finally(() => setLoadingTemplates(false));
  }, [open, copyEnabled, templates.length]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const trimmed = name.trim();
    if (!trimmed) {
      setNameError('Имя обязательно');
      return;
    }
    setNameError(null);
    setSubmitting(true);
    try {
      const body: { name: string; description?: string; copy_from_id?: string } =
        { name: trimmed };
      const desc = description.trim();
      if (desc) body.description = desc;
      if (copyEnabled && copyFromId) body.copy_from_id = copyFromId;
      const { id } = await networkTariffsApi.createTemplate(body);
      onOpenChange(false);
      nav(`/network/tariffs/editor/${id}?mode=template`);
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 409) {
          setNameError('Шаблон с таким именем уже существует');
        } else if (err.status === 400) {
          setNameError(err.message || 'Некорректные данные');
        } else {
          setNameError(err.message || 'Ошибка создания шаблона');
        }
      } else {
        setNameError('Ошибка создания шаблона');
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={() => onOpenChange(false)}
      title="Создать шаблон"
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label
            htmlFor="tpl-name"
            className="block text-sm font-medium text-slate-700 mb-1"
          >
            Имя <span className="text-red-600 dark:text-red-400">*</span>
          </label>
          <input
            id="tpl-name"
            type="text"
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              if (nameError) setNameError(null);
            }}
            autoFocus
            className="w-full rounded border border-slate-300 px-3 py-2 text-sm focus:border-sky-500 focus:outline-none"
            aria-invalid={!!nameError}
            aria-describedby={nameError ? 'tpl-name-error' : undefined}
          />
          {nameError && (
            <p
              id="tpl-name-error"
              role="alert"
              className="mt-1 text-xs text-red-600 dark:text-red-400"
            >
              {nameError}
            </p>
          )}
        </div>
        <div>
          <label
            htmlFor="tpl-desc"
            className="block text-sm font-medium text-slate-700 mb-1"
          >
            Описание
          </label>
          <textarea
            id="tpl-desc"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={2}
            className="w-full rounded border border-slate-300 px-3 py-2 text-sm focus:border-sky-500 focus:outline-none"
          />
        </div>
        <div>
          <label className="inline-flex items-center gap-2 text-sm text-slate-700">
            <input
              type="checkbox"
              checked={copyEnabled}
              onChange={(e) => setCopyEnabled(e.target.checked)}
              className="rounded border-slate-300"
            />
            Скопировать из шаблона…
          </label>
          {copyEnabled && (
            <select
              value={copyFromId}
              onChange={(e) => setCopyFromId(e.target.value)}
              disabled={loadingTemplates}
              className="mt-2 w-full rounded border border-slate-300 px-3 py-2 text-sm focus:border-sky-500 focus:outline-none"
            >
              <option value="">
                {loadingTemplates ? 'Загрузка…' : '— не копировать —'}
              </option>
              {templates.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name}
                </option>
              ))}
            </select>
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
            {submitting ? 'Создание…' : 'Создать'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
