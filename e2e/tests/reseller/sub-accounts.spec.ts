import { test, expect } from '@playwright/test';
import { SubAccountsListPage } from '../../pages/SubAccountsListPage';
import { ApiHelper } from '../../helpers/api';

test.use({ storageState: './reseller-auth-state.json' });

const UNIQUE_SUFFIX = () => Date.now().toString(36);

test.describe('Reseller — Суб-аккаунты: список и CRUD', () => {

  test('страница суб-аккаунтов загружается', async ({ page }) => {
    const listPage = new SubAccountsListPage(page);
    await listPage.goto();
    await listPage.expectLoaded();
  });

  test('отображается счётчик current/max', async ({ page }) => {
    const listPage = new SubAccountsListPage(page);
    await listPage.goto();
    await listPage.expectLoaded();
    await listPage.expectCountIndicator();
  });

  test('кнопка "Создать суб-аккаунт" открывает модальное окно', async ({ page }) => {
    const listPage = new SubAccountsListPage(page);
    await listPage.goto();
    await listPage.expectLoaded();
    await listPage.clickCreate();
    // Modal should be visible
    await expect(page.locator('[role="dialog"]')).toBeVisible();
    await listPage.cancelCreateForm();
  });

  test('валидация email при создании суб-аккаунта', async ({ page }) => {
    const listPage = new SubAccountsListPage(page);
    await listPage.goto();
    await listPage.expectLoaded();
    await listPage.clickCreate();
    await listPage.fillCreateForm({
      name: 'Test Invalid Email',
      email: 'not-an-email',
    });
    await listPage.submitCreateForm();
    await listPage.expectCreateFormEmailError();
  });

  test('создание суб-аккаунта → появляется в таблице', async ({ page, request }) => {
    const suffix = UNIQUE_SUFFIX();
    const subName = `E2E-SubAcc-${suffix}`;
    const subEmail = `e2e-sub-${suffix}@test.local`;

    const listPage = new SubAccountsListPage(page);
    await listPage.goto();
    await listPage.expectLoaded();
    await listPage.clickCreate();
    await listPage.fillCreateForm({
      name: subName,
      email: subEmail,
      dailyLimit: '500',
      monthlyLimit: '10000',
    });
    await listPage.submitCreateForm();
    await listPage.expectSuccessToast();

    // Verify in table
    await listPage.expectSubAccountInTable(subName);

    // Cleanup via API
    const api = new ApiHelper(request);
    const list = await api.listSubAccounts();
    const created = list.sub_accounts?.find((sa: { name: string }) => sa.name === subName);
    if (created) {
      await api.deleteSubAccount(created.id);
    }
  });

  test('создание суб-аккаунта с начальным балансом', async ({ page, request }) => {
    const suffix = UNIQUE_SUFFIX();
    const subName = `E2E-Balance-${suffix}`;
    const subEmail = `e2e-bal-${suffix}@test.local`;

    const listPage = new SubAccountsListPage(page);
    await listPage.goto();
    await listPage.expectLoaded();
    await listPage.clickCreate();

    // Should show parent balance
    await expect(page.locator('text=Доступный баланс')).toBeVisible({ timeout: 10_000 });

    await listPage.fillCreateForm({
      name: subName,
      email: subEmail,
      initialBalance: '1.00',
    });
    await listPage.submitCreateForm();
    await listPage.expectSuccessToast();

    // Cleanup
    const api = new ApiHelper(request);
    const list = await api.listSubAccounts();
    const created = list.sub_accounts?.find((sa: { name: string }) => sa.name === subName);
    if (created) {
      await api.deleteSubAccount(created.id);
    }
  });

  test('данные API совпадают с отображением в таблице', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const apiData = await api.listSubAccounts();

    const listPage = new SubAccountsListPage(page);
    await listPage.goto();
    await listPage.expectLoaded();

    if (apiData.sub_accounts?.length > 0) {
      await listPage.expectTable();
      const rowCount = await listPage.getTableRows().count();
      expect(rowCount).toBe(apiData.sub_accounts.length);
    }
  });
});
