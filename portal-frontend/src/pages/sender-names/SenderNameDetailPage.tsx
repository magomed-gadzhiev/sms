import { useState, useEffect, useCallback, type FormEvent } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import {
  senderNamesApi,
  senderNameRegistrationsApi,
  operatorsApi,
  ApiError,
  type SenderNameInfo,
  type SenderNameHistoryEntry,
  type OperatorRegistration,
  type SenderNameOperatorInfo,
} from '../../api/client';
import { Badge } from '../../components/ui/Badge';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { useToast } from '../../components/ui/Toast';

const STATUS_BADGE: Record<string, { variant: 'warning' | 'success' | 'danger' | 'default'; label: string }> = {
  pending: { variant: 'warning', label: 'На модерации' },
  approved: { variant: 'success', label: 'Одобрено' },
  rejected: { variant: 'danger', label: 'Отклонено' },
  deactivated: { variant: 'default', label: 'Деактивировано' },
};

function formatDate(dt: string) {
  return new Date(dt).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
}

export function SenderNameDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const toast = useToast();

  const [senderName, setSenderName] = useState<SenderNameInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Operator registrations
  const [registrations, setRegistrations] = useState<OperatorRegistration[]>([]);
  const [operators, setOperators] = useState<SenderNameOperatorInfo[]>([]);

  // History
  const [history, setHistory] = useState<SenderNameHistoryEntry[]>([]);
  const [showHistory, setShowHistory] = useState(false);
  const [historyLoading, setHistoryLoading] = useState(false);

  // Resubmit
  const [editName, setEditName] = useState('');
  const [editError, setEditError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError('');
    try {
      const sn = await senderNamesApi.get(id);
      setSenderName(sn);
      setEditName(sn.name);

      if (sn.status === 'approved') {
        const [regsRes, opsRes] = await Promise.all([
          senderNameRegistrationsApi.list(id),
          operatorsApi.list(),
        ]);
        setRegistrations(regsRes.registrations ?? []);
        setOperators(opsRes.operators ?? []);
      }
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { load(); }, [load]);

  const loadHistory = async () => {
    if (!id) return;
    setHistoryLoading(true);
    try {
      const res = await senderNamesApi.getHistory(id);
      setHistory(res.entries ?? []);
      setShowHistory(true);
    } catch {
      // ignore
    } finally {
      setHistoryLoading(false);
    }
  };

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
  const registeredIds = new Set(registrations.map((r) => r.operator_id));

  return (
    <div>
      <Link to="/sender-names" className="text-primary text-sm hover:underline mb-4 inline-block">← Имена отправителей</Link>

      <div className="flex items-center gap-3 mb-6">
        <h1 className="text-2xl font-bold font-mono">{senderName.name}</h1>
        <Badge variant={s.variant}>{s.label}</Badge>
      </div>

      {/* Metadata */}
      <div className="grid grid-cols-2 gap-4 text-sm mb-6">
        <div>
          <span className="text-gray-400">Создано</span>
          <div className="text-gray-900">{formatDate(senderName.created_at)}</div>
        </div>
        {senderName.reviewed_at && (
          <div>
            <span className="text-gray-400">{senderName.status === 'approved' ? 'Одобрено' : 'Отклонено'}</span>
            <div className="text-gray-900">{formatDate(senderName.reviewed_at)}</div>
          </div>
        )}
      </div>

      {/* Approved: operator registrations */}
      {senderName.status === 'approved' && (
        <div className="border-t pt-4 mb-6">
          <h3 className="text-sm font-medium text-gray-700 mb-3">Регистрация у операторов</h3>
          <div className="flex gap-2 flex-wrap mb-3">
            {operators.map((op) => {
              const registered = registeredIds.has(op.id);
              return (
                <span
                  key={op.id}
                  className={`px-3 py-1 rounded-full text-xs font-medium ${
                    registered ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-500'
                  }`}
                >
                  {registered ? '✓ ' : ''}{op.name}
                </span>
              );
            })}
          </div>
          <Button onClick={() => navigate(`/sender-names/${id}/operators`)}>
            Зарегистрировать у операторов →
          </Button>
        </div>
      )}

      {/* Rejected: reason + resubmit form */}
      {senderName.status === 'rejected' && (
        <div className="border-t pt-4 mb-6">
          {senderName.rejection_reason && (
            <div className="p-3 bg-red-50 border border-red-200 rounded-md mb-4">
              <p className="text-sm font-medium text-red-800">Причина отклонения</p>
              <p className="text-sm text-red-700 mt-1">{senderName.rejection_reason}</p>
            </div>
          )}
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
          </form>
        </div>
      )}

      {/* Pending info */}
      {senderName.status === 'pending' && (
        <div className="border-t pt-4 mb-6">
          <p className="text-sm text-gray-500">Имя находится на модерации. Мы уведомим вас о результате.</p>
        </div>
      )}

      {/* History */}
      <div className="border-t pt-4">
        {!showHistory ? (
          <button
            onClick={loadHistory}
            disabled={historyLoading}
            className="text-primary text-sm hover:underline"
          >
            ▸ Показать историю статусов
          </button>
        ) : (
          <div>
            <p className="text-sm font-medium text-gray-700 mb-2">История статусов</p>
            <div className="space-y-2">
              {history.map((e) => (
                <div key={e.id} className="flex items-start gap-2 text-sm">
                  <span className="text-gray-400 shrink-0">{formatDate(e.created_at)}</span>
                  <span>
                    {e.old_status ? `${e.old_status} → ` : ''}<strong>{e.new_status}</strong>
                    {e.comment && <span className="text-gray-500 ml-1">({e.comment})</span>}
                    <span className="text-xs text-gray-400 ml-1">({e.actor_type})</span>
                  </span>
                </div>
              ))}
              {history.length === 0 && <p className="text-sm text-gray-400">История пуста</p>}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
