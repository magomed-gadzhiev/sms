import { useState, useEffect } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';

interface Domain {
  id: string;
  domain: string;
  status: string;
  dns_txt_record: string;
}

export function DomainsPage() {
  const [domains, setDomains] = useState<Domain[]>([]);
  const [newDomain, setNewDomain] = useState('');
  const [loading, setLoading] = useState(true);

  const fetchDomains = async () => {
    const res = await fetch('/portal/v1/settings/domains', { credentials: 'include' });
    if (res.ok) {
      const data = await res.json();
      setDomains(data.domains || []);
    }
    setLoading(false);
  };

  useEffect(() => { fetchDomains(); }, []);

  const addDomain = async () => {
    if (!newDomain.trim()) return;
    await fetch('/portal/v1/settings/domains', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ domain: newDomain }),
    });
    setNewDomain('');
    fetchDomains();
  };

  const deleteDomain = async (id: string) => {
    await fetch(`/portal/v1/settings/domains/${id}`, { method: 'DELETE', credentials: 'include' });
    fetchDomains();
  };

  const statusColors: Record<string, string> = {
    active: 'text-green-600 bg-green-50',
    pending_dns: 'text-amber-600 bg-amber-50',
    pending_ssl: 'text-blue-600 bg-blue-50',
    failed: 'text-red-600 bg-red-50',
  };

  return (
    <div>
      <PageHeader title="Кастомные домены" description="Управление доменами для коротких ссылок" />

      <div className="flex gap-2 mb-6">
        <input
          value={newDomain}
          onChange={(e) => setNewDomain(e.target.value)}
          placeholder="go.yourbrand.com"
          className="flex-1 px-3 py-2 border border-gray-300 rounded text-sm"
        />
        <button onClick={addDomain} className="px-4 py-2 bg-primary text-white rounded text-sm hover:bg-primary/90">
          Добавить
        </button>
      </div>

      {loading ? (
        <p className="text-sm text-gray-500">Загрузка...</p>
      ) : domains.length === 0 ? (
        <p className="text-sm text-gray-500">Нет добавленных доменов</p>
      ) : (
        <div className="space-y-3">
          {domains.map((d) => (
            <div key={d.id} className="p-4 bg-white border border-gray-200 rounded flex items-center justify-between">
              <div>
                <p className="font-medium text-sm">{d.domain}</p>
                <span className={`text-xs px-2 py-0.5 rounded ${statusColors[d.status] || 'text-gray-600 bg-gray-50'}`}>
                  {d.status}
                </span>
                {d.status === 'pending_dns' && (
                  <p className="text-xs text-gray-500 mt-1">
                    Добавьте TXT запись: <code className="bg-gray-100 px-1">{d.dns_txt_record}</code>
                  </p>
                )}
              </div>
              <button onClick={() => deleteDomain(d.id)} className="text-sm text-red-600 hover:text-red-800">
                Удалить
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
