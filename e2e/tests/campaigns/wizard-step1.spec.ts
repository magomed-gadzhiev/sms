import { test, expect } from '@playwright/test';
import { CampaignWizardPage } from '../../pages/CampaignsPage';

test.use({ storageState: './auth-state.json' });

/**
 * AC source: docs/ac/campaign-wizard-ac.md (batch 1: Step 1 Message)
 * DRIFT note: some AC expected to fail until D-02 code fix is deployed
 *   (see docs/ac/_DRIFT.md). Red tests here = confirmation of drift, not test bugs.
 */
test.describe('Campaign Wizard — Step 1 (Message)', () => {

  test('AC-CW-M-01: Wizard opens on Step 1 by default', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.goto();

    await wizard.expectActiveStep('Сообщение');
    await expect(wizard.messageTextarea()).toBeVisible();
    await expect(wizard.nextButton()).toBeVisible();
    // Back button must not be visible on first step
    await expect(page.getByRole('button', { name: 'Назад', exact: true })).toBeHidden();
  });

  test('AC-CW-M-02: Sender dropdown loads and first approved is pre-selected', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.goto();

    await wizard.expectSenderLoaded();
    const senderValue = await wizard.getSenderValue();
    expect(
      senderValue,
      'First approved sender must be pre-selected. If empty, test user has no approved senders — seed required.'
    ).not.toBe('');
  });

  test('AC-CW-M-03: Cannot proceed when textarea is empty (validation error about text)', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.goto();

    expect(await wizard.getMessageText()).toBe('');

    // "Далее" button is always enabled in this implementation; canProceed validation
    // runs on click and blocks transition + shows red error
    await wizard.nextButton().click();
    await page.waitForTimeout(300);

    // Still on Step 1
    await wizard.expectActiveStep('Сообщение');
    // Error must mention text (spec: text is the required field)
    await expect(
      page.locator('p.text-red-600').filter({ hasText: /[Вв]ведите текст/ })
    ).toBeVisible();
  });

  test('AC-CW-M-04: Text validation passes after typing (canProceed text check)', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.goto();

    await wizard.fillMessageText('Привет мир');
    await wizard.nextButton().click();
    await page.waitForTimeout(500);

    // Specifically: error about empty text must NOT appear. Other errors
    // (e.g., sender missing) may still appear but are out of scope here.
    await expect(
      page.locator('p.text-red-600').filter({ hasText: /[Вв]ведите текст/ })
    ).toBeHidden();
  });

  test('AC-CW-M-05: Character counter reflects typed length', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.goto();

    const text = 'Hello world!';
    await wizard.fillMessageText(text);

    // CharacterCounter renders the current length as text — look for "12" near the textarea
    await expect(
      page.locator('text=/\\b' + text.length + '\\b/').first()
    ).toBeVisible();
  });

  test('AC-CW-M-06: Selecting template populates textarea with its body', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.goto();

    await wizard.selectTemplate();
    // After template selection, textarea should contain template body (non-empty)
    const textAfter = await wizard.getMessageText();
    expect(
      textAfter.length,
      'Spec §Step 1: "при выборе шаблона текст подставляется в textarea". Empty textarea after template pick = DRIFT D-02 not yet fixed.'
    ).toBeGreaterThan(0);
  });

  test('AC-CW-M-07: A/B toggle reveals variant B section', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.goto();

    // Initially hidden
    await wizard.expectVariantBHidden();
    expect(await wizard.isABEnabled()).toBe(false);

    // Toggle on
    await wizard.toggleAB();

    expect(await wizard.isABEnabled()).toBe(true);
    await wizard.expectVariantBVisible();
    // Split slider, duration select, metric radio — all must appear
    await expect(page.locator('input[aria-label="Доля аудитории для теста"]')).toBeVisible();
    await expect(page.locator('fieldset', { hasText: 'Метрика победителя' })).toBeVisible();
  });

  test('AC-CW-M-08: Cannot proceed when A/B on and variant B text is empty', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.goto();

    // Fill variant A to satisfy main validation
    await wizard.fillMessageText('Текст варианта A');

    // Enable A/B — variant B text is empty by default
    await wizard.toggleAB();
    await wizard.expectVariantBVisible();

    await wizard.nextButton().click();
    await page.waitForTimeout(300);

    // Still on Step 1
    await wizard.expectActiveStep('Сообщение');
    // Error must specifically mention VARIANT B TEXT (not template B) —
    // this is the drift-detecting assertion for D-02
    await expect(
      page.locator('p.text-red-600').filter({ hasText: /[Вв]ведите текст варианта B/ })
    ).toBeVisible();
  });

});
