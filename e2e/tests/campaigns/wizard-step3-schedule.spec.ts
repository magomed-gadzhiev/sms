import { test, expect } from '@playwright/test';
import { CampaignWizardPage } from '../../pages/CampaignsPage';

test.use({ storageState: './auth-state.json' });

/**
 * AC source: docs/ac/campaign-wizard-ac.md (Batch 2 Phase A: Step 3 Schedule)
 *
 * Seed requirements (see batch 2 preamble in AC doc):
 *   - 1 approved sender_name for test user
 *   - 1 contact list (any) for test user
 * Tests that reach Step 3 depend on these. If seed missing, tests will fail
 * at reachStep3() with visible reason.
 *
 * Phase B (deferred): timezone checkbox AC — see DRIFT D-04 + D-01.
 */
test.describe('Campaign Wizard — Step 3 (Schedule)', () => {

  // Build YYYY-MM-DD in LOCAL time. Using toISOString() would return UTC and
  // produce wrong dates in negative-offset timezones (e.g., US CI runs at UTC).
  function toLocalISODate(d: Date): string {
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${y}-${m}-${day}`;
  }

  function todayISODate(): string {
    return toLocalISODate(new Date());
  }

  function tomorrowISODate(): string {
    return toLocalISODate(new Date(Date.now() + 24 * 60 * 60 * 1000));
  }

  test('AC-CW-S-01: Step 3 opens with "Сейчас" mode selected by default', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep3();

    await wizard.expectScheduleModeChecked('now');
    await wizard.expectScheduleModeNotChecked('later');

    // Date/time inputs are not rendered in "now" mode
    await expect(wizard.scheduleDateInput()).toBeHidden();
    await expect(wizard.scheduleTimeInput()).toBeHidden();
  });

  test('AC-CW-S-02: Selecting "Позже" reveals date and time pickers', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep3();

    await wizard.clickScheduleModeRadio('later');

    await wizard.expectScheduleModeChecked('later');
    await wizard.expectScheduleModeNotChecked('now');
    // Verify label association as per AC — inputs accessible via their labels
    await expect(page.getByLabel('Дата')).toBeVisible();
    await expect(page.getByLabel('Время')).toBeVisible();
  });

  test('AC-CW-S-03: Switching back to "Сейчас" hides date/time pickers', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep3();

    // First go to "later" to reveal fields
    await wizard.clickScheduleModeRadio('later');
    await expect(page.getByLabel('Дата')).toBeVisible();

    // Then back to "now"
    await wizard.clickScheduleModeRadio('now');
    await wizard.expectScheduleModeChecked('now');
    await expect(page.getByLabel('Дата')).toBeHidden();
    await expect(page.getByLabel('Время')).toBeHidden();
  });

  test('AC-CW-S-04: Date picker has `min` attribute set to today', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep3();

    await wizard.clickScheduleModeRadio('later');

    const today = todayISODate();
    await expect(wizard.scheduleDateInput()).toHaveAttribute('min', today);
  });

  test('AC-CW-S-05: Inline error when scheduled time is in the past', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep3();

    await wizard.clickScheduleModeRadio('later');

    // Today at 00:00 — guaranteed <= now+5min except at the exact 00:00:00 tick
    await wizard.fillScheduledDateTime(todayISODate(), '00:00');

    await expect(
      page.locator('p.text-red-600', {
        hasText: /не менее чем через 5 минут/,
      })
    ).toBeVisible();
  });

  test('AC-CW-S-06: Valid future time allows proceeding to Step 4', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep3();

    await wizard.clickScheduleModeRadio('later');

    // Tomorrow noon — always safely in the future
    await wizard.fillScheduledDateTime(tomorrowISODate(), '12:00');

    // No inline error
    await expect(
      page.locator('p.text-red-600', { hasText: /не менее чем через 5 минут/ })
    ).toBeHidden();

    await wizard.nextButton().click();
    await wizard.expectActiveStep('Подтверждение');
  });

  test('AC-CW-S-07: "Позже" with empty date/time blocks Next', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep3();

    await wizard.clickScheduleModeRadio('later');
    // Do not fill date/time

    await wizard.nextButton().click();

    // Validation error should appear (expect auto-waits up to default timeout)
    await expect(
      page.locator('p.text-red-600', {
        hasText: /корректную дату и время отправки/,
      })
    ).toBeVisible();

    // And we're still on Step 3
    await wizard.expectActiveStep('Расписание');
  });

  test('AC-CW-S-08: "Сейчас" mode allows Next to Step 4 without extra fields', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep3();

    // Default is "now" — just click Next
    await wizard.expectScheduleModeChecked('now');
    await wizard.nextButton().click();

    await wizard.expectActiveStep('Подтверждение');
  });

});
