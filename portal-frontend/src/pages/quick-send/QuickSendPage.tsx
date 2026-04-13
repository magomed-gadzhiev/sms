import { useEffect, useRef, useState } from 'react';
import { messagesApi, senderNamesApi, type SenderNameInfo } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { useMessageStream } from '../../hooks/useMessageStream';

const STATUS_LABELS: Record<string, string> = {
  pending: 'Ожидание',
  queued: 'В очереди',
  sent: 'Отправлено',
  delivered: 'Доставлено',
  failed: 'Ошибка',
  expired: 'Истекло',
  rejected: 'Отклонено',
};

function StatusBadge({ status }: { status: string }) {
  const label = STATUS_LABELS[status] || status;
  const colorMap: Record<string, string> = {
    delivered: 'bg-green-100 text-green-800',
    sent: 'bg-blue-100 text-blue-800',
    failed: 'bg-red-100 text-red-800',
    rejected: 'bg-red-100 text-red-800',
    expired: 'bg-gray-100 text-gray-600',
    queued: 'bg-yellow-100 text-yellow-800',
    pending: 'bg-yellow-100 text-yellow-800',
  };
  const cls = colorMap[status] ?? 'bg-gray-100 text-gray-700';
  return (
    <span className={`inline-block rounded px-2 py-0.5 text-xs font-medium ${cls}`}>
      {label}
    </span>
  );
}

interface SentMessage {
  message_id: string;
  destination: string;
  text: string;
  status: string;
}

function parsePhones(raw: string): string[] {
  return raw
    .split(/[\n,;]+/)
    .map((s) => s.replace(/\s/g, ''))
    .filter((s) => s.length > 0);
}

