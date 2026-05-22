import { type Page, expect } from '@playwright/test';

export interface RegisterFormData {
  companyName: string;
  email: string;
  password: string;
  contactPerson?: string;
  phone?: string;
  plan?: 'free' | 'starter' | 'business' | 'pro';
}

export class RegisterPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/register');
    await expect(this.page).toHaveURL(/\/register/);
  }

  async fillForm(data: RegisterFormData) {
    await this.page.getByLabel('Название компании *').fill(data.companyName);
    await this.page.getByLabel('Email *').fill(data.email);
    await this.page.getByLabel('Пароль *').fill(data.password);
    if (data.contactPerson !== undefined) {
      await this.page.getByLabel('Контактное лицо').fill(data.contactPerson);
    }
    if (data.phone !== undefined) {
      await this.page.getByLabel('Телефон').fill(data.phone);
    }
    if (data.plan) {
      await this.page.locator('input[name="plan"][value="' + data.plan + '"]').check({ force: true });
    }
  }

  async submit() {
    await this.page.getByRole('button', { name: /зарегистрироваться|регистрация/i }).click();
  }

  async bypassNativeValidation() {
    await this.page.evaluate(() => {
      document.querySelector('form')?.setAttribute('novalidate', 'true');
    });
  }

  async expectFormError(text: string | RegExp) {
    await expect(this.page.locator('#register-error')).toContainText(text);
  }

  async expectInlineError(text: string) {
    const errors = this.page.locator('span[role="alert"]', { hasText: text });
    await expect(errors.first()).toBeVisible();
  }

  async expectOnRegisterPage() {
    await expect(this.page).toHaveURL(/\/register/);
  }
}
