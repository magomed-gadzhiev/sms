import { test, expect, type Page } from '@playwright/test';
import { RoutingPagePO } from '../../pages/RoutingPage';
import { ApiHelper } from '../../helpers/api';
import { pickProviderId, cleanupRoutesByPrefix } from '../../helpers/routing';

test.use({ storageState: './auth-state.json' });

const TEST_PREFIX = 'E2E C';

let activePrefix = '';

test.afterEach(async ({ request }) => {
  if (!activePrefix) return;
  const api = new ApiHelper(request);
  await cleanupRoutesByPrefix(api, activePrefix);
  activePrefix = '';
});

function conditionsSection(page: Page) {
  // Section is scoped via its heading; ConditionEditor is the next sibling inside.
  return page.getByRole('dialog').locator('section', { has: page.getByRole('heading', { name: 'Условия срабатывания' }) });
}

function scheduleSection(page: Page) {
  return page.getByRole('dialog').locator('section', { has: page.getByRole('heading', { name: 'Расписание' }) });
}

test.describe('Маршрутизация — ConditionEditor', () => {
  test('AC-C1: добавление второй группы условий', async ({ page }) => {
    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    const section = conditionsSection(page);

    // По умолчанию одна группа «ЕСЛИ» — нет кнопок «Удалить группу»
    await expect(section.getByRole('button', { name: 'Удалить группу' })).toHaveCount(0);
    await expect(section.getByText('ЕСЛИ', { exact: true })).toBeVisible();

    await section.getByRole('button', { name: '+ Добавить группу условий' }).click();

    await expect(section.getByRole('button', { name: 'Удалить группу' })).toHaveCount(1);
    // У второй группы рендерится <select> с опциями И/ИЛИ — native combobox
    await expect(section.getByRole('combobox')).toHaveCount(
      // 1 condition-type select в первой группе + 1 logic_op select во второй + 1 condition-type во второй
      3,
    );
  });

  test('AC-C2: смена logic_op второй группы на «ИЛИ»', async ({ page }) => {
    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    const section = conditionsSection(page);
    await section.getByRole('button', { name: '+ Добавить группу условий' }).click();

    // Второй логический select — его первый combobox во второй group-контейнере.
    const groupContainer = section.locator('div.border.rounded-lg.p-4.bg-gray-50').nth(1);
    const logicOp = groupContainer.getByRole('combobox').first();
    await logicOp.selectOption({ label: 'ИЛИ' });

    await expect(logicOp).toHaveValue('OR');
  });

  test('AC-C3: удаление второй группы возвращает одну ЕСЛИ', async ({ page }) => {
    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    const section = conditionsSection(page);
    await section.getByRole('button', { name: '+ Добавить группу условий' }).click();
    await expect(section.getByRole('button', { name: 'Удалить группу' })).toHaveCount(1);

    await section.getByRole('button', { name: 'Удалить группу' }).click();
    await expect(section.getByRole('button', { name: 'Удалить группу' })).toHaveCount(0);
    await expect(section.getByText('ЕСЛИ', { exact: true })).toBeVisible();
  });

  test('AC-C4: добавление и удаление условия внутри группы', async ({ page }) => {
    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    const section = conditionsSection(page);
    const removeCond = section.getByRole('button', { name: 'Удалить условие' });

    await expect(removeCond).toHaveCount(1);
    await section.getByRole('button', { name: '+ Условие' }).click();
    await expect(removeCond).toHaveCount(2);

    await removeCond.first().click();
    await expect(removeCond).toHaveCount(1);
  });

  test('AC-C5: смена типа условия с «Оператор» на «Страна» переключает контрол на input', async ({ page }) => {
    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    const section = conditionsSection(page);
    const conditionRow = section.locator('div.border.rounded-lg.p-4.bg-gray-50 >> div.flex.items-center.gap-2').first();

    // Исходно: operator → SearchableSelect (есть button с aria-haspopup="listbox")
    await expect(conditionRow.locator('button[aria-haspopup="listbox"]')).toHaveCount(1);
    await expect(conditionRow.locator('input[placeholder="Значение"]')).toHaveCount(0);

    await conditionRow.getByRole('combobox').first().selectOption({ label: 'Страна' });

    await expect(conditionRow.locator('input[placeholder="Значение"]')).toHaveCount(1);
    await expect(conditionRow.locator('button[aria-haspopup="listbox"]')).toHaveCount(0);
  });

  test('AC-C6: тип «Тип трафика» предоставляет фиксированный список', async ({ page }) => {
    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    const section = conditionsSection(page);
    const conditionRow = section.locator('div.border.rounded-lg.p-4.bg-gray-50 >> div.flex.items-center.gap-2').first();

    await conditionRow.getByRole('combobox').first().selectOption({ label: 'Тип трафика' });

    // Второй combobox в строке — select значения traffic_type
    const valueSelect = conditionRow.getByRole('combobox').nth(1);
    const options = await valueSelect.locator('option').allTextContents();
    expect(options).toContain('Авторизация');
    expect(options).toContain('Транзакционный');
    expect(options).toContain('Сервисный');
  });

  test('AC-C7: сохранение маршрута с regex-условием и проверка через API', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const providerId = await pickProviderId(api);
    test.skip(!providerId, 'нет ни одного provider');

    const prefix = `${TEST_PREFIX}7 ${Date.now()}`;
    activePrefix = prefix;
    const name = `${prefix} regex`;

    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    await routing.fillRouteName(name);
    await routing.selectFirstProvider();

    const section = conditionsSection(page);
    const conditionRow = section.locator('div.border.rounded-lg.p-4.bg-gray-50 >> div.flex.items-center.gap-2').first();
    await conditionRow.getByRole('combobox').first().selectOption({ label: 'Regex' });
    await conditionRow.locator('input[placeholder="Значение"]').fill('^7999.*');

    await routing.saveAsDraft();
    await expect(page.getByRole('dialog')).toBeHidden();
    await routing.expectRouteInTable(name);

    // Проверка persistence через API
    const list = await api.listRoutes();
    const created = (list.routes ?? []).find((r: { name: string }) => r.name === name);
    expect(created).toBeTruthy();
    const full = await api.getRoute(created.id);
    expect(full.condition_groups?.[0]?.conditions?.[0]).toMatchObject({
      type: 'regex',
      value: '^7999.*',
    });
  });
});

