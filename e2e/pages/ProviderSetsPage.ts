import { type Page, expect } from '@playwright/test';

/**
 * Page-object for /network/provider-sets (Plan 1).
 * Список + детали выбранного set'а: rename, добавление items, удаление.
 */
export class ProviderSetsPage {
  constructor(private readonly page: Page) {}

  async goto() {
    await this.page.goto('/network/provider-sets');
  }

  async expectLoaded() {
    await expect(this.page.locator('h1, h2').filter({ hasText: 'Provider-sets' }).first()).toBeVisible();
  }

  /** Создаёт новый set через "+ Создать" модал. */
  async create(name: string) {
    await this.page.click('button:has-text("+ Создать")');
    const modal = this.page.locator('[role="dialog"]').filter({ hasText: 'Создать provider-set' }).first();
    await expect(modal).toBeVisible();
    await modal.locator('label:has-text("Имя") + input, label:has-text("Имя") input').first().fill(name);
    await modal.locator('button:has-text("Создать")').click();
    await expect(this.page.locator('text=Создан').first()).toBeVisible({ timeout: 8_000 });
    // После create страница автоматически выбирает новый set — карточка с "По умолчанию" появляется.
    await expect(this.page.locator('text=По умолчанию для новых суб-аккаунтов').first()).toBeVisible();
  }

  /** Кликает по элементу левого списка. */
  async selectByName(name: string) {
    await this.page.locator('aside button').filter({ hasText: name }).first().click();
    await expect(this.page.locator('text=По умолчанию для новых суб-аккаунтов').first()).toBeVisible();
  }

  /** Открывает "+ Добавить провайдера" и кликает по первому подходящему. */
  async addItemByName(providerName: string) {
    await this.page.click('button:has-text("+ Добавить провайдера")');
    const modal = this.page.locator('[role="dialog"]').filter({ hasText: 'Добавить провайдера' }).first();
    await expect(modal).toBeVisible();
    // Click конкретного провайдера.
    await modal.locator('button').filter({ hasText: providerName }).first().click();
    // Modal закроется, items table перерисуется.
    await expect(modal).not.toBeVisible({ timeout: 5_000 });
  }

  async expectItemRow(providerName: string) {
    await expect(this.page.locator(`tbody tr:has-text("${providerName}")`).first()).toBeVisible({ timeout: 8_000 });
  }
}