export function QuickSendPage() {
  const [text, setText] = useState('');
  const [contacts, setContacts] = useState('');
  const [source, setSource] = useState('');
  const [senderNames, setSenderNames] = useState<SenderNameInfo[]>([]);
  const [loadingSenders, setLoadingSenders] = useState(true);

  const [sending, setSending] = useState(false);
  const [formError, setFormError] = useState('');

  const [sentMessages, setSentMessages] = useState<SentMessage[]>([]);

  const { streamStatus, updates: liveUpdates } = useMessageStream(sentMessages.length > 0);

  // Stop tracking after all messages reach a terminal state
  const terminalStatuses = new Set(['delivered', 'failed', 'expired', 'rejected']);
  const allDone =
    sentMessages.length > 0 &&
    sentMessages.every((m) => {
      const live = liveUpdates[m.message_id];
      return terminalStatuses.has(live ? live.status : m.status);
    });

  const tableRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    senderNamesApi
      .listApproved()
      .then((resp) => {
        setSenderNames(resp.sender_names);
        if (resp.sender_names.length > 0) {
          setSource(resp.sender_names[0].name);
        }
      })
      .catch(() => {})
      .finally(() => setLoadingSenders(false));
  }, []);

  const handleSend = async () => {
    setFormError('');

    if (!text.trim()) {
      setFormError('Введите текст сообщения');
      return;
    }
    if (!contacts.trim()) {
      setFormError('Вставьте номера получателей');
      return;
    }
    if (!source) {
      setFormError('Выберите имя отправителя');
      return;
    }

    const phones = parsePhones(contacts);
    if (phones.length === 0) {
      setFormError('Не удалось распознать ни одного номера');
      return;
    }

    const phoneRegex = /^\+?[0-9]{10,15}$/;
    const invalid = phones.filter((p) => !phoneRegex.test(p));
    if (invalid.length > 0) {
      setFormError(`Некорректные номера: ${invalid.slice(0, 5).join(', ')}${invalid.length > 5 ? ` и ещё ${invalid.length - 5}` : ''}`);
      return;
    }

    setSending(true);
    setSentMessages([]);

    const results: SentMessage[] = [];
    for (const phone of phones) {
      try {
        const resp = await messagesApi.send({ destination: phone, text: text.trim(), source });
        results.push({ message_id: resp.message_id, destination: phone, text: text.trim(), status: resp.status });
      } catch (err) {
        const msg = err instanceof Error ? err.message : 'Ошибка';
        results.push({ message_id: `err-${phone}`, destination: phone, text: text.trim(), status: `failed: ${msg}` });
      }
    }

    setSentMessages(results);
    setSending(false);

    // Scroll to results
    setTimeout(() => tableRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' }), 100);
  };

  return (
    <div>
      <PageHeader title="Быстрая отправка" />

      <div className="max-w-2xl bg-white border border-gray-200 rounded-lg p-6 mb-8">
        <div className="space-y-5">
          {/* Текст сообщения */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1" htmlFor="qs-text">
              Текст сообщения
            </label>
            <textarea
              id="qs-text"
              className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary"
              rows={4}
              placeholder="Введите текст SMS..."
              value={text}
              onChange={(e) => setText(e.target.value)}
              disabled={sending}
            />
          </div>

          {/* Контакты */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1" htmlFor="qs-contacts">
              Контакты
            </label>
            <p className="text-xs text-gray-500 mb-1">Вставьте номера списком — по одному на строку (или через запятую)</p>
            <textarea
              id="qs-contacts"
              className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm resize-none font-mono focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary"
              rows={5}
              placeholder={"+79001234567\n+79007654321\n..."}
              value={contacts}
              onChange={(e) => setContacts(e.target.value)}
              disabled={sending}
            />
          </div>

          {/* Имя отправителя */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1" htmlFor="qs-source">
              Имя отправителя
            </label>
            {loadingSenders ? (
              <div className="h-9 bg-gray-100 rounded animate-pulse" />
            ) : senderNames.length === 0 ? (
              <p className="text-sm text-gray-500">
                Нет одобренных имён отправителей.{' '}
                <a href="/sender-names" className="text-primary hover:underline">
                  Добавить
                </a>
              </p>
            ) : (
              <select
                id="qs-source"
                className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm bg-white focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary"
                value={source}
                onChange={(e) => setSource(e.target.value)}
                disabled={sending}
              >
                {senderNames.map((sn) => (
                  <option key={sn.id} value={sn.name}>
                    {sn.name}
                  </option>
                ))}
              </select>
            )}
          </div>

          {formError && (
            <p className="text-sm text-red-600" role="alert">
              {formError}
            </p>
          )}

          <Button
            onClick={handleSend}
            disabled={sending || loadingSenders || senderNames.length === 0}
            className="w-full"
          >
            {sending ? 'Отправка...' : 'Отправить'}
          </Button>
        </div>
      </div>

      {/* Results table */}
      {sentMessages.length > 0 && (
        <div ref={tableRef}>
          <div className="flex items-center gap-3 mb-3">
            <h2 className="text-base font-semibold text-gray-900">
              Результаты отправки ({sentMessages.length})
            </h2>
            {!allDone && streamStatus === 'connected' && (
              <span className="flex items-center gap-1 text-xs text-green-600" title="Статусы обновляются в реальном времени">
                <span className="inline-block w-2 h-2 rounded-full bg-green-500 animate-pulse" />
                Live
              </span>
            )}
          </div>
          <div className="bg-white border border-gray-200 rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 border-b border-gray-200">
                <tr>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Получатель</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Текст</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Статус</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {sentMessages.map((msg) => {
                  const live = liveUpdates[msg.message_id];
                  const currentStatus = live ? live.status : msg.status;
                  return (
                    <tr key={msg.message_id} className="hover:bg-gray-50">
                      <td className="px-4 py-3 font-mono text-xs">{msg.destination}</td>
                      <td className="px-4 py-3 max-w-xs truncate" title={msg.text}>{msg.text}</td>
                      <td className="px-4 py-3">
                        <StatusBadge status={currentStatus} />
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
