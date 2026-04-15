import { useEffect, useRef, useState } from 'react';
import { messagesApi, senderNamesApi, type SenderNameInfo } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { SearchableSelect } from '../../components/ui/SearchableSelect';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { useToast } from '../../components/ui/Toast';
import { CharacterCounter } from '../../components/ui/CharacterCounter';

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
    // strip whitespace, parentheses, dashes, dots — common in formats like +7(900)123-45-67
    .map((s) => s.replace(/[\s\-().]/g, ''))
    .filter((s) => s.length > 0);
}

function pluralMessages(n: number): string {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return `${n} сообщение`;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return `${n} сообщения`;
  return `${n} сообщений`;
}

export function QuickSendPage() {
  const toast = useToast();

  const [text, setText] = useState('');
  const [contacts, setContacts] = useState('');
  const [source, setSource] = useState('');
  const [senderNames, setSenderNames] = useState<SenderNameInfo[]>([]);
  const [loadingSenders, setLoadingSenders] = useState(true);

  const [sendersError, setSendersError] = useState(false);

  const [sending, setSending] = useState(false);
  const [formError, setFormError] = useState('');
  const [showConfirm, setShowConfirm] = useState(false);
  const [pendingPhones, setPendingPhones] = useState<string[]>([]);

  const [sentMessages, setSentMessages] = useState<SentMessage[]>([]);
  const [polling, setPolling] = useState(false);
  // Track consecutive poll attempts per message to avoid infinite 404 loops
  const pollAttemptsRef = useRef<Record<string, number>>({});

  const terminalStatuses = new Set(['delivered', 'failed', 'expired', 'rejected']);
  // A message is "done" when it reached a terminal status OR had a send error (err- prefix)
  const isMessageDone = (m: SentMessage) =>
    terminalStatuses.has(m.status) || m.message_id.startsWith('err-');
  const allDone = sentMessages.length > 0 && sentMessages.every(isMessageDone);

  const MAX_POLL_ATTEMPTS = 60; // 60 × 3s = 3 minutes max per message

  // Poll message statuses every 3s until all reach terminal state or timeout
  useEffect(() => {
    if (sentMessages.length === 0 || allDone) {
      setPolling(false);
      return;
    }
    setPolling(true);

    const timer = setInterval(async () => {
      const updated = await Promise.all(
        sentMessages.map(async (msg) => {
          if (isMessageDone(msg)) return msg;

          const attempts = (pollAttemptsRef.current[msg.message_id] ?? 0) + 1;
          pollAttemptsRef.current[msg.message_id] = attempts;

          // Stop polling this message after MAX_POLL_ATTEMPTS — treat as unknown
          if (attempts > MAX_POLL_ATTEMPTS) {
            return { ...msg, status: 'expired' };
          }

          try {
            const resp = await messagesApi.get(msg.message_id) as { status?: string };
            if (resp.status && resp.status !== msg.status) {
              return { ...msg, status: resp.status };
            }
          } catch {
            // keep current status on 404 or network error
          }
          return msg;
        }),
      );
      setSentMessages(updated);
    }, 3000);

    return () => clearInterval(timer);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sentMessages, allDone]);

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
      .catch(() => setSendersError(true))
      .finally(() => setLoadingSenders(false));
  }, []);

  const validateForm = (): string[] | null => {
    setFormError('');

    if (!text.trim()) {
      setFormError('Введите текст сообщения');
      return null;
    }
    if (!contacts.trim()) {
      setFormError('Вставьте номера получателей');
      return null;
    }
    if (!source) {
      setFormError('Выберите имя отправителя');
      return null;
    }

    const phones = parsePhones(contacts);
    if (phones.length === 0) {
      setFormError('Не удалось распознать ни одного номера');
      return null;
    }

    const MAX_RECIPIENTS = 500;
    if (phones.length > MAX_RECIPIENTS) {
      setFormError(`Слишком много получателей: ${phones.length}. Максимум — ${MAX_RECIPIENTS}. Используйте кампании для массовой рассылки.`);
      return null;
    }

    const phoneRegex = /^\+?[0-9]{10,15}$/;
    const invalid = phones.filter((p) => !phoneRegex.test(p));
    if (invalid.length > 0) {
      setFormError(`Некорректные номера: ${invalid.slice(0, 5).join(', ')}${invalid.length > 5 ? ` и ещё ${invalid.length - 5}` : ''}`);
      return null;
    }

    return phones;
  };

  const handleSend = () => {
    const phones = validateForm();
    if (!phones) return;
    setPendingPhones(phones);
    setShowConfirm(true);
  };

  const handleConfirmedSend = async () => {
    setShowConfirm(false);
    setSending(true);
    setSentMessages([]);
    pollAttemptsRef.current = {};

    let failedCount = 0;
    let sentCount = 0;
    for (const phone of pendingPhones) {
      let msg: SentMessage;
      try {
        const resp = await messagesApi.send({ destination: phone, text: text.trim(), source });
        msg = { message_id: resp.message_id, destination: phone, text: text.trim(), status: resp.status };
        sentCount++;
      } catch {
        // Normalize send error to 'failed' so StatusBadge renders correctly
        // and allDone includes this message via err- prefix on message_id
        msg = { message_id: `err-${phone}`, destination: phone, text: text.trim(), status: 'failed' };
        failedCount++;
      }
      // Stream results as they arrive — each send updates the table immediately
      setSentMessages((prev) => [...prev, msg]);
    }

    setSending(false);

    if (failedCount === 0) {
      toast.success(`Отправлено ${sentCount} сообщений`);
    } else {
      toast.error(`Отправлено ${sentCount}, ошибок: ${failedCount}`);
    }

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
              maxLength={765}
              aria-describedby="qs-text-counter"
            />
            <div id="qs-text-counter" className="flex justify-end">
              <CharacterCounter current={text.length} max={160} />
            </div>
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
            {loadingSenders ? (
              <>
                <span className="block text-sm font-medium text-gray-700 mb-1">Имя отправителя</span>
                <div className="h-9 bg-gray-100 rounded animate-pulse" />
              </>
            ) : sendersError ? (
              <>
                <span className="block text-sm font-medium text-gray-700 mb-1">Имя отправителя</span>
                <p className="text-sm text-red-600">
                  Не удалось загрузить имена отправителей.{' '}
                  <button className="underline" onClick={() => { setSendersError(false); setLoadingSenders(true); senderNamesApi.listApproved().then((r) => { setSenderNames(r.sender_names); if (r.sender_names.length > 0) setSource(r.sender_names[0].name); }).catch(() => setSendersError(true)).finally(() => setLoadingSenders(false)); }}>
                    Повторить
                  </button>
                </p>
              </>
            ) : senderNames.length === 0 ? (
              <>
                <span className="block text-sm font-medium text-gray-700 mb-1">Имя отправителя</span>
                <p className="text-sm text-gray-500">
                  Нет одобренных имён отправителей.{' '}
                  <a href="/sender-names" className="text-primary hover:underline">
                    Добавить
                  </a>
                </p>
              </>
            ) : (
              <SearchableSelect
                label="Имя отправителя"
                options={senderNames.map((sn) => ({ value: sn.name, label: sn.name }))}
                value={source}
                onChange={setSource}
                placeholder="Выберите отправителя..."
                disabled={sending}
              />
            )}
          </div>

          {formError && (
            <p className="text-sm text-red-600" role="alert">
              {formError}
            </p>
          )}

          <Button
            onClick={handleSend}
            disabled={sending || loadingSenders || sendersError || senderNames.length === 0}
            className="w-full"
          >
            {sending ? 'Отправка...' : 'Отправить'}
          </Button>
        </div>
      </div>

      <ConfirmDialog
        open={showConfirm}
        onConfirm={handleConfirmedSend}
        onCancel={() => setShowConfirm(false)}
        title="Подтвердите отправку"
        description={`Будет отправлено ${pluralMessages(pendingPhones.length)} с именем «${source}». Продолжить?`}
        confirmLabel="Отправить"
        variant="default"
      />

      {/* Results table */}
      {sentMessages.length > 0 && (
        <div ref={tableRef}>
          <div className="flex items-center gap-3 mb-3">
            <h2 className="text-base font-semibold text-gray-900">
              Результаты отправки ({sentMessages.length})
            </h2>
            {polling && (
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
                {sentMessages.map((msg) => (
                    <tr key={msg.message_id} className="hover:bg-gray-50">
                      <td className="px-4 py-3 font-mono text-xs">{msg.destination}</td>
                      <td className="px-4 py-3 max-w-xs truncate" title={msg.text}>{msg.text}</td>
                      <td className="px-4 py-3">
                        <StatusBadge status={msg.status} />
                      </td>
                    </tr>
                  ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
