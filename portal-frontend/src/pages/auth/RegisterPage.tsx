import { useState, useEffect, type FormEvent } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { authApi, ApiError } from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { destForRole } from '../../utils/authRedirect';

const PLANS = [
  { name: 'free', label: 'Free', price: '0₽' },
  { name: 'starter', label: 'Starter', price: '5 000₽' },
  { name: 'business', label: 'Business', price: '15 000₽' },
  { name: 'pro', label: 'Pro', price: '40 000₽' },
] as const;

export function RegisterPage() {
  const { isAuthenticated, refreshUser, role } = useAuth();
  const navigate = useNavigate();

  const [companyName, setCompanyName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [contactPerson, setContactPerson] = useState('');
  const [phone, setPhone] = useState('');
  const [planName, setPlanName] = useState<string>('free');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [touched, setTouched] = useState(false);

  useEffect(() => {
    if (isAuthenticated) {
      navigate(destForRole(role), { replace: true });
    }
  }, [isAuthenticated, role, navigate]);

  if (isAuthenticated) return null;

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setTouched(true);
    if (!companyName || !email || !password || password.length < 8) {
      setError('Заполните все обязательные поля');
      return;
    }
    setError('');
    setSubmitting(true);
    try {
      await authApi.register({
        email,
        password,
        company_name: companyName,
        ...(contactPerson ? { contact_person: contactPerson } : {}),
        ...(phone ? { phone } : {}),
        plan_name: planName,
      });
      const profile = await refreshUser();
      navigate(destForRole(profile.role), { replace: true });
    } catch (err) {
      if (err instanceof ApiError) {
        const msg = err.message.toLowerCase();
        if (msg.includes('already registered') || msg.includes('already exists')) {
          setError('Пользователь с таким email уже зарегистрирован');
        } else if (msg.includes('invalid') && msg.includes('email')) {
          setError('Некорректный email');
        } else if (msg.includes('password')) {
          setError('Пароль не соответствует требованиям');
        } else {
          setError(err.message);
        }
      } else {
        setError('Ошибка регистрации');
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="max-w-lg w-full p-8 bg-white rounded-lg shadow-sm border border-gray-200">
        <h2 className="text-2xl font-bold text-gray-900 mb-6">Создание аккаунта</h2>
        {error && (
          <p id="register-error" role="alert" className="text-sm text-danger bg-red-50 border border-red-200 rounded p-3 mb-4">
            {error}
          </p>
        )}
        <form onSubmit={handleSubmit} className="space-y-4">
          <Input
            label="Название компании *"
            type="text"
            value={companyName}
            onChange={(e) => setCompanyName(e.target.value)}
            required
            autoComplete="organization"
            autoFocus
            error={touched && !companyName ? 'Обязательное поле' : undefined}
            aria-describedby={error ? 'register-error' : undefined}
          />
          <Input
            label="Email *"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            autoComplete="email"
            error={touched && !email ? 'Обязательное поле' : undefined}
            aria-describedby={error ? 'register-error' : undefined}
          />
          <Input
            label="Пароль *"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            autoComplete="new-password"
            placeholder="минимум 8 символов"
            minLength={8}
            error={touched && password.length > 0 && password.length < 8 ? 'Минимум 8 символов' : touched && !password ? 'Обязательное поле' : undefined}
            aria-describedby={error ? 'register-error' : undefined}
          />
          <Input
            label="Контактное лицо"
            type="text"
            value={contactPerson}
            onChange={(e) => setContactPerson(e.target.value)}
            autoComplete="name"
          />
          <Input
            label="Телефон"
            type="tel"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            autoComplete="tel"
          />
          <fieldset className="border border-gray-300 rounded p-3">
            <legend className="text-sm font-medium text-gray-700 px-1">Тарифный план</legend>
            <div className="flex gap-3 flex-wrap mt-1">
              {PLANS.map((plan) => (
                <label
                  key={plan.name}
                  className={`flex items-center gap-1.5 px-3 py-1.5 border-2 rounded cursor-pointer text-sm transition-colors
                    ${planName === plan.name
                      ? 'border-primary bg-blue-50 font-semibold text-primary'
                      : 'border-gray-300 hover:border-gray-400 text-gray-700'}`}
                >
                  <input
                    type="radio"
                    name="plan"
                    value={plan.name}
                    checked={planName === plan.name}
                    onChange={() => setPlanName(plan.name)}
                    className="sr-only"
                  />
                  <span>{plan.label}</span>
                  <span className="text-xs text-gray-500">{plan.price}</span>
                </label>
              ))}
            </div>
          </fieldset>
          <Button type="submit" disabled={submitting} className="w-full">
            {submitting ? 'Регистрация...' : 'Зарегистрироваться'}
          </Button>
        </form>
        <p className="mt-4 text-sm text-gray-600">
          Уже есть аккаунт?{' '}
          <Link to="/login" className="text-primary hover:text-primary-dark">Войти</Link>
        </p>
      </div>
    </div>
  );
}
