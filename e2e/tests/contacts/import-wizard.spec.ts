import { test, expect } from '@playwright/test';
import { ContactListsPage } from '../../pages/ContactListsPage';
import { ImportWizardPage } from '../../pages/ImportWizardPage';

test.use({ storageState: './auth-state.json' });

function uniqueName(prefix: string) {
  return `${prefix} ${Date.now().toString(36)}`;
}

function makeCsv(phones: string[]): string {
  const lines = ['phone,name', ...phones.map((p, i) => `${p},User${i}`)];
  return lines.join('\n');
}

test.describe('Контактные базы — Import Wizard', () => {
  let baseName: string;

  test.beforeEach(async ({ page }) => {
    baseName = uniqueName('E2E Import');
    const lists = new ContactListsPage(page);
    await lists.goto();
    await lists.createViaUI(baseName);
    await lists.openRow(baseName);
    await expect(page).toHaveURL(/\/contact-lists\/[a-f0-9-]+$/);
    // Переходим в импорт через кнопку "Импорт" на карточке
    await page.getByRole('button', { name: 'Импорт', exact: true }).click();
    await expect(page).toHaveURL(/\/contact-lists\/[a-f0-9-]+\/import$/);
  });

  test.afterEach(async ({ page }, testInfo) => {
    const lists = new ContactListsPage(page);
    try {
      await lists.goto();
      await lists.deleteViaUI(baseName);
    } catch (err) {
      testInfo.annotations.push({
        type: 'cleanup-warning',
        description: `Не удалось удалить базу "${baseName}": ${String(err)}`,
      });
    }
  });

  test('AC-B1 открытие wizard — шаг 1 активен, drop-zone и кнопка', async ({ page }) => {
    const wizard = new ImportWizardPage(page);
    await wizard.expectLoaded();
    await wizard.expectStepActive('1. Загрузка файла');
    await wizard.expectUploadStep();
  });

  test('AC-B2/B3/B4/B5/B6 полный flow: upload → auto-map → start → result → back', async ({ page }) => {
    test.setTimeout(120_000);
    const wizard = new ImportWizardPage(page);
    const phones = [
      `+79${Date.now().toString().slice(-7)}0`,
      `+79${Date.now().toString().slice(-7)}1`,
    ];
    const csv = makeCsv(phones);

    // AC-B2: загружаем CSV → шаг 2
    await wizard.uploadCsv('contacts.csv', csv);
    await wizard.expectMappingStep();
    await wizard.expectStepActive('2. Сопоставление колонок');

    // AC-B3: auto-detect — колонка "phone" автоматически замаплена
    await expect(wizard.mappingHeaderSelect('phone')).toHaveValue('phone');

    // AC-B4: кнопка Начать импорт enabled (phone mapping есть)
    await wizard.expectStartImportEnabled();

    // AC-B5: старт → progress → result
    await wizard.clickStartImport();
    await wizard.waitForResult(60_000);
    await wizard.expectStepActive('4. Результат');

    // AC-B6: показаны 2 строки всего, 2 импортировано, есть кнопка "Вернуться"
    await wizard.expectResultCounts(2, 2);
    await wizard.clickReturnToList();
    await expect(page).toHaveURL(/\/contact-lists\/[a-f0-9-]+$/);
    // Контакты действительно попали в базу
    for (const phone of phones) {
      await expect(page.locator('table tbody tr').filter({ hasText: phone })).toBeVisible();
    }
  });

  test('AC-B7 Назад на шаге mapping возвращает на upload', async ({ page }) => {
    const wizard = new ImportWizardPage(page);
    const phone = `+79${Date.now().toString().slice(-8)}`;
    await wizard.uploadCsv('c.csv', makeCsv([phone]));
    await wizard.expectMappingStep();
    await wizard.clickBack();
    await wizard.expectUploadStep();
    await wizard.expectStepActive('1. Загрузка файла');
  });

  test('AC-B8 невалидное расширение → inline error', async ({ page }) => {
    const wizard = new ImportWizardPage(page);
    await page.locator('input[type="file"]').setInputFiles({
      name: 'bad.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from('not a csv'),
    });
    await wizard.expectUploadError(/CSV|XLSX/);
    // Остались на шаге 1
    await wizard.expectStepActive('1. Загрузка файла');
  });
});
