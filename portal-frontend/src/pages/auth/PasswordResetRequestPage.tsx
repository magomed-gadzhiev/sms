import { useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { authApi, ApiError } from '../../api/client';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';

export function PasswordResetRequestPage() {
  const [email, setEmail] = useState('');
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError('');
    setSubmitting(true);
    try {
      await authApi.requestPasswordReset(email);
      setSubmitted(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось отправить запрос');
    } finally {
      setSubmitting(false);
    }
  }

  if (submitted) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <div className="max-w-md w-full p-8 bg-white rounded-lg shadow-sm border border-gray-200">
          <h2 className="text-2xl font-bold text-gray-900 mb-4">Проверьте почту</h2>
          <p className="text-sm text-gray-600 mb-4">Если аккаунт с таким email существует, ссылка для сброса пароля была отправлена.</p>
          <Link to="/login" className="text-primary hover:text-primary-dark text-sm">Вернуться к входу</Link>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="max-w-md w-full p-8 bg-white rounded-lg shadow-sm border border-gray-200">
        <h2 className="text-2xl font-bold text-gray-900 mb-6">Сброс пароля</h2>
        {error && (
          <p id="reset-request-error" role="alert" className="text-sm text-danger bg-red-50 border border-red-200 rounded p-3 mb-4">
            {error}
          </p>
        )}
        {submitting && <div role="status" className="text-sm text-gray-500 mb-4">Отправка...</div>}
        <form onSubmit={handleSubmit} className="space-y-4">
          <Input
            label="Электронная почта"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            autoFocus
            aria-describedby={error ? 'reset-request-error' : undefined}
          />
          <Button type="submit" disabled={submitting} className="w-full">
            {submitting ? 'Отправка...' : 'Отправить ссылку'}
          </Button>
        </form>
        <p className="mt-4 text-sm text-gray-600">
          <Link to="/login" className="text-primary hover:text-primary-dark">Вернуться к входу</Link>
        </p>
      </div>
    </div>
  );
}
