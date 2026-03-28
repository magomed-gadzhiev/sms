import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { PageHeader } from '../../components/layout/PageHeader';

interface Segment {
  id: string;
  name: string;
  description: string;
  estimated_count: number;
  contact_list_ids: string[];
  created_at: string;
}

export function SegmentsPage() {
  const [segments, setSegments] = useState<Segment[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch('/portal/v1/segments', { credentials: 'include' })
      .then((r) => r.json())
      .then((data) => { setSegments(data.segments || []); setLoading(false); })
      .catch(() => setLoading(false));
  }, []);

  return (
    <div>
      <PageHeader title="Сегменты" description="Сохранённые сегменты для кампаний" />
      <div className="mb-4">
        <Link to="/segments/new" className="px-4 py-2 bg-primary text-white rounded text-sm hover:bg-primary/90">
          Создать сегмент
        </Link>
      </div>
      {loading ? (
        <p className="text-sm text-gray-500">Загрузка...</p>
      ) : segments.length === 0 ? (
        <p className="text-sm text-gray-500">Нет сегментов</p>
      ) : (
        <div className="space-y-2">
          {segments.map((s) => (
            <Link key={s.id} to={`/segments/${s.id}`}
              className="block p-4 bg-white border border-gray-200 rounded hover:border-gray-300">
              <div className="flex justify-between items-center">
                <div>
                  <p className="font-medium text-sm">{s.name}</p>
                  {s.description && <p className="text-xs text-gray-500 mt-0.5">{s.description}</p>}
                </div>
                <span className="text-sm text-gray-600">{s.estimated_count} контактов</span>
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
