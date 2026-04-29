import { getCookie } from '../utils/cookies';

const API_BASE = '/admin/v1';

export class AdminApiError extends Error {
  constructor(public status: number, message: string, public details?: unknown) {
    super(message);
  }
}

async function adminFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const csrfToken = getCookie('csrf_token');
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(csrfToken ? { 'X-CSRF-Token': csrfToken } : {}),
      ...options?.headers,
    },
    ...options,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: { message: res.statusText } }));
    throw new AdminApiError(res.status, err.error?.message || res.statusText, err.error);
  }
  if (res.status === 204) return {} as T;
  return res.json();
}

function qs(params: Record<string, string | number | boolean | undefined>): string {
  const filtered = Object.entries(params).filter(([, v]) => v !== undefined && v !== '');
  if (filtered.length === 0) return '';
  return '?' + new URLSearchParams(filtered.map(([k, v]) => [k, String(v)])).toString();
}

// ── Types ──

export interface ClientInfo {
  client_id: string;
  name: string;
  email: string;
  contact_person: string;
  phone: string;
  active: boolean;
  rate_limits: RateLimits;
  metadata: Record<string, string>;
  created_at: string;
  updated_at: string;
}

export interface RateLimits {
  messages_per_second: number;
  messages_per_minute: number;
  messages_per_hour: number;
  messages_per_day: number;
}

export interface ProviderInfo {
  provider_id: string;
  name: string;
  host: string;
  port: number;
  system_id: string;
  system_type: string;
  bind_type: number;
  max_connections: number;
  window_size: number;
  active: boolean;
  settings: Record<string, string>;
  created_at: string;
  updated_at: string;
}

export interface ProviderHealth {
  provider_id: string;
  status: string;
  active_connections: number;
  total_connections: number;
  success_rate: number;
  messages_sent_24h: number;
  messages_failed_24h: number;
  last_success?: string;
  last_failure?: string;
  last_error?: string;
}

export interface RouteInfo {
  route_id: string;
  name: string;
  pattern: string;
  priority: number;
  provider_ids: string[];
  load_balance_strategy: string;
  failover_enabled: boolean;
  active: boolean;
  metadata: Record<string, string>;
  created_at: string;
  updated_at: string;
}

export interface BalanceResponse {
  client_id: string;
  balance: string;
  currency: string;
  updated_at: string;
}

export interface Transaction {
  transaction_id: string;
  client_id: string;
  type: string;
  amount: string;
  currency: string;
  balance_before: string;
  balance_after: string;
  description: string;
  message_id?: string;
  created_at: string;
}

