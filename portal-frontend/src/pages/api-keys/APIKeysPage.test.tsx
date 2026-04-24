import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { APIKeysPage } from './APIKeysPage';
import { ApiError } from '../../api/client';

const listMock = vi.fn();
const createMock = vi.fn();
const revokeMock = vi.fn();
const updateMock = vi.fn();

vi.mock('../../api/client', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('../../api/client');
  return {
    ...actual,
    apiKeysApi: {
      list: () => listMock(),
      create: (d: unknown) => createMock(d),
      revoke: (id: string) => revokeMock(id),
      update: (id: string, d: unknown) => updateMock(id, d),
    },
  };
});

vi.mock('../../components/layout/PageHeader', () => ({
  PageHeader: ({ title, actions }: { title: string; actions?: React.ReactNode }) => (
    <div><h1>{title}</h1>{actions}</div>
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
    columns: { key: string; header: string; render?: (row: T) => React.ReactNode }[];
    rowActions?: (row: T) => React.ReactNode;
  }) => (
    <table>
      <thead>
        <tr>{columns.map((c) => <th key={c.key}>{c.header}</th>)}{rowActions && <th>Действия</th>}</tr>
      </thead>
      <tbody>
        {data.map((row) => (
          <tr key={row.id} data-testid={`row-${row.id}`}>
            {columns.map((c) => (
              <td key={c.key}>{c.render ? c.render(row) : (row as unknown as Record<string, unknown>)[c.key] as React.ReactNode}</td>
            ))}
            {rowActions && <td>{rowActions(row)}</td>}
          </tr>
        ))}
      </tbody>
    </table>
  ),
}));
vi.mock('../../components/ui/Badge', () => ({
  StatusBadge: ({ status }: { status: string }) => <span data-status={status}>{status}</span>,
}));
vi.mock('../../components/ui/ConfirmDialog', () => ({
  ConfirmDialog: ({ open, onConfirm, onCancel }: {
    open: boolean;
    onConfirm: () => void;
    onCancel: () => void;
  }) =>
    open ? (
      <div role="alertdialog">
        <button onClick={onConfirm}>Подтвердить</button>
        <button onClick={onCancel}>Отмена</button>
      </div>
    ) : null,
}));

function renderPage() {
  return render(
    <MemoryRouter>
      <APIKeysPage />
    </MemoryRouter>,
  );
}

describe('APIKeysPage (US5 — API key management)', () => {
  beforeEach(() => {
    listMock.mockReset();
    createMock.mockReset();
    revokeMock.mockReset();
    updateMock.mockReset();
    // Install a writable clipboard stub — jsdom's navigator.clipboard is a getter.
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: vi.fn() },
      configurable: true,
    });
  });

  it('loads and renders existing API keys', async () => {
    listMock.mockResolvedValue({
      keys: [
        { id: 'k1', name: 'Production', prefix: 'pk_prod', active: true, scopes: ['messages:send'], allowed_ips: [], created_at: '2026-03-20T10:00:00Z' },
        { id: 'k2', name: 'Staging', prefix: 'pk_stg', active: false, scopes: [], allowed_ips: [], created_at: '2026-03-20T10:00:00Z' },
      ],
    });
    renderPage();
    expect(await screen.findByText('Production')).toBeInTheDocument();
    expect(screen.getByText('Staging')).toBeInTheDocument();
  });

  it('surfaces load error', async () => {
    listMock.mockRejectedValue(new ApiError(500, 'boom'));
    renderPage();
    expect(await screen.findByText('boom')).toBeInTheDocument();
  });

  it('creates a key and displays its secret exactly once (US5 AC1)', async () => {
    listMock.mockResolvedValueOnce({ keys: [] });
    createMock.mockResolvedValue({
      api_key: 'secret-api-key-abc',
      api_key_id: 'k-new',
      created_at: '2026-04-24T10:00:00Z',
    });
    listMock.mockResolvedValueOnce({
      keys: [{ id: 'k-new', name: 'My Key', prefix: 'pk_my', active: true, scopes: [], allowed_ips: [], created_at: '2026-04-24T10:00:00Z' }],
    });

    renderPage();
    await waitFor(() => expect(listMock).toHaveBeenCalled());
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: /Создать API ключ|Создать ключ/i }));

    const dialog = await screen.findByRole('dialog');
    await user.type(within(dialog).getByLabelText(/Название/i), 'My Key');
    await user.click(within(dialog).getByRole('button', { name: /Создать$|Сохранить/i }));

    await waitFor(() => expect(createMock).toHaveBeenCalled());
    // The full secret is shown once after creation.
    expect(await screen.findByText('secret-api-key-abc')).toBeInTheDocument();
  });

  it('does not call create when name is empty (US5 AC1 — client guard)', async () => {
    listMock.mockResolvedValue({ keys: [] });
    renderPage();
    await waitFor(() => expect(listMock).toHaveBeenCalled());
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: /Создать/i }));

    const dialog = await screen.findByRole('dialog');
    // Submit form without typing a name — rely on native "required" attribute;
    // HTML5 blocks submit, so createMock must NOT be called.
    await user.click(within(dialog).getByRole('button', { name: /Создать$|Сохранить/i }));
    expect(createMock).not.toHaveBeenCalled();
  });
});
