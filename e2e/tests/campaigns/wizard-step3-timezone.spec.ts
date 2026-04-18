import { test, expect } from '@playwright/test';
import { CampaignWizardPage } from '../../pages/CampaignsPage';

test.use({ storageState: './auth-state.json' });

/**
 * AC source: docs/ac/campaign-wizard-ac.md (Batch 2 Phase B: timezone checkbox)
 *
 * Closes drift D-04 (checkbox was disabled placeholder) and D-01 (backend had
 * no persistence for use_subscriber_timezone).
 *
 * Seed: same as Phase A — 1 approved sender + 1 contact list.
 *
 * Round-trip AC-S-14 lives in `campaign-timezone-persistence.spec.ts` because
 * it needs raw API access + cleanup semantics, not UI flow.
 */
test.describe('Campaign Wizard — Step 3 Phase B (Timezone checkbox)', () => {

  function toLocalISODate(d: Date): string {
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${y}-${m}-${day}`;
  }

  function tomorrowISODate(): string {
    return toLocalISODate(new Date(Date.now() + 24 * 60 * 60 * 1000));
  }

  test('AC-CW-S-09: Checkbox is not in DOM when mode = "Сейчас"', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep3();

    // Default mode is "now" (AC-CW-S-01 already verifies that)
    await wizard.expectScheduleModeChecked('now');

    // Checkbox should be absent, not just disabled
    await wizard.expectTimezoneCheckboxHidden();
  });

  test('AC-CW-S-10: Checkbox appears enabled when mode = "Позже"', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep3();

    await wizard.clickScheduleModeRadio('later');

    await wizard.expectTimezoneCheckboxVisible();
    await wizard.expectTimezoneCheckboxEnabled();
    expect(
      await wizard.isTimezoneCheckboxChecked(),
      'Checkbox must start unchecked by default',
    ).toBe(false);
  });

  test('AC-CW-S-11: Checking box → POST body contains use_subscriber_timezone=true', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep3();

    await wizard.clickScheduleModeRadio('later');
    await wizard.fillScheduledDateTime(tomorrowISODate(), '12:00');
    await wizard.toggleTimezoneCheckbox();
    expect(await wizard.isTimezoneCheckboxChecked()).toBe(true);

    // Intercept POST /campaigns to capture body
    // Tight matcher: exact path /portal/v1/campaigns (not /estimate-cost etc.)
    const postPromise = page.waitForRequest(
      (req) =>
        req.method() === 'POST' &&
        new URL(req.url()).pathname === '/portal/v1/campaigns',
      { timeout: 15_000 },
    );

    await wizard.nextButton().click(); // Step 3 → Step 4
    await wizard.expectActiveStep('Подтверждение');

    // Click final submit
    // Exact match — "Отправить" also appears in an accordion header elsewhere on the page.
    // Tests always use "Позже" → submit button text is "Запланировать".
    await page.getByRole('button', { name: 'Запланировать', exact: true }).click();

    const req = await postPromise;
    const body = req.postDataJSON();
    expect(body.use_subscriber_timezone).toBe(true);
  });

  test('AC-CW-S-12: Without checkbox → POST body contains use_subscriber_timezone=false', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep3();

    await wizard.clickScheduleModeRadio('later');
    await wizard.fillScheduledDateTime(tomorrowISODate(), '12:00');
    // Do NOT toggle checkbox

    // Tight matcher: exact path /portal/v1/campaigns (not /estimate-cost etc.)
    const postPromise = page.waitForRequest(
      (req) =>
        req.method() === 'POST' &&
        new URL(req.url()).pathname === '/portal/v1/campaigns',
      { timeout: 15_000 },
    );

    await wizard.nextButton().click();
    await wizard.expectActiveStep('Подтверждение');
    // Exact match — "Отправить" also appears in an accordion header elsewhere on the page.
    // Tests always use "Позже" → submit button text is "Запланировать".
    await page.getByRole('button', { name: 'Запланировать', exact: true }).click();

    const req = await postPromise;
    const body = req.postDataJSON();
    expect(body.use_subscriber_timezone).toBe(false);
  });

  test('AC-CW-S-13: Switching "Позже" → "Сейчас" resets timezone state', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep3();

    // Enable "Позже" and check timezone box
    await wizard.clickScheduleModeRadio('later');
    await wizard.toggleTimezoneCheckbox();
    expect(await wizard.isTimezoneCheckboxChecked()).toBe(true);

    // Switch to "Сейчас" — checkbox leaves DOM, state should reset
    await wizard.clickScheduleModeRadio('now');
    await wizard.expectTimezoneCheckboxHidden();

    // Switch back to "Позже" — checkbox should be unchecked (not restored)
    await wizard.clickScheduleModeRadio('later');
    await wizard.expectTimezoneCheckboxVisible();
    expect(
      await wizard.isTimezoneCheckboxChecked(),
      'Switching away from "Позже" must reset checkbox state (AC-S-13)',
    ).toBe(false);
  });

});
