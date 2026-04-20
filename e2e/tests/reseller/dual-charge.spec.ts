import { test, expect } from '@playwright/test';
import {
  seedAggregator,
  seedSubaccount,
  setQuota,
  setTariffs,
  sendSMS,
  getBalance,
  getMarginLog,
} from '../../helpers/api';

/**
 * Task 16: E2E happy-path для dual-charge.
 *
 * Требования для запуска:
 *   1. Feature flag `tarification.commit_on_submit_enabled=true` в конфиге
 *      gateway/tarification сервисов. Сигналим через env `COMMIT_ON_SUBMIT_ENABLED=true`.
 *   2. Admin/test-only endpoints (или SQL-seed) для:
 *        - создания агрегатора (is_reseller=true) с начальным балансом
 *        - установки платформенного price_rule (operator tariffication periods)
 *        - вставки в `aggregator_quotas`
 *        - чтения `aggregator_margin_log`
 *      См. комментарии в `e2e/helpers/api.ts` — заглушки кидают explicit error.
 *   3. Пре-сидированный aggregator id, переданный через
 *      `E2E_DUAL_CHARGE_AGGREGATOR_ID`, а также storageState субаккаунта для
 *      отправки SMS под его сессией.
 *
 * Пока инфра (пункты 2–3) не готова, тест скипается. Самой логики dual-charge
 * это не блокирует — покрыто `internal/services/billing/application/charge_dual_integration_test.go`.
 */

const isEnabled = process.env.COMMIT_ON_SUBMIT_ENABLED === 'true';

test.use({ storageState: './reseller-auth-state.json' });

test.describe('Aggregator Dual-Charge', () => {
  test.skip(
    !isEnabled,
    'requires COMMIT_ON_SUBMIT_ENABLED=true + admin seed endpoints (aggregator_quotas, price_rule, aggregator_margin_log). See helper comments.',
  );

  test('happy path: отправка субаккаунтом списывает и с субаккаунта, и с агрегатора', async ({
    request,
  }) => {
    // 1. Агрегатор с балансом 1000₽ (pre-seeded, id из env).
    const aggregator = await seedAggregator(request, {
      balance: '1000',
      isReseller: true,
    });

    // 2. Субаккаунт с балансом 100₽ под агрегатором.
    const subaccount = await seedSubaccount(request, {
      parentClientId: aggregator.id,
      balance: '100',
    });

    // 3. Квота агрегатора: 1000 сегментов, overage_rate=2₽, auto_renew=true.
    await setQuota(request, {
      aggregatorId: aggregator.id,
      segmentLimit: 1000,
      overageRate: '2',
      autoRenew: true,
    });

    // 4. Тарифы: платформа 1₽/сегмент, sub-account 5₽/сегмент.
    await setTariffs(request, {
      aggregatorId: aggregator.id,
      subAccountId: subaccount.id,
      platformPrice: '1',
      subPrice: '5',
    });

    // 5. Отправка SMS от субаккаунта.
    //    NB: текущая storageState — reseller. Чтобы отправить под субаккаунтом,
    //    нужно либо переключить storageState, либо импersonate-endpoint.
    //    Пока инфра не готова — тест всё равно скипнут.
    const result = await sendSMS(request, {
      clientId: subaccount.id,
      text: 'test',
      to: '+70000000000',
    });
    expect(result.status).toBe('sent');

    // 6. Балансы: sub -5₽, aggregator -1₽.
    expect(await getBalance(request, subaccount.id)).toBe('95');
    expect(await getBalance(request, aggregator.id)).toBe('999');

    // 7. Margin log: одна запись, charge_mode='pool', margin=4₽.
    const margin = await getMarginLog(request, subaccount.id);
    expect(margin.length).toBe(1);
    expect(margin[0].charge_mode).toBe('pool');
    expect(margin[0].margin).toBe('4');
  });
});
