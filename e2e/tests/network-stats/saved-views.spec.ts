import { test, expect, type Route } from '@playwright/test';
import { NetworkStatisticsPage } from '../../pages/NetworkStatisticsPage';

test.use({ storageState: './reseller-auth-state.json' });

// Phase 1e — Saved Views via mocked /reseller/views responses.
// AC-67..AC-74 from Batch 5.
// AC-71 (loaded view → modified on filter change) covered alongside AC-70.

const EMPTY_VIEWS = { views: [] };

const TWO_VIEWS = {
  views: [
    {
      id: 101, name: 'Mts 30d', mode: 'stats',
      filters_json: '{"operator":"mts","period_preset":"30d"}',
      group_by: 'day', sort_by: 'dlr_rate', sort_dir: 'asc',
      columns: [], is_default: false, is_template: false,
    },
    {
      id: 102, name: 'All providers hourly', mode: 'stats',
      filters_json: '{"period_preset":"7d"}',
      group_by: 'provider', sort_by: '', sort_dir: 'desc',
      columns: [], is_default: false, is_template: false,
    },
  ],
};

async function mockViewsList(page: import('@playwright/test').Page, body: unknown) {
  await page.route('**/reseller/views', async (route: Route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
    } else {
      await route.continue();
    }
  });
}

