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

  // Quick filters (row 2)
  loginInput(): Locator {
    return this.page.getByPlaceholder('Логин');
  }

  operatorSelect(): Locator {
    return this.page.locator('select').filter({ has: this.page.locator('option', { hasText: /^Оператор$/ }) });
  }

  channelSelect(): Locator {
    return this.page.locator('select').filter({ has: this.page.locator('option', { hasText: /^Канал$/ }) });
  }

  serviceTypeSelect(): Locator {
    return this.page.locator('select').filter({ has: this.page.locator('option', { hasText: /^Тип услуги$/ }) });
  }

  // Advanced filters (7 fields under "Ещё фильтры")
  trafficTypeSelect(): Locator {
    return this.page.locator('select').filter({ has: this.page.locator('option', { hasText: /^Тип трафика$/ }) });
  }

  statusSelect(): Locator {
    return this.page.locator('select').filter({ has: this.page.locator('option', { hasText: /^Статус$/ }) });
  }

  providerInput(): Locator {
    return this.page.getByPlaceholder('Провайдер');
  }

  countryInput(): Locator {
    return this.page.getByPlaceholder('Страна');
  }

  managerInput(): Locator {
    return this.page.getByPlaceholder('Менеджер');
  }

  errorCodeInput(): Locator {
    return this.page.getByPlaceholder('Код ошибки');
  }

  // Error toast (fixed bottom-right, red)
  errorToast(): Locator {
    return this.page.locator('.fixed.bottom-4.right-4.bg-red-50');
  }

  // --- Statistics table ---

  // Main table container (wraps thead/tbody)
  // TODO(phase-2): DrillDownDrawer's internal table also has <th>Срез</th>. When drill-down
  // tests open the drawer, this locator will match 2 tables. Scope to mode-container or
  // use .first() with explicit "drawer closed" precondition.
  statisticsTable(): Locator {
    return this.page.locator('table').filter({ has: this.page.locator('th', { hasText: 'Срез' }) });
  }

  tableHeaders(): Locator {
    return this.statisticsTable().locator('thead th');
  }

  tableHeader(text: string): Locator {
    return this.statisticsTable().locator('thead th', { hasText: new RegExp(`^${text}$`) });
  }

  // The last th (health icon column) has no text
  healthHeader(): Locator {
    return this.statisticsTable().locator('thead th').last();
  }

  tableRows(): Locator {
    return this.statisticsTable().locator('tbody tr');
  }

  tableEmptyCell(): Locator {
    return this.statisticsTable().locator('tbody td', { hasText: 'Нет данных' });
  }

  tableLoadingCell(): Locator {
    return this.statisticsTable().locator('tbody td', { hasText: 'Загрузка...' });
  }

  // Pagination block (below table)
  paginationBlock(): Locator {
    // The pagination div is the flex with "Страница X из Y" text
    return this.page.locator('div.flex').filter({ hasText: /^Страница\s+\d+\s+из\s+\d+$/ });
  }

  // --- Monitoring mode ---

  // MonitoringKPIGrid: grid-cols-4 container
  monitoringKpiGrid(): Locator {
    return this.page.locator('div.grid.grid-cols-4');
  }

  // MonitoringTable: <table> containing <th>Провайдер</th>
  monitoringTable(): Locator {
    return this.page.locator('table').filter({ has: this.page.locator('th', { hasText: 'Провайдер' }) });
  }

  monitoringTableHeaders(): Locator {
    return this.monitoringTable().locator('thead th');
  }

  // Live indicator block (shown only in monitoring mode with pulse dot)
  liveIndicator(): Locator {
    return this.page.locator('span', { hasText: /Обновлено\s+\d+\s+сек\.\s+назад/ });
  }

  // Pause / Resume toggle button (in filter bar, monitoring mode only)
  pauseButton(): Locator {
    return this.page.getByRole('button', { name: 'Пауза', exact: true });
  }

  resumeButton(): Locator {
    return this.page.getByRole('button', { name: 'Продолжить', exact: true });
  }
}
