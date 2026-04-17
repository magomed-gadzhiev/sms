import { chromium } from '@playwright/test';

/**
 * Creates admin auth state for admin-only E2E tests.
 * Run manually or from admin test beforeAll:
 *   npx playwright test --project chromium admin-setup.ts
 */
export async function createAdminAuthState(baseURL: string) {
  const browser = await chromium.launch();
  const context = await browser.newContext();
  const page = await context.newPage();

  const adminEmail = process.env.TEST_ADMIN_EMAIL || 'admin@example.com';
  const adminPassword = process.env.TEST_ADMIN_PASSWORD || 'Admin123!';

  await page.goto(`${baseURL}/login`);
  await page.fill('[name="email"], #email, input[type="email"]', adminEmail);
  await page.fill('[name="password"], #password, input[type="password"]', adminPassword);
  await page.click('button[type="submit"]');
  await page.waitForURL('**/command-center', { timeout: 15_000 });

  await context.storageState({ path: './admin-auth-state.json' });
  await browser.close();
}
