import { type APIRequestContext } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

const API_BASE = process.env.API_URL || 'http://localhost:8083/portal/v1';

function extractCsrfToken(stateFile = 'auth-state.json'): string {
  try {
    const statePath = path.resolve(__dirname, '..', stateFile);
    const state = JSON.parse(fs.readFileSync(statePath, 'utf-8'));
    const csrfCookie = state.cookies?.find((c: { name: string; value: string }) => c.name === 'csrf_token');
    return csrfCookie?.value || '';
  } catch {
    return '';
  }
}

export class ApiHelper {
  private csrfToken: string;

  constructor(private request: APIRequestContext, storageStateFile?: string) {
    this.csrfToken = extractCsrfToken(storageStateFile);
  }

  private async fetch(path: string, options?: {
    method?: string;
    data?: unknown;
  }) {
    const url = `${API_BASE}${path}`;
    const method = options?.method || 'GET';
    const headers: Record<string, string> = {};

    // Add CSRF token for non-GET requests
    if (method !== 'GET' && this.csrfToken) {
      headers['X-CSRF-Token'] = this.csrfToken;
    }

    if (method === 'GET') {
      return this.request.get(url);
    }
    return this.request.fetch(url, {
      method,
      data: options?.data,
      headers,
    });
  }

  // --- Auth ---
  async login(email: string, password: string) {
    return this.fetch('/auth/login', { method: 'POST', data: { email, password } });
  }

  // --- Billing ---
  async getBalance() {
    const res = await this.fetch('/billing/balance');
    return res.json();
  }

  async getTransactions(params: Record<string, string> = {}) {
    const qs = new URLSearchParams(params).toString();
    const res = await this.fetch(`/billing/transactions?${qs}`);
    return res.json();
  }

  // --- Messages ---
  async sendMessage(destination: string, text: string, source: string) {
    const res = await this.fetch('/messages', {
      method: 'POST',
      data: { destination, text, source },
    });
    return res.json();
  }

  async getMessage(id: string) {
    const res = await this.fetch(`/detalization/${id}`);
    return res.json();
  }

  async listMessages(params: Record<string, string> = {}) {
    const qs = new URLSearchParams(params).toString();
    const res = await this.fetch(`/detalization?${qs}`);
    return res.json();
  }

  // --- Campaigns ---
  async createCampaign(data: {
    name: string;
    contact_list_id: string;
    template_id?: string;
    source?: string;
    send_rate?: number;
    scheduled_at?: string;
    use_subscriber_timezone?: boolean;
  }) {
    const res = await this.fetch('/campaigns', { method: 'POST', data });
    return res.json();
  }

  async getCampaign(id: string) {
    const res = await this.fetch(`/campaigns/${id}`);
    return res.json();
  }

  async launchCampaign(id: string) {
    const res = await this.fetch(`/campaigns/${id}/launch`, { method: 'POST' });
    return res.json();
  }

  async getCampaignStats(id: string) {
    const res = await this.fetch(`/campaigns/${id}/stats`);
    return res.json();
  }

  async deleteCampaign(id: string) {
    return this.fetch(`/campaigns/${id}`, { method: 'DELETE' });
  }

  async estimateCost(data: {
    contact_list_id: string;
    text: string;
    source: string;
  }) {
    const res = await this.fetch('/campaigns/estimate-cost', { method: 'POST', data });
    return res.json();
  }

  // --- Tariffs ---
  async getCurrentTariff() {
    const res = await this.fetch('/tariffs/current');
    return res.json();
  }

  async listTariffPlans() {
    const res = await this.fetch('/tariffs/plans');
    return res.json();
  }

  // --- Sender Names ---
  async listApprovedSenderNames() {
    const res = await this.fetch('/sender-names?status=approved&per_page=100');
    return res.json();
  }

  // --- Contact Lists ---
  async listContactLists() {
    const res = await this.fetch('/contact-lists?page=1&per_page=100');
    return res.json();
  }

  // --- Templates ---
  async listTemplates(params: Record<string, string> = {}) {
    const qs = new URLSearchParams({ per_page: '100', ...params }).toString();
    const res = await this.fetch(`/templates?${qs}`);
    return res.json();
  }

