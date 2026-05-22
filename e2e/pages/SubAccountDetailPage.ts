import { type Page, expect } from '@playwright/test';

export class SubAccountDetailPage {
  constructor(private page: Page) {}

  async goto(id: string) {
    await this.page.goto(`/network/sub-accounts/${id}`);
  }

  async expectLoaded() {
    // Tabs should be visible
    await expect(this.page.locator('button:has-text("Обзор")')).toBeVisible();
  }

  async expectName(name: string) {
    await expect(this.page.locator(`text=${name}`).first()).toBeVisible();
  }

  async expectStatusBadge() {
    await expect(this.page.locator('text=/active|inactive/')).toBeVisible();
  }

  // --- Tabs ---
  async clickTab(tab: 'Обзор' | 'Сообщения' | 'Кампании' | 'Транзакции' | 'Аналитика' | 'API Ключи' | 'Вебхуки') {
    await this.page.click(`button:has-text("${tab}")`);
  }

  // --- Overview Tab ---
  async expectBalanceCard() {
    await expect(this.page.locator('text=Баланс')).toBeVisible();
  }

  async expectLimitsForm() {
    await expect(this.page.locator('text=Изменить лимиты')).toBeVisible();
  }

  async expectTransferForm() {
    await expect(this.page.locator('text=Перевод средств')).toBeVisible();
  }

  async expectDeleteSection() {
    await expect(this.page.locator('text=Зона риска')).toBeVisible();
  }

  async setLimits(daily: string, monthly: string) {
    const form = this.page.locator('form:has(button:has-text("Сохранить лимиты"))');
    await form.locator('input[type="number"]').first().fill(daily);
    await form.locator('input[type="number"]').last().fill(monthly);
    await form.locator('button:has-text("Сохранить лимиты")').click();
  }

  async expectLimitsSaved() {
    await expect(this.page.locator('text=Лимиты успешно обновлены')).toBeVisible({ timeout: 10_000 });
  }

  async transferBalance(amount: string) {
    const form = this.page.locator('form:has(button:has-text("Перевести"))');
    await form.locator('input[type="number"]').fill(amount);
    await form.locator('button:has-text("Перевести")').click();
  }

  async expectTransferSuccess() {
    await expect(this.page.locator('text=/Переведено.*на баланс/')).toBeVisible({ timeout: 10_000 });
  }

  async clickDeleteButton() {
    await this.page.click('button:has-text("Удалить суб-аккаунт")');
  }

  async confirmDelete() {
    await this.page.click('button:has-text("Да, удалить")');
  }

  async cancelDelete() {
    const dialog = this.page.locator('[role="dialog"], [role="alertdialog"]');
    await dialog.locator('button:has-text("Отмена")').click();
  }

  // --- Messages Tab ---
  async expectMessagesTable() {
    await expect(this.page.locator('text=Отправитель')).toBeVisible();
  }

  async filterMessagesByStatus(status: string) {
    await this.page.selectOption('select', status);
  }

  // --- Transactions Tab ---
  async expectTransactionsContent() {
    // Either table or empty state
    const tableOrEmpty = this.page.locator('table, text=История транзакций суб-аккаунта пуста');
    await expect(tableOrEmpty.first()).toBeVisible();
  }

  // --- Analytics Tab ---
  async expectAnalyticsContent() {
    // Period buttons should be visible
    await expect(this.page.locator('button:has-text("7d")')).toBeVisible();
  }

  async selectAnalyticsPeriod(period: '7d' | '30d' | '90d') {
    await this.page.click(`button:has-text("${period}")`);
  }

  // --- API Keys Tab ---
  async expectAPIKeysContent() {
    await expect(this.page.locator('text=Просмотр API ключей суб-аккаунта')).toBeVisible();
  }

  // --- Webhooks Tab ---
  async expectWebhooksContent() {
    await expect(this.page.locator('text=Просмотр вебхуков суб-аккаунта')).toBeVisible();
  }
}
