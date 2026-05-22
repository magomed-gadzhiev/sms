import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { QuickSendPage } from './QuickSendPage';

const sendMock = vi.fn();
const listApprovedMock = vi.fn();
const getMock = vi.fn();
const toastSuccess = vi.fn();
const toastError = vi.fn();

vi.mock('../../api/client', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('../../api/client');
  return {
    ...actual,
    messagesApi: { send: (...args: unknown[]) => sendMock(...args), get: (...args: unknown[]) => getMock(...args), list: vi.fn() },
    senderNamesApi: { listApproved: () => listApprovedMock() },
  };
});

vi.mock('../../components/ui/Toast', () => ({
  useToast: () => ({ success: toastSuccess, error: toastError, info: vi.fn(), warning: vi.fn() }),
}));

// Minimal stubs for UI components the page imports — avoid transitive dep surprises.
vi.mock('../../components/layout/PageHeader', () => ({
  PageHeader: ({ title }: { title: string }) => <h1>{title}</h1>,
}));

vi.mock('../../components/ui/Button', () => ({
  Button: ({ children, onClick, disabled, ...rest }: {
    children: React.ReactNode;
    onClick?: () => void;
    disabled?: boolean;
  } & React.ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button onClick={onClick} disabled={disabled} {...rest}>{children}</button>
  ),
}));

vi.mock('../../components/ui/SearchableSelect', () => ({
  SearchableSelect: ({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void; options: { value: string; label: string }[] }) => (
    <label>
      {label}
      <input value={value} onChange={(e) => onChange(e.target.value)} aria-label={label} />
    </label>
  ),
}));

vi.mock('../../components/ui/ConfirmDialog', () => ({
  ConfirmDialog: ({ open, onConfirm, onCancel, description, confirmLabel }: {
    open: boolean;
    onConfirm: () => void;
    onCancel: () => void;
    description: string;
    confirmLabel: string;
  }) =>
    open ? (
      <div role="dialog" aria-label="confirm">
        <p>{description}</p>
        <button onClick={onConfirm}>{confirmLabel}</button>
        <button onClick={onCancel}>Отмена</button>
      </div>
    ) : null,
}));

vi.mock('../../components/ui/CharacterCounter', () => ({
  CharacterCounter: ({ current, max }: { current: number; max: number }) => (
    <span data-testid="cc">{current}/{max}</span>
  ),
}));

function renderPage() {
  return render(
    <MemoryRouter>
      <QuickSendPage />
    </MemoryRouter>,
  );
}

async function waitForSendersLoaded() {
  await screen.findByLabelText(/Имя отправителя/i);
}

