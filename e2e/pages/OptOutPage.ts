import { type Page, expect } from '@playwright/test';

export class OptOutPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/opt-out');
    await expect(
      this.page.getByRole('heading', { level: 1, name: /Список отписок/ }),
    ).toBeVisible();
  }

  async openAdd() {
    await this.page.locator('button:has-text("Добавить номер")').click();
    await expect(this.addDialog()).toBeVisible();
  }

  addDialog() {
    return this.page.locator('[role="dialog"]').filter({ hasText: 'Добавить номер в список отписок' });
  }

  async fillAdd(phone: string, keyword?: string) {
    const dialog = this.addDialog();
    await dialog.getByPlaceholder('+79001234567').fill(phone);
    if (keyword) {
      await dialog.getByPlaceholder('STOP, ОТПИСАТЬСЯ...').fill(keyword);
    }
  }

  async submitAdd() {
    const dialog = this.addDialog();
    const alert = dialog.locator('p.text-red-600');
    await dialog.locator('button:has-text("Добавить")').click();
    const hidden = dialog.waitFor({ state: 'hidden', timeout: 10_000 });
    const errored = alert.waitFor({ state: 'visible', timeout: 10_000 });
    hidden.catch(() => {});
    errored.catch(() => {});
    const winner = await Promise.race([
      hidden.then(() => 'hidden' as const),
      errored.then(() => 'errored' as const),
    ]);
    if (winner === 'errored') {
      const msg = await alert.textContent();
      throw new Error(`Add to opt-out failed: ${msg}`);
    }
  }

  row(phone: string) {
    return this.page.locator('table tbody tr').filter({ hasText: phone });
  }

  async expectRow(phone: string) {
    await expect(this.row(phone)).toBeVisible();
  }

  async expectRowHidden(phone: string) {
    await expect(this.row(phone)).toHaveCount(0);
  }

  async search(query: string) {
    await this.page.locator('input[placeholder="Поиск по номеру..."]').fill(query);
    await this.page.locator('button:has-text("Найти")').click();
  }

  async resetSearch() {
    await this.page.locator('button:has-text("Сбросить")').click();
  }

  async deleteRow(phone: string) {
    await this.row(phone).locator('button:has-text("Удалить")').click();
    const confirm = this.page.locator('[role="dialog"]').filter({ hasText: 'Удалить из списка отписок?' });
    await confirm.locator('button:has-text("Удалить")').click();
    await expect(confirm).toBeHidden();
  }

  async expectToast(text: string) {
    await expect(this.page.getByText(text).first()).toBeVisible({ timeout: 5000 });
  }

  async addViaUI(phone: string, keyword?: string) {
    await this.openAdd();
    await this.fillAdd(phone, keyword);
    await this.submitAdd();
    await this.expectRow(phone);
  }
}
