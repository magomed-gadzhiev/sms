import { useState, useEffect } from 'react';
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

export function NetworkTariffsPage() {
  const toast = useToast();
  const [subAccounts, setSubAccounts] = useState<SubAccountOption[]>([]);
  const [selectedSA, setSelectedSA] = useState('');
  const [tariffs, setTariffs] = useState<Tariff[]>([]);
  const [loading, setLoading] = useState(false);

  // Edit state
  const [editingPrices, setEditingPrices] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  // Copy modal
  const [showCopy, setShowCopy] = useState(false);
  const [copyFrom, setCopyFrom] = useState('');
  const [copying, setCopying] = useState(false);

  // Bulk price modal
  const [showBulkPrice, setShowBulkPrice] = useState(false);
  const [bulkPrice, setBulkPrice] = useState('');
  const [bulkSaving, setBulkSaving] = useState(false);

  useEffect(() => {
    subAccountsApi.list().then((r: any) => {
      const list = (r.sub_accounts || []).map((sa: any) => ({ id: sa.id, name: sa.name || sa.email }));
      setSubAccounts(list);
    }).catch(() => {});
  }, []);

  useEffect(() => {
    if (!selectedSA) {
      setTariffs([]);
      return;
    }
    setLoading(true);
    resellerApi.listTariffs({ sub_account_id: selectedSA })
      .then((r) => {
        const items = r.tariffs as Tariff[];
        setTariffs(items);
        const prices: Record<string, string> = {};
        items.forEach((t) => { prices[`${t.operator_id}_${t.sender_category}`] = t.price_per_sms; });
        setEditingPrices(prices);
      })
      .catch(() => toast.error('Ошибка загрузки тарифов'))
      .finally(() => setLoading(false));
  }, [selectedSA]);

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
        return;
      }
      await resellerApi.upsertTariffs({ sub_account_id: selectedSA, tariffs: tariffList });
      toast.success(`Сохранено ${tariffList.length} тарифов`);
      // Reload
      const r = await resellerApi.listTariffs({ sub_account_id: selectedSA });
      const items = r.tariffs as Tariff[];
      setTariffs(items);
      const prices: Record<string, string> = {};
      items.forEach((t) => { prices[`${t.operator_id}_${t.sender_category}`] = t.price_per_sms; });
      setEditingPrices(prices);
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
      // Reload
      const r = await resellerApi.listTariffs({ sub_account_id: selectedSA });
      const items = r.tariffs as Tariff[];
      setTariffs(items);
      const prices: Record<string, string> = {};
      items.forEach((t) => { prices[`${t.operator_id}_${t.sender_category}`] = t.price_per_sms; });
      setEditingPrices(prices);
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка копирования');
    } finally {
      setCopying(false);
    }
  }

  async function handleBulkPrice() {
    if (!selectedSA || !bulkPrice) return;
    setBulkSaving(true);
    try {
      // Get all operators from references
      const resp = await apiFetch<{ operators: { id: string }[] }>('/references/operators');
      const operators = resp.operators || [];
      const tariffList = operators.map((op: any) => ({
        operator_id: op.id,
        sender_category: 'standard',
        price_per_sms: bulkPrice,
      }));
      await resellerApi.upsertTariffs({ sub_account_id: selectedSA, tariffs: tariffList });
      toast.success(`Установлена единая цена для ${tariffList.length} операторов`);
      setShowBulkPrice(false);
      // Reload
      const r = await resellerApi.listTariffs({ sub_account_id: selectedSA });
      const items = r.tariffs as Tariff[];
      setTariffs(items);
      const prices: Record<string, string> = {};
      items.forEach((t) => { prices[`${t.operator_id}_${t.sender_category}`] = t.price_per_sms; });
      setEditingPrices(prices);
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    } finally {
      setBulkSaving(false);
    }
  }

  return (
    <div className="max-w-5xl">
      <PageHeader title="Тарифы субаккаунтов" />

      <div className="flex items-center gap-3 mb-4">
        <select
          value={selectedSA}
          onChange={(e) => setSelectedSA(e.target.value)}
          className="border border-gray-300 rounded px-3 py-2 text-sm min-w-[250px]"
        >
          <option value="">Выберите субаккаунт</option>
          {subAccounts.map((sa) => (
            <option key={sa.id} value={sa.id}>{sa.name}</option>
          ))}
        </select>
        {selectedSA && (
          <>
            <Button variant="secondary" size="sm" onClick={() => setShowCopy(true)}>Скопировать из...</Button>
            <Button variant="secondary" size="sm" onClick={() => { setShowBulkPrice(true); setBulkPrice(''); }}>Единая цена</Button>
          </>
        )}
      </div>

      {!selectedSA ? (
        <div className="py-12 text-center text-gray-400">Выберите субаккаунт для настройки тарифов</div>
      ) : loading ? (
        <div className="py-8 text-center text-gray-400">Загрузка...</div>
      ) : (
        <>
          {tariffs.length === 0 ? (
            <div className="py-8 text-center text-gray-400">
              <p className="mb-2">Тарифы не настроены</p>
              <p className="text-sm">Используйте «Единая цена» или «Скопировать из...» для быстрой настройки</p>
            </div>
          ) : (
            <table className="w-full text-sm bg-white rounded-lg border border-gray-200">
              <thead>
                <tr className="bg-gray-50 text-gray-500 text-xs uppercase">
                  <th className="text-left p-3">Оператор</th>
                  <th className="text-left p-3">Категория</th>
                  <th className="text-left p-3">Цена за SMS (руб.)</th>
                </tr>
              </thead>
              <tbody>
                {tariffs.map((t) => (
                  <tr key={t.id} className="border-t border-gray-100">
                    <td className="p-3 font-medium">{t.operator_name}</td>
                    <td className="p-3 text-gray-500">{t.sender_category}</td>
                    <td className="p-3">
                      <input
                        type="text"
                        value={editingPrices[`${t.operator_id}_${t.sender_category}`] ?? t.price_per_sms}
                        onChange={(e) => setEditingPrices((prev) => ({ ...prev, [`${t.operator_id}_${t.sender_category}`]: e.target.value }))}
                        className="border border-gray-300 rounded px-2 py-1 w-32 text-sm"
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}

          {tariffs.length > 0 && (
            <div className="mt-4">
              <Button onClick={handleSave} disabled={saving}>
                {saving ? 'Сохранение...' : 'Сохранить тарифы'}
              </Button>
            </div>
          )}
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
              className="border border-gray-300 rounded px-3 py-2 text-sm w-full"
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
