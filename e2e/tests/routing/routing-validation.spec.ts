import { test, expect } from '@playwright/test';
import { RoutingPagePO } from '../../pages/RoutingPage';
import { ApiHelper } from '../../helpers/api';

test.use({ storageState: './auth-state.json' });

const TEST_PREFIX = 'E2E D';

async function pickProviderId(api: ApiHelper): Promise<string | null> {
  const resp = await api.listRouteProviders();
  return resp.providers?.[0]?.id ?? null;
}

async function seedRoute(api: ApiHelper, providerId: string, name: string): Promise<{ id: string; name: string }> {
  const created = await api.createRoute({
    name,
    route_type: 'sms',
    provider_id: providerId,
    priority: 50,
    share: 100,
    status: 'draft',
    comment: 'seeded by routing-validation.spec',
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

let activePrefix = '';

test.afterEach(async ({ request }) => {
  if (!activePrefix) return;
  const api = new ApiHelper(request);
  await cleanupByPrefix(api, activePrefix);
  activePrefix = '';
});

test.describe('Маршрутизация — Валидация и edge-cases', () => {
  test('AC-D1: priority поле имеет min="0" и не сохраняет отрицательное значение', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const providerId = await pickProviderId(api);
    test.skip(!providerId, 'нет ни одного provider');

    const prefix = `${TEST_PREFIX}1 ${Date.now()}`;
    activePrefix = prefix;
    const name = `${prefix} neg-priority`;

    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    const priorityInput = page
      .getByRole('dialog')
      .locator('label:has-text("Приоритет")')
      .locator('..')
      .locator('input');
    await expect(priorityInput).toHaveAttribute('min', '0');

    await routing.fillRouteName(name);
    await priorityInput.fill('-5');
    await routing.selectFirstProvider();
    await routing.saveAsDraft();

    // Либо UI отклонил save (dialog остался), либо backend принял/нормализовал —
    // фиксируем факт: если строка появилась, её priority должен быть >= 0.
    const dialogStillOpen = await page.getByRole('dialog').isVisible().catch(() => false);
    if (dialogStillOpen) {
      // UI отклонил — OK
      return;
    }

    const list = await api.listRoutes();
    const created = (list.routes ?? []).find((r: { name: string }) => r.name === name);
    expect(created).toBeTruthy();
    expect(created.priority).toBeGreaterThanOrEqual(0);
  });

  test('AC-D2: share поле имеет max="100" — защита от переполнения', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const providerId = await pickProviderId(api);
    test.skip(!providerId, 'нет ни одного provider');

    const prefix = `${TEST_PREFIX}2 ${Date.now()}`;
    activePrefix = prefix;
    const name = `${prefix} overshare`;

    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    const shareInput = page
      .getByRole('dialog')
      .locator('label:has-text("Доля")')
      .locator('..')
      .locator('input');
    await expect(shareInput).toHaveAttribute('max', '100');
    await expect(shareInput).toHaveAttribute('min', '0');

    await routing.fillRouteName(name);
    await shareInput.fill('150');
    await routing.selectFirstProvider();
    await routing.saveAsDraft();

    const dialogStillOpen = await page.getByRole('dialog').isVisible().catch(() => false);
    if (dialogStillOpen) {
      return;
    }

    const list = await api.listRoutes();
    const created = (list.routes ?? []).find((r: { name: string }) => r.name === name);
    expect(created).toBeTruthy();
    // Drift-ловушка: если backend принял share>100 — тихий bug, тест упадёт.
    expect(created.share).toBeLessThanOrEqual(100);
  });

  test('AC-D3: regex-условие с пустым значением — либо ошибка, либо пустая строка persist', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const providerId = await pickProviderId(api);
    test.skip(!providerId, 'нет ни одного provider');

    const prefix = `${TEST_PREFIX}3 ${Date.now()}`;
    activePrefix = prefix;
    const name = `${prefix} empty-regex`;

    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    await routing.fillRouteName(name);
    await routing.selectFirstProvider();

    const section = page
      .getByRole('dialog')
      .locator('section', { has: page.getByRole('heading', { name: 'Условия срабатывания' }) });
    const conditionRow = section
      .locator('div.border.rounded-lg.p-4.bg-gray-50 >> div.flex.items-center.gap-2')
      .first();
    await conditionRow.getByRole('combobox').first().selectOption({ label: 'Regex' });
    // value оставляем пустым

    await routing.saveAsDraft();

    const dialogStillOpen = await page.getByRole('dialog').isVisible().catch(() => false);
    if (dialogStillOpen) {
      // Ожидаем видимую ошибку внутри модалки
      const errorVisible = await page
        .getByRole('dialog')
        .locator('.text-red-600')
        .isVisible()
        .catch(() => false);
      expect(errorVisible).toBeTruthy();
      return;
    }

    // Если сохранилось — должно быть в списке с пустой value
    const list = await api.listRoutes();
    const created = (list.routes ?? []).find((r: { name: string }) => r.name === name);
    expect(created).toBeTruthy();
    const full = await api.getRoute(created.id);
    const cond = full.condition_groups?.[0]?.conditions?.[0];
    expect(cond?.type).toBe('regex');
    expect(cond?.value).toBe('');
  });

  test('AC-D4: длинное имя (300 символов) — либо принимается, либо ошибка (не crash)', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const providerId = await pickProviderId(api);
    test.skip(!providerId, 'нет ни одного provider');

    const prefix = `${TEST_PREFIX}4 ${Date.now()}`;
    activePrefix = prefix;
    const longSuffix = 'x'.repeat(300 - prefix.length - 1);
    const name = `${prefix} ${longSuffix}`.slice(0, 300);
    expect(name.length).toBe(300);

    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    await routing.fillRouteName(name);
    await routing.selectFirstProvider();
    await routing.saveAsDraft();

    const dialogStillOpen = await page.getByRole('dialog').isVisible().catch(() => false);
    if (dialogStillOpen) {
      const errorVisible = await page
        .getByRole('dialog')
        .locator('.text-red-600')
        .isVisible()
        .catch(() => false);
      expect(errorVisible).toBeTruthy();
      return;
    }

    // Приняли — строка должна быть в таблице (возможно с усечением)
    const list = await api.listRoutes();
    const created = (list.routes ?? []).find((r: { name: string }) => r.name.startsWith(prefix));
    expect(created).toBeTruthy();
  });

  test('AC-D5: дубликат имени через UI — зафиксировать поведение', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const providerId = await pickProviderId(api);
    test.skip(!providerId, 'нет ни одного provider');

    const prefix = `${TEST_PREFIX}5 ${Date.now()}`;
    activePrefix = prefix;
    const name = `${prefix} duplicate`;

    // Seed первый маршрут через API
    await seedRoute(api, providerId!, name);

    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.expectRouteInTable(name);

    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');
    await routing.fillRouteName(name);
    await routing.selectFirstProvider();
    await routing.saveAsDraft();

    const dialogStillOpen = await page.getByRole('dialog').isVisible().catch(() => false);
    if (dialogStillOpen) {
      // Ожидаемый сценарий: backend отклонил, модалка показывает ошибку
      const errorVisible = await page
        .getByRole('dialog')
        .locator('.text-red-600')
        .isVisible()
        .catch(() => false);
      expect(errorVisible).toBeTruthy();
      return;
    }

    // Drift-ловушка: если backend допускает дубликаты — в списке ровно 2 строки.
    // Тест упадёт, если 1 (тихий дроп) или 3+.
    const list = await api.listRoutes();
    const dupes = (list.routes ?? []).filter((r: { name: string }) => r.name === name);
    expect(dupes.length).toBe(2);
  });

  test('AC-D7: saveAsActive без provider — ошибка "Выберите провайдера"', async ({ page }) => {
    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    await routing.fillRouteName(`${TEST_PREFIX}7 active-no-provider`);
    await routing.saveAsActive();

    await routing.expectModalError('Выберите провайдера');
    await expect(page.getByRole('dialog')).toBeVisible();
  });
});