  // --- Routes ---
  async listRoutes(params: Record<string, string> = {}) {
    const qs = new URLSearchParams(params).toString();
    const res = await this.fetch(`/routes?${qs}`);
    return res.json();
  }

  async getRoute(id: string) {
    const res = await this.fetch(`/routes/${id}`);
    return res.json();
  }

  async createRoute(data: {
    name: string;
    route_type: string;
    provider_id: string;
    priority: number;
    share: number;
    status: string;
    comment?: string;
    condition_groups?: unknown[];
    schedules?: unknown[];
  }) {
    const res = await this.fetch('/routes', { method: 'POST', data });
    return res.json();
  }

  async deleteRoute(id: string) {
    return this.fetch(`/routes/${id}`, { method: 'DELETE' });
  }

  async listRouteProviders() {
    const res = await this.fetch('/routes/providers');
    return res.json();
  }

  // --- Sub-Accounts ---
  async listSubAccounts() {
    const res = await this.fetch('/sub-accounts');
    return res.json();
  }

  async createSubAccount(data: {
    name: string;
    email: string;
    contact_person?: string;
    initial_balance?: string;
    daily_limit?: number;
    monthly_limit?: number;
  }) {
    const res = await this.fetch('/sub-accounts', { method: 'POST', data });
    return res.json();
  }

  async getSubAccount(id: string) {
    const res = await this.fetch(`/sub-accounts/${id}`);
    return res.json();
  }

  async updateSubAccountLimits(id: string, data: { daily_limit: number; monthly_limit: number }) {
    const res = await this.fetch(`/sub-accounts/${id}/limits`, { method: 'PUT', data });
    return res.json();
  }

  async transferToSubAccount(id: string, amount: string) {
    const res = await this.fetch(`/sub-accounts/${id}/transfer`, { method: 'POST', data: { amount } });
    return res.json();
  }

  async deleteSubAccount(id: string) {
    return this.fetch(`/sub-accounts/${id}`, { method: 'DELETE' });
  }

  async getSubAccountMessages(id: string, params: Record<string, string> = {}) {
    const qs = new URLSearchParams(params).toString();
    const res = await this.fetch(`/sub-accounts/${id}/messages?${qs}`);
    return res.json();
  }

  async getSubAccountAnalytics(id: string, params: Record<string, string> = {}) {
    const qs = new URLSearchParams(params).toString();
    const res = await this.fetch(`/sub-accounts/${id}/analytics?${qs}`);
    return res.json();
  }

  async getSubAccountTransactions(id: string, params: Record<string, string> = {}) {
    const qs = new URLSearchParams(params).toString();
    const res = await this.fetch(`/sub-accounts/${id}/transactions?${qs}`);
    return res.json();
  }

  // --- Companies ---
  async listCompanies(): Promise<{ companies: Array<{ id: string; name: string; is_default: boolean; is_offer: boolean; inn?: string }> }> {
    const res = await this.fetch('/companies');
    return res.json();
  }

  async setDefaultCompany(id: string) {
    return this.fetch(`/companies/${id}/set-default`, { method: 'POST' });
  }

  async detachCompany(id: string) {
    return this.fetch(`/companies/${id}/detach`, { method: 'DELETE' });
  }

  /**
   * Cleanup helper: detaches all non-default non-offer companies matching name prefix.
   * If a test-created company is currently default, first restores Оферта as default.
   */
  async cleanupTestCompanies(namePrefix: string) {
    const { companies } = await this.listCompanies();
    const offer = companies.find((c) => c.is_offer);
    const testComps = companies.filter((c) => c.name.startsWith(namePrefix));
    const hasDefaultTest = testComps.some((c) => c.is_default);
    if (hasDefaultTest && offer) {
      await this.setDefaultCompany(offer.id);
    }
    for (const c of testComps) {
      await this.detachCompany(c.id);
    }
  }

  // --- Reseller Dashboard ---
  async getResellerDashboard(period = 'today') {
    const res = await this.fetch(`/reseller/dashboard?period=${period}`);
    return res.json();
  }

