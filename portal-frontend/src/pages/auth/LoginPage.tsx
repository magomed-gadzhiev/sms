import { useState, type FormEvent } from 'react';
import { useNavigate, useLocation, Link } from 'react-router-dom';
import { useAuth } from '../../contexts/AuthContext';
import { ApiError } from '../../api/client';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';

export function LoginPage() {
  const { login, login2fa, isAuthenticated } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [totpCode, setTotpCode] = useState('');
  const [loginTicket, setLoginTicket] = useState<string | null>(null);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const from = (location.state as { from?: { pathname: string } })?.from?.pathname || '/dashboard';

  if (isAuthenticated) {
    navigate(from, { replace: true });
    return null;
  }

  async function handleLogin(e: FormEvent) {
    e.preventDefault();
    setError('');
    setSubmitting(true);
    try {
      const result = await login(email, password);
      if (result.requires2fa && result.loginTicket) {
        setLoginTicket(result.loginTicket);
      } else {
        const dest = result.role === 'admin' || result.role === 'superadmin' ? '/admin' : from;
        navigate(dest, { replace: true });
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Login failed');
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
      const dest = role === 'admin' || role === 'superadmin' ? '/admin' : from;
      navigate(dest, { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '2FA verification failed');
    } finally {
      setSubmitting(false);
    }
  }

  if (loginTicket) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <div className="max-w-md w-full p-8 bg-white rounded-lg shadow-sm border border-gray-200">
          <h2 className="text-2xl font-bold text-gray-900 mb-6">Two-Factor Authentication</h2>
          {error && (
            <p id="login-error" role="alert" className="text-sm text-danger bg-red-50 border border-red-200 rounded p-3 mb-4">
              {error}
            </p>
          )}
          <form onSubmit={handle2fa} className="space-y-4">
            <Input
              label="TOTP Code"
              type="text"
              value={totpCode}
              onChange={(e) => setTotpCode(e.target.value)}
              autoFocus
              required
              inputMode="numeric"
              pattern="[0-9]{6}"
              maxLength={6}
            />
            <Button type="submit" disabled={submitting} className="w-full">
              {submitting ? 'Verifying...' : 'Verify'}
            </Button>
          </form>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="max-w-md w-full p-8 bg-white rounded-lg shadow-sm border border-gray-200">
        <h2 className="text-2xl font-bold text-gray-900 mb-6">Login</h2>
        {error && (
          <p id="login-error" role="alert" className="text-sm text-danger bg-red-50 border border-red-200 rounded p-3 mb-4">
            {error}
          </p>
        )}
        {submitting && <div role="status" className="text-sm text-gray-500 mb-4">Logging in...</div>}
        <form onSubmit={handleLogin} className="space-y-4">
          <Input
            label="Email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            autoFocus
            error={error ? undefined : undefined}
            aria-describedby={error ? 'login-error' : undefined}
          />
          <Input
            label="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            aria-describedby={error ? 'login-error' : undefined}
          />
          <Button type="submit" disabled={submitting} className="w-full">
            {submitting ? 'Logging in...' : 'Login'}
          </Button>
        </form>
        <p className="mt-4 text-sm text-gray-600">
          <Link to="/reset-password-request" className="text-primary hover:text-primary-dark">
            Forgot password?
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
