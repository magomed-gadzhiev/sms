import { test, expect } from '@playwright/test';
import { ProfilePage } from '../../pages/ProfilePage';

test.use({ storageState: './auth-state.json' });

const TEST_USER_EMAIL = process.env.TEST_USER_EMAIL || 'test@example.com';

test.describe('Profile · Client (authenticated)', () => {
  test('AC-10 загрузка профиля показывает email и компанию', async ({ page }) => {
    const profile = new ProfilePage(page);
    await profile.goto();

    const email = await profile.getEmail();
    const company = await profile.getCompany();

    expect(email).toBe(TEST_USER_EMAIL);
    expect(company.length).toBeGreaterThan(0);
    await profile.expect2faSectionVisible();
  });

  test('AC-11 смена пароля: несовпадение подтверждения → «Пароли не совпадают»', async ({ page }) => {
    const profile = new ProfilePage(page);
    await profile.goto();

    let pwdRequestFired = false;
    page.on('request', (req) => {
      if (req.url().includes('/profile/password') && req.method() === 'PUT') {
        pwdRequestFired = true;
      }
    });

    await profile.fillPasswordChange('anycurrent', 'newpass123', 'different123');
    await profile.submitPasswordChange();

    await profile.expectPasswordError(/Пароли не совпадают/i);
    expect(pwdRequestFired).toBe(false);
  });

  test('AC-12 сохранение контактного лица и телефона → persists после reload', async ({ page }) => {
    const profile = new ProfilePage(page);
    await profile.goto();

    const originalContact = await profile.getContactPerson();
    const originalPhone = await profile.getPhone();

    const ts = Date.now();
    const newContact = `E2E-contact-${ts}`;
    const newPhone = `+7900${String(ts).slice(-7)}`;

    try {
      await profile.setContactPerson(newContact);
      await profile.setPhone(newPhone);

      const [resp] = await Promise.all([
        page.waitForResponse(
          (r) => r.url().includes('/profile') && r.request().method() === 'PUT' && !r.url().includes('/password') && !r.url().includes('/sandbox') && !r.url().includes('/2fa'),
        ),
        profile.saveProfile(),
      ]);
      expect(resp.status()).toBe(200);

      await profile.expectProfileSaved();

      await page.reload();
      await profile.goto();
      expect(await profile.getContactPerson()).toBe(newContact);
      expect(await profile.getPhone()).toBe(newPhone);
    } finally {
      // Rollback best-effort: не маскируем ошибку основного теста, если откат упадёт.
      try {
        await profile.goto();
        await profile.setContactPerson(originalContact);
        await profile.setPhone(originalPhone);
        await profile.saveProfile();
        await profile.expectProfileSaved();
      } catch (e) {
        console.warn('[AC-12] rollback failed, test user may be dirty:', e);
      }
    }
  });
});
