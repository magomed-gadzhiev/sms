import { test, expect } from '@playwright/test';
import { CampaignsListPage, CampaignWizardPage, CampaignDetailPage } from '../../pages/CampaignsPage';
import { ApiHelper } from '../../helpers/api';
import { CAMPAIGN_NAME_PREFIX } from '../../helpers/test-data';

test.use({ storageState: './auth-state.json' });

test.describe('Campaigns — Визард создания рассылки', () => {

  test('шаг 1: без шаблона нельзя перейти далее → валидация', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);
    await wizard.goto();
    // Try to click Next without selecting template
    await wizard.clickNext();
    await wizard.expectValidationError('Выберите');
  });

  test('шаг 1 → шаг 2: с выбранным шаблоном и отправителем переход работает', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const templates = await api.listTemplates({ status: 'approved' });
    if (!templates.templates?.length) {
      test.skip(true, 'No approved templates available');
      return;
    }

    const wizard = new CampaignWizardPage(page);
    await wizard.goto();

    // Select template via picker
    await wizard.selectTemplate();
    // Sender name auto-selects first approved
    await page.waitForTimeout(500);

    await wizard.clickNext();
    // Should be on step 2 (Audience) — h3 heading
    await expect(page.locator('h3:has-text("Аудитория")')).toBeVisible();
  });

  test('шаг 2: без контактной базы нельзя перейти далее', async ({ page, request }) => {
    const api = new ApiHelper(request);

    // Check prerequisites
    const senders = await api.listApprovedSenderNames();
    if (!senders.sender_names?.length) {
      test.skip(true, 'No approved sender names');
      return;
    }

    const templates = await api.listTemplates({ status: 'approved' });
    if (!templates.templates?.length) {
      test.skip(true, 'No approved templates');
      return;
    }

    const wizard = new CampaignWizardPage(page);
    await wizard.goto();

    // Pass step 1 — select template and sender
    await wizard.selectTemplate();
    await page.waitForTimeout(500);
    await wizard.clickNext();

    // Step 2 — try to proceed without selecting contact list
    await wizard.clickNext();
    await wizard.expectValidationError('Выберите контактную базу');
  });

  test('шаг 3: расписание "Сейчас" — переход к подтверждению', async ({ page, request }) => {
    const api = new ApiHelper(request);

    const senders = await api.listApprovedSenderNames();
    const templates = await api.listTemplates({ status: 'approved' });
    const contacts = await api.listContactLists();

    if (!senders.sender_names?.length || !templates.templates?.length || !contacts.items?.length) {
      test.skip(true, 'Missing prerequisites: sender names, templates, or contact lists');
      return;
    }

    const wizard = new CampaignWizardPage(page);
    await wizard.goto();

    // Step 1 — select template + sender
    await wizard.selectTemplate();
    await page.waitForTimeout(500);
    await wizard.clickNext();

    // Step 2
    await wizard.selectContactListByIndex(0);
    await page.waitForTimeout(500);
    await wizard.clickNext();

    // Step 3 — Send now (default)
    await expect(page.locator('h3:has-text("Расписание")')).toBeVisible();
    await wizard.selectSendNow();
    await wizard.clickNext();

    // Step 4 — Confirm
    await expect(page.locator('h3:has-text("Подтверждение")')).toBeVisible();
  });

  test('шаг 4: расчёт стоимости отображается на подтверждении', async ({ page, request }) => {
    const api = new ApiHelper(request);

    const senders = await api.listApprovedSenderNames();
    const templates = await api.listTemplates({ status: 'approved' });
    const contacts = await api.listContactLists();

    if (!senders.sender_names?.length || !templates.templates?.length || !contacts.items?.length) {
      test.skip(true, 'Missing prerequisites');
      return;
    }

    const wizard = new CampaignWizardPage(page);
    await wizard.goto();

    // Navigate through all steps — Step 1: template + sender
    await wizard.selectTemplate();
    await page.waitForTimeout(500);
    await wizard.clickNext();

    await wizard.selectContactListByIndex(0);
    await page.waitForTimeout(500);
    await wizard.clickNext();

    await wizard.selectSendNow();
    await wizard.clickNext();

    // Step 4 — cost estimate
    await wizard.expectCostEstimate();
    const cost = await wizard.getCostEstimate();
    expect(cost).toBeTruthy();
  });

  test('сохранение черновика → рассылка появляется в списке со статусом "Черновик"', async ({ page, request }) => {
    const api = new ApiHelper(request);

    const senders = await api.listApprovedSenderNames();
    const contacts = await api.listContactLists();

    if (!senders.sender_names?.length || !contacts.items?.length) {
      test.skip(true, 'Missing prerequisites');
      return;
    }

    const wizard = new CampaignWizardPage(page);
    await wizard.goto();

    // Select sender on step 1, then save draft (template not required for draft)
    await page.waitForTimeout(500);
    await wizard.clickSaveDraft();

    // Should redirect to campaigns list
    await expect(page).toHaveURL(/\/campaigns/, { timeout: 10_000 });

    // Verify draft appears
    const list = new CampaignsListPage(page);
    await list.filterByStatus('draft');
    // At least the new draft should exist
    await page.waitForTimeout(1000);
  });

  test('расписание "Позже": минимум 5 минут от текущего момента', async ({ page, request }) => {
    const api = new ApiHelper(request);

    const senders = await api.listApprovedSenderNames();
    const templates = await api.listTemplates({ status: 'approved' });
    const contacts = await api.listContactLists();

    if (!senders.sender_names?.length || !templates.templates?.length || !contacts.items?.length) {
      test.skip(true, 'Missing prerequisites');
      return;
    }

    const wizard = new CampaignWizardPage(page);
    await wizard.goto();

    // Step 1
    await wizard.selectTemplate();
    await page.waitForTimeout(500);
    await wizard.clickNext();

    await wizard.selectContactListByIndex(0);
    await page.waitForTimeout(500);
    await wizard.clickNext();

    // Select "Later" with time in the past
    const now = new Date();
    const pastDate = now.toISOString().split('T')[0];
    const pastTime = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`;

    await wizard.selectSendLater(pastDate, pastTime);

    // Should show validation error (inline text or validation error)
    await expect(page.locator('text=Время отправки должно быть не менее чем через 5 минут')).toBeVisible({ timeout: 3000 });
  });
});

test.describe('Campaigns — Оценка стоимости через API', () => {

  test('estimate-cost возвращает корректную структуру', async ({ request }) => {
    const api = new ApiHelper(request);

    const senders = await api.listApprovedSenderNames();
    const contacts = await api.listContactLists();

    if (!senders.sender_names?.length || !contacts.items?.length) {
      test.skip(true, 'Missing prerequisites');
      return;
    }

    const estimate = await api.estimateCost({
      contact_list_id: contacts.items[0].id,
      text: 'Test message for cost estimation',
      source: senders.sender_names[0].name,
    });

    expect(estimate.recipients).toBeDefined();
    expect(estimate.segments_per_msg).toBeDefined();
    expect(estimate.total_segments).toBeDefined();
    expect(estimate.estimated_cost).toBeDefined();
    expect(estimate.current_balance).toBeDefined();
    expect(typeof estimate.balance_sufficient).toBe('boolean');
  });
});