test.describe('Маршрутизация — ScheduleEditor', () => {
  test('AC-C8: включение switch разворачивает панель расписания', async ({ page }) => {
    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    const section = scheduleSection(page);
    const switchEl = section.getByRole('switch');

    await expect(switchEl).toHaveAttribute('aria-checked', 'false');
    // Изначально поля скрыты
    await expect(section.locator('input[type="date"]')).toHaveCount(0);

    await switchEl.click();

    await expect(switchEl).toHaveAttribute('aria-checked', 'true');
    await expect(section.locator('input[type="date"]')).toHaveCount(2);
    await expect(section.locator('input[type="time"]')).toHaveCount(2);
    // 7 кнопок дней недели
    const weekdayLabels = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'];
    for (const d of weekdayLabels) {
      await expect(section.getByRole('button', { name: d, exact: true })).toBeVisible();
    }
  });

  test('AC-C9: отключение дня недели снимает активный класс с кнопки', async ({ page }) => {
    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    const section = scheduleSection(page);
    await section.getByRole('switch').click();

    const monday = section.getByRole('button', { name: 'Пн', exact: true });
    const tuesday = section.getByRole('button', { name: 'Вт', exact: true });
    await expect(monday).toHaveClass(/bg-primary/);
    await expect(tuesday).toHaveClass(/bg-primary/);

    await monday.click();

    await expect(monday).not.toHaveClass(/bg-primary/);
    await expect(tuesday).toHaveClass(/bg-primary/);
  });

  test('AC-C10: сохранение расписания persist-ится (Пн-Пт 09:00-18:00 UTC)', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const providerId = await pickProviderId(api);
    test.skip(!providerId, 'нет ни одного provider');

    const prefix = `${TEST_PREFIX}10 ${Date.now()}`;
    activePrefix = prefix;
    const name = `${prefix} schedule`;

    const routing = new RoutingPagePO(page);
    await routing.goto();
    await routing.clickAddRoute();
    await routing.expectModalOpen('Новый маршрут');

    await routing.fillRouteName(name);
    await routing.selectFirstProvider();

    const section = scheduleSection(page);
    await section.getByRole('switch').click();
    await expect(section.getByRole('switch')).toHaveAttribute('aria-checked', 'true');

    await section.locator('input[type="time"]').nth(0).fill('09:00');
    await section.locator('input[type="time"]').nth(1).fill('18:00');
    await section.locator('select').selectOption({ value: 'UTC' });
    // Снять «Сб» и «Вс»
    await section.getByRole('button', { name: 'Сб', exact: true }).click();
    await section.getByRole('button', { name: 'Вс', exact: true }).click();

    await routing.saveAsDraft();
    await expect(page.getByRole('dialog')).toBeHidden();
    await routing.expectRouteInTable(name);

    const list = await api.listRoutes();
    const created = (list.routes ?? []).find((r: { name: string }) => r.name === name);
    expect(created).toBeTruthy();
    const full = await api.getRoute(created.id);
    const sch = full.schedules?.[0];
    expect(sch).toBeTruthy();
    expect(sch.timezone).toBe('UTC');
    expect(sch.time_from).toBe('09:00');
    expect(sch.time_to).toBe('18:00');
    // Пн|Вт|Ср|Чт|Пт = 1+2+4+8+16 = 31
    expect(sch.weekdays).toBe(31);
  });
});
