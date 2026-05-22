import { type Page, expect } from '@playwright/test';

export class ProfilePage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/profile');
    await expect(this.page.getByRole('heading', { name: 'Профиль' })).toBeVisible();
  }

  async getEmail(): Promise<string> {
    const text = await this.page.locator('p', { hasText: 'Email:' }).first().innerText();
    return text.replace('Email:', '').trim();
  }

  async getCompany(): Promise<string> {
    const text = await this.page.locator('p', { hasText: 'Компания:' }).first().innerText();
    return text.replace('Компания:', '').trim();
  }

  async getContactPerson(): Promise<string> {
    return (await this.page.getByLabel('Контактное лицо').inputValue()) ?? '';
  }

  async getPhone(): Promise<string> {
    return (await this.page.getByLabel('Телефон').inputValue()) ?? '';
  }

  async setContactPerson(value: string) {
    const input = this.page.getByLabel('Контактное лицо');
    await input.fill(value);
  }

  async setPhone(value: string) {
    const input = this.page.getByLabel('Телефон');
    await input.fill(value);
  }

  async saveProfile() {
    await this.page.getByRole('button', { name: /сохранить|сохранение/i }).first().click();
  }

  async expectProfileSaved() {
    await expect(
      this.page.getByRole('status').filter({ hasText: 'Профиль обновлён' }),
    ).toBeVisible();
  }

  async expect2faSectionVisible() {
    await expect(
      this.page.getByRole('heading', { name: 'Двухфакторная аутентификация' }),
    ).toBeVisible();
  }

  // ── Password change ──

  async fillPasswordChange(current: string, newPw: string, confirm: string) {
    await this.page.getByLabel('Текущий пароль').fill(current);
    await this.page.getByLabel('Новый пароль').fill(newPw);
    await this.page.getByLabel('Подтверждение пароля').fill(confirm);
  }

  async submitPasswordChange() {
    await this.page.getByRole('button', { name: /сменить пароль|сохранение/i }).click();
  }

  async expectPasswordError(text: string | RegExp) {
    await expect(this.page.locator('#pw-error')).toContainText(text);
  }
}
