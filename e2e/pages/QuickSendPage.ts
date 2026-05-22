import { type Page, expect } from '@playwright/test';

export class QuickSendPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/quick-send');
    await expect(this.page.locator('h1, [class*="PageHeader"]')).toContainText('Быстрая отправка');
    // Wait for sender names to load (auto-selects first approved)
    await this.page.waitForTimeout(1000);
  }

  async fillText(text: string) {
    await this.page.fill('#qs-text', text);
  }

  async fillContacts(phones: string[]) {
    await this.page.fill('#qs-contacts', phones.join('\n'));
  }

  async selectSenderName(name: string) {
    // SearchableSelect: click button to open, then select option
    const trigger = this.page.locator('button[aria-haspopup="listbox"]');
    await trigger.click();
    await this.page.locator(`button[role="option"]:has-text("${name}")`).click();
  }

  async clickSend() {
    // Scope to main content to avoid clicking sidebar "Отправить" nav button
    await this.page.locator('main button:has-text("Отправить")').click();
  }

  async confirmSend() {
    // ConfirmDialog renders inside a Modal (Radix) with role="dialog"
    const dialog = this.page.locator('[role="dialog"]');
    await expect(dialog.locator('h2:has-text("Подтвердите отправку")')).toBeVisible({ timeout: 5000 });
    await dialog.locator('button:has-text("Отправить")').click();
  }

  async expectResultsTable() {
    await expect(this.page.locator('text=Результаты отправки')).toBeVisible({ timeout: 15_000 });
  }

  async expectResultStatus(phone: string, status: string) {
    const row = this.page.locator(`tr:has-text("${phone}")`);
    await expect(row).toBeVisible();
    await expect(row.locator(`text=${status}`)).toBeVisible({ timeout: 30_000 });
  }

  async getResultsCount(): Promise<number> {
    // Wait a moment for all results to stream in
    await this.page.waitForTimeout(2000);
    const rows = this.page.locator('table tbody tr');
    return rows.count();
  }

  async expectResultsCount(expected: number) {
    await expect(this.page.locator('table tbody tr')).toHaveCount(expected, { timeout: 15_000 });
  }

  async expectFormError(text: string) {
    await expect(this.page.locator('p[role="alert"]')).toContainText(text, { timeout: 5000 });
  }

  async expectLiveIndicator() {
    await expect(this.page.locator('text=Live')).toBeVisible();
  }

  async waitForAllTerminal() {
    // Wait until Live indicator disappears (all messages reached terminal status)
    await expect(this.page.locator('text=Live')).toBeHidden({ timeout: 120_000 });
  }
}