describe('QuickSendPage (US1 — Send Single SMS)', () => {
  beforeEach(() => {
    sendMock.mockReset();
    getMock.mockReset();
    listApprovedMock.mockReset();
    toastSuccess.mockReset();
    toastError.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('loads approved sender names and auto-selects the first one', async () => {
    listApprovedMock.mockResolvedValue({
      sender_names: [{ id: '1', name: 'MYBRAND', status: 'approved' }],
      total: 1, page: 1, per_page: 100, total_pages: 1,
    });
    renderPage();
    await waitForSendersLoaded();
    await waitFor(() => {
      expect(screen.getByLabelText(/Имя отправителя/i)).toHaveValue('MYBRAND');
    });
  });

  it('shows a disabled submit and helpful hint when no approved senders exist', async () => {
    listApprovedMock.mockResolvedValue({ sender_names: [], total: 0, page: 1, per_page: 100, total_pages: 0 });
    renderPage();
    expect(await screen.findByText(/Нет одобренных имён отправителей/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Отправить/i })).toBeDisabled();
  });

  it('shows retry hint when sender-names load fails', async () => {
    listApprovedMock.mockRejectedValue(new Error('boom'));
    renderPage();
    expect(await screen.findByText(/Не удалось загрузить имена отправителей/i)).toBeInTheDocument();
  });

  it('requires text before sending', async () => {
    listApprovedMock.mockResolvedValue({
      sender_names: [{ id: '1', name: 'BRAND', status: 'approved' }],
      total: 1, page: 1, per_page: 100, total_pages: 1,
    });
    renderPage();
    await waitForSendersLoaded();
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: /Отправить/i }));
    expect(screen.getByRole('alert')).toHaveTextContent(/Введите текст/i);
    expect(sendMock).not.toHaveBeenCalled();
  });

  it('requires contacts before sending', async () => {
    listApprovedMock.mockResolvedValue({
      sender_names: [{ id: '1', name: 'BRAND', status: 'approved' }],
      total: 1, page: 1, per_page: 100, total_pages: 1,
    });
    renderPage();
    await waitForSendersLoaded();
    const user = userEvent.setup();
    await user.type(screen.getByLabelText(/Текст сообщения/i), 'Hello');
    await user.click(screen.getByRole('button', { name: /Отправить/i }));
    expect(screen.getByRole('alert')).toHaveTextContent(/Вставьте номера/i);
  });

  it('flags invalid phone numbers with a specific message', async () => {
    listApprovedMock.mockResolvedValue({
      sender_names: [{ id: '1', name: 'BRAND', status: 'approved' }],
      total: 1, page: 1, per_page: 100, total_pages: 1,
    });
    renderPage();
    await waitForSendersLoaded();
    const user = userEvent.setup();
    await user.type(screen.getByLabelText(/Текст сообщения/i), 'Hello');
    await user.type(screen.getByLabelText(/Контакты/i), 'bad-phone');
    await user.click(screen.getByRole('button', { name: /Отправить/i }));
    expect(screen.getByRole('alert')).toHaveTextContent(/Некорректные номера/i);
    expect(sendMock).not.toHaveBeenCalled();
  });

  it('rejects batches over MAX_RECIPIENTS (500) with a clear size error', async () => {
    listApprovedMock.mockResolvedValue({
      sender_names: [{ id: '1', name: 'BRAND', status: 'approved' }],
      total: 1, page: 1, per_page: 100, total_pages: 1,
    });
    renderPage();
    await waitForSendersLoaded();
    const user = userEvent.setup();
    await user.type(screen.getByLabelText(/Текст сообщения/i), 'Hi');
    const phones = Array.from({ length: 501 }, (_, i) => `+7900${String(1000000 + i).padStart(7, '0')}`).join('\n');
    // userEvent.type for 501 phones is too slow under fake timers — paste instead.
    const textarea = screen.getByLabelText(/Контакты/i) as HTMLTextAreaElement;
    await user.click(textarea);
    await user.paste(phones);
    await user.click(screen.getByRole('button', { name: /Отправить/i }));
    expect(screen.getByRole('alert')).toHaveTextContent(/Слишком много получателей/i);
  });

  it('opens confirm dialog, submits, and records each sent message in the results table', async () => {
    listApprovedMock.mockResolvedValue({
      sender_names: [{ id: '1', name: 'BRAND', status: 'approved' }],
      total: 1, page: 1, per_page: 100, total_pages: 1,
    });
    sendMock
      .mockResolvedValueOnce({ message_id: 'm1', status: 'queued' })
      .mockResolvedValueOnce({ message_id: 'm2', status: 'queued' });
    renderPage();
    await waitForSendersLoaded();
    const user = userEvent.setup();

    await user.type(screen.getByLabelText(/Текст сообщения/i), 'Hello');
    await user.click(screen.getByLabelText(/Контакты/i));
    await user.paste('+79001111111\n+79002222222');
    await user.click(screen.getByRole('button', { name: /Отправить/i }));

    // Confirm dialog.
    const dialog = await screen.findByRole('dialog', { name: 'confirm' });
    await user.click(within(dialog).getByRole('button', { name: 'Отправить' }));

    await waitFor(() => expect(sendMock).toHaveBeenCalledTimes(2));
    expect(sendMock).toHaveBeenNthCalledWith(1, { destination: '+79001111111', text: 'Hello', source: 'BRAND' });
    expect(sendMock).toHaveBeenNthCalledWith(2, { destination: '+79002222222', text: 'Hello', source: 'BRAND' });

    // Results table.
    expect(await screen.findByText(/Результаты отправки \(2\)/)).toBeInTheDocument();
    expect(screen.getByText('+79001111111')).toBeInTheDocument();
    expect(screen.getByText('+79002222222')).toBeInTheDocument();
    expect(toastSuccess).toHaveBeenCalledWith('Отправлено 2 сообщений');
  });

  it('marks per-phone failures with failed status and surfaces mixed-result toast', async () => {
    listApprovedMock.mockResolvedValue({
      sender_names: [{ id: '1', name: 'BRAND', status: 'approved' }],
      total: 1, page: 1, per_page: 100, total_pages: 1,
    });
    sendMock
      .mockResolvedValueOnce({ message_id: 'm1', status: 'queued' })
      .mockRejectedValueOnce(new Error('send failed'));
    renderPage();
    await waitForSendersLoaded();
    const user = userEvent.setup();
    await user.type(screen.getByLabelText(/Текст сообщения/i), 'Hello');
    await user.click(screen.getByLabelText(/Контакты/i));
    await user.paste('+79001111111\n+79002222222');
    await user.click(screen.getByRole('button', { name: /Отправить/i }));
    const dialog = await screen.findByRole('dialog', { name: 'confirm' });
    await user.click(within(dialog).getByRole('button', { name: 'Отправить' }));

    await waitFor(() => expect(sendMock).toHaveBeenCalledTimes(2));
    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith('Отправлено 1, ошибок: 1'),
    );
  });
});
