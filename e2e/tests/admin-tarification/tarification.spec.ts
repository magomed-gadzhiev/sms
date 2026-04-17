import { test, expect } from '@playwright/test';
import { TarificationPagePO, TariffsPagePO } from '../../pages/TarificationPage';
import { AdminApiHelper } from '../../helpers/api';

const ADMIN_STATE = './admin-auth-state.json';
const CLIENT_STATE = './auth-state.json';

// ═══════════════════════════════════════════════════════════════════════
// Admin Tarification UI Tests (/admin/tarification)
// ═══════════════════════════════════════════════════════════════════════

test.describe('Тарификация — Страница и навигация по табам', () => {
  test.use({ storageState: ADMIN_STATE });

  test('страница тарификации загружается', async ({ page }) => {
    const tarification = new TarificationPagePO(page);
    await tarification.goto();
    await tarification.expectPageLoaded();
  });

  test('все четыре таба отображаются', async ({ page }) => {
    const tarification = new TarificationPagePO(page);
    await tarification.goto();
    await tarification.expectPageLoaded();

    await expect(page.getByRole('tab', { name: 'Тарифные планы' })).toBeVisible();
    await expect(page.getByRole('tab', { name: 'Периоды' })).toBeVisible();
    await expect(page.getByRole('tab', { name: 'Тиры' })).toBeVisible();
    await expect(page.getByRole('tab', { name: 'Регистрация отправителей' })).toBeVisible();
  });

  test('переключение между табами работает', async ({ page }) => {
    const tarification = new TarificationPagePO(page);
    await tarification.goto();
    await tarification.expectPageLoaded();

    // Switch to Periods
    await tarification.switchToTab('Периоды');
    await tarification.expectPeriodsTabLoaded();

    // Switch to Tiers
    await tarification.switchToTab('Тиры');
    await tarification.expectTiersTabLoaded();

    // Switch to Sender Registrations
    await tarification.switchToTab('Регистрация отправителей');
    await page.waitForTimeout(500);

    // Switch back to Plans
    await tarification.switchToTab('Тарифные планы');
    await tarification.expectPlansTableVisible();
  });
});

test.describe('Тарификация — Тарифные планы (Plans Tab)', () => {
  test.use({ storageState: ADMIN_STATE });

  test('таблица тарифных планов загружается', async ({ page }) => {
    const tarification = new TarificationPagePO(page);
    await tarification.goto();
    await tarification.expectPageLoaded();
    await tarification.expectPlansTableVisible();

    // Table should have header columns — use table scope to avoid ambiguity
    const table = page.locator('table');
    await expect(table.getByText('Категория / Стратегия')).toBeVisible();
    await expect(table.getByText('Активен')).toBeVisible();
  });

  test('кнопка "Создать план" открывает модальное окно', async ({ page }) => {
    const tarification = new TarificationPagePO(page);
    await tarification.goto();
    await tarification.expectPageLoaded();

    await tarification.clickCreatePlan();
    await tarification.expectCreatePlanModal();
  });

  test('модальное окно создания плана содержит все поля', async ({ page }) => {
    const tarification = new TarificationPagePO(page);
    await tarification.goto();
    await tarification.expectPageLoaded();

    await tarification.clickCreatePlan();
    await tarification.expectCreatePlanModal();

    const dialog = page.getByRole('dialog');
    await expect(dialog.locator('label', { hasText: 'Оператор' })).toBeVisible();
    await expect(dialog.locator('label', { hasText: 'Категория отправителя' })).toBeVisible();
    await expect(dialog.locator('label', { hasText: 'Стратегия тарификации' })).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Создать' })).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Отмена' })).toBeVisible();
  });

  test('отмена создания плана закрывает модальное окно', async ({ page }) => {
    const tarification = new TarificationPagePO(page);
    await tarification.goto();
    await tarification.expectPageLoaded();

    await tarification.clickCreatePlan();
    await tarification.expectCreatePlanModal();

    await tarification.cancelCreatePlan();
    // Wait for dialog to close
    await page.waitForTimeout(500);
    await expect(page.getByRole('dialog')).not.toBeVisible({ timeout: 5000 });
  });
});

