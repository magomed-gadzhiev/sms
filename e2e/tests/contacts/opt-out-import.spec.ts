import { test, expect } from '@playwright/test';
import { OptOutPage } from '../../pages/OptOutPage';

test.use({ storageState: './auth-state.json' });

function uniquePhones(n: number): string[] {
  // Формат: +79 + 7 цифр из Date.now + 1 цифра индекса = 12 символов, валидный российский номер.
  const base = Date.now().toString().slice(-7);
  return Array.from({ length: n }, (_, i) => `+79${base}${i}`);
}

test.describe('Opt-Out — bulk import', () => {
  test('AC-C1 открытие модалки импорта', async ({ page }) => {
    const optOut = new OptOutPage(page);
    await optOut.goto();
    await optOut.openImport();
    const dialog = optOut.importDialog();
    await expect(dialog.locator('textarea')).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Импортировать' })).toBeVisible();
    await expect(dialog.locator('button:text-is("Закрыть")')).toBeVisible();
  });

  test('AC-C2/C3 импорт списка номеров → результат с imported/skipped → дубль даёт skipped', async ({ page }) => {
    const optOut = new OptOutPage(page);
    const phones = uniquePhones(3);

    await optOut.goto();

    // AC-C2: импорт трёх уникальных номеров через newline
    await optOut.openImport();
    await optOut.fillImportText(phones.join('\n'));
    await optOut.submitImport();
    await optOut.expectImportResult(3, 0);
    await optOut.closeImport();

    // Проверяем что номера появились в таблице
    for (const phone of phones) {
      await optOut.expectRow(phone);
    }

    // AC-C3: повторный импорт тех же номеров → все skipped
    await optOut.openImport();
    await optOut.fillImportText(phones.join(','));
    await optOut.submitImport();
    await optOut.expectImportResult(0, 3);
    await optOut.closeImport();

    // Cleanup: удаляем всё, что создали
    for (const phone of phones) {
      await optOut.deleteRow(phone);
    }
  });
});
