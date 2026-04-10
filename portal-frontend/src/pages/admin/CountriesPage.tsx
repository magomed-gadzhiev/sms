import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { Badge, StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { countriesApi, operatorsApi, type CountryInfo, type OperatorInfo, type OperatorPrefix } from '../../api/admin';

const PAGE_SIZE = 20;

type ViewMode = 'hierarchical' | 'flat';

const WIZARD_STEPS = ['Основная информация', 'Настройки'];

const emptyStep1 = { name: '', mcc: '', mnc: '' };
const emptyStep2 = { supports_paid_sender: false, supports_free_sender: true, monthly_tariff_amount: '' };

export function CountriesPage() {
  const toast = useToast();

  const [viewMode, setViewMode] = useState<ViewMode>('hierarchical');

  // Countries
  const [countries, setCountries] = useState<CountryInfo[]>([]);
  const [countryTotal, setCountryTotal] = useState(0);
  const [countryPage, setCountryPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [showCreateCountry, setShowCreateCountry] = useState(false);
  const [countryForm, setCountryForm] = useState({ name: '', code: '', phone_code: '' });

  // Selected
  const [selectedCountry, setSelectedCountry] = useState<CountryInfo | null>(null);
  const [operators, setOperators] = useState<OperatorInfo[]>([]);
  const [operatorsLoading, setOperatorsLoading] = useState(false);
  const [selectedOperator, setSelectedOperator] = useState<OperatorInfo | null>(null);
  const [prefixes, setPrefixes] = useState<OperatorPrefix[]>([]);
  const [prefixInput, setPrefixInput] = useState('');
  const [savingCountry, setSavingCountry] = useState(false);
  const [savingOperator, setSavingOperator] = useState(false);

  // Flat view
  const [allOperators, setAllOperators] = useState<(OperatorInfo & { country_name?: string })[]>([]);
  const [allOperatorsLoading, setAllOperatorsLoading] = useState(false);
  const [allOperatorsPage, setAllOperatorsPage] = useState(1);
  const [allOperatorsTotal, setAllOperatorsTotal] = useState(0);
  const [mccFilter, setMccFilter] = useState('');
  const [mncFilter, setMncFilter] = useState('');
  const [countryFilter, setCountryFilter] = useState('');

  // Wizard
  const [showWizard, setShowWizard] = useState(false);
  const [wizardStep, setWizardStep] = useState(0);
  const [wizardStep1, setWizardStep1] = useState(emptyStep1);
  const [wizardStep2, setWizardStep2] = useState(emptyStep2);

  const fetchCountries = useCallback(async () => {
    setLoading(true);
    try {
      const res = await countriesApi.list({ limit: PAGE_SIZE, offset: (countryPage - 1) * PAGE_SIZE });
      setCountries(res.countries || []);
      setCountryTotal(res.total);
    } catch {
      toast.error('Ошибка загрузки стран');
    } finally {
      setLoading(false);
    }
  }, [countryPage, toast]);

  useEffect(() => { fetchCountries(); }, [fetchCountries]);

  const fetchAllOperators = useCallback(async () => {
    setAllOperatorsLoading(true);
    try {
      const res = await operatorsApi.list({ limit: PAGE_SIZE, offset: (allOperatorsPage - 1) * PAGE_SIZE });
      const enriched = (res.operators || []).map((op) => ({
        ...op,
        country_name: countries.find((c) => c.country_id === op.country_id)?.name ?? op.country_id,
      }));
      setAllOperators(enriched);
      setAllOperatorsTotal(res.total);
    } catch {
      toast.error('Ошибка загрузки операторов');
    } finally {
      setAllOperatorsLoading(false);
    }
  }, [allOperatorsPage, countries, toast]);

  useEffect(() => {
    if (viewMode === 'flat') fetchAllOperators();
  }, [viewMode, fetchAllOperators]);

  const fetchOperators = useCallback(async (countryId: string) => {
    setOperatorsLoading(true);
    try {
      const res = await operatorsApi.list({ country_id: countryId, limit: 100, offset: 0 });
      setOperators(res.operators || []);
    } catch {
      toast.error('Ошибка загрузки операторов');
    } finally {
      setOperatorsLoading(false);
    }
  }, [toast]);

  const fetchPrefixes = useCallback(async (operatorId: string) => {
    try {
      const res = await operatorsApi.listPrefixes(operatorId);
      setPrefixes(res.prefixes || []);
    } catch {
      toast.error('Ошибка загрузки префиксов');
    }
  }, [toast]);

  const selectCountry = (country: CountryInfo) => {
    setSelectedCountry(country);
    setSelectedOperator(null);
    setPrefixes([]);
    fetchOperators(country.country_id);
  };

  const selectOperator = (op: OperatorInfo) => {
    setSelectedOperator(op);
    fetchPrefixes(op.operator_id);
  };

  const handleCreateCountry = async () => {
    setSavingCountry(true);
    try {
      await countriesApi.create(countryForm);
      toast.success('Страна создана');
      setShowCreateCountry(false);
      setCountryForm({ name: '', code: '', phone_code: '' });
      fetchCountries();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    } finally {
      setSavingCountry(false);
    }
  };

  const handleWizardFinish = async () => {
    if (!selectedCountry) return;
    setSavingOperator(true);
    try {
      const data: Partial<OperatorInfo> = {
        name: wizardStep1.name,
        mcc: wizardStep1.mcc,
        mnc: wizardStep1.mnc,
        supports_free_sender: wizardStep2.supports_free_sender,
        supports_paid_sender: wizardStep2.supports_paid_sender,
        monthly_tariff_amount: wizardStep2.supports_paid_sender ? wizardStep2.monthly_tariff_amount : undefined,
        country_id: selectedCountry.country_id,
        active: true,
      };
      await operatorsApi.create(data);
      toast.success('Оператор создан');
      setShowWizard(false);
      setWizardStep(0);
      setWizardStep1(emptyStep1);
      setWizardStep2(emptyStep2);
      fetchOperators(selectedCountry.country_id);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    } finally {
      setSavingOperator(false);
    }
  };

  const handleSaveOperatorSettings = async () => {
    if (!selectedOperator) return;
    setSavingOperator(true);
    try {
      await operatorsApi.update(selectedOperator.operator_id, {
        supports_paid_sender: selectedOperator.supports_paid_sender,
        supports_free_sender: selectedOperator.supports_free_sender,
        monthly_tariff_amount: selectedOperator.monthly_tariff_amount,
      });
      toast.success('Настройки сохранены');
      if (selectedCountry) fetchOperators(selectedCountry.country_id);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    } finally {
      setSavingOperator(false);
    }
  };

  const handleAddPrefix = async () => {
    if (!selectedOperator || !prefixInput) return;
    setSavingOperator(true);
    try {
      await operatorsApi.createPrefix(selectedOperator.operator_id, { prefix: prefixInput });
      toast.success('Префикс добавлен');
      setPrefixInput('');
      fetchPrefixes(selectedOperator.operator_id);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    } finally {
      setSavingOperator(false);
    }
  };

  const handleDeletePrefix = async (prefixId: string) => {
    if (!selectedOperator) return;
    try {
      await operatorsApi.deletePrefix(selectedOperator.operator_id, prefixId);
      toast.success('Префикс удалён');
      fetchPrefixes(selectedOperator.operator_id);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    }
  };

  const filteredOperators = allOperators.filter((op) => {
    if (mccFilter && !op.mcc.includes(mccFilter)) return false;
    if (mncFilter && !op.mnc.includes(mncFilter)) return false;
    if (countryFilter && !op.country_name?.toLowerCase().includes(countryFilter.toLowerCase())) return false;
    return true;
  });

  const countryColumns: Column<CountryInfo>[] = [
    { key: 'name', header: 'Страна', sortable: true },
    { key: 'code', header: 'Код' },
    { key: 'phone_code', header: 'Тел. код' },
  ];

  const operatorColumns: Column<OperatorInfo>[] = [
    { key: 'name', header: 'Оператор' },
    { key: 'mcc', header: 'MCC' },
    { key: 'mnc', header: 'MNC' },
    {
      key: 'active',
      header: 'Статус',
      render: (o) => <StatusBadge status={o.active ? 'active' : 'inactive'} />,
    },
  ];

  type EnrichedOperator = OperatorInfo & { country_name?: string };

  const flatColumns: Column<EnrichedOperator>[] = [
    { key: 'mcc', header: 'MCC' },
    { key: 'mnc', header: 'MNC' },
    { key: 'name', header: 'Оператор' },
    { key: 'country_name' as keyof EnrichedOperator, header: 'Страна', render: (o) => <span>{o.country_name}</span> },
    {
      key: 'supports_paid_sender',
      header: 'Платные',
      render: (o) => o.supports_paid_sender
        ? <Badge variant="success">Да</Badge>
        : <Badge variant="default">Нет</Badge>,
    },
    {
      key: 'active',
      header: 'Статус',
      render: (o) => <StatusBadge status={o.active ? 'active' : 'inactive'} />,
    },
  ];

  return (
    <>
      <PageHeader
        title="Справочник MCC/MNC"
        breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'MCC/MNC' }]}
      />

      {/* View toggle */}
      <div className="flex gap-1 mb-6 border-b border-gray-200">
        <button
          onClick={() => setViewMode('hierarchical')}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            viewMode === 'hierarchical'
              ? 'border-primary text-primary'
              : 'border-transparent text-gray-500 hover:text-gray-700'
          }`}
        >
          По странам
        </button>
        <button
          onClick={() => setViewMode('flat')}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            viewMode === 'flat'
              ? 'border-primary text-primary'
              : 'border-transparent text-gray-500 hover:text-gray-700'
          }`}
        >
          MCC/MNC таблица
        </button>
      </div>

      {/* Flat view */}
      {viewMode === 'flat' && (
        <>
          <div className="flex flex-wrap gap-3 mb-4">
            <Input label="MCC" value={mccFilter} onChange={(e) => setMccFilter(e.target.value)} placeholder="Фильтр MCC" />
            <Input label="MNC" value={mncFilter} onChange={(e) => setMncFilter(e.target.value)} placeholder="Фильтр MNC" />
            <Input label="Страна" value={countryFilter} onChange={(e) => setCountryFilter(e.target.value)} placeholder="Поиск по стране" />
          </div>
          <DataTable
            columns={flatColumns as unknown as Column<OperatorInfo>[]}
            data={filteredOperators as OperatorInfo[]}
            total={mccFilter || mncFilter || countryFilter ? filteredOperators.length : allOperatorsTotal}
            page={allOperatorsPage}
            pageSize={PAGE_SIZE}
            onPageChange={setAllOperatorsPage}
            loading={allOperatorsLoading}
            keyField="operator_id"
          />
        </>
      )}

      {/* Hierarchical view */}
      {viewMode === 'hierarchical' && (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Countries */}
          <div>
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-lg font-semibold">Страны</h2>
              <Button size="sm" onClick={() => { setCountryForm({ name: '', code: '', phone_code: '' }); setShowCreateCountry(true); }}>
                + Добавить
              </Button>
            </div>
            <DataTable
              columns={countryColumns}
              data={countries}
              total={countryTotal}
              page={countryPage}
              pageSize={PAGE_SIZE}
              onPageChange={setCountryPage}
              loading={loading}
              keyField="country_id"
              onRowClick={selectCountry}
            />
          </div>

          {/* Operators */}
          <div>
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-lg font-semibold">
                {selectedCountry ? `Операторы — ${selectedCountry.name}` : 'Операторы'}
              </h2>
              {selectedCountry && (
                <Button
                  size="sm"
                  onClick={() => {
                    setWizardStep(0);
                    setWizardStep1(emptyStep1);
                    setWizardStep2(emptyStep2);
                    setShowWizard(true);
                  }}
                >
                  + Добавить
                </Button>
              )}
            </div>
            {selectedCountry ? (
              <DataTable
                columns={operatorColumns}
                data={operators}
                total={operators.length}
                page={1}
                pageSize={100}
                onPageChange={() => {}}
                loading={operatorsLoading}
                keyField="operator_id"
                onRowClick={selectOperator}
              />
            ) : (
              <div className="text-sm text-gray-400 p-4 border border-gray-200 rounded-lg">Выберите страну</div>
            )}
          </div>

          {/* Operator detail */}
          <div>
            <h2 className="text-lg font-semibold mb-3">
              {selectedOperator ? selectedOperator.name : 'Детали оператора'}
            </h2>
            {selectedOperator ? (
              <>
                <div className="bg-white border border-gray-200 rounded-lg p-4 mb-4">
                  <h3 className="text-sm font-medium text-gray-700 mb-3">Настройки</h3>
                  <div className="space-y-2">
                    <label className="flex items-center gap-2 text-sm cursor-pointer">
                      <input
                        type="checkbox"
                        checked={selectedOperator.supports_free_sender ?? true}
                        onChange={(e) =>
                          setSelectedOperator({ ...selectedOperator, supports_free_sender: e.target.checked })
                        }
                        className="rounded"
                      />
                      Бесплатная регистрация имени
                    </label>
                    <label className="flex items-center gap-2 text-sm cursor-pointer">
                      <input
                        type="checkbox"
                        checked={selectedOperator.supports_paid_sender ?? false}
                        onChange={(e) =>
                          setSelectedOperator({
                            ...selectedOperator,
                            supports_paid_sender: e.target.checked,
                            monthly_tariff_amount: e.target.checked ? selectedOperator.monthly_tariff_amount : '',
                          })
                        }
                        className="rounded"
                      />
                      Платная регистрация имени
                    </label>
                    {selectedOperator.supports_paid_sender && (
                      <Input
                        label="Ежемесячный тариф (RUB)"
                        type="number"
                        min="0"
                        step="0.01"
                        value={selectedOperator.monthly_tariff_amount ?? ''}
                        onChange={(e) =>
                          setSelectedOperator({ ...selectedOperator, monthly_tariff_amount: e.target.value })
                        }
                        placeholder="1500.00"
                      />
                    )}
                  </div>
                  <div className="mt-3">
                    <Button size="sm" onClick={handleSaveOperatorSettings} disabled={savingOperator}>
                      Сохранить
                    </Button>
                  </div>
                </div>

                <div className="bg-white border border-gray-200 rounded-lg p-4">
                  <h3 className="text-sm font-medium text-gray-700 mb-3">Префиксы</h3>
                  <div className="flex gap-2 mb-3">
                    <Input value={prefixInput} onChange={(e) => setPrefixInput(e.target.value)} placeholder="+7921" />
                    <Button size="sm" onClick={handleAddPrefix} disabled={savingOperator || !prefixInput}>
                      Добавить
                    </Button>
                  </div>
                  <ul className="space-y-1">
                    {prefixes.map((p) => (
                      <li key={p.prefix_id} className="flex items-center justify-between py-1 px-2 bg-gray-50 rounded text-sm">
                        <span className="font-mono">{p.prefix}</span>
                        <button
                          className="text-xs text-danger hover:underline"
                          onClick={() => handleDeletePrefix(p.prefix_id)}
                        >
                          Удалить
                        </button>
                      </li>
                    ))}
                    {prefixes.length === 0 && <li className="text-sm text-gray-400">Нет префиксов</li>}
                  </ul>
                </div>
              </>
            ) : (
              <div className="text-sm text-gray-400 p-4 border border-gray-200 rounded-lg">Выберите оператора</div>
            )}
          </div>
        </div>
      )}

      {/* Create country modal */}
      <Modal open={showCreateCountry} onClose={() => setShowCreateCountry(false)} title="Добавить страну">
        <div className="space-y-4">
          <Input label="Название" value={countryForm.name} onChange={(e) => setCountryForm({ ...countryForm, name: e.target.value })} required />
          <Input label="Код (ISO 2)" value={countryForm.code} onChange={(e) => setCountryForm({ ...countryForm, code: e.target.value })} required placeholder="RU" maxLength={2} />
          <Input label="Телефонный код" value={countryForm.phone_code} onChange={(e) => setCountryForm({ ...countryForm, phone_code: e.target.value })} required placeholder="+7" />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowCreateCountry(false)}>Отмена</Button>
            <Button onClick={handleCreateCountry} disabled={savingCountry}>{savingCountry ? 'Создание...' : 'Создать'}</Button>
          </div>
        </div>
      </Modal>

      {/* 2-step operator wizard */}
      <Modal
        open={showWizard}
        onClose={() => setShowWizard(false)}
        title={`Добавить оператора — Шаг ${wizardStep + 1}: ${WIZARD_STEPS[wizardStep]}`}
      >
        {wizardStep === 0 && (
          <div className="space-y-4">
            <Input label="Название" value={wizardStep1.name} onChange={(e) => setWizardStep1({ ...wizardStep1, name: e.target.value })} required />
            <Input label="MCC" value={wizardStep1.mcc} onChange={(e) => setWizardStep1({ ...wizardStep1, mcc: e.target.value })} required placeholder="250" />
            <Input label="MNC" value={wizardStep1.mnc} onChange={(e) => setWizardStep1({ ...wizardStep1, mnc: e.target.value })} required placeholder="01" />
            <div className="flex justify-end gap-3 pt-2">
              <Button variant="secondary" onClick={() => setShowWizard(false)}>Отмена</Button>
              <Button
                onClick={() => setWizardStep(1)}
                disabled={!wizardStep1.name || !wizardStep1.mcc || !wizardStep1.mnc}
              >
                Далее →
              </Button>
            </div>
          </div>
        )}
        {wizardStep === 1 && (
          <div className="space-y-4">
            <label className="flex items-center gap-2 text-sm cursor-pointer">
              <input
                type="checkbox"
                checked={wizardStep2.supports_free_sender}
                onChange={(e) => setWizardStep2({ ...wizardStep2, supports_free_sender: e.target.checked })}
                className="rounded"
              />
              Поддержка бесплатной регистрации имени
            </label>
            <label className="flex items-center gap-2 text-sm cursor-pointer">
              <input
                type="checkbox"
                checked={wizardStep2.supports_paid_sender}
                onChange={(e) =>
                  setWizardStep2({
                    ...wizardStep2,
                    supports_paid_sender: e.target.checked,
                    monthly_tariff_amount: e.target.checked ? wizardStep2.monthly_tariff_amount : '',
                  })
                }
                className="rounded"
              />
              Поддержка платной регистрации имени
            </label>
            {wizardStep2.supports_paid_sender && (
              <Input
                label="Ежемесячный тариф (RUB)"
                type="number"
                min="0"
                step="0.01"
                value={wizardStep2.monthly_tariff_amount}
                onChange={(e) => setWizardStep2({ ...wizardStep2, monthly_tariff_amount: e.target.value })}
                placeholder="1500.00"
              />
            )}
            <div className="flex justify-between pt-2">
              <Button variant="secondary" onClick={() => setWizardStep(0)}>← Назад</Button>
              <div className="flex gap-2">
                <Button variant="secondary" onClick={() => setShowWizard(false)}>Отмена</Button>
                <Button onClick={handleWizardFinish} disabled={savingOperator}>{savingOperator ? 'Создание...' : 'Создать'}</Button>
              </div>
            </div>
          </div>
        )}
      </Modal>
    </>
  );
}
