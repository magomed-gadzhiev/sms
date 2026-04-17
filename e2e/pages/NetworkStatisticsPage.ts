import { type Page, type Locator, expect } from '@playwright/test';

export class NetworkStatisticsPage {
  constructor(private page: Page) {}

  async goto(query: string = '') {
    const url = query ? `/network/statistics${query}` : '/network/statistics';
    await this.page.goto(url);
  }

  async expectLoaded() {
    await expect(this.page.getByRole('tab', { name: 'Статистика' })).toBeVisible({ timeout: 10_000 });
  }

  // Tabs
  tab(name: 'Статистика' | 'Аналитика' | 'Мониторинг'): Locator {
    return this.page.getByRole('tab', { name });
  }

  // Filter bar — presets
  periodPreset(label: string): Locator {
    return this.page.getByRole('button', { name: label, exact: true });
  }

  // Apply button
  applyButton(): Locator {
    return this.page.getByRole('button', { name: 'Применить', exact: true });
  }

  // "Ещё фильтры" toggle
  extraFiltersToggle(): Locator {
    return this.page.getByRole('button', { name: /Ещё фильтры|Свернуть/ });
  }

  // Grouping dropdown (a native <select>)
  groupingSelect(): Locator {
    // Grouping <select> is the one whose options contain "по дням"
    return this.page.locator('select').filter({ hasText: 'по дням' });
  }

  // Advanced filter field — "Имя отправителя"
  senderNameField(): Locator {
    return this.page.getByPlaceholder('Имя отправителя');
  }

  // Date picker — opens custom date range popup
  datePickerTrigger(): Locator {
    // Button shows "Произвольный" when no range selected, or "YYYY-MM-DD — YYYY-MM-DD" when set
    return this.page.getByRole('button', { name: /Произвольный|\d{4}-\d{2}-\d{2}\s*—\s*\d{4}-\d{2}-\d{2}/ });
  }

  datePickerPopup(): Locator {
    // Unique class combination: absolute positioning + top-full + z-50 is only the date picker popup
    return this.page.locator('div.top-full.z-50');
  }

  datePickerFromInput(): Locator {
    return this.datePickerPopup().locator('input[type="date"]').first();
  }

  datePickerToInput(): Locator {
    return this.datePickerPopup().locator('input[type="date"]').last();
  }

  datePickerApply(): Locator {
    return this.datePickerPopup().getByRole('button', { name: 'Применить' });
  }
}
