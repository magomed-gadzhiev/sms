import { test, expect } from '@playwright/test';
import { RoutingPagePO } from '../../pages/RoutingPage';
import { ApiHelper } from '../../helpers/api';
import { pickProviderId, seedRoute, cleanupRoutesByPrefix } from '../../helpers/routing';

test.use({ storageState: './auth-state.json' });

const TEST_PREFIX = 'E2E AC';

// Each test registers its unique prefix; afterEach cleans up orphans even on failure.
// Assumes tests run serially within this file (Playwright default). If switched to
// `.configure({ mode: 'parallel' })`, replace with per-test state (e.g. test.info()).
let activePrefix = '';

test.afterEach(async ({ request }) => {
  if (!activePrefix) return;
  const api = new ApiHelper(request);
  await cleanupRoutesByPrefix(api, activePrefix);
  activePrefix = '';
});

test.describe('Маршрутизация — Создание маршрута (user flow)', () => {
  test('AC-1: создание активного маршрута через UI — happy path', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const providerId = await pickProviderId(api);
    test.skip(!providerId, 'нет ни одного provider — создание маршрута невозможно');

    const routing = new RoutingPagePO(page);
    const prefix = `${TEST_PREFIX}1 ${Date.now()}`;
    activePrefix = prefix;
    const name = `${prefix} happy`;

    await routing.goto();
    await routing.expectPageLoaded();
    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    await routing.fillRouteName(name);
    await routing.selectRouteType('SMS');
    await routing.fillPriority(50);
    await routing.fillShare(100);
    await routing.selectFirstProvider();
    await routing.saveAsActive();

    await expect(page.getByRole('dialog')).toBeHidden();
    await routing.expectRouteInTable(name);
    await routing.expectRouteRow(name, { priority: 50, share: 100, statusLabel: 'Активен' });
  });

  test('AC-2: сохранение как черновик', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const providerId = await pickProviderId(api);
    test.skip(!providerId, 'нет ни одного provider');

    const routing = new RoutingPagePO(page);
    const prefix = `${TEST_PREFIX}2 ${Date.now()}`;
    activePrefix = prefix;
    const name = `${prefix} draft`;

    await routing.goto();
    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');
    await routing.fillRouteName(name);
    await routing.selectFirstProvider();
    await routing.saveAsDraft();

    await expect(page.getByRole('dialog')).toBeHidden();
    await routing.expectRouteInTable(name);
    await routing.expectRouteRow(name, { statusLabel: 'Черновик' });

    await routing.filterByStatus('Черновики');
    await routing.expectRouteInTable(name);
    await routing.filterByStatus('Все');
  });

  test('AC-5: отмена создания — маршрут не появляется', async ({ page, request }) => {
    const api = new ApiHelper(request);

    const routing = new RoutingPagePO(page);
    const prefix = `${TEST_PREFIX}5 ${Date.now()}`;
    activePrefix = prefix;
    const name = `${prefix} cancelled`;

    await routing.goto();
    await routing.expectPageLoaded();

    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');
    await routing.fillRouteName(name);
    await page.getByRole('dialog').getByRole('button', { name: 'Отмена' }).click();

    await expect(page.getByRole('dialog')).toBeHidden();

    const afterList = await api.listRoutes();
    const found = (afterList.routes ?? []).find((r: { name: string }) => r.name === name);
    expect(found).toBeFalsy();
  });
});

test.describe('Маршрутизация — Редактирование', () => {
  test('AC-6: редактирование имени и приоритета', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const providerId = await pickProviderId(api);
    test.skip(!providerId, 'нет ни одного provider');

    const prefix = `${TEST_PREFIX}6 ${Date.now()}`;
    activePrefix = prefix;
    const { name: originalName } = await seedRoute(api, providerId!, {
      name: `${prefix} original`,
      priority: 50,
    });
    const updatedName = `${prefix} updated`;

    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.expectPageLoaded();
    await routing.expectRouteInTable(originalName);

    await routing.clickEditRoute(originalName);
    await routing.expectModalOpen('Редактировать маршрут');

    const dialog = page.getByRole('dialog');
    await expect(dialog.locator('input').first()).toHaveValue(originalName);

    await routing.fillRouteName(updatedName);
    await routing.fillPriority(75);
    await routing.saveAsActive();

    await expect(dialog).toBeHidden();
    await routing.expectRouteInTable(updatedName);
    await routing.expectRouteRow(updatedName, { priority: 75 });
    await expect(page.locator('table').getByText(originalName, { exact: true })).toHaveCount(0);
  });

  test('AC-7: смена типа маршрута SMS → HLR', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const providerId = await pickProviderId(api);
    test.skip(!providerId, 'нет ни одного provider');

    const prefix = `${TEST_PREFIX}7 ${Date.now()}`;
    activePrefix = prefix;
    const { name } = await seedRoute(api, providerId!, {
      name: `${prefix} typeswitch`,
      route_type: 'sms',
    });

    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.expectPageLoaded();
    await routing.clickEditRoute(name);
    await routing.expectModalOpen('Редактировать маршрут');
    await routing.expectRouteTypeActive('SMS');

    await routing.selectRouteType('HLR');
    await routing.expectRouteTypeActive('HLR');
    await routing.saveAsActive();

    await expect(page.getByRole('dialog')).toBeHidden();

    await routing.filterByType('HLR');
    await routing.expectRouteInTable(name);

    await routing.filterByType('SMS');
    await expect(page.locator('table').getByText(name, { exact: true })).toHaveCount(0);

    await routing.filterByType('Все');
  });
});

test.describe('Маршрутизация — Удаление', () => {
  test('AC-8: удаление с подтверждением', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const providerId = await pickProviderId(api);
    test.skip(!providerId, 'нет ни одного provider');

    const prefix = `${TEST_PREFIX}8 ${Date.now()}`;
    activePrefix = prefix;
    const { name } = await seedRoute(api, providerId!, { name: `${prefix} delete` });

    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.expectPageLoaded();
    await routing.expectRouteInTable(name);

    await routing.clickDeleteRoute(name);
    await routing.expectConfirmDialogFor(name);
    await routing.confirmDelete();

    await expect(page.getByRole('dialog')).toBeHidden();
    await expect(page.locator('table').getByText(name, { exact: true })).toHaveCount(0);

    const afterList = await api.listRoutes();
    const found = (afterList.routes ?? []).find((r: { name: string }) => r.name === name);
    expect(found).toBeFalsy();
  });

  test('AC-9: отмена удаления оставляет маршрут на месте', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const providerId = await pickProviderId(api);
    test.skip(!providerId, 'нет ни одного provider');

    const prefix = `${TEST_PREFIX}9 ${Date.now()}`;
    activePrefix = prefix;
    const { name } = await seedRoute(api, providerId!, { name: `${prefix} keep` });

    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.expectPageLoaded();
    await routing.expectRouteInTable(name);

    await routing.clickDeleteRoute(name);
    await routing.expectConfirmDialogFor(name);
    await routing.cancelDelete();

    await expect(page.getByRole('dialog')).toBeHidden();
    await routing.expectRouteInTable(name);
  });
});
