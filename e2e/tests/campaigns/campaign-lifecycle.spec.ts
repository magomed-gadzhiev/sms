import { test, expect } from '@playwright/test';
import { CampaignsListPage, CampaignDetailPage } from '../../pages/CampaignsPage';
import { ApiHelper } from '../../helpers/api';
import { CAMPAIGN_NAME_PREFIX } from '../../helpers/test-data';

test.use({ storageState: './auth-state.json' });

test.describe('Campaigns — Жизненный цикл рассылки', () => {

  test('создание рассылки через API → отображается в списке', async ({ page, request }) => {
    const api = new ApiHelper(request);

    const senders = await api.listApprovedSenderNames();
    const contacts = await api.listContactLists();
    const templates = await api.listTemplates({ status: 'approved' });

    if (!senders.sender_names?.length || !contacts.items?.length) {
      test.skip(true, 'Missing prerequisites');
      return;
    }

    const name = `${CAMPAIGN_NAME_PREFIX}-lifecycle-${Date.now()}`;
    const campaign = await api.createCampaign({
      name,
      contact_list_id: contacts.items[0].id,
      template_id: templates.templates?.[0]?.id,
      source: senders.sender_names[0].name,
      send_rate: 100,
    });

    expect(campaign.id).toBeTruthy();

    // Verify in UI list
    const list = new CampaignsListPage(page);
    await list.goto();
    await list.filterByStatus('draft');
    await page.waitForTimeout(1000);
    await list.expectCampaignInList(name);

    // Cleanup
    await api.deleteCampaign(campaign.id);
  });

  test('страница деталей рассылки: статистика загружается', async ({ page, request }) => {
    const api = new ApiHelper(request);

    const senders = await api.listApprovedSenderNames();
    const contacts = await api.listContactLists();
    const templates = await api.listTemplates({ status: 'approved' });

    if (!senders.sender_names?.length || !contacts.items?.length) {
      test.skip(true, 'Missing prerequisites');
      return;
    }

    const name = `${CAMPAIGN_NAME_PREFIX}-detail-${Date.now()}`;
    const campaign = await api.createCampaign({
      name,
      contact_list_id: contacts.items[0].id,
      template_id: templates.templates?.[0]?.id,
      source: senders.sender_names[0].name,
    });

    const detail = new CampaignDetailPage(page);
    await detail.goto(campaign.id);
    await detail.expectLoaded();
    await detail.expectStatus('Черновик');

    // Stat cards should be visible
    const recipients = await detail.getStatValue('Получатели');
    expect(recipients).toBeTruthy();

    // Cleanup
    await api.deleteCampaign(campaign.id);
  });

  test('запуск черновика → статус "Запущена" или "Подготовка"', async ({ page, request }) => {
    const api = new ApiHelper(request);

    const senders = await api.listApprovedSenderNames();
    const contacts = await api.listContactLists();
    const templates = await api.listTemplates({ status: 'approved' });

    if (!senders.sender_names?.length || !contacts.items?.length) {
      test.skip(true, 'Missing prerequisites');
      return;
    }

    const name = `${CAMPAIGN_NAME_PREFIX}-launch-${Date.now()}`;
    const campaign = await api.createCampaign({
      name,
      contact_list_id: contacts.items[0].id,
      template_id: templates.templates?.[0]?.id,
      source: senders.sender_names[0].name,
    });

    const detail = new CampaignDetailPage(page);
    await detail.goto(campaign.id);
    await detail.expectLoaded();
    await detail.clickLaunch();

    // Wait for status change
    await page.waitForTimeout(2000);

    const status = await detail.getStatus();
    // Status should be one of: running, materializing, or possibly completed (if tiny list)
    expect(['Запущена', 'Подготовка', 'Завершена']).toContain(status);
  });

  test('пауза запущенной рассылки → статус "На паузе"', async ({ page, request }) => {
    const api = new ApiHelper(request);

    const senders = await api.listApprovedSenderNames();
    const contacts = await api.listContactLists();
    const templates = await api.listTemplates({ status: 'approved' });

    if (!senders.sender_names?.length || !contacts.items?.length) {
      test.skip(true, 'Missing prerequisites');
      return;
    }

    const name = `${CAMPAIGN_NAME_PREFIX}-pause-${Date.now()}`;
    const campaign = await api.createCampaign({
      name,
      contact_list_id: contacts.items[0].id,
      template_id: templates.templates?.[0]?.id,
      source: senders.sender_names[0].name,
    });

    // Launch via API
    await api.launchCampaign(campaign.id);
    await new Promise((r) => setTimeout(r, 2000));

    // Check status — can only pause if running
    const campaignState = await api.getCampaign(campaign.id);
    if (campaignState.status !== 'running') {
      // May have completed already for small contact list
      test.skip(true, 'Campaign already completed — cannot test pause');
      return;
    }

    const detail = new CampaignDetailPage(page);
    await detail.goto(campaign.id);
    await detail.expectLoaded();
    await detail.clickPause();

    await page.waitForTimeout(2000);
    await detail.expectStatus('На паузе');
  });

  test('отмена рассылки → статус "Отменена"', async ({ page, request }) => {
    const api = new ApiHelper(request);

    const senders = await api.listApprovedSenderNames();
    const contacts = await api.listContactLists();
    const templates = await api.listTemplates({ status: 'approved' });

    if (!senders.sender_names?.length || !contacts.items?.length) {
      test.skip(true, 'Missing prerequisites');
      return;
    }

    const name = `${CAMPAIGN_NAME_PREFIX}-cancel-${Date.now()}`;
    const campaign = await api.createCampaign({
      name,
      contact_list_id: contacts.items[0].id,
      template_id: templates.templates?.[0]?.id,
      source: senders.sender_names[0].name,
    });

    // Launch via API
    await api.launchCampaign(campaign.id);
    await new Promise((r) => setTimeout(r, 2000));

    const campaignState = await api.getCampaign(campaign.id);
    if (campaignState.status !== 'running' && campaignState.status !== 'paused') {
      test.skip(true, 'Campaign not in cancellable state');
      return;
    }

    const detail = new CampaignDetailPage(page);
    await detail.goto(campaign.id);
    await detail.expectLoaded();
    await detail.clickCancel();

    await page.waitForTimeout(2000);
    await detail.expectStatus('Отменена');
  });

  test('статистика рассылки через API: структура корректна', async ({ request }) => {
    const api = new ApiHelper(request);

    const senders = await api.listApprovedSenderNames();
    const contacts = await api.listContactLists();

    if (!senders.sender_names?.length || !contacts.items?.length) {
      test.skip(true, 'Missing prerequisites');
      return;
    }

    const name = `${CAMPAIGN_NAME_PREFIX}-stats-${Date.now()}`;
    const campaign = await api.createCampaign({
      name,
      contact_list_id: contacts.items[0].id,
      source: senders.sender_names[0].name,
    });

    const stats = await api.getCampaignStats(campaign.id);
    expect(stats).toBeTruthy();
    expect(stats.total_recipients).toBeDefined();
    expect(stats.sent).toBeDefined();
    expect(stats.delivered).toBeDefined();
    expect(stats.failed).toBeDefined();

    // Cleanup
    await api.deleteCampaign(campaign.id);
  });

  test('стоимость рассылки отображается в деталях завершённой', async ({ page }) => {
    // Look for any completed campaign
    const list = new CampaignsListPage(page);
    await list.goto();
    await list.filterByStatus('completed');
    await page.waitForTimeout(1000);

    const rows = page.locator('tbody tr');
    const count = await rows.count();
    if (count === 0) {
      test.skip(true, 'No completed campaigns to check cost');
      return;
    }

    await rows.first().click();

    const detail = new CampaignDetailPage(page);
    await detail.expectLoaded();
    // Cost should be shown in delivery rate card subtext
    await detail.expectCostShown();
  });
});

