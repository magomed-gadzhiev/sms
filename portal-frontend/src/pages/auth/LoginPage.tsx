import { useState, useEffect, type FormEvent } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { useAuth } from '../../contexts/AuthContext';
import { ApiError } from '../../api/client';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';

export function LoginPage() {
  const { login, login2fa, isAuthenticated } = useAuth();
  const navigate = useNavigate();

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [totpCode, setTotpCode] = useState('');
  const [loginTicket, setLoginTicket] = useState<string | null>(null);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (isAuthenticated) {
      navigate('/dashboard', { replace: true });
    }
  }, [isAuthenticated, navigate]);

  if (isAuthenticated) return null;

  async function handleLogin(e: FormEvent) {
    e.preventDefault();
    setError('');

    if (!email.trim() || !password.trim()) {
      setError('Заполните все поля');
      return;
    }

    setSubmitting(true);
    try {
      const result = await login(email, password);
      if (result.requires2fa && result.loginTicket) {
        setLoginTicket(result.loginTicket);
      } else {
        const dest = result.role === 'admin' || result.role === 'superadmin' ? '/admin' : '/dashboard';
        navigate(dest, { replace: true });
      }
    } catch (err) {
      if (err instanceof ApiError) {
        const msg = err.message.toLowerCase();
        setError(msg.includes('invalid credentials') || msg.includes('unauthorized')
          ? 'Неверный email или пароль'
          : err.message);
      } else {
        setError('Ошибка входа');
      }
    } finally {
      setSubmitting(false);
    }
  }

  async function handle2fa(e: FormEvent) {
    e.preventDefault();
    setError('');
    setSubmitting(true);
    try {
      const role = await login2fa(loginTicket!, totpCode);
      const dest = role === 'admin' || role === 'superadmin' ? '/admin' : '/dashboard';
      navigate(dest, { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Ошибка проверки 2FA');
    } finally {
      setSubmitting(false);
    }
  }

  if (loginTicket) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <div className="max-w-md w-full p-8 bg-white rounded-lg shadow-sm border border-gray-200">
          <h2 className="text-2xl font-bold text-gray-900 mb-6">Двухфакторная аутентификация</h2>
          {error && (
            <p id="login-error" role="alert" className="text-sm text-danger bg-red-50 border border-red-200 rounded p-3 mb-4">
              {error}
            </p>
          )}
          <form onSubmit={handle2fa} className="space-y-4">
            <Input
              label="Код TOTP"
              type="text"
              value={totpCode}
              onChange={(e) => setTotpCode(e.target.value)}
              autoComplete="one-time-code"
              autoFocus
              required
              inputMode="numeric"
              pattern="[0-9]{6}"
              maxLength={6}
            />
            <Button type="submit" disabled={submitting} className="w-full">
              {submitting ? 'Проверка...' : 'Подтвердить'}
            </Button>
          </form>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="max-w-md w-full p-8 bg-white rounded-lg shadow-sm border border-gray-200">
        <h2 className="text-2xl font-bold text-gray-900 mb-6">Вход</h2>
        {error && (
          <p id="login-error" role="alert" className="text-sm text-danger bg-red-50 border border-red-200 rounded p-3 mb-4">
            {error}
          </p>
        )}
        {submitting && <div role="status" className="text-sm text-gray-500 mb-4">Выполняется вход...</div>}
        <form onSubmit={handleLogin} className="space-y-4">
          <Input
            id="email"
            label="Электронная почта"
            type="email"
            placeholder="name@example.com"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            autoComplete="email"
            autoFocus
            error={error ? undefined : undefined}
            aria-describedby={error ? 'login-error' : undefined}
          />
          <Input
            id="password"
            label="Пароль"
            type="password"
            placeholder="Ваш пароль"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            autoComplete="current-password"
            aria-describedby={error ? 'login-error' : undefined}
          />
          <Button type="submit" disabled={submitting} className="w-full">
            {submitting ? 'Вход...' : 'Войти'}
          </Button>
        </form>
        <p className="mt-4 text-sm text-gray-600">
          <Link to="/reset-password-request" className="text-primary hover:text-primary-dark">
            Забыли пароль?
          </Link>
        </p>
        <p className="mt-2 text-sm text-gray-600">
          Нет аккаунта?{' '}
          <Link to="/register" className="text-primary hover:text-primary-dark">
            Зарегистрироваться
          </Link>
        </p>
      </div>
    </div>
  );
}
