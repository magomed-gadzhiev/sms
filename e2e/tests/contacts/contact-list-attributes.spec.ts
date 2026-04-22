import { test, expect } from '@playwright/test';
import { ContactListsPage } from '../../pages/ContactListsPage';
import { ContactListDetailPage } from '../../pages/ContactListDetailPage';

test.use({ storageState: './auth-state.json' });

function uniqueName(prefix: string) {
  return `${prefix} ${Date.now().toString(36)}`;
}

test.describe('Контактные базы — настройки атрибутов', () => {
  let baseName: string;

  test.beforeEach(async ({ page }) => {
    baseName = uniqueName('E2E Attrs');
    const lists = new ContactListsPage(page);
    await lists.goto();
    await lists.createViaUI(baseName);
    await lists.openRow(baseName);
    await expect(page).toHaveURL(/\/contact-lists\/[a-f0-9-]+$/);
  });

  test.afterEach(async ({ page }, testInfo) => {
    const lists = new ContactListsPage(page);
    await lists.goto();
    try {
      await lists.deleteViaUI(baseName);
    } catch (err) {
      testInfo.annotations.push({
        type: 'cleanup-warning',
        description: `Не удалось удалить базу "${baseName}": ${String(err)}`,
      });
    }
  });

  test('AC-A1 открытие модалки атрибутов', async ({ page }) => {
    const detail = new ContactListDetailPage(page);
    await detail.openAttrsModal();
    const dialog = detail.attrsDialog();
    await expect(dialog.getByRole('button', { name: '+ Добавить атрибут' })).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Отмена' })).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Сохранить' })).toBeVisible();
  });

  test('AC-A3 валидация: пустое системное имя → alert', async ({ page }) => {
    const detail = new ContactListDetailPage(page);
    await detail.openAttrsModal();
    await detail.addAttributeRow();
    // Native alert блокирует JS; handler надо зарегистрировать ДО click,
    // иначе click не резолвится пока alert открыт.
    let alertMessage = '';
    page.once('dialog', async (dialog) => {
      alertMessage = dialog.message();
      await dialog.accept();
    });
    await detail.attrsDialog().getByRole('button', { name: 'Сохранить' }).click();
    expect(alertMessage).toContain('системное имя');
    // Модалка осталась открытой — сохранение не произошло
    await expect(detail.attrsDialog()).toBeVisible();
    await detail.cancelAttributes();
  });

  test('AC-A2/A4/A5 добавить атрибут → колонка в таблице → number-поле в add-contact → удалить', async ({ page }) => {
    const detail = new ContactListDetailPage(page);

    // AC-A2: создаём атрибут "city" (Город, number для A5-проверки)
    await detail.openAttrsModal();
    await detail.addAttributeRow();
    await detail.fillAttribute(0, { name: 'city', displayName: 'Город', type: 'number' });
    await detail.saveAttributes();

    // AC-A2: колонка "Город" появилась в таблице контактов
    await expect(
      page.locator('table thead th').filter({ hasText: 'Город' }).first(),
    ).toBeVisible();

    // AC-A5: в модалке "Добавить контакт" соответствующее поле имеет type=number
    await detail.openAddContact();
    const addDialog = detail.addDialog();
    // Поле атрибута помечено label'ом "Город"
    const cityLabel = addDialog.locator('label', { hasText: 'Город' });
    await expect(cityLabel).toBeVisible();
    // Input сразу после этого label должен быть number-типа
    const cityInput = addDialog.locator('input[type="number"]');
    await expect(cityInput).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(addDialog).toBeHidden();

    // AC-A4: удаляем атрибут, колонка исчезает
    await detail.openAttrsModal();
    await detail.removeAttributeRow(0);
    await detail.saveAttributes();
    await expect(page.locator('table thead th').filter({ hasText: 'Город' })).toHaveCount(0);
  });
});