export interface PricingRule {
  rule_id: string;
  client_id: string;
  destination_pattern: string;
  price_per_message: string;
  currency: string;
  priority: number;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface TemplateInfo {
  template_id: string;
  client_id: string;
  name: string;
  body: string;
  status: string;
  reviewer_id?: string;
  reviewed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface WebhookInfo {
  id: string;
  client_id: string;
  url: string;
  event_types: string[];
  active: boolean;
  secret?: string;
  created_at: string;
  updated_at: string;
}

export interface CountryInfo {
  country_id: string;
  name: string;
  code: string;
  phone_code: string;
  created_at: string;
  updated_at: string;
}

export interface OperatorInfo {
  operator_id: string;
  id?: string;
  name: string;
  code?: string;
  country_id: string;
  mcc: string;
  mnc: string;
  supports_paid_sender?: boolean;
  supports_free_sender?: boolean;
  monthly_tariff_amount?: string;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface OperatorPrefix {
  prefix_id: string;
  operator_id: string;
  prefix: string;
}

export interface TariffPlan {
  tariff_plan_id: string;
  name: string;
  description: string;
  operator_id: string;
  sender_category: string;
  strategy: string;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface HLRProvider {
  provider_id: string;
  name: string;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface AuditEntry {
  id: string;
  user_id: string;
  action: string;
  resource_type: string;
  resource_id: string;
  ip_address: string;
  details: Record<string, unknown>;
  created_at: string;
}

export interface TemplateAuditEntry {
  id: string;
  template_id: string;
  action: string;
  old_body?: string;
  new_body?: string;
  actor_id?: string;
  actor_type?: string;
  reason?: string;
  created_at: string;
}

export interface RealTimeMetrics {
  messages_per_second: number;
  messages_delivered: number;
  messages_failed: number;
  active_providers: number;
  queue_depth: number;
}

export interface GroupedStatGroup {
  label: string;
  sent: number;
  delivered: number;
  failed: number;
  delivery_rate: number;
  cost: number;
}

export interface GroupedStatsResponse {
  groups: GroupedStatGroup[];
  totals: {
    sent: number;
    delivered: number;
    failed: number;
    delivery_rate: number;
    cost: number;
  };
}

// ── API ──

export const clientsApi = {
  list: (params?: { active_only?: boolean; search?: string; limit?: number; offset?: number }) =>
    adminFetch<{ clients: ClientInfo[]; total: number; limit: number; offset: number }>(`/clients${qs(params || {})}`),
  get: (id: string) => adminFetch<{ client: ClientInfo }>(`/clients/${id}`),
  create: (data: { name: string; email: string; contact_person?: string; phone?: string; active?: boolean }) =>
    adminFetch<{ client_id: string; created_at: string }>('/clients', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: Partial<ClientInfo>) =>
    adminFetch<void>(`/clients/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) => adminFetch<void>(`/clients/${id}`, { method: 'DELETE' }),
  getConfig: (id: string) => adminFetch<unknown>(`/clients/${id}/config`),
  updateConfig: (id: string, data: unknown) =>
    adminFetch<void>(`/clients/${id}/config`, { method: 'PUT', body: JSON.stringify(data) }),
  updateRateLimits: (id: string, data: RateLimits) =>
    adminFetch<void>(`/clients/${id}/rate-limits`, { method: 'PUT', body: JSON.stringify(data) }),
};

export const providersApi = {
  list: (params?: { active_only?: boolean; limit?: number; offset?: number }) =>
    adminFetch<{ providers: ProviderInfo[]; total: number; limit: number; offset: number }>(`/providers${qs(params || {})}`),
  get: (id: string) => adminFetch<{ provider: ProviderInfo }>(`/providers/${id}`),
  create: (data: Partial<ProviderInfo> & { password: string }) =>
    adminFetch<{ provider_id: string; created_at: string }>('/providers', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: Partial<ProviderInfo>) =>
    adminFetch<void>(`/providers/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) => adminFetch<void>(`/providers/${id}`, { method: 'DELETE' }),
  health: (id: string) => adminFetch<ProviderHealth>(`/providers/${id}/health`),
};

export const routesApi = {
  list: (params?: { active_only?: boolean; limit?: number; offset?: number }) =>
    adminFetch<{ routes: RouteInfo[]; total: number; limit: number; offset: number }>(`/routes${qs(params || {})}`),
  create: (data: Partial<RouteInfo>) =>
    adminFetch<{ route_id: string; created_at: string }>('/routes', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: Partial<RouteInfo>) =>
    adminFetch<void>(`/routes/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) => adminFetch<void>(`/routes/${id}`, { method: 'DELETE' }),
};

export interface PlatformRoute {
  id: string;
  operator_id: string | null;       // null = All Networks
  operator_name: string;            // "All Networks" when operator_id is null
  channel_type: string;             // 'sms' | 'flash' | 'viber' | etc.
  provider_id: string;
  provider_name: string;
  legal_entity_id?: string;
  legal_entity_name?: string;
  legal_entity_inn?: string;
  priority: number;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface ConnectionInfo {
  provider_id: string;
  name: string;
  host: string;
  port: number;
  system_id: string;
  bind_type: number;
  max_connections: number;
  status: string;                   // 'healthy' | 'degraded' | 'unhealthy' | 'unknown'
  active_connections: number;
  success_rate: number;
  messages_sent_24h: number;
  messages_failed_24h: number;
  last_success?: string;
  last_failure?: string;
  last_error?: string;
  active: boolean;
  updated_at: string;
}

export const platformRoutesApi = {
  list: (params?: { limit?: number; offset?: number }) =>
    adminFetch<{ routes: PlatformRoute[]; total: number }>(`/platform-routes${qs(params || {})}`),
  create: (data: {
    operator_id?: string;
    channel_type: string;
    provider_id: string;
    legal_entity_id?: string;
    priority?: number;
  }) =>
    adminFetch<{ id: string; created_at: string }>('/platform-routes', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  update: (id: string, data: Partial<{
    operator_id: string;
    channel_type: string;
    provider_id: string;
    legal_entity_id: string;
    priority: number;
    active: boolean;
  }>) =>
    adminFetch<void>(`/platform-routes/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) => adminFetch<void>(`/platform-routes/${id}`, { method: 'DELETE' }),
  reorder: (items: Array<{ id: string; priority: number }>) =>
    adminFetch<{ updated: number }>('/platform-routes/reorder', {
      method: 'PUT',
      body: JSON.stringify({ items }),
    }),
};

export const connectionsApi = {
  list: () => adminFetch<{ connections: ConnectionInfo[]; total: number }>('/connections'),
  get: (id: string) => adminFetch<ConnectionInfo>(`/connections/${id}`),
  reconnect: (id: string) =>
    adminFetch<{ queued: boolean }>(`/connections/${id}/reconnect`, { method: 'POST' }),
  stop: (id: string) =>
    adminFetch<{ queued: boolean }>(`/connections/${id}/stop`, { method: 'POST' }),
};

export const billingApi = {
  getBalance: (clientId: string) => adminFetch<BalanceResponse>(`/billing/clients/${clientId}/balance`),
  addCredits: (clientId: string, data: { amount: string; currency?: string; description?: string }) =>
    adminFetch<{ transaction_id: string; new_balance: string; success: boolean }>(`/billing/clients/${clientId}/credits`, {
      method: 'POST', body: JSON.stringify(data),
    }),
  getTransactions: (params?: { client_id?: string; from?: string; to?: string; transaction_type?: string; limit?: number; offset?: number }) =>
    adminFetch<{ transactions: Transaction[]; total: number; limit: number; offset: number }>(`/billing/transactions${qs(params || {})}`),
  getPricingRules: (params?: { client_id?: string }) =>
    adminFetch<{ rules: PricingRule[] }>(`/billing/pricing-rules${qs(params || {})}`),
  createPricingRule: (data: Partial<PricingRule>) =>
    adminFetch<void>('/billing/pricing-rules', { method: 'POST', body: JSON.stringify(data) }),
  freeze: (clientId: string) =>
    adminFetch<void>(`/billing/clients/${clientId}/freeze`, { method: 'POST' }),
  unfreeze: (clientId: string) =>
    adminFetch<void>(`/billing/clients/${clientId}/unfreeze`, { method: 'POST' }),
  setCreditLimit: (clientId: string, data: { credit_limit: string }) =>
    adminFetch<void>(`/billing/clients/${clientId}/credit-limit`, { method: 'PUT', body: JSON.stringify(data) }),
  setLowBalanceThreshold: (clientId: string, data: { threshold: string }) =>
    adminFetch<void>(`/billing/clients/${clientId}/low-balance-threshold`, { method: 'PUT', body: JSON.stringify(data) }),
  listBalances: (params?: { search?: string; status?: string; below_threshold?: boolean; limit?: number; offset?: number }) =>
    adminFetch<{ balances: BalanceInfoItem[]; total: number; limit: number; offset: number }>(`/billing/balances${qs(params || {})}`),
};

export const analyticsAdminApi = {
  getStats: (params: { client_id?: string; from?: string; to?: string; group_by?: string }) =>
    adminFetch<unknown>(`/analytics/stats${qs(params)}`),
  generateReport: (data: unknown) =>
    adminFetch<unknown>('/analytics/reports', { method: 'POST', body: JSON.stringify(data) }),
  getRealTimeMetrics: () => adminFetch<RealTimeMetrics>('/analytics/metrics/realtime'),
  getProviderPerformance: (id: string, params?: { from?: string; to?: string }) =>
    adminFetch<unknown>(`/analytics/providers/${id}/performance${qs(params || {})}`),
  getGroupedStats: (params: { from: string; to: string; group_by: 'day' | 'operator' | 'country'; client_id?: string }) =>
    adminFetch<GroupedStatsResponse>(`/analytics/stats${qs(params)}`),
};

export const webhooksAdminApi = {
  list: (params: { client_id: string }) =>
    adminFetch<{ subscriptions: WebhookInfo[] }>(`/webhooks${qs(params)}`),
  get: (id: string, clientId: string) =>
    adminFetch<WebhookInfo>(`/webhooks/${id}${qs({ client_id: clientId })}`),
  create: (data: { client_id: string; url: string; event_types: string[] }) =>
    adminFetch<WebhookInfo>('/webhooks', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: { client_id: string; url?: string; event_types?: string[]; active?: boolean }) =>
    adminFetch<WebhookInfo>(`/webhooks/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string, clientId: string) =>
    adminFetch<void>(`/webhooks/${id}${qs({ client_id: clientId })}`, { method: 'DELETE' }),
};

export const templatesApi = {
  list: (params?: { client_id?: string; status?: string; limit?: number; offset?: number }) =>
    adminFetch<{ templates: TemplateInfo[]; total: number; limit: number; offset: number }>(`/templates${qs(params || {})}`),
  get: (id: string, clientId?: string) =>
    adminFetch<{ template: TemplateInfo }>(`/templates/${id}${qs({ client_id: clientId })}`),
  approve: (id: string) =>
    adminFetch<void>(`/templates/${id}/approve`, { method: 'POST' }),
  reject: (id: string, data?: { reason?: string }) =>
    adminFetch<void>(`/templates/${id}/reject`, { method: 'POST', body: JSON.stringify(data || {}) }),
  audit: (id: string) =>
    adminFetch<{ entries: TemplateAuditEntry[] }>(`/templates/${id}/audit`),
  assign: (id: string) =>
    adminFetch<void>(`/templates/${id}/assign`, { method: 'POST' }),
  requestRevision: (id: string, data: { comment: string }) =>
    adminFetch<void>(`/templates/${id}/request-revision`, { method: 'POST', body: JSON.stringify(data) }),
};

export const countriesApi = {
  list: (params?: { limit?: number; offset?: number }) =>
    adminFetch<{ countries: CountryInfo[]; total: number; limit: number; offset: number }>(`/countries${qs(params || {})}`),
  get: (id: string) => adminFetch<{ country: CountryInfo }>(`/countries/${id}`),
  create: (data: Partial<CountryInfo>) =>
    adminFetch<void>('/countries', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: Partial<CountryInfo>) =>
    adminFetch<void>(`/countries/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
};

export const operatorsApi = {
  list: (params?: { country_id?: string; active_only?: boolean; limit?: number; offset?: number }) =>
    adminFetch<{ operators: OperatorInfo[]; total: number; limit: number; offset: number }>(`/operators${qs(params || {})}`),
  get: (id: string) => adminFetch<{ operator: OperatorInfo }>(`/operators/${id}`),
  create: (data: Partial<OperatorInfo>) =>
    adminFetch<void>('/operators', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: Partial<OperatorInfo>) =>
    adminFetch<void>(`/operators/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  listPrefixes: (id: string) =>
    adminFetch<{ prefixes: OperatorPrefix[] }>(`/operators/${id}/prefixes`),
  createPrefix: (id: string, data: { prefix: string }) =>
    adminFetch<void>(`/operators/${id}/prefixes`, { method: 'POST', body: JSON.stringify(data) }),
  deletePrefix: (operatorId: string, prefixId: string) =>
    adminFetch<void>(`/operators/${operatorId}/prefixes/${prefixId}`, { method: 'DELETE' }),
};

export const tarificationApi = {
  listTariffPlans: () =>
    adminFetch<{ tariff_plans: TariffPlan[] }>('/tarification/tariff-plans'),
  createTariffPlan: (data: { operator_id: string; sender_category: string; strategy: string }) =>
    adminFetch<void>('/tarification/tariff-plans', { method: 'POST', body: JSON.stringify(data) }),
  updateTariffPlan: (id: string, data: Partial<TariffPlan>) =>
    adminFetch<void>(`/tarification/tariff-plans/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  listUsage: () => adminFetch<unknown>('/tarification/usage'),
  listTariffPeriods: (params?: { tariff_plan_id?: string }) =>
    adminFetch<{ periods: TariffPeriod[]; total: number }>(`/tarification/tariff-periods${qs(params || {})}`),
  createTariffPeriod: (data: { tariff_plan_id: string; start_date: string; end_date: string }) =>
    adminFetch<void>('/tarification/tariff-periods', { method: 'POST', body: JSON.stringify(data) }),
  listTariffTiers: (params?: { tariff_period_id?: string }) =>
    adminFetch<{ tiers: TariffTier[]; total: number }>(`/tarification/tariff-tiers${qs(params || {})}`),
  createTariffTier: (data: { tariff_period_id: string; from_count: number; price_per_segment: string }) =>
    adminFetch<void>('/tarification/tariff-tiers', { method: 'POST', body: JSON.stringify(data) }),
  updateTariffTier: (id: string, data: { from_count: number; price_per_segment: string }) =>
    adminFetch<void>(`/tarification/tariff-tiers/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  listSenderRegistrations: (params?: { client_id?: string; operator_id?: string; limit?: number; offset?: number }) =>
    adminFetch<{ registrations: SenderRegistration[]; total: number }>(`/tarification/sender-registrations${qs(params || {})}`),
  createSenderRegistration: (data: { client_id: string; operator_id: string; sender_name: string; type: string }) =>
    adminFetch<void>('/tarification/sender-registrations', { method: 'POST', body: JSON.stringify(data) }),
  updateSenderRegistration: (id: string, data: { status: string; type: string }) =>
    adminFetch<void>(`/tarification/sender-registrations/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  listPeriods: (params?: { country_id?: string; operator_id?: string; sender_category?: string; traffic_type?: string; client_id?: string }) =>
    adminFetch<{ periods: HierarchicalPeriod[]; total: number }>(`/tarification/periods${qs(params || {})}`),
  createPeriod: (data: CreateHierarchicalPeriodRequest) =>
    adminFetch<{ period: HierarchicalPeriod; auto_close_warning?: AutoCloseWarning }>('/tarification/periods', { method: 'POST', body: JSON.stringify(data) }),
  updatePeriod: (id: string, data: UpdateHierarchicalPeriodRequest) =>
    adminFetch<{ period: HierarchicalPeriod }>(`/tarification/periods/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deletePeriod: (id: string) =>
    adminFetch<void>(`/tarification/periods/${id}`, { method: 'DELETE' }),
  listPeriodTiers: (periodId: string) =>
    adminFetch<{ tiers: TariffTier[]; total: number }>(`/tarification/periods/${periodId}/tiers`),
  createPeriodTier: (periodId: string, data: { from_count: number; price_per_segment: string }) =>
    adminFetch<{ tier: TariffTier }>(`/tarification/periods/${periodId}/tiers`, { method: 'POST', body: JSON.stringify(data) }),
  updatePeriodTier: (periodId: string, tierId: string, data: { from_count: number; price_per_segment: string }) =>
    adminFetch<{ tier: TariffTier }>(`/tarification/periods/${periodId}/tiers/${tierId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deletePeriodTier: (periodId: string, tierId: string) =>
    adminFetch<void>(`/tarification/periods/${periodId}/tiers/${tierId}`, { method: 'DELETE' }),
};

export const hlrApi = {
  listProviders: (params?: { active_only?: boolean }) =>
    adminFetch<{ providers: HLRProvider[] }>(`/hlr/providers${qs(params || {})}`),
  getProvider: (id: string) => adminFetch<{ provider: HLRProvider }>(`/hlr/providers/${id}`),
  createProvider: (data: Partial<HLRProvider>) =>
    adminFetch<void>('/hlr/providers', { method: 'POST', body: JSON.stringify(data) }),
  updateProvider: (id: string, data: Partial<HLRProvider>) =>
    adminFetch<void>(`/hlr/providers/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteProvider: (id: string) => adminFetch<void>(`/hlr/providers/${id}`, { method: 'DELETE' }),
  providerHealth: (id: string) => adminFetch<ProviderHealth>(`/hlr/providers/${id}/health`),
};

export const auditAdminApi = {
  list: (params?: { user_id?: string; action?: string; from?: string; to?: string; limit?: number; offset?: number }) =>
    adminFetch<{ entries: AuditEntry[]; total: number; limit: number; offset: number }>(`/audit${qs(params || {})}`),
};

// ── Extended Types ──

export interface UserDetailInfo {
  id: string;
  username: string;
  email: string;
  role: { id: string; name: string; description: string };
  active: boolean;
  totp_enabled: boolean;
  last_login_at: string;
  created_at: string;
  updated_at: string;
}

export interface RoleDetail {
  id: string;
  name: string;
  description: string;
  builtin: boolean;
  user_count: number;
  permissions: PermissionInfo[];
  created_at: string;
  updated_at: string;
}

export interface PermissionInfo {
  id: string;
  resource: string;
  action: string;
}

export interface BalanceInfoItem {
  client_id: string;
  client_name: string;
  balance: string;
  currency: string;
  frozen: boolean;
  credit_limit: string;
  low_balance_threshold: string;
  frozen_at: string;
  frozen_by: string;
  updated_at: string;
}

export interface TariffPeriod {
  id: string;
  tariff_plan_id: string;
  start_date: string;
  end_date: string;
  created_at: string;
}

export interface HierarchicalPeriod {
  id: string;
  country_id: string | null;
  operator_id: string | null;
  sender_category: string | null;
  traffic_type: string | null;
  client_id: string | null;
  scope_key: string;
  scope_priority: number;
  strategy: string;
  start_date: string;
  end_date: string | null;
  created_at: string;
}

export interface AutoCloseWarning {
  period_id: string;
  new_end_date: string;
}

export interface CreateHierarchicalPeriodRequest {
  country_id?: string | null;
  operator_id?: string | null;
  sender_category?: string | null;
  traffic_type?: string | null;
  client_id?: string | null;
  strategy: string;
  start_date: string;
  end_date?: string | null;
}

export interface UpdateHierarchicalPeriodRequest {
  strategy?: string;
  end_date?: string | null;
}

export interface TariffTier {
  id: string;
  tariff_period_id: string;
  from_count: number;
  price_per_segment: string;
}

export interface SenderRegistration {
  id: string;
  client_id: string;
  operator_id: string;
  sender_name: string;
  type: string;
  status: string;
  created_at: string;
  updated_at: string;
}

// ── Users API ──

export const usersApi = {
  list: (params?: { search?: string; role_id?: string; active_only?: boolean; limit?: number; offset?: number }) =>
    adminFetch<{ users: UserDetailInfo[]; total: number; limit: number; offset: number }>(`/users${qs(params || {})}`),
  get: (id: string) => adminFetch<{ user: UserDetailInfo }>(`/users/${id}`),
  create: (data: { username: string; email: string; password: string; role_id: string; active?: boolean; client_id?: string }) =>
    adminFetch<{ user: UserDetailInfo }>('/users', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: { email?: string; role_id?: string; active?: boolean }) =>
    adminFetch<{ user: UserDetailInfo }>(`/users/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deactivate: (id: string) =>
    adminFetch<void>(`/users/${id}/deactivate`, { method: 'POST' }),
  reset2fa: (id: string) =>
    adminFetch<void>(`/users/${id}/reset-2fa`, { method: 'POST' }),
  resetPassword: (id: string) =>
    adminFetch<{ temporary_password: string }>(`/users/${id}/reset-password`, { method: 'POST' }),
};

// ── Roles API ──

export const rolesApi = {
  list: (params?: { limit?: number; offset?: number }) =>
    adminFetch<{ roles: RoleDetail[]; total: number }>(`/roles${qs(params || {})}`),
  get: (id: string) => adminFetch<{ role: RoleDetail }>(`/roles/${id}`),
  create: (data: { name: string; description: string; permission_ids: string[] }) =>
    adminFetch<{ role: RoleDetail }>('/roles', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: { name: string; description: string; permission_ids: string[] }) =>
    adminFetch<{ role: RoleDetail }>(`/roles/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) => adminFetch<void>(`/roles/${id}`, { method: 'DELETE' }),
};

// ── Permissions API ──

export const permissionsApi = {
  list: () => adminFetch<{ permissions: PermissionInfo[] }>('/permissions'),
};

// ── Client Routes API ──

export interface ClientRoute {
  id: string;
  client_id: string;
  operator_id: string;
  provider_id: string;
  priority: number;
  weight: number;
  active: boolean;
  shared: boolean;
}

export const clientRoutesApi = {
  list: (clientId: string, params?: { operator_id?: string }) =>
    adminFetch<{ routes: ClientRoute[] }>(`/clients/${clientId}/routes${qs(params || {})}`),
  create: (clientId: string, data: { operator_id: string; provider_id: string; priority: number; weight: number }) =>
    adminFetch<ClientRoute>(`/clients/${clientId}/routes`, { method: 'POST', body: JSON.stringify(data) }),
  update: (clientId: string, routeId: string, data: { priority: number; weight: number; active: boolean }) =>
    adminFetch<ClientRoute>(`/clients/${clientId}/routes/${routeId}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (clientId: string, routeId: string) =>
    adminFetch<void>(`/clients/${clientId}/routes/${routeId}`, { method: 'DELETE' }),
};

// ── System Defaults API ──

export interface SystemDefault {
  key: string;
  value: number;
  description?: string;
}

export const systemDefaultsApi = {
  getAll: () => adminFetch<Record<string, number>>('/system/defaults'),
  set: (key: string, value: number) =>
    adminFetch<void>(`/system/defaults/${key}`, { method: 'PUT', body: JSON.stringify({ value }) }),
};

// ── Sender Names Admin API ──

export interface SenderNameOperatorRegistration {
  operator_id: string;
  operator_name: string;
  mcc: string;
  mnc: string;
  status: 'not_registered' | 'pending' | 'registered' | 'rejected';
  registered_at?: string;
}

export interface AdminSenderNameInfo {
  id: string;
  client_id: string;
  name: string;
  status: 'pending' | 'approved' | 'rejected' | 'deactivated';
  rejection_reason?: string;
  reviewer_id?: string;
  reviewed_at?: string;
  created_at: string;
  updated_at: string;
}

export const adminSenderNamesApi = {
  list: (params?: { client_id?: string; status?: string; name_query?: string; limit?: number; offset?: number }) =>
    adminFetch<{ sender_names: AdminSenderNameInfo[]; total: number; limit: number; offset: number }>(
      `/sender-names${qs(params || {})}`,
    ),
  get: (id: string) =>
    adminFetch<{ sender_name: AdminSenderNameInfo }>(`/sender-names/${id}`),
  approve: (id: string) =>
    adminFetch<AdminSenderNameInfo>(`/sender-names/${id}/approve`, { method: 'POST' }),
  reject: (id: string, reason: string) =>
    adminFetch<AdminSenderNameInfo>(`/sender-names/${id}/reject`, {
      method: 'POST',
      body: JSON.stringify({ reason }),
    }),
  deactivate: (id: string, reason: string) =>
    adminFetch<AdminSenderNameInfo>(`/sender-names/${id}/deactivate`, {
      method: 'POST',
      body: JSON.stringify({ reason }),
    }),
  operatorRegistrations: (id: string) =>
    adminFetch<{ registrations: SenderNameOperatorRegistration[] }>(`/sender-names/${id}/operator-registrations`),
};

// ── Sender Names lightweight type (for selectors) ──

export interface SenderNameInfo {
  sender_name_id: string;
  name: string;
  status: string;
}

export const senderNamesApi = {
  list: (params?: { client_id?: string; status?: string; limit?: number }) =>
    adminFetch<{ sender_names: SenderNameInfo[]; total: number }>(
      `/sender-names${qs(params || {})}`,
    ),
};

// ── Admin Messages API (Детализация) ──

export interface AdminMessage {
  id: string;
  source: string;
  destination: string;
  text_preview: string;
  status: string;
  segment_count: number;
  created_at: string;
  delivered_at?: string;
  failed_at?: string;
  provider_name: string;
  client_name: string;
}

export interface AdminMessageDetail {
  id: string;
  source: string;
  destination: string;
  text: string;
  encoding?: string;
  status: string;
  status_message?: string;
  external_id?: string;
  segment_count: number;
  retry_count?: number;
  max_retries?: number;
  provider_id?: string;
  provider_name?: string;
  route_id?: string;
  route_name?: string;
  smpp_message_id?: string;
  client_id: string;
  client_name: string;
  created_at?: string;
  submitted_at?: string;
  delivered_at?: string;
  failed_at?: string;
  scheduled_at?: string;
  expired_at?: string;
  dlr?: {
    stat: string;
    err: number;
    text: string;
    submit_date?: string;
    done_date?: string;
    receipted_message_id?: string;
  };
  billing?: {
    segment_count: number;
    price_per_segment: string;
    total_amount: string;
    tariff_plan_id: string;
    billed_at: string;
  };
}

export const messagesApi = {
  list: (params?: {
    client_id?: string;
    status?: string;
    source?: string;
    destination?: string;
    provider_id?: string;
    date_from?: string;
    date_to?: string;
    limit?: number;
    offset?: number;
  }) =>
    adminFetch<{ messages: AdminMessage[]; total: number; limit: number; offset: number }>(
      `/messages${qs(params || {})}`,
    ),
  get: (id: string) => adminFetch<AdminMessageDetail>(`/messages/${id}`),
};

// ── Legal Entities API ──

export interface LegalEntity {
  id: string;
  inn: string;
  name: string;
  full_name?: string;
  address?: string;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export const legalEntitiesApi = {
  list: (params?: { limit?: number; offset?: number }) =>
    adminFetch<{ legal_entities: LegalEntity[]; total: number }>(
      `/legal-entities${qs(params || {})}`,
    ),
  get: (id: string) => adminFetch<LegalEntity>(`/legal-entities/${id}`),
  create: (data: { inn: string; name: string; full_name?: string; address?: string }) =>
    adminFetch<LegalEntity>('/legal-entities', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: { inn?: string; name?: string; full_name?: string; address?: string; active?: boolean }) =>
    adminFetch<LegalEntity>(`/legal-entities/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) => adminFetch<void>(`/legal-entities/${id}`, { method: 'DELETE' }),
};

// ── Contracts API ──

export interface Contract {
  id: string;
  contract_number: string;
  client_id: string;
  client_name: string;
  legal_entity_id?: string;
  legal_entity_inn?: string;
  legal_entity_name?: string;
  status: string;
  start_date: string;
  end_date?: string;
  description?: string;
  created_at: string;
  updated_at: string;
}

export const contractsApi = {
  list: (params?: { client_id?: string; status?: string; limit?: number; offset?: number }) =>
    adminFetch<{ contracts: Contract[]; total: number }>(
      `/contracts${qs(params || {})}`,
    ),
  get: (id: string) => adminFetch<Contract>(`/contracts/${id}`),
  create: (data: {
    contract_number: string;
    client_id: string;
    legal_entity_id?: string;
    status?: string;
    start_date: string;
    end_date?: string;
    description?: string;
  }) => adminFetch<Contract>('/contracts', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: {
    contract_number?: string;
    legal_entity_id?: string;
    status?: string;
    start_date?: string;
    end_date?: string;
    description?: string;
  }) => adminFetch<Contract>(`/contracts/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) => adminFetch<void>(`/contracts/${id}`, { method: 'DELETE' }),
};

// ── Operator Templates API ──

export interface OperatorTemplate {
  id: string;
  name: string;
  operator_id: string;
  operator_name: string;
  sender_name_id?: string;
  sender_name?: string;
  body: string;
  variables: string[];
  status: string;
  created_at: string;
  updated_at: string;
}

// ── Aggregator Quotas API ──

export interface AggregatorQuota {
  quota_id: string;
  aggregator_id: string;
  period_start: string;
  period_end: string;
  segment_limit: number;
  segments_used: number;
  overage_rate: string;
  currency: string;
  auto_renew: boolean;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export const aggregatorQuotasApi = {
  list: (aggregatorId: string, params?: { limit?: number; offset?: number }) =>
    adminFetch<{ quotas: AggregatorQuota[]; total: number }>(
      `/aggregators/${aggregatorId}/quotas${qs(params || {})}`,
    ),
  getActive: (aggregatorId: string) =>
    adminFetch<{ quota: AggregatorQuota }>(`/aggregators/${aggregatorId}/quotas/active`),
  create: (aggregatorId: string, data: {
    period_start: string;
    period_end: string;
    segment_limit: number;
    overage_rate: string;
    currency?: string;
    auto_renew?: boolean;
  }) =>
    adminFetch<{ quota: AggregatorQuota }>(`/aggregators/${aggregatorId}/quotas`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  update: (aggregatorId: string, quotaId: string, data: {
    segment_limit: number;
    overage_rate: string;
    auto_renew: boolean;
  }) =>
    adminFetch<{ quota: AggregatorQuota }>(`/aggregators/${aggregatorId}/quotas/${quotaId}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
};

export const operatorTemplatesApi = {
  list: (params?: { operator_id?: string; sender_name_id?: string; status?: string; limit?: number; offset?: number }) =>
    adminFetch<{ operator_templates: OperatorTemplate[]; total: number; limit: number; offset: number }>(
      `/operator-templates${qs(params || {})}`,
    ),
  get: (id: string) => adminFetch<OperatorTemplate>(`/operator-templates/${id}`),
  create: (data: {
    name: string;
    operator_id: string;
    sender_name_id?: string;
    body: string;
    variables?: string[];
    status?: string;
  }) => adminFetch<OperatorTemplate>('/operator-templates', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: {
    name?: string;
    sender_name_id?: string;
    body?: string;
    variables?: string[];
    status?: string;
  }) => adminFetch<OperatorTemplate>(`/operator-templates/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) => adminFetch<void>(`/operator-templates/${id}`, { method: 'DELETE' }),
};
