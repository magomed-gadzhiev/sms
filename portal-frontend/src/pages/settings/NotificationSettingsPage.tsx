import { useCallback, useEffect, useState } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { notificationSettingsApi, type NotifSetting } from '../../api/client';

const EVENT_LABELS: Record<string, string> = {
  campaign_completed: 'Завершение рассылки',
  campaign_failed: 'Ошибка рассылки',
  low_balance: 'Низкий баланс',
  sender_name_approved: 'Подтверждение имени отправителя',
  sender_name_rejected: 'Отклонение имени отправителя',
  balance_topped_up: 'Пополнение баланса',
};

export function NotificationSettingsPage() {
  const [settings, setSettings] = useState<NotifSetting[]>([]);
  const [extraEmails, setExtraEmails] = useState<string[]>([]);
  const [newEmail, setNewEmail] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [success, setSuccess] = useState('');
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      const res = await notificationSettingsApi.get();
      setSettings(res.settings);
      setExtraEmails(res.extra_emails);
    } catch {
      setError('Не удалось загрузить настройки');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const toggle = (eventType: string, field: 'in_app' | 'email') => {
    setSettings((prev) =>
      prev.map((s) =>
        s.event_type === eventType ? { ...s, [field]: !s[field] } : s,
      ),
    );
  };

  const addEmail = () => {
    const trimmed = newEmail.trim();
    if (!trimmed || !trimmed.includes('@')) return;
    if (extraEmails.includes(trimmed)) return;
    setExtraEmails([...extraEmails, trimmed]);
    setNewEmail('');
  };

  const removeEmail = (email: string) => {
    setExtraEmails(extraEmails.filter((e) => e !== email));
  };

  const save = async () => {
    setSaving(true);
    setError('');
    setSuccess('');
    try {
      await notificationSettingsApi.update({ settings, extra_emails: extraEmails });
      setSuccess('Настройки сохранены');
    } catch {
      setError('Ошибка сохранения');
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <p className="p-6 text-gray-500">Загрузка...</p>;

  return (
    <div>
      <PageHeader title="Настройки уведомлений" />

      <div className="grid md:grid-cols-2 gap-6">
        <div className="bg-white rounded-lg border p-4">
          <h3 className="font-medium mb-4">Получение уведомлений</h3>
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-gray-500">
                <th className="py-2">Событие</th>
                <th className="py-2 text-center">В приложении</th>
                <th className="py-2 text-center">Email</th>
              </tr>
            </thead>
            <tbody>
              {settings.map((s) => (
                <tr key={s.event_type} className="border-b last:border-0">
                  <td className="py-3">{EVENT_LABELS[s.event_type] || s.event_type}</td>
                  <td className="py-3 text-center">
                    <input
                      type="checkbox"
                      checked={s.in_app}
                      onChange={() => toggle(s.event_type, 'in_app')}
                      aria-label={`В приложении: ${EVENT_LABELS[s.event_type] || s.event_type}`}
                      className="w-4 h-4 rounded border-gray-300"
                    />
                  </td>
                  <td className="py-3 text-center">
                    <input
                      type="checkbox"
                      checked={s.email}
                      onChange={() => toggle(s.event_type, 'email')}
                      aria-label={`Email: ${EVENT_LABELS[s.event_type] || s.event_type}`}
                      className="w-4 h-4 rounded border-gray-300"
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div className="bg-white rounded-lg border p-4">
          <h3 className="font-medium mb-2">Дополнительные email для уведомлений</h3>
          <p className="text-sm text-gray-500 mb-4">
            Уведомления будут приходить на ваш email и дублироваться на указанные ниже
          </p>
          <div className="flex gap-2 mb-3">
            <Input
              type="email"
              placeholder="example@mail.ru"
              value={newEmail}
              onChange={(e) => setNewEmail(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && addEmail()}
              className="flex-1"
            />
            <Button variant="secondary" onClick={addEmail}>Добавить</Button>
          </div>
          {extraEmails.length > 0 ? (
            <ul className="space-y-2">
              {extraEmails.map((email) => (
                <li key={email} className="flex items-center justify-between bg-gray-50 rounded px-3 py-2 text-sm">
                  <span>{email}</span>
                  <button
                    onClick={() => removeEmail(email)}
                    aria-label={`Удалить ${email}`}
                    className="text-red-500 hover:text-red-700 text-xs"
                  >
                    Удалить
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-sm text-gray-400">Дополнительные email не указаны</p>
          )}
        </div>
      </div>

      <div className="mt-6 flex items-center gap-4">
        <Button onClick={save} disabled={saving}>
          {saving ? 'Сохранение...' : 'Сохранить'}
        </Button>
        {success && <span className="text-sm text-green-600">{success}</span>}
        {error && <span className="text-sm text-red-600">{error}</span>}
      </div>
    </div>
  );
}
