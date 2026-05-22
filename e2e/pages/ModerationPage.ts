import { type Page, expect } from '@playwright/test';

export class ModerationPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/network/moderation');
  }

  async expectLoaded() {
    await expect(this.page.locator('text=Модерация')).toBeVisible();
    await expect(this.page.locator('text=Заявки субаккаунтов')).toBeVisible();
  }

  // --- Tabs ---
  async clickSenderNamesTab() {
    await this.page.click('button:has-text("Имена отправителей")');
  }

  async clickTemplatesTab() {
    await this.page.click('button:has-text("Шаблоны")');
  }

  async clickRegistrationsTab() {
    await this.page.click('button:has-text("Регистрации у операторов")');
  }

  // --- Content ---
  async expectEmptyState() {
    await expect(this.page.locator('text=Нет заявок')).toBeVisible();
  }

  async expectTable() {
    await expect(this.page.locator('table')).toBeVisible();
  }

  // --- Status filter ---
  async filterByStatus(status: string) {
    await this.page.selectOption('select', status);
  }

  async clearStatusFilter() {
    await this.page.selectOption('select', '');
  }

  // --- Actions ---
  async clickApprove(index = 0) {
    const buttons = this.page.locator('button:has-text("Одобрить")');
    await buttons.nth(index).click();
  }

  async clickReject(index = 0) {
    const buttons = this.page.locator('button:has-text("Отклонить")');
    await buttons.nth(index).click();
  }

  async clickRevision(index = 0) {
    const buttons = this.page.locator('button:has-text("Доработка")');
    await buttons.nth(index).click();
  }

  async fillRejectReason(reason: string) {
    await this.page.fill('[role="dialog"] input', reason);
  }

  async submitModal() {
    await this.page.click('[role="dialog"] button:has-text("Подтвердить")');
  }

  async cancelModal() {
    await this.page.click('[role="dialog"] button:has-text("Отмена")');
  }

  async expectApproveToast() {
    await expect(this.page.locator('text=/одобрен/i')).toBeVisible({ timeout: 10_000 });
  }

  async expectRejectToast() {
    await expect(this.page.locator('text=/отклонен/i')).toBeVisible({ timeout: 10_000 });
  }

  async expectRevisionToast() {
    await expect(this.page.locator('text=/доработк/i')).toBeVisible({ timeout: 10_000 });
  }

  getModerationBadgeCount(tabName: string) {
    return this.page.locator(`button:has-text("${tabName}") span.rounded-full`);
  }
}
