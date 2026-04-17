import { type Page, expect } from '@playwright/test';

export class LoginPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/login');
  }

  async login(email: string, password: string) {
    await this.page.fill('input[type="email"], [name="email"]', email);
    await this.page.fill('input[type="password"], [name="password"]', password);
    await this.page.click('button[type="submit"]');
  }

  async expectLoggedIn() {
    await expect(this.page).toHaveURL(/\/command-center/);
  }

  async expectError(text: string) {
    await expect(this.page.locator('[role="alert"]')).toContainText(text);
  }
}
