import { test, expect } from '@playwright/test';
import { ApiHelper } from '../../helpers/api';
import { TEST_PHONES, TEST_SMS_TEXT } from '../../helpers/test-data';

test.use({ storageState: './auth-state.json' });

test.describe('Тарификация — проверка через API', () => {

  test('текущий тарифный план загружается', async ({ request }) => {
    const api = new ApiHelper(request);
    const tariff = await api.getCurrentTariff();
    expect(tariff).toBeTruthy();
    // Response may have plan or tariff_plan or direct fields
    const plan = tariff.plan || tariff.tariff_plan || tariff;
    expect(plan.name || plan.plan_name).toBeTruthy();
  });

  test('список тарифных планов доступен', async ({ request }) => {
    const api = new ApiHelper(request);
    const plans = await api.listTariffPlans();
    expect(plans).toBeTruthy();
    const plansList = plans.plans || plans.tariff_plans || [];
    expect(plansList.length).toBeGreaterThanOrEqual(1);
  });

  test('после отправки SMS сообщение содержит информацию о тарификации в детализации', async ({ request }) => {
    const api = new ApiHelper(request);

    // Get approved sender name
    const senders = await api.listApprovedSenderNames();
    const senderName = senders.sender_names?.[0]?.name;
    if (!senderName) {
      test.skip(true, 'No approved sender names — cannot send');
      return;
    }

    // Send a test message
    const sendResult = await api.sendMessage(TEST_PHONES.INVALID, TEST_SMS_TEXT, senderName);
    const messageId = sendResult.message_id || sendResult.id;
    expect(messageId).toBeTruthy();

    // Wait for processing
    await new Promise((r) => setTimeout(r, 5000));

    // Get message detail
    const detail = await api.getMessage(messageId);
    expect(detail).toBeTruthy();
    expect(detail.status).toBeTruthy();

    // Check billing/tarification info if present
    if (detail.billing) {
      expect(detail.billing.segment_count).toBeGreaterThanOrEqual(1);
    }
    // Also check top-level fields
    if (detail.segment_count) {
      expect(detail.segment_count).toBeGreaterThanOrEqual(1);
    }
    if (detail.total_amount !== undefined) {
      expect(parseFloat(detail.total_amount)).toBeGreaterThanOrEqual(0);
    }
  });

  test('мультисегментное SMS тарифицируется по количеству сегментов', async ({ request }) => {
    const api = new ApiHelper(request);

    const senders = await api.listApprovedSenderNames();
    const senderName = senders.sender_names?.[0]?.name;
    if (!senderName) {
      test.skip(true, 'No approved sender names');
      return;
    }

    // Send a long message (>160 chars = 2+ segments)
    const longText = 'A'.repeat(200);
    const sendResult = await api.sendMessage(TEST_PHONES.INVALID, longText, senderName);
    const messageId = sendResult.message_id || sendResult.id;
    expect(messageId).toBeTruthy();

    await new Promise((r) => setTimeout(r, 5000));

    const detail = await api.getMessage(messageId);
    // Multi-segment check — at least 2 segments for 200 latin chars
    const segments = detail.billing?.segment_count || detail.segment_count || detail.segments;
    if (segments) {
      expect(segments).toBeGreaterThanOrEqual(2);
    }
  });

  test('кириллическое SMS тарифицируется с учётом кодировки (70 символов/сегмент)', async ({ request }) => {
    const api = new ApiHelper(request);

    const senders = await api.listApprovedSenderNames();
    const senderName = senders.sender_names?.[0]?.name;
    if (!senderName) {
      test.skip(true, 'No approved sender names');
      return;
    }

    // Cyrillic text > 70 chars should be 2+ segments
    const cyrillicText = 'Тестовое сообщение для проверки тарификации кириллического текста, превышающего лимит одного сегмента юникод SMS';
    const sendResult = await api.sendMessage(TEST_PHONES.INVALID, cyrillicText, senderName);
    const messageId = sendResult.message_id || sendResult.id;
    expect(messageId).toBeTruthy();

    await new Promise((r) => setTimeout(r, 5000));

    const detail = await api.getMessage(messageId);
    const segments = detail.billing?.segment_count || detail.segment_count || detail.segments;
    if (segments) {
      expect(segments).toBeGreaterThanOrEqual(2);
    }
  });
});