test.describe('Campaigns — Список рассылок', () => {

  test('список рассылок загружается', async ({ page }) => {
    const list = new CampaignsListPage(page);
    await list.goto();
    // Should either show campaigns or empty state
    const hasCampaigns = await page.locator('tbody tr').count() > 0;
    const hasEmptyState = await page.locator('text=Рассылки не созданы').isVisible().catch(() => false);
    expect(hasCampaigns || hasEmptyState).toBeTruthy();
  });

  test('фильтр по статусу работает', async ({ page }) => {
    const list = new CampaignsListPage(page);
    await list.goto();
    await list.filterByStatus('draft');
    await page.waitForTimeout(1000);
    // Should not error
  });

  test('кнопка "Создать рассылку" ведёт на визард', async ({ page }) => {
    const list = new CampaignsListPage(page);
    await list.goto();
    await list.clickCreateCampaign();
    await expect(page).toHaveURL(/\/campaigns\/new/);
  });

  test('удаление черновика → кампания исчезает из списка', async ({ page, request }) => {
    const api = new ApiHelper(request);

    const senders = await api.listApprovedSenderNames();
    const contacts = await api.listContactLists();

    if (!senders.sender_names?.length || !contacts.items?.length) {
      test.skip(true, 'Missing prerequisites');
      return;
    }

    const name = `${CAMPAIGN_NAME_PREFIX}-delete-${Date.now()}`;
    await api.createCampaign({
      name,
      contact_list_id: contacts.items[0].id,
      source: senders.sender_names[0].name,
    });

    const list = new CampaignsListPage(page);
    await list.goto();
    await list.filterByStatus('draft');
    await page.waitForTimeout(1000);
    await list.expectCampaignInList(name);

    await list.deleteDraftCampaign(name);
    await page.waitForTimeout(1000);

    // Campaign should be gone
    await expect(page.locator(`text=${name}`)).toBeHidden();
  });
});
