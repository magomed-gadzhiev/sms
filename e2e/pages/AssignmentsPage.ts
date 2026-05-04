import { type Page, expect } from '@playwright/test';

/**
 * Page-object for /network/assignments (Plan 1).
 * Таблица суб-аккаунтов с inline-select provider-set'а на строку
 * + sticky bulk-panel при выделении.
 */
export class AssignmentsPage {
  constructor(private readonly page: Page) {}

  async goto() {
    await this.page.goto('/network/assignments');
  }

  async expectLoaded() {
    await expect(this.page.locator('h1, h2').filter({ hasText: 'Назначения суб-аккаунтам' }).first()).toBeVisible();
  }

  /** Назначает provider-set первому суб-аккаунту через inline-select. */
  async assignFirstSubAccount(providerSetName: string) {
    const firstRow = this.page.locator('tbody tr').first();
    await expect(firstRow).toBeVisible({ timeout: 8_000 });
    const select = firstRow.locator('select');
    await select.selectOption({ label: new RegExp(escapeRegex(providerSetName)) });
    // Toast "Назначение сохранено"
    await expect(this.page.locator('text=Назначение сохранено').first()).toBeVisible({ timeout: 8_000 });
  }

  /** Имя суб-аккаунта в первой строке (для последующего ассерта). */
  firstSubAccountCell() {
    return this.page.locator('tbody tr').first().locator('td').nth(1);
  }
}

function escapeRegex(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}
