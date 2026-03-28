import { useState, useRef, type FormEvent, type DragEvent, useEffect } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { lookupApi, ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { DataTable, type Column } from '../../components/data/DataTable';
import { StatCard } from '../../components/data/StatCard';

interface LookupResult {
  phone: string;
  operator_name: string;
  country_code: string;
  number_status: string;
  number_type: string;
  is_ported: boolean;
}

interface HistoryEntry {
  id: string;
  phone: string;
  operator_name: string;
  country_code: string;
  number_status: string;
  created_at: string;
}

interface LookupStats {
  total_lookups: number;
  period: string;
}

const PAGE_SIZE = 20;

export function LookupPage() {
  // Single lookup
  const [phone, setPhone] = useState('');
  const [singleResult, setSingleResult] = useState<LookupResult | null>(null);
  const [singleLoading, setSingleLoading] = useState(false);
  const [singleError, setSingleError] = useState('');

  // Bulk lookup
  const [bulkFile, setBulkFile] = useState<File | null>(null);
  const [bulkResults, setBulkResults] = useState<LookupResult[]>([]);
  const [bulkLoading, setBulkLoading] = useState(false);
  const [bulkError, setBulkError] = useState('');
  const [dragOver, setDragOver] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // History
  const [history, setHistory] = useState<HistoryEntry[]>([]);
  const [historyTotal, setHistoryTotal] = useState(0);
  const [historyPage, setHistoryPage] = useState(1);
  const [historyLoading, setHistoryLoading] = useState(false);

  // Stats
  const [stats, setStats] = useState<LookupStats | null>(null);

  useEffect(() => {
    loadHistory(1);
    loadStats();
  }, []);

  async function loadHistory(page: number) {
    setHistoryLoading(true);
    try {
      const resp = await lookupApi.history({
        page: String(page),
        page_size: String(PAGE_SIZE),
      }) as { items: HistoryEntry[]; total: number };
      setHistory(resp.items || []);
      setHistoryTotal(resp.total || 0);
      setHistoryPage(page);
    } catch {
      // silent
    } finally {
      setHistoryLoading(false);
    }
  }

  async function loadStats() {
    try {
      const resp = await lookupApi.stats() as LookupStats;
      setStats(resp);
    } catch {
      // silent
    }
  }

  async function handleSingleLookup(e: FormEvent) {
    e.preventDefault();
    if (!phone.trim()) return;
    setSingleLoading(true);
    setSingleError('');
    setSingleResult(null);
    try {
      const result = await lookupApi.single(phone.trim()) as LookupResult;
      setSingleResult(result);
      loadHistory(1);
      loadStats();
    } catch (err) {
      setSingleError(err instanceof ApiError ? err.message : 'Ошибка при проверке номера');
    } finally {
      setSingleLoading(false);
    }
  }

  async function handleBulkLookup(e: FormEvent) {
    e.preventDefault();
    if (!bulkFile) return;
    setBulkLoading(true);
    setBulkError('');
    setBulkResults([]);
    try {
      const resp = await lookupApi.bulk(bulkFile) as { results: LookupResult[] };
      setBulkResults(resp.results || []);
      loadHistory(1);
      loadStats();
    } catch (err) {
      setBulkError(err instanceof ApiError ? err.message : 'Ошибка при массовой проверке');
    } finally {
      setBulkLoading(false);
    }
  }

  function handleDragOver(e: DragEvent) {
    e.preventDefault();
    setDragOver(true);
  }

  function handleDragLeave(e: DragEvent) {
    e.preventDefault();
    setDragOver(false);
  }

  function handleDrop(e: DragEvent) {
    e.preventDefault();
    setDragOver(false);
    const file = e.dataTransfer.files?.[0];
    if (file && file.name.endsWith('.csv')) {
      setBulkFile(file);
    } else {
      setBulkError('Пожалуйста, загрузите CSV файл');
    }
  }

  function handleFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (file) {
      setBulkFile(file);
      setBulkError('');
    }
  }

  const bulkColumns: Column<LookupResult>[] = [
    { key: 'phone', header: 'Номер' },
    { key: 'operator_name', header: 'Оператор' },
    { key: 'country_code', header: 'Страна' },
    { key: 'number_status', header: 'Статус' },
    { key: 'number_type', header: 'Тип' },
    {
      key: 'is_ported',
      header: 'Портирован',
      render: (item) => (item.is_ported ? 'Да' : 'Нет'),
    },
  ];

  const historyColumns: Column<HistoryEntry>[] = [
    { key: 'phone', header: 'Номер' },
    { key: 'operator_name', header: 'Оператор' },
    { key: 'country_code', header: 'Страна' },
    { key: 'number_status', header: 'Статус' },
    {
      key: 'created_at',
      header: 'Дата',
      render: (item) => new Date(item.created_at).toLocaleString(),
    },
  ];

  const tabTriggerClass =
    'px-4 py-2 text-sm font-medium text-gray-600 border-b-2 border-transparent data-[state=active]:text-primary data-[state=active]:border-primary hover:text-gray-900 transition-colors';

  return (
    <div className="max-w-[900px]">
      <PageHeader title="HLR Lookup" subtitle="Проверка номеров телефонов" />

      <Tabs.Root defaultValue="single" className="mb-8">
        <Tabs.List className="flex border-b border-gray-200 mb-6" aria-label="Тип проверки">
          <Tabs.Trigger value="single" className={tabTriggerClass}>
            Одиночный
          </Tabs.Trigger>
          <Tabs.Trigger value="bulk" className={tabTriggerClass}>
            Массовый
          </Tabs.Trigger>
        </Tabs.List>

        {/* Single lookup tab */}
        <Tabs.Content value="single">
          <form onSubmit={handleSingleLookup} className="flex items-end gap-3 mb-6">
            <div className="flex-1 max-w-xs">
              <Input
                label="Номер телефона"
                type="tel"
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
                placeholder="+7 900 123 45 67"
                required
              />
            </div>
            <Button type="submit" disabled={singleLoading}>
              {singleLoading ? 'Проверка...' : 'Проверить'}
            </Button>
          </form>

          {singleError && (
            <p role="alert" className="text-red-600 mb-4">{singleError}</p>
          )}

          {singleResult && (
            <div className="bg-white border border-gray-200 rounded-lg p-5 mb-6">
              <h3 className="text-lg font-semibold mb-3">Результат</h3>
              <dl className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm">
                <div>
                  <dt className="text-gray-500">Оператор</dt>
                  <dd className="font-medium text-gray-900">{singleResult.operator_name}</dd>
                </div>
                <div>
                  <dt className="text-gray-500">Код страны</dt>
                  <dd className="font-medium text-gray-900">{singleResult.country_code}</dd>
                </div>
                <div>
                  <dt className="text-gray-500">Статус номера</dt>
                  <dd className="font-medium text-gray-900">{singleResult.number_status}</dd>
                </div>
                <div>
                  <dt className="text-gray-500">Тип номера</dt>
                  <dd className="font-medium text-gray-900">{singleResult.number_type}</dd>
                </div>
                <div>
                  <dt className="text-gray-500">Портирован</dt>
                  <dd className="font-medium text-gray-900">{singleResult.is_ported ? 'Да' : 'Нет'}</dd>
                </div>
              </dl>
            </div>
          )}
        </Tabs.Content>

        {/* Bulk lookup tab */}
        <Tabs.Content value="bulk">
          <form onSubmit={handleBulkLookup} className="mb-6">
            <div
              className={`border-2 border-dashed rounded-lg p-8 text-center cursor-pointer transition-colors mb-4
                ${dragOver ? 'border-primary bg-primary/5' : 'border-gray-300 hover:border-gray-400'}`}
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
              onDrop={handleDrop}
              onClick={() => fileInputRef.current?.click()}
              role="button"
              tabIndex={0}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  fileInputRef.current?.click();
                }
              }}
              aria-label="Загрузить CSV файл"
            >
              <input
                ref={fileInputRef}
                type="file"
                accept=".csv"
                className="hidden"
                onChange={handleFileSelect}
              />
              {bulkFile ? (
                <div>
                  <p className="text-gray-900 font-medium">{bulkFile.name}</p>
                  <p className="text-sm text-gray-500 mt-1">
                    {(bulkFile.size / 1024).toFixed(1)} KB
                  </p>
                </div>
              ) : (
                <div>
                  <p className="text-gray-600 mb-1">
                    Перетащите CSV файл сюда или нажмите для выбора
                  </p>
                  <p className="text-sm text-gray-400">Максимум 10 000 номеров</p>
                </div>
              )}
            </div>

            <Button type="submit" disabled={!bulkFile || bulkLoading}>
              {bulkLoading ? 'Обработка...' : 'Отправить'}
            </Button>
          </form>

          {bulkError && (
            <p role="alert" className="text-red-600 mb-4">{bulkError}</p>
          )}

          {bulkResults.length > 0 && (
            <div className="mb-6">
              <h3 className="text-lg font-semibold mb-3">
                Результаты ({bulkResults.length})
              </h3>
              <DataTable
                columns={bulkColumns}
                data={bulkResults}
                total={bulkResults.length}
                page={1}
                pageSize={bulkResults.length}
                onPageChange={() => {}}
                keyField="phone"
              />
            </div>
          )}
        </Tabs.Content>
      </Tabs.Root>

      {/* Stats */}
      {stats && (
        <div className="mb-8 max-w-xs">
          <StatCard
            title="Всего проверок"
            value={stats.total_lookups.toLocaleString()}
            subtitle={stats.period}
          />
        </div>
      )}

      {/* History */}
      <section>
        <h2 className="text-lg font-semibold mb-4">История проверок</h2>
        <DataTable
          columns={historyColumns}
          data={history}
          total={historyTotal}
          page={historyPage}
          pageSize={PAGE_SIZE}
          onPageChange={loadHistory}
          loading={historyLoading}
          keyField="id"
        />
      </section>
    </div>
  );
}
