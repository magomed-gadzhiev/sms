import { test, expect } from '@playwright/test';
import { NetworkStatisticsPage } from '../../pages/NetworkStatisticsPage';

test.use({ storageState: './reseller-auth-state.json' });

// Phase 1b — seed-free delta of Batch 2 (Statistics Table).
// Seed-dependent AC (AC-20 ISO slice, AC-22/23 DLR color, AC-24 health row bg,
// AC-25 profit color, AC-26/27 pending/error thresholds, AC-30 row click,
// AC-31 totals, AC-33 divide-by-zero, AC-37 "Показано X–Y из N", AC-39 page reset)
// require seed data and are deferred to Phase 2.

test.describe('Network Statistics — Table (seed-free)', () => {

  test('AC-19: Table header contains exactly 12 columns', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    const headers = await stats.tableHeaders().allTextContents();
    // Drift note (AC-19): spec claims 13, code has 12. Last <th> has no text (health column).
    expect(headers, `Expected 12 columns per StatisticsTable.tsx:38-51. Got: ${JSON.stringify(headers)}`).toHaveLength(12);

    // Pin exact order of columns 1-11 (the 12th is empty-text health column).
    // Header cells wrap their text in a span with ChevronsUpDown icon, so raw textContent
    // may include whitespace — trim each before compare.
    const trimmedFirst11 = headers.slice(0, 11).map(h => h.trim());
    expect(trimmedFirst11).toEqual([
      'Срез', 'Всего', 'Достав.', 'Не достав.',
      'Ожидание', 'Таймаут', 'Ошибки', 'DLR%',
      'Выручка', 'Прибыль', 'Маржа',
    ]);
    // 12th is empty-text health column
    expect(headers[11].trim()).toBe('');
  });

  test('AC-28: Clicking numeric column header fires sorted request immediately', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    const apiCalls: string[] = [];
    page.on('request', req => {
      if (req.url().includes('/reseller/statistics')) apiCalls.push(req.url());
    });

    const initialCount = apiCalls.length;

    // First click: sort_by=dlr_rate, direction defaults to desc.
    const firstSortReq = page.waitForRequest(req =>
      req.url().includes('/reseller/statistics') && req.url().includes('sort_by=dlr_rate')
    );
    await stats.tableHeader('DLR%').click();
    const firstReq = await firstSortReq;
    expect(firstReq.url()).toMatch(/sort_dir=desc/);

    const newCalls = apiCalls.length - initialCount;
    expect(newCalls, 'Sort click should fire exactly one request').toBe(1);

    // Give React a tick to commit the updated `filters` prop before the next click,
    // so handleSort's closure sees sort_dir='desc' and computes 'asc'.
    // (handleSort reads `filters` from props, not from a ref.)
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(300);

    // Second click on same header: dir flips to asc
    const secondSortReq = page.waitForRequest(req =>
      req.url().includes('/reseller/statistics') && req.url().includes('sort_dir=asc')
    );
    await stats.tableHeader('DLR%').click();
    const secondReq = await secondSortReq;
    expect(secondReq.url()).toMatch(/sort_by=dlr_rate/);
  });

  test('AC-29: Clicking health column header does NOT fire request', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    const apiCalls: string[] = [];
    page.on('request', req => {
      if (req.url().includes('/reseller/statistics')) apiCalls.push(req.url());
    });

    // Click the last (health) th — per StatisticsTable.tsx:90 `col.key !== 'health'`, no handler
    await stats.healthHeader().click({ force: true });
    await page.waitForTimeout(500); // give spurious request a chance to appear

    expect(apiCalls, 'Health column click must not fire a request').toHaveLength(0);
    expect(page.url(), 'URL must not gain sort_by parameter after health-col click').not.toMatch(/sort_by=/);
  });

  test('AC-34: Loading cell "Загрузка..." appears during first request on empty table', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);

    // Delay statistics response so we can observe the loading state
    await page.route('**/reseller/statistics**', async (route) => {
      await new Promise(r => setTimeout(r, 1500));
      await route.continue();
    });

    await stats.goto();
    await stats.expectLoaded();

    // Before response arrives, tbody shows "Загрузка..."
    await expect(stats.tableLoadingCell(), 'Loading cell must appear while rows=[] && loading').toBeVisible({ timeout: 1_000 });

    await page.waitForLoadState('networkidle');

    // After response, loading cell disappears
    await expect(stats.tableLoadingCell()).toBeHidden();
  });

  test('AC-36: Empty state shows "Нет данных" when filter yields zero rows', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    // Apply a filter guaranteed to return zero rows (non-existent login).
    // Deep-link style: go directly with the filter to avoid needing "Ещё фильтры" toggle.
    await stats.goto('?mode=stats&period_preset=7d&group_by=day&login=__nonexistent_user_xxx_test__');
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    // Empty-state row must be visible
    await expect(stats.tableEmptyCell(), 'Empty filter should produce "Нет данных" cell').toBeVisible({ timeout: 5_000 });

    // "Итого" footer must not render for empty rows
    const foot = stats.statisticsTable().locator('tfoot');
    await expect(foot).toHaveCount(0);
  });

});