  // --- Reseller Moderation ---
  async getModerationCounts() {
    const res = await this.fetch('/reseller/moderation/counts');
    return res.json();
  }

  async listResellerSenderNames(params: Record<string, string> = {}) {
    const qs = new URLSearchParams(params).toString();
    const res = await this.fetch(`/reseller/sender-names?${qs}`);
    return res.json();
  }

  async approveResellerSenderName(id: string) {
    return this.fetch(`/reseller/sender-names/${id}/approve`, { method: 'POST' });
  }

  async rejectResellerSenderName(id: string, reason: string) {
    return this.fetch(`/reseller/sender-names/${id}/reject`, { method: 'POST', data: { reason } });
  }

  async listResellerTemplates(params: Record<string, string> = {}) {
    const qs = new URLSearchParams(params).toString();
    const res = await this.fetch(`/reseller/templates?${qs}`);
    return res.json();
  }

  async approveResellerTemplate(id: string) {
    return this.fetch(`/reseller/templates/${id}/approve`, { method: 'POST' });
  }

  async rejectResellerTemplate(id: string, reason: string) {
    return this.fetch(`/reseller/templates/${id}/reject`, { method: 'POST', data: { reason } });
  }
}

// -----------------------------------------------------------------------------
// Dual-charge E2E helpers (Task 16)
//
// NOTE: the plan's helper signatures (`seedAggregator`, `setQuota`, etc.) assume
// the existence of admin/system-level seed endpoints that are NOT currently
// exposed via the portal HTTP API. The list of operations needed:
//
//   - create an aggregator (client with is_reseller=true) with initial balance
//   - set is_reseller flag + fund the account
//   - set price_rule (platform tariff) for aggregator
//   - insert into `aggregator_tariffs`
//   - insert into `aggregator_quotas`
//   - read `aggregator_margin_log` rows
//
// Only (2) — transfer-to-subaccount — has a public API. The rest require
// either a direct DB seed or a new admin/test-only endpoint. Until that
// infrastructure exists, this helper set is a thin wrapper that relies on
// pre-seeded fixtures (aggregator + subaccount + tariffs + quota must already
// exist in the target env) and identifies them via env vars:
//
//   E2E_DUAL_CHARGE_AGGREGATOR_ID
//   E2E_DUAL_CHARGE_SUBACCOUNT_ID
//
// Phase 2 dual-charge is the only sub-account path now (flag removed). The
// test skips itself unless `E2E_DUAL_CHARGE_READY=true` is set — signalling
// that the admin-seed endpoints listed above are wired in the target env.
// -----------------------------------------------------------------------------

export interface SeedAggregatorOpts { balance: string; isReseller: boolean }
export interface SeededAggregator { id: string; initialBalance: string }

/**
 * Returns the pre-seeded aggregator id from env. Full programmatic seed
 * requires an admin-only endpoint that does not exist yet — add one in
 * `internal/gateway/portal/handlers/test_fixtures.go` (behind a build tag
 * or env guard) before this helper can construct aggregators from scratch.
 */
export async function seedAggregator(
  _request: APIRequestContext,
  _opts: SeedAggregatorOpts,
): Promise<SeededAggregator> {
  const id = process.env.E2E_DUAL_CHARGE_AGGREGATOR_ID;
  if (!id) {
    throw new Error(
      'seedAggregator: E2E_DUAL_CHARGE_AGGREGATOR_ID not set. ' +
      'Programmatic aggregator seed requires an admin/test endpoint that is not yet implemented. ' +
      'Pre-seed the aggregator (with is_reseller=true and funded balance) and export its id.',
    );
  }
  return { id, initialBalance: _opts.balance };
}

export interface SeedSubaccountOpts { parentClientId: string; balance: string }
export interface SeededSubaccount { id: string; parentClientId: string }

/**
 * Creates a subaccount under the aggregator via the existing portal API.
 * Requires the reseller storage state (reseller-auth-state.json) so that
 * `parent_client_id` is inferred from the session.
 */
