import { Page, expect, Locator } from '@playwright/test';

export class TarificationPagePO {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/admin/tarification');
    // Wait for content to render rather than networkidle (SPA may have ongoing connections)
    await this.page.waitForTimeout(2000);
  }

  async expectPageLoaded() {
    // The page header renders "Тарификация" in a PageHeader component
    await expect(this.page.locator('h1, h2, [class*="text-2xl"], [class*="text-xl"]').filter({ hasText: 'Тарификация' }).first()).toBeVisible({ timeout: 10000 });
  }

  // --- Tab navigation ---

  async switchToTab(tab: 'Тарифные планы' | 'Периоды' | 'Тиры' | 'Регистрация отправителей') {
    await this.page.getByRole('tab', { name: tab }).click();
    await this.page.waitForTimeout(300);
  }

  async expectTabActive(tab: string) {
    await expect(this.page.getByRole('tab', { name: tab })).toHaveAttribute('data-state', 'active');
  }

  // --- Plans Tab ---

  async expectPlansTableVisible() {
    await expect(this.page.getByText('Тарифные планы').first()).toBeVisible();
  }

  async clickCreatePlan() {
    await this.page.getByRole('button', { name: 'Создать план' }).click();
  }

  async expectCreatePlanModal() {
    await expect(this.page.getByRole('dialog').getByRole('heading', { name: 'Создать тарифный план' })).toBeVisible();
  }

  async selectPlanOperator(name: string) {
    const dialog = this.page.getByRole('dialog');
    await dialog.locator('label:has-text("Оператор")').locator('..').locator('select').selectOption({ label: name });
  }

  async selectFirstPlanOperator() {
    const dialog = this.page.getByRole('dialog');
    const select = dialog.locator('label:has-text("Оператор")').locator('..').locator('select');
    const options = await select.locator('option').allTextContents();
    const realOption = options.find(o => o && o !== 'Выберите оператора');
    if (realOption) {
      await select.selectOption({ label: realOption });
    }
  }

  async selectPlanSenderCategory(category: string) {
    const dialog = this.page.getByRole('dialog');
    await dialog.locator('label:has-text("Категория отправителя")').locator('..').locator('select').selectOption({ label: category });
  }

  async selectPlanStrategy(strategy: string) {
    const dialog = this.page.getByRole('dialog');
    await dialog.locator('label:has-text("Стратегия тарификации")').locator('..').locator('select').selectOption({ label: strategy });
  }

  async submitCreatePlan() {
    await this.page.getByRole('dialog').getByRole('button', { name: 'Создать' }).click();
  }

  async cancelCreatePlan() {
    await this.page.getByRole('dialog').getByRole('button', { name: 'Отмена' }).click();
  }

  async getPlanRowCount(): Promise<number> {
    return this.page.locator('table tbody tr').count();
  }

  async clickTogglePlanActive(planName: string) {
    const row = this.page.locator('table tbody tr', { hasText: planName });
    const btn = row.getByRole('button', { name: /Активировать|Деактивировать/ });
    await btn.click();
  }

  // --- Periods Tab ---

  async expectPeriodsTabLoaded() {
    await expect(this.page.getByText('Иерархические периоды тарификации')).toBeVisible();
  }

  async clickCreatePeriod() {
    await this.page.getByRole('button', { name: 'Создать период' }).click();
  }

  async expectCreatePeriodModal() {
    await expect(this.page.getByRole('dialog').getByRole('heading', { name: 'Создать период тарификации' })).toBeVisible();
  }

  async selectPeriodStrategy(strategy: string) {
    const dialog = this.page.getByRole('dialog');
    await dialog.locator('label:has-text("Стратегия")').locator('..').locator('select').selectOption({ label: strategy });
  }

  async fillPeriodStartDate(date: string) {
    const dialog = this.page.getByRole('dialog');
    await dialog.locator('input[type="date"]').fill(date);
  }

  async submitCreatePeriod() {
    await this.page.getByRole('dialog').getByRole('button', { name: 'Создать' }).click();
  }

  async getPeriodRowCount(): Promise<number> {
    return this.page.locator('table tbody tr').count();
  }

  async clickEditPeriod(index: number) {
    const rows = this.page.locator('table tbody tr');
    await rows.nth(index).getByRole('button', { name: 'Изменить' }).click();
  }

  async clickDeletePeriod(index: number) {
    const rows = this.page.locator('table tbody tr');
    await rows.nth(index).getByRole('button', { name: 'Удалить' }).click();
  }

  async confirmDeletePeriod() {
    await this.page.getByRole('dialog').getByRole('button', { name: 'Удалить' }).click();
  }

  // --- Tiers Tab ---

  async expectTiersTabLoaded() {
    await expect(this.page.getByText('Тиры тарификации')).toBeVisible();
  }

  async clickCreateTier() {
    await this.page.getByRole('button', { name: 'Создать тир' }).click();
  }

  async expectCreateTierModal() {
    await expect(this.page.getByRole('dialog').getByRole('heading', { name: 'Создать тир' })).toBeVisible();
  }

  async fillTierFromCount(value: string) {
    const dialog = this.page.getByRole('dialog');
    await dialog.locator('label:has-text("От (кол-во сегментов)")').locator('..').locator('input').fill(value);
  }

  async fillTierPrice(value: string) {
    const dialog = this.page.getByRole('dialog');
    await dialog.locator('label:has-text("Цена за сегмент")').locator('..').locator('input').fill(value);
  }

  async selectTierPeriod() {
    const dialog = this.page.getByRole('dialog');
    const select = dialog.locator('label:has-text("Период")').locator('..').locator('select');
    const options = await select.locator('option').allTextContents();
    const realOption = options.find(o => o && o !== 'Выберите период');
    if (realOption) {
      await select.selectOption({ label: realOption });
    }
  }

  async submitCreateTier() {
    await this.page.getByRole('dialog').getByRole('button', { name: 'Создать' }).click();
  }

  async getTierRowCount(): Promise<number> {
    return this.page.locator('table tbody tr').count();
  }

  async clickEditTier(index: number) {
    const rows = this.page.locator('table tbody tr');
    await rows.nth(index).getByRole('button', { name: 'Изменить' }).click();
  }

  // --- Sender Registrations Tab ---

  async expectSenderRegistrationsTabLoaded() {
    // The tab contains sender registrations
    await this.page.waitForTimeout(500);
  }
}

// Client-facing tariffs page
export class TariffsPagePO {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/tariffs');
    await this.page.waitForLoadState('networkidle');
  }

  async expectPageLoaded() {
    // The tariffs page should show current plan info
    await expect(this.page.locator('body')).not.toBeEmpty();
  }

  async expectCurrentPlanVisible() {
    // Look for plan card
    await expect(this.page.getByText(/Текущий план|Ваш план/)).toBeVisible();
  }

  async expectPlanComparisonVisible() {
    // The grid of available plans
    await expect(this.page.getByText(/Сравнение планов|Доступные планы/).first()).toBeVisible();
  }

  async expectUsageBarVisible() {
    // Usage progress bar exists
    await expect(this.page.locator('[role="progressbar"], .bg-green-500, .bg-yellow-500, .bg-red-500').first()).toBeVisible();
  }

  async clickSwitchPlan(planName: string) {
    const planCard = this.page.locator('div', { hasText: planName });
    await planCard.getByRole('button', { name: /Переключить|Выбрать/ }).click();
  }
}
