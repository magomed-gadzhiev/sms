import { useState, type FormEvent } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { authApi, ApiError } from '../../api/client';

export function PasswordResetPage() {
  const [searchParams] = useSearchParams();
  const token = searchParams.get('token') || '';

  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [done, setDone] = useState(false);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError('');
    if (password !== confirm) {
      setError('Passwords do not match');
      return;
    }
    if (password.length < 8) {
      setError('Password must be at least 8 characters');
      return;
    }
    if (!token) {
      setError('Missing reset token');
      return;
    }
    setSubmitting(true);
    try {
      await authApi.resetPassword(token, password);
      setDone(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Reset failed');
    } finally {
      setSubmitting(false);
    }
  }

  if (done) {
    return (
      <div style={{ maxWidth: 400, margin: '80px auto' }}>
        <h2>Password Reset</h2>
        <p>Your password has been reset successfully.</p>
        <Link to="/login">Go to login</Link>
      </div>
    );
  }

  return (
    <div style={{ maxWidth: 400, margin: '80px auto' }}>
      <h2>Set New Password</h2>
      {error && <p id="reset-error" role="alert" style={{ color: '#d32f2f' }}>{error}</p>}
      {submitting && <div role="status">Resetting...</div>}
      <form onSubmit={handleSubmit}>
        <div style={{ marginBottom: 12 }}>
          <label>
            New Password
            <br />
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={8}
              autoFocus
              aria-describedby={error ? 'reset-error' : undefined}
            />
          </label>
        </div>
        <div style={{ marginBottom: 12 }}>
          <label>
            Confirm Password
            <br />
            <input
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              required
              minLength={8}
              aria-describedby={error ? 'reset-error' : undefined}
            />
          </label>
        </div>
        <button type="submit" disabled={submitting}>
          {submitting ? 'Resetting...' : 'Reset Password'}
        </button>
      </form>
      <p style={{ marginTop: 16 }}>
        <Link to="/login">Back to login</Link>
      </p>
    </div>
  );
}
