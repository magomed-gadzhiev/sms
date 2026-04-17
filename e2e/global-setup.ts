import { chromium, type FullConfig } from '@playwright/test';

async function loginAndSaveState(
  baseURL: string,
  email: string,
  password: string,
  statePath: string,
) {
  const browser = await chromium.launch();
  const context = await browser.newContext();
  const page = await context.newPage();

  await page.goto(`${baseURL}/login`);
  await page.fill('[name="email"], #email, input[type="email"]', email);
  await page.fill('[name="password"], #password, input[type="password"]', password);
  await page.click('button[type="submit"]');
  await page.waitForURL('**/command-center', { timeout: 15_000 });

  await context.storageState({ path: statePath });
  await browser.close();
}

async function globalSetup(config: FullConfig) {
  const baseURL = config.projects[0].use.baseURL!;

  // Check server availability
  const res = await fetch(baseURL).catch(() => null);
  if (!res) throw new Error(`Server not available at ${baseURL}`);

  // Client auth
  await loginAndSaveState(
    baseURL,
    process.env.TEST_USER_EMAIL || 'test@example.com',
    process.env.TEST_USER_PASSWORD || 'testpassword',
    './auth-state.json',
  );

  // Admin auth
  await loginAndSaveState(
    baseURL,
    process.env.TEST_ADMIN_EMAIL || 'admin@example.com',
    process.env.TEST_ADMIN_PASSWORD || 'Admin123!',
    './admin-auth-state.json',
  );

  // Reseller auth (may be same as client if reseller flag is on the test user)
  const resellerEmail = process.env.TEST_RESELLER_EMAIL;
  const resellerPassword = process.env.TEST_RESELLER_PASSWORD;
  if (resellerEmail && resellerPassword) {
    await loginAndSaveState(baseURL, resellerEmail, resellerPassword, './reseller-auth-state.json');
  }
}

export default globalSetup;
