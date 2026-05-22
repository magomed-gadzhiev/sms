import { type Page, expect } from '@playwright/test';

export class ImportWizardPage {
  constructor(private page: Page) {}

  async expectLoaded() {
    await expect(
      this.page.getByRole('heading', { level: 1, name: 'Импорт контактов' }),
    ).toBeVisible();
  }

  // Step indicator helpers
  async expectStepActive(label: string) {
    // Активный шаг: div.bg-blue-100 с текстом шага
    const active = this.page.locator('div.bg-blue-100').filter({ hasText: label });
    await expect(active).toBeVisible();
  }

  // --- Step 1: Upload ---

  async expectUploadStep() {
    await expect(this.page.getByText('Перетащите CSV или XLSX файл сюда')).toBeVisible();
    await expect(this.page.getByRole('button', { name: 'Выбрать файл' })).toBeVisible();
  }

  async uploadCsv(fileName: string, content: string) {
    await this.page.locator('input[type="file"]').setInputFiles({
      name: fileName,
      mimeType: 'text/csv',
      buffer: Buffer.from(content, 'utf-8'),
    });
  }

  async expectUploadError(text: string | RegExp) {
    await expect(this.page.locator('.bg-red-50.border.border-red-200')).toContainText(text);
  }

  // --- Step 2: Mapping ---

  async expectMappingStep() {
    await expect(
      this.page.getByRole('heading', { name: 'Сопоставление колонок' }),
    ).toBeVisible();
  }

  mappingHeaderSelect(headerText: string) {
    return this.page
      .locator('table thead th')
      .filter({ hasText: headerText })
      .locator('select');
  }

  async setMapping(headerText: string, value: string) {
    await this.mappingHeaderSelect(headerText).selectOption(value);
  }

  async expectStartImportDisabled() {
    await expect(this.page.getByRole('button', { name: /Начать импорт|Запуск/ })).toBeDisabled();
  }

  async expectStartImportEnabled() {
    await expect(this.page.getByRole('button', { name: 'Начать импорт' })).toBeEnabled();
  }

  async clickStartImport() {
    await this.page.getByRole('button', { name: 'Начать импорт' }).click();
  }

  async clickBack() {
    await this.page.getByRole('button', { name: 'Назад' }).click();
  }

  // --- Step 3: Progress ---

  async expectProgressStep() {
    await expect(
      this.page.getByRole('heading', { name: 'Импорт в процессе...' }),
    ).toBeVisible();
  }

  // --- Step 4: Result ---

  async waitForResult(timeoutMs = 60_000) {
    await expect(
      this.page
        .getByRole('heading', { name: /Импорт завершён/ })
        .first(),
    ).toBeVisible({ timeout: timeoutMs });
  }

  async expectResultCounts(totalRows: number, imported: number) {
    // В результат-блоке 4 ячейки: Всего / Импортировано / Обновлено / Ошибки
    const totalCell = this.page.locator('.bg-gray-50').filter({ hasText: 'Всего строк' }).locator('.text-2xl');
    const importedCell = this.page.locator('.bg-green-50').filter({ hasText: 'Импортировано' }).locator('.text-2xl');
    await expect(totalCell).toHaveText(String(totalRows));
    await expect(importedCell).toHaveText(String(imported));
  }

  async clickReturnToList() {
    await this.page.getByRole('button', { name: 'Вернуться к базе' }).click();
  }
}
