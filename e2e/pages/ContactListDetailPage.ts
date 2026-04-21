import { type Page, expect } from '@playwright/test';

export class ContactListDetailPage {
  constructor(private page: Page) {}

  async expectLoaded(title: string) {
    await expect(
      this.page.locator('h1, [class*="PageHeader"]').filter({ hasText: title }).first(),
    ).toBeVisible();
    await expect(
      this.page.getByLabel('Breadcrumb').getByRole('link', { name: 'Контактные базы' }),
    ).toBeVisible();
  }

  async expectActionButtons() {
    await expect(this.page.getByRole('button', { name: 'Настройки атрибутов' })).toBeVisible();
    await expect(this.page.getByRole('button', { name: 'Импорт', exact: true })).toBeVisible();
    await expect(this.page.getByRole('button', { name: '+ Добавить контакт' })).toBeVisible();
  }

  async expectEmptyContacts() {
    await expect(this.page.getByText('Контакты не добавлены')).toBeVisible();
  }

  async expectHeaders(headers: string[]) {
    for (const h of headers) {
      await expect(this.page.locator('table thead th').filter({ hasText: h }).first()).toBeVisible();
    }
  }

  async openAddContact() {
    await this.page.locator('button:has-text("+ Добавить контакт")').click();
    await expect(
      this.addDialog().getByRole('heading', { name: 'Добавить контакт' }),
    ).toBeVisible();
  }

  addDialog() {
    return this.page.locator('[role="dialog"]').filter({ hasText: 'Добавить контакт' });
  }

  async fillContact(phone: string, tags?: string) {
    const dialog = this.addDialog();
    await dialog.getByPlaceholder('+79001234567').fill(phone);
    if (tags) {
      await dialog.getByPlaceholder('vip, москва').fill(tags);
    }
  }

  async submitAddContact() {
    const dialog = this.addDialog();
    const alert = dialog.locator('[role="alert"]');
    await dialog.locator('button:has-text("Добавить")').click();
    const hidden = dialog.waitFor({ state: 'hidden', timeout: 10_000 });
    const errored = alert.waitFor({ state: 'visible', timeout: 10_000 });
    hidden.catch(() => {});
    errored.catch(() => {});
    const winner = await Promise.race([
      hidden.then(() => 'hidden' as const),
      errored.then(() => 'errored' as const),
    ]);
    if (winner === 'errored') {
      const msg = await alert.textContent();
      throw new Error(`Add contact failed: ${msg}`);
    }
  }

  contactRow(phone: string) {
    return this.page.locator('table tbody tr').filter({ hasText: phone });
  }

  async expectContactRow(phone: string) {
    await expect(this.contactRow(phone)).toBeVisible();
  }

  async expectTag(phone: string, tag: string) {
    await expect(this.contactRow(phone).getByText(tag, { exact: true })).toBeVisible();
  }

  async search(query: string) {
    await this.page.locator('input#contact-search').fill(query);
    await this.page.locator('button:has-text("Найти")').click();
  }

  async expectNotFound() {
    await expect(this.page.getByText('По запросу ничего не найдено')).toBeVisible();
  }

  async deleteContact(phone: string) {
    await this.contactRow(phone).locator('button:has-text("Удалить")').click();
    const confirm = this.page.locator('[role="dialog"]').filter({ hasText: 'Удалить контакт' });
    await confirm.locator('button:has-text("Удалить")').click();
    await expect(confirm).toBeHidden();
  }

  async addContactViaUI(phone: string, tags?: string) {
    await this.openAddContact();
    await this.fillContact(phone, tags);
    await this.submitAddContact();
    await this.expectContactRow(phone);
  }

  // --- Attributes modal ---

  async openAttrsModal() {
    await this.page.getByRole('button', { name: 'Настройки атрибутов' }).click();
    await expect(
      this.attrsDialog().getByRole('heading', { name: 'Настройки атрибутов' }),
    ).toBeVisible();
  }

  attrsDialog() {
    return this.page.locator('[role="dialog"]').filter({ hasText: 'Настройки атрибутов' });
  }

  attrRow(index: number) {
    return this.attrsDialog().locator('.border.border-gray-200.rounded.p-3').nth(index);
  }

  async addAttributeRow() {
    await this.attrsDialog().getByRole('button', { name: '+ Добавить атрибут' }).click();
  }

  async fillAttribute(
    index: number,
    values: { name?: string; displayName?: string; type?: 'string' | 'number' | 'date' | 'boolean' },
  ) {
    const row = this.attrRow(index);
    if (values.name !== undefined) {
      await row.getByPlaceholder('Системное имя').fill(values.name);
    }
    if (values.displayName !== undefined) {
      await row.getByPlaceholder('Отображаемое имя').fill(values.displayName);
    }
    if (values.type !== undefined) {
      await row.locator('select').selectOption(values.type);
    }
  }

  async removeAttributeRow(index: number) {
    await this.attrRow(index).getByRole('button', { name: 'Удалить атрибут' }).click();
  }

  async saveAttributes() {
    await this.attrsDialog().getByRole('button', { name: 'Сохранить' }).click();
    await expect(this.attrsDialog()).toBeHidden();
  }

  async cancelAttributes() {
    await this.attrsDialog().getByRole('button', { name: 'Отмена' }).click();
    await expect(this.attrsDialog()).toBeHidden();
  }
}
