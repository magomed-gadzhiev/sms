import { Link, useNavigate } from 'react-router-dom';
import type { SubAccountTariffSummary } from '../../../api/client';
import { pluralizeRules } from '../../../utils/pluralize';
import { SubAccountTariffRowMenu } from './SubAccountTariffRowMenu';

function formatPrice(value: number | null, currency: string): string {
  if (value === null) return '—';
  const unit = currency === 'RUB' ? '₽' : currency;
  return `${value.toFixed(2)} ${unit}`;
}

interface Props {
  row: SubAccountTariffSummary;
  onChanged: () => void;
}

export function SubAccountTariffCard({ row, onChanged }: Props) {
  const nav = useNavigate();
  const href = `/network/tariffs/editor/${row.sub_account_id}?mode=override`;

  return (
    <article
      role="link"
      tabIndex={0}
      onClick={(e) => {
        if ((e.target as HTMLElement).closest('[data-no-row-click]')) return;
        nav(href);
      }}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          nav(href);
        }
      }}
      className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800
                 rounded-lg p-3 hover:bg-slate-50 dark:hover:bg-slate-800/50
                 cursor-pointer focus:outline-none focus:ring-2 focus:ring-sky-500"
    >
      <header className="flex items-start justify-between gap-2 mb-2">
        <div>
          <div className="font-medium text-slate-900 dark:text-slate-100">
            {row.sub_account_name}
          </div>
          <div className="text-xs text-slate-500">{row.sub_account_email}</div>
        </div>
        <div data-no-row-click>
          <SubAccountTariffRowMenu row={row} onChanged={onChanged} />
        </div>
      </header>
      <div className="flex flex-wrap items-center gap-2 mb-2" data-no-row-click>
        {row.template_id && row.template_name ? (
          <Link
            to={`/network/tariffs/editor/${row.template_id}?mode=template`}
            className="inline-block px-2 py-0.5 rounded-full text-xs bg-sky-100 text-sky-800 hover:bg-sky-200 dark:bg-sky-900/40 dark:text-sky-300 dark:hover:bg-sky-900"
          >
            {row.template_name}
          </Link>
        ) : null}
        {row.override_count > 0 ? (
          <Link
            to={`/network/tariffs/editor/${row.sub_account_id}?mode=override`}
            className="inline-block px-2 py-0.5 rounded-full text-xs bg-slate-100 text-slate-700 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
          >
            {row.override_count} {pluralizeRules(row.override_count)}
          </Link>
        ) : null}
      </div>
      <div className="flex items-center justify-between text-sm">
        <span className="text-slate-500">Средняя ₽/SMS</span>
        <span className="tabular-nums">
          {row.avg_price_per_sms === null ? (
            <span className="text-slate-500">—</span>
          ) : (
            formatPrice(row.avg_price_per_sms, row.currency)
          )}
        </span>
      </div>
    </article>
  );
}
