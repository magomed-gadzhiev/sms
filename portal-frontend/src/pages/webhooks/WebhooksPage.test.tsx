import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { WebhooksPage } from './WebhooksPage';
import { ApiError } from '../../api/client';

const listMock = vi.fn();
const createMock = vi.fn();
const updateMock = vi.fn();
const removeMock = vi.fn();
const testMock = vi.fn();

vi.mock('../../api/client', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('../../api/client');
  return {
    ...actual,
    webhooksApi: {
      list: () => listMock(),
      create: (d: unknown) => createMock(d),
      update: (id: string, d: unknown) => updateMock(id, d),
      remove: (id: string) => removeMock(id),
      test: (id: string) => testMock(id),
    },
  };
});

// Stubs for UI primitives — keep tests focused on page logic.
vi.mock('../../components/layout/PageHeader', () => ({
  PageHeader: ({ title, actions }: { title: string; actions?: React.ReactNode }) => (
    <div>
      <h1>{title}</h1>
      {actions}
    </div>
  ),
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
vi.mock('../../components/ui/Modal', () => ({
  Modal: ({ open, onClose, title, children }: {
    open: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
  }) =>
    open ? (
      <div role="dialog" aria-label={title}>
        <h2>{title}</h2>
        {children}
        <button onClick={onClose}>Закрыть</button>
      </div>
    ) : null,
}));
vi.mock('../../components/ui/Input', () => ({
  Input: ({ label, value, onChange, ...rest }: {
    label?: string;
    value?: string;
    onChange?: (e: React.ChangeEvent<HTMLInputElement>) => void;
  } & React.InputHTMLAttributes<HTMLInputElement>) => (
    <label>
      {label}
      <input value={value ?? ''} onChange={onChange} {...rest} />
    </label>
  ),
}));
vi.mock('../../components/data/DataTable', () => ({
  DataTable: <T extends { id: string }>({ data, columns, rowActions }: {
    data: T[];
    columns: { key: string; header: string; render: (row: T) => React.ReactNode }[];
    rowActions?: (row: T) => React.ReactNode;
  }) => (
    <table>
      <thead>
        <tr>{columns.map((c) => <th key={c.key}>{c.header}</th>)}{rowActions && <th>Действия</th>}</tr>
      </thead>
      <tbody>
        {data.map((row) => (
          <tr key={row.id} data-testid={`row-${row.id}`}>
            {columns.map((c) => <td key={c.key}>{c.render(row)}</td>)}
            {rowActions && <td>{rowActions(row)}</td>}
          </tr>
        ))}
      </tbody>
    </table>
  ),
}));
vi.mock('../../components/ui/Badge', () => ({
  Badge: ({ children }: { children: React.ReactNode }) => <span>{children}</span>,
  StatusBadge: ({ status }: { status: string }) => <span data-status={status}>{status}</span>,
}));
vi.mock('../../components/ui/ConfirmDialog', () => ({
  ConfirmDialog: ({ open, onConfirm, onCancel, description }: {
    open: boolean;
    onConfirm: () => void;
    onCancel: () => void;
    description: string;
  }) =>
    open ? (
      <div role="alertdialog" aria-label="confirm-delete">
        <p>{description}</p>
        <button onClick={onConfirm}>Да, удалить</button>
        <button onClick={onCancel}>Отмена</button>
      </div>
    ) : null,
}));

function renderPage() {
  return render(
    <MemoryRouter>
      <WebhooksPage />
    </MemoryRouter>,
  );
}

describe('WebhooksPage (US6 — Webhook subscriptions)', () => {
  beforeEach(() => {
    listMock.mockReset();
    createMock.mockReset();
    updateMock.mockReset();
    removeMock.mockReset();
    testMock.mockReset();
  });

  it('shows loading status while fetching webhooks', async () => {
    let resolveList: (v: { subscriptions: [] }) => void = () => {};
    listMock.mockImplementation(() => new Promise((resolve) => { resolveList = resolve; }));
    renderPage();
    expect(screen.getByRole('status')).toHaveTextContent(/Загрузка/i);
    resolveList({ subscriptions: [] });
    await waitFor(() => expect(listMock).toHaveBeenCalled());
  });

  it('renders the list of existing webhooks (US6 AC3: visible subscriptions)', async () => {
    listMock.mockResolvedValue({
      subscriptions: [
        { id: 'wh-1', client_id: 'c', url: 'https://a.example/hook', event_types: ['delivered'], active: true },
        { id: 'wh-2', client_id: 'c', url: 'https://b.example/hook', event_types: ['delivered', 'failed'], active: false },
      ],
    });
    renderPage();
    expect(await screen.findByText('https://a.example/hook')).toBeInTheDocument();
    expect(screen.getByText('https://b.example/hook')).toBeInTheDocument();
  });

  it('shows an error when loading fails', async () => {
    listMock.mockRejectedValue(new ApiError(500, 'Server exploded'));
    renderPage();
    expect(await screen.findByText('Server exploded')).toBeInTheDocument();
  });

  it('creates a new webhook and surfaces the returned secret once (US6 AC1)', async () => {
    listMock.mockResolvedValueOnce({ subscriptions: [] });
    createMock.mockResolvedValue({
      id: 'wh-new',
      client_id: 'c',
      url: 'https://new.example/hook',
      event_types: ['delivered'],
      active: true,
      secret: 'top-secret-1234',
    });
    listMock.mockResolvedValueOnce({
      subscriptions: [
        { id: 'wh-new', client_id: 'c', url: 'https://new.example/hook', event_types: ['delivered'], active: true },
      ],
    });

    renderPage();
    await waitFor(() => expect(listMock).toHaveBeenCalledTimes(1));
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: /Создать вебхук/i }));

    const dialog = await screen.findByRole('dialog');
    await user.type(within(dialog).getByLabelText(/URL/i), 'https://new.example/hook');
    await user.click(within(dialog).getByLabelText(/delivered/i));
    await user.click(within(dialog).getByRole('button', { name: /Создать$|Сохранить/i }));

    await waitFor(() => expect(createMock).toHaveBeenCalledWith({
      url: 'https://new.example/hook',
      event_types: ['delivered'],
    }));

    // Secret banner is rendered once — exposed as an alert with the value.
    const banner = await screen.findByRole('alert');
    expect(banner).toHaveTextContent('top-secret-1234');
  });

  it('opens the confirm dialog when delete is clicked, and calls remove on confirm', async () => {
    listMock.mockResolvedValue({
      subscriptions: [{ id: 'wh-1', client_id: 'c', url: 'https://a.example/hook', event_types: ['delivered'], active: true }],
    });
    removeMock.mockResolvedValue(undefined);
    listMock.mockResolvedValueOnce({
      subscriptions: [{ id: 'wh-1', client_id: 'c', url: 'https://a.example/hook', event_types: ['delivered'], active: true }],
    });

    renderPage();
    await screen.findByText('https://a.example/hook');

    const user = userEvent.setup();
    const row = screen.getByTestId('row-wh-1');
    await user.click(within(row).getByRole('button', { name: /Удалить/i }));

    const dialog = await screen.findByRole('alertdialog');
    await user.click(within(dialog).getByRole('button', { name: /Да, удалить/i }));

    await waitFor(() => expect(removeMock).toHaveBeenCalledWith('wh-1'));
  });

  it('triggers test webhook and shows the success message (US6 AC3)', async () => {
    listMock.mockResolvedValue({
      subscriptions: [{ id: 'wh-1', client_id: 'c', url: 'https://a.example/hook', event_types: ['delivered'], active: true }],
    });
    testMock.mockResolvedValue({ message: 'Тестовое событие отправлено' });

    renderPage();
    await screen.findByText('https://a.example/hook');
    const user = userEvent.setup();
    const row = screen.getByTestId('row-wh-1');
    await user.click(within(row).getByRole('button', { name: /Тест/i }));

    await waitFor(() => expect(testMock).toHaveBeenCalledWith('wh-1'));
    expect(await screen.findByText(/Тестовое событие отправлено/i)).toBeInTheDocument();
  });

  it('shows an error when webhook creation fails', async () => {
    listMock.mockResolvedValue({ subscriptions: [] });
    createMock.mockRejectedValue(new ApiError(400, 'URL must be https'));

    renderPage();
    await waitFor(() => expect(listMock).toHaveBeenCalled());
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: /Создать вебхук/i }));

    const dialog = await screen.findByRole('dialog');
    await user.type(within(dialog).getByLabelText(/URL/i), 'http://insecure');
    await user.click(within(dialog).getByLabelText(/delivered/i));
    await user.click(within(dialog).getByRole('button', { name: /Создать$|Сохранить/i }));

    expect(await screen.findByText('URL must be https')).toBeInTheDocument();
  });
});
