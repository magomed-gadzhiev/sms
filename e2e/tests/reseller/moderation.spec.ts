import { test, expect } from '@playwright/test';
import { ModerationPage } from '../../pages/ModerationPage';
import { ApiHelper } from '../../helpers/api';

test.use({ storageState: './reseller-auth-state.json' });

test.describe('Reseller — Модерация', () => {

  test('страница модерации загружается', async ({ page }) => {
    const modPage = new ModerationPage(page);
    await modPage.goto();
    await modPage.expectLoaded();
  });

  test('три вкладки видны: Имена, Шаблоны, Регистрации', async ({ page }) => {
    const modPage = new ModerationPage(page);
    await modPage.goto();
    await modPage.expectLoaded();

    await expect(page.locator('button:has-text("Имена отправителей")')).toBeVisible();
    await expect(page.locator('button:has-text("Шаблоны")')).toBeVisible();
    await expect(page.locator('button:has-text("Регистрации у операторов")')).toBeVisible();
  });

  // --- Sender Names Tab ---

  test('вкладка "Имена отправителей" загружается', async ({ page }) => {
    const modPage = new ModerationPage(page);
    await modPage.goto();
    await modPage.expectLoaded();

    await modPage.clickSenderNamesTab();
    // Either table or empty state
    const content = page.locator('table').or(page.locator('text=Нет заявок'));
    await expect(content.first()).toBeVisible();
  });

  test('фильтр статуса для имён отправителей', async ({ page }) => {
    const modPage = new ModerationPage(page);
    await modPage.goto();
    await modPage.expectLoaded();

    await modPage.clickSenderNamesTab();
    await modPage.filterByStatus('pending');
    // Should not error
    const content = page.locator('table').or(page.locator('text=Нет заявок'));
    await expect(content.first()).toBeVisible();

    await modPage.filterByStatus('approved');
    await expect(content.first()).toBeVisible();

    await modPage.clearStatusFilter();
    await expect(content.first()).toBeVisible();
  });

  // --- Templates Tab ---

  test('вкладка "Шаблоны" загружается', async ({ page }) => {
    const modPage = new ModerationPage(page);
    await modPage.goto();
    await modPage.expectLoaded();

    await modPage.clickTemplatesTab();
    const content = page.locator('table').or(page.locator('text=Нет заявок'));
    await expect(content.first()).toBeVisible();
  });

  test('фильтр статуса для шаблонов', async ({ page }) => {
    const modPage = new ModerationPage(page);
    await modPage.goto();
    await modPage.expectLoaded();

    await modPage.clickTemplatesTab();
    await modPage.filterByStatus('pending');
    const content = page.locator('table').or(page.locator('text=Нет заявок'));
    await expect(content.first()).toBeVisible();

    await modPage.filterByStatus('revision_requested');
    await expect(content.first()).toBeVisible();
  });

  // --- Registrations Tab ---

  test('вкладка "Регистрации у операторов" загружается', async ({ page }) => {
    const modPage = new ModerationPage(page);
    await modPage.goto();
    await modPage.expectLoaded();

    await modPage.clickRegistrationsTab();
    const content = page.locator('table').or(page.locator('text=Нет заявок'));
    await expect(content.first()).toBeVisible();
  });

  test('фильтр статуса для регистраций', async ({ page }) => {
    const modPage = new ModerationPage(page);
    await modPage.goto();
    await modPage.expectLoaded();

    await modPage.clickRegistrationsTab();
    await modPage.filterByStatus('submitted');
    const content = page.locator('table').or(page.locator('text=Нет заявок'));
    await expect(content.first()).toBeVisible();
  });

  // --- Moderation Counts API ---

  test('API возвращает счётчики модерации', async ({ request }) => {
    const api = new ApiHelper(request);
    const counts = await api.getModerationCounts();

    expect(counts).toHaveProperty('sender_names');
    expect(counts).toHaveProperty('templates');
    expect(counts).toHaveProperty('registrations');
    expect(typeof counts.sender_names).toBe('number');
    expect(typeof counts.templates).toBe('number');
    expect(typeof counts.registrations).toBe('number');
  });

  // --- Modal interaction ---

  test('модалка отклонения: кнопка "Подтвердить" заблокирована без причины', async ({ page }) => {
    const modPage = new ModerationPage(page);
    await modPage.goto();
    await modPage.expectLoaded();

    // Try to find a pending sender name to test reject modal
    await modPage.clickSenderNamesTab();
    await modPage.filterByStatus('pending');

    const rejectButtons = page.locator('button:has-text("Отклонить")');
    const count = await rejectButtons.count();

    if (count > 0) {
      await rejectButtons.first().click();
      // Modal should be visible
      await expect(page.locator('[role="dialog"]')).toBeVisible();
      // Confirm button should be disabled without text
      const confirmBtn = page.locator('[role="dialog"] button:has-text("Подтвердить")');
      await expect(confirmBtn).toBeDisabled();
      // Fill reason
      await modPage.fillRejectReason('E2E тест — причина отклонения');
      await expect(confirmBtn).toBeEnabled();
      // Cancel instead of submitting
      await modPage.cancelModal();
    } else {
      test.skip(true, 'Нет ожидающих заявок для тестирования модалки');
    }
  });
});
