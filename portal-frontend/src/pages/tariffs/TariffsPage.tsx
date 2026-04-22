// Client self-view of their effective network tariff matrix (Task 23).
// Replaces the legacy subscription-plan picker. Reuses the shared TariffMatrix
// component in read-only mode (scope.kind='client'), with no inheritance
// markers and no period history — the backend returns only the currently
// active period.
//
// Adaptation: the client endpoint's response shape is narrower than the
// editor's TariffEditorData (no price_template/price_override/source,
// single `period` instead of `periods[]` + active_period_id). We widen it
// here before handing to the matrix — template price is set to `effective`,
// source='template', so the matrix's inheritance code paths are well-fed
// but are disabled at render-time via showInheritance=false.

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  clientTariffsApi,
  ApiError,
  type ClientTariffsEffectiveResponse,
  type TariffEditorData,
} from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { TariffMatrix } from '../../components/tariffs/TariffMatrix';

interface FilterParams {
  channel: string;
  country: string;
  sender_category: string;
  traffic_type: string;
}

const SENDER_CATEGORIES: { value: string; label: string }[] = [
  { value: 'paid_registered', label: 'Платный (зарег.)' },
  { value: 'paid_unregistered', label: 'Платный (незарег.)' },
  { value: 'free', label: 'Бесплатный' },
];

const TRAFFIC_TYPES: { value: string; label: string }[] = [
  { value: 'any', label: 'Любой' },
  { value: 'service', label: 'Сервисный' },
  { value: 'advertising', label: 'Рекламный' },
];

const COUNTRIES: { value: string; label: string }[] = [
  { value: 'RU', label: 'Россия' },
  { value: 'KZ', label: 'Казахстан' },
  { value: 'BY', label: 'Беларусь' },
];

// adaptToEditorData widens the client-endpoint response to TariffEditorData
// so the shared TariffMatrix can render it without a separate code path.
// With showInheritance=false the matrix ignores price_template/price_override
// distinctions — we fill them identically to `effective`.
function adaptToEditorData(r: ClientTariffsEffectiveResponse): TariffEditorData | null {
  if (!r.plan || !r.period) return null;
  return {
    scope: { kind: 'template' }, // unused visually; scope prop below is {kind:'client'}
    template: null,
    plan: r.plan,
    periods: [{ id: r.period.id, from: r.period.from, to: r.period.to, active: true }],
    active_period_id: r.period.id,
    operators: r.operators,
    tiers: r.tiers,
    cells: r.cells.map((c) => ({
      operator_id: c.operator_id,
      tier_id: c.tier_id,
      price_template: c.effective,
      price_override: null,
      effective: c.effective,
      source: c.effective == null ? 'unset' : 'template',
    })),
  };
}

export function TariffsPage() {
  const [params, setParams] = useState<FilterParams>({
    channel: 'sms',
    country: 'RU',
    sender_category: 'paid_registered',
    traffic_type: 'any',
  });

  const [raw, setRaw] = useState<ClientTariffsEffectiveResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const r = await clientTariffsApi.getEffective(params);
      setRaw(r);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось загрузить тарифы');
    } finally {
      setLoading(false);
    }
  }, [params]);

  useEffect(() => {
    load();
  }, [load]);

  const data = useMemo(() => (raw ? adaptToEditorData(raw) : null), [raw]);

  return (
    <div>
      <PageHeader title="Тарифы" subtitle="Действующие цены по операторам" />

      <div className="flex flex-wrap items-center gap-3 mb-4">
        <FilterSelect
          label="Страна"
          value={params.country}
          options={COUNTRIES}
          onChange={(v) => setParams((p) => ({ ...p, country: v }))}
        />
        <FilterSelect
          label="Тип имени"
          value={params.sender_category}
          options={SENDER_CATEGORIES}
          onChange={(v) => setParams((p) => ({ ...p, sender_category: v }))}
        />
        <FilterSelect
          label="Тип трафика"
          value={params.traffic_type}
          options={TRAFFIC_TYPES}
          onChange={(v) => setParams((p) => ({ ...p, traffic_type: v }))}
        />
      </div>

      {loading && !data ? (
        <div className="animate-pulse p-8 text-center text-gray-500">Загрузка...</div>
      ) : error ? (
        <div className="text-red-600 p-4">{error}</div>
      ) : !data ? (
        <div className="text-center text-gray-400 py-12">
          Тариф для вашего аккаунта не настроен.
        </div>
      ) : (
        <div>
          {raw?.period && (
            <div className="text-xs text-slate-500 mb-2">
              Период: {raw.period.from}
              {raw.period.to ? ` — ${raw.period.to}` : ' — по настоящее время'}
              {raw.plan ? ` · Валюта: ${raw.plan.currency}` : ''}
            </div>
          )}
          <TariffMatrix
            data={data}
            scope={{ kind: 'client' }}
            editable={false}
            showInheritance={false}
            currency={raw?.plan?.currency ?? 'RUB'}
          />
        </div>
      )}
    </div>
  );
}

interface FilterSelectProps {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
}

function FilterSelect({ label, value, options, onChange }: FilterSelectProps) {
  return (
    <label className="flex items-center gap-2 text-sm">
      <span className="text-slate-600">{label}:</span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="rounded border border-slate-300 px-2 py-1 text-sm bg-white"
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </label>
  );
}
