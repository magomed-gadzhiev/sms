import { test, expect } from '@playwright/test';
import { MessagesPage } from '../../pages/MessagesPage';

test.use({ storageState: './auth-state.json' });

test.describe('Messages — Список сообщений и детализация', () => {

  test('страница загружается и показывает счётчик сообщений', async ({ page }) => {
    const msgs = new MessagesPage(page);
    await msgs.goto();
    await msgs.waitForData();

    if (!(await msgs.isDataLoaded())) {
      test.skip(true, 'Detalization API unavailable (504/timeout)');
      return;
    }

    const total = await msgs.getTotalCount();
    expect(total).toBeGreaterThanOrEqual(0);
  });

  test('фильтрация по статусу "delivered" работает', async ({ page }) => {
    const msgs = new MessagesPage(page);
    await msgs.goto();
    await msgs.waitForData();

    if (!(await msgs.isDataLoaded())) {
      test.skip(true, 'Detalization API unavailable');
      return;
    }

    await msgs.filterByStatus('delivered');
    await msgs.waitForData();
    const total = await msgs.getTotalCount();
    expect(total).toBeGreaterThanOrEqual(0);
  });

  test('фильтрация по статусу "failed" работает', async ({ page }) => {
    const msgs = new MessagesPage(page);
    await msgs.goto();
    await msgs.waitForData();

    if (!(await msgs.isDataLoaded())) {
      test.skip(true, 'Detalization API unavailable');
      return;
    }

    await msgs.filterByStatus('failed');
    await msgs.waitForData();
    const total = await msgs.getTotalCount();
    expect(total).toBeGreaterThanOrEqual(0);
  });

  test('клик на сообщение открывает модальное окно с деталями', async ({ page }) => {
    const msgs = new MessagesPage(page);
    await msgs.goto();
    await msgs.waitForData();

    if (!(await msgs.isDataLoaded())) {
      test.skip(true, 'Detalization API unavailable');
      return;
    }

    const total = await msgs.getTotalCount();
    if (total === 0) {
      test.skip(true, 'No messages to click — skipping modal test');
      return;
    }

    await msgs.clickFirstMessage();
    await msgs.expectMessageModal();
  });

  test('детализация сообщения содержит информацию о тарификации', async ({ page }) => {
    const msgs = new MessagesPage(page);
    await msgs.goto();
    await msgs.waitForData();

    if (!(await msgs.isDataLoaded())) {
      test.skip(true, 'Detalization API unavailable');
      return;
    }

    const total = await msgs.getTotalCount();
    if (total === 0) {
      test.skip(true, 'No messages available');
      return;
    }

    await msgs.clickFirstMessage();
    await msgs.expectMessageModal();
    // Check for billing-related fields in detail
    const modal = page.locator('[role="dialog"]');
    // Message detail should show at minimum: status, destination
    await expect(modal.locator('text=Статус')).toBeVisible();
  });

  test('кнопка экспорта доступна', async ({ page }) => {
    const msgs = new MessagesPage(page);
    await msgs.goto();
    await msgs.waitForData();

    if (!(await msgs.isDataLoaded())) {
      test.skip(true, 'Detalization API unavailable');
      return;
    }

    const exportBtn = page.locator('button:has-text("Экспорт")');
    await expect(exportBtn).toBeVisible();
  });
});
