import { useCallback, useEffect, useRef, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { companiesApi, ApiError, type CompanyInfo, type CompanyUpsertRequest } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Badge } from '../../components/ui/Badge';

function validateINN(inn: string): string {
  if (inn === '') return '';
  if (!/^\d+$/.test(inn)) return 'ИНН должен содержать только цифры';
  if (inn.length !== 10 && inn.length !== 12) return 'ИНН должен быть 10 или 12 цифр';
  return '';
}

export function CompanyDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [company, setCompany] = useState<CompanyInfo | null>(null);
  const [form, setForm] = useState<CompanyUpsertRequest>({ name: '' });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [detaching, setDetaching] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const successTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError('');
    try {
      const c = await companiesApi.get(id);
      setCompany(c);
      setForm({
        inn: c.inn ?? '',
        name: c.name,
        full_name: c.full_name ?? '',
        kpp: c.kpp ?? '',
        ogrn: c.ogrn ?? '',
        legal_address: c.legal_address ?? '',
        actual_address: c.actual_address ?? '',
        ceo_name: c.ceo_name ?? '',
        ceo_title: c.ceo_title ?? '',
        acting_basis: c.acting_basis ?? '',
        bank_name: c.bank_name ?? '',
        bank_bik: c.bank_bik ?? '',
        bank_corr_account: c.bank_corr_account ?? '',
        bank_account: c.bank_account ?? '',
        email: c.email ?? '',
        phone: c.phone ?? '',
      });
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { load(); }, [load]);

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    const innErr = validateINN(form.inn ?? '');
    if (innErr) { setError(innErr); return; }
    if (!form.name.trim()) { setError('Краткое наименование обязательно'); return; }
    setSaving(true);
    setError('');
    setSuccess('');
    try {
      const updated = await companiesApi.update(id, form);
      setCompany(updated);
      if (successTimerRef.current) clearTimeout(successTimerRef.current);
      setSuccess('Реквизиты сохранены');
      successTimerRef.current = setTimeout(() => setSuccess(''), 4000);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Ошибка сохранения');
    } finally {
      setSaving(false);
    }
  };

  const handleDetach = async () => {
    if (!id) return;
    if (!window.confirm('Отвязать компанию от вашего аккаунта? Это действие нельзя отменить.')) return;
    setDetaching(true);
    setError('');
    try {
      await companiesApi.detach(id);
      navigate('/companies');
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Ошибка отвязки компании');
    } finally {
      setDetaching(false);
    }
  };

  const set = (key: keyof CompanyUpsertRequest) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm((f) => ({ ...f, [key]: e.target.value }));

  if (loading) return <div className="p-8 text-gray-500">Загрузка...</div>;
  if (!company) return <div className="p-8 text-red-600">{error || 'Компания не найдена'}</div>;

  const isOffer = company.is_offer;

  return (
    <div>
      <PageHeader
        title={company.name}
        subtitle={
          isOffer
            ? 'Работа по договору-оферте'
            : company.inn
            ? `ИНН: ${company.inn}`
            : 'Компания без ИНН'
        }
        actions={
          <div className="flex items-center gap-2">
            {company.is_default && <Badge variant="success">Основная компания</Badge>}
            {isOffer && <Badge variant="default">Оферта</Badge>}
          </div>
        }
      />

      {error && <div className="mb-4 p-3 bg-red-50 text-red-700 rounded-md text-sm">{error}</div>}
      {success && <div className="mb-4 p-3 bg-green-50 text-green-700 rounded-md text-sm">{success}</div>}

      {isOffer ? (
        <div className="max-w-2xl">
          <div className="bg-blue-50 border border-blue-200 rounded-lg p-4 mb-6 text-sm text-blue-800">
            Эта компания работает по договору-оферте. Реквизиты не требуются.
            Для работы с именами отправителей вы можете использовать её без заполнения реквизитов.
          </div>
          <div className="flex justify-between">
            <Button type="button" variant="ghost" onClick={() => navigate('/companies')}>
              ← Назад
            </Button>
            {!company.is_default && (
              <Button
                type="button"
                variant="ghost"
                onClick={handleDetach}
                disabled={detaching}
              >
                {detaching ? 'Отвязка...' : 'Отвязать компанию'}
              </Button>
            )}
          </div>
        </div>
      ) : (
        <form onSubmit={handleSave} className="space-y-6 max-w-2xl">
          {/* Основные реквизиты */}
          <section className="bg-white rounded-lg border border-gray-200 p-6 space-y-4">
            <h3 className="text-sm font-semibold text-gray-700 uppercase tracking-wide">Основные реквизиты</h3>
            <div className="grid grid-cols-2 gap-4">
              <Input label="ИНН" value={form.inn ?? ''} onChange={set('inn')} maxLength={12} placeholder="10 или 12 цифр" />
              <Input label="КПП" value={form.kpp ?? ''} onChange={set('kpp')} maxLength={9} />
            </div>
            <Input label="Краткое наименование *" value={form.name} onChange={set('name')} />
            <Input label="Полное наименование" value={form.full_name ?? ''} onChange={set('full_name')} />
            <Input label="ОГРН/ОГРНИП" value={form.ogrn ?? ''} onChange={set('ogrn')} maxLength={15} />
          </section>

          {/* Руководитель */}
          <section className="bg-white rounded-lg border border-gray-200 p-6 space-y-4">
            <h3 className="text-sm font-semibold text-gray-700 uppercase tracking-wide">Руководитель</h3>
            <Input label="ФИО руководителя" value={form.ceo_name ?? ''} onChange={set('ceo_name')} placeholder="Иванов Иван Иванович" />
            <Input label="Должность" value={form.ceo_title ?? ''} onChange={set('ceo_title')} placeholder="Генеральный директор" />
            <Input label="Действует на основании" value={form.acting_basis ?? ''} onChange={set('acting_basis')} placeholder="Устава" />
          </section>

          {/* Адреса */}
          <section className="bg-white rounded-lg border border-gray-200 p-6 space-y-4">
            <h3 className="text-sm font-semibold text-gray-700 uppercase tracking-wide">Адреса</h3>
            <Input label="Юридический адрес" value={form.legal_address ?? ''} onChange={set('legal_address')} />
            <Input label="Фактический адрес" value={form.actual_address ?? ''} onChange={set('actual_address')} placeholder="Если отличается от юридического" />
          </section>

          {/* Банковские реквизиты */}
          <section className="bg-white rounded-lg border border-gray-200 p-6 space-y-4">
            <h3 className="text-sm font-semibold text-gray-700 uppercase tracking-wide">Банковские реквизиты</h3>
            <Input label="Наименование банка" value={form.bank_name ?? ''} onChange={set('bank_name')} />
            <div className="grid grid-cols-2 gap-4">
              <Input label="БИК" value={form.bank_bik ?? ''} onChange={set('bank_bik')} maxLength={9} />
              <Input label="Корр. счёт" value={form.bank_corr_account ?? ''} onChange={set('bank_corr_account')} maxLength={20} />
            </div>
            <Input label="Расчётный счёт" value={form.bank_account ?? ''} onChange={set('bank_account')} maxLength={20} />
          </section>

          {/* Контакты */}
          <section className="bg-white rounded-lg border border-gray-200 p-6 space-y-4">
            <h3 className="text-sm font-semibold text-gray-700 uppercase tracking-wide">Контакты</h3>
            <div className="grid grid-cols-2 gap-4">
              <Input label="Email" value={form.email ?? ''} onChange={set('email')} placeholder="info@company.ru" />
              <Input label="Телефон" value={form.phone ?? ''} onChange={set('phone')} placeholder="+7 (999) 000-00-00" />
            </div>
          </section>

          <div className="flex justify-between">
            <div className="flex gap-2">
              <Button type="button" variant="ghost" onClick={() => navigate('/companies')}>
                ← Назад
              </Button>
              {!company.is_default && (
                <Button
                  type="button"
                  variant="ghost"
                  onClick={handleDetach}
                  disabled={detaching}
                >
                  {detaching ? 'Отвязка...' : 'Отвязать компанию'}
                </Button>
              )}
            </div>
            <Button type="submit" disabled={saving}>
              {saving ? 'Сохранение...' : 'Сохранить реквизиты'}
            </Button>
          </div>
        </form>
      )}
    </div>
  );
}
