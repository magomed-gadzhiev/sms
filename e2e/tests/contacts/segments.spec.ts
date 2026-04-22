import { test, expect } from '@playwright/test';
import { SegmentsPage } from '../../pages/SegmentsPage';

test.use({ storageState: './auth-state.json' });

function uniqueName(prefix: string) {
  return `${prefix} ${Date.now().toString(36)}`;
}

test.describe('Сегменты (/segments)', () => {
  test('AC-D1 открытие списка: заголовок и кнопка создания', async ({ page }) => {
    const segments = new SegmentsPage(page);
    await segments.goto();
    await expect(page.getByText('Сохранённые сегменты для кампаний')).toBeVisible();
    await expect(page.getByRole('link', { name: 'Создать сегмент' })).toBeVisible();
  });

  test('AC-D2/D3/D4/D5 создать сегмент → redirect на детали → Пересчитать → найти в списке', async ({ page }) => {
    const segments = new SegmentsPage(page);
    const name = uniqueName('E2E Segment');
    const description = 'auto-generated';

    await segments.goto();

    // AC-D2: клик "Создать сегмент" → /segments/new + форма сегмента загрузилась
    await segments.clickCreate();
    await expect(page.getByPlaceholder('Название')).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Правила фильтрации' })).toBeVisible();

    // AC-D3: заполнение + сохранение → redirect /segments/:id, заголовок = name
    await segments.fillName(name);
    await segments.fillDescription(description);
    await segments.save();
    await expect(page).toHaveURL(/\/segments\/[a-f0-9-]+$/, { timeout: 10_000 });
    await segments.expectDetailLoaded(name);

    // AC-D4: на детальной странице видна кнопка "Пересчитать"
    await expect(segments.recalcButton()).toBeVisible();

    // AC-D5: сегмент виден в общем списке
    await segments.goto();
    await segments.expectSegmentVisible(name);
  });
});
