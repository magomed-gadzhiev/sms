import { type Page, expect } from '@playwright/test';

export class MessagesPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/messages');
    await expect(this.page.locator('h1, [class*="PageHeader"]')).toContainText('Сообщения');
  }

  async waitForData() {
    // Wait for data to load or error state
    // The detalization API can be slow (up to 30s) or return 504
    const dataLoaded = this.page.locator('text=Найдено:');
    const errorAlert = this.page.locator('div[role="alert"]');
    await expect(dataLoaded.or(errorAlert)).toBeVisible({ timeout: 45_000 });
  }

  async isDataLoaded(): Promise<boolean> {
    return this.page.locator('text=Найдено:').isVisible();
  }

  async getTotalCount(): Promise<number> {
    const el = this.page.locator('[aria-live="polite"] strong');
    const isVisible = await el.isVisible().catch(() => false);
    if (!isVisible) return 0;
    const text = await el.textContent();
    return parseInt(text || '0', 10);
  }

  async filterByStatus(status: string) {
    // MessageFilters uses native <select> rendered by renderField
    // Find select element that has the matching option
    const selects = this.page.locator('select');
    const count = await selects.count();
    for (let i = 0; i < count; i++) {
      const label = await selects.nth(i).locator('..').locator('label').textContent().catch(() => '');
      if (label?.includes('Статус')) {
        await selects.nth(i).selectOption(status);
        break;
      }
    }
    // Click "Найти" button in MessageFilters
    await this.page.click('button:has-text("Найти")');
    await this.page.waitForTimeout(1000);
  }

  async filterByDestination(phone: string) {
    // "Номер" field with placeholder "+7..."
    const input = this.page.locator('input[placeholder="+7..."]');
    await input.fill(phone);
    await this.page.click('button:has-text("Найти")');
    await this.page.waitForTimeout(1000);
  }

  async clickFirstMessage() {
    await this.page.locator('tbody tr').first().click();
  }

  async expectMessageModal() {
    await expect(this.page.locator('[role="dialog"]')).toBeVisible({ timeout: 5000 });
  }

  async expectMessageDetailField(field: string) {
    const modal = this.page.locator('[role="dialog"]');
    await expect(modal.locator(`text=${field}`)).toBeVisible();
  }

  async closeModal() {
    const dialog = this.page.locator('[role="dialog"]');
    // Try close button or X button
    const closeBtn = dialog.locator('button:has-text("Закрыть"), button[aria-label="Close"], button[aria-label="Закрыть"]');
    if (await closeBtn.count() > 0) {
      await closeBtn.first().click();
    } else {
      await this.page.keyboard.press('Escape');
    }
  }

  async clickExport() {
    await this.page.click('button:has-text("Экспорт")');
  }

  async expectExportInProgress() {
    await expect(this.page.locator('text=Экспорт...')).toBeVisible();
  }
}
