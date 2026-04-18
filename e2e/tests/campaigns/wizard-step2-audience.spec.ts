import { test, expect } from '@playwright/test';
import { CampaignWizardPage } from '../../pages/CampaignsPage';

test.use({ storageState: './auth-state.json' });

/**
 * AC source: docs/ac/campaign-wizard-ac.md (Batch 3: Step 2 Audience).
 * Seed: 1 contact list for test user. For AC-A-05 and AC-A-07 the list must
 * have ≥1 contact so segments endpoint returns at least one country.
 *
 * Related drift: D-06 — spec promises client-side exclusion count, code
 * shows only a "фильтры применяются при запуске" note. AC-A-07 written
 * against the code, not the spec.
 */
test.describe('Campaign Wizard — Step 2 (Audience)', () => {

  test('AC-CW-A-01: Step 2 opens with empty contact list selection', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep2();

    // Placeholder option shown, meaning no list selected
    const currentValue = await wizard.contactListSelect().inputValue();
    expect(currentValue).toBe('');

    // Filter toggle and summary not in DOM yet
    await expect(wizard.filtersToggleButton()).toBeHidden();
    await expect(wizard.summaryBlock()).toBeHidden();
  });

  test('AC-CW-A-02: Dropdown shows each list as "<name> (<count> контакт...)"', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep2();

    const options = await wizard.getContactListOptions();
    const realOptions = options.filter((o) => o.value !== '');
    expect(
      realOptions.length,
      'Seed missing: test user has no contact lists. Batch 3 requires ≥1.',
    ).toBeGreaterThan(0);

    // Labels match "<name> (N контакт|контакта|контактов)"
    for (const opt of realOptions) {
      expect(
        opt.label,
        `Option "${opt.label}" must match "<name> (<n> контакт[а|ов])"`,
      ).toMatch(/^.+\s\(\d[\d\s]*\sконтакт(а|ов)?\)$/);
    }
  });

  test('AC-CW-A-03: Selecting list reveals filter toggle and summary', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep2();

    const options = await wizard.getContactListOptions();
    const firstList = options.find((o) => o.value !== '');
    expect(firstList, 'No contact list available for test').toBeTruthy();

    await wizard.selectContactListByValue(firstList!.value);

    await expect(wizard.filtersToggleButton()).toBeVisible();
    await expect(wizard.summaryBlock()).toBeVisible();
    await expect(page.getByText('Выберите контактную базу.')).toBeHidden();
  });

  test('AC-CW-A-04: Summary displays the list count (toLocaleString)', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep2();

    const options = await wizard.getContactListOptions();
    const firstList = options.find((o) => o.value !== '');
    expect(firstList).toBeTruthy();

    // Parse the count from label "<name> (<N> контакт...)"
    const match = firstList!.label.match(/\((\d[\d\s]*)\s/);
    expect(match, `Could not extract count from label "${firstList!.label}"`).toBeTruthy();
    const expectedCount = Number(match![1].replace(/\s/g, ''));

    await wizard.selectContactListByValue(firstList!.value);

    // Summary contains the count formatted via toLocaleString (digits + whitespace separator OR plain for < 1000)
    const expectedFormatted = expectedCount.toLocaleString();
    // Tolerant match: look for the count number in the summary block, allowing either plain or space-grouped
    await expect(wizard.summaryBlock()).toContainText(
      new RegExp(expectedFormatted.replace(/\s/g, '[\\s\\u00A0]')),
    );
  });

  test('AC-CW-A-05: Clicking filter toggle reveals countries/operators (if segments exist)', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep2();

    const options = await wizard.getContactListOptions();
    const firstList = options.find((o) => o.value !== '');
    expect(firstList).toBeTruthy();

    await wizard.selectContactListByValue(firstList!.value);

    // Before click: toggle says "Добавить", filter block hidden
    await expect(wizard.filtersToggleButton()).toHaveText(/Добавить фильтры исключения/);

    await wizard.clickFiltersToggle();

    // Button text flips
    await expect(wizard.filtersToggleButton()).toHaveText(/Скрыть фильтры/);

    // At least one of countries/operators labels should appear.
    // If the selected list has NO contacts, neither will appear — document as skip condition.
    const countriesLabel = page.getByText('Исключить страны', { exact: true });
    const operatorsLabel = page.getByText('Исключить операторов', { exact: true });

    const countriesVisible = await countriesLabel.isVisible();
    const operatorsVisible = await operatorsLabel.isVisible();

    if (!countriesVisible && !operatorsVisible) {
      throw new Error(
        'AC-A-05 requires a contact list with ≥1 contact so segments endpoint ' +
        'returns at least one country or operator. Selected list appears empty. ' +
        `Choose a non-empty list or seed contacts into "${firstList!.label}".`,
      );
    }
  });

  test('AC-CW-A-06: Switching contact list resets filters state', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep2();

    const options = await wizard.getContactListOptions();
    const real = options.filter((o) => o.value !== '');
    if (real.length < 2) {
      test.skip(true, 'AC-A-06 requires ≥2 contact lists to test switching. Seed more lists.');
    }

    await wizard.selectContactListByValue(real[0].value);
    await wizard.clickFiltersToggle();
    await expect(wizard.filtersToggleButton()).toHaveText(/Скрыть фильтры/);

    // Switch to second list
    await wizard.selectContactListByValue(real[1].value);

    // Filter block is collapsed again: toggle says "Добавить"
    await expect(wizard.filtersToggleButton()).toHaveText(/Добавить фильтры исключения/);
  });

  test('AC-CW-A-07: Checking a country filter adds "применяются при запуске" note (D-06)', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep2();

    const options = await wizard.getContactListOptions();
    const firstList = options.find((o) => o.value !== '');
    expect(firstList).toBeTruthy();

    await wizard.selectContactListByValue(firstList!.value);
    await wizard.clickFiltersToggle();

    const countriesLabel = page.getByText('Исключить страны', { exact: true });
    if (!(await countriesLabel.isVisible())) {
      test.skip(true, 'AC-A-07 requires at least one country segment. Seed contacts into the list.');
    }

    const firstCheckbox = wizard.firstCountryCheckbox();
    await firstCheckbox.check();

    await expect(
      page.getByText('(фильтры применяются при запуске)'),
    ).toBeVisible();
  });

  test('AC-CW-A-08: Next blocked without list selection', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep2();

    // No list selected
    await wizard.nextButton().click();

    // Still on Step 2
    await wizard.expectActiveStep('Аудитория');

    // Error visible
    await expect(page.getByText('Выберите контактную базу.')).toBeVisible();
  });

  test('AC-CW-A-09: Next advances to Step 3 with list selected', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.reachStep2();

    const options = await wizard.getContactListOptions();
    const firstList = options.find((o) => o.value !== '');
    expect(firstList).toBeTruthy();

    await wizard.selectContactListByValue(firstList!.value);
    await wizard.nextButton().click();

    await wizard.expectActiveStep('Расписание');
  });

});
