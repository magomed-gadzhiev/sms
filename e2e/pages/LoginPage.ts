import { type Page, expect } from '@playwright/test';

export class LoginPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/login');
    await expect(this.page).toHaveURL(/\/login/);
  }

  async fillEmail(email: string) {
    await this.page.getByLabel('Электронная почта').fill(email);
  }

  async fillPassword(password: string) {
    await this.page.getByLabel('Пароль').fill(password);
  }

  async submit() {
    await this.page.getByRole('button', { name: /войти|вход/i }).click();
  }

  async login(email: string, password: string) {
    await this.fillEmail(email);
    await this.fillPassword(password);
    await this.submit();
  }

  async submitEmpty() {
    await this.bypassNativeValidation();
    await this.submit();
  }

  async clickForgotPassword() {
    await this.page.getByRole('link', { name: 'Забыли пароль?' }).click();
  }

  async clickRegisterLink() {
    await this.page.getByRole('link', { name: 'Зарегистрироваться' }).click();
  }

  async bypassNativeValidation() {
    await this.page.evaluate(() => {
      document.querySelector('form')?.setAttribute('novalidate', 'true');
    });
  }

  async expectLoggedIn() {
    await expect(this.page).toHaveURL(/\/command-center/);
  }

  async expectOnLoginPage() {
    await expect(this.page).toHaveURL(/\/login/);
  }

  async expectError(text: string | RegExp) {
    await expect(this.page.locator('#login-error')).toContainText(text);
  }
}
