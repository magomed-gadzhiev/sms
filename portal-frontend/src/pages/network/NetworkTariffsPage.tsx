import { useState, useEffect, useMemo } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { useToast } from '../../components/ui/Toast';
import { resellerApi, subAccountsApi, apiFetch, ApiError } from '../../api/client';

interface Tariff {
  id: string;
  sub_account_id: string | null;
  operator_id: string;
  operator_name: string;
  sender_category: string;
  price_per_sms: string;
  active: boolean;
}

interface SubAccountOption {
  id: string;
  name: string;
}

const CATEGORY_LABELS: Record<string, string> = {
  paid_registered: 'Платная регистрация',
  free_registered: 'Бесплатная регистрация',
  shared: 'Общая',
  standard: 'Стандартная',
};

const CATEGORY_SHORT: Record<string, string> = {
  paid_registered: 'Платная рег.',
  free_registered: 'Бесплатная рег.',
  shared: 'Общая',
  standard: 'Стандартная',
};

function formatPrice(raw: string): string {
  const n = parseFloat(raw);
  if (isNaN(n)) return raw;
  return n.toFixed(2);
}

export function NetworkTariffsPage() {
  const toast = useToast();
  const [subAccounts, setSubAccounts] = useState<SubAccountOption[]>([]);
  const [subAccountsError, setSubAccountsError] = useState(false);
  const [selectedSA, setSelectedSA] = useState('');
  const [tariffs, setTariffs] = useState<Tariff[]>([]);
  const [loading, setLoading] = useState(false);

  // Edit state: key = `${operator_id}_${category}`, value = price string
  const [editingPrices, setEditingPrices] = useState<Record<string, string>>({});
  // Original prices for dirty tracking
  const [originalPrices, setOriginalPrices] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  // Copy modal
  const [showCopy, setShowCopy] = useState(false);
  const [copyFrom, setCopyFrom] = useState('');
  const [copying, setCopying] = useState(false);

  // Bulk price modal
  const [showBulkPrice, setShowBulkPrice] = useState(false);
  const [bulkPrice, setBulkPrice] = useState('');
  const [bulkCategory, setBulkCategory] = useState('all');
  const [bulkSaving, setBulkSaving] = useState(false);

  function loadSubAccounts() {
    setSubAccountsError(false);
    subAccountsApi.list().then((r: any) => {
      const list = (r.sub_accounts || []).map((sa: any) => ({ id: sa.id, name: sa.name || sa.email }));
      setSubAccounts(list);
    }).catch(() => setSubAccountsError(true));
  }

  useEffect(() => {
    loadSubAccounts();
  }, []);

  function applyTariffs(items: Tariff[]) {
    setTariffs(items);
    const prices: Record<string, string> = {};
    items.forEach((t) => { prices[`${t.operator_id}_${t.sender_category}`] = formatPrice(t.price_per_sms); });
    setEditingPrices(prices);
    setOriginalPrices({ ...prices });
  }

  useEffect(() => {
    if (!selectedSA) {
      setTariffs([]);
      setEditingPrices({});
      setOriginalPrices({});
      return;
    }
    setLoading(true);
    resellerApi.listTariffs({ sub_account_id: selectedSA })
      .then((r) => applyTariffs(r.tariffs as Tariff[]))
      .catch(() => toast.error('Ошибка загрузки тарифов'))
      .finally(() => setLoading(false));
  }, [selectedSA]);

  // Matrix: unique operators and categories
  const { operators, categories } = useMemo(() => {
    const opMap = new Map<string, string>();
    const catSet = new Set<string>();
    tariffs.forEach((t) => {
      opMap.set(t.operator_id, t.operator_name);
      catSet.add(t.sender_category);
    });
    // Sort categories in a logical order
    const catOrder = ['paid_registered', 'free_registered', 'shared', 'standard'];
    const cats = [...catSet].sort((a, b) => {
      const ai = catOrder.indexOf(a);
      const bi = catOrder.indexOf(b);
      return (ai === -1 ? 99 : ai) - (bi === -1 ? 99 : bi);
    });
    // Sort operators alphabetically, but Default first
    const ops = [...opMap.entries()].sort((a, b) => {
      if (a[1].startsWith('Default')) return -1;
      if (b[1].startsWith('Default')) return 1;
      return a[1].localeCompare(b[1], 'ru');
    });
    return { operators: ops, categories: cats };
  }, [tariffs]);

  // Count dirty (changed) cells
  const dirtyCount = useMemo(() => {
    let count = 0;
    for (const key of Object.keys(editingPrices)) {
      if (editingPrices[key] !== originalPrices[key]) count++;
    }
    return count;
  }, [editingPrices, originalPrices]);

  async function handleSave() {
    if (!selectedSA) return;
    setSaving(true);
    try {
      const tariffList = tariffs
        .map((t) => {
          const price = editingPrices[`${t.operator_id}_${t.sender_category}`];
          return price !== undefined && price !== ''
            ? { operator_id: t.operator_id, sender_category: t.sender_category, price_per_sms: price }
            : null;
        })
        .filter(Boolean) as { operator_id: string; sender_category: string; price_per_sms: string }[];
      if (tariffList.length === 0) {
        toast.error('Нет тарифов для сохранения');
        setSaving(false);
        return;
      }
      const invalidPrice = tariffList.find((t) => {
        const n = parseFloat(t.price_per_sms);
        return isNaN(n) || n < 0;
      });
      if (invalidPrice) {
        toast.error('Цена должна быть неотрицательным числом');
        setSaving(false);
        return;
      }
      await resellerApi.upsertTariffs({ sub_account_id: selectedSA, tariffs: tariffList });
      toast.success(`Сохранено ${tariffList.length} тарифов`);
      const r = await resellerApi.listTariffs({ sub_account_id: selectedSA });
      applyTariffs(r.tariffs as Tariff[]);
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка сохранения');
    } finally {
      setSaving(false);
    }
  }

  async function handleCopy() {
    if (!copyFrom || !selectedSA) return;
    setCopying(true);
    try {
      const result = await resellerApi.copyTariffs(copyFrom, selectedSA);
      toast.success(`Скопировано тарифов: ${(result as any).copied}`);
      setShowCopy(false);
      const r = await resellerApi.listTariffs({ sub_account_id: selectedSA });
      applyTariffs(r.tariffs as Tariff[]);
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка копирования');
    } finally {
      setCopying(false);
    }
  }

  async function handleBulkPrice() {
    if (!selectedSA || !bulkPrice) return;
    const n = parseFloat(bulkPrice);
    if (isNaN(n) || n < 0) {
      toast.error('Цена должна быть неотрицательным числом');
      return;
    }
    setBulkSaving(true);
    try {
      const resp = await apiFetch<{ operators: { id: string }[] }>('/references/operators');
      const ops = resp.operators || [];
      const categoriesToSet = bulkCategory === 'all' ? ['standard'] : [bulkCategory];
      const tariffList = ops.flatMap((op: any) =>
        categoriesToSet.map((cat) => ({
          operator_id: op.id,
          sender_category: cat,
          price_per_sms: bulkPrice,
        }))
      );
      await resellerApi.upsertTariffs({ sub_account_id: selectedSA, tariffs: tariffList });
      toast.success(`Установлена единая цена для ${ops.length} операторов`);
      setShowBulkPrice(false);
      const r = await resellerApi.listTariffs({ sub_account_id: selectedSA });
      applyTariffs(r.tariffs as Tariff[]);
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    } finally {
      setBulkSaving(false);
    }
  }

  function handleResetChanges() {
    setEditingPrices({ ...originalPrices });
  }

  return (
    <div className="max-w-6xl">
      <PageHeader title="Тарифы субаккаунтов" />

      {subAccountsError && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded text-sm text-red-700 flex items-center gap-2">
          Не удалось загрузить список субаккаунтов.
          <button onClick={loadSubAccounts} className="underline font-medium">Повторить</button>
        </div>
      )}

      {/* Toolbar */}
      <div className="flex items-center gap-3 mb-6 flex-wrap">
        <div className="flex flex-col gap-1">
          <label className="text-xs font-medium text-gray-500" htmlFor="sa-select">Субаккаунт</label>
          <select
            id="sa-select"
            value={selectedSA}
            onChange={(e) => setSelectedSA(e.target.value)}
            className="border border-gray-300 rounded px-3 py-2 text-sm min-w-[250px] focus:ring-2 focus:ring-primary/50 focus:border-primary"
          >
            <option value="">Выберите субаккаунт</option>
            {subAccounts.map((sa) => (
              <option key={sa.id} value={sa.id}>{sa.name}</option>
            ))}
          </select>
        </div>
        {selectedSA && (
          <div className="flex items-end gap-2">
            <Button variant="secondary" size="sm" onClick={() => setShowCopy(true)}>Скопировать из...</Button>
            <Button variant="secondary" size="sm" onClick={() => { setShowBulkPrice(true); setBulkPrice(''); setBulkCategory('all'); }}>Единая цена</Button>
          </div>
        )}
      </div>

      {/* Content */}
      {!selectedSA ? (
        <div className="py-16 text-center">
          <div className="text-gray-400 text-sm">Выберите субаккаунт для настройки тарифов</div>
        </div>
      ) : loading ? (
        <div className="space-y-3">
          <div className="h-10 bg-gray-100 rounded animate-pulse" />
          <div className="h-64 bg-gray-100 rounded animate-pulse" />
        </div>
      ) : tariffs.length === 0 ? (
        <div className="py-16 text-center border border-gray-200 rounded-lg bg-white">
          <div className="text-gray-400 mb-2">Тарифы не настроены</div>
          <div className="text-sm text-gray-400 mb-4">Используйте кнопки выше для быстрой настройки</div>
          <div className="flex justify-center gap-2">
            <Button variant="secondary" size="sm" onClick={() => { setShowBulkPrice(true); setBulkPrice(''); setBulkCategory('all'); }}>Единая цена</Button>
            <Button variant="secondary" size="sm" onClick={() => setShowCopy(true)}>Скопировать из...</Button>
          </div>
        </div>
      ) : (
        <>
          {/* Summary bar */}
          <div className="flex items-center justify-between mb-3">
            <div className="text-xs text-gray-500">
              {operators.length} {operators.length === 1 ? 'оператор' : operators.length < 5 ? 'оператора' : 'операторов'} &middot; {categories.length} {categories.length === 1 ? 'категория' : categories.length < 5 ? 'категории' : 'категорий'}
            </div>
            {dirtyCount > 0 && (
              <div className="flex items-center gap-3">
                <span className="text-xs text-amber-600 font-medium">
                  {dirtyCount} {dirtyCount === 1 ? 'изменение' : dirtyCount < 5 ? 'изменения' : 'изменений'}
                </span>
                <button
                  onClick={handleResetChanges}
                  className="text-xs text-gray-500 hover:text-gray-700 underline"
                >
                  Сбросить
                </button>
              </div>
            )}
          </div>

          {/* Matrix table */}
          <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white shadow-sm">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-gray-50">
                  <th className="text-left p-3 text-xs font-semibold text-gray-500 uppercase tracking-wide sticky left-0 bg-gray-50 min-w-[160px]">
                    Оператор
                  </th>
                  {categories.map((cat) => (
                    <th key={cat} className="text-center p-3 text-xs font-semibold text-gray-500 uppercase tracking-wide min-w-[140px]">
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
                      const key = `${opId}_${cat}`;
                      const value = editingPrices[key];
                      const isDirty = value !== undefined && value !== originalPrices[key];
                      const hasValue = value !== undefined && value !== '';

                      if (!hasValue && originalPrices[key] === undefined) {
                        // No tariff for this operator+category
                        return (
                          <td key={cat} className="p-2 text-center">
                            <span className="text-gray-300 text-xs">&mdash;</span>
                          </td>
                        );
                      }

                      return (
                        <td key={cat} className="p-2 text-center">
                          <div className="relative inline-block">
                            <input
                              type="text"
                              inputMode="decimal"
                              value={value ?? ''}
                              onChange={(e) => setEditingPrices((prev) => ({ ...prev, [key]: e.target.value }))}
                              className={`border rounded px-2 py-1.5 w-24 text-sm text-center transition-colors
                                ${isDirty
                                  ? 'border-amber-400 bg-amber-50 ring-1 ring-amber-200'
                                  : 'border-gray-300 hover:border-gray-400'
                                }
                                focus:ring-2 focus:ring-primary/50 focus:border-primary`}
                              aria-label={`${opName} — ${CATEGORY_LABELS[cat] || cat}`}
                            />
                          </div>
                        </td>
                      );
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Save bar */}
          <div className="sticky bottom-0 mt-4 py-3 bg-white/95 backdrop-blur border-t border-gray-200 -mx-6 px-6 flex items-center gap-3">
            <Button onClick={handleSave} disabled={saving}>
              {saving ? 'Сохранение...' : 'Сохранить тарифы'}
            </Button>
            {dirtyCount > 0 && (
              <span className="text-xs text-gray-500">
                {dirtyCount} несохранённых {dirtyCount === 1 ? 'изменение' : dirtyCount < 5 ? 'изменения' : 'изменений'}
              </span>
            )}
          </div>
        </>
      )}

      {/* Copy modal */}
      <Modal open={showCopy} onClose={() => setShowCopy(false)} title="Скопировать тарифы">
        <div className="flex flex-col gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Из субаккаунта</label>
            <select
              value={copyFrom}
              onChange={(e) => setCopyFrom(e.target.value)}
              className="border border-gray-300 rounded px-3 py-2 text-sm w-full focus:ring-2 focus:ring-primary/50"
            >
              <option value="">Выберите источник</option>
              {subAccounts.filter((sa) => sa.id !== selectedSA).map((sa) => (
                <option key={sa.id} value={sa.id}>{sa.name}</option>
              ))}
            </select>
          </div>
          <div className="flex gap-2">
            <Button onClick={handleCopy} disabled={copying || !copyFrom}>
              {copying ? 'Копирование...' : 'Скопировать'}
            </Button>
            <Button variant="secondary" onClick={() => setShowCopy(false)}>Отмена</Button>
          </div>
        </div>
      </Modal>

      {/* Bulk price modal */}
      <Modal open={showBulkPrice} onClose={() => setShowBulkPrice(false)} title="Единая цена для всех операторов">
        <div className="flex flex-col gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Категория</label>
            <select
              value={bulkCategory}
              onChange={(e) => setBulkCategory(e.target.value)}
              className="border border-gray-300 rounded px-3 py-2 text-sm w-full focus:ring-2 focus:ring-primary/50"
            >
              <option value="all">Все категории</option>
              {Object.entries(CATEGORY_LABELS).map(([key, label]) => (
                <option key={key} value={key}>{label}</option>
              ))}
            </select>
          </div>
          <Input
            label="Цена за SMS (руб.)"
            value={bulkPrice}
            onChange={(e) => setBulkPrice(e.target.value)}
            placeholder="0.00"
            required
          />
          <div className="flex gap-2">
            <Button onClick={handleBulkPrice} disabled={bulkSaving || !bulkPrice}>
              {bulkSaving ? 'Установка...' : 'Установить'}
            </Button>
            <Button variant="secondary" onClick={() => setShowBulkPrice(false)}>Отмена</Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