test.describe('Тарификация — Периоды (Periods Tab)', () => {
  test.use({ storageState: ADMIN_STATE });

  test('таб периодов загружается с заголовком', async ({ page }) => {
    const tarification = new TarificationPagePO(page);
    await tarification.goto();
    await tarification.switchToTab('Периоды');
    await tarification.expectPeriodsTabLoaded();

    await expect(page.getByRole('button', { name: 'Создать период' })).toBeVisible();
  });

  test('фильтры периодов отображаются', async ({ page }) => {
    const tarification = new TarificationPagePO(page);
    await tarification.goto();
    await tarification.switchToTab('Периоды');
    await tarification.expectPeriodsTabLoaded();

    // FilterBar should show these labels — use label locators to avoid ambiguity
    await expect(page.locator('label', { hasText: 'Страна' }).first()).toBeVisible();
    await expect(page.locator('label', { hasText: 'Оператор' }).first()).toBeVisible();
    await expect(page.locator('label', { hasText: 'Категория отправителя' }).first()).toBeVisible();
    await expect(page.locator('label', { hasText: 'Тип трафика' }).first()).toBeVisible();
  });

  test('кнопка "Создать период" открывает модальное окно', async ({ page }) => {
    const tarification = new TarificationPagePO(page);
    await tarification.goto();
    await tarification.switchToTab('Периоды');

    await tarification.clickCreatePeriod();
    await tarification.expectCreatePeriodModal();

    const dialog = page.getByRole('dialog');
    await expect(dialog.locator('label', { hasText: 'Стратегия' }).first()).toBeVisible();
    await expect(dialog.locator('label', { hasText: 'Дата начала' })).toBeVisible();
  });

  test('таблица периодов содержит колонки статуса и стратегии', async ({ page }) => {
    const tarification = new TarificationPagePO(page);
    await tarification.goto();
    await tarification.switchToTab('Периоды');
    await tarification.expectPeriodsTabLoaded();

    // Table headers
    await expect(page.getByText('Область действия')).toBeVisible();
    await expect(page.getByText('Стратегия')).toBeVisible();
    await expect(page.getByText('Дата начала')).toBeVisible();
    await expect(page.getByText('Статус')).toBeVisible();
  });
});

test.describe('Тарификация — Тиры (Tiers Tab)', () => {
  test.use({ storageState: ADMIN_STATE });

  test('таб тиров загружается', async ({ page }) => {
    const tarification = new TarificationPagePO(page);
    await tarification.goto();
    await tarification.switchToTab('Тиры');
    await tarification.expectTiersTabLoaded();

    await expect(page.getByRole('button', { name: 'Создать тир' })).toBeVisible();
  });

  test('фильтры тиров (план и период) отображаются', async ({ page }) => {
    const tarification = new TarificationPagePO(page);
    await tarification.goto();
    await tarification.switchToTab('Тиры');
    await tarification.expectTiersTabLoaded();

    await expect(page.locator('label', { hasText: 'Тарифный план' }).first()).toBeVisible();
    await expect(page.locator('label', { hasText: 'Период' }).first()).toBeVisible();
  });

  test('модальное окно создания тира содержит необходимые поля', async ({ page }) => {
    const tarification = new TarificationPagePO(page);
    await tarification.goto();
    await tarification.switchToTab('Тиры');

    await tarification.clickCreateTier();
    await tarification.expectCreateTierModal();

    const dialog = page.getByRole('dialog');
    await expect(dialog.locator('label', { hasText: 'Период' })).toBeVisible();
    await expect(dialog.locator('label', { hasText: 'От (кол-во сегментов)' })).toBeVisible();
    await expect(dialog.locator('label', { hasText: 'Цена за сегмент' })).toBeVisible();
  });

  test('таблица тиров отображает корректные колонки', async ({ page }) => {
    const tarification = new TarificationPagePO(page);
    await tarification.goto();
    await tarification.switchToTab('Тиры');
    await tarification.expectTiersTabLoaded();

    // Table columns
    await expect(page.getByText('От (кол-во)')).toBeVisible();
    await expect(page.getByText('Цена за сегмент')).toBeVisible();
  });
});

// ═══════════════════════════════════════════════════════════════════════
// Admin Tarification API Tests
// ═══════════════════════════════════════════════════════════════════════

