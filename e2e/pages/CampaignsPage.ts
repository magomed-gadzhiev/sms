import { type Page, expect } from '@playwright/test';

export class CampaignsListPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/campaigns');
    await expect(this.page.locator('h1, [class*="PageHeader"]')).toContainText('Рассылки');
  }

  async clickCreateCampaign() {
    await this.page.click('button:has-text("Создать рассылку")');
    await expect(this.page).toHaveURL(/\/campaigns\/new/);
  }

  async expectCampaignInList(name: string) {
    await expect(this.page.locator(`text=${name}`)).toBeVisible();
  }

  async filterByStatus(status: string) {
    await this.page.selectOption('#campaign-status-filter', status);
  }

  async expectEmptyState() {
    await expect(this.page.locator('text=Рассылки не созданы')).toBeVisible();
  }

  async openCampaign(name: string) {
    await this.page.locator(`tr:has-text("${name}")`).click();
  }

  async deleteDraftCampaign(name: string) {
    const row = this.page.locator(`tr:has-text("${name}")`);
    await row.locator('button:has-text("Удалить")').click();
    // Confirm deletion in dialog
    const dialog = this.page.locator('[role="dialog"]');
    await dialog.locator('button:has-text("Удалить")').click();
  }
}

export class CampaignWizardPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/campaigns/new');
    await expect(this.page.locator('text=Создать рассылку')).toBeVisible();
    // Wait for sender names and contact lists to load
    await this.page.waitForTimeout(1500);
  }

  // Step 1: Message
  async selectTemplate() {
    // TemplatePicker — click to open modal, select first template
    const pickerBtn = this.page.locator('[role="button"][aria-label="Выбрать шаблон"]');
    await pickerBtn.click();
    // Wait for modal with templates to load
    const dialog = this.page.locator('[role="dialog"]');
    await expect(dialog).toBeVisible({ timeout: 5000 });
    await this.page.waitForTimeout(1000); // Wait for templates to load
    // Click first template item in the modal
    const templateItem = dialog.locator('button, [role="button"], [class*="cursor-pointer"]').filter({ hasNotText: /Закрыть|Отмена|Поиск/ });
    const count = await templateItem.count();
    if (count > 0) {
      await templateItem.first().click();
    }
  }

  async selectSenderNameByIndex(index: number) {
    // Select component with label "Имя отправителя *" — auto-selects first on load
    // The id is generated from label: "имя-отправителя-*"
    const selects = this.page.locator('select');
    const count = await selects.count();
    for (let i = 0; i < count; i++) {
      const label = await selects.nth(i).locator('..').locator('label').textContent().catch(() => '');
      if (label?.includes('Имя отправителя')) {
        const options = await selects.nth(i).locator('option').all();
        // Skip placeholder option (value="")
        const realOptions = [];
        for (const opt of options) {
          const val = await opt.getAttribute('value');
          if (val) realOptions.push(opt);
        }
        if (realOptions.length > index) {
          const value = await realOptions[index].getAttribute('value');
          if (value) await selects.nth(i).selectOption(value);
        }
        return;
      }
    }
  }

  async enableABTest() {
    await this.page.click('button[role="switch"]');
  }

  async clickNext() {
    await this.page.click('button:has-text("Далее")');
  }

  // Step 2: Audience
  async selectContactListByIndex(index: number) {
    const selects = this.page.locator('select');
    const count = await selects.count();
    for (let i = 0; i < count; i++) {
      const label = await selects.nth(i).locator('..').locator('label').textContent().catch(() => '');
      if (label?.includes('Контактная база')) {
        const options = await selects.nth(i).locator('option').all();
        const realOptions = [];
        for (const opt of options) {
          const val = await opt.getAttribute('value');
          if (val) realOptions.push(opt);
        }
        if (realOptions.length > index) {
          const value = await realOptions[index].getAttribute('value');
          if (value) await selects.nth(i).selectOption(value);
        }
        return;
      }
    }
  }

  // Step 3: Schedule
  async selectSendNow() {
    await this.page.click('input[name="sendMode"][value="now"]');
  }

  async selectSendLater(date: string, time: string) {
    await this.page.click('input[name="sendMode"][value="later"]');
    await this.page.waitForTimeout(300);
    // Date and Time inputs appear when "later" is selected
    const dateInput = this.page.locator('input[type="date"]');
    const timeInput = this.page.locator('input[type="time"]');
    await dateInput.fill(date);
    await timeInput.fill(time);
  }

  // Step 4: Confirm
  async expectCostEstimate() {
    await expect(this.page.locator('text=Предварительная стоимость')).toBeVisible({ timeout: 15_000 });
  }

  async getCostEstimate(): Promise<string> {
    const row = this.page.locator('text=Итого').locator('..');
    const bold = row.locator('.font-bold');
    return (await bold.textContent()) || '0';
  }

  async editCampaignName(name: string) {
    await this.page.click('button[aria-label="Редактировать название"]');
    await this.page.fill('input[type="text"]', name);
    await this.page.locator('input[type="text"]').blur();
  }

  async clickLaunch() {
    await this.page.click('button:has-text("Отправить"), button:has-text("Запланировать")');
  }

  async clickSaveDraft() {
    await this.page.click('button:has-text("Сохранить как черновик")');
  }

  async expectValidationError(text: string) {
    await expect(this.page.locator(`p.text-red-600:has-text("${text}"), [role="alert"]:has-text("${text}")`).first()).toBeVisible({ timeout: 5000 });
  }

  // === AC helpers (batch 1: Step 1 Message) ===

  async expectActiveStep(label: 'Сообщение' | 'Аудитория' | 'Расписание' | 'Подтверждение') {
    await expect(
      this.page.locator('button[aria-current="step"]', { hasText: label })
    ).toBeVisible();
  }

  messageTextarea() {
    return this.page.locator('#msg-text');
  }

  async fillMessageText(text: string) {
    await this.messageTextarea().fill(text);
  }

  async getMessageText(): Promise<string> {
    return this.messageTextarea().inputValue();
  }

  async getSenderValue(): Promise<string> {
    // Select component renders native <select> with id derived from "Имя отправителя *"
    // Scope: the select whose preceding <label> contains "Имя отправителя"
    const senderSelect = this.page.locator('select').filter({
      has: this.page.locator('option', { hasText: '-- Выберите отправителя --' })
    });
    return senderSelect.inputValue();
  }

  async expectSenderLoaded() {
    // Either a sender is preselected or a "нет одобренных" hint is shown
    const senderSelect = this.page.locator('select').filter({
      has: this.page.locator('option', { hasText: '-- Выберите отправителя --' })
    });
    await expect(senderSelect).toBeVisible();
  }

  nextButton() {
    return this.page.getByRole('button', { name: 'Далее', exact: true });
  }

  async expectNextEnabled() {
    await expect(this.nextButton()).toBeEnabled();
  }

  async expectNextDisabled() {
    await expect(this.nextButton()).toBeDisabled();
  }

  abToggle() {
    return this.page.locator('button[role="switch"]');
  }

  async isABEnabled(): Promise<boolean> {
    const checked = await this.abToggle().getAttribute('aria-checked');
    return checked === 'true';
  }

  async toggleAB() {
    await this.abToggle().click();
  }

  variantBHeader() {
    return this.page.locator('h4', { hasText: 'Вариант B' });
  }

  async expectVariantBVisible() {
    await expect(this.variantBHeader()).toBeVisible();
  }

  async expectVariantBHidden() {
    await expect(this.variantBHeader()).toBeHidden();
  }

  variantBTextarea() {
    // Per spec, variant B has its own textarea. Currently not in code — will be added via D-02 fix.
    // Locator: the second #msg-text-like textarea, or a textarea inside the variant B panel
    return this.page.locator('[class*="border-l-2"] textarea, textarea#msg-text-b');
  }

  async fillVariantBText(text: string) {
    await this.variantBTextarea().fill(text);
  }

  // === AC helpers (batch 2: Step 3 Schedule) ===

  /**
   * Navigate wizard Step 1 → Step 2 → Step 3 with minimal valid data.
   * Seed required: 1 approved sender_name + 1 contact list for test user.
   * Throws with specific message if seed missing, rather than a generic
   * Playwright timeout on step transition.
   */
  async reachStep3() {
    await this.goto();
    await this.expectActiveStep('Сообщение');

    // Seed check: sender must be preselected (AC-M-02 invariant)
    const senderValue = await this.getSenderValue();
    if (!senderValue) {
      throw new Error(
        'reachStep3 failed at Step 1: no sender preselected. ' +
        'Seed missing — test user has no approved sender_name. ' +
        'Create one before running Batch 2 tests.',
      );
    }

    await this.fillMessageText('Test message for schedule AC tests');
    await this.nextButton().click();
    await this.expectActiveStep('Аудитория');

    // Seed check: at least one contact list must exist
    const contactListSelect = this.page.locator('select').filter({
      has: this.page.locator('option').filter({ hasText: /Контактная|выбер/i }),
    }).first();
    const optionsCount = await contactListSelect.locator('option').count();
    if (optionsCount <= 1) {
      // Only placeholder, no real options
      throw new Error(
        'reachStep3 failed at Step 2: no contact lists. ' +
        'Seed missing — test user has no contact lists. ' +
        'Create at least one before running Batch 2 tests.',
      );
    }

    await this.selectContactListByIndex(0);
    await this.nextButton().click();
    await this.expectActiveStep('Расписание');
  }

  scheduleModeRadio(mode: 'now' | 'later') {
    return this.page.locator(`input[name="sendMode"][value="${mode}"]`);
  }

  async expectScheduleModeChecked(mode: 'now' | 'later') {
    await expect(this.scheduleModeRadio(mode)).toBeChecked();
  }

  async expectScheduleModeNotChecked(mode: 'now' | 'later') {
    await expect(this.scheduleModeRadio(mode)).not.toBeChecked();
  }

  async clickScheduleModeRadio(mode: 'now' | 'later') {
    await this.scheduleModeRadio(mode).click();
  }

  scheduleDateInput() {
    return this.page.locator('input[type="date"]');
  }

  scheduleTimeInput() {
    return this.page.locator('input[type="time"]');
  }

  async fillScheduledDateTime(date: string, time: string) {
    await this.scheduleDateInput().fill(date);
    await this.scheduleTimeInput().fill(time);
  }
}

