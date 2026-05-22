import { type Page, expect } from '@playwright/test';

export class PasswordResetRequestPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/reset-password-request');
    await expect(this.page).toHaveURL(/\/reset-password-request/);
  }

  async fillEmailAndSubmit(email: string) {
    await this.page.getByLabel('Электронная почта').fill(email);
    await this.page.getByRole('button', { name: /отправить ссылку|отправка/i }).click();
  }

  async expectNeutralConfirmation() {
    await expect(this.page.getByRole('heading', { name: 'Проверьте почту' })).toBeVisible();
    await expect(
      this.page.getByText(/Если аккаунт с таким email существует/i),
    ).toBeVisible();
  }
}

export class PasswordResetConfirmPage {
  constructor(private page: Page) {}

  async goto(token?: string) {
    const url = token ? `/reset-password?token=${encodeURIComponent(token)}` : '/reset-password';
    await this.page.goto(url);
    await expect(this.page).toHaveURL(/\/reset-password/);
  }

  async fillPasswords(password: string, confirm: string) {
    await this.page.getByLabel('Новый пароль').fill(password);
    await this.page.getByLabel('Подтверждение пароля').fill(confirm);
  }

  async submit() {
    await this.page.getByRole('button', { name: /сбросить пароль|сброс/i }).click();
  }

  async expectError(text: string | RegExp) {
    await expect(this.page.locator('#reset-error')).toContainText(text);
  }
}
