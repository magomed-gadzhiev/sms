import { useCallback, useEffect, useId, useState } from 'react';
import { usePageTitle } from '../../contexts/PageTitleContext';
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
  usePageTitle('Настройки уведомлений');
  const newEmailId = useId();
  const newEmailDescId = useId();
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

  if (loading) return <p className="p-6 text-gray-500 dark:text-slate-400">Загрузка...</p>;

  return (
    <div>

      <div className="grid md:grid-cols-2 gap-6">
        <div className="bg-white dark:bg-slate-900 rounded-lg border p-4">
          <h3 className="font-medium mb-4">Получение уведомлений</h3>
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-gray-500 dark:text-slate-400">
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
                      className="w-4 h-4 rounded border-gray-300 dark:border-slate-600"
                    />
                  </td>
                  <td className="py-3 text-center">
                    <input
                      type="checkbox"
                      checked={s.email}
                      onChange={() => toggle(s.event_type, 'email')}
                      aria-label={`Email: ${EVENT_LABELS[s.event_type] || s.event_type}`}
                      className="w-4 h-4 rounded border-gray-300 dark:border-slate-600"
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div className="bg-white dark:bg-slate-900 rounded-lg border p-4">
          <h3 className="font-medium mb-2">Дополнительные email для уведомлений</h3>
          <p id={newEmailDescId} className="text-sm text-gray-500 dark:text-slate-400 mb-4">
            Уведомления будут приходить на ваш email и дублироваться на указанные ниже
          </p>
          <div className="flex gap-2 mb-3 items-end">
            <div className="flex-1 flex flex-col gap-1">
              <label htmlFor={newEmailId} className="text-xs text-gray-500 dark:text-slate-400 font-medium">Email</label>
              <Input
                id={newEmailId}
                type="email"
                aria-describedby={newEmailDescId}
                placeholder="example@mail.ru"
                value={newEmail}
                onChange={(e) => setNewEmail(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && addEmail()}
              />
            </div>
            <Button variant="secondary" onClick={addEmail}>Добавить</Button>
          </div>
          {extraEmails.length > 0 ? (
            <ul className="space-y-2">
              {extraEmails.map((email) => (
                <li key={email} className="flex items-center justify-between bg-gray-50 dark:bg-slate-950 rounded px-3 py-2 text-sm">
                  <span>{email}</span>
                  <button
                    onClick={() => removeEmail(email)}
                    aria-label={`Удалить ${email}`}
                    className="text-red-500 dark:text-red-400 hover:text-red-700 text-xs"
                  >
                    Удалить
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-sm text-gray-400 dark:text-slate-500">Дополнительные email не указаны</p>
          )}
        </div>
      </div>

      <div className="mt-6 flex items-center gap-4">
        <Button onClick={save} disabled={saving}>
          {saving ? 'Сохранение...' : 'Сохранить'}
        </Button>
        {success && <span className="text-sm text-green-600 dark:text-green-400">{success}</span>}
        {error && <span className="text-sm text-red-600 dark:text-red-400">{error}</span>}
      </div>
    </div>
  );
}
