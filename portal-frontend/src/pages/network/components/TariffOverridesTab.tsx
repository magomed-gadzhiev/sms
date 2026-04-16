import { useState, useEffect, useCallback } from 'react';
import { useToast } from '../../../components/ui/Toast';
import {
  resellerTariffApi,
  subAccountsApi,
  type ResellerTariffPlan,
  ApiError,
} from '../../../api/client';
import { TariffPlanEditor } from './TariffPlanEditor';

interface SubAccountOption {
  id: string;
  name: string;
}

export function TariffOverridesTab() {
  const toast = useToast();
  const [subAccounts, setSubAccounts] = useState<SubAccountOption[]>([]);
  const [selectedSA, setSelectedSA] = useState('');
  const [plans, setPlans] = useState<ResellerTariffPlan[]>([]);
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

  const loadPlans = useCallback(
    (saId: string) => {
      setLoading(true);
      resellerTariffApi
        .listPlans({ sub_account_id: saId })
        .then((r) => setPlans(r.plans || []))
        .catch((e) =>
          toast.error(e instanceof ApiError ? e.message : 'Ошибка загрузки планов'),
        )
        .finally(() => setLoading(false));
    },
    [],
  );

  useEffect(() => {
    if (!selectedSA) {
      setPlans([]);
      return;
    }
    loadPlans(selectedSA);
  }, [selectedSA, loadPlans]);

  return (
    <div>
      {/* Sub-account selector */}
      <div className="flex flex-col gap-1 mb-6">
        <label className="text-xs font-medium text-gray-500" htmlFor="overrides-sa-select">
          Субаккаунт
        </label>
        <select
          id="overrides-sa-select"
          value={selectedSA}
          onChange={(e) => setSelectedSA(e.target.value)}
          disabled={saLoading}
          className="border border-gray-300 rounded px-3 py-2 text-sm min-w-[250px] max-w-sm focus:ring-2 focus:ring-primary/50 focus:border-primary"
        >
          <option value="">Выберите субаккаунт</option>
          {subAccounts.map((sa) => (
            <option key={sa.id} value={sa.id}>
              {sa.name}
            </option>
          ))}
        </select>
      </div>

      {/* Content */}
      {!selectedSA ? (
        <div className="py-16 text-center">
          <div className="text-gray-400 text-sm">
            Выберите субаккаунт для управления переопределениями тарифов
          </div>
        </div>
      ) : loading ? (
        <div className="space-y-3">
          <div className="h-10 bg-gray-100 rounded animate-pulse" />
          <div className="h-32 bg-gray-100 rounded animate-pulse" />
        </div>
      ) : (
        <TariffPlanEditor
          plans={plans}
          ownerId={selectedSA}
          ownerType="sub_account"
          onRefresh={() => loadPlans(selectedSA)}
        />
      )}
    </div>
  );
}
