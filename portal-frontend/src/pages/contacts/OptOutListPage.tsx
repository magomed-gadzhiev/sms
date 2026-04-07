import { useState, useEffect, useCallback } from 'react';
import { optOutApi, type OptOutEntry } from '../../api/opt_out';
import { ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { DataTable, type Column } from '../../components/data/DataTable';
import { useToast } from '../../components/ui/Toast';

export function OptOutListPage() {
  const toast = useToast();

  const [items, setItems] = useState<OptOutEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const [searchInput, setSearchInput] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Add single number modal
  const [showAdd, setShowAdd] = useState(false);
  const [addPhone, setAddPhone] = useState('');
  const [addKeyword, setAddKeyword] = useState('');
  const [adding, setAdding] = useState(false);
  const [addError, setAddError] = useState('');

  // Delete confirm
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  // Import modal
  const [showImport, setShowImport] = useState(false);
  const [importText, setImportText] = useState('');
  const [importing, setImporting] = useState(false);
  const [importResult, setImportResult] = useState<{ imported: number; skipped: number } | null>(null);

  const perPage = 50;

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const resp = await optOutApi.list({ page, per_page: perPage, search: search || undefined });
      setItems(resp.items ?? []);
      setTotal(resp.total ?? 0);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить список отписок');
    } finally {
      setLoading(false);
    }
  }, [page, search]);

  useEffect(() => {
    load();
  }, [load]);

  function handleSearch() {
    setSearch(searchInput);
    setPage(1);
  }

  function handleSearchKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter') handleSearch();
  }

  async function handleAdd() {
    if (!addPhone.trim()) return;
    setAdding(true);
    setAddError('');
    try {
      await optOutApi.add(addPhone.trim(), addKeyword.trim() || undefined);
      toast.success('Номер добавлен в список отписок');
      setShowAdd(false);
      setAddPhone('');
      setAddKeyword('');
      await load();
    } catch (err) {
      setAddError(err instanceof ApiError ? err.message : 'Ошибка добавления');
    } finally {
      setAdding(false);
    }
  }

  async function handleDelete() {
    if (!deleteId) return;
    setDeleting(true);
    try {
      await optOutApi.remove(deleteId);
      toast.success('Номер удалён из списка отписок');
      setDeleteId(null);
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка удаления');
    } finally {
      setDeleting(false);
    }
  }

  async function handleImport() {
    const phones = importText
      .split(/[\n,;]+/)
      .map((p) => p.trim())
      .filter(Boolean);
    if (phones.length === 0) return;
    setImporting(true);
    setImportResult(null);
    try {
      const result = await optOutApi.import(phones);
      setImportResult(result);
      toast.success(`Импортировано: ${result.imported}, пропущено: ${result.skipped}`);
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка импорта');
    } finally {
      setImporting(false);
    }
  }

  function formatDate(dateStr: string) {
    if (!dateStr) return '-';
    try {
      return new Date(dateStr).toLocaleString('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      });
    } catch {
      return dateStr;
    }
  }

  const perPage = 20;
  const columns: Column<OptOutEntry>[] = [
    {
      key: 'phone',
      header: 'Телефон',
      render: (item) => <span className="font-mono text-sm">{item.phone}</span>,
    },
    {
      key: 'keyword',
      header: 'Ключевое слово',
      render: (item) =>
        item.keyword ? (
          <span className="inline-block px-2 py-0.5 rounded text-xs bg-red-100 text-red-700 font-medium">
            {item.keyword}
          </span>
        ) : (
          <span className="text-gray-400 text-xs">—</span>
        ),
    },
    {
      key: 'opted_out_at',
      header: 'Дата отписки',
      render: (item) => (
        <span className="text-sm text-gray-600">{formatDate(item.opted_out_at)}</span>
      ),
    },
    {
      key: 'actions',
      header: '',
      render: (item) => (
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setDeleteId(item.id)}
          className="text-red-600 hover:text-red-800 hover:bg-red-50"
        >
          Удалить
        </Button>
      ),
    },
  ];

  return (
    <div className="space-y-6">
      <PageHeader
        title="Список отписок (Opt-Out)"
        subtitle="Номера телефонов, отказавшихся от рассылок. Они автоматически исключаются при запуске кампаний."
        actions={
          <div className="flex gap-2">
            <Button variant="secondary" onClick={() => setShowImport(true)}>
              Импорт CSV
            </Button>
            <Button onClick={() => setShowAdd(true)}>Добавить номер</Button>
          </div>
        }
      />

      {/* Search */}
      <div className="flex gap-3">
        <Input
          placeholder="Поиск по номеру..."
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          onKeyDown={handleSearchKeyDown}
          className="max-w-xs"
        />
        <Button variant="secondary" onClick={handleSearch}>
          Найти
        </Button>
        {search && (
          <Button
            variant="ghost"
            onClick={() => {
              setSearch('');
              setSearchInput('');
              setPage(1);
            }}
          >
            Сбросить
          </Button>
        )}
      </div>

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {error}
        </div>
      )}

      <DataTable
        columns={columns as Column<Record<string, unknown>>[]}
        data={items}
        total={total}
        page={page}
        pageSize={perPage}
        onPageChange={setPage}
        loading={loading}
        keyField="id"
      />

      {/* Add modal */}
      <Modal
        open={showAdd}
        onClose={() => {
          setShowAdd(false);
          setAddPhone('');
          setAddKeyword('');
          setAddError('');
        }}
        title="Добавить номер в список отписок"
      >
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Номер телефона <span className="text-red-500">*</span>
            </label>
            <Input
              placeholder="+79001234567"
              value={addPhone}
              onChange={(e) => setAddPhone(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Ключевое слово (необязательно)
            </label>
            <Input
              placeholder="STOP, ОТПИСАТЬСЯ..."
              value={addKeyword}
              onChange={(e) => setAddKeyword(e.target.value)}
            />
          </div>
          {addError && <p className="text-sm text-red-600">{addError}</p>}
          <div className="flex justify-end gap-2 pt-2">
            <Button
              variant="secondary"
              onClick={() => {
                setShowAdd(false);
                setAddPhone('');
                setAddKeyword('');
                setAddError('');
              }}
            >
              Отмена
            </Button>
            <Button onClick={handleAdd} disabled={adding || !addPhone.trim()}>
              {adding ? 'Добавление...' : 'Добавить'}
            </Button>
          </div>
        </div>
      </Modal>

      {/* Import modal */}
      <Modal
        open={showImport}
        onClose={() => {
          setShowImport(false);
          setImportText('');
          setImportResult(null);
        }}
        title="Импорт номеров в список отписок"
      >
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            Введите номера телефонов — по одному на строку, или через запятую/точку с запятой.
            Максимум 10 000 номеров за раз.
          </p>
          <textarea
            className="w-full border border-gray-300 rounded-lg p-3 text-sm font-mono resize-y min-h-[180px] focus:outline-none focus:ring-2 focus:ring-primary/50"
            placeholder={`+79001234567\n+79007654321\n+79009999999`}
            value={importText}
            onChange={(e) => setImportText(e.target.value)}
          />
          {importResult && (
            <div className="rounded-lg bg-green-50 border border-green-200 px-4 py-3 text-sm text-green-700">
              Добавлено: <strong>{importResult.imported}</strong>, пропущено (дубли):{' '}
              <strong>{importResult.skipped}</strong>
            </div>
          )}
          <div className="flex justify-end gap-2 pt-2">
            <Button
              variant="secondary"
              onClick={() => {
                setShowImport(false);
                setImportText('');
                setImportResult(null);
              }}
            >
              Закрыть
            </Button>
            <Button onClick={handleImport} disabled={importing || !importText.trim()}>
              {importing ? 'Импорт...' : 'Импортировать'}
            </Button>
          </div>
        </div>
      </Modal>

      {/* Delete confirm */}
      <ConfirmDialog
        open={!!deleteId}
        title="Удалить из списка отписок?"
        description="Номер будет удалён из списка отписок. При следующей рассылке он снова может получать сообщения."
        confirmLabel="Удалить"
        onConfirm={handleDelete}
        onCancel={() => setDeleteId(null)}
        loading={deleting}
        variant="danger"
      />
    </div>
  );
}
