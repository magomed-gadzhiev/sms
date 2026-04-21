import { test, expect } from '@playwright/test';
import {
  adminCreateRule,
  adminDeleteRule,
  adminListRules,
  resellerCreateRule,
  clientCreateRule,
  clientListRules,
  clientDeleteRule,
  pickAnyProviderId,
  findPlatformGeneralId,
  requireSecondClientId,
  requireOtherResellerSubaccountId,
  type RuleResponse,
} from '../../helpers/routing-hierarchy';

// -----------------------------------------------------------------------------
// Многоуровневая маршрутизация — API-level RBAC + singleton protection.
//
// Спека: docs/superpowers/specs/2026-04-21-routing-hierarchy-design.md
// План:  docs/superpowers/plans/2026-04-21-routing-hierarchy.md (Tasks 15–18)
//
// ВСЕ ТЕСТЫ — test.fixme: handler'ы ещё не созданы
// (ни /admin/routing/rules, ни /portal/reseller/routing/rules,
// ни /portal/routing/rules нет в portal/router.go на 2026-04-21).
//
// Снять fixme после:
//   - AC-H2, H3, H4:   Task 15 (assertOwnable) + Task 18 (admin handler)
//   - AC-H5:           Task 15 + Task 16 (reseller handler)
//   - AC-H6, H7:       Task 15 + Task 17 (subaccount/regular handler)
//
// Плюс для H5/H7 требуются env vars (см. helpers/routing-hierarchy.ts):
//   E2E_ROUTING_SECOND_CLIENT_ID      — id второго обычного клиента
//   E2E_ROUTING_OTHER_SUBACCOUNT_ID   — id subaccount чужого реселлера
// -----------------------------------------------------------------------------

