import { test, expect } from '@playwright/test';
import { ProviderWizardPage } from '../../pages/ProviderWizardPage';

test.use({ storageState: './auth-state.json' });

test.describe('Step5 Routing — provider wizard', () => {

  test('AC-P1: Step5 открывается пустым после прохождения шагов 1-4', async ({ page }) => {
    const wiz = new ProviderWizardPage(page);
    await wiz.gotoStep5();

    await expect(wiz.addRuleButton()).toBeVisible();
    await expect(page.locator('input[placeholder="^7.*"]')).toHaveCount(0);
    await expect(page.locator('button:has-text("×")')).toHaveCount(0);
  });

  test('AC-P2: «+ Add Rule» добавляет строку с пустым pattern и priority=1', async ({ page }) => {
    const wiz = new ProviderWizardPage(page);
    await wiz.gotoStep5();
    await wiz.clickAddRule();

    expect(await wiz.countRules()).toBe(1);
    await expect(wiz.patternInput(0)).toHaveValue('');
    await expect(wiz.priorityInput(0)).toHaveValue('1');
    await expect(wiz.removeRuleButton(0)).toBeVisible();
  });

  test('AC-P3: редактирование pattern и priority сохраняется в поле', async ({ page }) => {
    const wiz = new ProviderWizardPage(page);
    await wiz.gotoStep5();
    await wiz.clickAddRule();

    await wiz.patternInput(0).fill('^79.*');
    await wiz.priorityInput(0).fill('5');

    await expect(wiz.patternInput(0)).toHaveValue('^79.*');
    await expect(wiz.priorityInput(0)).toHaveValue('5');
  });

  test('AC-P4: удаление одной из двух строк через × оставляет вторую', async ({ page }) => {
    const wiz = new ProviderWizardPage(page);
    await wiz.gotoStep5();

    await wiz.clickAddRule();
    await wiz.patternInput(0).fill('A');
    await wiz.clickAddRule();
    await wiz.patternInput(1).fill('B');

    expect(await wiz.countRules()).toBe(2);

    await wiz.removeRuleButton(0).click();

    expect(await wiz.countRules()).toBe(1);
    await expect(wiz.patternInput(0)).toHaveValue('B');
  });

  test('AC-P5: правило сохраняется при навигации Back → Next', async ({ page }) => {
    const wiz = new ProviderWizardPage(page);
    await wiz.gotoStep5();

    await wiz.clickAddRule();
    await wiz.patternInput(0).fill('^7');
    await wiz.priorityInput(0).fill('10');

    await wiz.clickBack(); // → Step 4
    await expect(page.locator('h3:has-text("Routing Rules")')).not.toBeVisible();

    await wiz.clickNext(); // → обратно Step 5
    await wiz.expectOnStep5();

    expect(await wiz.countRules()).toBe(1);
    await expect(wiz.patternInput(0)).toHaveValue('^7');
    await expect(wiz.priorityInput(0)).toHaveValue('10');
  });

});
