import { useState, useEffect, useCallback, useRef, type FormEvent } from 'react';
import { templatesApi, senderNamesApi, ApiError, type TemplateInfo, type SenderNameInfo } from '../../api/client';
import { useFormValidation } from '../../hooks/useFormValidation';
import { CharacterCounter } from '../../components/ui/CharacterCounter';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
import { DataTable, type Column } from '../../components/data/DataTable';
import type { BulkAction } from '../../components/data/BulkActionBar';
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

  const tplValidation = useFormValidation({
    name: { required: true, minLength: 3, maxLength: 100 },
    body: { required: true, maxLength: 1600 },
  });

  // Approved sender names for dropdown
  const [approvedSenderNames, setApprovedSenderNames] = useState<SenderNameInfo[]>([]);
  const [sendersError, setSendersError] = useState(false);

  // Create / Edit modal
  const [showForm, setShowForm] = useState(false);
  const [editingTemplate, setEditingTemplate] = useState<TemplateInfo | null>(null);
  const [formName, setFormName] = useState('');
  const [formBody, setFormBody] = useState('');
  const [formSenderNameId, setFormSenderNameId] = useState('');
  const [saving, setSaving] = useState(false);
  const bodyRef = useRef<HTMLTextAreaElement>(null);

  // Preview modal
  const [previewTemplate, setPreviewTemplate] = useState<TemplateInfo | null>(null);
  const [previewVars, setPreviewVars] = useState<Record<string, string>>({});
  const [previewResult, setPreviewResult] = useState<string | null>(null);
  const [previewing, setPreviewing] = useState(false);
  const [previewError, setPreviewError] = useState('');

  // Submit for review loading
  const [submittingId, setSubmittingId] = useState<string | null>(null);

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

  function loadSenderNames() {
    setSendersError(false);
    senderNamesApi.listApproved()
      .then((res) => setApprovedSenderNames(res.sender_names || []))
      .catch(() => setSendersError(true));
  }

  useEffect(() => {
    loadSenderNames();
  }, []);

  // --- Create / Edit ---

  function openCreateForm() {
    tplValidation.reset();
    setEditingTemplate(null);
    setFormName('');
    setFormBody('');
    setFormSenderNameId('');
    setError('');
    setShowForm(true);
  }

  function openEditForm(tpl: TemplateInfo) {
    tplValidation.reset();
    setEditingTemplate(tpl);
    setFormName(tpl.name);
    setFormBody(tpl.body);
    setFormSenderNameId(tpl.sender_name_id || '');
    setError('');
    setShowForm(true);
  }

  function closeForm() {
    setShowForm(false);
    setEditingTemplate(null);
    setFormName('');
    setFormBody('');
    setFormSenderNameId('');
  }

  function insertAtCursor(token: string) {
    const ta = bodyRef.current;
    if (!ta) {
      setFormBody((prev) => prev + token);
      return;
    }
    const start = ta.selectionStart ?? formBody.length;
    const end = ta.selectionEnd ?? formBody.length;
    const next = formBody.slice(0, start) + token + formBody.slice(end);
    setFormBody(next);
    requestAnimationFrame(() => {
      ta.focus();
      const caret = start + token.length;
      ta.setSelectionRange(caret, caret);
    });
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const valid = tplValidation.validateAll({ name: formName, body: formBody });
    if (!valid) {
      tplValidation.scrollToFirstError();
      return;
    }
    setSaving(true);
    setError('');
    try {
      if (editingTemplate) {
        await templatesApi.update(editingTemplate.id, {
          name: formName,
          body: formBody,
          sender_name_id: formSenderNameId || undefined,
        });
      } else {
        await templatesApi.create({
          name: formName,
          body: formBody,
          sender_name_id: formSenderNameId || undefined,
        });
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
    setPreviewError('');
  }

  function closePreview() {
    setPreviewTemplate(null);
    setPreviewVars({});
    setPreviewResult(null);
    setPreviewError('');
  }

  async function handleRender() {
    if (!previewTemplate) return;
    setPreviewing(true);
    setPreviewError('');
    try {
      const res = await templatesApi.render(previewTemplate.id, previewVars);
      setPreviewResult(res.rendered_text);
    } catch (err) {
      setPreviewError(err instanceof ApiError ? err.message : 'Не удалось отрендерить шаблон');
    } finally {
      setPreviewing(false);
    }
  }

  // --- Submit for review ---

  async function handleSubmitForReview(id: string) {
    if (submittingId) return;
    setSubmittingId(id);
    setError('');
    try {
      await templatesApi.submit(id);
      await fetchTemplates();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось отправить на модерацию');
    } finally {
      setSubmittingId(null);
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
      key: 'sender_name',
      header: 'Отправитель',
      render: (tpl) =>
        tpl.sender_name ? (
          <span className="text-sm font-medium">{tpl.sender_name}</span>
        ) : (
          <span className="text-gray-400 text-sm">—</span>
        ),
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
      render: (tpl) => new Date(tpl.created_at).toLocaleDateString('ru-RU'),
    },
  ];

  const deleteTemplate = templates.find((t) => t.id === deleteId);

  return (
    <div className="w-full">
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
              onChange={(e) => { setFormName(e.target.value); tplValidation.fieldProps('name').onChange(e); }}
              onBlur={(e) => tplValidation.fieldProps('name').onBlur(e)}
              aria-invalid={tplValidation.errors.name ? true : undefined}
              aria-describedby={tplValidation.errors.name ? 'name-error' : undefined}
              required
              placeholder="Например: Код подтверждения"
              className={`w-full ${tplValidation.errors.name ? 'border-red-400 focus:border-red-400' : ''}`}
            />
            {tplValidation.errors.name && (
              <p id="name-error" className="mt-1 text-xs text-red-600">{tplValidation.errors.name}</p>
            )}
          </div>

          <div className="mb-4">
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Имя отправителя
            </label>
            {sendersError ? (
              <p className="text-xs text-red-600">
                Не удалось загрузить имена отправителей.{' '}
                <button type="button" className="underline" onClick={loadSenderNames}>Повторить</button>
              </p>
            ) : approvedSenderNames.length > 0 ? (
              <select
                value={formSenderNameId}
                onChange={(e) => setFormSenderNameId(e.target.value)}
                className="w-full rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary"
              >
                <option value="">— Без отправителя —</option>
                {approvedSenderNames.map((sn) => (
                  <option key={sn.id} value={sn.id}>{sn.name}</option>
                ))}
              </select>
            ) : (
              <p className="text-xs text-gray-500">
                Нет одобренных имён отправителей.{' '}
                <a href="/sender-names" className="text-primary underline">Зарегистрировать →</a>
              </p>
            )}
          </div>

          <div className="mb-4">
            <div className="flex items-center justify-between mb-1">
              <label htmlFor="tpl-body" className="block text-sm font-medium text-gray-700">
                Текст шаблона *
              </label>
              <CharacterCounter current={formBody.length} max={1600} />
            </div>
            <textarea
              id="tpl-body"
              ref={bodyRef}
              value={formBody}
              onChange={(e) => { setFormBody(e.target.value); tplValidation.fieldProps('body').onChange(e); }}
              onBlur={(e) => tplValidation.fieldProps('body').onBlur(e)}
              aria-invalid={tplValidation.errors.body ? true : undefined}
              aria-describedby={tplValidation.errors.body ? 'body-error' : 'body-hint'}
              required
              rows={5}
              placeholder="Ваш код: {{code}}. Здравствуйте, {{name}}!"
              className={`w-full rounded border px-3 py-2 text-sm transition-colors focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary ${tplValidation.errors.body ? 'border-red-400' : 'border-gray-300'}`}
            />
            <div className="mt-2 flex flex-wrap gap-1.5" role="toolbar" aria-label="Вставить переменную">
              {[
                { label: 'Уважаемый(ая)', token: 'Уважаем{ый|ая}' },
                { label: 'Имя', token: '{{name}}' },
                { label: 'Отчество', token: '{{lastname}}' },
                { label: 'Фамилия', token: '{{firstname}}' },
                { label: 'День рождения', token: '{{birthday}}' },
                { label: 'Промокод', token: '{{promocode}}' },
              ].map((chip) => (
                <button
                  key={chip.label}
                  type="button"
                  onClick={() => insertAtCursor(chip.token)}
                  className="px-2.5 py-1 text-xs rounded-full border border-gray-300 bg-gray-50 hover:bg-gray-100 hover:border-gray-400 text-gray-700 transition-colors"
                >
                  {chip.label}
                </button>
              ))}
            </div>
            {tplValidation.errors.body ? (
              <p id="body-error" className="mt-1 text-xs text-red-600">{tplValidation.errors.body}</p>
            ) : (
              <p id="body-hint" className="mt-1 text-xs text-gray-500">
                Значения подставляются из колонок контактной базы при отправке.
              </p>
            )}
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

          {previewTemplate?.status !== 'approved' && Object.keys(previewVars).length > 0 && (
            <p className="text-xs text-amber-600 bg-amber-50 border border-amber-200 rounded px-3 py-2">
              Рендеринг доступен только для одобренных шаблонов.
            </p>
          )}

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

              <Button onClick={handleRender} disabled={previewing || previewTemplate?.status !== 'approved'}>
                {previewing ? 'Рендеринг...' : 'Показать'}
              </Button>
            </>
          ) : (
            <p className="text-sm text-gray-500">Шаблон не содержит переменных.</p>
          )}

          {previewError && (
            <p className="text-sm text-red-600">{previewError}</p>
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

      {loading && templates.length === 0 && (
        <div role="status">Загрузка шаблонов...</div>
      )}

      {!loading && !error && templates.length === 0 && (
        <div className="text-center py-12 text-gray-500">
          <p className="mb-2">Шаблоны не созданы</p>
          <p className="text-sm mb-4">Создайте первый шаблон для отправки сообщений</p>
          <Button onClick={openCreateForm}>Создать шаблон</Button>
        </div>
      )}

      {/* Templates table */}
      {(loading || templates.length > 0) && (
      <DataTable<TemplateInfo>
        columns={columns}
        data={templates}
        total={total}
        page={page}
        pageSize={PAGE_SIZE}
        onPageChange={setPage}
        loading={loading}
        keyField="id"
        bulkActions={[
          {
            label: 'На проверку',
            variant: 'primary',
            requiresConfirmation: true,
            confirmMessage: (n) => `Отправить ${n} шаблонов на проверку?`,
            onAction: async (ids) => {
              const submittable = templates
                .filter((t) => ids.includes(t.id) && (t.status === 'draft' || t.status === 'revision_requested'))
                .map((t) => t.id);
              await Promise.all(submittable.map((id) => templatesApi.submit(id).catch(() => {})));
              await fetchTemplates();
            },
          } as BulkAction<TemplateInfo>,
          {
            label: 'Удалить',
            variant: 'danger',
            requiresConfirmation: true,
            confirmMessage: (n) => `Удалить ${n} шаблонов? Это действие необратимо.`,
            onAction: async (ids) => {
              await Promise.all(ids.map((id) => templatesApi.remove(id).catch(() => {})));
              await fetchTemplates();
            },
          } as BulkAction<TemplateInfo>,
        ]}
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
                disabled={submittingId === tpl.id}
                onClick={() => handleSubmitForReview(tpl.id)}
              >
                {submittingId === tpl.id ? 'Отправка...' : 'На модерацию'}
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
      )}
    </div>
  );
}
