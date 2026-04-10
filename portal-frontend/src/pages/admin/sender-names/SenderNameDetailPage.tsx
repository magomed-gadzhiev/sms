import { useState, useEffect, useCallback, type FormEvent } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { PageHeader } from '../../../components/layout/PageHeader';
import { Badge } from '../../../components/ui/Badge';
import { Button } from '../../../components/ui/Button';
import { Modal } from '../../../components/ui/Modal';
import { Input } from '../../../components/ui/Input';
import { useToast } from '../../../components/ui/Toast';
import {
  adminSenderNamesApi,
  type AdminSenderNameInfo,
  type SenderNameOperatorRegistration,
  AdminApiError,
} from '../../../api/admin';

const STATUS_BADGE: Record<string, { variant: 'warning' | 'success' | 'danger' | 'default'; label: string }> = {
  pending: { variant: 'warning', label: 'На модерации' },
  approved: { variant: 'success', label: 'Одобрено' },
  rejected: { variant: 'danger', label: 'Отклонено' },
  deactivated: { variant: 'default', label: 'Деактивировано' },
};

const REG_STATUS_BADGE: Record<string, { variant: 'warning' | 'success' | 'danger' | 'default'; label: string }> = {
  not_registered: { variant: 'default', label: 'Не зарегистрировано' },
  pending: { variant: 'warning', label: 'Ожидает' },
  registered: { variant: 'success', label: 'Зарегистрировано' },
  rejected: { variant: 'danger', label: 'Отклонено' },
};

function formatDate(dt: string) {
  return new Date(dt).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
}

