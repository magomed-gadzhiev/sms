import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { DropdownMenu, type DropdownMenuItem } from '../../../components/ui/DropdownMenu';
import { ConfirmDialog } from '../../../components/ui/ConfirmDialog';
import { useToast } from '../../../components/ui/Toast';
import { subAccountsApi, ApiError } from '../../../api/client';
import { TransferDialog } from './TransferDialog';
import { EditLimitsDialog } from './EditLimitsDialog';

interface SubAccount {
  id: string;
  name: string;
  balance: string;
  daily_limit: number;
  monthly_limit: number;
}

interface SubAccountKebabMenuProps {
  subAccount: SubAccount;
  onChanged: () => void;
}

export function SubAccountKebabMenu({ subAccount, onChanged }: SubAccountKebabMenuProps) {
  const navigate = useNavigate();
  const toast = useToast();
  const [transferOpen, setTransferOpen] = useState(false);
  const [limitsOpen, setLimitsOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const items: DropdownMenuItem[] = [
    { label: 'Открыть детали', onSelect: () => navigate(`/network/sub-accounts/${subAccount.id}`) },
    { label: 'Пополнить баланс', onSelect: () => setTransferOpen(true) },
    { label: 'Изменить лимиты', onSelect: () => setLimitsOpen(true) },
    { separator: true },
    { label: 'Удалить', onSelect: () => setDeleteOpen(true), variant: 'danger' },
  ];

  async function handleDelete() {
    setDeleting(true);
    try {
      await subAccountsApi.remove(subAccount.id);
      toast.success(`Суб-аккаунт ${subAccount.name} удалён`);
      onChanged();
      setDeleteOpen(false);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Не удалось удалить');
    } finally {
      setDeleting(false);
    }
  }

  return (
    <>
      <DropdownMenu
        trigger={
          <button
            type="button"
            aria-label={`Действия для ${subAccount.name}`}
            className="p-1.5 rounded hover:bg-gray-100 dark:hover:bg-slate-700 cursor-pointer
                       focus:outline-none focus:ring-2 focus:ring-primary"
          >
            <svg className="w-5 h-5 text-gray-500 dark:text-slate-400" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
              <path d="M10 6a2 2 0 110-4 2 2 0 010 4zM10 12a2 2 0 110-4 2 2 0 010 4zM10 18a2 2 0 110-4 2 2 0 010 4z" />
            </svg>
          </button>
        }
        items={items}
      />
      <TransferDialog
        open={transferOpen}
        subAccountId={subAccount.id}
        subAccountName={subAccount.name}
        subAccountBalance={subAccount.balance}
        onClose={() => setTransferOpen(false)}
        onSuccess={onChanged}
      />
      <EditLimitsDialog
        open={limitsOpen}
        subAccountId={subAccount.id}
        subAccountName={subAccount.name}
        currentDaily={subAccount.daily_limit}
        currentMonthly={subAccount.monthly_limit}
        onClose={() => setLimitsOpen(false)}
        onSuccess={onChanged}
      />
      <ConfirmDialog
        open={deleteOpen}
        title="Удалить суб-аккаунт?"
        description={`Суб-аккаунт «${subAccount.name}» будет удалён. Это действие необратимо.`}
        confirmLabel="Удалить"
        variant="danger"
        loading={deleting}
        onConfirm={handleDelete}
        onCancel={() => setDeleteOpen(false)}
      />
    </>
  );
}
