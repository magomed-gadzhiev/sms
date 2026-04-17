import { type Page, expect } from '@playwright/test';

export class NetworkDashboardPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/network/dashboard');
  }

  async expectLoaded() {
    await expect(this.page.locator('text=Дашборд сети')).toBeVisible();
  }

  async expectBalanceCard() {
    await expect(this.page.locator('text=Баланс сети')).toBeVisible();
  }

  async expectTrafficCard() {
    await expect(this.page.locator('text=Трафик')).toBeVisible();
  }

  async expectDeliveryRateCard() {
    await expect(this.page.locator('text=Delivery Rate')).toBeVisible();
  }

  async expectModerationCard() {
    await expect(this.page.locator('text=Модерация')).toBeVisible();
  }

  async expectRevenueCard() {
    await expect(this.page.locator('text=Выручка сети')).toBeVisible();
  }

  async expectTopSubAccounts() {
    await expect(this.page.locator('text=Топ-5 субаккаунтов по трафику')).toBeVisible();
  }

  async expectProblemSubAccounts() {
    await expect(this.page.locator('text=Проблемные субаккаунты')).toBeVisible();
  }

  async selectPeriod(period: 'Сегодня' | '7 дней' | '30 дней') {
    await this.page.click(`button:has-text("${period}")`);
  }

  async expectAllMetricCards() {
    await this.expectBalanceCard();
    await this.expectTrafficCard();
    await this.expectDeliveryRateCard();
    await this.expectModerationCard();
    await this.expectRevenueCard();
    await this.expectTopSubAccounts();
    await this.expectProblemSubAccounts();
  }
}
