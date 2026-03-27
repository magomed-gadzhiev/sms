import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { countriesApi, operatorsApi, type CountryInfo, type OperatorInfo, type OperatorPrefix } from '../../api/admin';

const PAGE_SIZE = 20;

export function CountriesPage() {
  const toast = useToast();
  const [countries, setCountries] = useState<CountryInfo[]>([]);
  const [countryTotal, setCountryTotal] = useState(0);
  const [countryPage, setCountryPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [showCreateCountry, setShowCreateCountry] = useState(false);
  const [countryForm, setCountryForm] = useState({ name: '', code: '', phone_code: '' });
  const [selectedCountry, setSelectedCountry] = useState<CountryInfo | null>(null);
  const [operators, setOperators] = useState<OperatorInfo[]>([]);
  const [operatorsLoading, setOperatorsLoading] = useState(false);
  const [showCreateOperator, setShowCreateOperator] = useState(false);
  const [operatorForm, setOperatorForm] = useState({ name: '', mcc: '', mnc: '' });
  const [selectedOperator, setSelectedOperator] = useState<OperatorInfo | null>(null);
  const [prefixes, setPrefixes] = useState<OperatorPrefix[]>([]);
  const [prefixInput, setPrefixInput] = useState('');
  const [saving, setSaving] = useState(false);

  const fetchCountries = useCallback(async () => {
    setLoading(true);
    try { const res = await countriesApi.list({ limit: PAGE_SIZE, offset: (countryPage - 1) * PAGE_SIZE }); setCountries(res.countries || []); setCountryTotal(res.total); }
    catch { toast.error('Failed to load countries'); }
    finally { setLoading(false); }
  }, [countryPage, toast]);

  useEffect(() => { fetchCountries(); }, [fetchCountries]);

  const fetchOperators = useCallback(async (countryId: string) => {
    setOperatorsLoading(true);
    try { const res = await operatorsApi.list({ country_id: countryId, limit: 100, offset: 0 }); setOperators(res.operators || []); }
    catch { toast.error('Failed to load operators'); }
    finally { setOperatorsLoading(false); }
  }, [toast]);

  const fetchPrefixes = useCallback(async (operatorId: string) => {
    try { const res = await operatorsApi.listPrefixes(operatorId); setPrefixes(res.prefixes || []); }
    catch { toast.error('Failed to load prefixes'); }
  }, [toast]);

  const selectCountry = (country: CountryInfo) => { setSelectedCountry(country); setSelectedOperator(null); setPrefixes([]); fetchOperators(country.country_id); };
  const selectOperator = (op: OperatorInfo) => { setSelectedOperator(op); fetchPrefixes(op.operator_id); };

  const handleCreateCountry = async () => { setSaving(true); try { await countriesApi.create(countryForm); toast.success('Country created'); setShowCreateCountry(false); fetchCountries(); } catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); } finally { setSaving(false); } };
  const handleCreateOperator = async () => { if (!selectedCountry) return; setSaving(true); try { await operatorsApi.create({ ...operatorForm, country_id: selectedCountry.country_id, active: true }); toast.success('Operator created'); setShowCreateOperator(false); fetchOperators(selectedCountry.country_id); } catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); } finally { setSaving(false); } };
  const handleAddPrefix = async () => { if (!selectedOperator || !prefixInput) return; setSaving(true); try { await operatorsApi.createPrefix(selectedOperator.operator_id, { prefix: prefixInput }); toast.success('Prefix added'); setPrefixInput(''); fetchPrefixes(selectedOperator.operator_id); } catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); } finally { setSaving(false); } };
  const handleDeletePrefix = async (prefixId: string) => { if (!selectedOperator) return; try { await operatorsApi.deletePrefix(selectedOperator.operator_id, prefixId); toast.success('Prefix removed'); fetchPrefixes(selectedOperator.operator_id); } catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); } };

  const countryColumns: Column<CountryInfo>[] = [{ key: 'name', header: 'Country', sortable: true }, { key: 'code', header: 'Code' }, { key: 'phone_code', header: 'Phone Code' }];
  const operatorColumns: Column<OperatorInfo>[] = [{ key: 'name', header: 'Operator' }, { key: 'mcc', header: 'MCC' }, { key: 'mnc', header: 'MNC' }, { key: 'active', header: 'Status', render: (o) => <StatusBadge status={o.active ? 'active' : 'inactive'} /> }];

  return (
    <>
      <PageHeader title="Countries & Operators" />
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div>
          <div className="flex items-center justify-between mb-3"><h2 className="text-lg font-semibold">Countries</h2><Button size="sm" onClick={() => { setCountryForm({ name: '', code: '', phone_code: '' }); setShowCreateCountry(true); }}>Add</Button></div>
          <DataTable columns={countryColumns} data={countries} total={countryTotal} page={countryPage} pageSize={PAGE_SIZE} onPageChange={setCountryPage} loading={loading} keyField="country_id" onRowClick={selectCountry} />
        </div>
        <div>
          <div className="flex items-center justify-between mb-3"><h2 className="text-lg font-semibold">{selectedCountry ? `Operators — ${selectedCountry.name}` : 'Operators'}</h2>{selectedCountry && <Button size="sm" onClick={() => { setOperatorForm({ name: '', mcc: '', mnc: '' }); setShowCreateOperator(true); }}>Add</Button>}</div>
          {selectedCountry ? <DataTable columns={operatorColumns} data={operators} total={operators.length} page={1} pageSize={100} onPageChange={() => {}} loading={operatorsLoading} keyField="operator_id" onRowClick={selectOperator} /> : <div className="text-sm text-gray-400 p-4 border border-gray-200 rounded-lg">Select a country</div>}
        </div>
        <div>
          <h2 className="text-lg font-semibold mb-3">{selectedOperator ? `Prefixes — ${selectedOperator.name}` : 'Prefixes'}</h2>
          {selectedOperator ? (
            <>
              <div className="flex gap-2 mb-3"><Input value={prefixInput} onChange={(e) => setPrefixInput(e.target.value)} placeholder="e.g. +7921" /><Button size="sm" onClick={handleAddPrefix} disabled={saving || !prefixInput}>Add</Button></div>
              <ul className="space-y-1">
                {prefixes.map((p) => (<li key={p.prefix_id} className="flex items-center justify-between py-1 px-2 bg-gray-50 rounded text-sm"><span className="font-mono">{p.prefix}</span><button className="text-xs text-danger hover:underline" onClick={() => handleDeletePrefix(p.prefix_id)}>Remove</button></li>))}
                {prefixes.length === 0 && <li className="text-sm text-gray-400">No prefixes</li>}
              </ul>
            </>
          ) : <div className="text-sm text-gray-400 p-4 border border-gray-200 rounded-lg">Select an operator</div>}
        </div>
      </div>
      <Modal open={showCreateCountry} onClose={() => setShowCreateCountry(false)} title="Add Country">
        <div className="space-y-4">
          <Input label="Name" value={countryForm.name} onChange={(e) => setCountryForm({ ...countryForm, name: e.target.value })} required />
          <Input label="Code (ISO)" value={countryForm.code} onChange={(e) => setCountryForm({ ...countryForm, code: e.target.value })} required placeholder="US" maxLength={2} />
          <Input label="Phone Code" value={countryForm.phone_code} onChange={(e) => setCountryForm({ ...countryForm, phone_code: e.target.value })} required placeholder="+1" />
          <div className="flex justify-end gap-3 pt-2"><Button variant="secondary" onClick={() => setShowCreateCountry(false)}>Cancel</Button><Button onClick={handleCreateCountry} disabled={saving}>{saving ? 'Creating...' : 'Create'}</Button></div>
        </div>
      </Modal>
      <Modal open={showCreateOperator} onClose={() => setShowCreateOperator(false)} title="Add Operator">
        <div className="space-y-4">
          <Input label="Name" value={operatorForm.name} onChange={(e) => setOperatorForm({ ...operatorForm, name: e.target.value })} required />
          <Input label="MCC" value={operatorForm.mcc} onChange={(e) => setOperatorForm({ ...operatorForm, mcc: e.target.value })} required />
          <Input label="MNC" value={operatorForm.mnc} onChange={(e) => setOperatorForm({ ...operatorForm, mnc: e.target.value })} required />
          <div className="flex justify-end gap-3 pt-2"><Button variant="secondary" onClick={() => setShowCreateOperator(false)}>Cancel</Button><Button onClick={handleCreateOperator} disabled={saving}>{saving ? 'Creating...' : 'Create'}</Button></div>
        </div>
      </Modal>
    </>
  );
}