export class CampaignDetailPage {
  constructor(private page: Page) {}

  async goto(id: string) {
    await this.page.goto(`/campaigns/${id}`);
  }

  async expectLoaded() {
    await expect(this.page.locator('text=Получатели')).toBeVisible({ timeout: 15_000 });
  }

  async getStatus(): Promise<string> {
    // Badge renders as <span class="...rounded-full...">
    const badge = this.page.locator('span.rounded-full').first();
    return ((await badge.textContent()) || '').trim();
  }

  async expectStatus(status: string) {
    await expect(this.page.locator(`span.rounded-full:has-text("${status}")`)).toBeVisible({ timeout: 10_000 });
  }

  async getStatValue(label: string): Promise<string> {
    const card = this.page.locator(`text=${label}`).locator('..');
    const value = card.locator('[class*="text-2xl"], [class*="font-semibold"]');
    return (await value.textContent()) || '0';
  }

  async clickLaunch() {
    await this.page.click('button:has-text("Запустить")');
  }

  async clickPause() {
    await this.page.click('button:has-text("Пауза")');
  }

  async clickResume() {
    await this.page.click('button:has-text("Возобновить")');
  }

  async clickCancel() {
    await this.page.click('button:has-text("Отменить")');
    // Confirm cancellation in dialog
    const dialog = this.page.locator('[role="dialog"]');
    if (await dialog.isVisible()) {
      await dialog.locator('button:has-text("Отменить рассылку")').click();
    }
  }

  async expectProgressBar() {
    await expect(this.page.locator('text=Прогресс отправки')).toBeVisible();
  }

  async expectCostShown() {
    // Cost appears as subtext "Стоимость: X.XX RUB" in a stat card
    await expect(this.page.locator('text=Стоимость:').first()).toBeVisible({ timeout: 10_000 });
  }
}
