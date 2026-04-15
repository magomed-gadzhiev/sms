import { useState, useEffect, type ReactNode } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { StatusBadge } from '../../components/ui/Badge';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
import { useToast } from '../../components/ui/Toast';
import {
  resellerApi,
  ApiError,
  type ResellerSenderName,
  type ResellerTemplate,
  type ModerationCounts,
} from '../../api/client';

type Tab = 'sender_names' | 'templates' | 'registrations';

function TabButton({
  active,
  onClick,
  children,
  count,
}: {
  active: boolean;
  onClick: () => void;
  children: ReactNode;
  count?: number;
}) {
  return (
    <button
      onClick={onClick}
      className={`px-4 py-2 text-sm font-medium rounded-t-lg border-b-2 transition-colors ${
        active
          ? 'border-primary text-primary bg-white'
          : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
      }`}
    >
      {children}
      {count != null && count > 0 && (
        <span className="ml-1.5 inline-flex items-center justify-center px-1.5 py-0.5 text-xs font-bold rounded-full bg-red-100 text-red-700">
          {count}
        </span>
      )}
    </button>
  );
}

export function ModerationPage() {
  const toast = useToast();
  const [tab, setTab] = useState<Tab>('sender_names');
  const [counts, setCounts] = useState<ModerationCounts>({ sender_names: 0, templates: 0, registrations: 0 });

  // Sender names state
  const [senderNames, setSenderNames] = useState<ResellerSenderName[]>([]);
  const [snLoading, setSnLoading] = useState(false);
  const [snStatusFilter, setSnStatusFilter] = useState('');

  // Templates state
  const [templates, setTemplates] = useState<ResellerTemplate[]>([]);
  const [tplLoading, setTplLoading] = useState(false);
  const [tplStatusFilter, setTplStatusFilter] = useState('');

  // Operator registrations state
  const [registrations, setRegistrations] = useState<unknown[]>([]);
  const [regLoading, setRegLoading] = useState(false);
  const [regStatusFilter, setRegStatusFilter] = useState('');

  // Modal state
  const [modal, setModal] = useState<{
    type: 'reject_sn' | 'reject_tpl' | 'revision_tpl' | 'reject_reg' | 'revision_reg';
    id: string;
  } | null>(null);
  const [modalText, setModalText] = useState('');
  const [modalSubmitting, setModalSubmitting] = useState(false);

  useEffect(() => {
    resellerApi.getModerationCounts().then(setCounts).catch(() => {});
  }, []);

  // Load sender names
  useEffect(() => {
    if (tab !== 'sender_names') return;
    setSnLoading(true);
    resellerApi
      .listSenderNames(snStatusFilter ? { status: snStatusFilter } : undefined)
      .then((r) => setSenderNames(r.sender_names))
      .catch(() => toast.error('Не удалось загрузить имена'))
      .finally(() => setSnLoading(false));
  }, [tab, snStatusFilter]);

  // Load templates
  useEffect(() => {
    if (tab !== 'templates') return;
    setTplLoading(true);
    resellerApi
      .listTemplates(tplStatusFilter ? { status: tplStatusFilter } : undefined)
      .then((r) => setTemplates(r.templates))
      .catch(() => toast.error('Не удалось загрузить шаблоны'))
      .finally(() => setTplLoading(false));
  }, [tab, tplStatusFilter]);

  // Load registrations
  useEffect(() => {
    if (tab !== 'registrations') return;
    setRegLoading(true);
    resellerApi
      .listOperatorRegistrations(regStatusFilter ? { status: regStatusFilter } : undefined)
      .then((r) => setRegistrations(r.registrations))
      .catch(() => toast.error('Не удалось загрузить регистрации'))
      .finally(() => setRegLoading(false));
  }, [tab, regStatusFilter]);

  async function handleApproveSN(id: string) {
    try {
      await resellerApi.approveSenderName(id);
      toast.success('Имя одобрено');
      setSenderNames((prev) => prev.filter((sn) => sn.id !== id));
      resellerApi.getModerationCounts().then(setCounts).catch(() => {});
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    }
  }

  async function handleApproveTpl(id: string) {
    try {
      await resellerApi.approveTemplate(id);
      toast.success('Шаблон одобрен');
      setTemplates((prev) => prev.filter((t) => t.id !== id));
      resellerApi.getModerationCounts().then(setCounts).catch(() => {});
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    }
  }

  async function handleApproveReg(id: string) {
    try {
      await resellerApi.approveOperatorRegistration(id);
      toast.success('Регистрация одобрена');
      setRegistrations((prev) => (prev as { id: string }[]).filter((r) => r.id !== id));
      resellerApi.getModerationCounts().then(setCounts).catch(() => {});
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    }
  }

  async function handleModalSubmit() {
    if (!modal || !modalText.trim()) return;
    setModalSubmitting(true);
    try {
      switch (modal.type) {
        case 'reject_sn':
          await resellerApi.rejectSenderName(modal.id, modalText);
          setSenderNames((prev) => prev.filter((sn) => sn.id !== modal.id));
          toast.success('Имя отклонено');
          break;
        case 'reject_tpl':
          await resellerApi.rejectTemplate(modal.id, modalText);
          setTemplates((prev) => prev.filter((t) => t.id !== modal.id));
          toast.success('Шаблон отклонён');
          break;
        case 'revision_tpl':
          await resellerApi.requestRevisionTemplate(modal.id, modalText);
          setTemplates((prev) => prev.filter((t) => t.id !== modal.id));
          toast.success('Запрос на доработку отправлен');
          break;
        case 'reject_reg':
          await resellerApi.rejectOperatorRegistration(modal.id, modalText);
          setRegistrations((prev) => (prev as { id: string }[]).filter((r) => r.id !== modal.id));
          toast.success('Регистрация отклонена');
          break;
        case 'revision_reg':
          await resellerApi.requestRevisionOperatorRegistration(modal.id, modalText);
          setRegistrations((prev) => (prev as { id: string }[]).filter((r) => r.id !== modal.id));
          toast.success('Запрос на доработку отправлен');
          break;
      }
      resellerApi.getModerationCounts().then(setCounts).catch(() => {});
      setModal(null);
      setModalText('');
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    } finally {
      setModalSubmitting(false);
    }
  }

  const modalTitle =
    modal?.type === 'reject_sn' || modal?.type === 'reject_tpl' || modal?.type === 'reject_reg'
      ? 'Отклонить'
      : 'Запросить доработку';
  const modalLabel =
    modal?.type === 'revision_tpl' || modal?.type === 'revision_reg' ? 'Комментарий' : 'Причина отклонения';

  // --- Column defs ---
  const snColumns: Column<ResellerSenderName>[] = [
    { key: 'sub_account_email', header: 'Субаккаунт' },
    { key: 'name', header: 'Имя отправителя' },
    {
      key: 'status',
      header: 'Статус',
      render: (sn) => <StatusBadge status={sn.status} />,
    },
    {
      key: 'created_at',
      header: 'Создан',
      render: (sn) => new Date(sn.created_at).toLocaleDateString('ru-RU'),
    },
    {
      key: 'actions' as keyof ResellerSenderName,
      header: '',
      render: (sn) =>
        sn.status === 'pending' ? (
          <div className="flex gap-1">
            <Button size="sm" onClick={() => handleApproveSN(sn.id)}>
              Одобрить
            </Button>
            <Button size="sm" variant="danger" onClick={() => { setModal({ type: 'reject_sn', id: sn.id }); setModalText(''); }}>
              Отклонить
            </Button>
          </div>
        ) : null,
    },
  ];

  const tplColumns: Column<ResellerTemplate>[] = [
    { key: 'sub_account_email', header: 'Субаккаунт' },
    { key: 'name', header: 'Название' },
    { key: 'body_preview', header: 'Содержание' },
    {
      key: 'status',
      header: 'Статус',
      render: (t) => <StatusBadge status={t.status} />,
    },
    {
      key: 'created_at',
      header: 'Создан',
      render: (t) => new Date(t.created_at).toLocaleDateString('ru-RU'),
    },
    {
      key: 'actions' as keyof ResellerTemplate,
      header: '',
      render: (t) =>
        t.status === 'pending' || t.status === 'revision_requested' ? (
          <div className="flex gap-1">
            <Button size="sm" onClick={() => handleApproveTpl(t.id)}>
              Одобрить
            </Button>
            <Button size="sm" variant="danger" onClick={() => { setModal({ type: 'reject_tpl', id: t.id }); setModalText(''); }}>
              Отклонить
            </Button>
            <Button size="sm" variant="secondary" onClick={() => { setModal({ type: 'revision_tpl', id: t.id }); setModalText(''); }}>
              Доработка
            </Button>
          </div>
        ) : null,
    },
  ];

  const regColumns: Column<Record<string, unknown>>[] = [
    { key: 'sub_account_email' as string, header: 'Субаккаунт' },
    { key: 'sender_name' as string, header: 'Имя отправителя' },
    { key: 'operator_name' as string, header: 'Оператор' },
    { key: 'registration_type' as string, header: 'Тип' },
    {
      key: 'status' as string,
      header: 'Статус',
      render: (r) => <StatusBadge status={r.status as string} />,
    },
    {
      key: 'actions' as string,
      header: '',
      render: (r) =>
        r.status === 'submitted' ? (
          <div className="flex gap-1">
            <Button size="sm" onClick={() => handleApproveReg(r.id as string)}>
              Одобрить
            </Button>
            <Button size="sm" variant="danger" onClick={() => { setModal({ type: 'reject_reg', id: r.id as string }); setModalText(''); }}>
              Отклонить
            </Button>
            <Button size="sm" variant="secondary" onClick={() => { setModal({ type: 'revision_reg', id: r.id as string }); setModalText(''); }}>
              Доработка
            </Button>
          </div>
        ) : null,
    },
  ];

  function StatusFilter({ value, onChange, options }: { value: string; onChange: (v: string) => void; options: { value: string; label: string }[] }) {
    return (
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="border border-gray-300 rounded px-2 py-1 text-sm"
      >
        <option value="">Все статусы</option>
        {options.map((o) => (
          <option key={o.value} value={o.value}>{o.label}</option>
        ))}
      </select>
    );
  }

  return (
    <div className="max-w-6xl">
      <PageHeader title="Модерация" subtitle="Заявки субаккаунтов" />

      {/* Tabs */}
      <div className="flex gap-1 border-b border-gray-200 mb-4">
        <TabButton active={tab === 'sender_names'} onClick={() => setTab('sender_names')} count={counts.sender_names}>
          Имена отправителей
        </TabButton>
        <TabButton active={tab === 'templates'} onClick={() => setTab('templates')} count={counts.templates}>
          Шаблоны
        </TabButton>
        <TabButton active={tab === 'registrations'} onClick={() => setTab('registrations')} count={counts.registrations}>
          Регистрации у операторов
        </TabButton>
      </div>

      {/* Sender names tab */}
      {tab === 'sender_names' && (
        <>
          <div className="mb-3">
            <StatusFilter
              value={snStatusFilter}
              onChange={setSnStatusFilter}
              options={[
                { value: 'pending', label: 'На рассмотрении' },
                { value: 'approved', label: 'Одобрено' },
                { value: 'rejected', label: 'Отклонено' },
              ]}
            />
          </div>
          {snLoading ? (
            <div className="py-8 text-center text-gray-400">Загрузка...</div>
          ) : senderNames.length === 0 ? (
            <div className="py-8 text-center text-gray-400">Нет заявок</div>
          ) : (
            <DataTable columns={snColumns} data={senderNames} total={senderNames.length} page={1} pageSize={50} onPageChange={() => {}} keyField="id" />
          )}
        </>
      )}

      {/* Templates tab */}
      {tab === 'templates' && (
        <>
          <div className="mb-3">
            <StatusFilter
              value={tplStatusFilter}
              onChange={setTplStatusFilter}
              options={[
                { value: 'pending', label: 'На рассмотрении' },
                { value: 'approved', label: 'Одобрено' },
                { value: 'rejected', label: 'Отклонено' },
                { value: 'revision_requested', label: 'На доработке' },
              ]}
            />
          </div>
          {tplLoading ? (
            <div className="py-8 text-center text-gray-400">Загрузка...</div>
          ) : templates.length === 0 ? (
            <div className="py-8 text-center text-gray-400">Нет заявок</div>
          ) : (
            <DataTable columns={tplColumns} data={templates} total={templates.length} page={1} pageSize={50} onPageChange={() => {}} keyField="id" />
          )}
        </>
      )}

      {/* Registrations tab */}
      {tab === 'registrations' && (
        <>
          <div className="mb-3">
            <StatusFilter
              value={regStatusFilter}
              onChange={setRegStatusFilter}
              options={[
                { value: 'submitted', label: 'На рассмотрении' },
                { value: 'approved', label: 'Одобрено' },
                { value: 'rejected', label: 'Отклонено' },
                { value: 'revision_requested', label: 'На доработке' },
              ]}
            />
          </div>
          {regLoading ? (
            <div className="py-8 text-center text-gray-400">Загрузка...</div>
          ) : registrations.length === 0 ? (
            <div className="py-8 text-center text-gray-400">Нет заявок</div>
          ) : (
            <DataTable columns={regColumns} data={registrations as Record<string, unknown>[]} total={registrations.length} page={1} pageSize={50} onPageChange={() => {}} keyField="id" />
          )}
        </>
      )}

      {/* Reject / Request Revision modal */}
      <Modal
        open={!!modal}
        onClose={() => { setModal(null); setModalText(''); }}
        title={modalTitle}
      >
        <div className="flex flex-col gap-4">
          <Input
            label={modalLabel}
            value={modalText}
            onChange={(e) => setModalText(e.target.value)}
            required
            placeholder={modalLabel === 'Комментарий' ? 'Укажите, что нужно исправить' : 'Укажите причину'}
          />
          <div className="flex gap-2">
            <Button onClick={handleModalSubmit} disabled={modalSubmitting || !modalText.trim()}>
              {modalSubmitting ? 'Отправка...' : 'Подтвердить'}
            </Button>
            <Button variant="secondary" onClick={() => { setModal(null); setModalText(''); }}>
              Отмена
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
