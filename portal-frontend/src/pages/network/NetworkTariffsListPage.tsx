import { useEffect, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import {
  networkTariffsApi,
  type SubAccountTariffSummary,
} from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { pluralizeRules } from '../../utils/pluralize';

function formatPrice(value: number | null, currency: string): string {
  if (value == null) return '—';
  const unit = currency === 'RUB' ? '₽' : currency;
  return `${value.toFixed(2)} ${unit}`;
}

export function NetworkTariffsListPage() {
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

  useEffect(() => {
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
  }, []);

  const filtered = rows.filter(
    (r) =>
      (!onlyOverrides || r.override_count > 0) &&
      (search === '' ||
        r.sub_account_name.toLowerCase().includes(search.toLowerCase()) ||
        r.sub_account_email.toLowerCase().includes(search.toLowerCase())),
  );

  if (loading) {
    return (
      <div className="max-w-6xl">
        <PageHeader title="Тарифы субаккаунтов" />
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
        <PageHeader title="Тарифы субаккаунтов" />
        <div
          role="alert"
          className="rounded border border-red-200 bg-red-50 text-red-800 px-4 py-3 text-sm"
        >
          Не удалось загрузить список: {error}
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-6xl">
      <PageHeader
        title="Тарифы субаккаунтов"
        actions={
          <Link
            to="/network/tariffs/templates"
            className="text-sm text-sky-700 hover:text-sky-900 hover:underline"
          >
            Шаблоны →
          </Link>
        }
      />
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
      </div>
      {filtered.length === 0 ? (
        <div className="text-center text-slate-500 py-12 border border-dashed border-slate-200 rounded">
          {rows.length === 0
            ? 'Нет субаккаунтов.'
            : 'Ничего не найдено по заданным фильтрам.'}
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
              <th className="px-3 py-2" aria-label="Действия" />
            </tr>
          </thead>
          <tbody>
            {filtered.map((r) => (
              <tr
                key={r.sub_account_id}
                className="border-t border-slate-100 hover:bg-slate-50 cursor-pointer"
                onClick={() =>
                  nav(
                    `/network/tariffs/editor/${r.sub_account_id}?mode=override`,
                  )
                }
              >
                <td className="px-3 py-2">
                  <div className="font-medium text-slate-900">
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
                      className="inline-block px-2 py-0.5 rounded-full text-xs bg-sky-100 text-sky-800 hover:bg-sky-200"
                      onClick={(e) => e.stopPropagation()}
                    >
                      {r.template_name}
                    </Link>
                  ) : (
                    <span className="text-slate-400">—</span>
                  )}
                </td>
                <td className="px-3 py-2">
                  {r.override_count > 0 ? (
                    <Link
                      to={`/network/tariffs/editor/${r.sub_account_id}?mode=override`}
                      className="inline-block px-2 py-0.5 rounded-full text-xs bg-amber-100 text-amber-800 hover:bg-amber-200"
                      onClick={(e) => e.stopPropagation()}
                    >
                      {r.override_count} {pluralizeRules(r.override_count)}
                    </Link>
                  ) : (
                    <span className="text-slate-400">—</span>
                  )}
                </td>
                <td className="px-3 py-2 text-right tabular-nums">
                  {formatPrice(r.avg_price_per_sms, r.currency)}
                </td>
                <td className="px-3 py-2 text-right">
                  <Link
                    to={`/network/tariffs/editor/${r.sub_account_id}?mode=override`}
                    className="text-sky-700 hover:text-sky-900 hover:underline"
                    onClick={(e) => e.stopPropagation()}
                  >
                    Открыть ▸
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
