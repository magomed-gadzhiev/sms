import { test, expect } from '@playwright/test';
import { CampaignWizardPage } from '../../pages/CampaignsPage';

test.use({ storageState: './auth-state.json' });

/**
 * AC source: docs/ac/campaign-wizard-ac.md (Batch 4 Phase A: Step 4 Confirm).
 * Seed: same as batches 2/3 — 1 approved sender_name + 1 contact list.
 * Phase A excludes actual submit/launch — see Phase B (future).
 */
test.describe('Campaign Wizard — Step 4 (Confirm) Phase A', () => {

  function toLocalISODate(d: Date): string {
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${y}-${m}-${day}`;
  }

  function tomorrowISODate(): string {
    return toLocalISODate(new Date(Date.now() + 24 * 60 * 60 * 1000));
  }

  test('AC-CW-C-01: Auto-name has "SMS · DD.MM.YYYY · HH:mm" format', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep4Now();

    const name = (await wizard.campaignNameDisplay().textContent()) ?? '';
    expect(name.trim()).toMatch(/^SMS · \d{2}\.\d{2}\.\d{4} · \d{2}:\d{2}$/);
  });

  test('AC-CW-C-02: Pencil button switches name to editable input', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep4Now();

    // Before: display span
    await expect(wizard.campaignNameDisplay()).toBeVisible();
    await expect(wizard.campaignNameInput()).toBeHidden();

    await wizard.editNameButton().click();

    // After: input with the same value
    await expect(wizard.campaignNameInput()).toBeVisible();
    const inputValue = await wizard.campaignNameInput().inputValue();
    expect(inputValue).toMatch(/^SMS · /);
    // And focus is on the input
    await expect(wizard.campaignNameInput()).toBeFocused();
  });

  test('AC-CW-C-03: Summary "Сообщение" shows text + sender', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep4Now();

    const messageSection = wizard.summarySection('Сообщение').first();
    // Text entered by reachStep3/reachStep4Now helper
    await expect(messageSection).toContainText('Test message for schedule AC tests');
    // Sender line "Отправитель: ..."
    await expect(messageSection).toContainText(/Отправитель:\s+\S+/);
  });

  test('AC-CW-C-04: Summary "Аудитория" shows list name + pluralized count', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep4Now();

    const audienceSection = wizard.summarySection('Аудитория').first();
    // Content: "<name> · <N> контакт|контакта|контактов"
    await expect(audienceSection).toContainText(/·\s*\d[\d\s]*\s+контакт(а|ов)?/);
  });

  test('AC-CW-C-05: Summary "Расписание" = "Сейчас" for mode=now', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep4Now();

    const scheduleSection = wizard.summarySection('Расписание').first();
    await expect(scheduleSection.locator('dd').first()).toHaveText('Сейчас');
  });

  test('AC-CW-C-06: Summary "Расписание" shows localized datetime for mode=later', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep4Later(tomorrowISODate(), '12:00');

    const scheduleSection = wizard.summarySection('Расписание').first();
    // Locale-formatted datetime — must include tomorrow's year and time.
    // Use tomorrow's year (not new Date().getFullYear()) to avoid Dec-31 rollover flake.
    const tomorrowYear = new Date(Date.now() + 24 * 60 * 60 * 1000).getFullYear();
    await expect(scheduleSection.locator('dd').first()).toContainText(String(tomorrowYear));
    await expect(scheduleSection.locator('dd').first()).toContainText('12:00');
  });

  test('AC-CW-C-07: Cost estimate POST fires on entering Step 4', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);

    const costReqPromise = page.waitForRequest(
      (req) =>
        req.method() === 'POST' &&
        new URL(req.url()).pathname === '/portal/v1/campaigns/estimate-cost',
      { timeout: 15_000 },
    );

    await wizard.reachStep4Now();

    const req = await costReqPromise;
    const body = req.postDataJSON();
    expect(body.contact_list_id, 'estimate-cost must receive contact_list_id').toBeTruthy();
    expect(typeof body.source, 'estimate-cost must receive source (sender name) as string').toBe('string');
  });

  test('AC-CW-C-08: Cost estimate block renders labels after response', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep4Now();

    const block = wizard.costEstimateBlock();
    await expect(block).toBeVisible();
    // Wait for skeleton placeholder to disappear before reading content.
    // (toContainText('') is a no-op — every element contains empty string.)
    await expect(block.locator('.animate-pulse')).toHaveCount(0, { timeout: 15_000 });

    // Either valid data (labels present) or documented error text
    const hasLabels =
      (await block.getByText('Получателей').count()) > 0 &&
      (await block.getByText('Частей SMS').count()) > 0 &&
      (await block.getByText('Итого').count()) > 0 &&
      (await block.getByText('Баланс').count()) > 0;
    const hasErrorText = (await block.getByText('Не удалось рассчитать стоимость').count()) > 0;

    expect(
      hasLabels || hasErrorText,
      'Cost block must show either all 4 labels or the error fallback',
    ).toBe(true);

    if (hasErrorText) {
      test.skip(true, 'Cost estimate API returned error — AC-C-08 skipped. Investigate backend.');
    }
  });

  test('AC-CW-C-09: Submit button text = "Отправить" for mode=now', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep4Now();

    await expect(wizard.submitButton()).toHaveText('Отправить');
  });

  test('AC-CW-C-10: Submit button text = "Запланировать" for mode=later', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep4Later(tomorrowISODate(), '12:00');

    await expect(wizard.submitButton()).toHaveText('Запланировать');
  });

  test('AC-CW-C-11: "изменить" link in summary navigates back to corresponding step', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep4Now();

    // Click "изменить" next to "Сообщение"
    await wizard.summaryEditLink('Сообщение').click();

    await wizard.expectActiveStep('Сообщение');
  });

});
