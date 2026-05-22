import { type Page, expect } from '@playwright/test';

export class BillingPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/billing');
    await expect(this.page.locator('h1, [class*="PageHeader"]')).toContainText('Биллинг');
  }

  async getBalance(): Promise<string> {
    const el = this.page.locator('text=Текущий баланс').locator('..').locator('p.text-3xl');
    await expect(el).toBeVisible({ timeout: 10_000 });
    return (await el.textContent()) || '0';
  }

  async getBalanceNumber(): Promise<number> {
    const text = await this.getBalance();
    // Parse "1 234,56 ₽" → 1234.56
    // Remove non-numeric chars except comma and minus, then convert
    const cleaned = text.replace(/[^\d,\-]/g, '').replace(',', '.');
    return parseFloat(cleaned) || 0;
  }

  async expectTransactionsTable() {
    await expect(this.page.locator('text=История транзакций')).toBeVisible();
  }

  async filterTransactionsByType(type: string) {
    // FilterBar uses Select component that fires onChange immediately (no submit button)
    // Find the select near "Тип" label
    const typeLabel = this.page.locator('label:has-text("Тип")');
    const select = typeLabel.locator('..').locator('select');
    await select.selectOption(type);
    // Wait for data to refresh
    await this.page.waitForTimeout(1000);
  }

  async expectChargeTransaction() {
    await expect(this.page.locator('text=Списание').first()).toBeVisible();
  }

  async getLatestTransactionAmount(): Promise<string> {
    const row = this.page.locator('tbody tr').first();
    const amountCell = row.locator('td').nth(2); // Amount column
    return (await amountCell.textContent()) || '';
  }

  async expectThresholdSection() {
    await expect(this.page.locator('text=Настройки уведомлений')).toBeVisible();
  }

  async setThreshold(value: string) {
    // Label: "Порог низкого баланса (₽)"
    const input = this.page.locator('input[type="number"][min="0"]');
    await input.fill(value);
    await this.page.click('button:has-text("Сохранить")');
  }

  async expectThresholdSaved() {
    await expect(this.page.locator('text=Порог сохранён')).toBeVisible({ timeout: 5000 });
  }
}
