import { type Page, type Locator, expect } from '@playwright/test';

export class CompaniesPage {
  readonly page: Page;
  readonly addButton: Locator;
  readonly modal: Locator;
  readonly innInput: Locator;
  readonly nameInput: Locator;
  readonly submitButton: Locator;
  readonly formError: Locator;

  constructor(page: Page) {
    this.page = page;
    this.addButton = page.getByRole('button', { name: /Добавить (первую )?компанию/ });
    this.modal = page.getByRole('dialog');
    this.innInput = this.modal.getByLabel('ИНН', { exact: false });
    this.nameInput = this.modal.getByLabel(/Краткое наименование/);
    this.submitButton = this.modal.getByRole('button', { name: /^(Создать|Создание\.\.\.)$/ });
    this.formError = this.modal.locator('p.text-red-600');
  }

  async goto() {
    await this.page.goto('/companies');
    await expect(this.page.locator('h1')).toContainText('Мои компании');
  }

  row(name: string): Locator {
    return this.page.locator('tbody tr').filter({ hasText: name });
  }

  async openCreateModal() {
    await this.addButton.first().click();
    await expect(this.modal).toBeVisible();
  }

  async fillAndSubmit({ inn, name }: { inn?: string; name: string }) {
    if (inn !== undefined) await this.innInput.fill(inn);
    await this.nameInput.fill(name);
    await this.submitButton.click();
  }

  async openDetailsFor(companyName: string) {
    const row = this.row(companyName);
    await row.getByRole('button', { name: 'Реквизиты' }).click();
    await this.page.waitForURL(/\/companies\/[0-9a-f-]+$/);
  }

  async setDefault(companyName: string) {
    await this.row(companyName).getByRole('button', { name: 'Сделать основной' }).click();
  }

  async expectDefaultBadge(companyName: string) {
    await expect(this.row(companyName).getByText('Основная', { exact: true })).toBeVisible();
  }

  async expectOfferBadge(companyName: string) {
    // Name «Оферта» + Badge «Оферта» → expect exactly 2 occurrences in the row
    await expect(this.row(companyName).getByText('Оферта', { exact: true })).toHaveCount(2);
  }

  async expectRowMissing(companyName: string, timeout = 5000) {
    await expect(this.row(companyName)).toHaveCount(0, { timeout });
  }
}

export class CompanyDetailPage {
  readonly page: Page;
  readonly offerNotice: Locator;
  readonly backButton: Locator;
  readonly detachButton: Locator;
  readonly saveButton: Locator;
  readonly successBanner: Locator;
  readonly errorBanner: Locator;

  constructor(page: Page) {
    this.page = page;
    this.offerNotice = page.locator('div.bg-blue-50', { hasText: 'договору-оферте' });
    this.backButton = page.getByRole('button', { name: '← Назад' });
    this.detachButton = page.getByRole('button', { name: /Отвязать компанию|Отвязка\.\.\./ });
    this.saveButton = page.getByRole('button', { name: /Сохранить реквизиты|Сохранение\.\.\./ });
    this.successBanner = page.locator('div.bg-green-50');
    this.errorBanner = page.locator('div.bg-red-50');
  }

  field(label: string | RegExp): Locator {
    return this.page.getByLabel(label);
  }

  async fillField(label: string | RegExp, value: string) {
    await this.field(label).fill(value);
  }

  async readField(label: string | RegExp): Promise<string> {
    return this.field(label).inputValue();
  }

  async save() {
    await this.saveButton.click();
  }

  async detach() {
    this.page.once('dialog', (d) => d.accept());
    await this.detachButton.click();
  }
}
