import { type APIRequestContext } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

// -----------------------------------------------------------------------------
// Hierarchy-routing E2E helpers (spec: 2026-04-21-routing-hierarchy-design.md;
// plan: docs/superpowers/plans/2026-04-21-routing-hierarchy.md Tasks 15–18).
//
// Тесты на этих helper'ах — test.fixme до появления handler'ов:
//   - /admin/routing/rules          (Task 18)
//   - /portal/reseller/routing/rules (Task 16)
//   - /portal/routing/rules          (Task 17)
//
// Контракт request/response body угадан из спеки §"Portal API" и доменной
// модели §"Доменная модель (client_route.go)". ПЕРЕВЕРИФИЦИРОВАТЬ после
// появления реальных handler'ов (Task 15–18) — это нарушение
// feedback_verify_before_asserting, но иного источника нет до реализации.
//
// Pre-seed инфраструктура клиентов/реселлеров не имеет public API —
// паттерн как в dual-charge: env vars + throw до появления admin endpoint'ов.
// -----------------------------------------------------------------------------

const API_BASE = process.env.API_URL || 'http://localhost:8083';
const PORTAL = `${API_BASE}/portal/v1`;
const ADMIN = `${API_BASE}/admin/v1`;

export type OwnerType = 'platform' | 'client' | 'subaccount';
export type RouteType = 'sms' | 'hlr' | 'max';

export interface RuleCreateRequest {
  owner_type: OwnerType;
  owner_id?: string | null;
  route_type: RouteType;
  operator_id?: string | null;
  country_code?: string | null;
  traffic_type?: string | null;
  number_from?: number | null;
  number_to?: number | null;
  provider_id: string;
  priority: number;
}

export interface RuleResponse {
  id: string;
  owner_type: OwnerType;
  owner_id: string | null;
  route_type: RouteType;
  operator_id: string | null;
  country_code: string | null;
  traffic_type: string | null;
  number_from: number | null;
  number_to: number | null;
  provider_id: string;
  priority: number;
  created_at?: string;
}

function csrfFromState(stateFile: string): string {
  try {
    const p = path.resolve(__dirname, '..', stateFile);
    const s = JSON.parse(fs.readFileSync(p, 'utf-8'));
    const c = s.cookies?.find((x: { name: string }) => x.name === 'csrf_token');
    return c?.value || '';
  } catch {
    return '';
  }
}

function csrfHeaders(stateFile: string): Record<string, string> {
  const h: Record<string, string> = {};
  const t = csrfFromState(stateFile);
  if (t) h['X-CSRF-Token'] = t;
  return h;
}

// --- /admin/routing/rules (Task 18: platform rules) ---

export async function adminCreateRule(
  request: APIRequestContext,
  data: RuleCreateRequest,
  stateFile = 'admin-auth-state.json',
) {
  return request.fetch(`${ADMIN}/routing/rules`, {
    method: 'POST',
    headers: csrfHeaders(stateFile),
    data,
  });
}

export async function adminListRules(
  request: APIRequestContext,
) {
  return request.get(`${ADMIN}/routing/rules`);
}

export async function adminDeleteRule(
  request: APIRequestContext,
  id: string,
  stateFile = 'admin-auth-state.json',
) {
  return request.fetch(`${ADMIN}/routing/rules/${id}`, {
    method: 'DELETE',
    headers: csrfHeaders(stateFile),
  });
}

// --- /portal/reseller/routing/rules (Task 16: client + subaccount rules) ---

export async function resellerCreateRule(
  request: APIRequestContext,
  data: RuleCreateRequest,
  stateFile = 'reseller-auth-state.json',
) {
  return request.fetch(`${PORTAL}/reseller/routing/rules`, {
    method: 'POST',
    headers: csrfHeaders(stateFile),
    data,
  });
}

// --- /portal/routing/rules (Task 17: regular client / subaccount self) ---

export async function clientCreateRule(
  request: APIRequestContext,
  data: RuleCreateRequest,
  stateFile = 'auth-state.json',
) {
  return request.fetch(`${PORTAL}/routing/rules`, {
    method: 'POST',
    headers: csrfHeaders(stateFile),
    data,
  });
}