export async function seedSubaccount(
  request: APIRequestContext,
  opts: SeedSubaccountOpts,
): Promise<SeededSubaccount> {
  const api = new ApiHelper(request);
  const suffix = Date.now().toString(36);
  const result = await api.createSubAccount({
    name: `E2E-DualCharge-${suffix}`,
    email: `e2e-dc-${suffix}@test.local`,
    initial_balance: opts.balance,
  });
  const id: string | undefined = result.id || result.sub_account?.id;
  if (!id) {
    throw new Error(`seedSubaccount: failed to create — response: ${JSON.stringify(result)}`);
  }
  return { id, parentClientId: opts.parentClientId };
}

export interface SetQuotaOpts {
  aggregatorId: string;
  segmentLimit: number;
  overageRate: string;
  autoRenew?: boolean;
}

/**
 * Inserts/updates a row in `aggregator_quotas`. There is currently no public
 * endpoint for this — admin UI uses a server-side handler that is scoped to
 * `/admin/v1/aggregator-quotas` (TODO: verify path and implement if missing).
 * For now this helper throws; the calling test will skip.
 */
export async function setQuota(
  _request: APIRequestContext,
  _opts: SetQuotaOpts,
): Promise<void> {
  throw new Error(
    'setQuota: no programmatic API — aggregator_quotas must be seeded via SQL or a ' +
    'new admin endpoint. Expose POST /admin/v1/aggregator-quotas before enabling this test.',
  );
}

export interface SetTariffsOpts {
  aggregatorId: string;
  subAccountId: string;
  platformPrice: string;
  subPrice: string;
}

/**
 * Sets the platform-level price rule for the aggregator AND the
 * aggregator-to-subaccount tariff. The subaccount side is settable via the
 * existing reseller API (PUT /portal/v1/reseller/tariffs). The platform-side
 * price rule requires admin access (operator tariffication periods) which
 * this helper does not yet automate.
 */
export async function setTariffs(
  request: APIRequestContext,
  opts: SetTariffsOpts,
): Promise<void> {
  // Sub-account side: aggregator_tariffs row via reseller API.
  const csrf = extractCsrfToken();
  const headers: Record<string, string> = {};
  if (csrf) headers['X-CSRF-Token'] = csrf;
  const res = await request.fetch(`${API_BASE}/reseller/tariffs`, {
    method: 'PUT',
    headers,
    data: {
      sub_account_id: opts.subAccountId,
      tariffs: [{ price_per_segment: opts.subPrice }],
    },
  });
  if (!res.ok()) {
    throw new Error(`setTariffs (sub-account side) failed: ${res.status()} ${await res.text()}`);
  }
  // Platform side (price_rule): requires admin endpoint — not implemented here.
  throw new Error(
    'setTariffs: platform-side price_rule seeding not automated. ' +
    'Seed via SQL or add admin fixture endpoint before enabling this test.',
  );
}

export interface SendSMSOpts { clientId: string; text: string; to: string }
export interface SendSMSResult { status: string; message_id?: string }

/**
 * Sends an SMS "as" a subaccount. The current portal session is tied to a
 * single client; to send as a subaccount we'd need either:
 *   (a) a subaccount login session, or
 *   (b) an admin impersonate endpoint.
 * Neither is currently wired here — the subaccount's auth state must be
 * pre-generated (see `e2e/global-setup.ts`) and swapped in. For now this
 * helper uses the active request context's session, assuming the caller
 * switched storageState to the subaccount.
 */
export async function sendSMS(
  request: APIRequestContext,
  opts: SendSMSOpts,
): Promise<SendSMSResult> {
  const csrf = extractCsrfToken();
  const headers: Record<string, string> = {};
  if (csrf) headers['X-CSRF-Token'] = csrf;
  const res = await request.fetch(`${API_BASE}/messages`, {
    method: 'POST',
    headers,
    data: { destination: opts.to, text: opts.text, source: 'E2E' },
  });
  const body = await res.json();
  return { status: body.status ?? (res.ok() ? 'sent' : 'failed'), message_id: body.id };
}

