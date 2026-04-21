import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { PageHeader } from '../../components/layout/PageHeader';
import { SegmentBuilder } from '../../components/segments/SegmentBuilder';

export function SegmentDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  // Route /segments/new идёт без параметра :id → id=undefined; /segments/:id даёт id='uuid'.
  const isNew = !id;

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [contactListIds, setContactListIds] = useState('');
  const [rules, setRules] = useState<{ operator: 'AND' | 'OR'; conditions: { field: string; op: string; value: string }[] }>({
    operator: 'AND', conditions: [],
  });
  const [estimatedCount, setEstimatedCount] = useState<number | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!isNew && id) {
      fetch(`/portal/v1/segments/${id}`, { credentials: 'include' })
        .then((r) => r.json())
        .then((data) => {
          setName(data.name || '');
          setDescription(data.description || '');
          setContactListIds((data.contact_list_ids || []).join(', '));
          if (data.rules) setRules(data.rules);
          setEstimatedCount(data.estimated_count);
        });
    }
  }, [id, isNew]);

  const save = async () => {
    setSaving(true);
    const body = {
      name,
      description,
      contact_list_ids: contactListIds.split(',').map((s: string) => s.trim()).filter(Boolean),
      rules,
    };
    const url = isNew ? '/portal/v1/segments' : `/portal/v1/segments/${id}`;
    const method = isNew ? 'POST' : 'PUT';
    const res = await fetch(url, {
      method, headers: { 'Content-Type': 'application/json' },
      credentials: 'include', body: JSON.stringify(body),
    });
    setSaving(false);
    if (res.ok) {
      const data = await res.json();
      if (isNew) navigate(`/segments/${data.id}`);
    }
  };

  const estimate = async () => {
    if (!id || isNew) return;
    const res = await fetch(`/portal/v1/segments/${id}/estimate`, {
      method: 'POST', credentials: 'include',
    });
    if (res.ok) {
      const data = await res.json();
      setEstimatedCount(data.estimated_count);
    }
  };

  return (
    <div>
      <PageHeader title={isNew ? 'Новый сегмент' : name} />
      <div className="max-w-2xl space-y-4">
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Название"
          className="w-full px-3 py-2 border border-gray-300 rounded text-sm" />
        <input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Описание"
          className="w-full px-3 py-2 border border-gray-300 rounded text-sm" />
        <input value={contactListIds} onChange={(e) => setContactListIds(e.target.value)}
          placeholder="ID списков контактов (через запятую)"
          className="w-full px-3 py-2 border border-gray-300 rounded text-sm" />

        <div className="p-4 border border-gray-200 rounded">
          <h3 className="text-sm font-semibold mb-3">Правила фильтрации</h3>
          <SegmentBuilder rules={rules} onChange={setRules} />
        </div>

        {estimatedCount !== null && (
          <p className="text-sm text-gray-600">Оценка: {estimatedCount} контактов</p>
        )}

        <div className="flex gap-2">
          <button onClick={save} disabled={saving}
            className="px-4 py-2 bg-primary text-white rounded text-sm hover:bg-primary/90 disabled:opacity-50">
            {saving ? 'Сохранение...' : 'Сохранить'}
          </button>
          {!isNew && (
            <button onClick={estimate}
              className="px-4 py-2 bg-gray-100 text-gray-700 rounded text-sm hover:bg-gray-200">
              Пересчитать
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
