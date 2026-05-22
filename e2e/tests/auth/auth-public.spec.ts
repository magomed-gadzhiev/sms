import { test, expect } from '@playwright/test';
import { LoginPage } from '../../pages/LoginPage';
import { RegisterPage } from '../../pages/RegisterPage';
import {
  PasswordResetRequestPage,
  PasswordResetConfirmPage,
} from '../../pages/PasswordResetPage';

// Публичный auth-flow: Login / Register / Password reset — без storageState.
// Явно сбрасываем auth, чтобы не подтягивался auth-state.json по умолчанию.
test.use({ storageState: { cookies: [], origins: [] } });

const ADMIN_EMAIL = process.env.TEST_ADMIN_EMAIL || 'admin@example.test';

test.describe('Auth · Login (public)', () => {
  test('AC-1 пустая форма логина → клиентская валидация «Заполните все поля»', async ({ page }) => {
    const login = new LoginPage(page);
    await login.goto();

    let loginRequestFired = false;
    page.on('request', (req) => {
      if (req.url().includes('/auth/login') && req.method() === 'POST') {
        loginRequestFired = true;
      }
    });

    await login.submitEmpty();

    await login.expectError('Заполните все поля');
    await login.expectOnLoginPage();
    expect(loginRequestFired).toBe(false);
  });

  test('AC-2 неверные креды → ошибка «Неверный email или пароль»', async ({ page }) => {
    const login = new LoginPage(page);
    await login.goto();
    await login.login(`noone-${Date.now()}@example.com`, 'wrongpass123');

    await login.expectError(/Неверный email или пароль/);
    await login.expectOnLoginPage();
    // Exact 'Войти' (не regex) — отсекает переходное состояние 'Вход...'.
    await expect(page.getByRole('button', { name: 'Войти' })).toBeEnabled();
  });

  test('AC-3 навигация «Забыли пароль?» и «Зарегистрироваться»', async ({ page }) => {
    const login = new LoginPage(page);
    await login.goto();

    await login.clickForgotPassword();
    await expect(page).toHaveURL(/\/reset-password-request/);

    await page.goBack();
    await expect(page).toHaveURL(/\/login/);

    await login.clickRegisterLink();
    await expect(page).toHaveURL(/\/register/);
  });
});

test.describe('Auth · Register (public)', () => {
  test('AC-4 регистрация свежего email → автологин → /command-center', async ({ page }) => {
    const register = new RegisterPage(page);
    const email = `e2e-reg-${Date.now()}@example.com`;

    await register.goto();
    await register.fillForm({
      companyName: `E2E Company ${Date.now()}`,
      email,
      password: 'Testpass123!',
      plan: 'free',
    });

    const [resp] = await Promise.all([
      page.waitForResponse((r) => r.url().includes('/auth/register') && r.request().method() === 'POST'),
      register.submit(),
    ]);
    expect(resp.status()).toBe(201);

    await page.waitForURL(/\/command-center/, { timeout: 15_000 });

    const cookies = await page.context().cookies();
    expect(cookies.find((c) => c.name === 'portal_session')?.value).toBeTruthy();
  });

  test('AC-5 регистрация на существующий email → «уже зарегистрирован»', async ({ page }) => {
    const register = new RegisterPage(page);
    await register.goto();
    await register.fillForm({
      companyName: 'Duplicate Co',
      email: ADMIN_EMAIL,
      password: 'Testpass123!',
      plan: 'free',
    });
    await register.submit();

    await register.expectFormError(/уже зарегистрирован/i);
    await register.expectOnRegisterPage();
  });

  test('AC-6 пароль короче 8 символов → inline «Минимум 8 символов»', async ({ page }) => {
    const register = new RegisterPage(page);
    await register.goto();

    let registerFired = false;
    page.on('request', (req) => {
      if (req.url().includes('/auth/register') && req.method() === 'POST') {
        registerFired = true;
      }
    });

    await register.fillForm({
      companyName: 'Short PW Co',
      email: `e2e-short-${Date.now()}@example.com`,
      password: 'pwd12',
      plan: 'free',
    });
    await register.bypassNativeValidation();
    await register.submit();

    await register.expectFormError(/Заполните все обязательные поля/i);
    await register.expectInlineError('Минимум 8 символов');
    await register.expectOnRegisterPage();
    expect(registerFired).toBe(false);
  });
});

test.describe('Auth · Password reset (public)', () => {
  test('AC-7 запрос сброса → нейтральный ответ «Проверьте почту»', async ({ page }) => {
    const reset = new PasswordResetRequestPage(page);
    await reset.goto();

    const respPromise = page.waitForResponse(
      (r) => r.url().includes('/auth/password/reset-request') && r.request().method() === 'POST',
    );
    await reset.fillEmailAndSubmit(`nobody-${Date.now()}@example.com`);
    const resp = await respPromise;
    expect(resp.status()).toBe(202);

    await reset.expectNeutralConfirmation();
  });

  test('AC-8 /reset-password без токена → ошибка «Отсутствует токен сброса»', async ({ page }) => {
    const reset = new PasswordResetConfirmPage(page);
    await reset.goto();

    let resetFired = false;
    page.on('request', (req) => {
      if (req.url().includes('/auth/password/reset') && req.method() === 'POST') {
        resetFired = true;
      }
    });

    await reset.fillPasswords('newpass12', 'newpass12');
    await reset.submit();

    await reset.expectError(/Отсутствует токен сброса/i);
    expect(resetFired).toBe(false);
  });

  test('AC-9 несовпадение паролей → ошибка «Пароли не совпадают»', async ({ page }) => {
    const reset = new PasswordResetConfirmPage(page);
    await reset.goto('fake-token');

    let resetFired = false;
    page.on('request', (req) => {
      if (req.url().includes('/auth/password/reset') && req.method() === 'POST') {
        resetFired = true;
      }
    });

    await reset.fillPasswords('password1', 'password2');
    await reset.submit();

    await reset.expectError(/Пароли не совпадают/i);
    expect(resetFired).toBe(false);
  });
});
