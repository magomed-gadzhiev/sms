import { Page, expect, Locator } from '@playwright/test';

export class RoutingPagePO {
  private searchInput: Locator;
  private addRouteBtn: Locator;
  private routeTable: Locator;

  constructor(private page: Page) {
    this.searchInput = page.locator('input[placeholder="Поиск по названию..."]');
    this.addRouteBtn = page.getByRole('button', { name: '+ Добавить маршрут' });
    this.routeTable = page.locator('table');
  }

  async goto() {
    await this.page.goto('/routing');
    await this.page.waitForLoadState('networkidle');
  }

  async expectPageLoaded() {
    await expect(this.page.getByText('Общие маршруты')).toBeVisible();
  }

  async expectRoutesTableVisible() {
    await expect(this.routeTable).toBeVisible();
  }

  async expectEmptyState() {
    await expect(this.page.getByText('Нет маршрутов')).toBeVisible();
  }

  async getRouteRowCount(): Promise<number> {
    return this.routeTable.locator('tbody tr').count();
  }

  async search(text: string) {
    await this.searchInput.fill(text);
    await this.page.waitForTimeout(500); // debounce
  }

  async filterByType(type: 'Все' | 'SMS' | 'HLR' | 'MAX') {
    const typeSection = this.page.locator('text=Тип:').locator('..');
    await typeSection.getByRole('button', { name: type, exact: true }).click();
    await this.page.waitForTimeout(300);
  }

  async filterByStatus(status: 'Все' | 'Активные' | 'Черновики') {
    const statusSection = this.page.locator('text=Статус:').locator('..');
    await statusSection.getByRole('button', { name: status, exact: true }).click();
    await this.page.waitForTimeout(300);
  }

  async clickAddRoute() {
    await this.addRouteBtn.click();
  }

  async expectModalOpen(title: string) {
    await expect(this.page.getByRole('dialog').getByRole('heading', { name: title })).toBeVisible();
  }

  async fillRouteName(name: string) {
    await this.page.getByRole('dialog').locator('input').first().fill(name);
  }

  async selectRouteType(type: 'SMS' | 'HLR' | 'MAX') {
    await this.page.getByRole('dialog').getByRole('button', { name: type, exact: true }).click();
  }

  async fillPriority(value: number) {
    const dialog = this.page.getByRole('dialog');
    await dialog.locator('label:has-text("Приоритет")').locator('..').locator('input').fill(String(value));
  }

  async fillShare(value: number) {
    const dialog = this.page.getByRole('dialog');
    await dialog.locator('label:has-text("Доля")').locator('..').locator('input').fill(String(value));
  }

  async selectProvider(providerName: string) {
    const dialog = this.page.getByRole('dialog');
    // SearchableSelect — click to open, then select option
    await dialog.locator('label:has-text("Провайдер")').locator('..').click();
    await this.page.getByText(providerName, { exact: false }).first().click();
  }

  async selectFirstProvider() {
    const dialog = this.page.getByRole('dialog');
    const searchableSelect = dialog.locator('text=Провайдер').locator('..');
    const input = searchableSelect.locator('input');
    await input.click();
    // Wait for options to appear and click first one
    const option = this.page.locator('[role="option"]').first();
    if (await option.isVisible({ timeout: 3000 }).catch(() => false)) {
      await option.click();
    } else {
      // Fallback: try listbox items
      const listItem = this.page.locator('[role="listbox"] > *').first();
      if (await listItem.isVisible({ timeout: 2000 }).catch(() => false)) {
        await listItem.click();
      }
    }
  }

  async saveAsDraft() {
    await this.page.getByRole('dialog').getByRole('button', { name: 'Сохранить как черновик' }).click();
  }

  async saveAsActive() {
    await this.page.getByRole('dialog').getByRole('button', { name: 'Сохранить маршрут' }).click();
  }

  async expectModalError(text: string) {
    await expect(this.page.getByRole('dialog').locator('.text-red-600')).toContainText(text);
  }

  async clickEditRoute(routeName: string) {
    const row = this.routeTable.locator('tr', { hasText: routeName });
    await row.getByRole('button', { name: 'Изменить' }).click();
  }

  async clickDeleteRoute(routeName: string) {
    const row = this.routeTable.locator('tr', { hasText: routeName });
    await row.getByRole('button', { name: 'Удалить' }).click();
  }

  async confirmDelete() {
    await this.page.getByRole('dialog').getByRole('button', { name: 'Удалить' }).click();
  }

  async cancelDelete() {
    await this.page.getByRole('dialog').getByRole('button', { name: 'Отмена' }).click();
  }

  async expectRouteInTable(name: string) {
    await expect(this.routeTable.getByText(name)).toBeVisible();
  }

  async expectRouteNotInTable(name: string) {
    await expect(this.routeTable.getByText(name)).not.toBeVisible();
  }

  async getRouteNames(): Promise<string[]> {
    const cells = this.routeTable.locator('tbody tr td:first-child .font-medium');
    return cells.allTextContents();
  }
}
