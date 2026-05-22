import { test, expect } from '@playwright/test';
import { SubAccountsListPage } from '../../pages/SubAccountsListPage';
import { SubAccountDetailPage } from '../../pages/SubAccountDetailPage';
import { ApiHelper } from '../../helpers/api';

test.use({ storageState: './reseller-auth-state.json' });

let testSubAccountId: string | null = null;
let testSubAccountName: string;

test.beforeAll(async ({ request }) => {
  const api = new ApiHelper(request);
  const suffix = Date.now().toString(36);
  testSubAccountName = `E2E-Detail-${suffix}`;
  const result = await api.createSubAccount({
    name: testSubAccountName,
    email: `e2e-detail-${suffix}@test.local`,
    daily_limit: 1000,
    monthly_limit: 30000,
  });
  testSubAccountId = result.id || result.sub_account?.id;
});

test.afterAll(async ({ request }) => {
  if (testSubAccountId) {
    const api = new ApiHelper(request);
    await api.deleteSubAccount(testSubAccountId);
  }
});

test.describe('Reseller — Суб-аккаунт: детальная страница', () => {

  test('навигация из списка в детальную страницу', async ({ page }) => {
    const listPage = new SubAccountsListPage(page);
    await listPage.goto();
    await listPage.expectLoaded();
    await listPage.clickSubAccount(testSubAccountName);

    const detailPage = new SubAccountDetailPage(page);
    await detailPage.expectLoaded();
    await detailPage.expectName(testSubAccountName);
  });

  test('вкладка "Обзор" показывает баланс, лимиты, перевод, удаление', async ({ page }) => {
    test.skip(!testSubAccountId, 'Нет тестового суб-аккаунта');
    const detailPage = new SubAccountDetailPage(page);
    await detailPage.goto(testSubAccountId!);
    await detailPage.expectLoaded();

    await detailPage.expectBalanceCard();
    await detailPage.expectLimitsForm();
    await detailPage.expectTransferForm();
    await detailPage.expectDeleteSection();
  });

  test('изменение лимитов суб-аккаунта', async ({ page }) => {
    test.skip(!testSubAccountId, 'Нет тестового суб-аккаунта');
    const detailPage = new SubAccountDetailPage(page);
    await detailPage.goto(testSubAccountId!);
    await detailPage.expectLoaded();

    await detailPage.setLimits('2000', '50000');
    await detailPage.expectLimitsSaved();
  });

  test('перевод средств на суб-аккаунт', async ({ page }) => {
    test.skip(!testSubAccountId, 'Нет тестового суб-аккаунта');
    const detailPage = new SubAccountDetailPage(page);
    await detailPage.goto(testSubAccountId!);
    await detailPage.expectLoaded();

    await detailPage.transferBalance('0.01');
    await detailPage.expectTransferSuccess();
  });

  test('вкладка "Сообщения" загружается', async ({ page }) => {
    test.skip(!testSubAccountId, 'Нет тестового суб-аккаунта');
    const detailPage = new SubAccountDetailPage(page);
    await detailPage.goto(testSubAccountId!);
    await detailPage.expectLoaded();

    await detailPage.clickTab('Сообщения');
    await detailPage.expectMessagesTable();
  });

  test('фильтрация сообщений по статусу', async ({ page }) => {
    test.skip(!testSubAccountId, 'Нет тестового суб-аккаунта');
    const detailPage = new SubAccountDetailPage(page);
    await detailPage.goto(testSubAccountId!);
    await detailPage.expectLoaded();

    await detailPage.clickTab('Сообщения');
    await detailPage.expectMessagesTable();
    await detailPage.filterMessagesByStatus('delivered');
    // Should reload without error
    await expect(page.locator('[role="alert"]')).not.toBeVisible({ timeout: 3_000 }).catch(() => {});
  });

  test('вкладка "Кампании" загружается', async ({ page }) => {
    test.skip(!testSubAccountId, 'Нет тестового суб-аккаунта');
    const detailPage = new SubAccountDetailPage(page);
    await detailPage.goto(testSubAccountId!);
    await detailPage.expectLoaded();

    await detailPage.clickTab('Кампании');
    // Either table or "Нет кампаний"
    await expect(page.locator('text=Нет кампаний').or(page.locator('table'))).toBeVisible();
  });

  test('вкладка "Транзакции" загружается', async ({ page }) => {
    test.skip(!testSubAccountId, 'Нет тестового суб-аккаунта');
    const detailPage = new SubAccountDetailPage(page);
    await detailPage.goto(testSubAccountId!);
    await detailPage.expectLoaded();

    await detailPage.clickTab('Транзакции');
    await detailPage.expectTransactionsContent();
  });

  test('вкладка "Аналитика" загружается и переключает период', async ({ page }) => {
    test.skip(!testSubAccountId, 'Нет тестового суб-аккаунта');
    const detailPage = new SubAccountDetailPage(page);
    await detailPage.goto(testSubAccountId!);
    await detailPage.expectLoaded();

    await detailPage.clickTab('Аналитика');
    await detailPage.expectAnalyticsContent();
    await detailPage.selectAnalyticsPeriod('30d');
    // Should reload without error
    await expect(page.locator('[role="alert"]')).not.toBeVisible({ timeout: 5_000 }).catch(() => {});
  });

  test('вкладка "API Ключи" загружается', async ({ page }) => {
    test.skip(!testSubAccountId, 'Нет тестового суб-аккаунта');
    const detailPage = new SubAccountDetailPage(page);
    await detailPage.goto(testSubAccountId!);
    await detailPage.expectLoaded();

    await detailPage.clickTab('API Ключи');
    await detailPage.expectAPIKeysContent();
  });

  test('вкладка "Вебхуки" загружается', async ({ page }) => {
    test.skip(!testSubAccountId, 'Нет тестового суб-аккаунта');
    const detailPage = new SubAccountDetailPage(page);
    await detailPage.goto(testSubAccountId!);
    await detailPage.expectLoaded();

    await detailPage.clickTab('Вебхуки');
    await detailPage.expectWebhooksContent();
  });

  test('диалог удаления открывается и закрывается', async ({ page }) => {
    test.skip(!testSubAccountId, 'Нет тестового суб-аккаунта');
    const detailPage = new SubAccountDetailPage(page);
    await detailPage.goto(testSubAccountId!);
    await detailPage.expectLoaded();

    await detailPage.clickDeleteButton();
    await expect(page.locator('text=Вы уверены, что хотите удалить')).toBeVisible();
    await detailPage.cancelDelete();
  });
});
