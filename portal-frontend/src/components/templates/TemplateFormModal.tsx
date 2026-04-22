import { useState, useEffect, type FormEvent } from 'react';
import { templatesApi, ApiError, type TemplateInfo, type SenderNameInfo } from '../../api/client';
import { useFormValidation } from '../../hooks/useFormValidation';
import { CharacterCounter } from '../ui/CharacterCounter';
import { Button } from '../ui/Button';
import { Modal } from '../ui/Modal';
import { Input } from '../ui/Input';
import { Select } from '../ui/Select';

export interface TemplateFormModalProps {
  open: boolean;
  onClose: () => void;
  onSubmitted: () => void; // called after successful save
  editingTemplate?: TemplateInfo | null;
  preselectedSenderNameId?: string; // for sender-name-first flow (Task 6)
  availableSenderNames: SenderNameInfo[];
  availableSenderNamesError?: boolean;
  onReloadSenderNames?: () => void;
}

export function TemplateFormModal({
  open,
  onClose,
  onSubmitted,
  editingTemplate,
  preselectedSenderNameId,
  availableSenderNames,
  availableSenderNamesError,
  onReloadSenderNames,
}: TemplateFormModalProps) {
  const [formName, setFormName] = useState('');
  const [formBody, setFormBody] = useState('');
  const [formSenderNameId, setFormSenderNameId] = useState('');
  const [formTrafficType, setFormTrafficType] = useState('transactional');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const validation = useFormValidation({
    name: { required: true, minLength: 3, maxLength: 100 },
    body: { required: true, maxLength: 1600 },
  });

  useEffect(() => {
    if (!open) return;
    validation.reset();
    setFormName(editingTemplate?.name ?? '');
    setFormBody(editingTemplate?.body ?? '');
    setFormSenderNameId(editingTemplate?.sender_name_id ?? preselectedSenderNameId ?? '');
    setFormTrafficType(editingTemplate?.traffic_type ?? 'transactional');
    setError('');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, editingTemplate?.id, preselectedSenderNameId]);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const valid = validation.validateAll({ name: formName, body: formBody });
    if (!valid) {
      validation.scrollToFirstError();
      return;
    }
    setSaving(true);
    setError('');
    try {
      if (editingTemplate) {
        await templatesApi.update(editingTemplate.id, {
          name: formName,
          body: formBody,
          sender_name_id: formSenderNameId || undefined,
          traffic_type: formTrafficType || undefined,
        });
      } else {
        await templatesApi.create({
          name: formName,
          body: formBody,
          sender_name_id: formSenderNameId || undefined,
          traffic_type: formTrafficType || undefined,
        });
      }
      onSubmitted();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось сохранить шаблон');
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={editingTemplate ? 'Редактировать шаблон' : 'Создать шаблон'}
    >
      <form onSubmit={handleSubmit}>
        {error && <p className="mb-3 text-sm text-red-600">{error}</p>}

        <div className="mb-4">
          <Input
            label="Название *"
            type="text"
            value={formName}
            onChange={(e) => { setFormName(e.target.value); validation.fieldProps('name').onChange(e); }}
            onBlur={(e) => validation.fieldProps('name').onBlur(e)}
            aria-invalid={validation.errors.name ? true : undefined}
            aria-describedby={validation.errors.name ? 'name-error' : undefined}
            required
            placeholder="Например: Код подтверждения"
            className={`w-full ${validation.errors.name ? 'border-red-400 focus:border-red-400' : ''}`}
          />
          {validation.errors.name && (
            <p id="name-error" className="mt-1 text-xs text-red-600">{validation.errors.name}</p>
          )}
        </div>

        <div className="mb-4">
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Имя отправителя
          </label>
          {availableSenderNamesError ? (
            <p className="text-xs text-red-600">
              Не удалось загрузить имена отправителей.{' '}
              {onReloadSenderNames && (
                <button type="button" className="underline" onClick={onReloadSenderNames}>Повторить</button>
              )}
            </p>
          ) : availableSenderNames.length > 0 ? (
            <select
              value={formSenderNameId}
              onChange={(e) => !preselectedSenderNameId && setFormSenderNameId(e.target.value)}
              disabled={!!preselectedSenderNameId}
              className="w-full rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary disabled:opacity-60 disabled:cursor-not-allowed"
            >
              <option value="">— Без отправителя —</option>
              {availableSenderNames.map((sn) => (
                <option key={sn.id} value={sn.id}>{sn.name}</option>
              ))}
            </select>
          ) : (
            <p className="text-xs text-gray-500">
              Нет одобренных имён отправителей.{' '}
              <a href="/sender-names" className="text-primary underline">Зарегистрировать →</a>
            </p>
          )}
        </div>

        <div className="mb-4">
          <Select
            label="Тип трафика"
            value={formTrafficType}
            onChange={setFormTrafficType}
            options={[
              { value: 'transactional', label: 'Транзакционный' },
              { value: 'authorization', label: 'Авторизационный' },
              { value: 'service', label: 'Сервисный' },
            ]}
          />
        </div>

        <div className="mb-4">
          <div className="flex items-center justify-between mb-1">
            <label className="block text-sm font-medium text-gray-700">
              Текст шаблона *
            </label>
            <CharacterCounter current={formBody.length} max={1600} />
          </div>
          <textarea
            value={formBody}
            onChange={(e) => { setFormBody(e.target.value); validation.fieldProps('body').onChange(e); }}
            onBlur={(e) => validation.fieldProps('body').onBlur(e)}
            aria-invalid={validation.errors.body ? true : undefined}
            aria-describedby={validation.errors.body ? 'body-error' : undefined}
            required
            rows={5}
            placeholder="Ваш код: {{code}}. Здравствуйте, {{name}}!"
            className={`w-full rounded border px-3 py-2 text-sm transition-colors focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary ${validation.errors.body ? 'border-red-400' : 'border-gray-300'}`}
          />
          {validation.errors.body ? (
            <p id="body-error" className="mt-1 text-xs text-red-600">{validation.errors.body}</p>
          ) : (
            <p className="mt-1 text-xs text-gray-500">
              Используйте переменные в двойных фигурных скобках: {'{{name}}'}, {'{{code}}'}, {'{{company}}'}
            </p>
          )}
        </div>

        <div className="flex gap-2 justify-end">
          <Button type="button" variant="secondary" onClick={onClose}>
            Отмена
          </Button>
          <Button type="submit" disabled={saving || !formName.trim() || !formBody.trim()}>
            {saving ? 'Сохранение...' : editingTemplate ? 'Сохранить' : 'Создать'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
