const API_BASE = '/admin/v1';

function getCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp(`(^| )${name}=([^;]+)`));
  return match ? match[2] : null;
}

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
  webhook_id: string;
  client_id: string;
  url: string;
  events: string[];
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
  name: string;
  country_id: string;
  mcc: string;
  mnc: string;
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

export interface SmartRouteWeight {
  weight_id: string;
  country_code: string;
  provider_id: string;
  weight: number;
  created_at: string;
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
};

export const webhooksAdminApi = {
  list: (params?: { client_id?: string }) =>
    adminFetch<{ webhooks: WebhookInfo[] }>(`/webhooks${qs(params || {})}`),
  get: (id: string, clientId?: string) =>
    adminFetch<{ webhook: WebhookInfo }>(`/webhooks/${id}${qs({ client_id: clientId })}`),
  create: (data: Partial<WebhookInfo>) =>
    adminFetch<void>('/webhooks', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: Partial<WebhookInfo>) =>
    adminFetch<void>(`/webhooks/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string, clientId?: string) =>
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
  createTariffPlan: (data: Partial<TariffPlan>) =>
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
  listWeights: (params?: { country_code?: string }) =>
    adminFetch<{ weights: SmartRouteWeight[] }>(`/routing/weights${qs(params || {})}`),
  setWeights: (data: { country_code: string; provider_id: string; weight: number }) =>
    adminFetch<void>('/routing/weights', { method: 'POST', body: JSON.stringify(data) }),
  deleteWeight: (id: string) => adminFetch<void>(`/routing/weights/${id}`, { method: 'DELETE' }),
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
  create: (data: { username: string; email: string; password: string; role_id: string; active?: boolean }) =>
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

// ── Sender Names Admin API ──

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
};
