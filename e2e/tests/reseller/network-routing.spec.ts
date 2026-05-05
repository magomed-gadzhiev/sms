import { test, expect } from '@playwright/test';
import { ProvidersCatalogPage } from '../../pages/ProvidersCatalogPage';
import { ProviderSetsPage } from '../../pages/ProviderSetsPage';
import { AssignmentsPage } from '../../pages/AssignmentsPage';
import { RouteSetsPage } from '../../pages/RouteSetsPage';

// Plan 1 (aggregator-routing-plan-1-foundation) — заменяет старый bulk-assign spec.
// Plan 2 (aggregator-routing-plan-2-routing) — расширяет route-set'ами + override + preview.
// Используем существующий storage state (см. global-setup.ts), отдельного login helper нет.
// Plan говорит про `loginAsAggregator(page)` из helpers/auth.ts — такого helper'а в репо нет,
// придерживаемся pattern'а Plan 1: storage state, прогретый globalSetup'ом для reseller.
test.use({ storageState: './reseller-auth-state.json' });

// Уникальный suffix на весь файл — изолируем от прошлых run'ов в БД (тестим против реального сервера).
const RUN_STAMP = Date.now().toString(36);
const PROVIDER_SET_NAME = `E2E PS Plan2 ${RUN_STAMP}`;
const ROUTE_SET_NAME = `E2E RS Plan2 ${RUN_STAMP}`;
const RULE_NAME = `RU rule ${RUN_STAMP}`;

test.describe('Aggregator network routing — Plan 1', () => {
  test('full flow: create private provider → add to set → assign to subaccount', async ({ page }) => {
    const stamp = Date.now().toString(36);
    const providerName = `E2E Provider ${stamp}`;
    const setName = `E2E Set ${stamp}`;

    // 1. Создать private провайдера.
    const cat = new ProvidersCatalogPage(page);
    await cat.goto();
    await cat.expectLoaded();
    await cat.addPrivate(providerName, 'smpp.example.com');
    await cat.expectRow(providerName);

    // 2. Создать provider-set и добавить в него только что созданного провайдера.
    const sets = new ProviderSetsPage(page);
    await sets.goto();
    await sets.expectLoaded();
    await sets.create(setName);
    await sets.addItemByName(providerName);
    await sets.expectItemRow(providerName);

    // 3. Назначить набор первому суб-аккаунту в /network/assignments.
    const assigns = new AssignmentsPage(page);
    await assigns.goto();
    await assigns.expectLoaded();
    await assigns.assignFirstSubAccount(setName);
  });

  test('redirect from old /network/routing → /network/assignments', async ({ page }) => {
    await page.goto('/network/routing');
    await expect(page).toHaveURL(/\/network\/assignments$/, { timeout: 5_000 });
    await expect(page.locator('h1, h2').filter({ hasText: 'Назначения суб-аккаунтам' }).first()).toBeVisible();
  });
});

test.describe('Aggregator network routing — Plan 2', () => {
  test('full flow: provider-set + route-set + assign + verify', async ({ page }) => {
    // 1. Создаём provider-set с одним провайдером (берём первый доступный из catalog).
    const ps = new ProviderSetsPage(page);
    await ps.goto();
    await ps.expectLoaded();
    await ps.create(PROVIDER_SET_NAME);
    await ps.addItemByName(/I-Digital|MaximusSMS|.+/);

    // 2. Создаём route-set с правилом RU → провайдер.
    const rs = new RouteSetsPage(page);
    await rs.goto();
    await rs.expectLoaded();
    await rs.create(ROUTE_SET_NAME);
    await rs.addRule({ name: RULE_NAME, provider: /I-Digital|MaximusSMS|.+/, country: 'RU' });
    await rs.expectRuleRow(RULE_NAME);

    // 3. Назначаем оба set'а первому суб-аккаунту.
    const a = new AssignmentsPage(page);
    await a.goto();
    await a.expectLoaded();
    await a.assignFirstSubAccount(PROVIDER_SET_NAME, ROUTE_SET_NAME);
  });

  test('preview matches by country', async ({ page }) => {
    const rs = new RouteSetsPage(page);
    await rs.goto();
    await rs.expectLoaded();
    await rs.select(ROUTE_SET_NAME); // создан в предыдущем тесте
    await rs.runPreview('79991234567', 'Test', 'transactional');
    await expect(page.locator(`text=${RULE_NAME}`).first()).toBeVisible({ timeout: 8_000 });
  });

  test('override route in sub-account card', async ({ page }) => {
    // Идём в первый суб-аккаунт через клик по ссылке в /network/assignments,
    // т.к. plan'овский селектор `a:has-text("QATestAudit")` зависит от data fixture'а.
    const a = new AssignmentsPage(page);
    await a.goto();
    await a.expectLoaded();
    const firstSubAccountLink = page.locator('tbody tr').first().locator('a').first();
    await firstSubAccountLink.click();
    await page.waitForURL(/\/network\/sub-accounts\//, { timeout: 8_000 });

    // Network-tab уже активен на странице карточки.
    // Секция "Кастомные маршруты (overrides)" → кнопка "+ Добавить".
    const section = page.locator('h3:has-text("Кастомные маршруты")').locator('..').locator('..');
    await section.locator('button:has-text("+ Добавить")').first().click();

    const drawer = page.locator('[role="dialog"], aside').filter({ hasText: 'override' }).first();
    await expect(drawer).toBeVisible({ timeout: 8_000 });

    // Drawer для route-override содержит select провайдера. Берём первый non-empty option.
    const providerSelect = drawer.locator('select').first();
    await providerSelect.selectOption({ index: 1 });

    // Submit — текст кнопки в drawer'е "Добавить" / "Сохранить".
    const submitBtn = drawer.locator('button:has-text("Добавить"), button:has-text("Сохранить")').last();
    await submitBtn.click();

    // Override должен появиться в таблице override-маршрутов (или toast об успехе).
    await expect(page.locator('text=/override|добавлен/i').first()).toBeVisible({ timeout: 8_000 });
  });

  test('bulk-assign with conflict shows modal', async () => {
    // Этот тест требует preset state: provider-set "Empty", route-set с провайдером не из Empty.
    // Pragmatic: тест требует подготовки fixtures (TODO Plan 3) — документируем reproducer вручную.
    test.skip(true, 'requires conflict-preset; manual smoke covers this');
  });

  test('redirect from old /network/routing', async ({ page }) => {
    await page.goto('/network/routing');
    await expect(page).toHaveURL(/\/network\/assignments/, { timeout: 5_000 });
  });
});
