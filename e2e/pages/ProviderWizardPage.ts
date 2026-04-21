import { type Page, expect } from '@playwright/test';

export class ProviderWizardPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/providers/new');
    await expect(this.page.locator('text=Добавить SMPP провайдера')).toBeVisible();
  }

  // ── Step 1: Basic Info ─────────────────────────────────────────────────
  async fillStep1(name: string) {
    await this.page.locator('input[placeholder="My SMPP Provider"]').fill(name);
  }

  // ── Step 2: Connection ─────────────────────────────────────────────────
  async fillStep2(opts: { host: string; port: number; systemId: string; password: string }) {
    await this.page.locator('input[placeholder="smpp.example.com"]').fill(opts.host);
    await this.page.locator('input[placeholder="2775"]').fill(String(opts.port));
    await this.page.locator('input[placeholder="smppclient1"]').fill(opts.systemId);
    await this.page.locator('input[placeholder="••••••••"]').fill(opts.password);
  }

  // ── Navigation ─────────────────────────────────────────────────────────
  nextButton() {
    return this.page.locator('button:has-text("Next →")');
  }

  backButton() {
    return this.page.locator('button:has-text("← Back")');
  }

  async clickNext() {
    await this.nextButton().click();
  }

  async clickBack() {
    await this.backButton().click();
  }

  /**
   * Заполняет минимально-валидные шаги 1-2 и жмёт Next до шага 5.
   */
  async gotoStep5() {
    await this.goto();
    await this.fillStep1('E2E Provider ' + Date.now().toString(36));
    await this.clickNext(); // 1 → 2
    await this.fillStep2({
      host: 'smpp.example.test',
      port: 2775,
      systemId: 'e2e-smppclient',
      password: 'e2e-password',
    });
    await this.clickNext(); // 2 → 3
    await this.clickNext(); // 3 → 4
    await this.clickNext(); // 4 → 5
    await this.expectOnStep5();
  }

  // ── Step 5: Routing Rules ──────────────────────────────────────────────
  async expectOnStep5() {
    await expect(this.page.locator('h3:has-text("Routing Rules")')).toBeVisible();
  }

  addRuleButton() {
    return this.page.locator('button:has-text("+ Add Rule")');
  }

  async clickAddRule() {
    await this.addRuleButton().click();
  }

  patternInput(index: number) {
    return this.page.locator('input[placeholder="^7.*"]').nth(index);
  }

  priorityInput(index: number) {
    // Правило — это flex-row с pattern input + number input + × button.
    // Используем nth(index) по number input (type=number + placeholder="Priority").
    return this.page.locator('input[type="number"][placeholder="Priority"]').nth(index);
  }

  removeRuleButton(index: number) {
    return this.page.locator('button:has-text("×")').nth(index);
  }

  async countRules(): Promise<number> {
    return this.page.locator('input[placeholder="^7.*"]').count();
  }
}
