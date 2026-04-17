import { test, expect } from '@playwright/test';
import { NetworkDashboardPage } from '../../pages/NetworkDashboardPage';
import { ApiHelper } from '../../helpers/api';

test.use({ storageState: './reseller-auth-state.json' });

test.describe('Reseller — Дашборд сети', () => {

  test('дашборд загружается и отображает все карточки метрик', async ({ page }) => {
    const dashboard = new NetworkDashboardPage(page);
    await dashboard.goto();
    await dashboard.expectLoaded();
    await dashboard.expectAllMetricCards();
  });

  test('переключение периода "Сегодня" → "7 дней" обновляет данные', async ({ page }) => {
    const dashboard = new NetworkDashboardPage(page);
    await dashboard.goto();
    await dashboard.expectLoaded();

    await dashboard.selectPeriod('7 дней');
    // Should reload without error
    await dashboard.expectLoaded();
    await dashboard.expectTrafficCard();
  });

  test('переключение периода "30 дней"', async ({ page }) => {
    const dashboard = new NetworkDashboardPage(page);
    await dashboard.goto();
    await dashboard.expectLoaded();

    await dashboard.selectPeriod('30 дней');
    await dashboard.expectLoaded();
    await dashboard.expectDeliveryRateCard();
  });

  test('карточка "Баланс сети" показывает свой и сетевой баланс', async ({ page }) => {
    const dashboard = new NetworkDashboardPage(page);
    await dashboard.goto();
    await dashboard.expectLoaded();

    await expect(page.locator('text=Свой:')).toBeVisible();
    await expect(page.locator('text=Сеть:')).toBeVisible();
  });

  test('карточка "Трафик" показывает доставлено и ошибки', async ({ page }) => {
    const dashboard = new NetworkDashboardPage(page);
    await dashboard.goto();
    await dashboard.expectLoaded();

    await expect(page.locator('text=Доставлено:')).toBeVisible();
    await expect(page.locator('text=Ошибки:')).toBeVisible();
  });

  test('карточка "Модерация" видна', async ({ page }) => {
    const dashboard = new NetworkDashboardPage(page);
    await dashboard.goto();
    await dashboard.expectLoaded();
    await dashboard.expectModerationCard();

    // Should show either count or "Нет ожидающих заявок"
    const content = page.locator('text=заявок').or(page.locator('text=Нет ожидающих заявок'));
    await expect(content.first()).toBeVisible();
  });

  test('карточка "Топ-5 субаккаунтов" отображается', async ({ page }) => {
    const dashboard = new NetworkDashboardPage(page);
    await dashboard.goto();
    await dashboard.expectLoaded();
    await dashboard.expectTopSubAccounts();

    // Either data table or "Нет данных"
    const content = page.locator('text=Нет данных за текущий месяц').or(page.locator('table'));
    await expect(content.first()).toBeVisible();
  });

  test('карточка "Проблемные субаккаунты" отображается', async ({ page }) => {
    const dashboard = new NetworkDashboardPage(page);
    await dashboard.goto();
    await dashboard.expectLoaded();
    await dashboard.expectProblemSubAccounts();

    // Either problems or "Все субаккаунты в норме"
    const content = page.locator('text=Все субаккаунты в норме').or(page.locator('ul li'));
    await expect(content.first()).toBeVisible();
  });

  test('API данные дашборда доступны', async ({ request }) => {
    const api = new ApiHelper(request);
    const data = await api.getResellerDashboard('today');

    expect(data).toHaveProperty('network_balance');
    expect(data).toHaveProperty('traffic');
    expect(data).toHaveProperty('moderation_counts');
    expect(data.network_balance).toHaveProperty('total');
    expect(data.traffic).toHaveProperty('total_sent');
    expect(data.traffic).toHaveProperty('delivery_rate');
  });
});
