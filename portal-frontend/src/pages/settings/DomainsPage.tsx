import { useState, useEffect } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { usePageTitle } from '../../contexts/PageTitleContext';
import { Input } from '../../components/ui/Input';
import { Button } from '../../components/ui/Button';

interface Domain {
  id: string;
  domain: string;
  status: string;
  dns_txt_record: string;
}

export function DomainsPage() {
  usePageTitle('Кастомные домены');
  const [domains, setDomains] = useState<Domain[]>([]);
  const [newDomain, setNewDomain] = useState('');
  const [error, setError] = useState('');
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
    const domain = newDomain.trim();
    if (!domain) {
      setError('Введите домен');
      return;
    }

    const domainRegex = /^(?!-)(?:[a-zA-Z0-9-]{1,63}\.)+[a-zA-Z]{2,}$/;
    if (!domainRegex.test(domain)) {
      setError('Введите корректный домен, например go.yourbrand.com');
      return;
    }

    setError('');
    await fetch('/portal/v1/settings/domains', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ domain }),
    });
    setNewDomain('');
    fetchDomains();
  };

  const deleteDomain = async (id: string) => {
    await fetch(`/portal/v1/settings/domains/${id}`, { method: 'DELETE', credentials: 'include' });
    fetchDomains();
  };

  const statusColors: Record<string, string> = {
    active: 'text-green-600 dark:text-green-400 bg-green-50 dark:bg-green-950/40',
    pending_dns: 'text-amber-600 bg-amber-50',
    pending_ssl: 'text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-950/40',
    failed: 'text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-950/40',
  };
  const statusLabels: Record<string, string> = {
    active: 'Активен',
    pending_dns: 'Ожидает DNS',
    pending_ssl: 'Ожидает SSL',
    failed: 'Ошибка',
  };

  return (
    <div>
      <PageHeader subtitle="Управление доменами для коротких ссылок" />

      <form
        className="flex flex-col sm:flex-row gap-2 mb-6"
        onSubmit={(e) => {
          e.preventDefault();
          addDomain();
        }}
      >
        <div className="flex-1">
          <Input
            id="new-domain"
            label="Новый домен"
            value={newDomain}
            onChange={(e) => setNewDomain(e.target.value)}
            placeholder="go.yourbrand.com"
            aria-describedby={error ? 'domain-error' : undefined}
          />
        </div>
        <div className="self-end">
          <Button type="submit">Добавить</Button>
        </div>
      </form>
      {error && (
        <p id="domain-error" role="alert" className="text-sm text-red-600 dark:text-red-400 -mt-4 mb-4">
          {error}
        </p>
      )}

      {loading ? (
        <p className="text-sm text-gray-500 dark:text-slate-400" role="status" aria-live="polite">Загрузка...</p>
      ) : domains.length === 0 ? (
        <p className="text-sm text-gray-500 dark:text-slate-400">Нет добавленных доменов</p>
      ) : (
        <div className="space-y-3">
          {domains.map((d) => (
            <div key={d.id} className="p-4 bg-white dark:bg-slate-900 border border-gray-200 dark:border-slate-700 rounded flex items-center justify-between">
              <div>
                <p className="font-medium text-sm">{d.domain}</p>
                <span className={`text-xs px-2 py-0.5 rounded ${statusColors[d.status] || 'text-gray-600 dark:text-slate-400 bg-gray-50 dark:bg-slate-950'}`}>
                  {statusLabels[d.status] || d.status}
                </span>
                {d.status === 'pending_dns' && (
                  <p className="text-xs text-gray-500 dark:text-slate-400 mt-1">
                    Добавьте TXT запись: <code className="bg-gray-100 dark:bg-slate-800 px-1">{d.dns_txt_record}</code>
                  </p>
                )}
              </div>
              <button
                onClick={() => deleteDomain(d.id)}
                className="text-sm text-red-600 dark:text-red-400 hover:text-red-800"
                aria-label={`Удалить домен ${d.domain}`}
              >
                Удалить
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
