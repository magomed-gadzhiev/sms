import { type Page, expect } from '@playwright/test';

export class ContactListsPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/contact-lists', { waitUntil: 'domcontentloaded' });
    await expect(
      this.page.getByRole('heading', { level: 1, name: 'Контактные базы' }),
    ).toBeVisible();
  }

  async expectEmptyState() {
    await expect(this.page.getByText('Контактные базы не созданы')).toBeVisible();
    await expect(
      this.page.getByText('Создайте первую базу контактов для отправки рассылок'),
    ).toBeVisible();
  }

  async openCreateModal() {
    await this.page.locator('button:has-text("+ Создать базу")').first().click();
    await expect(
      this.createDialog().getByRole('heading', { name: 'Создать контактную базу' }),
    ).toBeVisible();
  }

  createDialog() {
    return this.page.locator('[role="dialog"]').filter({ hasText: 'Создать контактную базу' });
  }

  async fillCreateForm(name: string, description?: string) {
    const dialog = this.createDialog();
    await dialog.locator('input').first().fill(name);
    if (description) {
      await dialog.locator('textarea#create-desc').fill(description);
    }
  }

  createSubmitButton() {
    return this.createDialog().locator('button:has-text("Создать")');
  }

  async submitCreate() {
    const dialog = this.createDialog();
    const alert = dialog.locator('[role="alert"]');
    await this.createSubmitButton().click();
    const hidden = dialog.waitFor({ state: 'hidden', timeout: 10_000 });
    const errored = alert.waitFor({ state: 'visible', timeout: 10_000 });
    // Проигравший promise надо съесть заранее — иначе unhandledRejection при двойном таймауте
    hidden.catch(() => {});
    errored.catch(() => {});
    const winner = await Promise.race([
      hidden.then(() => 'hidden' as const),
      errored.then(() => 'errored' as const),
    ]);
    if (winner === 'errored') {
      const msg = await alert.textContent();
      throw new Error(`Create list failed: ${msg}`);
    }
  }

  row(name: string) {
    return this.page.locator('table tbody tr').filter({ hasText: name });
  }

  async expectRowVisible(name: string) {
    await expect(this.row(name)).toBeVisible();
  }

  async expectRowHidden(name: string) {
    await expect(this.row(name)).toHaveCount(0);
  }

  async openRow(name: string) {
    await this.row(name).locator('button:has-text("Открыть")').click();
  }

  async clickDelete(name: string) {
    await this.row(name).locator('button:has-text("Удалить")').click();
  }

  confirmDialog() {
    return this.page.locator('[role="dialog"]').filter({ hasText: 'Удалить контактную базу' });
  }

  async confirmDelete() {
    await this.confirmDialog().locator('button:has-text("Удалить")').click();
    await expect(this.confirmDialog()).toBeHidden();
  }

  async expectToast(text: string) {
    await expect(this.page.getByText(text).first()).toBeVisible({ timeout: 5000 });
  }

  async createViaUI(name: string, description?: string) {
    await this.openCreateModal();
    await this.fillCreateForm(name, description);
    await this.submitCreate();
    await this.expectRowVisible(name);
  }

  async deleteViaUI(name: string) {
    await this.clickDelete(name);
    await this.confirmDelete();
    await this.expectRowHidden(name);
  }
}
