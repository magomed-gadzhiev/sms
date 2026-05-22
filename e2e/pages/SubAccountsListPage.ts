import { type Page, expect } from '@playwright/test';

export class SubAccountsListPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/network/sub-accounts');
  }

  async expectLoaded() {
    await expect(this.page.locator('text=Суб-аккаунты')).toBeVisible();
  }

  async expectCountIndicator() {
    // Shows "current / max" indicator
    await expect(this.page.locator('text=/\\d+ \\/ \\d+/')).toBeVisible();
  }

  async expectEmptyState() {
    await expect(this.page.locator('text=Суб-аккаунтов пока нет')).toBeVisible();
  }

  async expectTable() {
    await expect(this.page.locator('table')).toBeVisible();
  }

  async expectSubAccountInTable(name: string) {
    await expect(this.page.locator(`table a:has-text("${name}")`)).toBeVisible();
  }

  async expectFeatureUnavailable() {
    await expect(this.page.locator('text=Суб-аккаунты недоступны')).toBeVisible();
  }

  async clickCreate() {
    await this.page.click('button:has-text("Создать суб-аккаунт")');
    await expect(this.page.locator('text=Создать суб-аккаунт').last()).toBeVisible();
  }

  async fillCreateForm(data: {
    name: string;
    email: string;
    contactPerson?: string;
    initialBalance?: string;
    dailyLimit?: string;
    monthlyLimit?: string;
  }) {
    await this.page.fill('input[required]:first-of-type', data.name);

    const emailInput = this.page.locator('input[type="email"]');
    await emailInput.fill(data.email);

    if (data.contactPerson) {
      await this.page.fill('input[placeholder="Имя контактного лица"]', data.contactPerson);
    }
    if (data.initialBalance) {
      await this.page.fill('input[placeholder="0.00"]', data.initialBalance);
    }
    if (data.dailyLimit) {
      await this.page.fill('input[placeholder="напр. 1000"]', data.dailyLimit);
    }
    if (data.monthlyLimit) {
      await this.page.fill('input[placeholder="напр. 30000"]', data.monthlyLimit);
    }
  }

  async submitCreateForm() {
    await this.page.click('[role="dialog"] button[type="submit"]:has-text("Создать")');
  }

  async cancelCreateForm() {
    await this.page.click('[role="dialog"] button:has-text("Отмена")');
  }

  async expectCreateFormEmailError() {
    await expect(this.page.locator('#email-error')).toBeVisible();
  }

  async expectSuccessToast() {
    await expect(this.page.locator('text=Суб-аккаунт успешно создан')).toBeVisible({ timeout: 10_000 });
  }

  async clickSubAccount(name: string) {
    await this.page.click(`table a:has-text("${name}")`);
  }

  getTableRows() {
    return this.page.locator('table tbody tr');
  }
}
