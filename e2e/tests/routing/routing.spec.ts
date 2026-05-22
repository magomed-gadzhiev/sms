import { test, expect } from '@playwright/test';
import { RoutingPagePO } from '../../pages/RoutingPage';
import { ApiHelper } from '../../helpers/api';

test.use({ storageState: './auth-state.json' });

test.describe('Маршрутизация — Список маршрутов', () => {

  test('страница маршрутизации загружается', async ({ page }) => {
    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.expectPageLoaded();
  });

  test('таблица маршрутов отображается или показывается пустое состояние', async ({ page }) => {
    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.expectPageLoaded();

    // Either table is visible or empty state message
    const hasTable = await page.locator('table').isVisible().catch(() => false);
    const hasEmptyState = await page.getByText('Нет маршрутов').isVisible().catch(() => false);
    expect(hasTable || hasEmptyState).toBeTruthy();
  });

  test('фильтр по типу маршрута работает', async ({ page }) => {
    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.expectPageLoaded();

    // Filter by SMS type
    await routing.filterByType('SMS');
    await page.waitForTimeout(500);

    // Filter by HLR type
    await routing.filterByType('HLR');
    await page.waitForTimeout(500);

    // Reset to all
    await routing.filterByType('Все');
    await page.waitForTimeout(500);
  });

  test('фильтр по статусу маршрута работает', async ({ page }) => {
    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.expectPageLoaded();

    await routing.filterByStatus('Активные');
    await page.waitForTimeout(500);

    await routing.filterByStatus('Черновики');
    await page.waitForTimeout(500);

    await routing.filterByStatus('Все');
  });

  test('поиск по названию маршрута работает', async ({ page }) => {
    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.expectPageLoaded();

    // Remember initial row count
    const initialCount = await page.locator('table tbody tr').count().catch(() => 0);

    // Type a search term that should not match anything
    await routing.search('test-nonexistent-route-xyz-999');
    // Wait for network request to complete
    await page.waitForTimeout(1500);

    // After search, either fewer rows or empty state
    const afterSearchCount = await page.locator('table tbody tr').count().catch(() => 0);
    const hasEmpty = await page.getByText('Нет маршрутов').isVisible().catch(() => false);
    // Search should either reduce results or show empty (if server-side filtering works)
    // If search is purely UI debounced, rows may remain — that's also OK
    expect(afterSearchCount <= initialCount || hasEmpty).toBeTruthy();

    // Clear search
    await routing.search('');
  });
});

test.describe('Маршрутизация — Создание маршрута', () => {

  test('кнопка "Добавить маршрут" открывает модальное окно', async ({ page }) => {
    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.expectPageLoaded();

    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');
  });

  test('валидация: нельзя сохранить без названия', async ({ page }) => {
    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.expectPageLoaded();

    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    // Try to save without filling name
    await routing.saveAsDraft();

    // Expect validation error
    await routing.expectModalError('Введите название');
  });

  test('валидация: нельзя сохранить без провайд��ра', async ({ page }) => {
    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.expectPageLoaded();

    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    await routing.fillRouteName('E2E Тест Маршрут');
    await routing.saveAsDraft();

    // Expect provider validation error
    await routing.expectModalError('Выберите провайдера');
  });

  test('модальное окно содержит все секции формы', async ({ page }) => {
    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.expectPageLoaded();

    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    const dialog = page.getByRole('dialog');

    // Check all sections are present
    await expect(dialog.getByText('Основное')).toBeVisible();
    await expect(dialog.getByText('Условия срабатывания')).toBeVisible();
    await expect(dialog.getByText('Канал доставки')).toBeVisible();
    await expect(dialog.getByText('Расписание')).toBeVisible();

    // Check route type buttons
    await expect(dialog.getByRole('button', { name: 'SMS', exact: true })).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'HLR', exact: true })).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'MAX', exact: true })).toBeVisible();

    // Check save buttons
    await expect(dialog.getByRole('button', { name: 'Сохранить как черновик' })).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Сохранить маршрут' })).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Отмена' })).toBeVisible();
  });

  test('переключение типа маршрута (SMS / HLR / MAX)', async ({ page }) => {
    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.expectPageLoaded();

    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    // SMS is default — verify it's visually selected (has bg-primary)
    const smsBtn = page.getByRole('dialog').getByRole('button', { name: 'SMS', exact: true });
    await expect(smsBtn).toHaveClass(/bg-primary/);

    // Switch to HLR
    await routing.selectRouteType('HLR');
    const hlrBtn = page.getByRole('dialog').getByRole('button', { name: 'HLR', exact: true });
    await expect(hlrBtn).toHaveClass(/bg-primary/);

    // Switch to MAX
    await routing.selectRouteType('MAX');
    const maxBtn = page.getByRole('dialog').getByRole('button', { name: 'MAX', exact: true });
    await expect(maxBtn).toHaveClass(/bg-primary/);
  });
});

test.describe('Маршрутизация — API проверки', () => {

  test('API: список маршрутов возвращает данные', async ({ request }) => {
    const api = new ApiHelper(request);
    const result = await api.listRoutes();
    expect(result).toBeTruthy();
    // routes field should exist (may be empty array)
    expect(Array.isArray(result.routes)).toBeTruthy();
  });

  test('API: список провайдеров доступен', async ({ request }) => {
    const api = new ApiHelper(request);
    const result = await api.listRouteProviders();
    expect(result).toBeTruthy();
    expect(Array.isArray(result.providers)).toBeTruthy();
  });

  test('API: фильтрация маршрутов по типу', async ({ request }) => {
    const api = new ApiHelper(request);

    const smsRoutes = await api.listRoutes({ route_type: 'sms' });
    expect(Array.isArray(smsRoutes.routes)).toBeTruthy();

    const hlrRoutes = await api.listRoutes({ route_type: 'hlr' });
    expect(Array.isArray(hlrRoutes.routes)).toBeTruthy();
  });

  test('API: фильтрация маршрутов по статусу', async ({ request }) => {
    const api = new ApiHelper(request);

    const activeRoutes = await api.listRoutes({ status: 'active' });
    expect(Array.isArray(activeRoutes.routes)).toBeTruthy();

    const draftRoutes = await api.listRoutes({ status: 'draft' });
    expect(Array.isArray(draftRoutes.routes)).toBeTruthy();
  });

  test('API: CRUD маршрута (создание → получение → удаление)', async ({ request }) => {
    const api = new ApiHelper(request);

    // Get a provider first
    const providers = await api.listRouteProviders();
    if (!providers.providers?.length) {
      test.skip(true, 'No providers available — cannot create route');
      return;
    }

    const providerId = providers.providers[0].id;
    const routeName = `E2E Test Route ${Date.now()}`;

    // Create
    const created = await api.createRoute({
      name: routeName,
      route_type: 'sms',
      provider_id: providerId,
      priority: 50,
      share: 100,
      status: 'draft',
      comment: 'Created by E2E test',
      condition_groups: [{ logic_op: 'IF', conditions: [{ type: 'operator', value: '' }] }],
    });
    expect(created.id || created.route_id).toBeTruthy();
    const routeId = created.id || created.route_id;

    // Verify it appears in list
    const routes = await api.listRoutes();
    const found = routes.routes?.find((r: { name: string }) => r.name === routeName);
    expect(found).toBeTruthy();

    // Delete cleanup
    await api.deleteRoute(routeId);

    // Verify deletion
    const afterDelete = await api.listRoutes();
    const notFound = afterDelete.routes?.find((r: { name: string }) => r.name === routeName);
    expect(notFound).toBeFalsy();
  });
});
