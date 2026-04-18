import { test, expect } from '@playwright/test';
import { CampaignWizardPage } from '../../pages/CampaignsPage';

test.use({ storageState: './auth-state.json' });

/**
 * AC source: docs/ac/campaign-wizard-ac.md (Batch 5 Phase A: Navigation + Drafts).
 * Seed: 1 approved sender + 1 contact list (same as batches 2-4).
 * Phase A excludes save-as-draft and ?draft=id loading — those create/read
 * real campaigns and need cleanup (Phase B).
 *
 * Related drift: D-08 — dialog's discard button label is "Отмена" in code
 * vs "Не сохранять" in spec. Tests assert behavior (navigate on click), not label.
 */
test.describe('Campaign Wizard — Navigation Phase A', () => {

  test('AC-CW-N-01: "Отмена" in wizard nav opens confirm dialog', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.goto();
    await wizard.expectActiveStep('Сообщение');

    // Dialog not visible yet
    await expect(wizard.cancelDialog()).toBeHidden();

    await wizard.cancelButton().click();

    await expect(wizard.cancelDialog()).toBeVisible();
    await expect(wizard.cancelDialog()).toContainText('Отменить создание рассылки?');
    await expect(wizard.cancelDialog()).toContainText(
      'Хотите сохранить текущий прогресс как черновик или выйти без сохранения?'
    );
  });

  test('AC-CW-N-02: Dialog has both action buttons', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.goto();
    await wizard.cancelButton().click();

    await expect(wizard.cancelDialog()).toBeVisible();

    await expect(wizard.dialogSaveDraftButton()).toBeVisible();
    await expect(wizard.dialogDiscardButton()).toBeVisible();
  });

  test('AC-CW-N-03: Dialog discard closes dialog and navigates to /campaigns with no POST', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.goto();

    // Track POSTs to /campaigns — should see NONE
    const postCalls: string[] = [];
    page.on('request', (req) => {
      if (
        req.method() === 'POST' &&
        new URL(req.url()).pathname === '/portal/v1/campaigns'
      ) {
        postCalls.push(req.url());
      }
    });

    await wizard.cancelButton().click();
    await expect(wizard.cancelDialog()).toBeVisible();

    await wizard.dialogDiscardButton().click();

    // Dialog gone
    await expect(wizard.cancelDialog()).toBeHidden();
    // URL is /campaigns
    await expect(page).toHaveURL(/\/campaigns(\?|$)/);
    // No POST /campaigns fired
    expect(postCalls, 'Discard must not create a campaign').toHaveLength(0);
  });

  test('AC-CW-N-04: "Назад" button is not rendered on Step 1', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.goto();
    await wizard.expectActiveStep('Сообщение');

    await expect(wizard.backButton()).toBeHidden();
  });

  test('AC-CW-N-05: "Назад" on Step 2 returns to Step 1', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep2();

    await expect(wizard.backButton()).toBeVisible();
    await wizard.backButton().click();

    await wizard.expectActiveStep('Сообщение');
  });

  test('AC-CW-N-06: "Назад" on Step 3 returns to Step 2', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep3();

    await wizard.backButton().click();

    await wizard.expectActiveStep('Аудитория');
  });

  test('AC-CW-N-07: "Назад" on Step 4 returns to Step 3', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep4Now();

    await wizard.backButton().click();

    await wizard.expectActiveStep('Расписание');
  });

  test('AC-CW-N-08: StepIndicator click on reached step navigates back', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep3();

    // Click the "Сообщение" step button in StepIndicator
    await wizard.stepIndicatorButton('Сообщение').click();

    await wizard.expectActiveStep('Сообщение');
  });

  test('AC-CW-N-09: StepIndicator button for unreached step is disabled', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.goto();
    await wizard.expectActiveStep('Сообщение');

    // On Step 1, maxReachedIndex=0 → Step 4 button unreachable
    await expect(wizard.stepIndicatorButton('Подтверждение')).toBeDisabled();
  });

});
