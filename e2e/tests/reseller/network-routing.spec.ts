import { test, expect } from '@playwright/test';
import { ProvidersCatalogPage } from '../../pages/ProvidersCatalogPage';
import { ProviderSetsPage } from '../../pages/ProviderSetsPage';
import { AssignmentsPage } from '../../pages/AssignmentsPage';

// Plan 1 (aggregator-routing-plan-1-foundation) — заменяет старый bulk-assign spec.
// Используем существующий storage state (см. global-setup.ts), отдельного login helper нет.
test.use({ storageState: './reseller-auth-state.json' });

test.describe('Aggregator network routing — Plan 1', () => {
  test('full flow: create private provider → add to set → assign to subaccount', async ({ page }) => {
    // Уникальные имена через Date.now() — изолируем от прошлых run'ов в БД (тестим против реального сервера).
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
    // create() автоматически селектит новый set в правой панели.
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
