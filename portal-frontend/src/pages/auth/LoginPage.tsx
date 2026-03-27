import { useState, type FormEvent } from 'react';
import { useNavigate, useLocation, Link } from 'react-router-dom';
import { useAuth } from '../../contexts/AuthContext';
import { ApiError } from '../../api/client';

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
      <div style={{ maxWidth: 400, margin: '80px auto' }}>
        <h2>Two-Factor Authentication</h2>
        {error && <p id="login-error" role="alert" style={{ color: '#d32f2f' }}>{error}</p>}
        <form onSubmit={handle2fa}>
          <div style={{ marginBottom: 12 }}>
            <label>
              TOTP Code
              <br />
              <input
                type="text"
                value={totpCode}
                onChange={(e) => setTotpCode(e.target.value)}
                autoFocus
                required
                inputMode="numeric"
                pattern="[0-9]{6}"
                maxLength={6}
              />
            </label>
          </div>
          <button type="submit" disabled={submitting}>
            {submitting ? 'Verifying...' : 'Verify'}
          </button>
        </form>
      </div>
    );
  }

  return (
    <div style={{ maxWidth: 400, margin: '80px auto' }}>
      <h2>Login</h2>
      {error && <p id="login-error" role="alert" style={{ color: '#d32f2f' }}>{error}</p>}
      {submitting && <div role="status">Logging in...</div>}
      <form onSubmit={handleLogin}>
        <div style={{ marginBottom: 12 }}>
          <label>
            Email
            <br />
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoFocus
              aria-describedby={error ? 'login-error' : undefined}
            />
          </label>
        </div>
        <div style={{ marginBottom: 12 }}>
          <label>
            Password
            <br />
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              aria-describedby={error ? 'login-error' : undefined}
            />
          </label>
        </div>
        <button type="submit" disabled={submitting}>
          {submitting ? 'Logging in...' : 'Login'}
        </button>
      </form>
      <p style={{ marginTop: 16 }}>
        <Link to="/reset-password-request">Forgot password?</Link>
      </p>
      <p style={{ marginTop: 8 }}>
        Нет аккаунта? <Link to="/register">Зарегистрироваться</Link>
      </p>
    </div>
  );
}
