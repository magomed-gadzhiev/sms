import { test, expect } from '@playwright/test';
import { BillingPage } from '../../pages/BillingPage';
import { ApiHelper } from '../../helpers/api';

test.use({ storageState: './auth-state.json' });

test.describe('Billing — Баланс и транзакции', () => {

  test('страница биллинга загружается и показывает баланс', async ({ page }) => {
    const billing = new BillingPage(page);
    await billing.goto();
    const balance = await billing.getBalance();
    expect(balance).toBeTruthy();
  });

  test('отображается история транзакций', async ({ page }) => {
    const billing = new BillingPage(page);
    await billing.goto();
    await billing.expectTransactionsTable();
  });

  test('фильтрация транзакций по типу "charge" (списание)', async ({ page }) => {
    const billing = new BillingPage(page);
    await billing.goto();
    await billing.expectTransactionsTable();
    await billing.filterTransactionsByType('charge');
    // Should not error, may have 0 results
  });

  test('фильтрация транзакций по типу "credit" (пополнение)', async ({ page }) => {
    const billing = new BillingPage(page);
    await billing.goto();
    await billing.expectTransactionsTable();
    await billing.filterTransactionsByType('credit');
  });

  test('настройка порога уведомления сохраняется', async ({ page }) => {
    const billing = new BillingPage(page);
    await billing.goto();
    await billing.expectThresholdSection();
    await billing.setThreshold('100');
    await billing.expectThresholdSaved();
  });

  test('баланс через API совпадает с отображаемым на странице', async ({ page, request }) => {
    const api = new ApiHelper(request);
    const apiBalance = await api.getBalance();

    const billing = new BillingPage(page);
    await billing.goto();
    const uiBalance = await billing.getBalanceNumber();
    const apiBalanceValue = parseFloat(apiBalance.balance);

    // Allow small rounding difference
    expect(Math.abs(uiBalance - apiBalanceValue)).toBeLessThan(0.1);
  });

  test('кнопка пополнения открывает модальное окно', async ({ page }) => {
    const billing = new BillingPage(page);
    await billing.goto();
    await page.click('button:has-text("Пополнить")');
    await expect(page.locator('text=Пополнение баланса')).toBeVisible();
    // Close without submitting
    await page.click('button:has-text("Отмена")');
  });

  test('валидация суммы пополнения: 0 → кнопка заблокирована', async ({ page }) => {
    const billing = new BillingPage(page);
    await billing.goto();
    await page.click('button:has-text("Пополнить")');
    await expect(page.locator('text=Пополнение баланса')).toBeVisible();
    // Fill 0 — button should be disabled
    await page.fill('input[type="number"]', '0');
    const payBtn = page.locator('[role="dialog"] button:has-text("Оплатить")');
    await expect(payBtn).toBeDisabled();
  });

  test('валидация суммы пополнения: >1000000 → кнопка заблокирована', async ({ page }) => {
    const billing = new BillingPage(page);
    await billing.goto();
    await page.click('button:has-text("Пополнить")');
    await expect(page.locator('text=Пополнение баланса')).toBeVisible();
    await page.fill('input[type="number"]', '2000000');
    const payBtn = page.locator('[role="dialog"] button:has-text("Оплатить")');
    await expect(payBtn).toBeDisabled();
  });
});
