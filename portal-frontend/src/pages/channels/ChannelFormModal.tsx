import { useState, type FormEvent } from 'react';
import { cascadeChannelsApi, type DeliveryChannel } from '../../api/cascade';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
import { Button } from '../../components/ui/Button';

const CHANNEL_TYPES = [
  { value: 'sms', label: 'SMS' },
  { value: 'flash_call', label: 'Flash Call' },
  { value: 'reverse_call', label: 'Reverse Call' },
  { value: 'messenger', label: 'Messenger' },
  { value: 'max_messenger', label: 'Max Messenger' },
];

interface Props {
  channel: DeliveryChannel | null;
  onClose: () => void;
  onSaved: () => void;
}

export function ChannelFormModal({ channel, onClose, onSaved }: Props) {
  const isEdit = channel !== null;

  const [channelType, setChannelType] = useState(channel?.channel_type ?? 'sms');
  const [name, setName] = useState(channel?.name ?? '');
  const [description, setDescription] = useState(channel?.description ?? '');
  const [configJson, setConfigJson] = useState('{}');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');

    // Validate JSON
    try {
      JSON.parse(configJson);
    } catch {
      setError('config_json содержит невалидный JSON');
      return;
    }

    setSubmitting(true);
    try {
      if (isEdit) {
        await cascadeChannelsApi.update(channel.channel_id, { name, description, config_json: configJson });
      } else {
        await cascadeChannelsApi.create({ channel_type: channelType, name, description, config_json: configJson });
      }
      onSaved();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Ошибка сохранения');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal open={true} title={isEdit ? 'Редактировать канал' : 'Добавить канал'} onClose={onClose}>
      <form onSubmit={handleSubmit} className="space-y-4">
        {!isEdit && (
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Тип канала</label>
            <select
              value={channelType}
              onChange={(e) => setChannelType(e.target.value)}
              className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              required
            >
              {CHANNEL_TYPES.map((t) => (
                <option key={t.value} value={t.value}>{t.label}</option>
              ))}
            </select>
          </div>
        )}

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Название</label>
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Название канала"
            required
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Описание</label>
          <Input
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Описание (необязательно)"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Конфигурация (JSON)</label>
          <textarea
            value={configJson}
            onChange={(e) => setConfigJson(e.target.value)}
            rows={5}
            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary"
            placeholder='{"api_url": "https://...", "api_key": "..."}'
          />
          <p className="text-xs text-gray-500 mt-1">Sensitive поля будут зашифрованы на сервере</p>
        </div>

        {error && (
          <p className="text-sm text-red-600">{error}</p>
        )}

        <div className="flex justify-end gap-3 pt-2">
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Отмена
          </Button>
          <Button type="submit" disabled={submitting}>
            {isEdit ? 'Сохранить' : 'Создать'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
