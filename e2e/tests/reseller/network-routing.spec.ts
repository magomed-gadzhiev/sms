import { test, expect } from '@playwright/test';
import { NetworkRoutingPage } from '../../pages/NetworkRoutingPage';
import { ApiHelper } from '../../helpers/api';
import { pickProviderId } from '../../helpers/routing';

test.use({ storageState: './reseller-auth-state.json' });

const TEST_PREFIX = 'E2E NR';
let createdSubAccountIds: string[] = [];

test.afterEach(async ({ request }) => {
  const api = new ApiHelper(request, 'reseller-auth-state.json');
  for (const id of createdSubAccountIds) {
    await api.deleteSubAccount(id).catch(() => {});
  }
  createdSubAccountIds = [];
});

async function seedSubAccount(api: ApiHelper, suffix: string): Promise<{ id: string; name: string }> {
  const name = `${TEST_PREFIX} ${suffix} ${Date.now().toString(36)}`;
  const res = await api.createSubAccount({
    name,
    email: `e2e-nr-${Date.now().toString(36)}@test.local`,
  });
  const id: string = res.id ?? res.sub_account?.id;
  if (!id) throw new Error(`seedSubAccount: нет id в ответе: ${JSON.stringify(res)}`);
  createdSubAccountIds.push(id);
  return { id, name };
}

// ─── Seed-free ────────────────────────────────────────────────────────────────

test.describe('NetworkRouting — загрузка и навигация', () => {

  test('AC-N1: страница загружается с заголовком и табами', async ({ page }) => {
    const p = new NetworkRoutingPage(page);
    await p.goto();
    await p.expectLoaded();
    await expect(p.tabButton('Провайдеры')).toBeVisible();
    await expect(p.tabButton('Маршруты')).toBeVisible();
  });

  test('AC-N2: таб «Провайдеры» активен по умолчанию, кнопка «Массовое назначение» видна', async ({ page }) => {
    const p = new NetworkRoutingPage(page);
    await p.goto();
    await p.expectLoaded();
    await p.expectTabActive('Провайдеры');
    await p.expectTabInactive('Маршруты');
    await expect(p.bulkAssignButton()).toBeVisible();
  });

  test('AC-N3: переключение на «Маршруты» убирает кнопку «Массовое назначение»', async ({ page }) => {
    const p = new NetworkRoutingPage(page);
    await p.goto();
    await p.expectLoaded();
    await p.clickTab('Маршруты');
    await p.expectTabActive('Маршруты');
    await p.expectTabInactive('Провайдеры');
    await expect(p.bulkAssignButton()).not.toBeVisible();
  });

  test('AC-N9: bulk-assign modal открывается и закрывается кнопкой «Отмена»', async ({ page }) => {
    const p = new NetworkRoutingPage(page);
    await p.goto();
    await p.expectLoaded();
    await p.openBulkAssignModal();
    await expect(page.locator('text=Массовое назначение провайдера')).toBeVisible();
    await p.closeBulkAssignModal();
  });

  test('AC-N10: кнопка «Назначить» disabled если ничего не выбрано', async ({ page }) => {
    const p = new NetworkRoutingPage(page);
    await p.goto();
    await p.expectLoaded();
    await p.openBulkAssignModal();
    await p.expectBulkButtonDisabled();
  });

});

// ─── API-level ────────────────────────────────────────────────────────────────

test.describe('NetworkRouting — API', () => {

  test('AC-N7: GET /reseller/routing/providers возвращает 200 и массив providers', async ({ request }) => {
    const api = new ApiHelper(request, 'reseller-auth-state.json');
    const data = await api.listNetworkProviders();
    expect(data).toHaveProperty('providers');
    expect(Array.isArray(data.providers)).toBe(true);
  });

  test('AC-N8: GET /reseller/routing/routes возвращает 200 и массив routes', async ({ request }) => {
    const api = new ApiHelper(request, 'reseller-auth-state.json');
    const data = await api.listNetworkRoutes();
    expect(data).toHaveProperty('routes');
    expect(Array.isArray(data.routes)).toBe(true);
  });

});

// ─── Seed: 1 subaccount ───────────────────────────────────────────────────────

test.describe('NetworkRouting — фильтр и пустые состояния', () => {

  test('AC-N4: select субаккаунтов показывает созданный субаккаунт', async ({ page, request }) => {
    const api = new ApiHelper(request, 'reseller-auth-state.json');
    const { name } = await seedSubAccount(api, 'filter');

    const p = new NetworkRoutingPage(page);
    await p.goto();
    await p.expectLoaded();
    await p.expectSubAccountOption(name);
  });

  test('AC-N5: фильтр по субаккаунту без провайдера — пустое состояние «Провайдеры»', async ({ page, request }) => {
    const api = new ApiHelper(request, 'reseller-auth-state.json');
    const { id } = await seedSubAccount(api, 'empty-prov');

    const p = new NetworkRoutingPage(page);
    await p.goto();
    await p.expectLoaded();
    await p.selectSubAccount(id);
    await p.expectEmptyProviders();
  });

  test('AC-N6: фильтр по субаккаунту без маршрутов — пустое состояние «Маршруты»', async ({ page, request }) => {
    const api = new ApiHelper(request, 'reseller-auth-state.json');
    const { id } = await seedSubAccount(api, 'empty-routes');

    const p = new NetworkRoutingPage(page);
    await p.goto();
    await p.expectLoaded();
    await p.selectSubAccount(id);
    await p.clickTab('Маршруты');
    await p.expectEmptyRoutes();
  });

});

// ─── Seed: subaccount + provider (bulk-assign) ────────────────────────────────

test.describe('NetworkRouting — bulk-assign и таблица', () => {

  test('AC-N11: успешный bulk-assign через UI — toast и таблица обновляется', async ({ page, request }) => {
    const api = new ApiHelper(request, 'reseller-auth-state.json');
    const { name: subName } = await seedSubAccount(api, 'bulk');
    const provId = await pickProviderId(api);
    test.skip(!provId, 'нет доступного provider');

    const p = new NetworkRoutingPage(page);
    await p.goto();
    await p.expectLoaded();
    await p.openBulkAssignModal();

    await p.selectBulkProvider(provId!);
    await p.checkSubAccountInModal(subName);
    await expect(p.submitBulkButton()).not.toBeDisabled();
    await p.submitBulkButton().click();

    await expect(page.locator('text=Назначено: 1 из 1')).toBeVisible({ timeout: 8_000 });
    await expect(page.locator('text=Массовое назначение провайдера')).not.toBeVisible();

    // после закрытия modal таблица перезагрузилась — субаккаунт должен появиться
    await expect(page.locator(`td:has-text("${subName}")`).first()).toBeVisible({ timeout: 8_000 });
  });

  test('AC-N12: таблица провайдеров показывает все 4 столбца и active-dot', async ({ page, request }) => {
    const api = new ApiHelper(request, 'reseller-auth-state.json');
    const { id: subId, name: subName } = await seedSubAccount(api, 'table');
    const provId = await pickProviderId(api);
    test.skip(!provId, 'нет доступного provider');

    // seed через API напрямую, не через UI
    await api.bulkAssignProvider({ sub_account_ids: [subId], provider_id: provId! });

    const p = new NetworkRoutingPage(page);
    await p.goto();
    await p.expectLoaded();
    await p.expectProvidersTableVisible();
    await p.expectProvidersTableHasRow();
    await p.expectActiveDotVisible();
  });

});
