import { useState, useEffect } from 'react';
import { templatesApi, ApiError, type TemplateInfo } from '../../api/client';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';

export interface TemplatePreviewModalProps {
  template: TemplateInfo | null;
  onClose: () => void;
}

/** Extract {{variable}} names from a template body */
function extractVariables(body: string): string[] {
  const matches = body.match(/\{\{(\w+)\}\}/g);
  if (!matches) return [];
  return [...new Set(matches.map((m) => m.replace(/[{}]/g, '')))];
}

export function TemplatePreviewModal({ template, onClose }: TemplatePreviewModalProps) {
  const [previewVars, setPreviewVars] = useState<Record<string, string>>({});
  const [previewResult, setPreviewResult] = useState<string | null>(null);
  const [previewing, setPreviewing] = useState(false);
  const [previewError, setPreviewError] = useState('');

  // Reset state when template changes
  useEffect(() => {
    if (!template) return;
    const vars = template.variables?.length ? template.variables : extractVariables(template.body);
    const initial: Record<string, string> = {};
    vars.forEach((v) => { initial[v] = ''; });
    setPreviewVars(initial);
    setPreviewResult(null);
    setPreviewError('');
  }, [template]);

  async function handleRender() {
    if (!template) return;
    setPreviewing(true);
    setPreviewError('');
    try {
      const res = await templatesApi.render(template.id, previewVars);
      setPreviewResult(res.rendered_text);
    } catch (err) {
      setPreviewError(err instanceof ApiError ? err.message : 'Не удалось отрендерить шаблон');
    } finally {
      setPreviewing(false);
    }
  }

  return (
    <Modal
      open={!!template}
      onClose={onClose}
      title={`Превью: ${template?.name || ''}`}
    >
      <div className="space-y-4">
        <div className="bg-gray-50 rounded p-3 text-sm">
          <p className="text-gray-500 text-xs mb-1">Исходный текст:</p>
          <p className="whitespace-pre-wrap">{template?.body}</p>
        </div>

        {template?.status !== 'approved' && Object.keys(previewVars).length > 0 && (
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

            <Button onClick={handleRender} disabled={previewing || template?.status !== 'approved'}>
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
  );
}
