import { type Page, expect } from '@playwright/test';

export class SegmentsPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/segments', { waitUntil: 'domcontentloaded' });
    await expect(
      this.page.getByRole('heading', { level: 1, name: 'Сегменты' }),
    ).toBeVisible();
  }

  async clickCreate() {
    await this.page.getByRole('link', { name: 'Создать сегмент' }).click();
    await expect(this.page).toHaveURL(/\/segments\/new$/);
  }

  segmentRow(name: string) {
    return this.page.locator('a[href^="/segments/"]').filter({ hasText: name });
  }

  async expectSegmentVisible(name: string) {
    await expect(this.segmentRow(name)).toBeVisible();
  }

  // --- Detail (new & existing) ---

  async fillName(name: string) {
    await this.page.getByPlaceholder('Название').fill(name);
  }

  async fillDescription(description: string) {
    await this.page.getByPlaceholder('Описание').fill(description);
  }

  async save() {
    await this.page.getByRole('button', { name: /Сохранить|Сохранение/ }).click();
  }

  async expectDetailLoaded(name: string) {
    await expect(
      this.page.getByRole('heading', { level: 1, name }),
    ).toBeVisible();
  }

  recalcButton() {
    return this.page.getByRole('button', { name: 'Пересчитать' });
  }
}