export function SenderNameDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const toast = useToast();

  const [senderName, setSenderName] = useState<AdminSenderNameInfo | null>(null);
  const [registrations, setRegistrations] = useState<SenderNameOperatorRegistration[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [showReject, setShowReject] = useState(false);
  const [rejectReason, setRejectReason] = useState('');
  const [showDeactivate, setShowDeactivate] = useState(false);
  const [deactivateReason, setDeactivateReason] = useState('');
  const [showDelete, setShowDelete] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError('');
    try {
      const [snRes, regRes] = await Promise.all([
        adminSenderNamesApi.get(id),
        adminSenderNamesApi.operatorRegistrations(id).catch(() => ({ registrations: [] })),
      ]);
      setSenderName(snRes.sender_name);
      setRegistrations(regRes.registrations);
    } catch (e) {
      setError(e instanceof AdminApiError ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { load(); }, [load]);

  const handleApprove = async () => {
    if (!senderName) return;
    setSubmitting(true);
    try {
      await adminSenderNamesApi.approve(senderName.id);
      toast.success('Имя одобрено');
      load();
    } catch (e) {
      toast.error(e instanceof AdminApiError ? e.message : 'Ошибка');
    } finally {
      setSubmitting(false);
    }
  };

  const handleReject = async (e: FormEvent) => {
    e.preventDefault();
    if (!senderName || !rejectReason.trim()) return;
    setSubmitting(true);
    try {
      await adminSenderNamesApi.reject(senderName.id, rejectReason.trim());
      toast.success('Имя отклонено');
      setShowReject(false);
      setRejectReason('');
      load();
    } catch (e) {
      toast.error(e instanceof AdminApiError ? e.message : 'Ошибка');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeactivate = async (e: FormEvent) => {
    e.preventDefault();
    if (!senderName) return;
    setSubmitting(true);
    try {
      await adminSenderNamesApi.deactivate(senderName.id, deactivateReason.trim());
      toast.success('Имя деактивировано');
      setShowDeactivate(false);
      setDeactivateReason('');
      load();
    } catch (e) {
      toast.error(e instanceof AdminApiError ? e.message : 'Ошибка');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async () => {
    if (!senderName) return;
    setSubmitting(true);
    try {
      await adminSenderNamesApi.deactivate(senderName.id, 'Удалено администратором');
      toast.success('Имя удалено');
      navigate('/admin/sender-names');
    } catch (e) {
      toast.error(e instanceof AdminApiError ? e.message : 'Ошибка');
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return <div className="text-center py-12 text-gray-400">Загрузка...</div>;
  }

  if (error || !senderName) {
    return (
      <div className="p-6">
        <div className="p-3 bg-red-50 text-red-700 rounded-md text-sm mb-4">{error || 'Не найдено'}</div>
        <Button variant="secondary" onClick={() => navigate('/admin/sender-names')}>← Назад</Button>
      </div>
    );
  }

  const statusInfo = STATUS_BADGE[senderName.status] ?? { variant: 'default' as const, label: senderName.status };

  return (
    <>
      <PageHeader
        title={`Имя отправителя: ${senderName.name}`}
        breadcrumbs={[
          { label: 'Админ', href: '/admin/dashboard' },
          { label: 'Имена отправителей', href: '/admin/sender-names' },
          { label: senderName.name },
        ]}
        actions={
          <div className="flex gap-2">
            {senderName.status === 'pending' && (
              <>
                <Button variant="secondary" onClick={handleApprove} disabled={submitting}>Одобрить</Button>
                <Button variant="ghost" onClick={() => { setRejectReason(''); setShowReject(true); }}>Отклонить</Button>
              </>
            )}
            {senderName.status === 'approved' && (
              <Button variant="ghost" onClick={() => { setDeactivateReason(''); setShowDeactivate(true); }}>Деактивировать</Button>
            )}
            {senderName.status !== 'deactivated' && (
              <Button variant="ghost" onClick={() => setShowDelete(true)}>Удалить</Button>
            )}
          </div>
        }
      />

      <div className="bg-white border border-gray-200 rounded-lg p-6 mb-6">
        <h2 className="text-base font-semibold text-gray-900 mb-4">Общая информация</h2>
        <dl className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div>
            <dt className="text-xs text-gray-500 mb-1">Имя</dt>
            <dd className="font-mono font-medium text-gray-900">{senderName.name}</dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 mb-1">Клиент</dt>
            <dd className="text-sm text-gray-700">{senderName.client_id}</dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 mb-1">Статус</dt>
            <dd><Badge variant={statusInfo.variant}>{statusInfo.label}</Badge></dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 mb-1">Создано</dt>
            <dd className="text-sm text-gray-700">{formatDate(senderName.created_at)}</dd>
          </div>
          {senderName.rejection_reason && (
            <div className="col-span-2 md:col-span-4">
              <dt className="text-xs text-gray-500 mb-1">Причина отклонения</dt>
              <dd className="text-sm text-red-700">{senderName.rejection_reason}</dd>
            </div>
          )}
        </dl>
      </div>

      <div className="bg-white border border-gray-200 rounded-lg p-6">
        <h2 className="text-base font-semibold text-gray-900 mb-4">Регистрация у операторов</h2>
        {registrations.length === 0 ? (
          <p className="text-sm text-gray-400">Данные о регистрации у операторов отсутствуют</p>
        ) : (
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3">
            {registrations.map((reg) => {
              const regStatus = REG_STATUS_BADGE[reg.status] ?? { variant: 'default' as const, label: reg.status };
              return (
                <div key={reg.operator_id} className="border border-gray-200 rounded-lg p-3">
                  <div className="text-sm font-medium text-gray-900 mb-1 truncate">{reg.operator_name}</div>
                  <div className="text-xs text-gray-400 mb-2">MCC {reg.mcc} / MNC {reg.mnc}</div>
                  <Badge variant={regStatus.variant}>{regStatus.label}</Badge>
                  {reg.registered_at && (
                    <div className="text-xs text-gray-400 mt-1">{formatDate(reg.registered_at)}</div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>

      <Modal open={showReject} onClose={() => setShowReject(false)} title="Отклонить имя отправителя">
        <form onSubmit={handleReject} className="space-y-4">
          <p className="text-sm text-gray-600">Имя: <strong className="font-mono">{senderName.name}</strong></p>
          <Input
            label="Причина отклонения"
            value={rejectReason}
            onChange={(e) => setRejectReason(e.target.value)}
            placeholder="Укажите причину..."
            required
          />
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => setShowReject(false)}>Отмена</Button>
            <Button type="submit" disabled={submitting || !rejectReason.trim()}>Отклонить</Button>
          </div>
        </form>
      </Modal>

      <Modal open={showDeactivate} onClose={() => setShowDeactivate(false)} title="Деактивировать имя отправителя">
        <form onSubmit={handleDeactivate} className="space-y-4">
          <p className="text-sm text-gray-600">Имя: <strong className="font-mono">{senderName.name}</strong></p>
          <Input
            label="Причина (опционально)"
            value={deactivateReason}
            onChange={(e) => setDeactivateReason(e.target.value)}
            placeholder="Укажите причину..."
          />
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => setShowDeactivate(false)}>Отмена</Button>
            <Button type="submit" disabled={submitting}>Деактивировать</Button>
          </div>
        </form>
      </Modal>

      <Modal open={showDelete} onClose={() => setShowDelete(false)} title="Удалить имя отправителя">
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            Вы уверены, что хотите удалить имя <strong className="font-mono">{senderName.name}</strong>?
            Это действие необратимо.
          </p>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => setShowDelete(false)}>Отмена</Button>
            <Button variant="secondary" onClick={handleDelete} disabled={submitting}>Удалить</Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
