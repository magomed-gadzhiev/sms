import { useState, useEffect, useCallback, type FormEvent } from 'react';
import { templatesApi, ApiError, type TemplateInfo } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Badge } from '../../components/ui/Badge';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';

const PAGE_SIZE = 20;

const STATUS_BADGE: Record<string, { variant: 'warning' | 'success' | 'danger' | 'default'; label: string }> = {
  draft: { variant: 'default', label: 'Черновик' },
  pending: { variant: 'warning', label: 'На модерации' },
  review: { variant: 'warning', label: 'На ревью' },
  revision_requested: { variant: 'danger', label: 'Доработка' },
  approved: { variant: 'success', label: 'Одобрен' },
  rejected: { variant: 'danger', label: 'Отклонён' },
};

function pluralize(n: number, one: string, few: string, many: string): string {
  const abs = Math.abs(n);
  const mod10 = abs % 10;
  const mod100 = abs % 100;
  if (mod10 === 1 && mod100 !== 11) return `${n} ${one}`;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return `${n} ${few}`;
  return `${n} ${many}`;
}

function truncate(text: string, max: number): string {
  return text.length > max ? text.slice(0, max) + '...' : text;
}

/** Extract {{variable}} names from a template body */
function extractVariables(body: string): string[] {
  const matches = body.match(/\{\{(\w+)\}\}/g);
  if (!matches) return [];
  return [...new Set(matches.map((m) => m.replace(/[{}]/g, '')))];
}