test.describe('Маршрутизация — иерархия: RBAC и singleton (API)', () => {
  test.fixme(
    'AC-H1: admin создаёт platform-general SMS правило',
    async ({ request }) => {
      const providerId = await pickAnyProviderId(request);

      const res = await adminCreateRule(request, {
        owner_type: 'platform',
        owner_id: null,
        route_type: 'sms',
        operator_id: null,
        country_code: null,
        traffic_type: null,
        number_from: null,
        number_to: null,
        provider_id: providerId,
        priority: 500, // намеренно отличается от существующих platform-sms, чтобы не конфликтовать
      });

      expect(res.status(), await res.text()).toBe(201);
      const body = await res.json();
      expect(body.owner_type).toBe('platform');
      expect(body.owner_id).toBeNull();

      // cleanup: priority не определяет general/specific (generalness = все условия NULL).
      // Если platform-sms general уже существует с тем же provider_id,
      // handler может вернуть 409 на create (unique-cell constraint) — TODO
      // переверифицировать при снятии fixme и скорректировать тест под
      // реальный сценарий (либо seed с уникальным provider, либо проверять
      // uq-конфликт явно).
      await adminDeleteRule(request, body.id);
    },
  );

  test.fixme(
    'AC-H2: non-admin получает 403 на POST /admin/routing/rules',
    async ({ request }) => {
      const providerId = await pickAnyProviderId(request);

      // Используем обычный client storage state (auth-state.json) — не admin.
      // CSRF подхватится из того же state через headers() helper.
      const res = await adminCreateRule(
        request,
        {
          owner_type: 'platform',
          owner_id: null,
          route_type: 'sms',
          operator_id: null,
          country_code: null,
          traffic_type: null,
          number_from: null,
          number_to: null,
          provider_id: providerId,
          priority: 999,
        },
        'auth-state.json',
      );

      expect([401, 403]).toContain(res.status());
    },
  );

  test.fixme(
    'AC-H3: DELETE последнего platform-general SMS отклоняется 409',
    async ({ request }) => {
      const generalId = await findPlatformGeneralId(request, 'sms');

      const res = await adminDeleteRule(request, generalId);
      expect(res.status(), await res.text()).toBe(409);

      // правило должно остаться — проверяем через list напрямую (не через
      // findPlatformGeneralId, который бросает — нужно чистое assertion-failure).
      const listRes = await adminListRules(request);
      expect(listRes.ok()).toBeTruthy();
      const listBody = await listRes.json();
      const rules: RuleResponse[] = listBody.rules ?? [];
      const stillExists = rules.some((r) => r.id === generalId);
      expect(stillExists, `platform-general ${generalId} должен остаться после отклонённого DELETE`).toBe(true);
    },
  );

  test.fixme(
    'AC-H4: DELETE specific platform-правила проходит 200',
    async ({ request }) => {
      const providerId = await pickAnyProviderId(request);

      // создаём specific: country_code заполнен → не general
      const createRes = await adminCreateRule(request, {
        owner_type: 'platform',
        owner_id: null,
        route_type: 'sms',
        operator_id: null,
        country_code: 'RU',
        traffic_type: null,
        number_from: null,
        number_to: null,
        provider_id: providerId,
        priority: 700,
      });
      expect(createRes.ok()).toBeTruthy();
      const { id } = await createRes.json();

      const delRes = await adminDeleteRule(request, id);
      expect(delRes.status()).toBeGreaterThanOrEqual(200);
      expect(delRes.status()).toBeLessThan(300);
    },
  );

  test.fixme(
    'AC-H5: reseller R1 не может создать правило для subaccount чужого реселлера R2',
    async ({ request }) => {
      const otherSubaccountId = requireOtherResellerSubaccountId();
      const providerId = await pickAnyProviderId(request);

      const res = await resellerCreateRule(request, {
        owner_type: 'subaccount',
        owner_id: otherSubaccountId,
        route_type: 'sms',
        operator_id: null,
        country_code: null,
        traffic_type: null,
        number_from: null,
        number_to: null,
        provider_id: providerId,
        priority: 100,
      });

      expect([403, 404]).toContain(res.status());
    },
  );

  test.fixme(
    'AC-H6: regular client CRUD собственных правил',
    async ({ request }) => {
      const providerId = await pickAnyProviderId(request);

      const createRes = await clientCreateRule(request, {
        // owner_type/owner_id инферятся из сессии на стороне handler'а
        // (§Portal API "Regular client"); явно задаём для контракта.
        owner_type: 'client',
        route_type: 'sms',
        operator_id: null,
        country_code: null,
        traffic_type: null,
        number_from: null,
        number_to: null,
        provider_id: providerId,
        priority: 100,
      });
      expect(createRes.ok(), await createRes.text()).toBeTruthy();
      const { id } = await createRes.json();

      const listRes = await clientListRules(request);
      expect(listRes.ok()).toBeTruthy();
      const list = await listRes.json();
      const ids = (list.rules ?? []).map((r: { id: string }) => r.id);
      expect(ids).toContain(id);

      const delRes = await clientDeleteRule(request, id);
      expect(delRes.ok()).toBeTruthy();
    },
  );

  test.fixme(
    'AC-H7: regular client не видит правил другого клиента',
    async ({ request }) => {
      // Требует env E2E_ROUTING_SECOND_CLIENT_ID и предварительно
      // созданное правило для этого клиента (через SQL-seed или admin endpoint).
      const otherClientId = requireSecondClientId();

      // Убедиться что у второго клиента есть хотя бы одно правило —
      // иначе тест пройдёт вакуально и ничего не докажет.
      const adminList = await adminListRules(request);
      expect(adminList.ok()).toBeTruthy();
      const adminBody = await adminList.json();
      const otherRules: RuleResponse[] = (adminBody.rules ?? []).filter(
        (r: RuleResponse) => r.owner_id === otherClientId,
      );
      test.skip(otherRules.length === 0, `второй клиент ${otherClientId} не имеет правил — seed через SQL или admin endpoint`);

      const listRes = await clientListRules(request);
      expect(listRes.ok()).toBeTruthy();
      const list = await listRes.json();
      const rules: Array<{ owner_type: string; owner_id: string }> = list.rules ?? [];

      const foreign = rules.filter(
        (r) => r.owner_type === 'client' && r.owner_id === otherClientId,
      );
      expect(foreign).toHaveLength(0);
    },
  );
});
