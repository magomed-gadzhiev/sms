import { useState } from 'react';

interface PreviewResult {
  rendered: string;
  length: number;
  segments: number;
  warnings: string[];
  error: string;
}

interface TemplatePreviewProps {
  templateText: string;
  contactListId?: string;
}

export function TemplatePreview({ templateText, contactListId }: TemplatePreviewProps) {
  const [results, setResults] = useState<PreviewResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const fetchPreview = async () => {
    if (!templateText.trim()) return;
    setLoading(true);
    setError('');
    try {
      const res = await fetch('/portal/v1/campaigns/templates/preview', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          template_text: templateText,
          contact_list_id: contactListId || '',
          test_data: [
            { bindings: { name: 'Иван', phone: '79991234567', vip: 'true', discount: '15' } },
            { bindings: { name: 'Мария', phone: '79997654321', vip: 'false' } },
            { bindings: {} },
          ],
        }),
      });
      if (!res.ok) throw new Error('Ошибка загрузки превью');
      const data = await res.json();
      setResults(data.results || []);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Ошибка');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="mt-4 space-y-3">
      <button
        type="button"
        onClick={fetchPreview}
        disabled={loading || !templateText.trim()}
        className="px-3 py-1.5 text-sm bg-gray-100 hover:bg-gray-200 rounded border border-gray-300 disabled:opacity-50"
      >
        {loading ? 'Загрузка...' : 'Превью шаблона'}
      </button>

      {error && <p className="text-sm text-red-600">{error}</p>}

      {results.length > 0 && (
        <div className="space-y-2">
          <h4 className="text-sm font-medium text-gray-700">Результат рендеринга:</h4>
          {results.map((r, i) => (
            <div key={i} className="p-3 bg-white border border-gray-200 rounded text-sm">
              {r.error ? (
                <p className="text-red-600">{r.error}</p>
              ) : (
                <>
                  <p className="whitespace-pre-wrap text-gray-900">{r.rendered}</p>
                  <p className="mt-1 text-xs text-gray-500">
                    {r.length} симв. / {r.segments} {r.segments === 1 ? 'сегмент' : 'сегментов'}
                  </p>
                  {r.warnings?.map((w, j) => (
                    <p key={j} className="mt-1 text-xs text-amber-600">{w}</p>
                  ))}
                </>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
