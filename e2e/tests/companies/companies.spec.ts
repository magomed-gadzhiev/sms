import { test, expect } from '@playwright/test';
import { CompaniesPage, CompanyDetailPage } from '../../pages/CompaniesPage';
import { ApiHelper } from '../../helpers/api';

test.use({ storageState: './auth-state.json' });

const COMP_PREFIX = 'E2E-Comp-';
// Synthetic 10-digit ИНН with valid checksum (not a real entity).
// Checksum per internal/services/company/domain/models.go ValidateINN:
// weights [2,4,10,3,5,9,4,6,8] for digits 1-9: 2+8+30+12+25+54+28+48+72 = 279; (279 % 11) % 10 = 4.
const VALID_INN_10 = '1234567894';

function uniqueName() {
  return `${COMP_PREFIX}${Date.now()}-${Math.floor(Math.random() * 1000)}`;
}

test.describe('Companies — Мои компании', () => {
  test.beforeEach(async ({ request }) => {
    const api = new ApiHelper(request);
    await api.cleanupTestCompanies(COMP_PREFIX);
  });

  test.afterEach(async ({ request }) => {
    const api = new ApiHelper(request);
    await api.cleanupTestCompanies(COMP_PREFIX);
  });

  // ── Phase A: seed-free (Оферта + валидации) ──

  test('AC1 — список показывает компанию Оферта с бейджами "Оферта" и "Основная"', async ({ page }) => {
    const comps = new CompaniesPage(page);
    await comps.goto();
    await expect(page.locator('h1')).toHaveText(/Мои компании/);
    const offerRow = comps.row('Оферта');
    await expect(offerRow).toBeVisible();
    // Name «Оферта» + Badge «Оферта» = 2 occurrences; Badge «Основная» = 1
    await expect(offerRow.getByText('Оферта', { exact: true })).toHaveCount(2);
    await expect(offerRow.getByText('Основная', { exact: true })).toBeVisible();
  });

  test('AC2 — детальная Оферты: синий блок, без формы реквизитов', async ({ page }) => {
    const comps = new CompaniesPage(page);
    const detail = new CompanyDetailPage(page);
    await comps.goto();
    await comps.openDetailsFor('Оферта');
    await expect(page.locator('h1')).toHaveText('Оферта');
    await expect(page.locator('text=Работа по договору-оферте')).toBeVisible();
    await expect(detail.offerNotice).toBeVisible();
    // Форма реквизитов отсутствует
    await expect(page.locator('text=Основные реквизиты')).toHaveCount(0);
    await expect(detail.saveButton).toHaveCount(0);
  });

  test('AC3 — у Оферты (is_default) отсутствует кнопка "Отвязать"', async ({ page }) => {
    const comps = new CompaniesPage(page);
    const detail = new CompanyDetailPage(page);
    await comps.goto();
    await comps.openDetailsFor('Оферта');
    await expect(detail.detachButton).toHaveCount(0);
  });

  test('AC4 — модалка create: пустое имя блокирует отправку', async ({ page }) => {
    const comps = new CompaniesPage(page);
    await comps.goto();
    await comps.openCreateModal();
    let createPosted = false;
    page.on('request', (req) => {
      if (req.method() === 'POST' && req.url().endsWith('/companies')) createPosted = true;
    });
    // ИНН пустой, имя пустое
    await comps.submitButton.click();
    await expect(comps.formError).toContainText('Наименование обязательно');
    await expect(comps.modal).toBeVisible();
    expect(createPosted).toBe(false);
  });

  test('AC5 — валидация ИНН: нечисловой ввод', async ({ page }) => {
    const comps = new CompaniesPage(page);
    await comps.goto();
    await comps.openCreateModal();
    let createPosted = false;
    page.on('request', (req) => {
      if (req.method() === 'POST' && req.url().endsWith('/companies')) createPosted = true;
    });
    await comps.fillAndSubmit({ inn: 'abc123', name: 'ООО Тест' });
    await expect(comps.formError).toContainText('ИНН должен содержать только цифры');
    expect(createPosted).toBe(false);
  });

  test('AC6 — валидация ИНН: неверная длина (5 цифр)', async ({ page }) => {
    const comps = new CompaniesPage(page);
    await comps.goto();
    await comps.openCreateModal();
    let createPosted = false;
    page.on('request', (req) => {
      if (req.method() === 'POST' && req.url().endsWith('/companies')) createPosted = true;
    });
    await comps.fillAndSubmit({ inn: '12345', name: 'ООО Тест' });
    await expect(comps.formError).toContainText('ИНН должен быть 10 или 12 цифр');
    expect(createPosted).toBe(false);
  });

  // ── Phase B: create / set-default / save / detach ──

  test('AC7 — создание компании с валидным ИНН (10 цифр) добавляет строку', async ({ page }) => {
    const comps = new CompaniesPage(page);
    const name = uniqueName();
    await comps.goto();
    await comps.openCreateModal();
    await comps.fillAndSubmit({ inn: VALID_INN_10, name });
    await expect(comps.modal).toBeHidden();
    const row = comps.row(name);
    await expect(row).toBeVisible();
    await expect(row).toContainText(VALID_INN_10);
    // Не основная → есть кнопка "Сделать основной"
    await expect(row.getByRole('button', { name: 'Сделать основной' })).toBeVisible();
    await expect(row.getByText('Основная', { exact: true })).toHaveCount(0);
  });

  test('AC8 — "Сделать основной" меняет статус default-компании', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const comps = new CompaniesPage(page);
    const name = uniqueName();
    await comps.goto();
    await comps.openCreateModal();
    await comps.fillAndSubmit({ inn: VALID_INN_10, name });
    await expect(comps.row(name)).toBeVisible();

    await comps.setDefault(name);

    // После reload новая компания — «Основная», у Оферты появляется «Сделать основной»
    await expect(comps.row(name).getByText('Основная', { exact: true })).toBeVisible();
    await expect(
      comps.row('Оферта').getByRole('button', { name: 'Сделать основной' }),
    ).toBeVisible();

    // Cleanup: вернуть Оферту в default, иначе detach теста не пройдёт
    const { companies } = await api.listCompanies();
    const offer = companies.find((c) => c.is_offer);
    if (offer) await api.setDefaultCompany(offer.id);
  });

  test('AC9 — сохранение реквизитов показывает "Реквизиты сохранены" и персистентно', async ({ page }) => {
    const comps = new CompaniesPage(page);
    const detail = new CompanyDetailPage(page);
    const name = uniqueName();

    await comps.goto();
    await comps.openCreateModal();
    await comps.fillAndSubmit({ inn: VALID_INN_10, name });
    await expect(comps.row(name)).toBeVisible();
    await comps.openDetailsFor(name);

    await detail.fillField(/Полное наименование/, 'Общество с ограниченной ответственностью E2E');
    await detail.fillField(/КПП/, '770701001');
    await detail.fillField(/ФИО руководителя/, 'Иванов И. И.');
    await detail.save();

    await expect(detail.successBanner).toContainText('Реквизиты сохранены');

    // Персистентность: перезагрузить и прочитать значения
    await page.reload();
    await expect(page.locator('h1')).toHaveText(name);
    expect(await detail.readField(/Полное наименование/)).toBe(
      'Общество с ограниченной ответственностью E2E',
    );
    expect(await detail.readField(/КПП/)).toBe('770701001');
    expect(await detail.readField(/ФИО руководителя/)).toBe('Иванов И. И.');
  });

  test('AC10 — отвязка non-default компании удаляет её из списка', async ({ page }) => {
    const comps = new CompaniesPage(page);
    const detail = new CompanyDetailPage(page);
    const name = uniqueName();

    await comps.goto();
    await comps.openCreateModal();
    await comps.fillAndSubmit({ inn: VALID_INN_10, name });
    await expect(comps.row(name)).toBeVisible();
    await comps.openDetailsFor(name);

    await expect(detail.detachButton).toBeVisible();
    await detail.detach();

    await page.waitForURL(/\/companies$/);
    await comps.expectRowMissing(name);
  });

  test.fixme('AC11 — empty state (требует fixture без автосоздания Оферты)', async () => {
    // Не воспроизводим: у каждого клиента миграцией создаётся компания "Оферта" (is_offer=true).
    // Снять fixme, когда появится fixture-юзер без автосозданной Оферты.
  });
});