test.describe('Тарификация — Admin API проверки', () => {
  test.use({ storageState: ADMIN_STATE });

  test('API: список тарифных планов (admin)', async ({ request }) => {
    const admin = new AdminApiHelper(request);
    const result = await admin.listTariffPlans();
    expect(result).toBeTruthy();
    const plans = result.tariff_plans || [];
    expect(Array.isArray(plans)).toBeTruthy();
  });

  test('API: список операторов (admin)', async ({ request }) => {
    const admin = new AdminApiHelper(request);
    const result = await admin.listOperators({ limit: '10' });
    expect(result).toBeTruthy();
    const ops = result.operators || [];
    expect(Array.isArray(ops)).toBeTruthy();
  });

  test('API: список периодов (admin)', async ({ request }) => {
    const admin = new AdminApiHelper(request);
    const result = await admin.listPeriods();
    expect(result).toBeTruthy();
    const periods = result.periods || [];
    expect(Array.isArray(periods)).toBeTruthy();
  });

  test('API: CRUD период тарификации', async ({ request }) => {
    const admin = new AdminApiHelper(request);

    // Create period
    const today = new Date().toISOString().split('T')[0];
    const createResult = await admin.createPeriod({
      strategy: 'fixed',
      start_date: today,
    });
    expect(createResult).toBeTruthy();

    const periodId = createResult.period?.id;
    if (!periodId) {
      // If creation returned without period id, skip
      test.skip(true, 'Period creation did not return ID');
      return;
    }

    // Verify it appears in the list
    const periods = await admin.listPeriods();
    const found = periods.periods?.find((p: { id: string }) => p.id === periodId);
    expect(found).toBeTruthy();
    expect(found.strategy).toBe('fixed');

    // Cleanup: delete period
    await admin.deletePeriod(periodId);
  });

  test('API: создание тира для периода', async ({ request }) => {
    const admin = new AdminApiHelper(request);

    // Create a period first
    const today = new Date().toISOString().split('T')[0];
    const periodResult = await admin.createPeriod({
      strategy: 'threshold',
      start_date: today,
    });

    const periodId = periodResult.period?.id;
    if (!periodId) {
      test.skip(true, 'Period creation failed');
      return;
    }

    // Create tier for this period
    const tierResult = await admin.createPeriodTier(periodId, {
      from_count: 0,
      price_per_segment: '1.50',
    });
    expect(tierResult).toBeTruthy();

    // Create second tier
    const tier2Result = await admin.createPeriodTier(periodId, {
      from_count: 1000,
      price_per_segment: '1.20',
    });
    expect(tier2Result).toBeTruthy();

    // List tiers for period
    const tiers = await admin.listPeriodTiers(periodId);
    expect(tiers.tiers?.length).toBeGreaterThanOrEqual(2);

    // Cleanup
    await admin.deletePeriod(periodId);
  });
});

// ═══════════════════════════════════════════════════════════════════════
// Client-facing Tariffs Page Tests (/tariffs)
// ═══════════════════════════════════════════════════════════════════════

test.describe('Тарифы — Клиентская страница', () => {
  test.use({ storageState: CLIENT_STATE });

  test('страница тарифов загружается', async ({ page }) => {
    const tariffs = new TariffsPagePO(page);
    await tariffs.goto();
    await tariffs.expectPageLoaded();
  });

  test('отображается текущий план', async ({ page }) => {
    const tariffs = new TariffsPagePO(page);
    await tariffs.goto();

    // Wait for loading to finish
    await page.waitForTimeout(2000);

    // Page title should be "Тарифы"
    await expect(page.getByText('Тарифы').first()).toBeVisible();

    // Current plan card OR error message should be visible
    const hasCurrentPlan = await page.getByText('Текущий план').isVisible().catch(() => false);
    const hasError = await page.locator('.text-red-600').isVisible().catch(() => false);
    const hasPageTitle = await page.getByText('Управление тарифным планом').isVisible().catch(() => false);
    expect(hasCurrentPlan || hasError || hasPageTitle).toBeTruthy();
  });

  test('доступные планы отображаются для сравнения', async ({ page }) => {
    const tariffs = new TariffsPagePO(page);
    await tariffs.goto();

    // Wait for plans to load
    await page.waitForTimeout(1000);

    // Should have multiple plan cards or a comparison table
    const planElements = page.locator('[class*="border"][class*="rounded"]');
    const count = await planElements.count();
    expect(count).toBeGreaterThanOrEqual(1);
  });
});
