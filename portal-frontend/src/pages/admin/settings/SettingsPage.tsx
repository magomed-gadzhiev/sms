import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../../components/layout/PageHeader';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { useToast } from '../../../components/ui/Toast';
import { systemDefaultsApi } from '../../../api/admin';

interface SettingField {
  key: string;
  label: string;
  description: string;
}

const SECTIONS: { title: string; fields: SettingField[] }[] = [
  {
    title: 'Лимиты скорости (глобальные)',
    fields: [
      { key: 'rate_limit_per_second', label: 'Сообщений в секунду', description: 'Максимальное количество сообщений в секунду для одного клиента' },
      { key: 'rate_limit_per_minute', label: 'Сообщений в минуту', description: 'Максимальное количество сообщений в минуту для одного клиента' },
      { key: 'rate_limit_per_hour', label: 'Сообщений в час', description: 'Максимальное количество сообщений в час для одного клиента' },
    ],
  },
  {
    title: 'Провайдеры и клиенты',
    fields: [
      { key: 'default_tps_per_provider', label: 'TPS по умолчанию (провайдер)', description: 'Транзакций в секунду по умолчанию при подключении нового провайдера' },
      { key: 'max_providers_per_client', label: 'Максимум провайдеров на клиента', description: 'Максимальное количество провайдеров, которые можно назначить одному клиенту' },
      { key: 'max_sub_accounts', label: 'Максимум субаккаунтов', description: 'Максимальное количество субаккаунтов для одного клиента' },
    ],
  },
];

type ValuesMap = Record<string, number>;

export function SettingsPage() {
  const toast = useToast();

  const [saved, setSaved] = useState<ValuesMap>({});
  const [draft, setDraft] = useState<ValuesMap>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState<Record<string, boolean>>({});

  const fetchDefaults = useCallback(async () => {
    setLoading(true);
    try {
      const data = await systemDefaultsApi.getAll();
      setSaved(data);
      setDraft(data);
    } catch {
      toast.error('Не удалось загрузить настройки');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => { fetchDefaults(); }, [fetchDefaults]);

  function hasChanges(fields: SettingField[]): boolean {
    return fields.some((f) => draft[f.key] !== saved[f.key]);
  }

  async function handleSaveSection(fields: SettingField[]) {
    const changedFields = fields.filter((f) => draft[f.key] !== saved[f.key]);
    if (changedFields.length === 0) return;

    const sectionKey = fields[0].key;
    setSaving((s) => ({ ...s, [sectionKey]: true }));
    try {
      await Promise.all(
        changedFields.map((f) => systemDefaultsApi.set(f.key, draft[f.key] ?? 0)),
      );
      setSaved((s) => {
        const next = { ...s };
        changedFields.forEach((f) => { next[f.key] = draft[f.key] ?? 0; });
        return next;
      });
      toast.success('Настройки сохранены');
    } catch {
      toast.error('Не удалось сохранить настройки');
    } finally {
      setSaving((s) => ({ ...s, [sectionKey]: false }));
    }
  }

  function handleReset(fields: SettingField[]) {
    setDraft((d) => {
      const next = { ...d };
      fields.forEach((f) => { next[f.key] = saved[f.key] ?? 0; });
      return next;
    });
  }

  if (loading) {
    return (
      <>
        <PageHeader
          title="Настройки"
          breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Настройки' }]}
        />
        <div className="text-center py-16 text-gray-400">Загрузка...</div>
      </>
    );
  }

  return (
    <>
      <PageHeader
        title="Настройки"
        breadcrumbs={[
          { label: 'Админ', href: '/admin/dashboard' },
          { label: 'Настройки' },
        ]}
      />

      <div className="space-y-6 max-w-2xl">
        {SECTIONS.map((section) => {
          const changed = hasChanges(section.fields);
          const isSaving = saving[section.fields[0].key];

          return (
            <div
              key={section.title}
              className={`bg-white border rounded-lg p-6 transition-colors ${
                changed ? 'border-yellow-400 shadow-sm' : 'border-gray-200'
              }`}
            >
              <div className="flex items-center justify-between mb-4">
                <h2 className="text-base font-semibold text-gray-900">{section.title}</h2>
                {changed && (
                  <span className="text-xs text-yellow-600 bg-yellow-50 border border-yellow-200 px-2 py-0.5 rounded-full">
                    Есть несохранённые изменения
                  </span>
                )}
              </div>

              <div className="space-y-4">
                {section.fields.map((field) => (
                  <div key={field.key}>
                    <label className="block text-sm font-medium text-gray-700 mb-1">
                      {field.label}
                    </label>
                    <Input
                      type="number"
                      min={0}
                      value={String(draft[field.key] ?? 0)}
                      onChange={(e) =>
                        setDraft((d) => ({ ...d, [field.key]: Number(e.target.value) }))
                      }
                    />
                    <p className="mt-1 text-xs text-gray-500">{field.description}</p>
                  </div>
                ))}
              </div>

              {changed && (
                <div className="mt-4 flex justify-end gap-3 pt-4 border-t border-gray-100">
                  <Button variant="ghost" onClick={() => handleReset(section.fields)}>
                    Отменить изменения
                  </Button>
                  <Button
                    onClick={() => handleSaveSection(section.fields)}
                    disabled={isSaving}
                  >
                    {isSaving ? 'Сохранение...' : 'Сохранить'}
                  </Button>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </>
  );
}
