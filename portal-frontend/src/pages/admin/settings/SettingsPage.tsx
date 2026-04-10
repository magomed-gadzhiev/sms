import { useState, useEffect, useCallback, useMemo } from 'react';
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

// Keys already covered by static SECTIONS — excluded from dynamic grouping
const STATIC_KEYS = new Set(SECTIONS.flatMap((s) => s.fields.map((f) => f.key)));

const SECTION_LABELS: Record<string, string> = {
  'sms':      'SMS по умолчанию',
  'rate':     'Rate Limits',
  'provider': 'Провайдеры',
  'notify':   'Уведомления',
  'security': 'Безопасность',
  'general':  'Общие',
};

type ValuesMap = Record<string, number>;

export function SettingsPage() {
  const toast = useToast();

  const [saved, setSaved] = useState<ValuesMap>({});
  const [draft, setDraft] = useState<ValuesMap>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState<Record<string, boolean>>({});
  const [hasChanges, setHasChanges] = useState(false);

  // Warn before unload when there are unsaved changes
  useEffect(() => {
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = '';
    };
    if (hasChanges) {
      window.addEventListener('beforeunload', handler);
    }
    return () => window.removeEventListener('beforeunload', handler);
  }, [hasChanges]);

  const fetchDefaults = useCallback(async () => {
    setLoading(true);
    try {
      const data = await systemDefaultsApi.getAll();
      setSaved(data);
      setDraft(data);
      setHasChanges(false);
    } catch {
      toast.error('Не удалось загрузить настройки');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => { fetchDefaults(); }, [fetchDefaults]);

  // Dynamic groups from keys returned by API that are NOT in static sections
  const groupedSettings = useMemo(() => {
    const groups: Record<string, Record<string, string>> = {};
    for (const [key, value] of Object.entries(saved)) {
      if (STATIC_KEYS.has(key)) continue;
      const prefix = key.includes('.') ? key.split('.')[0] : 'general';
      if (!groups[prefix]) groups[prefix] = {};
      groups[prefix][key] = String(value);
    }
    return groups;
  }, [saved]);

  function sectionHasChanges(fields: SettingField[]): boolean {
    return fields.some((f) => draft[f.key] !== saved[f.key]);
  }

  function dynamicGroupHasChanges(keys: string[]): boolean {
    return keys.some((k) => draft[k] !== saved[k]);
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
      // Re-evaluate global unsaved state after save
      setHasChanges((prev) => {
        if (!prev) return false;
        // Check if any other field still differs (draft vs saved after update)
        return false; // will be recalculated on next render via effect below
      });
    } catch {
      toast.error('Не удалось сохранить настройки');
    } finally {
      setSaving((s) => ({ ...s, [sectionKey]: false }));
    }
  }

  async function handleSaveDynamicGroup(prefix: string, keys: string[]) {
    const changedKeys = keys.filter((k) => draft[k] !== saved[k]);
    if (changedKeys.length === 0) return;

    setSaving((s) => ({ ...s, [prefix]: true }));
    try {
      await Promise.all(
        changedKeys.map((k) => systemDefaultsApi.set(k, draft[k] ?? 0)),
      );
      setSaved((s) => {
        const next = { ...s };
        changedKeys.forEach((k) => { next[k] = draft[k] ?? 0; });
        return next;
      });
      toast.success('Настройки сохранены');
    } catch {
      toast.error('Не удалось сохранить настройки');
    } finally {
      setSaving((s) => ({ ...s, [prefix]: false }));
    }
  }

  function handleReset(fields: SettingField[]) {
    setDraft((d) => {
      const next = { ...d };
      fields.forEach((f) => { next[f.key] = saved[f.key] ?? 0; });
      return next;
    });
  }

  function handleResetDynamicGroup(keys: string[]) {
    setDraft((d) => {
      const next = { ...d };
      keys.forEach((k) => { next[k] = saved[k] ?? 0; });
      return next;
    });
  }

  // Sync hasChanges with actual draft/saved diff
  useEffect(() => {
    const anyChanged = Object.keys(draft).some((k) => draft[k] !== saved[k]);
    setHasChanges(anyChanged);
  }, [draft, saved]);

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
        {/* Static sections (hard-coded fields with labels/descriptions) */}
        {SECTIONS.map((section) => {
          const changed = sectionHasChanges(section.fields);
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

        {/* Dynamic sections — keys returned by API that are not in static sections */}
        {Object.entries(groupedSettings).map(([prefix, keysObj]) => {
          const keys = Object.keys(keysObj);
          const title = SECTION_LABELS[prefix] ?? prefix.charAt(0).toUpperCase() + prefix.slice(1);
          const changed = dynamicGroupHasChanges(keys);
          const isSaving = saving[prefix];

          return (
            <div
              key={prefix}
              className={`bg-white border rounded-lg p-6 transition-colors ${
                changed ? 'border-yellow-400 shadow-sm' : 'border-gray-200'
              }`}
            >
              <div className="flex items-center justify-between mb-4">
                <h2 className="text-base font-semibold text-gray-900">{title}</h2>
                {changed && (
                  <span className="text-xs text-yellow-600 bg-yellow-50 border border-yellow-200 px-2 py-0.5 rounded-full">
                    Есть несохранённые изменения
                  </span>
                )}
              </div>

              <div className="space-y-4">
                {keys.map((key) => (
                  <div key={key}>
                    <label className="block text-sm font-medium text-gray-700 mb-1">
                      {key}
                    </label>
                    <Input
                      type="number"
                      min={0}
                      value={String(draft[key] ?? 0)}
                      onChange={(e) =>
                        setDraft((d) => ({ ...d, [key]: Number(e.target.value) }))
                      }
                    />
                  </div>
                ))}
              </div>

              {changed && (
                <div className="mt-4 flex justify-end gap-3 pt-4 border-t border-gray-100">
                  <Button variant="ghost" onClick={() => handleResetDynamicGroup(keys)}>
                    Отменить изменения
                  </Button>
                  <Button
                    onClick={() => handleSaveDynamicGroup(prefix, keys)}
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
