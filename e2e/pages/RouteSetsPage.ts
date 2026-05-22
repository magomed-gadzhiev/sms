import { type Page, expect } from '@playwright/test';

/**
 * Page-object for /network/route-sets (Plan 2).
 * Левый список шаблонов + правая панель с правилами + drawer для создания/редактирования правила.
 *
 * DOM-ссылки: portal-frontend/src/pages/network/RouteSetsPage.tsx,
 * components/network/RouteRuleDrawer.tsx, components/network/RouteConditionsEditor.tsx.
 */
export class RouteSetsPage {
  constructor(private readonly page: Page) {}

  async goto() {
    await this.page.goto('/network/route-sets');
  }

  async expectLoaded() {
    await expect(this.page.locator('h1, h2').filter({ hasText: 'Route-sets' }).first()).toBeVisible();
  }

  /** Создаёт новый шаблон через "+ Создать" модал. */
  async create(name: string) {
    await this.page.click('button:has-text("+ Создать")');
    const modal = this.page.locator('[role="dialog"]').filter({ hasText: 'Создать route-set' }).first();
    await expect(modal).toBeVisible();
    await modal.locator('label:has-text("Имя") + input, label:has-text("Имя") input').first().fill(name);
    await modal.locator('button:has-text("Создать")').click();
    await expect(this.page.locator('text=Создан').first()).toBeVisible({ timeout: 8_000 });
    // После create страница автоматически выбирает новый set — toggle "По умолчанию" появляется в правой панели.
    await expect(this.page.locator('text=По умолчанию для новых суб-аккаунтов').first()).toBeVisible();
  }

  /** Кликает по элементу левого списка. */
  async select(name: string) {
    await this.page.locator('aside button').filter({ hasText: name }).first().click();
    await expect(this.page.locator('text=По умолчанию для новых суб-аккаунтов').first()).toBeVisible();
  }

  /** Открывает drawer "+ Правило", заполняет, опционально добавляет country-условие, сохраняет. */
  async addRule(opts: { name: string; provider: string | RegExp; country?: string }) {
    await this.page.click('button:has-text("+ Правило")');
    const drawer = this.page.locator('[role="dialog"], aside').filter({ hasText: 'Новое правило' }).first();
    await expect(drawer).toBeVisible();

    await drawer.locator('label:has-text("Имя") + input, label:has-text("Имя") input').first().fill(opts.name);

    const providerLabel = typeof opts.provider === 'string' ? new RegExp(escapeRegex(opts.provider)) : opts.provider;
    // Drawer содержит несколько select'ов (status и т.п.). Берём первый — он провайдерский.
    await drawer.locator('select').first().selectOption({ label: providerLabel });

    if (opts.country) {
      // Добавляем группу условий (по умолчанию первое условие — type=country).
      await drawer.locator('button:has-text("+ Группа условий")').click();
      const conditionInput = drawer.locator('input[placeholder*="RU"]').first();
      await expect(conditionInput).toBeVisible();
      await conditionInput.fill(opts.country);
    }

    await drawer.locator('button:has-text("Сохранить")').click();
    await expect(this.page.locator('text=Создано').first()).toBeVisible({ timeout: 8_000 });
  }

  /** Ассёртит, что строка с именем правила появилась в таблице правил. */
  async expectRuleRow(name: string) {
    await expect(this.page.locator(`tbody tr:has-text("${name}")`).first()).toBeVisible({ timeout: 8_000 });
  }

  /** Раскрывает details "Превью маршрутизации", заполняет и нажимает "Симулировать". */
  async runPreview(phone: string, sender: string, traffic: string) {
    await this.page.locator('summary:has-text("Превью маршрутизации")').click();
    await this.page.locator('label:has-text("Телефон") + input, label:has-text("Телефон") input').first().fill(phone);
    await this.page.locator('label:has-text("Sender ID") + input, label:has-text("Sender ID") input').first().fill(sender);
    // Traffic-type select определяется по соседнему label.
    const trafficLabel = this.page.locator('label:has-text("Traffic type")').first();
    await trafficLabel.locator('xpath=following-sibling::select[1]').selectOption(traffic);
    await this.page.locator('button:has-text("Симулировать")').click();
  }
}

function escapeRegex(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}
