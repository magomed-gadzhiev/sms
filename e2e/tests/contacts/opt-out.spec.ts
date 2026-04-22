import { test, expect } from '@playwright/test';
import { OptOutPage } from '../../pages/OptOutPage';

test.use({ storageState: './auth-state.json' });

test.describe('Opt-Out список (/opt-out)', () => {
  test('AC-12/AC-13/AC-14 добавление → поиск → удаление', async ({ page }) => {
    const optOut = new OptOutPage(page);
    const phone = `+7900555${Date.now().toString().slice(-4)}`;

    await optOut.goto();

    // AC-12
    await optOut.openAdd();
    await optOut.fillAdd(phone, 'STOP');
    await optOut.submitAdd();
    await optOut.expectToast('Номер добавлен в список отписок');
    await optOut.expectRow(phone);
    await expect(optOut.row(phone).getByText('STOP', { exact: true })).toBeVisible();

    // AC-13: поиск
    await optOut.search(phone.replace(/\D/g, '').slice(-10));
    await optOut.expectRow(phone);
    // После фильтра в таблице должна остаться ровно одна строка с этим номером
    await expect(optOut.row(phone)).toHaveCount(1);
    await optOut.resetSearch();
    await optOut.expectRow(phone);

    // AC-14: удаление
    await optOut.deleteRow(phone);
    await optOut.expectToast('Номер удалён из списка отписок');
    await optOut.expectRowHidden(phone);
  });
});