export async function clientListRules(
  request: APIRequestContext,
) {
  return request.get(`${PORTAL}/routing/rules`);
}

export async function clientDeleteRule(
  request: APIRequestContext,
  id: string,
  stateFile = 'auth-state.json',
) {
  return request.fetch(`${PORTAL}/routing/rules/${id}`, {
    method: 'DELETE',
    headers: csrfHeaders(stateFile),
  });
}

// --- Pre-seed references (no public API; env vars per dual-charge pattern) ---

/**
 * Pre-seeded id "второго" клиента (обычного), не равного test@example.com.
 * Нужен для AC-H7 (regular client не видит правил другого клиента).
 * Бросает — пока нет admin endpoint для создания клиентов. Seed через SQL
 * или новый admin-fixture-endpoint, потом установить env.
 */
export function requireSecondClientId(): string {
  const id = process.env.E2E_ROUTING_SECOND_CLIENT_ID;
  if (!id) {
    throw new Error(
      'requireSecondClientId: E2E_ROUTING_SECOND_CLIENT_ID not set. ' +
      'Pre-seed a second regular client (parent_client_id IS NULL) and export its id.',
    );
  }
  return id;
}

/**
 * Pre-seeded id стороннего реселлера + его subaccount.
 * Нужен для AC-H5 (reseller R1 пытается создать правило для subaccount R2).
 * Активный reseller (sessionOwner) логинится через reseller-auth-state.json —
 * его id должен быть известен через env.
 */
export function requireOtherResellerSubaccountId(): string {
  const id = process.env.E2E_ROUTING_OTHER_SUBACCOUNT_ID;
  if (!id) {
    throw new Error(
      'requireOtherResellerSubaccountId: E2E_ROUTING_OTHER_SUBACCOUNT_ID not set. ' +
      'Pre-seed reseller R2 with subaccount SA2 and export SA2.id.',
    );
  }
  return id;
}

/**
 * Выбирает любой активный provider_id через публичный /routes/providers.
 * Нужен для любого create-rule теста.
 */
export async function pickAnyProviderId(
  request: APIRequestContext,
): Promise<string> {
  const res = await request.get(`${PORTAL}/routes/providers`);
  if (!res.ok()) {
    throw new Error(`pickAnyProviderId: ${res.status()} ${await res.text()}`);
  }
  const body = await res.json();
  const id = body.providers?.[0]?.id;
  if (!id) throw new Error('pickAnyProviderId: /routes/providers вернул пустой список');
  return id;
}

/**
 * Находит ровно один platform-general rule для заданного route_type через
 * admin list. Необходим для AC-H3 (защита singleton): тест удаляет
 * именно этот id и ожидает 409. Если general'ов больше одного —
 * бросает (нельзя гарантировать "последний"); если ноль — бросает
 * (не singleton-кейс, hlr/max в проде не seed'ится).
 */
export async function findPlatformGeneralId(
  request: APIRequestContext,
  routeType: RouteType,
): Promise<string> {
  const res = await adminListRules(request);
  if (!res.ok()) throw new Error(`adminListRules: ${res.status()}`);
  const body = await res.json();
  const rules: RuleResponse[] = body.rules ?? [];
  const generals = rules.filter(
    (r) =>
      r.owner_type === 'platform' &&
      r.route_type === routeType &&
      r.operator_id === null &&
      r.country_code === null &&
      r.traffic_type === null &&
      r.number_from === null &&
      r.number_to === null,
  );
  if (generals.length === 0) {
    throw new Error(`findPlatformGeneralId(${routeType}): general отсутствует — seed ожидался`);
  }
  if (generals.length > 1) {
    throw new Error(
      `findPlatformGeneralId(${routeType}): найдено ${generals.length} general'ов; ` +
      'AC-H3 проверяет поведение при удалении последнего — ожидается ровно один',
    );
  }
  return generals[0].id;
}
