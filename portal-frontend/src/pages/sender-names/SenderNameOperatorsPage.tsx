import { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import {
  senderNamesApi,
  senderNameRegistrationsApi,
  operatorsApi,
  ApiError,
  type SenderNameInfo,
  type SenderNameOperatorInfo,
  type OperatorRegistration,
} from '../../api/client';

function RegistrationStatusBadge({ reg, onResubmit }: { reg: OperatorRegistration; onResubmit: (id: string) => void }) {
  if (reg.approved_type) {
    return (
      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
        Зарегистрировано ({reg.approved_type === 'free' ? 'бесплатно' : 'платно'})
      </span>
    );
  }
  const statusMap: Record<string, { label: string; className: string }> = {
    submitted: { label: 'На модерации', className: 'bg-yellow-100 text-yellow-800' },
    rejected: { label: 'Отклонено', className: 'bg-red-100 text-red-800' },
    revision_requested: { label: 'Требует доработки', className: 'bg-orange-100 text-orange-800' },
  };
  const style = statusMap[reg.status] ?? { label: reg.status, className: 'bg-gray-100 text-gray-800' };
  return (
    <div className="flex flex-col gap-1 items-start">
      <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${style.className}`}>
        {style.label}
      </span>
      {reg.moderator_note && (
        <span className="text-xs text-gray-500" title={reg.moderator_note}>
          {reg.moderator_note.length > 40 ? reg.moderator_note.slice(0, 40) + '…' : reg.moderator_note}
        </span>
      )}
      {reg.status === 'revision_requested' && (
        <button
          onClick={() => onResubmit(reg.id)}
          className="text-xs px-2 py-0.5 bg-orange-600 text-white rounded hover:bg-orange-700"
        >
          Повторно подать
        </button>
      )}
    </div>
  );
}
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';

export function SenderNameOperatorsPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const toast = useToast();

  const [senderName, setSenderName] = useState<SenderNameInfo | null>(null);
  const [operators, setOperators] = useState<SenderNameOperatorInfo[]>([]);
  const [registrations, setRegistrations] = useState<OperatorRegistration[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Selection state: { operatorId: { selected, type } }
  const [selection, setSelection] = useState<Record<string, { selected: boolean; type: string }>>({});
  const [submitting, setSubmitting] = useState(false);
  const [resubmitting, setResubmitting] = useState(false);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError('');
    try {
      const [sn, opsRes, regsRes] = await Promise.all([
        senderNamesApi.get(id),
        operatorsApi.list(),
        senderNameRegistrationsApi.list(id),
      ]);
      setSenderName(sn);
      setOperators(opsRes.operators ?? []);
      setRegistrations(regsRes.registrations ?? []);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { load(); }, [load]);

  // Initialize selection defaults once data loads
  useEffect(() => {
    const registeredIds = new Set(registrations.map((r) => r.operator_id));
    const initial: Record<string, { selected: boolean; type: string }> = {};
    for (const op of operators) {
      if (registeredIds.has(op.id)) continue; // skip registered
      if (op.registration_types.length === 0) continue; // skip unavailable
      const defaultType = op.registration_types.includes('free') ? 'free' : op.registration_types[0];
      initial[op.id] = { selected: false, type: defaultType };
    }
    setSelection(initial);
  }, [operators, registrations]);

  const toggleSelect = (opId: string) => {
    setSelection((prev) => ({
      ...prev,
      [opId]: { ...prev[opId], selected: !prev[opId]?.selected },
    }));
  };

  const toggleAll = () => {
    const allSelected = Object.values(selection).every((s) => s.selected);
    setSelection((prev) => {
      const next = { ...prev };
      for (const key of Object.keys(next)) {
        next[key] = { ...next[key], selected: !allSelected };
      }
      return next;
    });
  };

  const setType = (opId: string, type: string) => {
    setSelection((prev) => ({
      ...prev,
      [opId]: { ...prev[opId], type },
    }));
  };

  const selectedCount = Object.values(selection).filter((s) => s.selected).length;

  const handleResubmit = async (registrationId: string) => {
    if (!id) return;
    setResubmitting(true);
    try {
      await senderNameRegistrationsApi.resubmit(id, registrationId);
      toast.success('Заявка повторно подана');
      load();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    } finally {
      setResubmitting(false);
    }
  };

  const handleSubmit = async () => {
    if (!id) return;
    const regs = Object.entries(selection)
      .filter(([, s]) => s.selected)
      .map(([operatorId, s]) => ({ operator_id: operatorId, type: s.type }));
    if (regs.length === 0) return;

    setSubmitting(true);
    try {
      await senderNameRegistrationsApi.bulkCreate(id, regs);
      toast.success('Регистрация отправлена');
      navigate(`/sender-names/${id}`);
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка регистрации');
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

  const registeredIds = new Set(registrations.map((r) => r.operator_id));

  return (
    <div>
      {/* Breadcrumb */}
      <div className="text-sm text-gray-500 mb-4">
        <Link to="/sender-names" className="text-primary hover:underline">Имена отправителей</Link>
        {' › '}
        <Link to={`/sender-names/${id}`} className="text-primary hover:underline">{senderName.name}</Link>
        {' › '}
        <span>Регистрация у операторов</span>
      </div>

      <h1 className="text-xl font-semibold mb-1">
        Регистрация имени «{senderName.name}» у операторов
      </h1>
      <p className="text-sm text-gray-500 mb-6">
        Выберите операторов, укажите тип регистрации и нажмите «Зарегистрировать»
      </p>

      {/* Table */}
      <div className="border rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b-2 border-gray-200 bg-gray-50">
              <th className="w-10 px-3 py-3 text-left">
                <input
                  type="checkbox"
                  checked={Object.keys(selection).length > 0 && Object.values(selection).every((s) => s.selected)}
                  onChange={toggleAll}
                  className="w-4 h-4"
                />
              </th>
              <th className="px-3 py-3 text-left text-gray-500 font-medium">Оператор</th>
              <th className="px-3 py-3 text-left text-gray-500 font-medium">Тип регистрации</th>
              <th className="px-3 py-3 text-left text-gray-500 font-medium">Стоимость</th>
              <th className="px-3 py-3 text-left text-gray-500 font-medium">Статус</th>
            </tr>
          </thead>
          <tbody>
            {operators.map((op) => {
              const isRegistered = registeredIds.has(op.id);
              const hasTypes = op.registration_types.length > 0;
              const isDisabled = isRegistered || !hasTypes;
              const sel = selection[op.id];
              const isSelected = sel?.selected ?? false;
              const currentType = sel?.type ?? '';

              const existingReg = registrations.find((r) => r.operator_id === op.id);

              return (
                <tr
                  key={op.id}
                  className={`border-b border-gray-100 ${
                    isRegistered ? 'bg-gray-50' : isSelected ? 'bg-blue-50' : ''
                  }`}
                >
                  <td className="px-3 py-3">
                    <input
                      type="checkbox"
                      checked={isSelected}
                      onChange={() => toggleSelect(op.id)}
                      disabled={isDisabled}
                      className="w-4 h-4 disabled:opacity-40"
                    />
                  </td>
                  <td className="px-3 py-3">
                    <div className={`font-medium ${isDisabled ? 'text-gray-400' : 'text-gray-900'}`}>{op.name}</div>
                    <div className="text-xs text-gray-400">{op.slug.toLowerCase()}</div>
                  </td>
                  <td className="px-3 py-3">
                    {isDisabled ? (
                      <span className="text-gray-400">—</span>
                    ) : (
                      <select
                        value={currentType}
                        onChange={(e) => setType(op.id, e.target.value)}
                        className="border border-gray-300 rounded-md px-2 py-1 text-sm"
                      >
                        {op.registration_types.includes('free') && <option value="free">Бесплатная</option>}
                        {op.registration_types.includes('paid') && <option value="paid">Платная</option>}
                      </select>
                    )}
                  </td>
                  <td className="px-3 py-3">
                    {!isDisabled && currentType === 'paid' && op.monthly_tariff_amount ? (
                      <span className="font-semibold">
                        {parseFloat(op.monthly_tariff_amount).toLocaleString('ru-RU', { style: 'currency', currency: 'RUB' })}/мес
                      </span>
                    ) : (
                      <span className="text-gray-400">—</span>
                    )}
                  </td>
                  <td className="px-3 py-3">
                    {existingReg ? (
                      <RegistrationStatusBadge reg={existingReg} onResubmit={handleResubmit} />
                    ) : (
                      <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800">
                        Не зарегистрировано
                      </span>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {/* Footer */}
      <div className="flex items-center gap-3 mt-4">
        <Button onClick={handleSubmit} disabled={submitting || resubmitting || selectedCount === 0}>
          {submitting ? 'Регистрация...' : `Зарегистрировать у выбранных (${selectedCount})`}
        </Button>
        <Button variant="ghost" onClick={() => navigate(`/sender-names/${id}`)}>Отмена</Button>
      </div>
    </div>
  );
}
