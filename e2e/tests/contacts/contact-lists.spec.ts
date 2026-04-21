import { test, expect } from '@playwright/test';
import { ContactListsPage } from '../../pages/ContactListsPage';

test.use({ storageState: './auth-state.json' });

function uniqueName(prefix: string) {
  return `${prefix} ${Date.now().toString(36)}`;
}

test.describe('Контактные базы — список (/contact-lists)', () => {
  test.fixme('AC-1 пустой стейт', async () => {
    // Снять когда появится изолированный клиент без существующих баз
    // (shared storageState накапливает базы из других тестов)
  });

  test('AC-2 открытие модалки создания', async ({ page }) => {
    const lists = new ContactListsPage(page);
    await lists.goto();
    await lists.openCreateModal();
    await expect(lists.createDialog().locator('text=Название *').first()).toBeVisible();
  });

  test('AC-4 кнопка "Создать" задизейблена при пустом названии', async ({ page }) => {
    const lists = new ContactListsPage(page);
    await lists.goto();
    await lists.openCreateModal();
    await expect(lists.createSubmitButton()).toBeDisabled();
  });

  test('AC-3/AC-5/AC-6 полный CRUD: создать → открыть карточку → вернуться → удалить', async ({ page }) => {
    const lists = new ContactListsPage(page);
    const name = uniqueName('E2E Base');

    await lists.goto();
    await lists.createViaUI(name, 'auto-generated');
    await lists.expectToast('Контактная база создана');

    // AC-5: навигация в карточку
    await lists.openRow(name);
    await expect(page).toHaveURL(/\/contact-lists\/[a-f0-9-]+$/);
    await expect(
      page.locator('h1, [class*="PageHeader"]').filter({ hasText: name }).first(),
    ).toBeVisible();
    await expect(page.getByRole('link', { name: 'Контактные базы' })).toBeVisible();

    // Возвращаемся на список через breadcrumb
    await page.getByRole('link', { name: 'Контактные базы' }).click();
    await expect(page).toHaveURL(/\/contact-lists$/);
    await lists.expectRowVisible(name);

    // AC-6: удаление
    await lists.deleteViaUI(name);
    await lists.expectToast('Контактная база удалена');
  });
});
