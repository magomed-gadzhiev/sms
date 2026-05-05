import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';

const listMock = vi.fn();
const resetMock = vi.fn();
const toastSuccessMock = vi.fn();
const toastErrorMock = vi.fn();

vi.mock('../../api/admin', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('../../api/admin');
  return {
    ...actual,
    sraStuckApi: {
      list: () => listMock(),
      reset: (clientId: string) => resetMock(clientId),
    },
  };
});

vi.mock('../../components/ui/Toast', () => ({
  useToast: () => ({
    success: toastSuccessMock,
    error: toastErrorMock,
  }),
  ToastProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

const stuckRow = {
  client_id: 'aaa-111',
  client_email: 'client@example.com',
  provider_set_id: 'pset-1',
  route_set_id: 'rset-1',
  materialize_retry_count: 120,
  last_materialize_error_at: '2026-05-01T10:00:00Z',
  last_materialize_error_text: 'connection refused',
};

function renderPage() {
  return render(
    <MemoryRouter>
      <SRAStuckPage />
    </MemoryRouter>,
  );
}

// Deferred import so mock is installed first.
let SRAStuckPage: React.ComponentType;

beforeEach(async () => {
  listMock.mockReset();
  resetMock.mockReset();
  toastSuccessMock.mockReset();
  toastErrorMock.mockReset();
  vi.spyOn(window, 'confirm').mockReturnValue(true);
  const mod = await import('./SRAStuckPage');
  SRAStuckPage = mod.SRAStuckPage;
});

describe('SRAStuckPage', () => {
  it('renders empty state when no stuck rows', async () => {
    listMock.mockResolvedValue({ rows: [] });
    renderPage();
    expect(await screen.findByText(/Нет залипших row/i)).toBeInTheDocument();
  });

  it('renders rows and Reset button when stuck rows exist', async () => {
    listMock.mockResolvedValue({ rows: [stuckRow] });
    renderPage();
    expect(await screen.findByText('client@example.com')).toBeInTheDocument();
    expect(screen.getByText('120')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Reset/i })).toBeInTheDocument();
  });

  it('calls reset + refetch + toast.success on confirm', async () => {
    listMock.mockResolvedValue({ rows: [stuckRow] });
    resetMock.mockResolvedValue({ ok: true });

    renderPage();
    await screen.findByText('client@example.com');

    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: /Reset/i }));

    await waitFor(() => expect(resetMock).toHaveBeenCalledWith('aaa-111'));
    await waitFor(() => expect(listMock).toHaveBeenCalledTimes(2));
    expect(toastSuccessMock).toHaveBeenCalled();
    expect(toastErrorMock).not.toHaveBeenCalled();
  });

  it('shows toast.error and does NOT refetch when reset fails', async () => {
    listMock.mockResolvedValue({ rows: [stuckRow] });
    resetMock.mockRejectedValue(new Error('server error'));

    renderPage();
    await screen.findByText('client@example.com');

    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: /Reset/i }));

    await waitFor(() => expect(toastErrorMock).toHaveBeenCalled());
    expect(listMock).toHaveBeenCalledTimes(1);
    expect(toastSuccessMock).not.toHaveBeenCalled();
  });

  it('renders error state when list fetch fails', async () => {
    listMock.mockRejectedValue(new Error('network down'));
    renderPage();
    expect(await screen.findByText(/network down/i)).toBeInTheDocument();
  });
});
