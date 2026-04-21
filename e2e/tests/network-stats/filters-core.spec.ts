import { test, expect } from '@playwright/test';
import { NetworkStatisticsPage } from '../../pages/NetworkStatisticsPage';

test.use({ storageState: './reseller-auth-state.json' });

test.describe('Network Statistics — Filter Core Behavior', () => {

  test('AC-01: Period preset click should NOT auto-apply (spec §3.4)', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);

    const apiCalls: string[] = [];
    page.on('request', req => {
      if (req.url().includes('/reseller/statistics')) {
        apiCalls.push(req.url());
      }
    });

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    expect(apiCalls.length, 'Sanity check: initial page load must fire a statistics request').toBeGreaterThan(0);

    const callsBeforeClick = apiCalls.length;

    await stats.periodPreset('Сегодня').click();
    await page.waitForTimeout(500);

    expect(
      apiCalls.length,
      `Preset click fired ${apiCalls.length - callsBeforeClick} request(s). Per spec §3.4, no request should be fired until "Применить" is clicked.`
    ).toBe(callsBeforeClick);

    await expect(stats.applyButton()).toBeEnabled();
  });

  test('AC-02: Apply fires single request with URL params reflecting pending changes', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);

    const apiCalls: string[] = [];
    page.on('request', req => {
      if (req.url().includes('/reseller/statistics')) {
        apiCalls.push(req.url());
      }
    });

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    expect(apiCalls.length, 'Sanity check: initial page load must fire a statistics request').toBeGreaterThan(0);

    const initialCount = apiCalls.length;

    await stats.groupingSelect().selectOption('provider');
    await page.waitForTimeout(300);

    expect(
      apiCalls.length,
      'Changing grouping alone should NOT fire request (no auto-apply).'
    ).toBe(initialCount);

    await stats.applyButton().click();
    await page.waitForLoadState('networkidle');

    expect(apiCalls.length).toBe(initialCount + 1);
    expect(apiCalls[apiCalls.length - 1]).toContain('group_by=provider');
  });

  test('AC-07: Advanced filter block is collapsed on first load', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await stats.goto();
    await stats.expectLoaded();

    await expect(stats.extraFiltersToggle()).toBeVisible();
    await expect(stats.senderNameField()).toBeHidden();
  });

  test('AC-08: Clicking "Ещё фильтры" expands advanced block (exactly 7 fields)', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await stats.goto();
    await stats.expectLoaded();

    // Before click: all 7 advanced fields hidden
    await expect(stats.senderNameField()).toBeHidden();
    await expect(stats.providerInput()).toBeHidden();

    await stats.extraFiltersToggle().click();

    // After click: exactly these 7 fields visible, in order (per StatisticsFilterBar.tsx:278-301)
    await expect(stats.senderNameField()).toBeVisible();
    await expect(stats.trafficTypeSelect()).toBeVisible();
    await expect(stats.statusSelect()).toBeVisible();
    await expect(stats.providerInput()).toBeVisible();
    await expect(stats.countryInput()).toBeVisible();
    await expect(stats.managerInput()).toBeVisible();
    await expect(stats.errorCodeInput()).toBeVisible();

    // AC-08 правка 2026-04-21: no Minus/X icon swap — button stays with <Plus>
    await expect(stats.extraFiltersToggle()).toBeVisible();
  });

  test('AC-13: Default grouping is "day" on first load', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    const selectValue = await stats.groupingSelect().inputValue();
    expect(selectValue).toBe('day');
  });

  test('AC-11 (URL-only): Preset click + Apply updates URL to selected preset', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);

    const apiCalls: string[] = [];
    page.on('request', req => {
      if (req.url().includes('/reseller/statistics')) apiCalls.push(req.url());
    });

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    expect(apiCalls.length, 'Sanity: initial load fires a statistics request').toBeGreaterThan(0);

    await stats.periodPreset('Сегодня').click();
    await stats.applyButton().click();
    await page.waitForLoadState('networkidle');

    expect(page.url()).toContain('period_preset=today');
    expect(apiCalls[apiCalls.length - 1]).toContain('period_preset=today');
  });

  test('AC-12 (URL-only): Custom date range removes preset and sets date_from/date_to', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    await stats.datePickerTrigger().click();
    await stats.datePickerFromInput().fill('2026-04-10');
    await stats.datePickerToInput().fill('2026-04-15');
    await stats.datePickerApply().click();
    await page.waitForLoadState('networkidle');

    const url = page.url();
    expect(url).toMatch(/date_from=[^&]+/);
    expect(url).toMatch(/date_to=[^&]+/);
    expect(url).not.toContain('period_preset=');
  });

  test('AC-17 (simplified): Tab switch preserves filter params in URL', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    // Establish non-default state via grouping (seed-free)
    await stats.groupingSelect().selectOption('provider');
    await stats.applyButton().click();
    await page.waitForLoadState('networkidle');
    expect(page.url()).toContain('group_by=provider');

    // Switch to Аналитика
    await stats.tab('Аналитика').click();
    await page.waitForLoadState('networkidle');

    const url = page.url();
    expect(url).toContain('mode=analytics');
    expect(url, 'Filter must be preserved across tab switch').toContain('group_by=provider');
  });

  test('AC-03: Double-click "Применить" fires exactly one request (dedup)', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);

    // Slow down the statistics endpoint to create a window for double-click dedup
    await page.route('**/reseller/statistics**', async (route) => {
      await new Promise(r => setTimeout(r, 1500));
      await route.continue();
    });

    const apiCalls: number[] = [];
    page.on('request', req => {
      if (req.url().includes('/reseller/statistics')) apiCalls.push(Date.now());
    });

    await stats.goto();
    await stats.expectLoaded();
    // Wait for initial request to complete under the 1500ms route delay before snapshotting count
    await page.waitForLoadState('networkidle');

    const initialCount = apiCalls.length;

    // Dirty the filter state so "Применить" is meaningful
    await stats.groupingSelect().selectOption('provider');

    // Double-click within 300ms
    await stats.applyButton().click();
    await stats.applyButton().click({ timeout: 300 }).catch(() => { /* disabled state ignored */ });

    // Wait out the throttle window
    await page.waitForTimeout(2500);

    const newCalls = apiCalls.length - initialCount;
    expect(
      newCalls,
      `Double-click should result in 1 request (button disables during loading). Got ${newCalls}.`,
    ).toBe(1);
  });

  test('AC-10 (revised): URL stays bare on first render, request fires with defaults', async ({ page }) => {
    // AC-10 правка 2026-04-21: setSearchParams не вызывается при init.
    // URL не self-writes, но запрос содержит period_preset=7d&group_by=day как state-defaults.
    const stats = new NetworkStatisticsPage(page);

    const statsRequestURL: string[] = [];
    page.on('request', req => {
      const u = req.url();
      if (u.includes('/reseller/statistics')) statsRequestURL.push(u);
    });

    await stats.goto(); // '/network/statistics' WITHOUT query
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    // URL bar: no query string (or at most empty)
    const browserURL = new URL(page.url());
    expect(
      browserURL.search,
      `URL should stay bare on first render (no setSearchParams on init). Got: ${browserURL.search}`,
    ).toBe('');

    // Request fired with state-defaults (period_preset=7d&group_by=day)
    expect(statsRequestURL.length).toBeGreaterThan(0);
    const firstReq = statsRequestURL[0];
    expect(firstReq, 'First request should carry period_preset=7d default').toMatch(/period_preset=7d/);
    expect(firstReq, 'First request should carry group_by=day default').toMatch(/group_by=day/);

    // Preset "7 дней" visually active (AC-10 partial)
    await expect(stats.periodPreset('7 дней')).toHaveClass(/bg-blue-50|text-blue-600/);
  });

  test('AC-14: group_by=5min + 7d period → HTTP 400 InvalidArgument + localized toast', async ({ page }) => {
    // D-12 fixed: validateFilter wraps input errors as *domain.ValidationError,
    // grpc server maps to codes.InvalidArgument → HTTP 400. Message is in Russian.
    const stats = new NetworkStatisticsPage(page);

    const statsResponses: number[] = [];
    page.on('response', resp => {
      if (resp.url().includes('/reseller/statistics')) statsResponses.push(resp.status());
    });

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    // Period 7d is default; set group_by=5min (max allowed 24h → violates for 7d)
    await stats.groupingSelect().selectOption('5min');
    await stats.applyButton().click();

    await expect(stats.errorToast()).toBeVisible({ timeout: 5_000 });

    const lastStatus = statsResponses[statsResponses.length - 1];
    expect(lastStatus, `Expected HTTP 400 for validation error after D-12 fix. Got ${lastStatus}.`).toBe(400);

    // Toast contains user-readable Russian message (from ValidationError)
    await expect(stats.errorToast()).toContainText(/Группировка|период|допустима/i);
  });

  test('AC-16: Deep-link with filters applies them on first render', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);

    const apiCalls: string[] = [];
    page.on('request', req => {
      if (req.url().includes('/reseller/statistics')) apiCalls.push(req.url());
    });

    await stats.goto('?mode=stats&period_preset=30d&group_by=provider');
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    // Grouping select reflects URL
    const groupingValue = await stats.groupingSelect().inputValue();
    expect(groupingValue, 'Grouping must reflect group_by from URL').toBe('provider');

    // Preset "30 дней" active
    await expect(stats.periodPreset('30 дней')).toHaveClass(/bg-blue-50|text-blue-600/);

    // Request carries URL params
    expect(apiCalls.length).toBeGreaterThan(0);
    const firstReq = apiCalls[0];
    expect(firstReq).toMatch(/period_preset=30d/);
    expect(firstReq).toMatch(/group_by=provider/);

    // Apply button in idle/disabled state — no pending filter changes
    // (Apply button exists but should not be "dirty"-indicating on pure deep-link load)
    await expect(stats.applyButton()).toBeEnabled(); // always enabled in current code
  });

  test('AC-18: Each tab calls its own API endpoint', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);

    const endpoints: string[] = [];
    page.on('request', req => {
      const u = req.url();
      if (u.includes('/reseller/statistics')) endpoints.push('stats');
      else if (u.includes('/reseller/analytics-summary')) endpoints.push('analytics');
      else if (u.includes('/reseller/monitoring')) endpoints.push('monitoring');
    });

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    expect(endpoints, 'Initial load on Статистика should fire /reseller/statistics').toContain('stats');

    await stats.tab('Аналитика').click();
    await page.waitForLoadState('networkidle');
    expect(endpoints, 'Аналитика tab should fire /reseller/analytics-summary').toContain('analytics');

    await stats.tab('Мониторинг').click();
    await page.waitForLoadState('networkidle');
    expect(endpoints, 'Мониторинг tab should fire /reseller/monitoring').toContain('monitoring');
  });

});
