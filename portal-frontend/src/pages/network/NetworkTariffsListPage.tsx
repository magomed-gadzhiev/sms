import { useEffect, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { ChevronRight } from 'lucide-react';
import {
  networkTariffsApi,
  type SubAccountTariffSummary,
} from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { usePageTitle } from '../../contexts/PageTitleContext';
import { pluralizeRules } from '../../utils/pluralize';
import { useMediaQuery } from '../../hooks/useMediaQuery';
import { SubAccountTariffCard } from './components/SubAccountTariffCard';
import { SubAccountTariffRowMenu } from './components/SubAccountTariffRowMenu';

function formatPrice(value: number | null, currency: string): string {
  if (value == null) return '—';
  const unit = currency === 'RUB' ? '₽' : currency;
  return `${value.toFixed(2)} ${unit}`;
}

export function NetworkTariffsListPage() {
  usePageTitle('Тарифы сети');
  const [rows, setRows] = useState<SubAccountTariffSummary[]>([]);
  const [search, setSearch] = useState('');
  const [onlyOverrides, setOnlyOverrides] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [params] = useSearchParams();
  const nav = useNavigate();

  // Task 10: legacy ?tab=* URL redirect. Old UI used a single /network/tariffs
  // page with tab query param; new URL-based routing needs to catch those.
  useEffect(() => {
    const tab = params.get('tab');
    if (tab === 'templates') {
      nav('/network/tariffs/templates', { replace: true });
    } else if (tab === 'overrides' || tab === 'overview') {
      nav('/network/tariffs', { replace: true });
    }
  }, [params, nav]);

  const refetch = () => {
    let cancelled = false;
    setLoading(true);
    networkTariffsApi
      .listSubaccounts()
      .then((d) => {
        if (!cancelled) setRows(d);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  };

  useEffect(() => {
    return refetch();
  }, []);

  const q = search.trim().toLowerCase();
  const filtered = rows.filter(
    (r) =>
      (!onlyOverrides || r.override_count > 0) &&
      (q === '' ||
        r.sub_account_name.toLowerCase().includes(q) ||
        r.sub_account_email.toLowerCase().includes(q)),
  );

  const isMobile = useMediaQuery('(max-width: 639px)');

  if (loading) {
    return (
      <div className="max-w-6xl">
        <div className="space-y-2" aria-busy="true" aria-label="Загрузка">
          {[0, 1, 2, 3].map((i) => (
            <div key={i} className="h-10 bg-slate-100 rounded animate-pulse" />
          ))}
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="max-w-6xl">
        <div
          role="alert"
          className="rounded border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-950/40 text-red-800 dark:text-red-300 px-4 py-3 text-sm"
        >
          Не удалось загрузить список: {error}
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-6xl">
      <PageHeader />
      <div className="flex flex-wrap gap-3 mb-4 items-center">
        <input
          type="search"
          className="border border-slate-300 rounded px-3 py-1.5 text-sm flex-1 max-w-md"
          placeholder="Поиск субаккаунта"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          aria-label="Поиск субаккаунта"
        />
        <label className="flex items-center gap-2 text-sm text-slate-700">
          <input
            type="checkbox"
            checked={onlyOverrides}
            onChange={(e) => setOnlyOverrides(e.target.checked)}
          />
          Только с переопределениями
        </label>
        <Link
          to="/network/tariffs/templates"
          className="ml-auto inline-flex items-center gap-1.5 px-2 py-1.5 rounded border border-slate-300 hover:border-slate-400 hover:bg-slate-50 text-sm font-medium text-slate-700 dark:border-slate-600 dark:text-slate-300 dark:hover:bg-slate-800"
        >
          Шаблоны
          <ChevronRight className="w-4 h-4" />
        </Link>
      </div>
      {filtered.length === 0 ? (
        <div className="text-center text-slate-500 py-12 border border-dashed border-slate-200 dark:border-slate-700 rounded">
          {rows.length === 0
            ? 'Нет субаккаунтов.'
            : 'Ничего не найдено по заданным фильтрам.'}
        </div>
      ) : isMobile ? (
        <div className="space-y-2">
          {filtered.map((r) => (
            <SubAccountTariffCard key={r.sub_account_id} row={r} onChanged={refetch} />
          ))}
        </div>
      ) : (
        <table className="w-full text-sm">
          <thead className="text-left text-slate-500 bg-slate-50">
            <tr>
              <th className="px-3 py-2 font-medium">Субаккаунт</th>
              <th className="px-3 py-2 font-medium">Шаблон</th>
              <th className="px-3 py-2 font-medium">Переопр.</th>
              <th className="px-3 py-2 font-medium text-right">
                Средняя ₽/SMS
              </th>
              <th className="w-10 px-3 py-2" aria-label="Действия" />
            </tr>
          </thead>
          <tbody>
            {filtered.map((r) => (
              <tr
                key={r.sub_account_id}
                className="border-t border-slate-100 hover:bg-slate-50 dark:border-slate-800 dark:hover:bg-slate-800/50 cursor-pointer focus:outline-none focus:ring-2 focus:ring-sky-500"
                onClick={() =>
                  nav(
                    `/network/tariffs/editor/${r.sub_account_id}?mode=override`,
                  )
                }
                role="link"
                tabIndex={0}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    nav(`/network/tariffs/editor/${r.sub_account_id}?mode=override`);
                  }
                }}
              >
                <td className="px-3 py-2">
                  <div className="font-medium text-slate-900 dark:text-slate-100">
                    {r.sub_account_name}
                  </div>
                  <div className="text-xs text-slate-500">
                    {r.sub_account_email}
                  </div>
                </td>
                <td className="px-3 py-2">
                  {r.template_id && r.template_name ? (
                    <Link
                      to={`/network/tariffs/editor/${r.template_id}?mode=template`}
                      className="inline-block px-2 py-0.5 rounded-full text-xs bg-sky-100 text-sky-800 hover:bg-sky-200 dark:bg-sky-900/40 dark:text-sky-300 dark:hover:bg-sky-900"
                      onClick={(e) => e.stopPropagation()}
                    >
                      {r.template_name}
                    </Link>
                  ) : (
                    <span className="text-slate-500 dark:text-slate-400">—</span>
                  )}
                </td>
                <td className="px-3 py-2">
                  {r.override_count > 0 ? (
                    <Link
                      to={`/network/tariffs/editor/${r.sub_account_id}?mode=override`}
                      className="inline-block px-2 py-0.5 rounded-full text-xs bg-slate-100 text-slate-700 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
                      onClick={(e) => e.stopPropagation()}
                    >
                      {r.override_count} {pluralizeRules(r.override_count)}
                    </Link>
                  ) : (
                    <span className="text-slate-500 dark:text-slate-400">—</span>
                  )}
                </td>
                <td className="px-3 py-2 text-right tabular-nums">
                  {r.avg_price_per_sms === null ? (
                    <span className="text-slate-500 dark:text-slate-400">—</span>
                  ) : (
                    formatPrice(r.avg_price_per_sms, r.currency)
                  )}
                </td>
                <td className="px-3 py-2">
                  <SubAccountTariffRowMenu row={r} onChanged={refetch} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
