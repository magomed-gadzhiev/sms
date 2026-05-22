import { useState, useEffect, useRef, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  contactListsApi,
  type ContactList,
  type ContactAttribute,
  type ImportJob,
} from '../../api/contacts';
import { ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { usePageTitle } from '../../contexts/PageTitleContext';
import { Button } from '../../components/ui/Button';

type WizardStep = 'upload' | 'mapping' | 'progress' | 'result';

const STEP_LABELS: Record<WizardStep, string> = {
  upload: '1. Загрузка файла',
  mapping: '2. Сопоставление колонок',
  progress: '3. Импорт',
  result: '4. Результат',
};

const STEPS: WizardStep[] = ['upload', 'mapping', 'progress', 'result'];

export function ImportWizardPage() {
  usePageTitle('Импорт контактов');
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const [list, setList] = useState<ContactList | null>(null);
  const [attributes, setAttributes] = useState<ContactAttribute[]>([]);
  const [step, setStep] = useState<WizardStep>('upload');
  const [error, setError] = useState('');

  // Upload step
  const [uploading, setUploading] = useState(false);
  const [dragOver, setDragOver] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Mapping step
  const [importId, setImportId] = useState('');
  const [preview, setPreview] = useState<string[][]>([]);
  const [mapping, setMapping] = useState<Record<number, string>>({});
  const [starting, setStarting] = useState(false);

  // Progress step
  const [job, setJob] = useState<ImportJob | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    if (!id) return;
    contactListsApi.get(id).then(setList).catch(() => {});
    contactListsApi
      .getAttributes(id)
      .then((resp) => setAttributes(resp.attributes ?? []))
      .catch(() => {});
  }, [id]);

  // Cleanup polling on unmount
  useEffect(() => {
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, []);

  async function handleFileUpload(file: File) {
    if (!id) return;

    // Проверяем размер до загрузки (лимит 50MB)
    const MAX_FILE_SIZE = 50 * 1024 * 1024;
    if (file.size > MAX_FILE_SIZE) {
      setError('Файл слишком большой. Максимальный размер — 50 МБ');
      return;
    }

    const allowedExtensions = ['.csv', '.xlsx', '.xls'];
    const fileExt = file.name.toLowerCase().slice(file.name.lastIndexOf('.'));
    if (!allowedExtensions.includes(fileExt)) {
      setError('Поддерживаются только файлы CSV и XLSX');
      return;
    }

    setUploading(true);
    setError('');
    try {
      const resp = await contactListsApi.uploadImport(id, file);
      setImportId(resp.import_id);
      setPreview(resp.preview ?? []);
      // Auto-detect phone column
      const newMapping: Record<number, string> = {};
      if (resp.preview && resp.preview.length > 0) {
        resp.preview[0].forEach((header, idx) => {
          const h = header.toLowerCase().trim();
          if (h === 'phone' || h === 'телефон' || h === 'номер') {
            newMapping[idx] = 'phone';
          }
        });
      }
      setMapping(newMapping);
      setStep('mapping');
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else if (err instanceof TypeError) {
        setError('Не удалось подключиться к серверу. Проверьте соединение');
      } else {
        setError('Ошибка при загрузке файла');
      }
    } finally {
      setUploading(false);
    }
  }

  function onDrop(e: React.DragEvent) {
    e.preventDefault();
    setDragOver(false);
    const file = e.dataTransfer.files[0];
    if (file) handleFileUpload(file);
  }

  function onFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (file) handleFileUpload(file);
  }

  async function handleStartImport() {
    if (!id || !importId) return;
    setStarting(true);
    setError('');
    try {
      const columnMapping: Record<string, number> = {};
      for (const [colIdx, field] of Object.entries(mapping)) {
        if (field) {
          columnMapping[field] = Number(colIdx);
        }
      }
      const result = await contactListsApi.startImport(
        id,
        importId,
        columnMapping,
      );
      setJob(result);
      setStep('progress');
      startPolling();
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : 'Ошибка при запуске импорта',
      );
    } finally {
      setStarting(false);
    }
  }

  const startPolling = useCallback(() => {
    if (!id || !importId) return;
    pollRef.current = setInterval(async () => {
      try {
        const status = await contactListsApi.getImportStatus(id, importId);
        setJob(status);
        if (
          status.status === 'completed' ||
          status.status === 'failed'
        ) {
          if (pollRef.current) clearInterval(pollRef.current);
          setStep('result');
        }
      } catch {
        // Ignore polling errors
      }
    }, 2000);
  }, [id, importId]);

  // Available mapping targets
  const mappingOptions = [
    { value: '', label: '-- Пропустить --' },
    { value: 'phone', label: 'Телефон' },
    ...attributes.map((a) => ({
      value: `attr:${a.name}`,
      label: a.display_name || a.name,
    })),
    { value: 'tags', label: 'Теги' },
  ];

  const hasPhoneMapping = Object.values(mapping).includes('phone');

  const progressPercent =
    job && job.total_rows > 0
      ? Math.round(
          ((job.imported_count + job.updated_count + job.error_count) /
            job.total_rows) *
            100,
        )
      : 0;

  return (
    <div>
      <PageHeader
        breadcrumbs={[
          { label: 'Контактные базы', href: '/contact-lists' },
          { label: list?.name ?? '...', href: `/contact-lists/${id}` },
          { label: 'Импорт' },
        ]}
      />

      {/* Step indicator */}
      <div className="flex items-center gap-2 mb-6">
        {STEPS.map((s, idx) => {
          const isActive = s === step;
          const isPast = STEPS.indexOf(step) > idx;
          return (
            <div key={s} className="flex items-center gap-2">
              {idx > 0 && (
                <div
                  className={`w-8 h-0.5 ${isPast ? 'bg-blue-500' : 'bg-gray-300'}`}
                />
              )}
              <div
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm font-medium ${
                  isActive
                    ? 'bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-300'
                    : isPast
                      ? 'bg-green-100 dark:bg-green-900/40 text-green-700 dark:text-green-300'
                      : 'bg-gray-100 dark:bg-slate-800 text-gray-500 dark:text-slate-400'
                }`}
              >
                {isPast && (
                  <svg
                    className="w-4 h-4"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M5 13l4 4L19 7"
                    />
                  </svg>
                )}
                {STEP_LABELS[s]}
              </div>
            </div>
          );
        })}
      </div>

      {error && (
        <div className="bg-red-50 dark:bg-red-950/40 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 rounded p-3 mb-4 text-sm">
          {error}
        </div>
      )}

      {/* Step 1: Upload */}
      {step === 'upload' && (
        <div
          className={`border-2 border-dashed rounded-lg p-12 text-center transition-colors ${
            dragOver
              ? 'border-blue-400 bg-blue-50 dark:bg-blue-950/40'
              : 'border-gray-300 dark:border-slate-600 bg-white dark:bg-slate-900'
          }`}
          onDragOver={(e) => {
            e.preventDefault();
            setDragOver(true);
          }}
          onDragLeave={() => setDragOver(false)}
          onDrop={onDrop}
        >
          <svg
            className="mx-auto w-12 h-12 text-gray-400 dark:text-slate-500 mb-4"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={1.5}
              d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"
            />
          </svg>
          <p className="text-gray-600 dark:text-slate-400 mb-2">
            Перетащите CSV или XLSX файл сюда
          </p>
          <p className="text-sm text-gray-400 dark:text-slate-500 mb-4">или</p>
          <Button
            onClick={() => fileInputRef.current?.click()}
            disabled={uploading}
          >
            {uploading ? 'Загрузка...' : 'Выбрать файл'}
          </Button>
          <input
            ref={fileInputRef}
            type="file"
            accept=".csv,.xlsx,.xls"
            onChange={onFileSelect}
            className="hidden"
          />
        </div>
      )}

      {/* Step 2: Column mapping */}
      {step === 'mapping' && (
        <div className="bg-white dark:bg-slate-900 border border-gray-200 dark:border-slate-700 rounded-lg p-6">
          <h3 className="text-lg font-medium text-gray-900 dark:text-slate-100 mb-4">
            Сопоставление колонок
          </h3>
          <p className="text-sm text-gray-500 dark:text-slate-400 mb-4">
            Укажите, какие колонки файла соответствуют полям контактов.
            Колонка "Телефон" обязательна.
          </p>

          {preview.length > 0 && (
            <div className="overflow-x-auto mb-6">
              <table className="w-full text-sm border border-gray-200 dark:border-slate-700 rounded">
                <caption className="sr-only">Предпросмотр и сопоставление колонок импортируемого файла</caption>
                <thead className="bg-gray-50 dark:bg-slate-950">
                  <tr>
                    {preview[0].map((header, idx) => (
                      <th key={idx} className="px-3 py-2 text-left">
                        <div className="mb-2 font-medium text-gray-700 dark:text-slate-300">
                          {header}
                        </div>
                        <select
                          value={mapping[idx] ?? ''}
                          onChange={(e) =>
                            setMapping((prev) => ({
                              ...prev,
                              [idx]: e.target.value,
                            }))
                          }
                          className="w-full border border-gray-300 dark:border-slate-600 rounded px-2 py-1 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                        >
                          {mappingOptions.map((opt) => (
                            <option key={opt.value} value={opt.value}>
                              {opt.label}
                            </option>
                          ))}
                        </select>
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100 dark:divide-slate-800">
                  {preview.slice(1, 6).map((row, rowIdx) => (
                    <tr key={rowIdx}>
                      {row.map((cell, cellIdx) => (
                        <td
                          key={cellIdx}
                          className="px-3 py-2 text-gray-600 dark:text-slate-400 truncate max-w-[200px]"
                        >
                          {cell}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {!hasPhoneMapping && (
            <p className="text-amber-600 text-sm mb-4">
              Необходимо сопоставить хотя бы одну колонку с полем "Телефон"
            </p>
          )}

          <div className="flex justify-between">
            <Button variant="secondary" onClick={() => setStep('upload')}>
              Назад
            </Button>
            <Button
              onClick={handleStartImport}
              disabled={starting || !hasPhoneMapping}
            >
              {starting ? 'Запуск...' : 'Начать импорт'}
            </Button>
          </div>
        </div>
      )}

      {/* Step 3: Progress */}
      {step === 'progress' && (
        <div className="bg-white dark:bg-slate-900 border border-gray-200 dark:border-slate-700 rounded-lg p-6">
          <h3 className="text-lg font-medium text-gray-900 dark:text-slate-100 mb-4">
            Импорт в процессе...
          </h3>
          <div className="mb-4">
            <div className="flex justify-between text-sm text-gray-600 dark:text-slate-400 mb-1">
              <span>Прогресс</span>
              <span>{progressPercent}%</span>
            </div>
            <div className="w-full bg-gray-200 dark:bg-slate-700 rounded-full h-3">
              <div
                className="bg-blue-600 h-3 rounded-full transition-all duration-300"
                style={{ width: `${progressPercent}%` }}
              />
            </div>
          </div>
          {job && (
            <div className="grid grid-cols-3 gap-4 text-center text-sm">
              <div className="bg-green-50 dark:bg-green-950/40 rounded p-3">
                <div className="text-2xl font-semibold text-green-700 dark:text-green-300">
                  {job.imported_count}
                </div>
                <div className="text-green-600 dark:text-green-400">Импортировано</div>
              </div>
              <div className="bg-blue-50 dark:bg-blue-950/40 rounded p-3">
                <div className="text-2xl font-semibold text-blue-700 dark:text-blue-300">
                  {job.updated_count}
                </div>
                <div className="text-blue-600 dark:text-blue-400">Обновлено</div>
              </div>
              <div className="bg-red-50 dark:bg-red-950/40 rounded p-3">
                <div className="text-2xl font-semibold text-red-700 dark:text-red-300">
                  {job.error_count}
                </div>
                <div className="text-red-600 dark:text-red-400">Ошибки</div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Step 4: Result */}
      {step === 'result' && job && (
        <div className="bg-white dark:bg-slate-900 border border-gray-200 dark:border-slate-700 rounded-lg p-6">
          <div className="text-center mb-6">
            {job.status === 'completed' ? (
              <>
                <svg
                  className="mx-auto w-16 h-16 text-green-500 mb-3"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
                <h3 className="text-lg font-medium text-gray-900 dark:text-slate-100">
                  Импорт завершён
                </h3>
              </>
            ) : (
              <>
                <svg
                  className="mx-auto w-16 h-16 text-red-500 dark:text-red-400 mb-3"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
                <h3 className="text-lg font-medium text-gray-900 dark:text-slate-100">
                  Импорт завершён с ошибками
                </h3>
              </>
            )}
          </div>

          <div className="grid grid-cols-4 gap-4 text-center text-sm mb-6">
            <div className="bg-gray-50 dark:bg-slate-950 rounded p-3">
              <div className="text-2xl font-semibold text-gray-700 dark:text-slate-300">
                {job.total_rows}
              </div>
              <div className="text-gray-500 dark:text-slate-400">Всего строк</div>
            </div>
            <div className="bg-green-50 dark:bg-green-950/40 rounded p-3">
              <div className="text-2xl font-semibold text-green-700 dark:text-green-300">
                {job.imported_count}
              </div>
              <div className="text-green-600 dark:text-green-400">Импортировано</div>
            </div>
            <div className="bg-blue-50 dark:bg-blue-950/40 rounded p-3">
              <div className="text-2xl font-semibold text-blue-700 dark:text-blue-300">
                {job.updated_count}
              </div>
              <div className="text-blue-600 dark:text-blue-400">Обновлено</div>
            </div>
            <div className="bg-red-50 dark:bg-red-950/40 rounded p-3">
              <div className="text-2xl font-semibold text-red-700 dark:text-red-300">
                {job.error_count}
              </div>
              <div className="text-red-600 dark:text-red-400">Ошибки</div>
            </div>
          </div>

          <div className="flex justify-center">
            <Button onClick={() => navigate(`/contact-lists/${id}`)}>
              Вернуться к базе
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