export function TemplatesPage() {
  const [templates, setTemplates] = useState<TemplateInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Create / Edit modal
  const [showForm, setShowForm] = useState(false);
  const [editingTemplate, setEditingTemplate] = useState<TemplateInfo | null>(null);
  const [formName, setFormName] = useState('');
  const [formBody, setFormBody] = useState('');
  const [saving, setSaving] = useState(false);

  // Preview modal
  const [previewTemplate, setPreviewTemplate] = useState<TemplateInfo | null>(null);
  const [previewVars, setPreviewVars] = useState<Record<string, string>>({});
  const [previewResult, setPreviewResult] = useState<string | null>(null);
  const [previewing, setPreviewing] = useState(false);

  // Delete confirmation
  const [deleteId, setDeleteId] = useState<string | null>(null);

  const fetchTemplates = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await templatesApi.list({
        page: String(page),
        per_page: String(PAGE_SIZE),
      });
      setTemplates(res.templates || []);
      setTotal(res.total);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить шаблоны');
    } finally {
      setLoading(false);
    }
  }, [page]);

  useEffect(() => {
    fetchTemplates();
  }, [fetchTemplates]);

  // --- Create / Edit ---

  function openCreateForm() {
    setEditingTemplate(null);
    setFormName('');
    setFormBody('');
    setShowForm(true);
  }

  function openEditForm(tpl: TemplateInfo) {
    setEditingTemplate(tpl);
    setFormName(tpl.name);
    setFormBody(tpl.body);
    setShowForm(true);
  }

  function closeForm() {
    setShowForm(false);
    setEditingTemplate(null);
    setFormName('');
    setFormBody('');
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError('');
    try {
      if (editingTemplate) {
        await templatesApi.update(editingTemplate.id, { name: formName, body: formBody });
      } else {
        await templatesApi.create({ name: formName, body: formBody });
      }
      closeForm();
      await fetchTemplates();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось сохранить шаблон');
    } finally {
      setSaving(false);
    }
  }

  // --- Preview ---

  function openPreview(tpl: TemplateInfo) {
    setPreviewTemplate(tpl);
    const vars = tpl.variables?.length ? tpl.variables : extractVariables(tpl.body);
    const initial: Record<string, string> = {};
    vars.forEach((v) => { initial[v] = ''; });
    setPreviewVars(initial);
    setPreviewResult(null);
  }

  function closePreview() {
    setPreviewTemplate(null);
    setPreviewVars({});
    setPreviewResult(null);
  }

  async function handleRender() {
    if (!previewTemplate) return;
    setPreviewing(true);
    setError('');
    try {
      const res = await templatesApi.render(previewTemplate.id, previewVars);
      setPreviewResult(res.rendered_text);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось отрендерить шаблон');
    } finally {
      setPreviewing(false);
    }
  }

  // --- Submit for review ---

  async function handleSubmitForReview(id: string) {
    setError('');
    try {
      await templatesApi.submit(id);
      await fetchTemplates();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось отправить на модерацию');
    }
  }

  // --- Delete ---

  async function handleDelete(id: string) {
    setError('');
    try {
      await templatesApi.remove(id);
      setDeleteId(null);
      await fetchTemplates();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось удалить шаблон');
    }
  }

  // --- Table columns ---

  const columns: Column<TemplateInfo>[] = [
    { key: 'name', header: 'Название' },
    {
      key: 'status',
      header: 'Статус',
      render: (tpl) => {
        const cfg = STATUS_BADGE[tpl.status] || { variant: 'default' as const, label: tpl.status };
        return <Badge variant={cfg.variant}>{cfg.label}</Badge>;
      },
    },
    {
      key: 'body',
      header: 'Текст',
      render: (tpl) => (
        <span className="text-gray-600 max-w-[250px] inline-block truncate" title={tpl.body}>
          {truncate(tpl.body, 60)}
        </span>
      ),
    },
    {
      key: 'created_at',
      header: 'Создан',
      render: (tpl) => new Date(tpl.created_at).toLocaleDateString(),
    },
  ];

  const deleteTemplate = templates.find((t) => t.id === deleteId);

  if (loading && templates.length === 0) {
    return <div role="status">Загрузка шаблонов...</div>;
  }

  return (
    <div className="max-w-[900px]">
      <PageHeader
        title="Шаблоны"
        subtitle={total > 0 ? pluralize(total, 'шаблон', 'шаблона', 'шаблонов') : undefined}
        actions={
          <Button onClick={openCreateForm}>
            Создать шаблон
          </Button>
        }
      />

      {error && <p className="text-red-600 mb-4">{error}</p>}

      {/* Create / Edit modal */}
      <Modal
        open={showForm}
        onClose={closeForm}
        title={editingTemplate ? 'Редактировать шаблон' : 'Создать шаблон'}
      >
        <form onSubmit={handleSubmit}>
          <div className="mb-4">
            <Input
              label="Название *"
              type="text"
              value={formName}
              onChange={(e) => setFormName(e.target.value)}
              required
              placeholder="Например: Код подтверждения"
              className="w-full"
            />
          </div>

          <div className="mb-4">
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Текст шаблона *
            </label>
            <textarea
              value={formBody}
              onChange={(e) => setFormBody(e.target.value)}
              required
              rows={5}
              placeholder="Ваш код: {{code}}. Здравствуйте, {{name}}!"
              className="w-full rounded border border-gray-300 px-3 py-2 text-sm transition-colors focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary"
            />
            <p className="mt-1 text-xs text-gray-500">
              Используйте переменные в двойных фигурных скобках: {'{{name}}'}, {'{{code}}'}, {'{{company}}'}
            </p>
          </div>

          <div className="flex gap-2 justify-end">
            <Button type="button" variant="secondary" onClick={closeForm}>
              Отмена
            </Button>
            <Button type="submit" disabled={saving || !formName.trim() || !formBody.trim()}>
              {saving ? 'Сохранение...' : editingTemplate ? 'Сохранить' : 'Создать'}
            </Button>
          </div>
        </form>
      </Modal>

      {/* Preview modal */}
      <Modal
        open={!!previewTemplate}
        onClose={closePreview}
        title={`Превью: ${previewTemplate?.name || ''}`}
      >
        <div className="space-y-4">
          <div className="bg-gray-50 rounded p-3 text-sm">
            <p className="text-gray-500 text-xs mb-1">Исходный текст:</p>
            <p className="whitespace-pre-wrap">{previewTemplate?.body}</p>
          </div>

          {Object.keys(previewVars).length > 0 ? (
            <>
              <div className="space-y-3">
                {Object.keys(previewVars).map((varName) => (
                  <Input
                    key={varName}
                    label={varName}
                    type="text"
                    value={previewVars[varName]}
                    onChange={(e) =>
                      setPreviewVars((prev) => ({ ...prev, [varName]: e.target.value }))
                    }
                    placeholder={`Значение для {{${varName}}}`}
                    className="w-full"
                  />
                ))}
              </div>

              <Button onClick={handleRender} disabled={previewing}>
                {previewing ? 'Рендеринг...' : 'Показать'}
              </Button>
            </>
          ) : (
            <p className="text-sm text-gray-500">Шаблон не содержит переменных.</p>
          )}

          {previewResult !== null && (
            <div className="bg-green-50 border border-green-200 rounded p-3">
              <p className="text-xs text-green-700 mb-1">Результат:</p>
              <p className="whitespace-pre-wrap text-sm">{previewResult}</p>
            </div>
          )}
        </div>
      </Modal>

      {/* Delete confirmation */}
      <ConfirmDialog
        open={!!deleteId}
        onCancel={() => setDeleteId(null)}
        onConfirm={() => deleteId && handleDelete(deleteId)}
        title="Удалить шаблон"
        description={`Вы уверены, что хотите удалить шаблон${deleteTemplate ? ` "${deleteTemplate.name}"` : ''}? Это действие необратимо.`}
        confirmLabel="Удалить"
        variant="danger"
      />

      {/* Templates table */}
      <DataTable<TemplateInfo>
        columns={columns}
        data={templates}
        total={total}
        page={page}
        pageSize={PAGE_SIZE}
        onPageChange={setPage}
        loading={loading}
        keyField="id"
        rowActions={(tpl) => (
          <div className="flex gap-1 flex-wrap">
            <Button
              variant="ghost"
              size="sm"
              aria-label={`Превью ${tpl.name}`}
              onClick={() => openPreview(tpl)}
            >
              Превью
            </Button>

            {(tpl.status === 'draft' || tpl.status === 'revision_requested') && (
              <Button
                variant="primary"
                size="sm"
                aria-label={`Отправить ${tpl.name} на модерацию`}
                onClick={() => handleSubmitForReview(tpl.id)}
              >
                На модерацию
              </Button>
            )}

            {(tpl.status === 'draft' || tpl.status === 'revision_requested' || tpl.status === 'rejected') && (
              <Button
                variant="ghost"
                size="sm"
                aria-label={`Редактировать ${tpl.name}`}
                onClick={() => openEditForm(tpl)}
              >
                Редактировать
              </Button>
            )}

            <Button
              variant="ghost"
              size="sm"
              aria-label={`Удалить ${tpl.name}`}
              onClick={() => setDeleteId(tpl.id)}
            >
              Удалить
            </Button>

            {tpl.status === 'rejected' && tpl.rejection_reason && (
              <span
                className="inline-flex items-center text-xs text-red-600 ml-1"
                title={tpl.rejection_reason}
              >
                Причина: {truncate(tpl.rejection_reason, 30)}
              </span>
            )}
          </div>
        )}
      />
    </div>
  );
}
