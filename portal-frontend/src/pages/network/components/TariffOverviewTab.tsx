import { useState, useEffect, useMemo } from 'react';
import { Badge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import {
  resellerTariffApi,
  subAccountsApi,
  type TariffOverviewItem,
  ApiError,
} from '../../../api/client';

const CATEGORY_SHORT: Record<string, string> = {
  paid_registered: 'Платная рег.',
  free_registered: 'Бесплатная рег.',
  shared: 'Общая',
  standard: 'Стандартная',
};

const CATEGORY_ORDER = ['paid_registered', 'free_registered', 'shared', 'standard'];

const STRATEGY_LABELS: Record<string, string> = {
  fixed: 'Фикс.',
  threshold: 'Порог.',
  threshold_recalc: 'Порог. пересч.',
  prepaid_threshold: 'Предопл. порог.',
};

function sourceBadge(source: TariffOverviewItem['source']) {
  switch (source) {
    case 'override':
      return <Badge variant="warning">Override</Badge>;
    case 'template':
      return <Badge variant="success">Template</Badge>;
    default:
      return <Badge>Legacy</Badge>;
  }
}

function formatPrice(raw: string): string {
  const n = parseFloat(raw);
  if (isNaN(n)) return raw;
  return n.toFixed(2);
}

interface SubAccountOption {
  id: string;
  name: string;
}

interface TariffOverviewTabProps {
  onNavigateToPlan?: (planId: string) => void;
}

export function TariffOverviewTab({ onNavigateToPlan }: TariffOverviewTabProps) {
  const toast = useToast();
  const [subAccounts, setSubAccounts] = useState<SubAccountOption[]>([]);
  const [selectedSA, setSelectedSA] = useState('');
  const [tariffs, setTariffs] = useState<TariffOverviewItem[]>([]);
  const [templateName, setTemplateName] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [saLoading, setSaLoading] = useState(true);

  useEffect(() => {
    subAccountsApi
      .list()
      .then((r: any) => {
        const list = (r.sub_accounts || []).map((sa: any) => ({
          id: sa.id,
          name: sa.name || sa.email,
        }));
        setSubAccounts(list);
      })
      .catch(() => toast.error('Ошибка загрузки субаккаунтов'))
      .finally(() => setSaLoading(false));
  }, []);

  useEffect(() => {
    if (!selectedSA) {
      setTariffs([]);
      setTemplateName(null);
      return;
    }
    setLoading(true);
    resellerTariffApi
      .overview(selectedSA)
      .then((r) => {
        setTariffs(r.tariffs || []);
        setTemplateName(r.template_name || null);
      })
      .catch((e) =>
        toast.error(e instanceof ApiError ? e.message : 'Ошибка загрузки обзора тарифов'),
      )
      .finally(() => setLoading(false));
  }, [selectedSA]);

  const { operators, categories } = useMemo(() => {
    const opMap = new Map<string, string>();
    const catSet = new Set<string>();
    tariffs.forEach((t) => {
      opMap.set(t.operator_id, t.operator_name);
      catSet.add(t.sender_category);
    });
    const cats = [...catSet].sort((a, b) => {
      const ai = CATEGORY_ORDER.indexOf(a);
      const bi = CATEGORY_ORDER.indexOf(b);
      return (ai === -1 ? 99 : ai) - (bi === -1 ? 99 : bi);
    });
    const ops = [...opMap.entries()].sort((a, b) => a[1].localeCompare(b[1], 'ru'));
    return { operators: ops, categories: cats };
  }, [tariffs]);

  const tariffMap = useMemo(() => {
    const m = new Map<string, TariffOverviewItem>();
    tariffs.forEach((t) => m.set(`${t.operator_id}_${t.sender_category}`, t));
    return m;
  }, [tariffs]);

  return (
    <div>
      {/* Sub-account selector */}
      <div className="flex items-center gap-3 mb-6 flex-wrap">
        <div className="flex flex-col gap-1">
          <label className="text-xs font-medium text-gray-500" htmlFor="overview-sa-select">
            Субаккаунт
          </label>
          <select
            id="overview-sa-select"
            value={selectedSA}
            onChange={(e) => setSelectedSA(e.target.value)}
            disabled={saLoading}
            className="border border-gray-300 rounded px-3 py-2 text-sm min-w-[250px] focus:ring-2 focus:ring-primary/50 focus:border-primary"
          >
            <option value="">Выберите субаккаунт</option>
            {subAccounts.map((sa) => (
              <option key={sa.id} value={sa.id}>
                {sa.name}
              </option>
            ))}
          </select>
        </div>
        {templateName && (
          <div className="flex items-center gap-2 mt-5">
            <Badge variant="success">{templateName}</Badge>
          </div>
        )}
      </div>

      {/* Content */}
      {!selectedSA ? (
        <div className="py-16 text-center">
          <div className="text-gray-400 text-sm">
            Выберите субаккаунт для просмотра эффективных тарифов
          </div>
        </div>
      ) : loading ? (
        <div className="space-y-3">
          <div className="h-10 bg-gray-100 rounded animate-pulse" />
          <div className="h-64 bg-gray-100 rounded animate-pulse" />
        </div>
      ) : tariffs.length === 0 ? (
        <div className="py-16 text-center border border-gray-200 rounded-lg bg-white">
          <div className="text-gray-400">Тарифы не найдены</div>
          <div className="text-sm text-gray-400 mt-1">
            Настройте тарифы через шаблоны или переопределения
          </div>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white shadow-sm">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-gray-50">
                <th className="text-left p-3 text-xs font-semibold text-gray-500 uppercase tracking-wide sticky left-0 bg-gray-50 min-w-[160px]">
                  Оператор
                </th>
                {categories.map((cat) => (
                  <th
                    key={cat}
                    className="text-center p-3 text-xs font-semibold text-gray-500 uppercase tracking-wide min-w-[180px]"
                  >
                    {CATEGORY_SHORT[cat] || cat}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {operators.map(([opId, opName], idx) => (
                <tr
                  key={opId}
                  className={`border-t border-gray-100 ${idx % 2 === 1 ? 'bg-gray-50/50' : ''} hover:bg-blue-50/30 transition-colors`}
                >
                  <td className="p-3 font-medium text-gray-900 sticky left-0 bg-inherit">
                    {opName}
                  </td>
                  {categories.map((cat) => {
                    const item = tariffMap.get(`${opId}_${cat}`);
                    if (!item) {
                      return (
                        <td key={cat} className="p-2 text-center">
                          <span className="text-gray-300 text-xs">&mdash;</span>
                        </td>
                      );
                    }
                    const clickable = !!item.plan_id && !!onNavigateToPlan;
                    return (
                      <td key={cat} className="p-2 text-center">
                        <div
                          className={`inline-flex flex-col items-center gap-1 ${clickable ? 'cursor-pointer hover:bg-blue-50 rounded px-2 py-1' : ''}`}
                          onClick={clickable ? () => onNavigateToPlan!(item.plan_id!) : undefined}
                        >
                          <span className="font-mono text-sm">{formatPrice(item.price)}</span>
                          <div className="flex items-center gap-1">
                            {sourceBadge(item.source)}
                            {item.strategy !== 'fixed' && (
                              <span className="text-[10px] text-gray-500">
                                {STRATEGY_LABELS[item.strategy] || item.strategy}
                              </span>
                            )}
                          </div>
                        </div>
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
