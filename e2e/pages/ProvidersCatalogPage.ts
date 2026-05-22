import { type Page, expect } from '@playwright/test';

/**
 * Page-object for /network/providers (Plan 1).
 * Provider catalog: список платформенных + private провайдеров,
 * drawer для создания своего провайдера.
 */
export class ProvidersCatalogPage {
  constructor(private readonly page: Page) {}

  async goto() {
    await this.page.goto('/network/providers');
  }

  async expectLoaded() {
    await expect(this.page.locator('h1, h2').filter({ hasText: 'Провайдеры' }).first()).toBeVisible();
  }

  /** Открывает drawer "Добавить своего", заполняет минимальные поля и сохраняет. */
  async addPrivate(name: string, host: string) {
    await this.page.click('button:has-text("+ Добавить своего")');
    await expect(this.page.locator('text=Добавить своего провайдера')).toBeVisible();

    // Drawer fields, scoped к нему.
    const dialog = this.page.locator('[role="dialog"], aside').filter({ hasText: 'Добавить своего провайдера' }).first();
    await dialog.locator('label:has-text("Имя") + input, label:has-text("Имя") input').first().fill(name);
    await dialog.locator('label:has-text("SMPP host") + input, label:has-text("SMPP host") input').first().fill(host);
    await dialog.locator('label:has-text("System ID") + input, label:has-text("System ID") input').first().fill('e2e-system');
    await dialog.locator('label:has-text("Password") + input, label:has-text("Password") input').first().fill('e2e-pass');

    await dialog.locator('button:has-text("Сохранить")').click();
    // Toast "Создан"
    await expect(this.page.locator('text=Создан').first()).toBeVisible({ timeout: 8_000 });
  }

  async expectRow(name: string) {
    await expect(this.page.locator(`tr:has-text("${name}")`).first()).toBeVisible({ timeout: 8_000 });
  }
}