/**
 * Fetches balance for an arbitrary client id. The portal API only exposes
 * "current session balance" — per-client lookup requires admin access. This
 * helper falls back to `/billing/balance` when the id matches the session;
 * otherwise it throws.
 */
export async function getBalance(
  request: APIRequestContext,
  clientId: string,
): Promise<string> {
  const res = await request.get(`${API_BASE}/billing/balance`);
  if (!res.ok()) {
    throw new Error(`getBalance(${clientId}): ${res.status()} ${await res.text()}`);
  }
  const body = await res.json();
  // Expected shape: { balance: string } or { amount: string }.
  return String(body.balance ?? body.amount ?? '');
}

export interface MarginLogRow {
  id: string;
  charge_mode: 'pool' | 'overage' | 'margin' | string;
  margin: string;
}

/**
 * Reads `aggregator_margin_log` rows for a given subaccount. No public API
 * exists — must go through an admin endpoint. TODO: add
 * GET /admin/v1/aggregator-margin-log?sub_account_id=... and point this here.
 */
export async function getMarginLog(
  _request: APIRequestContext,
  _subAccountId: string,
): Promise<MarginLogRow[]> {
  throw new Error(
    'getMarginLog: no admin endpoint yet. Add GET /admin/v1/aggregator-margin-log ' +
    'and wire this helper before enabling the dual-charge E2E test.',
  );
}

// Admin API helper (uses /admin/v1 base)
const ADMIN_API_BASE = process.env.ADMIN_API_URL || `${process.env.BASE_URL || 'http://localhost:8083'}/admin/v1`;

export class AdminApiHelper {
  private csrfToken: string;

  constructor(
    private request: import('@playwright/test').APIRequestContext,
    storageStateFile: string = 'admin-auth-state.json',
  ) {
    this.csrfToken = extractCsrfToken(storageStateFile);
  }

  private async fetch(path: string, options?: {
    method?: string;
    data?: unknown;
  }) {
    const url = `${ADMIN_API_BASE}${path}`;
    const method = options?.method || 'GET';
    const headers: Record<string, string> = {};

    if (method !== 'GET' && this.csrfToken) {
      headers['X-CSRF-Token'] = this.csrfToken;
    }

    if (method === 'GET') {
      return this.request.get(url);
    }
    return this.request.fetch(url, {
      method,
      data: options?.data,
      headers,
    });
  }

  // --- Tarification ---
  async listTariffPlans() {
    const res = await this.fetch('/tarification/tariff-plans');
    return res.json();
  }

  async createTariffPlan(data: { operator_id: string; sender_category: string; strategy: string }) {
    const res = await this.fetch('/tarification/tariff-plans', { method: 'POST', data });
    return res.json();
  }

  async updateTariffPlan(id: string, data: { active?: boolean }) {
    const res = await this.fetch(`/tarification/tariff-plans/${id}`, { method: 'PUT', data });
    return res.json();
  }

  async listPeriods(params: Record<string, string> = {}) {
    const qs = new URLSearchParams(params).toString();
    const res = await this.fetch(`/tarification/periods?${qs}`);
    return res.json();
  }

  async createPeriod(data: {
    country_id?: string | null;
    operator_id?: string | null;
    sender_category?: string | null;
    traffic_type?: string | null;
    client_id?: string | null;
    strategy: string;
    start_date: string;
  }) {
    const res = await this.fetch('/tarification/periods', { method: 'POST', data });
    return res.json();
  }

  async deletePeriod(id: string) {
    return this.fetch(`/tarification/periods/${id}`, { method: 'DELETE' });
  }

  async listPeriodTiers(periodId: string) {
    const res = await this.fetch(`/tarification/periods/${periodId}/tiers`);
    return res.json();
  }

  async createPeriodTier(periodId: string, data: { from_count: number; price_per_segment: string }) {
    const res = await this.fetch(`/tarification/periods/${periodId}/tiers`, { method: 'POST', data });
    return res.json();
  }

  async listOperators(params: Record<string, string> = {}) {
    const qs = new URLSearchParams(params).toString();
    const res = await this.fetch(`/operators?${qs}`);
    return res.json();
  }
}
