import { test, expect } from '@playwright/test';
import { ContactListsPage } from '../../pages/ContactListsPage';
import { ContactListDetailPage } from '../../pages/ContactListDetailPage';

test.use({ storageState: './auth-state.json' });

function uniqueName(prefix: string) {
  return `${prefix} ${Date.now().toString(36)}`;
}

test.describe('Контактные базы — карточка (/contact-lists/:id)', () => {
  let baseName: string;

  test.beforeEach(async ({ page }) => {
    baseName = uniqueName('E2E Detail');
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
      // Не проглатываем тихо: логируем в test-report. Падать из afterEach
      // не нужно (это маскирует настоящий fail теста), но след должен остаться.
      testInfo.annotations.push({
        type: 'cleanup-warning',
        description: `Не удалось удалить базу "${baseName}": ${String(err)}`,
      });
    }
  });

  test('AC-7 пустая карточка: кнопки действий и плейсхолдер', async ({ page }) => {
    const detail = new ContactListDetailPage(page);
    await detail.expectLoaded(baseName);
    await detail.expectActionButtons();
    await detail.expectEmptyContacts();
    await expect(page.locator('input#contact-search')).toHaveAttribute(
      'placeholder',
      'Поиск по номеру телефона...',
    );
  });

  test('AC-8 заголовки таблицы контактов', async ({ page }) => {
    const detail = new ContactListDetailPage(page);
    await detail.expectHeaders(['Телефон', 'Теги']);
  });

  test('AC-9/AC-10/AC-11 добавление → поиск "не найдено" → удаление контакта', async ({ page }) => {
    const detail = new ContactListDetailPage(page);
    const phone = `+7900${Date.now().toString().slice(-7)}`;

    // AC-9
    await detail.addContactViaUI(phone, 'vip, e2e');
    await detail.expectTag(phone, 'vip');
    await detail.expectTag(phone, 'e2e');

    // AC-10
    await detail.search('+79999000000');
    await detail.expectNotFound();

    // Сброс поиска — очищаем и ищем пустой строкой
    await page.locator('input#contact-search').fill('');
    await page.locator('button:has-text("Найти")').click();
    await detail.expectContactRow(phone);

    // AC-11
    await detail.deleteContact(phone);
    await detail.expectEmptyContacts();
  });
});
