import { useState, type FormEvent } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { authApi, ApiError } from '../../api/client';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';

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
      setError('Пароли не совпадают');
      return;
    }
    if (password.length < 8) {
      setError('Пароль должен содержать минимум 8 символов');
      return;
    }
    if (!token) {
      setError('Отсутствует токен сброса');
      return;
    }
    setSubmitting(true);
    try {
      await authApi.resetPassword(token, password);
      setDone(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось сбросить пароль');
    } finally {
      setSubmitting(false);
    }
  }

  if (done) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <div className="max-w-md w-full p-8 bg-white rounded-lg shadow-sm border border-gray-200">
          <h2 className="text-2xl font-bold text-gray-900 mb-4">Пароль сброшен</h2>
          <p className="text-sm text-gray-600 mb-4">Ваш пароль был успешно изменён.</p>
          <Link to="/login" className="text-primary hover:text-primary-dark text-sm">Перейти к входу</Link>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="max-w-md w-full p-8 bg-white rounded-lg shadow-sm border border-gray-200">
        <h2 className="text-2xl font-bold text-gray-900 mb-6">Новый пароль</h2>
        {error && (
          <p id="reset-error" role="alert" className="text-sm text-danger bg-red-50 border border-red-200 rounded p-3 mb-4">
            {error}
          </p>
        )}
        {submitting && <div role="status" className="text-sm text-gray-500 mb-4">Сброс...</div>}
        <form onSubmit={handleSubmit} className="space-y-4">
          <Input
            label="Новый пароль"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={8}
            autoFocus
            aria-describedby={error ? 'reset-error' : undefined}
          />
          <Input
            label="Подтверждение пароля"
            type="password"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            required
            minLength={8}
            aria-describedby={error ? 'reset-error' : undefined}
          />
          <Button type="submit" disabled={submitting} className="w-full">
            {submitting ? 'Сброс...' : 'Сбросить пароль'}
          </Button>
        </form>
        <p className="mt-4 text-sm text-gray-600">
          <Link to="/login" className="text-primary hover:text-primary-dark">Вернуться к входу</Link>
        </p>
      </div>
    </div>
  );
}
