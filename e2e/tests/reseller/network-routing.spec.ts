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

  test('bulk-assign with conflict shows modal', async ({ page, request }) => {
    // Plan 3 Task 11: вместо ручного skip создаём conflict-preset через API:
    // 1) два private-провайдера A и B (агрегатор-owned),
    // 2) provider-set "PS-only-A" содержит только A,
    // 3) route-set "RS-uses-B" содержит правило с провайдером B,
    // 4) bulk-assign (PS-only-A, RS-uses-B) → backend ConflictValidator вернёт
    //    missing_providers=[B] → dry-run status=conflict → UI открывает Modal "Найдены конфликты".
    //
    // CSRF: используем cookie из reseller-auth-state.json (как ApiHelper).
    // baseURL передаётся playwright config'ом, /portal/v1 — портальный API префикс.
    const fs = await import('fs');
    const stateRaw = fs.readFileSync('./reseller-auth-state.json', 'utf-8');
    const state = JSON.parse(stateRaw) as { cookies: Array<{ name: string; value: string }> };
    const csrf = state.cookies.find((c) => c.name === 'csrf_token')?.value || '';
    const headers = { 'X-CSRF-Token': csrf, 'Content-Type': 'application/json' };
    const apiBase = '/portal/v1';

    // 1. Два private-провайдера. Имена уникальны через RUN_STAMP.
    const provAName = `E2E ConfA ${RUN_STAMP}`;
    const provBName = `E2E ConfB ${RUN_STAMP}`;
    const provARes = await request.post(`${apiBase}/reseller/network/providers`, {
      headers,
      data: { name: provAName, smpp_host: 'a.example.com', smpp_port: 2775, system_id: 'a', password: 'x', system_type: '' },
    });
    expect(provARes.ok(), `create provA: ${provARes.status()} ${await provARes.text()}`).toBeTruthy();
    const provA = await provARes.json() as { id: string };

    const provBRes = await request.post(`${apiBase}/reseller/network/providers`, {
      headers,
      data: { name: provBName, smpp_host: 'b.example.com', smpp_port: 2775, system_id: 'b', password: 'x', system_type: '' },
    });
    expect(provBRes.ok(), `create provB: ${provBRes.status()} ${await provBRes.text()}`).toBeTruthy();
    const provB = await provBRes.json() as { id: string };

    // 2. provider-set "PS-only-A" → PUT items с одним только A.
    const psName = `E2E PS-only-A ${RUN_STAMP}`;
    const psRes = await request.post(`${apiBase}/reseller/network/provider-sets`, {
      headers,
      data: { name: psName, is_default: false },
    });
    expect(psRes.ok(), `create PS: ${psRes.status()} ${await psRes.text()}`).toBeTruthy();
    const ps = await psRes.json() as { id: string };

    const psItemsRes = await request.fetch(`${apiBase}/reseller/network/provider-sets/${ps.id}/items`, {
      method: 'PUT',
      headers,
      data: { items: [{ provider_id: provA.id, priority: 100, expose_cost: false, expose_provider_name: false }] },
    });
    expect(psItemsRes.ok(), `PUT PS items: ${psItemsRes.status()} ${await psItemsRes.text()}`).toBeTruthy();

    // 3. route-set "RS-uses-B" с правилом на провайдере B.
    const rsName = `E2E RS-uses-B ${RUN_STAMP}`;
    const rsRes = await request.post(`${apiBase}/reseller/network/route-sets`, {
      headers,
      data: { name: rsName, is_default: false },
    });
    expect(rsRes.ok(), `create RS: ${rsRes.status()} ${await rsRes.text()}`).toBeTruthy();
    const rs = await rsRes.json() as { id: string };

    const ruleRes = await request.post(`${apiBase}/reseller/network/route-sets/${rs.id}/items`, {
      headers,
      data: {
        name: `Rule uses B ${RUN_STAMP}`,
        comment: '',
        provider_id: provB.id,
        priority: 100,
        share: 100,
        route_type: 'sms',
        status: 'active',
        condition_groups: [],
        schedules: [],
      },
    });
    expect(ruleRes.ok(), `create RS item: ${ruleRes.status()} ${await ruleRes.text()}`).toBeTruthy();

    // 4. UI: /network/assignments, выбираем первый sub-account, ставим bulk (PS-only-A, RS-uses-B), Применить.
    const a = new AssignmentsPage(page);
    await a.goto();
    await a.expectLoaded();

    const firstRow = page.locator('tbody tr').first();
    await expect(firstRow).toBeVisible({ timeout: 8_000 });
    // Чекбокс есть только если в таблице есть хоть один реальный sub-account.
    await firstRow.locator('input[type="checkbox"]').check();

    // Sticky bulk-panel появилась — заполняем оба select'а по value=id.
    await page.locator('select[aria-label="Provider-set для bulk"]').selectOption(ps.id);
    await page.locator('select[aria-label="Route-set для bulk"]').selectOption(rs.id);

    // Дожидаемся auto-dry-run: backend возвращает status='conflict' → виден прогноз-бейдж.
    await expect(page.locator('text=/Прогноз: \\d+ конфликт/').first()).toBeVisible({ timeout: 8_000 });

    // Нажимаем "Применить" — должен открыться Modal "Найдены конфликты" (Apply не выполняется).
    await page.locator('button:has-text("Применить")').first().click();

    // Modal с заголовком "Найдены конфликты" + текстом "provider-set не содержит провайдеров".
    const modal = page.locator('[role="dialog"]').filter({ hasText: 'Найдены конфликты' }).first();
    await expect(modal).toBeVisible({ timeout: 5_000 });
    await expect(modal.locator('text=/provider-set не содержит провайдеров/')).toBeVisible();
    // Кнопка отмены и кнопка "Всё равно применить" присутствуют (контракт UI).
    await expect(modal.locator('button:has-text("Отмена")')).toBeVisible();
    await expect(modal.locator('button:has-text("Всё равно применить")')).toBeVisible();

    // Закрываем — без коммита.
    await modal.locator('button:has-text("Отмена")').click();
  });

  test('redirect from old /network/routing', async ({ page }) => {
    await page.goto('/network/routing');
    await expect(page).toHaveURL(/\/network\/assignments/, { timeout: 5_000 });
  });
});
