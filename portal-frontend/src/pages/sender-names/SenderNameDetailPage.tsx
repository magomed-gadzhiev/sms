import { useState, useEffect, useCallback, type FormEvent } from 'react';
import { useParams, useSearchParams, Link } from 'react-router-dom';
import {
  senderNamesApi,
  ApiError,
  type SenderNameInfo,
} from '../../api/client';
import { Badge } from '../../components/ui/Badge';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { useToast } from '../../components/ui/Toast';
import { OverviewTab }   from './tabs/OverviewTab';
import { TemplatesTab }  from './tabs/TemplatesTab';
import { OperatorsTab }  from './tabs/OperatorsTab';
import { BillingTab }    from './tabs/BillingTab';
import { HistoryTab }    from './tabs/HistoryTab';
import { STATUS_BADGE } from './senderNameUtils';

const TABS = [
  { value: 'overview',   label: 'Обзор' },
  { value: 'templates',  label: 'Шаблоны' },
  { value: 'operators',  label: 'Операторы' },
  { value: 'billing',    label: 'Биллинг' },
  { value: 'history',    label: 'История' },
] as const;
type TabValue = typeof TABS[number]['value'];

export function SenderNameDetailPage() {
  const { id } = useParams<{ id: string }>();
  const toast = useToast();

  // Tab state synced to URL
  const [searchParams, setSearchParams] = useSearchParams();
  const rawTab = searchParams.get('tab');
  const activeTab: TabValue = (TABS.some((t) => t.value === rawTab) ? rawTab : 'overview') as TabValue;
  const setActiveTab = (t: TabValue) => {
    const next = new URLSearchParams(searchParams);
    next.set('tab', t);
    setSearchParams(next, { replace: true });
  };

  const [senderName, setSenderName] = useState<SenderNameInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Resubmit form state (only relevant when status === 'rejected')
  const [editName, setEditName] = useState('');
  const [editError, setEditError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [showResubmit, setShowResubmit] = useState(false);

  const onTabKeyDown = (e: React.KeyboardEvent<HTMLButtonElement>) => {
    const currentIdx = TABS.findIndex((t) => t.value === activeTab);
    if (e.key === 'ArrowRight') {
      e.preventDefault();
      const next = TABS[(currentIdx + 1) % TABS.length];
      setActiveTab(next.value);
    } else if (e.key === 'ArrowLeft') {
      e.preventDefault();
      const prev = TABS[(currentIdx - 1 + TABS.length) % TABS.length];
      setActiveTab(prev.value);
    }
  };

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError('');
    try {
      const sn = await senderNamesApi.get(id);
      setSenderName(sn);
      setEditName(sn.name);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { load(); }, [load]);

  const handleResubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!id || !senderName) return;
    const trimmed = editName.trim();
    if (!trimmed) { setEditError('Имя обязательно'); return; }
    setSubmitting(true);
    setEditError('');
    try {
      if (trimmed !== senderName.name) {
        await senderNamesApi.update(id, trimmed);
      }
      await senderNamesApi.resubmit(id);
      toast.success('Имя отправлено на повторное рассмотрение');
      setShowResubmit(false);
      load();
    } catch (e) {
      setEditError(e instanceof ApiError ? e.message : 'Ошибка');
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
      </div>
    );
  }

  if (error || !senderName) {
    return (
      <div>
        <Link to="/sender-names" className="text-primary text-sm hover:underline">← Имена отправителей</Link>
        <div className="mt-4 p-3 bg-red-50 text-red-700 rounded-md text-sm">{error || 'Не найдено'}</div>
      </div>
    );
  }

  const s = STATUS_BADGE[senderName.status] ?? { variant: 'default' as const, label: senderName.status };

  return (
    <div>
      {/* Breadcrumb */}
      <Link to="/sender-names" className="text-primary text-sm hover:underline mb-4 inline-block">
        ← Имена отправителей
      </Link>

      {/* Page header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-bold font-mono">{senderName.name}</h1>
          <Badge variant={s.variant}>{s.label}</Badge>
        </div>

        {/* Header actions */}
        <div className="flex items-center gap-2">
          {senderName.status === 'rejected' && (
            <Button
              variant="secondary"
              onClick={() => {
                setShowResubmit((v) => !v);
                setEditError('');
                setEditName(senderName.name);
              }}
            >
              Исправить и переотправить
            </Button>
          )}
          {senderName.status === 'approved' && (
            <Button
              variant="secondary"
              onClick={() => {
                const next = new URLSearchParams(searchParams);
                next.set('tab', 'operators');
                setSearchParams(next, { replace: true });
              }}
            >
              Управление операторами
            </Button>
          )}
        </div>
      </div>

      {/* Resubmit form (toggled from header, cross-tab) */}
      {showResubmit && senderName.status === 'rejected' && (
        <div className="mb-6 p-4 border border-gray-200 rounded-lg bg-gray-50">
          <h3 className="text-sm font-medium text-gray-700 mb-2">Исправить и отправить повторно</h3>
          <form onSubmit={handleResubmit} className="flex gap-2 items-start">
            <div>
              <Input
                value={editName}
                onChange={(e) => { setEditName(e.target.value); setEditError(''); }}
                maxLength={15}
                className="font-mono w-48"
              />
              <p className="text-xs text-gray-400 mt-1">1–11 латинских букв/цифр или 1–15 цифр</p>
              {editError && <p className="text-sm text-red-600 mt-1">{editError}</p>}
            </div>
            <Button type="submit" disabled={submitting}>Отправить</Button>
            <Button
              type="button"
              variant="secondary"
              onClick={() => setShowResubmit(false)}
            >
              Отмена
            </Button>
          </form>
        </div>
      )}

      {/* Tab bar */}
      <div className="border-b border-gray-200 mb-6" role="tablist" aria-label="Разрезы имени отправителя">
        <div className="flex gap-6">
          {TABS.map((t) => {
            const active = activeTab === t.value;
            return (
              <button
                key={t.value}
                type="button"
                role="tab"
                id={`sn-tab-${t.value}`}
                aria-selected={active}
                aria-controls={`sn-panel-${t.value}`}
                tabIndex={active ? 0 : -1}
                onClick={() => setActiveTab(t.value)}
                onKeyDown={onTabKeyDown}
                className={[
                  'py-3 -mb-px border-b-2 text-sm font-medium transition-colors',
                  active
                    ? 'border-primary text-primary'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300',
                ].join(' ')}
              >
                {t.label}
              </button>
            );
          })}
        </div>
      </div>

      {/* Tab panels */}
      <div
        id={`sn-panel-${activeTab}`}
        role="tabpanel"
        aria-labelledby={`sn-tab-${activeTab}`}
      >
        {activeTab === 'overview'  && <OverviewTab  senderName={senderName} />}
        {activeTab === 'templates' && <TemplatesTab senderName={senderName} />}
        {activeTab === 'operators' && <OperatorsTab senderName={senderName} />}
        {activeTab === 'billing'   && <BillingTab   senderName={senderName} />}
        {activeTab === 'history'   && <HistoryTab   senderName={senderName} />}
      </div>
    </div>
  );
}
