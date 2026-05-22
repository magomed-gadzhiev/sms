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
    requireEnv('TEST_USER_EMAIL'),
    requireEnv('TEST_USER_PASSWORD'),
    './auth-state.json',
  );

  // Admin auth
  await loginAndSaveState(
    baseURL,
    requireEnv('TEST_ADMIN_EMAIL'),
    requireEnv('TEST_ADMIN_PASSWORD'),
    './admin-auth-state.json',
  );

  // Reseller auth (may be same as client if reseller flag is on the test user)
  const resellerEmail = process.env.TEST_RESELLER_EMAIL;
  const resellerPassword = process.env.TEST_RESELLER_PASSWORD;
  if (resellerEmail && resellerPassword) {
    await loginAndSaveState(baseURL, resellerEmail, resellerPassword, './reseller-auth-state.json');
  }
}

function requireEnv(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`${name} must be set`);
  return value;
}

export default globalSetup;
