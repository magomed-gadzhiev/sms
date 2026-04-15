import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';

type Status = 'idle' | 'sending' | 'success' | 'error';

export function ContactForm() {
  const { t } = useTranslation();
  const [status, setStatus] = useState<Status>('idle');
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [company, setCompany] = useState('');
  const [message, setMessage] = useState('');

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setStatus('sending');
    try {
      const res = await fetch('/portal/v1/contact', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, email, company, message }),
      });
      if (!res.ok) throw new Error('Request failed');
      setStatus('success');
      setName('');
      setEmail('');
      setCompany('');
      setMessage('');
    } catch {
      setStatus('error');
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-5">
      <input
        type="text"
        required
        placeholder={t('contact.name')}
        value={name}
        onChange={(e) => setName(e.target.value)}
        className="w-full rounded-xl border border-gray-300 px-4 py-3 text-sm outline-none transition focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200"
      />
      <input
        type="email"
        required
        placeholder={t('contact.email')}
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        className="w-full rounded-xl border border-gray-300 px-4 py-3 text-sm outline-none transition focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200"
      />
      <input
        type="text"
        placeholder={t('contact.company')}
        value={company}
        onChange={(e) => setCompany(e.target.value)}
        className="w-full rounded-xl border border-gray-300 px-4 py-3 text-sm outline-none transition focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200"
      />
      <textarea
        required
        rows={5}
        placeholder={t('contact.message')}
        value={message}
        onChange={(e) => setMessage(e.target.value)}
        className="w-full rounded-xl border border-gray-300 px-4 py-3 text-sm outline-none transition focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200"
      />

      <button
        type="submit"
        disabled={status === 'sending'}
        className="w-full rounded-xl bg-gradient-to-r from-indigo-600 to-purple-600 px-6 py-3 text-sm font-semibold text-white shadow-md transition hover:opacity-90 disabled:opacity-60"
      >
        {status === 'sending' ? t('contact.sending') : t('contact.send')}
      </button>

      {status === 'success' && (
        <p className="rounded-lg bg-green-50 px-4 py-3 text-sm text-green-700">
          {t('contact.success')}
        </p>
      )}
      {status === 'error' && (
        <p className="rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">
          {t('contact.error')}
        </p>
      )}
    </form>
  );
}
