import { useNavigate } from 'react-router-dom';
import { StatusBadge } from '../../../components/ui/Badge';
import { SubAccountKebabMenu } from './SubAccountKebabMenu';
import { formatNumber, formatRub } from '../utils/formatNumber';

interface SubAccount {
  id: string;
  name: string;
  email: string;
  active: boolean;
  balance: string;
  daily_limit: number;
  monthly_limit: number;
  messages_today: number;
  messages_this_month: number;
}

interface SubAccountCardProps {
  subAccount: SubAccount;
  onChanged: () => void;
}

const LOW_BALANCE_THRESHOLD = 100;

export function SubAccountCard({ subAccount, onChanged }: SubAccountCardProps) {
  const navigate = useNavigate();
  const balance = parseFloat(subAccount.balance);
  const isLow = balance < LOW_BALANCE_THRESHOLD;

  function handleClick(e: React.MouseEvent) {
    if ((e.target as HTMLElement).closest('[data-no-row-click]')) return;
    navigate(`/network/sub-accounts/${subAccount.id}`);
  }

  return (
    <article
      className="bg-white dark:bg-slate-900 border border-gray-200 dark:border-slate-700
                 rounded-lg p-4 hover:bg-gray-50 dark:hover:bg-slate-800/50
                 cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary"
      tabIndex={0}
      onClick={handleClick}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          navigate(`/network/sub-accounts/${subAccount.id}`);
        }
      }}
    >
      <header className="flex items-center justify-between gap-2 mb-2">
        <span className="inline-flex items-center gap-2 font-medium text-gray-900 dark:text-slate-100">
          {isLow && (
            <span
              className="w-2 h-2 rounded-full bg-red-500 shrink-0"
              aria-label="Низкий баланс"
              title="Баланс ниже 100 ₽"
            />
          )}
          {subAccount.name}
        </span>
        <div className="flex items-center gap-2" data-no-row-click>
          <StatusBadge status={subAccount.active ? 'active' : 'inactive'} />
          <SubAccountKebabMenu subAccount={subAccount} onChanged={onChanged} />
        </div>
      </header>
      <p className="text-sm text-gray-500 dark:text-slate-400 mb-3 break-all">
        {subAccount.email || <span className="text-gray-400">—</span>}
      </p>
      <dl className="grid grid-cols-2 gap-x-4 gap-y-1.5 text-sm">
        <dt className="text-gray-500 dark:text-slate-400">Баланс</dt>
        <dd className={isLow ? 'text-red-600 dark:text-red-400 font-medium' : 'text-gray-900 dark:text-slate-100'}>
          {formatRub(subAccount.balance)}
        </dd>

        <dt className="text-gray-500 dark:text-slate-400">Дневной лимит</dt>
        <dd className="text-gray-900 dark:text-slate-100">
          {subAccount.daily_limit === 0
            ? <span aria-label="без лимита">∞</span>
            : `${formatNumber(subAccount.daily_limit)} msg`}
        </dd>

        <dt className="text-gray-500 dark:text-slate-400">Сегодня</dt>
        <dd className="text-gray-900 dark:text-slate-100">
          {subAccount.daily_limit > 0
            ? `${formatNumber(subAccount.messages_today)} / ${formatNumber(subAccount.daily_limit)}`
            : `${formatNumber(subAccount.messages_today)} msg`}
        </dd>

        <dt className="text-gray-500 dark:text-slate-400">За месяц</dt>
        <dd className="text-gray-900 dark:text-slate-100">
          {subAccount.monthly_limit > 0
            ? `${formatNumber(subAccount.messages_this_month)} / ${formatNumber(subAccount.monthly_limit)}`
            : `${formatNumber(subAccount.messages_this_month)} msg`}
        </dd>
      </dl>
    </article>
  );
}
