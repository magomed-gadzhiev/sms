import { test, expect, type Route } from '@playwright/test';
import { NetworkStatisticsPage } from '../../pages/NetworkStatisticsPage';

test.use({ storageState: './reseller-auth-state.json' });

// Phase 1d — structural AC for Drill-down Drawer using mocked API responses.
// Real drill-down behavior with seeded data is Phase 2.
// Strategy: mock /reseller/statistics with 1 row so table has something to click,
// mock /reseller/drilldown with synthetic drill-down response.

const MOCK_STATS_RESPONSE = {
  kpis: [],
  rows: [
    {
      slice: 'mts',
      total: 1000, sent: 1000, delivered: 950, failed: 30, pending: 10, timeout: 10,
      error: 15, dlr_rate: 0.95, revenue: 5000, cost: 3000, profit: 2000, margin: 0.4,
      health: 'ok', alerts: {},
    },
  ],
  pagination: { page: 1, page_size: 20, total_rows: 1, total_pages: 1 },
};

const MOCK_DRILLDOWN_RESPONSE = {
  summary: [
    { name: 'Всего', value: 1000, delta: 0, status: 'ok' },
    { name: 'DLR%', value: 0.95, delta: 0, status: 'ok' },
    { name: 'Выручка', value: 5000, delta: 0, status: 'ok' },
  ],
  rows: [
    {
      slice: 'beeline',
      total: 500, sent: 500, delivered: 480, failed: 15, pending: 5, timeout: 0,
      error: 5, dlr_rate: 0.96, revenue: 2500, cost: 1500, profit: 1000, margin: 0.4,
      health: 'ok', alerts: {},
    },
  ],
  trends: [],
  health: 'ok',
};

const MOCK_DRILLDOWN_EMPTY = {
  summary: [],
  rows: [],
  trends: [],
  health: 'ok',
};

async function mockStatsRow(page: import('@playwright/test').Page) {
  await page.route('**/reseller/statistics**', async (route: Route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(MOCK_STATS_RESPONSE),
    });
  });
}

test.describe('Network Statistics — Drill-down Drawer (mocked)', () => {

  test('AC-54: Drawer opens with overlay on row click, positioned right', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await mockStatsRow(page);
    await page.route('**/reseller/drilldown**', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_DRILLDOWN_RESPONSE) });
    });

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    // Drawer hidden initially
    await expect(stats.drawer()).toHaveCount(0);

    // Click the single row
    await stats.tableRows().first().click();
    await page.waitForLoadState('networkidle');

    // Drawer visible, overlay visible
    await expect(stats.drawer()).toBeVisible();
    await expect(stats.drawerOverlay()).toBeVisible();
  });

  test('AC-55: X button closes drawer', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await mockStatsRow(page);
    await page.route('**/reseller/drilldown**', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_DRILLDOWN_RESPONSE) });
    });

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');
    await stats.tableRows().first().click();
    await page.waitForLoadState('networkidle');
    await expect(stats.drawer()).toBeVisible();

    // Close via X
    await stats.drawerCloseButton().click();

    await expect(stats.drawer()).toHaveCount(0);
    await expect(stats.drawerOverlay()).toHaveCount(0);
  });

  test('AC-55 (variant): Overlay click closes drawer', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await mockStatsRow(page);
    await page.route('**/reseller/drilldown**', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_DRILLDOWN_RESPONSE) });
    });

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');
    await stats.tableRows().first().click();
    await page.waitForLoadState('networkidle');
    await expect(stats.drawer()).toBeVisible();

    await stats.drawerOverlay().click();

    await expect(stats.drawer()).toHaveCount(0);
  });

  test('AC-55 (variant): "Статистика" breadcrumb closes drawer', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await mockStatsRow(page);
    await page.route('**/reseller/drilldown**', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_DRILLDOWN_RESPONSE) });
    });

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');
    await stats.tableRows().first().click();
    await page.waitForLoadState('networkidle');
    await expect(stats.drawer()).toBeVisible();

    await stats.drawerBreadcrumbRoot().click();

    await expect(stats.drawer()).toHaveCount(0);
  });

  test('AC-60: Drawer has exactly 5 tabs with Russian labels', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await mockStatsRow(page);
    await page.route('**/reseller/drilldown**', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_DRILLDOWN_RESPONSE) });
    });

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');
    await stats.tableRows().first().click();
    await page.waitForLoadState('networkidle');

    await expect(stats.drawerTabs()).toHaveCount(5);

    const expectedLabels = ['По операторам', 'По статусам', 'По ошибкам', 'Динамика', 'Деньги'] as const;
    for (const label of expectedLabels) {
      await expect(stats.drawerTab(label)).toBeVisible();
    }

    // Default active tab is "По операторам"
    await expect(stats.drawerTab('По операторам')).toHaveAttribute('data-state', 'active');
  });

  test('AC-62: Drawer-table header has exactly 5 columns', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await mockStatsRow(page);
    await page.route('**/reseller/drilldown**', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_DRILLDOWN_RESPONSE) });
    });

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');
    await stats.tableRows().first().click();
    await page.waitForLoadState('networkidle');

    const headers = await stats.drawerTableHeaders().allTextContents();
    const trimmed = headers.map(h => h.trim());
    expect(trimmed).toEqual(['Срез', 'Всего', 'Достав.', 'Ошибки', 'DLR%']);
  });

  test('AC-64: Loading state "Загрузка..." appears during slow drill-down response', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await mockStatsRow(page);
    await page.route('**/reseller/drilldown**', async (route) => {
      // Delay long enough to reliably observe loading state under CI load
      await new Promise(r => setTimeout(r, 2500));
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_DRILLDOWN_RESPONSE) });
    });

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');
    await stats.tableRows().first().click();

    // Before drill-down response arrives: drawer open, content shows "Загрузка..."
    await expect(stats.drawerLoadingCell()).toBeVisible({ timeout: 2_000 });

    await page.waitForLoadState('networkidle');

    // After: loading gone, table visible
    await expect(stats.drawerLoadingCell()).toBeHidden();
    await expect(stats.drawerTable()).toBeVisible();
  });

  test('AC-65+AC-66: Empty drill-down shows "Нет данных" and hides hint banner', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await mockStatsRow(page);
    await page.route('**/reseller/drilldown**', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_DRILLDOWN_EMPTY) });
    });

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');
    await stats.tableRows().first().click();
    await page.waitForLoadState('networkidle');

    // "Нет данных" cell visible, table not rendered
    await expect(stats.drawerEmptyCell()).toBeVisible();
    await expect(stats.drawerTable()).toHaveCount(0);

    // Hint banner hidden (condition: data?.rows && data.rows.length > 0)
    await expect(stats.drawerHintBanner()).toHaveCount(0);
  });

});
