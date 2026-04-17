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

  test('AC-08: Clicking "Ещё фильтры" expands advanced block', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await stats.goto();
    await stats.expectLoaded();

    await expect(stats.senderNameField()).toBeHidden();

    await stats.extraFiltersToggle().click();

    await expect(stats.senderNameField()).toBeVisible();
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
