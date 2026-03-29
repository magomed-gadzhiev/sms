import { useState, useEffect, useCallback } from 'react';
import { tariffsApi, ApiError, type TariffPlanInfo, type CurrentPlan } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';

function formatPrice(rub: number): string {
  return new Intl.NumberFormat('ru-RU', {
    style: 'currency',
    currency: 'RUB',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(rub);
}

function formatNumber(n: number): string {
  return new Intl.NumberFormat('ru-RU').format(n);
}

/**
 * Russian pluralization: picks the correct word form based on count.
 * forms: [singular, genitive singular, genitive plural]
 * e.g. pluralizeRu(1, ['подключение', 'подключения', 'подключений']) => 'подключение'
 */
function pluralizeRu(n: number, forms: [string, string, string]): string {
  const abs = Math.abs(n);
  const mod10 = abs % 10;
  const mod100 = abs % 100;
  if (mod10 === 1 && mod100 !== 11) return forms[0];
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return forms[1];
  return forms[2];
}

export function TariffsPage() {
  const [current, setCurrent] = useState<CurrentPlan | null>(null);
  const [plans, setPlans] = useState<TariffPlanInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Switch plan dialog
  const [switchTarget, setSwitchTarget] = useState<TariffPlanInfo | null>(null);
  const [switching, setSwitching] = useState(false);
  const [switchError, setSwitchError] = useState('');

  const loadData = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [currentResp, plansResp] = await Promise.all([
        tariffsApi.getCurrent(),
        tariffsApi.listPlans(),
      ]);
      setCurrent(currentResp);
      setPlans(plansResp.plans || []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить тарифы');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadData();
  }, [loadData]);

  async function handleSwitchPlan() {
    if (!switchTarget) return;
    setSwitching(true);
    setSwitchError('');
    try {
      await tariffsApi.switchPlan(switchTarget.name);
      setSwitchTarget(null);
      await loadData();
    } catch (err) {
      setSwitchError(err instanceof ApiError ? err.message : 'Не удалось сменить тариф');
    } finally {
      setSwitching(false);
    }
  }

  if (loading) {
    return (
      <div>
        <PageHeader title="Тарифы" />
        <div className="animate-pulse p-8 text-center text-gray-500">Загрузка...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div>
        <PageHeader title="Тарифы" />
        <div className="text-red-600 p-4">{error}</div>
      </div>
    );
  }

  const usagePercent = current?.plan
    ? Math.min(
        100,
        (current.plan.max_sms_per_month ?? 0) > 0
          ? Math.round(((current.monthly_sms_count ?? 0) / current.plan.max_sms_per_month) * 100)
          : 0,
      )
    : 0;

  return (
    <div>
      <PageHeader title="Тарифы" subtitle="Управление тарифным планом" />

      {/* Current plan card */}
      {current?.plan && (
        <div className="bg-white border-2 border-primary/30 rounded-lg p-6 mb-8">
          <div className="flex items-center justify-between mb-4">
            <div>
              <span className="text-xs font-medium text-primary uppercase tracking-wide">
                Текущий план
              </span>
              <h2 className="text-xl font-semibold text-gray-900 mt-1">
                {current.plan.display_name}
              </h2>
            </div>
            <div className="text-right">
              <span className="text-2xl font-bold text-gray-900">
                {formatPrice(current.plan.monthly_price_rub)}
              </span>
              <span className="text-sm text-gray-500"> / мес</span>
            </div>
          </div>

          {/* Limits */}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-5">
            <div className="bg-gray-50 rounded-lg p-3">
              <div className="text-xs text-gray-500 mb-1">SMS в месяц</div>
              <div className="text-sm font-semibold text-gray-900">
                {formatNumber(current.plan.max_sms_per_month)}
              </div>
            </div>
            <div className="bg-gray-50 rounded-lg p-3">
              <div className="text-xs text-gray-500 mb-1">SMPP подключения</div>
              <div className="text-sm font-semibold text-gray-900">
                {formatNumber(current.plan.max_smpp_connections)}
              </div>
            </div>
            <div className="bg-gray-50 rounded-lg p-3">
              <div className="text-xs text-gray-500 mb-1">Пользователи</div>
              <div className="text-sm font-semibold text-gray-900">
                {formatNumber(current.plan.max_users)}
              </div>
            </div>
          </div>

          {/* SMS usage progress */}
          <div>
            <div className="flex justify-between text-sm mb-1.5">
              <span className="text-gray-600">Использовано SMS</span>
              <span className="text-gray-900 font-medium">
                {formatNumber(current.monthly_sms_count)} / {formatNumber(current.plan.max_sms_per_month)}
              </span>
            </div>
            <div className="w-full bg-gray-200 rounded-full h-3">
              <div
                className={`h-3 rounded-full transition-all duration-500 ${
                  usagePercent >= 90
                    ? 'bg-red-500'
                    : usagePercent >= 70
                      ? 'bg-yellow-500'
                      : 'bg-green-500'
                }`}
                style={{ width: `${usagePercent}%` }}
              />
            </div>
            <div className="text-xs text-gray-400 mt-1 text-right">{usagePercent}%</div>
          </div>
        </div>
      )}

      {/* Available plans */}
      <h2 className="text-lg font-semibold text-gray-900 mb-4">Доступные планы</h2>

      {switchError && (
        <div className="text-red-600 text-sm mb-4 p-3 bg-red-50 rounded">{switchError}</div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
        {plans.map((plan) => {
          const isCurrent = current?.plan?.name === plan.name;
          return (
            <div
              key={plan.name}
              className={`bg-white border rounded-lg p-5 flex flex-col transition-shadow hover:shadow-md ${
                isCurrent
                  ? 'border-primary ring-2 ring-primary/20'
                  : 'border-gray-200'
              }`}
            >
              {/* Plan header */}
              <div className="mb-4">
                {isCurrent && (
                  <span className="inline-block text-[10px] font-semibold uppercase tracking-wider text-primary bg-primary/10 rounded-full px-2 py-0.5 mb-2">
                    Текущий
                  </span>
                )}
                <h3 className="text-lg font-semibold text-gray-900">{plan.display_name}</h3>
                <div className="mt-1">
                  <span className="text-2xl font-bold text-gray-900">
                    {formatPrice(plan.monthly_price_rub)}
                  </span>
                  <span className="text-sm text-gray-500"> / мес</span>
                </div>
              </div>

              {/* Plan limits */}
              <ul className="text-sm text-gray-600 space-y-2 mb-5 flex-1">
                <li className="flex items-center gap-2">
                  <span className="text-green-500 text-base leading-none">{'\u2713'}</span>
                  {formatNumber(plan.max_sms_per_month)} SMS / мес
                </li>
                <li className="flex items-center gap-2">
                  <span className="text-green-500 text-base leading-none">{'\u2713'}</span>
                  {formatNumber(plan.max_smpp_connections)} SMPP {pluralizeRu(plan.max_smpp_connections, ['подключение', 'подключения', 'подключений'])}
                </li>
                <li className="flex items-center gap-2">
                  <span className="text-green-500 text-base leading-none">{'\u2713'}</span>
                  {formatNumber(plan.max_users)} {pluralizeRu(plan.max_users, ['пользователь', 'пользователя', 'пользователей'])}
                </li>
                {(() => {
                  const feats = plan.features;
                  if (!feats) return null;
                  // API returns either string[] or Record<string, boolean>
                  const items = Array.isArray(feats)
                    ? feats
                    : Object.entries(feats).filter(([, v]) => v).map(([k]) => k);
                  const featureLabels: Record<string, string> = {
                    analytics: 'Аналитика',
                    hlr: 'HLR проверки',
                    smart_routing: 'Умная маршрутизация',
                    sub_accounts: 'Суб-аккаунты',
                    webhooks: 'Вебхуки',
                    white_label: 'White Label',
                  };
                  return items.map((feat) => (
                    <li key={feat} className="flex items-center gap-2">
                      <span className="text-green-500 text-base leading-none">{'\u2713'}</span>
                      {featureLabels[feat] || feat}
                    </li>
                  ));
                })()}
              </ul>

              {/* Action */}
              <Button
                variant={isCurrent ? 'secondary' : 'primary'}
                disabled={isCurrent}
                className="w-full"
                onClick={() => !isCurrent && setSwitchTarget(plan)}
              >
                {isCurrent ? 'Текущий план' : 'Выбрать'}
              </Button>
            </div>
          );
        })}
      </div>

      {plans.length === 0 && (
        <div className="text-center text-gray-400 py-12">Нет доступных планов</div>
      )}

      {/* Confirm switch dialog */}
      <ConfirmDialog
        open={!!switchTarget}
        onConfirm={handleSwitchPlan}
        onCancel={() => setSwitchTarget(null)}
        title="Сменить тарифный план"
        description={
          switchTarget
            ? `Вы уверены, что хотите перейти на план "${switchTarget.display_name}" (${formatPrice(switchTarget.monthly_price_rub)}/мес)?`
            : ''
        }
        confirmLabel="Подтвердить"
        loading={switching}
      />
    </div>
  );
}
