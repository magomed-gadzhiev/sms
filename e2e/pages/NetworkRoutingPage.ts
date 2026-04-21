import { type Page, expect } from '@playwright/test';

export class NetworkRoutingPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/network/routing');
  }

  async expectLoaded() {
    await expect(this.page.locator('text=Маршрутизация сети')).toBeVisible();
  }

  // --- Tabs ---

  tabButton(label: 'Провайдеры' | 'Маршруты') {
    return this.page.locator(`button:has-text("${label}")`).first();
  }

  async clickTab(label: 'Провайдеры' | 'Маршруты') {
    await this.tabButton(label).click();
    await this.page.waitForTimeout(300);
  }

  async expectTabActive(label: 'Провайдеры' | 'Маршруты') {
    await expect(this.tabButton(label)).toHaveClass(/bg-primary/);
  }

  async expectTabInactive(label: 'Провайдеры' | 'Маршруты') {
    await expect(this.tabButton(label)).not.toHaveClass(/bg-primary/);
  }

  // --- Sub-account filter ---

  subFilter() {
    return this.page.locator('select').first();
  }

  async selectSubAccount(id: string) {
    await this.subFilter().selectOption(id);
    await this.page.waitForTimeout(400);
  }

  async expectSubAccountOption(name: string) {
    await expect(this.subFilter().locator(`option:has-text("${name}")`)).toHaveCount(1);
  }

  // --- Bulk assign button ---

  bulkAssignButton() {
    return this.page.locator('button:has-text("Массовое назначение")');
  }

  async openBulkAssignModal() {
    await this.bulkAssignButton().click();
    await expect(this.page.locator('text=Массовое назначение провайдера')).toBeVisible();
  }

  async closeBulkAssignModal() {
    await this.page.locator('button:has-text("Отмена")').click();
    await expect(this.page.locator('text=Массовое назначение провайдера')).not.toBeVisible();
  }

  // --- Bulk assign modal ---

  async selectBulkProvider(providerIdOrName: string) {
    // Modal условно рендерит <select> если есть providers, иначе <Input> (UUID).
    // Scope через [role="dialog"] чтобы не зацепить subFilter на странице.
    const modal = this.page.locator('[role="dialog"]');
    const select = modal.locator('select');
    if (await select.count()) {
      await select.selectOption(providerIdOrName);
    } else {
      await modal.locator('input[placeholder*="UUID"]').fill(providerIdOrName);
    }
  }

  async checkSubAccountInModal(name: string) {
    await this.page.locator(`label:has-text("${name}") input[type="checkbox"]`).check();
  }

  submitBulkButton() {
    return this.page.locator('button').filter({ hasText: /^Назначить/ });
  }

  async expectBulkButtonDisabled() {
    await expect(this.submitBulkButton()).toBeDisabled();
  }

  // --- Tables ---

  async expectProvidersTableVisible() {
    await expect(this.page.locator('th:has-text("Субаккаунт")')).toBeVisible();
    await expect(this.page.locator('th:has-text("Провайдер")')).toBeVisible();
    await expect(this.page.locator('th:has-text("Приоритет")')).toBeVisible();
    await expect(this.page.locator('th:has-text("Активен")')).toBeVisible();
  }

  async expectProvidersTableHasRow() {
    const rows = this.page.locator('tbody tr');
    await expect(rows.first()).toBeVisible();
    const firstSubAccount = rows.first().locator('td').first();
    await expect(firstSubAccount).not.toBeEmpty();
  }

  async expectActiveDotVisible() {
    await expect(this.page.locator('span.rounded-full').first()).toBeVisible();
  }

  // --- Empty states ---

  async expectEmptyProviders() {
    await expect(this.page.locator('text=Нет назначенных провайдеров')).toBeVisible();
  }

  async expectEmptyRoutes() {
    await expect(this.page.locator('text=Нет маршрутов')).toBeVisible();
  }
}
