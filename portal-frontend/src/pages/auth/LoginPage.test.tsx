import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { LoginPage } from './LoginPage';
import { ApiError } from '../../api/client';

// Mock useAuth so we can control login/login2fa without wiring the real context.
const loginMock = vi.fn();
const login2faMock = vi.fn();
let isAuthenticatedStub = false;

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({
    login: loginMock,
    login2fa: login2faMock,
    isAuthenticated: isAuthenticatedStub,
  }),
}));

const navigateMock = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => navigateMock };
});

function renderLogin() {
  return render(
    <MemoryRouter>
      <LoginPage />
    </MemoryRouter>,
  );
}

describe('LoginPage (US5 — authentication)', () => {
  beforeEach(() => {
    loginMock.mockReset();
    login2faMock.mockReset();
    navigateMock.mockReset();
    isAuthenticatedStub = false;
  });

  it('renders email and password inputs + submit button', () => {
    renderLogin();
    expect(screen.getByLabelText(/Электронная почта/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/Пароль/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Войти/i })).toBeInTheDocument();
  });

  it('blocks submit with inline error when email or password is empty', async () => {
    renderLogin();
    // Submit the form directly — bypasses HTML5 "required" browser validation
    // which would otherwise prevent our own handler from running.
    const form = screen.getByRole('button', { name: /Войти/i }).closest('form')!;
    fireEvent.submit(form);
    expect(await screen.findByRole('alert')).toHaveTextContent('Заполните все поля');
    expect(loginMock).not.toHaveBeenCalled();
  });

  it('navigates to /dashboard on successful client login (US5 AC1)', async () => {
    loginMock.mockResolvedValue({ requires2fa: false, role: 'client' });
    const user = userEvent.setup();
    renderLogin();
    await user.type(screen.getByLabelText(/Электронная почта/i), 'user@example.com');
    await user.type(screen.getByLabelText(/Пароль/i), 'secret-pw');
    await user.click(screen.getByRole('button', { name: /Войти/i }));

    await waitFor(() => expect(loginMock).toHaveBeenCalledWith('user@example.com', 'secret-pw'));
    await waitFor(() => expect(navigateMock).toHaveBeenCalledWith('/dashboard', { replace: true }));
  });

  it('navigates to /admin when role is admin', async () => {
    loginMock.mockResolvedValue({ requires2fa: false, role: 'admin' });
    const user = userEvent.setup();
    renderLogin();
    await user.type(screen.getByLabelText(/Электронная почта/i), 'admin@example.com');
    await user.type(screen.getByLabelText(/Пароль/i), 'secret-pw');
    await user.click(screen.getByRole('button', { name: /Войти/i }));
    await waitFor(() => expect(navigateMock).toHaveBeenCalledWith('/admin', { replace: true }));
  });

  it('shows the 2FA form when server requests a TOTP code', async () => {
    loginMock.mockResolvedValue({ requires2fa: true, loginTicket: 'ticket-123' });
    const user = userEvent.setup();
    renderLogin();
    await user.type(screen.getByLabelText(/Электронная почта/i), 'user@example.com');
    await user.type(screen.getByLabelText(/Пароль/i), 'secret-pw');
    await user.click(screen.getByRole('button', { name: /Войти/i }));

    expect(await screen.findByText(/Двухфакторная аутентификация/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/Код TOTP/i)).toBeInTheDocument();
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it('2FA: successful verification navigates to the role-appropriate destination', async () => {
    loginMock.mockResolvedValue({ requires2fa: true, loginTicket: 'ticket-123' });
    login2faMock.mockResolvedValue('client');
    const user = userEvent.setup();
    renderLogin();
    await user.type(screen.getByLabelText(/Электронная почта/i), 'u@e.com');
    await user.type(screen.getByLabelText(/Пароль/i), 'pw');
    await user.click(screen.getByRole('button', { name: /Войти/i }));

    const code = await screen.findByLabelText(/Код TOTP/i);
    await user.type(code, '123456');
    await user.click(screen.getByRole('button', { name: /Подтвердить/i }));

    await waitFor(() => expect(login2faMock).toHaveBeenCalledWith('ticket-123', '123456'));
    await waitFor(() => expect(navigateMock).toHaveBeenCalledWith('/dashboard', { replace: true }));
  });

  it('surfaces a localized error when credentials are invalid (US5 AC3)', async () => {
    loginMock.mockRejectedValue(new ApiError(401, 'Invalid credentials'));
    const user = userEvent.setup();
    renderLogin();
    await user.type(screen.getByLabelText(/Электронная почта/i), 'u@e.com');
    await user.type(screen.getByLabelText(/Пароль/i), 'bad');
    await user.click(screen.getByRole('button', { name: /Войти/i }));

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Неверный email или пароль');
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it('surfaces unknown ApiError message as-is (passes server text)', async () => {
    loginMock.mockRejectedValue(new ApiError(500, 'Service unavailable'));
    const user = userEvent.setup();
    renderLogin();
    await user.type(screen.getByLabelText(/Электронная почта/i), 'u@e.com');
    await user.type(screen.getByLabelText(/Пароль/i), 'pw');
    await user.click(screen.getByRole('button', { name: /Войти/i }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Service unavailable');
  });

  it('surfaces generic "Ошибка входа" for non-ApiError exceptions', async () => {
    loginMock.mockRejectedValue(new Error('network hiccup'));
    const user = userEvent.setup();
    renderLogin();
    await user.type(screen.getByLabelText(/Электронная почта/i), 'u@e.com');
    await user.type(screen.getByLabelText(/Пароль/i), 'pw');
    await user.click(screen.getByRole('button', { name: /Войти/i }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Ошибка входа');
  });

  it('disables the submit button while login is in flight', async () => {
    let resolveLogin: (v: { requires2fa: false; role: 'client' }) => void = () => {};
    loginMock.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveLogin = resolve;
        }),
    );
    const user = userEvent.setup();
    renderLogin();
    await user.type(screen.getByLabelText(/Электронная почта/i), 'u@e.com');
    await user.type(screen.getByLabelText(/Пароль/i), 'pw');
    await user.click(screen.getByRole('button', { name: /Войти/i }));

    const btn = screen.getByRole('button', { name: /Вход\.\.\./i });
    expect(btn).toBeDisabled();
    expect(screen.getByRole('status')).toHaveTextContent(/Выполняется вход/i);

    resolveLogin({ requires2fa: false, role: 'client' });
    await waitFor(() => expect(navigateMock).toHaveBeenCalled());
  });
});
