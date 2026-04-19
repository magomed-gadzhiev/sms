import { test, expect } from '@playwright/test';
import { RoutingPagePO } from '../../pages/RoutingPage';
import { ApiHelper } from '../../helpers/api';

test.use({ storageState: './auth-state.json' });

const TEST_PREFIX = 'E2E B';

async function pickProviderId(api: ApiHelper): Promise<string | null> {
  const resp = await api.listRouteProviders();
  return resp.providers?.[0]?.id ?? null;
}

async function seedRoute(api: ApiHelper, providerId: string, overrides: Partial<{
  name: string;
  route_type: string;
  priority: number;
  share: number;
  status: string;
}> = {}): Promise<{ id: string; name: string }> {
  const name = overrides.name ?? `${TEST_PREFIX} Seed ${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
  const created = await api.createRoute({
    name,
    route_type: overrides.route_type ?? 'sms',
    provider_id: providerId,
    priority: overrides.priority ?? 50,
    share: overrides.share ?? 100,
    status: overrides.status ?? 'active',
    comment: 'seeded by routing-types-status.spec',
    condition_groups: [{ logic_op: 'IF', conditions: [{ type: 'operator', value: '' }] }],
  });
  const id = created.id || created.route_id;
  if (!id) throw new Error(`seedRoute failed: ${JSON.stringify(created)}`);
  return { id, name };
}

async function cleanupByPrefix(api: ApiHelper, prefix: string) {
  const resp = await api.listRoutes();
  const matches = (resp.routes ?? []).filter((r: { name: string }) => r.name.startsWith(prefix));
  for (const r of matches as Array<{ id: string }>) {
    await api.deleteRoute(r.id).catch(() => undefined);
  }
}

// Tests run serially within this file (Playwright default), so a module-level
// activePrefix is safe. Switching to parallel mode would require per-test state.
let activePrefix = '';

test.afterEach(async ({ request }) => {
  if (!activePrefix) return;
  const api = new ApiHelper(request);
  await cleanupByPrefix(api, activePrefix);
  activePrefix = '';
});

test.describe('Маршрутизация — Типы маршрутов', () => {
  test('AC-B1: создание HLR маршрута через UI', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const providerId = await pickProviderId(api);
    test.skip(!providerId, 'нет ни одного provider');

    const prefix = `${TEST_PREFIX}1 ${Date.now()}`;
    activePrefix = prefix;
    const name = `${prefix} hlr`;

    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.expectPageLoaded();
    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    await routing.fillRouteName(name);
    await routing.selectRouteType('HLR');
    await routing.selectFirstProvider();
    await routing.saveAsActive();

    await expect(page.getByRole('dialog')).toBeHidden();
    await routing.expectRouteInTable(name);

    await routing.filterByType('HLR');
    await routing.expectRouteInTable(name);

    await routing.filterByType('SMS');
    await expect(page.locator('table').getByText(name, { exact: true })).toHaveCount(0);

    await routing.filterByType('Все');
  });

  test('AC-B2: создание MAX маршрута через UI', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const providerId = await pickProviderId(api);
    test.skip(!providerId, 'нет ни одного provider');

    const prefix = `${TEST_PREFIX}2 ${Date.now()}`;
    activePrefix = prefix;
    const name = `${prefix} max`;

    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    await routing.fillRouteName(name);
    await routing.selectRouteType('MAX');
    await routing.selectFirstProvider();
    await routing.saveAsActive();

    await expect(page.getByRole('dialog')).toBeHidden();
    await routing.expectRouteInTable(name);

    await routing.filterByType('MAX');
    await routing.expectRouteInTable(name);

    await routing.filterByType('HLR');
    await expect(page.locator('table').getByText(name, { exact: true })).toHaveCount(0);

    await routing.filterByType('Все');
  });
});

test.describe('Маршрутизация — Смена статуса через edit', () => {
  test('AC-B3: промоут черновика до активного', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const providerId = await pickProviderId(api);
    test.skip(!providerId, 'нет ни одного provider');

    const prefix = `${TEST_PREFIX}3 ${Date.now()}`;
    activePrefix = prefix;
    const { name } = await seedRoute(api, providerId!, {
      name: `${prefix} promote`,
      status: 'draft',
    });

    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.expectPageLoaded();
    await routing.expectRouteRow(name, { statusLabel: 'Черновик' });

    await routing.clickEditRoute(name);
    await routing.expectModalOpen('Редактировать маршрут');
    await routing.saveAsActive();

    await expect(page.getByRole('dialog')).toBeHidden();
    await routing.expectRouteRow(name, { statusLabel: 'Активен' });

    await routing.filterByStatus('Активные');
    await routing.expectRouteInTable(name);

    await routing.filterByStatus('Черновики');
    await expect(page.locator('table').getByText(name, { exact: true })).toHaveCount(0);

    await routing.filterByStatus('Все');
  });

  test('AC-B4: откат активного в черновик', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const providerId = await pickProviderId(api);
    test.skip(!providerId, 'нет ни одного provider');

    const prefix = `${TEST_PREFIX}4 ${Date.now()}`;
    activePrefix = prefix;
    const { name } = await seedRoute(api, providerId!, {
      name: `${prefix} demote`,
      status: 'active',
    });

    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.expectPageLoaded();
    await routing.expectRouteRow(name, { statusLabel: 'Активен' });

    await routing.clickEditRoute(name);
    await routing.expectModalOpen('Редактировать маршрут');
    await routing.saveAsDraft();

    await expect(page.getByRole('dialog')).toBeHidden();
    await routing.expectRouteRow(name, { statusLabel: 'Черновик' });

    await routing.filterByStatus('Черновики');
    await routing.expectRouteInTable(name);

    await routing.filterByStatus('Активные');
    await expect(page.locator('table').getByText(name, { exact: true })).toHaveCount(0);

    await routing.filterByStatus('Все');
  });
});

test.describe('Маршрутизация — Сквозная проверка типов', () => {
  test('AC-B5: три типа сосуществуют, фильтр по типу разделяет', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const providerId = await pickProviderId(api);
    test.skip(!providerId, 'нет ни одного provider');

    const prefix = `${TEST_PREFIX}5 ${Date.now()}`;
    activePrefix = prefix;

    const { name: smsName } = await seedRoute(api, providerId!, {
      name: `${prefix} sms-row`,
      route_type: 'sms',
    });
    const { name: hlrName } = await seedRoute(api, providerId!, {
      name: `${prefix} hlr-row`,
      route_type: 'hlr',
    });
    const { name: maxName } = await seedRoute(api, providerId!, {
      name: `${prefix} max-row`,
      route_type: 'max',
    });

    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.expectPageLoaded();

    // Все три видны без фильтра
    await routing.expectRouteInTable(smsName);
    await routing.expectRouteInTable(hlrName);
    await routing.expectRouteInTable(maxName);

    // Фильтр SMS: только sms-row
    await routing.filterByType('SMS');
    await routing.expectRouteInTable(smsName);
    await expect(page.locator('table').getByText(hlrName, { exact: true })).toHaveCount(0);
    await expect(page.locator('table').getByText(maxName, { exact: true })).toHaveCount(0);

    // Фильтр HLR: только hlr-row
    await routing.filterByType('HLR');
    await routing.expectRouteInTable(hlrName);
    await expect(page.locator('table').getByText(smsName, { exact: true })).toHaveCount(0);
    await expect(page.locator('table').getByText(maxName, { exact: true })).toHaveCount(0);

    // Фильтр MAX: только max-row
    await routing.filterByType('MAX');
    await routing.expectRouteInTable(maxName);
    await expect(page.locator('table').getByText(smsName, { exact: true })).toHaveCount(0);
    await expect(page.locator('table').getByText(hlrName, { exact: true })).toHaveCount(0);

    await routing.filterByType('Все');
    await routing.expectRouteInTable(smsName);
    await routing.expectRouteInTable(hlrName);
    await routing.expectRouteInTable(maxName);
  });
});