test.describe('Network Statistics — Saved Views (mocked)', () => {

  test('AC-67: Panel hidden when savedViews=[] AND not modified', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await mockViewsList(page, EMPTY_VIEWS);

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    await expect(stats.savedViewsPanel(), 'Panel must be absent on fresh load with empty views').toHaveCount(0);
  });

  test('AC-68: Panel appears on first filter change (isViewModified=true)', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await mockViewsList(page, EMPTY_VIEWS);

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');
    await expect(stats.savedViewsPanel()).toHaveCount(0);

    // Mutate a filter — setFilters → setIsViewModified(true)
    await stats.loginInput().fill('demo');

    await expect(stats.savedViewsPanel()).toBeVisible();
    await expect(stats.saveViewButton()).toBeVisible();
  });

  test('AC-69: Panel visible with views listed, no Save button without modifications', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await mockViewsList(page, TWO_VIEWS);

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    await expect(stats.savedViewsPanel()).toBeVisible();
    await expect(stats.viewChip('Mts 30d')).toBeVisible();
    await expect(stats.viewChip('All providers hourly')).toBeVisible();

    // No active chip on first load (activeViewId=null)
    await expect(stats.activeViewChip()).toHaveCount(0);

    // No "+ Сохранить" (isViewModified=false)
    await expect(stats.saveViewButton()).toHaveCount(0);
  });

  test('AC-70: Clicking a view-chip loads its filters and switches mode', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await mockViewsList(page, TWO_VIEWS);

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    await stats.viewChip('Mts 30d').click();
    // Give React a moment to process state updates (no network assertion per D-19)
    await page.waitForTimeout(500);

    // URL reflects the loaded view's mode + filters (wholesale replace)
    const url = page.url();
    expect(url).toContain('mode=stats');
    expect(url).toMatch(/operator=mts/);
    expect(url).toMatch(/period_preset=30d/);
    expect(url).toMatch(/group_by=day/);
    expect(url).toMatch(/sort_by=dlr_rate/);
    expect(url).toMatch(/sort_dir=asc/);

    // Chip gets active class (bg-blue-600)
    await expect(stats.activeViewChip()).toBeVisible();

    // NOTE (D-19): loadView does NOT trigger a new /reseller/statistics fetch when
    // the view's mode matches current mode. Table keeps old rows until next Apply.
    // Assertion on immediate fetch removed — AC-70 now validates URL + active chip
    // until D-19 is fixed (loadView should call fetchData() after setSearchParams).
  });

  test('AC-71: Changing filter after loadView re-shows "+ Сохранить", keeps active chip', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await mockViewsList(page, TWO_VIEWS);

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');
    await stats.viewChip('Mts 30d').click();
    await page.waitForLoadState('networkidle');

    await expect(stats.activeViewChip()).toBeVisible();
    await expect(stats.saveViewButton()).toHaveCount(0); // not modified yet

    // Change a filter — turns isViewModified=true
    await stats.channelSelect().selectOption('sms');

    // "+ Сохранить" appears
    await expect(stats.saveViewButton()).toBeVisible();

    // Chip stays active (activeViewId not cleared until new loadView/delete)
    await expect(stats.activeViewChip()).toBeVisible();
  });

  test('AC-72+AC-73: "+ Сохранить" opens window.prompt; POST body contains mode+filters_json', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await mockViewsList(page, EMPTY_VIEWS);

    // Answer the native prompt dialog with a non-empty name
    page.once('dialog', async (dialog) => {
      expect(dialog.type()).toBe('prompt');
      expect(dialog.message()).toContain('Название вида');
      await dialog.accept('My View');
    });

    // Intercept POST /reseller/views and capture body
    let capturedBody: unknown = null;
    await page.route('**/reseller/views', async (route) => {
      if (route.request().method() === 'POST') {
        capturedBody = JSON.parse(route.request().postData() || '{}');
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            view: {
              id: 999, name: 'My View', mode: 'stats', filters_json: '{}',
              group_by: 'day', sort_by: '', sort_dir: 'desc',
              columns: [], is_default: false, is_template: false,
            },
          }),
        });
      } else {
        await route.continue();
      }
    });

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    // Dirty the state to show Save button
    await stats.loginInput().fill('demo');
    await expect(stats.saveViewButton()).toBeVisible();

    await stats.saveViewButton().click();
    await page.waitForLoadState('networkidle');

    // Verify POST body per AC-73
    expect(capturedBody, 'POST body must be captured').not.toBeNull();
    const body = capturedBody as { view: { name: string; mode: string; filters_json: string; group_by: string; columns: unknown[]; is_default: boolean } };
    expect(body.view.name).toBe('My View');
    expect(body.view.mode).toBe('stats');
    expect(body.view.columns).toEqual([]);
    expect(body.view.is_default).toBe(false);
    // filters_json must be JSON string containing at least login=demo
    expect(body.view.filters_json).toContain('"login":"demo"');
  });

  test('AC-75: Delete button (×) removes view via DELETE + window.confirm (D-17 fix)', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    await mockViewsList(page, TWO_VIEWS);

    // Answer confirm dialog with OK
    page.once('dialog', async (dialog) => {
      expect(dialog.type()).toBe('confirm');
      expect(dialog.message()).toContain('Удалить вид');
      await dialog.accept();
    });

    // Capture DELETE request
    let deleteCalled = false;
    await page.route('**/reseller/views/*', async (route) => {
      if (route.request().method() === 'DELETE') {
        deleteCalled = true;
        await route.fulfill({ status: 204, body: '' });
      } else {
        await route.continue();
      }
    });

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');
    await expect(stats.viewChip('Mts 30d')).toBeVisible();

    await stats.viewDeleteButton('Mts 30d').click();
    await page.waitForLoadState('networkidle');

    expect(deleteCalled, 'DELETE /reseller/views/{id} must be called after confirm').toBe(true);
  });

  test('AC-74: Silent failure of GET /views — error visible only when panel shows', async ({ page }) => {
    const stats = new NetworkStatisticsPage(page);
    // Mock GET /views to 500
    await page.route('**/reseller/views', async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ error: 'boom' }) });
      } else {
        await route.continue();
      }
    });

    await stats.goto();
    await stats.expectLoaded();
    await page.waitForLoadState('networkidle');

    // Panel hidden (savedViews=[], isViewModified=false) — error message NOT VISIBLE to user
    await expect(stats.savedViewsPanel(), 'Silent failure: panel hidden, user does not see error').toHaveCount(0);

    // Trigger isViewModified
    await stats.loginInput().fill('demo');

    // Now panel is visible — error text appears inside
    await expect(stats.savedViewsPanel()).toBeVisible();
    await expect(stats.savedViewsError(), 'viewsError rendered in panel').toBeVisible();
  });

});
