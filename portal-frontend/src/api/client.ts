import { getCookie } from '../utils/cookies';

const API_BASE = '/portal/v1';

export async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const csrfToken = getCookie('csrf_token');
  const isFormData = options?.body instanceof FormData;
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), isFormData ? 120_000 : 30_000);
  let res: Response;
  try {
    res = await fetch(`${API_BASE}${path}`, {
      credentials: 'include',
      signal: controller.signal,
      ...options,
      headers: {
        ...(isFormData ? {} : { 'Content-Type': 'application/json' }),
        ...(csrfToken ? { 'X-CSRF-Token': csrfToken } : {}),
        ...options?.headers,
      },
    });
  } catch (err) {
    clearTimeout(timeoutId);
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw new ApiError(0, 'Превышено время ожидания ответа от сервера');
    }
    throw err;
  }
  clearTimeout(timeoutId);
  const PUBLIC_PATHS = ['/', '/pricing', '/features', '/docs', '/blog', '/about', '/contact', '/en'];
  const isPublicPage = PUBLIC_PATHS.some(p =>
    window.location.pathname === p || window.location.pathname.startsWith('/docs/') ||
    window.location.pathname.startsWith('/blog/') || window.location.pathname.startsWith('/en/')
  );

  if (!res.ok) {
    if (res.status === 401 && !path.startsWith('/auth/')) {
      if (window.location.pathname !== '/login' && !isPublicPage) {
        window.location.href = '/login';
      }
      throw new ApiError(401, 'Unauthorized');
    }
    const err = await res.json().catch(() => ({ error: { message: res.statusText } }));
    let msg = err.error?.message || res.statusText;
    if (typeof msg === 'string') {
      msg = msg.replace('parent client not found', 'Parent client not found');
    }
    throw new ApiError(res.status, msg, err.error);
  }
  if (res.status === 204) return {} as T;
  return res.json();
}

export class ApiError extends Error {
  constructor(public status: number, message: string, public details?: unknown) {
    super(message);
  }
}

// Auth API
export const authApi = {
  login: (email: string, password: string) =>
    apiFetch<{ requires_2fa?: boolean; login_ticket?: string }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),
  login2fa: (loginTicket: string, totpCode: string) =>
    apiFetch<unknown>('/auth/login/2fa', {
      method: 'POST',
      body: JSON.stringify({ login_ticket: loginTicket, totp_code: totpCode }),
    }),
  logout: () => apiFetch<void>('/auth/logout', { method: 'POST' }),
  requestPasswordReset: (email: string) =>
    apiFetch<void>('/auth/password/reset-request', {
      method: 'POST',
      body: JSON.stringify({ email }),
    }),
  resetPassword: (token: string, newPassword: string) =>
    apiFetch<void>('/auth/password/reset', {
      method: 'POST',
      body: JSON.stringify({ token, new_password: newPassword }),
    }),
  register: (data: {
    email: string;
    password: string;
    company_name: string;
    contact_person?: string;
    phone?: string;
    plan_name: string;
  }) =>
    apiFetch<{ client_id: string; user: { id: string; email: string; role: string } }>('/auth/register', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
};

// Profile API
export const profileApi = {
  get: () => apiFetch<ProfileData>('/profile'),
  update: (data: { contact_person?: string; phone?: string }) =>
    apiFetch<ProfileData>('/profile', { method: 'PUT', body: JSON.stringify(data) }),
  toggleSandbox: (enable: boolean) =>
    apiFetch<{ is_sandbox: boolean }>('/profile/sandbox', {
      method: 'PUT',
      body: JSON.stringify({ enable }),
    }),
  setupTOTP: () =>
    apiFetch<{ secret: string; qr_code_url: string }>('/profile/2fa/setup', { method: 'POST' }),
  verifyTOTP: (code: string) =>
    apiFetch<unknown>('/profile/2fa/verify', {
      method: 'POST',
      body: JSON.stringify({ totp_code: code }),
    }),
  disableTOTP: (password: string) =>
    apiFetch<void>('/profile/2fa', { method: 'DELETE', body: JSON.stringify({ password }) }),
  changePassword: (currentPassword: string, newPassword: string) =>
    apiFetch<unknown>('/profile/password', {
      method: 'PUT',
      body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
    }),
};

export interface ProfileData {
  id: string;
  email: string;
  company_name: string;
  contact_person: string;
  phone: string;
  totp_enabled: boolean;
  is_sandbox?: boolean;
  role?: 'client' | 'admin' | 'superadmin';
  is_reseller?: boolean;
  parent_client_id?: string;
  max_sub_accounts?: number;
}

// Dashboard API
export const dashboardApi = {
  get: () => apiFetch<unknown>('/dashboard'),
};

// Messages API
export const messagesApi = {
  list: (params: Record<string, string>) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<unknown>(`/messages?${qs}`);
  },
  send: (data: { destination: string; text: string; source: string }) =>
    apiFetch<{ message_id: string; status: string }>('/messages', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  get: (id: string) => apiFetch<unknown>(`/messages/${id}`),
};

// API Keys API
export interface APIKeyInfo {
  id: string;
  name: string;
  prefix: string;
  active: boolean;
  scopes: string[] | null;
  allowed_ips: string[] | null;
  created_at: string;
  expires_at?: string;
  last_used_at?: string;
  revoke_at?: string;
}

export interface CreateAPIKeyRequest {
  name: string;
  allowed_ips?: string[];
  scopes?: string[];
  expires_at?: string;
}

export interface CreateAPIKeyResponse {
  api_key: string;
  api_key_id: string;
  created_at: string;
  expires_at?: string;
}

export interface RotateAPIKeyResponse {
  api_key: string;
  api_key_id: string;
  created_at: string;
  expires_at?: string;
  old_key_revoke_at?: string;
}

export const apiKeysApi = {
  list: () => apiFetch<{ keys: APIKeyInfo[] }>('/api-keys'),
  create: (data: CreateAPIKeyRequest) =>
    apiFetch<CreateAPIKeyResponse>('/api-keys', { method: 'POST', body: JSON.stringify(data) }),
  revoke: (id: string) => apiFetch<void>(`/api-keys/${id}`, { method: 'DELETE' }),
  get: (id: string) => apiFetch<APIKeyInfo>(`/api-keys/${id}`),
  update: (id: string, data: { name: string; scopes: string[]; allowed_ips: string[]; expires_at?: string }) =>
    apiFetch<APIKeyInfo>(`/api-keys/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  rotate: (id: string) =>
    apiFetch<RotateAPIKeyResponse>(`/api-keys/${id}/rotate`, { method: 'POST' }),
};

// Webhooks API
export const webhooksApi = {
  list: () => apiFetch<unknown>('/webhooks'),
  create: (data: Record<string, unknown>) =>
    apiFetch<unknown>('/webhooks', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: Record<string, unknown>) =>
    apiFetch<unknown>(`/webhooks/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  remove: (id: string) => apiFetch<void>(`/webhooks/${id}`, { method: 'DELETE' }),
  test: (id: string) => apiFetch<unknown>(`/webhooks/${id}/test`, { method: 'POST' }),
};

// Analytics API
export const analyticsApi = {
  get: (params: Record<string, string>, options?: RequestInit) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<unknown>(`/analytics?${qs}`, options);
  },
};

// Sub-accounts API
export const subAccountsApi = {
  list: () => apiFetch<unknown>('/sub-accounts'),
  create: (data: Record<string, unknown>) =>
    apiFetch<unknown>('/sub-accounts', { method: 'POST', body: JSON.stringify(data) }),
  get: (id: string) => apiFetch<unknown>(`/sub-accounts/${id}`),
  updateLimits: (id: string, data: Record<string, unknown>) =>
    apiFetch<unknown>(`/sub-accounts/${id}/limits`, { method: 'PUT', body: JSON.stringify(data) }),
  transfer: (id: string, amount: string) =>
    apiFetch<unknown>(`/sub-accounts/${id}/transfer`, {
      method: 'POST',
      body: JSON.stringify({ amount }),
    }),
  remove: (id: string) => apiFetch<unknown>(`/sub-accounts/${id}`, { method: 'DELETE' }),
  messages: (id: string, params: Record<string, string>) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<unknown>(`/sub-accounts/${id}/messages?${qs}`);
  },
  analytics: (id: string, params: Record<string, string>) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<unknown>(`/sub-accounts/${id}/analytics?${qs}`);
  },
  apiKeys: (id: string) => apiFetch<unknown>(`/sub-accounts/${id}/api-keys`),
  webhooks: (id: string) => apiFetch<unknown>(`/sub-accounts/${id}/webhooks`),
  campaigns: (id: string, params?: Record<string, string>) => {
    const qs = params ? new URLSearchParams(params).toString() : '';
    return apiFetch<{ campaigns: Array<{ id: string; name: string; status: string; total_recipients: number; delivered_count: number; created_at: string }>; total: number }>(`/sub-accounts/${id}/campaigns${qs ? `?${qs}` : ''}`);
  },
  transactions: (id: string, params: Record<string, string> = {}) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<{ transactions: Array<{ transaction_id: string; type: string; amount: string; currency: string; balance_before: string; balance_after: string; description: string; message_id?: string; created_at: string }>; total: number; page: number; per_page: number; total_pages: number }>(
      `/sub-accounts/${id}/transactions${qs ? `?${qs}` : ''}`,
    );
  },
};

// Audit API
export const auditApi = {
  list: (params: Record<string, string>) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<unknown>(`/audit-log?${qs}`);
  },
};

// Providers API
export interface RoutingRule {
  pattern: string;
  priority: number;
}

export interface Provider {
  id: string;
  client_id: string;
  name: string;
  description: string;
  tags: string[];
  host: string;
  port: number;
  system_id: string;
  bind_type: number;
  window_size: number;
  max_connections: number;
  tps_limit: number;
  active: boolean;
  routing_rules: RoutingRule[];
  created_at: string;
  updated_at: string;
}

export interface CreateProviderRequest {
  name: string;
  description?: string;
  tags?: string[];
  host: string;
  port: number;
  system_id: string;
  password: string;
  bind_type: number;
  window_size?: number;
  max_connections?: number;
  tps_limit?: number;
  routing_rules?: RoutingRule[];
}

export interface TestConnectionRequest {
  host: string;
  port: number;
  system_id: string;
  password: string;
  bind_type: number;
}

export interface TestConnectionResult {
  success: boolean;
  latency_ms: number;
  log: string[];
  error: string;
}

export const providersApi = {
  list: () => apiFetch<{ providers: Provider[] }>('/providers'),
  create: (data: CreateProviderRequest) =>
    apiFetch<Provider>('/providers', { method: 'POST', body: JSON.stringify(data) }),
  get: (id: string) => apiFetch<Provider>(`/providers/${id}`),
  update: (id: string, data: Partial<CreateProviderRequest>) =>
    apiFetch<Provider>(`/providers/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  remove: (id: string) => apiFetch<void>(`/providers/${id}`, { method: 'DELETE' }),
  testConnection: (data: TestConnectionRequest) =>
    apiFetch<TestConnectionResult>('/providers/test-connection', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
};

// Routing API
export interface OperatorInfo {
  id: string;
  name: string;
  code: string;
  country_id: string;
  active: boolean;
}

export interface ClientRoute {
  id: string;
  client_id: string;
  operator_id: string;
  provider_id: string;
  priority: number;
  weight: number;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface RoutingStrategy {
  id: string;
  client_id: string;
  operator_id: string;
  strategy: string;
  created_at: string;
  updated_at: string;
}

export const routingApi = {
  getMode: () => apiFetch<{ routing_mode: string }>('/routing/mode'),
  setMode: (routing_mode: string) =>
    apiFetch<{ routing_mode: string }>('/routing/mode', {
      method: 'PUT',
      body: JSON.stringify({ routing_mode }),
    }),
  listOperators: () =>
    apiFetch<{ operators: OperatorInfo[]; total: number }>('/routing/operators'),
  listRoutes: (operatorId?: string) => {
    const qs = operatorId ? `?operator_id=${operatorId}` : '';
    return apiFetch<{ routes: ClientRoute[] }>(`/routing/routes${qs}`);
  },
  createRoute: (data: { operator_id: string; provider_id: string; priority: number; weight: number }) =>
    apiFetch<ClientRoute>('/routing/routes', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  updateRoute: (id: string, data: { priority?: number; weight?: number; active?: boolean }) =>
    apiFetch<ClientRoute>(`/routing/routes/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  deleteRoute: (id: string) => apiFetch<void>(`/routing/routes/${id}`, { method: 'DELETE' }),
  getStrategy: (operatorId?: string) => {
    const qs = operatorId ? `?operator_id=${operatorId}` : '';
    return apiFetch<RoutingStrategy>(`/routing/strategy${qs}`);
  },
  setStrategy: (data: { operator_id?: string; strategy: string }) =>
    apiFetch<RoutingStrategy>('/routing/strategy', {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
};

// Route management API (admin default routes)
import type { RouteListItem, RouteDetail, RouteFormData, RouteReferences } from '../pages/routing/types';

export const routesApi = {
  list: (params: Record<string, string> = {}) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<{ routes: RouteListItem[]; total: number }>(`/routes?${qs}`);
  },
  get: (id: string) => apiFetch<RouteDetail>(`/routes/${id}`),
  create: (data: RouteFormData) =>
    apiFetch<RouteDetail>('/routes', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: RouteFormData) =>
    apiFetch<RouteDetail>(`/routes/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  remove: (id: string) => apiFetch<void>(`/routes/${id}`, { method: 'DELETE' }),
  references: () => apiFetch<RouteReferences>('/routes/references'),
  listProviders: () => apiFetch<{ providers: { id: string; name: string }[] }>('/routes/providers'),
};

// Templates API
export interface TemplateInfo {
  id: string;
  client_id: string;
  name: string;
  body: string;
  variables: string[];
  status: string;
  rejection_reason?: string;
  sender_name_id?: string;
  sender_name?: string;
  traffic_type?: string;
  created_at: string;
  updated_at: string;
}

export const templatesApi = {
  list: (params: Record<string, string> = {}) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<{ templates: TemplateInfo[]; total: number; page: number; per_page: number; total_pages: number }>(`/templates?${qs}`);
  },
  get: (id: string) => apiFetch<TemplateInfo>(`/templates/${id}`),
  create: (data: { name: string; body: string; sender_name_id?: string; traffic_type?: string }) =>
    apiFetch<TemplateInfo>('/templates', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: { name?: string; body?: string; sender_name_id?: string; traffic_type?: string }) =>
    apiFetch<TemplateInfo>(`/templates/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  remove: (id: string) => apiFetch<void>(`/templates/${id}`, { method: 'DELETE' }),
  render: (id: string, variables: Record<string, string>) =>
    apiFetch<{ rendered_text: string; template_name: string }>(`/templates/${id}/render`, {
      method: 'POST',
      body: JSON.stringify({ variables }),
    }),
  auditLog: (id: string, params: Record<string, string> = {}) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<{ entries: unknown[]; total: number }>(`/templates/${id}/audit?${qs}`);
  },
  submit: (id: string) =>
    apiFetch<TemplateInfo>(`/templates/${id}/submit`, { method: 'POST' }),
};

// Sender Names API
export interface SenderNameInfo {
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

export interface SenderNameHistoryEntry {
  id: string;
  sender_name_id: string;
  old_status?: string;
  new_status: string;
  actor_id?: string;
  actor_type: string;
  comment?: string;
  created_at: string;
}

export const senderNamesApi = {
  list: (params: Record<string, string> = {}) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<{ sender_names: SenderNameInfo[]; total: number; page: number; per_page: number; total_pages: number }>(
      `/sender-names?${qs}`,
    );
  },
  get: (id: string) => apiFetch<SenderNameInfo>(`/sender-names/${id}`),
  create: (name: string, companyId?: string) =>
    apiFetch<SenderNameInfo>('/sender-names', { method: 'POST', body: JSON.stringify({ name, ...(companyId ? { company_id: companyId } : {}) }) }),
  update: (id: string, name: string) =>
    apiFetch<SenderNameInfo>(`/sender-names/${id}`, { method: 'PUT', body: JSON.stringify({ name }) }),
  resubmit: (id: string) =>
    apiFetch<SenderNameInfo>(`/sender-names/${id}/resubmit`, { method: 'POST' }),
  getHistory: (id: string, params: Record<string, string> = {}) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<{ entries: SenderNameHistoryEntry[]; total: number }>(`/sender-names/${id}/history?${qs}`);
  },
  listApproved: () =>
    apiFetch<{ sender_names: SenderNameInfo[]; total: number; page: number; per_page: number; total_pages: number }>(
      '/sender-names?status=approved&per_page=100',
    ),
};

// Sender Tariff API
export const senderTariffApi = {
  getOperatorTariff: (operatorId: string) =>
    apiFetch<{ operator_id: string; monthly_tariff_amount: string; currency: string; current_month_amount: string }>(
      `/operators/${operatorId}/sender-tariff`,
    ),
  createRegistration: (data: { operator_id: string; sender_name: string; type: 'paid' | 'free' }) =>
    apiFetch<{ id: string; client_id: string; operator_id: string; sender_name: string; type: string; status: string }>(
      '/sender-registrations',
      { method: 'POST', body: JSON.stringify(data) },
    ),
  getBillingHistory: (registrationId: string) =>
    apiFetch<{ records: Array<{ id: string; billing_month: string; amount: string; created_at: string }>; total: number }>(
      `/sender-registrations/${registrationId}/billing`,
    ),
};

// Operators API
export interface SenderNameOperatorInfo {
  id: string;
  name: string;
  slug: string;
  registration_types: string[];
  monthly_tariff_amount: string | null;
}

export interface OperatorRegistration {
  id: string;
  operator_id: string;
  operator_name: string;
  type: string;
  registration_type: string;
  status: 'submitted' | 'approved' | 'rejected' | 'revision_requested';
  approved_type: string | null;
  approved_at: string | null;
  moderator_note: string | null;
  submitted_at: string;
}

export interface OperatorTemplate {
  id: string;
  name: string;
  operator_id: string;
  operator_name: string;
  body: string;
  moderation_status: 'draft' | 'submitted' | 'approved' | 'rejected' | 'revision_requested';
  moderator_note: string | null;
  submitted_at: string | null;
  resolved_at: string | null;
  created_at: string;
  updated_at: string;
}

export const operatorsApi = {
  list: () =>
    apiFetch<{ operators: SenderNameOperatorInfo[] }>('/operators'),
};

export const senderNameRegistrationsApi = {
  list: (senderNameId: string) =>
    apiFetch<{ registrations: OperatorRegistration[] }>(
      `/sender-names/${senderNameId}/operator-registrations`,
    ),
  bulkCreate: (senderNameId: string, registrations: { operator_id: string; type: string }[]) =>
    apiFetch<{ results: Array<{ operator_id: string; id?: string; status: string; error?: string }> }>(
      `/sender-names/${senderNameId}/operator-registrations`,
      { method: 'POST', body: JSON.stringify({ registrations }) },
    ),
  resubmit: (senderNameId: string, registrationId: string) =>
    apiFetch<{ status: string }>(
      `/sender-names/${senderNameId}/operator-registrations/${registrationId}/resubmit`,
      { method: 'POST', body: JSON.stringify({}) },
    ),
};

export const operatorTemplatesApi = {
  list: (senderNameId: string, operatorId?: string) => {
    const qs = operatorId ? `?operator_id=${operatorId}` : '';
    return apiFetch<{ templates: OperatorTemplate[] }>(
      `/sender-names/${senderNameId}/operator-templates${qs}`,
    );
  },
  create: (senderNameId: string, data: { operator_id: string; name: string; body: string }) =>
    apiFetch<{ id: string; moderation_status: string }>(
      `/sender-names/${senderNameId}/operator-templates`,
      { method: 'POST', body: JSON.stringify(data) },
    ),
  update: (senderNameId: string, tid: string, data: { name: string; body: string }) =>
    apiFetch<{ id: string }>(
      `/sender-names/${senderNameId}/operator-templates/${tid}`,
      { method: 'PUT', body: JSON.stringify(data) },
    ),
  delete: (senderNameId: string, tid: string) =>
    apiFetch<void>(
      `/sender-names/${senderNameId}/operator-templates/${tid}`,
      { method: 'DELETE' },
    ),
  submit: (senderNameId: string, tid: string) =>
    apiFetch<{ moderation_status: string }>(
      `/sender-names/${senderNameId}/operator-templates/${tid}/submit`,
      { method: 'POST', body: JSON.stringify({}) },
    ),
  resubmit: (senderNameId: string, tid: string) =>
    apiFetch<{ moderation_status: string }>(
      `/sender-names/${senderNameId}/operator-templates/${tid}/resubmit`,
      { method: 'POST', body: JSON.stringify({}) },
    ),
};

// Billing API
export const billingApi = {
  getBalance: () =>
    apiFetch<{ client_id: string; balance: string; currency: string; updated_at?: string; low_balance_threshold?: string }>('/billing/balance'),
  getTransactions: (params: Record<string, string> = {}) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<{ transactions: unknown[]; total: number; page: number; per_page: number; total_pages: number }>(
      `/billing/transactions?${qs}`,
    );
  },
  topUp: (amount: string, currency?: string, returnUrl?: string) =>
    apiFetch<{ payment_id: string; payment_url: string; expires_at: string }>('/billing/top-up', {
      method: 'POST',
      body: JSON.stringify({ amount, currency: currency || 'RUB', return_url: returnUrl || window.location.href }),
    }),
  setLowBalanceThreshold: (threshold: string) =>
    apiFetch<{ success: boolean; threshold: string }>('/billing/low-balance-threshold', {
      method: 'PUT',
      body: JSON.stringify({ threshold }),
    }),
};

// Tariffs API
export interface TariffPlanInfo {
  id: string;
  name: string;
  display_name: string;
  monthly_price_rub: number;
  max_sms_per_month: number;
  max_smpp_connections: number;
  max_users: number;
  features: Record<string, boolean> | string[];
}

export interface CurrentPlan {
  plan: TariffPlanInfo;
  monthly_sms_count: number;
}

export const tariffsApi = {
  getCurrent: () => apiFetch<CurrentPlan>('/tariffs/current'),
  listPlans: () => apiFetch<{ plans: TariffPlanInfo[] }>('/tariffs/plans'),
  switchPlan: (planId: string) =>
    apiFetch<{ message: string }>('/tariffs/change', {
      method: 'POST',
      body: JSON.stringify({ plan_id: planId }),
    }),
  getUsage: () => apiFetch<{ counters: unknown[]; total: number }>('/tariffs/usage'),
};

// Client effective-price matrix (spec §5.1.8, Task 22). Returns the
// read-only effective tariff for the caller's own sub-account.
export interface ClientTariffsEffectiveCell {
  operator_id: string;
  tier_id: string;
  effective: number | null;
}

export interface ClientTariffsEffectiveResponse {
  plan: { id: string; strategy: string; currency: string } | null;
  period: { id: string; from: string; to: string | null } | null;
  operators: { id: string; name: string; icon: string | null }[];
  tiers: { id: string; from_quantity: number }[];
  cells: ClientTariffsEffectiveCell[];
}

export const clientTariffsApi = {
  getEffective: (params?: {
    channel?: string;
    country?: string;
    sender_category?: string;
    traffic_type?: string;
  }) => {
    const qs = new URLSearchParams();
    qs.set('channel', params?.channel ?? 'sms');
    qs.set('country', params?.country ?? 'RU');
    qs.set('sender_category', params?.sender_category ?? 'paid_registered');
    qs.set('traffic_type', params?.traffic_type ?? 'any');
    return apiFetch<ClientTariffsEffectiveResponse>(
      `/client/tariffs/effective?${qs.toString()}`,
    );
  },
};

// Lookup API
export const lookupApi = {
  single: (phone: string) =>
    apiFetch<unknown>('/lookup', { method: 'POST', body: JSON.stringify({ phone }) }),
  bulk: (file: File) => {
    const formData = new FormData();
    formData.append('file', file);
    const csrfToken = getCookie('csrf_token');
    return fetch(`${API_BASE}/lookup/bulk`, {
      method: 'POST',
      credentials: 'include',
      headers: csrfToken ? { 'X-CSRF-Token': csrfToken } : {},
      body: formData,
    }).then(async (res) => {
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: { message: res.statusText } }));
        throw new ApiError(res.status, err.error?.message || res.statusText, err.error);
      }
      return res.json();
    });
  },
  history: (params: Record<string, string> = {}) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<unknown>(`/lookup/history?${qs}`);
  },
  stats: (params: Record<string, string> = {}) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<unknown>(`/lookup/stats?${qs}`);
  },
};

// Dashboard charts types (US1)
export interface TimelineEntry {
  date: string;
  sent: number;
  delivered: number;
  failed: number;
}

export interface StatusDistributionItem {
  status: string;
  count: number;
  label: string;
}

export interface DashboardChartData {
  timeline_7d: TimelineEntry[];
  status_distribution: StatusDistributionItem[];
  delivery_rate_trend: number;
}

export interface ProfileCompletionStep {
  key: string;
  label: string;
  completed: boolean;
}

export interface ProfileCompletion {
  percentage: number;
  steps: ProfileCompletionStep[];
}

export interface DashboardData {
  balance: string;
  currency: string;
  messages_today: number;
  messages_delivered_today: number;
  delivery_rate_today: number;
  active_api_keys: number;
  active_webhooks: number;
  charts?: DashboardChartData;
  profile_completion?: ProfileCompletion;
}

// Notifications API (US7)
export interface NotificationItem {
  id: string;
  type: string;
  body: string;
  object_type?: string;
  object_id?: string;
  is_read: boolean;
  created_at: string;
}

export interface NotificationsResponse {
  items: NotificationItem[];
  unread_count: number;
}

export const notificationsApi = {
  list: (params: Record<string, string> = {}) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<NotificationsResponse>(`/notifications?${qs}`);
  },
  markRead: (id: string) =>
    apiFetch<{ ok: boolean }>(`/notifications/${id}/read`, { method: 'POST' }),
  markAllRead: () =>
    apiFetch<{ ok: boolean }>('/notifications/read-all', { method: 'POST' }),
};

// Search / Command Palette API (US5)
export interface CommandItem {
  id: string;
  type: string;
  category: string;
  title: string;
  subtitle?: string;
  url: string;
}

export const searchApi = {
  search: (q: string) =>
    apiFetch<{ items: CommandItem[] }>(`/search?q=${encodeURIComponent(q)}`),
};

// CSV Export API (US4)
export interface ExportJob {
  job_id: string;
  status: 'pending' | 'processing' | 'ready' | 'error';
  total_rows?: string;
}

export const exportApi = {
  start: (filters: Record<string, string> = {}) =>
    apiFetch<{ job_id: string }>('/export/start', {
      method: 'POST',
      body: JSON.stringify(filters),
    }),
  getStatus: (jobId: string) =>
    apiFetch<ExportJob>(`/export/${jobId}/status`),
  download: (jobId: string) => {
    const csrfToken = getCookie('csrf_token');
    return fetch(`${API_BASE}/export/${jobId}/download`, {
      credentials: 'include',
      headers: csrfToken ? { 'X-CSRF-Token': csrfToken } : {},
    });
  },
};

// Analytics extended types (US8)
export interface AnalyticsParams {
  period?: string;
  date_from?: string;
  date_to?: string;
  group_by?: string;
  compare?: boolean;
  include_cost?: boolean;
}

export interface CostByDay {
  date: string;
  amount: string;
}

export interface AnalyticsDataExtended {
  summary: {
    total_sent: number;
    total_delivered: number;
    total_failed: number;
    total_expired: number;
    delivery_rate: number;
    total_cost: string;
    currency: string;
  };
  timeline: Array<{ period: string; sent: number; delivered: number; failed: number; delivery_rate: number }>;
  previous_timeline?: Array<{ period: string; sent: number; delivered: number }>;
  by_country: Array<{ country: string; sent: number; delivered: number; failed: number; delivery_rate: number }>;
  cost_by_day?: CostByDay[];
  cost_forecast?: string;
}

// --- Detalization API ---
export interface DetalizationMessage {
  id: string;
  source: string;
  destination: string;
  text_preview: string;
  status: string;
  segment_count: number;
  created_at: string;
  submitted_at?: string;
  delivered_at?: string;
  failed_at?: string;
  provider_name: string;
  operator_name?: string;
  country_name?: string;
  channel?: string;
  send_method?: string;
  login?: string;
  total_amount?: string;
}

export interface DetalizationMessageDetail {
  id: string;
  source: string;
  destination: string;
  text: string;
  encoding?: string;
  status: string;
  status_message?: string;
  external_id?: string;
  segment_count: number;
  provider_name?: string;
  route_name?: string;
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

export const detalizationApi = {
  list: (params?: {
    status?: string;
    source?: string;
    sender_name?: string;
    destination?: string;
    date_from?: string;
    date_to?: string;
    login?: string;
    operator?: string;
    channel?: string;
    country?: string;
    send_method?: string;
    message_id?: string;
    sort_by?: string;
    sort_order?: string;
    limit?: number;
    offset?: number;
  }) => {
    const filtered: Record<string, string> = {};
    if (params) {
      for (const [k, v] of Object.entries(params)) {
        if (v !== undefined && v !== null && v !== '') filtered[k] = String(v);
      }
    }
    const qs = new URLSearchParams(filtered).toString();
    return apiFetch<{ messages: DetalizationMessage[]; total: number; limit: number; offset: number }>(
      `/detalization${qs ? `?${qs}` : ''}`,
    );
  },
  get: (id: string) => apiFetch<DetalizationMessageDetail>(`/detalization/${id}`),
};

// References API for filter dropdowns
export interface OperatorRef {
  id: string;
  name: string;
  code: string;
}

export interface CountryRef {
  id: string;
  name: string;
  iso_code: string;
}

export const referencesApi = {
  operators: () => apiFetch<{ operators: OperatorRef[] }>('/references/operators'),
  countries: () => apiFetch<{ countries: CountryRef[] }>('/references/countries'),
};

// --- Notification Settings API ---
export interface NotifSetting {
  event_type: string;
  in_app: boolean;
  email: boolean;
}

export const notificationSettingsApi = {
  get: () =>
    apiFetch<{ settings: NotifSetting[]; extra_emails: string[] }>('/settings/notifications'),
  update: (data: { settings: NotifSetting[]; extra_emails: string[] }) =>
    apiFetch<{ ok: boolean }>('/settings/notifications', {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
};

// --- Campaign Schedules API ---
export interface CampaignSchedule {
  id: string;
  name: string;
  template_campaign_id: string;
  template_campaign_name: string;
  frequency: string;
  cron_expression?: string;
  next_run_at?: string;
  last_run_at?: string;
  is_active: boolean;
  run_count: number;
  max_runs?: number;
  created_at: string;
}

export const campaignSchedulesApi = {
  list: () => apiFetch<{ schedules: CampaignSchedule[] }>('/campaign-schedules'),
  create: (data: {
    name: string;
    template_campaign_id: string;
    frequency: string;
    cron_expression?: string;
    max_runs?: number;
  }) =>
    apiFetch<{ id: string }>('/campaign-schedules', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  update: (id: string, data: {
    name: string;
    frequency: string;
    cron_expression?: string;
    max_runs?: number;
    clear_max_runs?: boolean;
  }) =>
    apiFetch<{ ok: boolean }>(`/campaign-schedules/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),
  toggle: (id: string, is_active: boolean) =>
    apiFetch<{ ok: boolean }>(`/campaign-schedules/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ is_active }),
    }),
  remove: (id: string) =>
    apiFetch<void>(`/campaign-schedules/${id}`, { method: 'DELETE' }),
};

// --- Default Sender Names API ---
// Returns map of channel -> sender_name_id
export const defaultSendersApi = {
  get: () => apiFetch<Record<string, string>>('/settings/default-senders'),
  set: (data: Record<string, string>) =>
    apiFetch<{ ok: boolean }>('/settings/default-senders', {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
};

// ===== Companies API =====

export interface CompanyInfo {
  id: string;
  inn?: string;
  name: string;
  full_name?: string;
  kpp?: string;
  ogrn?: string;
  legal_address?: string;
  actual_address?: string;
  ceo_name?: string;
  ceo_title?: string;
  acting_basis?: string;
  bank_name?: string;
  bank_bik?: string;
  bank_corr_account?: string;
  bank_account?: string;
  email?: string;
  phone?: string;
  is_offer: boolean;
  is_default: boolean;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface CompanyUpsertRequest {
  inn?: string;
  name: string;
  full_name?: string;
  kpp?: string;
  ogrn?: string;
  legal_address?: string;
  actual_address?: string;
  ceo_name?: string;
  ceo_title?: string;
  acting_basis?: string;
  bank_name?: string;
  bank_bik?: string;
  bank_corr_account?: string;
  bank_account?: string;
  email?: string;
  phone?: string;
}

export const companiesApi = {
  list: () =>
    apiFetch<{ companies: CompanyInfo[] }>('/companies'),

  create: (data: CompanyUpsertRequest) =>
    apiFetch<CompanyInfo>('/companies', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  get: (id: string) =>
    apiFetch<CompanyInfo>(`/companies/${id}`),

  update: (id: string, data: CompanyUpsertRequest) =>
    apiFetch<CompanyInfo>(`/companies/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  setDefault: (id: string) =>
    apiFetch<void>(`/companies/${id}/set-default`, { method: 'POST' }),

  detach: (id: string) =>
    apiFetch<void>(`/companies/${id}/detach`, { method: 'DELETE' }),
};

// ── Command Center Types ─────────────────────────────────────────────

export interface DashboardMetrics {
  balance: string;
  currency: string;
  msg_per_sec: number;
  msg_per_sec_trend_pct: number;
  delivery_rate_24h: number;
  delivery_rate_trend_pct: number;
  burn_rate_per_hour: string;
  forecast_hours: number;
  sparkline_1h: number[];              // last 24 data points (1 per hour; backend groups by hour)
  messages_today: number;
  active_campaigns: ActiveCampaign[];
}

export interface ActiveCampaign {
  id: string;
  name: string;
  started_at: string;
  total: number;
  sent: number;
  delivery_rate: number;
  eta_minutes: number;
}

export interface ProviderHealth {
  id: string;
  name: string;
  connection_type: 'SMPP' | 'HTTP';
  connections_active: number;
  connections_total: number;
  success_rate: number;
  msg_per_sec: number;
  is_degraded: boolean;
}

export interface AlertItem {
  id: string;
  type: 'critical' | 'warning' | 'success' | 'info';
  title: string;
  description: string;
  created_at: string;
}

export interface LiveMessageEvent {
  message_id: string;
  timestamp: string;
  status: 'delivered' | 'sent' | 'failed' | 'pending' | 'expired';
  phone_masked: string;
  operator: string;
  provider: string;
  sender: string;
  text_fragment: string;
}

// ── Command Center API functions ──────────────────────────────────────

export const commandCenterApi = {
  getMetrics: () =>
    apiFetch<DashboardMetrics>('/dashboard/metrics'),

  getProviderHealth: () =>
    apiFetch<{ providers: ProviderHealth[] }>('/providers/health'),

  getAlerts: () =>
    apiFetch<{ items: AlertItem[] }>('/alerts'),

  /** Returns the WebSocket URL (ws:// or wss://) for the live message stream. */
  getLiveFeedUrl: (): string => {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    return `${proto}://${location.host}/portal/v1/ws/messages`;
  },
};

// Quota API
export interface QuotaData {
  id: string;
  client_id: string;
  segment_limit: number;
  segments_used: number;
  overage_segments: number;
  overage_rate: string;
  currency: string;
  period_start: string;
  period_end: string;
  is_active: boolean;
}

export interface QuotaHistoryEntry {
  id: string;
  segment_limit: number;
  segments_used: number;
  overage_segments: number;
  overage_rate: string;
  currency: string;
  period_start: string;
  period_end: string;
}

export const quotaApi = {
  getMyQuota: () => apiFetch<{ quota: QuotaData | null }>('/quota'),
  getQuotaHistory: () => apiFetch<{ history: QuotaHistoryEntry[] }>('/quota/history'),
};

// Reseller moderation API
export interface ResellerSenderName {
  id: string;
  client_id: string;
  sub_account_email: string;
  name: string;
  status: string;
  rejection_reason: string | null;
  created_at: string;
}

export interface ResellerTemplate {
  id: string;
  client_id: string;
  sub_account_email: string;
  name: string;
  body_preview: string;
  status: string;
  rejection_reason: string | null;
  created_at: string;
}

export interface ModerationCounts {
  sender_names: number;
  templates: number;
  registrations: number;
}

export const resellerApi = {
  getModerationCounts: () =>
    apiFetch<ModerationCounts>('/reseller/moderation/counts'),

  // Sender names
  listSenderNames: (params?: { status?: string }) => {
    const qs = params?.status ? `?status=${params.status}` : '';
    return apiFetch<{ sender_names: ResellerSenderName[]; total: number }>(`/reseller/sender-names${qs}`);
  },
  approveSenderName: (id: string) =>
    apiFetch<unknown>(`/reseller/sender-names/${id}/approve`, { method: 'POST' }),
  rejectSenderName: (id: string, reason: string) =>
    apiFetch<unknown>(`/reseller/sender-names/${id}/reject`, {
      method: 'POST',
      body: JSON.stringify({ reason }),
    }),

  // Templates
  listTemplates: (params?: { status?: string }) => {
    const qs = params?.status ? `?status=${params.status}` : '';
    return apiFetch<{ templates: ResellerTemplate[]; total: number }>(`/reseller/templates${qs}`);
  },
  approveTemplate: (id: string) =>
    apiFetch<unknown>(`/reseller/templates/${id}/approve`, { method: 'POST' }),
  rejectTemplate: (id: string, reason: string) =>
    apiFetch<unknown>(`/reseller/templates/${id}/reject`, {
      method: 'POST',
      body: JSON.stringify({ reason }),
    }),
  requestRevisionTemplate: (id: string, comment: string) =>
    apiFetch<unknown>(`/reseller/templates/${id}/request-revision`, {
      method: 'POST',
      body: JSON.stringify({ comment }),
    }),

  // Operator registrations (existing endpoints, typed access)
  listOperatorRegistrations: (params?: { status?: string; sub_account_id?: string }) => {
    const qs = new URLSearchParams();
    if (params?.status) qs.set('status', params.status);
    if (params?.sub_account_id) qs.set('sub_account_id', params.sub_account_id);
    const q = qs.toString();
    return apiFetch<{ registrations: unknown[] }>(`/reseller/operator-registrations${q ? `?${q}` : ''}`);
  },
  approveOperatorRegistration: (id: string) =>
    apiFetch<unknown>(`/reseller/operator-registrations/${id}/approve`, { method: 'POST' }),
  rejectOperatorRegistration: (id: string, note?: string) =>
    apiFetch<unknown>(`/reseller/operator-registrations/${id}/reject`, {
      method: 'POST',
      body: JSON.stringify({ note }),
    }),
  requestRevisionOperatorRegistration: (id: string, note?: string) =>
    apiFetch<unknown>(`/reseller/operator-registrations/${id}/request-revision`, {
      method: 'POST',
      body: JSON.stringify({ note }),
    }),

  // --- Routing ---
  listNetworkProviders: (params?: { sub_account_id?: string }) => {
    const qs = params?.sub_account_id ? `?sub_account_id=${params.sub_account_id}` : '';
    return apiFetch<{ providers: unknown[]; total: number }>(`/reseller/routing/providers${qs}`);
  },
  listNetworkRoutes: (params?: { sub_account_id?: string }) => {
    const qs = params?.sub_account_id ? `?sub_account_id=${params.sub_account_id}` : '';
    return apiFetch<{ routes: unknown[]; total: number }>(`/reseller/routing/routes${qs}`);
  },
  bulkAssignProvider: (data: { sub_account_ids: string[]; provider_id: string; priority?: number }) =>
    apiFetch<{ results: unknown[] }>('/reseller/routing/bulk-assign', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  // --- Tariffs ---
  listTariffs: (params?: { sub_account_id?: string }) => {
    const qs = params?.sub_account_id ? `?sub_account_id=${params.sub_account_id}` : '';
    return apiFetch<{ tariffs: unknown[]; total: number }>(`/reseller/tariffs${qs}`);
  },
  upsertTariffs: (data: { sub_account_id?: string; tariffs: { operator_id: string; sender_category?: string; price_per_sms: string }[] }) =>
    apiFetch<{ updated: number }>('/reseller/tariffs', {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  copyTariffs: (from_sub_account_id: string, to_sub_account_id: string) =>
    apiFetch<{ copied: number }>('/reseller/tariffs/copy', {
      method: 'POST',
      body: JSON.stringify({ from_sub_account_id, to_sub_account_id }),
    }),

  // --- Analytics ---
  getNetworkAnalytics: (params?: { period?: string; sub_account_id?: string; group_by?: string }) => {
    const qs = new URLSearchParams();
    if (params?.period) qs.set('period', params.period);
    if (params?.sub_account_id) qs.set('sub_account_id', params.sub_account_id);
    if (params?.group_by) qs.set('group_by', params.group_by);
    const q = qs.toString();
    return apiFetch<unknown>(`/reseller/analytics${q ? `?${q}` : ''}`);
  },

  // --- Dashboard ---
  getDashboard: (params?: { period?: string }) => {
    const qs = params?.period ? `?period=${params.period}` : '';
    return apiFetch<unknown>(`/reseller/dashboard${qs}`);
  },
};

// --- Reseller Tariff Plans API (new system) ---

export interface ResellerTemplate {
  id: string;
  name: string;
  description: string | null;
  assigned_count: number;
  created_at: string;
  updated_at: string;
}

export interface ResellerTariffPlan {
  id: string;
  template_id: string | null;
  sub_account_id: string | null;
  country_id: string | null;
  operator_id: string | null;
  sender_category: string;
  traffic_type: string;
  strategy: string;
  active: boolean;
  operator_name: string;
  country_name: string;
  created_at: string;
  updated_at: string;
}

export interface ResellerTariffPeriod {
  id: string;
  start_date: string;
  end_date: string | null;
  created_at: string;
}

export interface ResellerTariffTier {
  id: string;
  from_count: number;
  price_per_segment: string;
}

export interface TariffOverviewItem {
  operator_id: string;
  operator_name: string;
  sender_category: string;
  price: string;
  source: 'override' | 'template' | 'legacy';
  strategy: string;
  plan_id: string | null;
}

export const resellerTariffApi = {
  // Templates
  listTemplates: () =>
    apiFetch<{ templates: ResellerTemplate[]; total: number }>('/reseller/tariff-templates'),
  createTemplate: (data: { name: string; description?: string }) =>
    apiFetch<{ id: string }>('/reseller/tariff-templates', { method: 'POST', body: JSON.stringify(data) }),
  updateTemplate: (id: string, data: { name?: string; description?: string }) =>
    apiFetch<{ status: string }>(`/reseller/tariff-templates/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteTemplate: (id: string) =>
    apiFetch<{ status: string }>(`/reseller/tariff-templates/${id}`, { method: 'DELETE' }),
  assignTemplate: (templateId: string, subAccountIds: string[]) =>
    apiFetch<{ assigned: number }>(`/reseller/tariff-templates/${templateId}/assign`, {
      method: 'POST', body: JSON.stringify({ sub_account_ids: subAccountIds }),
    }),
  unassignTemplate: (templateId: string, subAccountId: string) =>
    apiFetch<{ status: string }>(`/reseller/tariff-templates/${templateId}/assign/${subAccountId}`, { method: 'DELETE' }),

  // Plans
  listPlans: (params?: { template_id?: string; sub_account_id?: string }) => {
    const qs = new URLSearchParams();
    if (params?.template_id) qs.set('template_id', params.template_id);
    if (params?.sub_account_id) qs.set('sub_account_id', params.sub_account_id);
    const q = qs.toString();
    return apiFetch<{ plans: ResellerTariffPlan[]; total: number }>(`/reseller/tariff-plans${q ? '?' + q : ''}`);
  },
  createPlan: (data: {
    template_id?: string; sub_account_id?: string;
    country_id?: string; operator_id?: string;
    sender_category: string; traffic_type: string; strategy: string;
  }) => apiFetch<{ id: string }>('/reseller/tariff-plans', { method: 'POST', body: JSON.stringify(data) }),
  updatePlan: (id: string, data: { strategy?: string; sender_category?: string; traffic_type?: string }) =>
    apiFetch<{ status: string }>(`/reseller/tariff-plans/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deletePlan: (id: string) =>
    apiFetch<{ status: string }>(`/reseller/tariff-plans/${id}`, { method: 'DELETE' }),

  // Periods
  listPeriods: (planId: string) =>
    apiFetch<{ periods: ResellerTariffPeriod[]; total: number }>(`/reseller/tariff-plans/${planId}/periods`),
  createPeriod: (planId: string, data: { start_date: string; end_date?: string | null }) =>
    apiFetch<{ id: string }>(`/reseller/tariff-plans/${planId}/periods`, { method: 'POST', body: JSON.stringify(data) }),
  updatePeriod: (id: string, data: { start_date: string; end_date?: string | null }) =>
    apiFetch<{ status: string }>(`/reseller/tariff-periods/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deletePeriod: (id: string) =>
    apiFetch<{ status: string }>(`/reseller/tariff-periods/${id}`, { method: 'DELETE' }),

  // Tiers
  listTiers: (periodId: string) =>
    apiFetch<{ tiers: ResellerTariffTier[]; total: number }>(`/reseller/tariff-periods/${periodId}/tiers`),
  upsertTiers: (periodId: string, tiers: { from_count: number; price_per_segment: string }[]) =>
    apiFetch<{ saved: number }>(`/reseller/tariff-periods/${periodId}/tiers`, {
      method: 'POST', body: JSON.stringify({ tiers }),
    }),

  // Overview
  overview: (subAccountId: string) =>
    apiFetch<{ tariffs: TariffOverviewItem[]; total: number; template_id: string | null; template_name: string | null }>(
      `/reseller/tariff-overview?sub_account_id=${subAccountId}`
    ),

  // Copy
  copyPlans: (data: {
    from_template_id?: string; from_sub_account_id?: string;
    to_template_id?: string; to_sub_account_id?: string;
  }) => apiFetch<{ copied_plans: number }>('/reseller/tariff-plans/copy', {
    method: 'POST', body: JSON.stringify(data),
  }),
};

// =============================================================================
// Network Tariffs (redesign, 2026-04-22)
// =============================================================================

export interface SubAccountTariffSummary {
  sub_account_id: string;
  sub_account_name: string;
  sub_account_email: string;
  template_id: string | null;
  template_name: string | null;
  override_count: number;
  avg_price_per_sms: number | null;
  currency: string;
}

export interface TariffTemplateSummary {
  id: string;
  name: string;
  description: string | null;
  plans_count: number;
  bound_subaccount_count: number;
  created_at: string;
}

export interface TariffEditorCell {
  operator_id: string;
  tier_id: string;
  price_template: number | null;
  price_override: number | null;
  effective: number | null;
  source: 'template' | 'override' | 'unset';
}

export interface TariffEditorScope {
  kind: 'template' | 'override';
  template_id?: string;
  template_name?: string;
  sub_account_id?: string;
  sub_account_name?: string;
}

export interface TariffEditorData {
  scope: TariffEditorScope;
  template: { id: string; name: string } | null;
  plan: { id: string; strategy: string; currency: string } | null;
  periods: { id: string; from: string; to: string | null; active: boolean }[];
  active_period_id: string | null;
  operators: { id: string; name: string; icon: string | null }[];
  tiers: { id: string; from_quantity: number }[];
  cells: TariffEditorCell[];
}

// NOTE: `price_per_segment` REQUIRED when `id === null` (backend contract — Task 6).
export interface TariffBulkTierUpsert {
  id: string | null;
  from_quantity: number;
  price_per_segment?: number;
}

export interface TariffBulkCellUpsert {
  operator_id: string;
  tier_id: string;
  price: number;
  scope: 'template' | 'override';
  sub_account_id?: string;
}

export interface TariffBulkCellDelete {
  operator_id: string;
  tier_id: string;
  scope: 'template' | 'override';
  sub_account_id?: string;
}

export const networkTariffsApi = {
  listSubaccounts: () =>
    apiFetch<SubAccountTariffSummary[]>('/network/tariffs/subaccounts-summary'),

  listTemplates: () =>
    apiFetch<TariffTemplateSummary[]>('/network/tariff-templates'),

  createTemplate: (body: { name: string; description?: string; copy_from_id?: string }) =>
    apiFetch<{ id: string }>('/network/tariff-templates', {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  bindTemplate: (id: string, subAccountIds: string[]) =>
    apiFetch<{ bound: string[] }>(`/network/tariff-templates/${id}/bind`, {
      method: 'POST',
      body: JSON.stringify({ sub_account_ids: subAccountIds }),
    }),

  duplicateTemplate: (id: string, name: string) =>
    apiFetch<{ id: string }>(`/network/tariff-templates/${id}/duplicate`, {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),

  getEditor: (
    id: string,
    params: {
      mode: 'template' | 'override';
      channel: string;
      country: string;
      sender_category: string;
      traffic_type: string;
      period_id?: string;
    },
  ) => {
    const qs = new URLSearchParams();
    qs.set('mode', params.mode);
    qs.set('channel', params.channel);
    qs.set('country', params.country);
    qs.set('sender_category', params.sender_category);
    qs.set('traffic_type', params.traffic_type);
    if (params.period_id) qs.set('period_id', params.period_id);
    return apiFetch<TariffEditorData>(`/network/tariff-editor/${id}?${qs.toString()}`);
  },

  bulkPatchPlan: (
    planId: string,
    body: {
      period_id: string;
      tiers_upsert: TariffBulkTierUpsert[];
      tiers_delete: string[];
      cells_upsert: TariffBulkCellUpsert[];
      cells_delete: TariffBulkCellDelete[];
    },
  ) =>
    apiFetch<{ ok: boolean; errors?: unknown[] }>(`/network/tariff-plans/${planId}/bulk`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),

  createPeriod: (
    planId: string,
    body: { from: string; to?: string; copy_from_period_id?: string; keep_tiers: boolean },
  ) =>
    apiFetch<{ id: string }>(`/network/tariff-plans/${planId}/periods`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  createPlan: (body: {
    template_id?: string;
    sub_account_id?: string;
    country: string;
    operator_id?: string;
    sender_category: string;
    traffic_type: string;
    strategy?: string;
  }) =>
    apiFetch<{ id: string; period_id: string }>(`/network/tariff-plans`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  updateStrategy: (body: {
    template_id?: string;
    sub_account_id?: string;
    country: string;
    sender_category: string;
    traffic_type: string;
    strategy: string;
  }) =>
    apiFetch<{ updated: number }>(`/network/tariff-plans/strategy`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
};

// =============================================================================
// Network Routing (Plan 1 — providers, provider-sets, assignments, overrides)
// =============================================================================

export interface NetworkProvider {
  id: string;
  name: string;
  ownership: 'platform' | 'private';
  smpp_host: string | null;
  smpp_port: number | null;
  system_id: string | null;
  system_type: string | null;
  active: boolean;
}

export interface NetworkProviderSet {
  id: string;
  name: string;
  is_default: boolean;
  item_count: number;
  assigned_count: number;
  created_at: string;
  updated_at: string;
}

export interface NetworkProviderSetItem {
  id: string;
  provider_id: string;
  provider_name: string;
  priority: number;
  expose_cost: boolean;
  expose_provider_name: boolean;
}

export interface NetworkAssignment {
  client_id: string;
  sub_account_name: string;
  provider_set_id: string | null;
  provider_set_name: string | null;
  route_set_id: string | null;
  route_set_name: string | null;
  has_overrides: boolean;
  validation_status: 'ok' | 'unassigned' | 'conflict';
  validation_error?: string;
}

export interface NetworkBulkAssignResult {
  client_id: string;
  status: 'ok' | 'error' | 'conflict';
  error?: string;
}

export interface NetworkProviderOverride {
  provider_id: string;
  name: string;
  priority: number;
  ownership: string;
}

export interface NetworkSubAccountOverview {
  provider_set: { id: string; name: string } | null;
  route_set: { id: string; name: string } | null;
  provider_overrides: NetworkProviderOverride[];
  route_overrides: NetworkRouteOverride[];
}

export interface RouteCondition {
  type: 'operator' | 'country' | 'traffic_type' | 'paid_name' | 'regex';
  value: string;
}

export interface RouteConditionGroup {
  logic_op: 'IF' | 'AND' | 'AND_NOT' | 'OR' | 'OR_NOT';
  conditions: RouteCondition[];
}

export interface RouteSchedule {
  date_from?: string | null;
  date_to?: string | null;
  time_from?: string | null;
  time_to?: string | null;
  weekdays: number;
  timezone: string;
}

export interface NetworkRouteSet {
  id: string;
  name: string;
  is_default: boolean;
  item_count: number;
  assigned_count: number;
}

export interface NetworkRouteSetItem {
  id: string;
  name: string;
  comment: string;
  provider_id: string;
  provider_name: string;
  priority: number;
  share: number;
  route_type: string;
  status: 'active' | 'inactive';
  condition_groups: RouteConditionGroup[];
  schedules: RouteSchedule[];
}

export interface RoutePreviewMatch {
  matched_item_id: string;
  item_name: string;
  provider_id: string;
  provider_name: string;
  priority: number;
}

export interface NetworkRouteOverride {
  id: string;
  name: string;
  provider_id: string;
  provider_name: string;
  priority: number;
  status: string;
}

export const networkApi = {
  // ── Providers ──────────────────────────────────────────────────────────────

  /** GET /reseller/network/providers — platform + reseller's private providers */
  listProviders: () =>
    apiFetch<{ providers: NetworkProvider[] }>('/reseller/network/providers'),

  /** POST /reseller/network/providers — create a private provider */
  createProvider: (data: {
    name: string;
    smpp_host: string;
    smpp_port: number;
    system_id: string;
    password: string;
    system_type?: string;
  }) =>
    apiFetch<NetworkProvider>('/reseller/network/providers', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  /** PUT /reseller/network/providers/{id} — update a private provider */
  updateProvider: (id: string, data: {
    name: string;
    smpp_host: string;
    smpp_port: number;
    system_id: string;
    password?: string;
    system_type?: string;
  }) =>
    apiFetch<{ id: string }>(`/reseller/network/providers/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  /** DELETE /reseller/network/providers/{id} — remove a private provider */
  deleteProvider: (id: string) =>
    apiFetch<void>(`/reseller/network/providers/${id}`, { method: 'DELETE' }),

  // ── Provider Sets ──────────────────────────────────────────────────────────

  /** GET /reseller/network/provider-sets */
  listProviderSets: () =>
    apiFetch<{ provider_sets: NetworkProviderSet[] }>('/reseller/network/provider-sets'),

  /** POST /reseller/network/provider-sets */
  createProviderSet: (data: { name: string; is_default?: boolean }) =>
    apiFetch<NetworkProviderSet>('/reseller/network/provider-sets', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  /** PUT /reseller/network/provider-sets/{id} */
  updateProviderSet: (id: string, data: { name: string; is_default?: boolean }) =>
    apiFetch<{ id: string }>(`/reseller/network/provider-sets/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  /** DELETE /reseller/network/provider-sets/{id} */
  deleteProviderSet: (id: string) =>
    apiFetch<void>(`/reseller/network/provider-sets/${id}`, { method: 'DELETE' }),

  // ── Provider Set Items ─────────────────────────────────────────────────────

  /** GET /reseller/network/provider-sets/{id}/items */
  listProviderSetItems: (setId: string) =>
    apiFetch<{ items: NetworkProviderSetItem[] }>(`/reseller/network/provider-sets/${setId}/items`),

  /** PUT /reseller/network/provider-sets/{id}/items — atomic replace */
  putProviderSetItems: (setId: string, items: Array<{
    provider_id: string;
    priority: number;
    expose_cost: boolean;
    expose_provider_name: boolean;
  }>) =>
    apiFetch<{ set_id: string; count: number }>(`/reseller/network/provider-sets/${setId}/items`, {
      method: 'PUT',
      body: JSON.stringify({ items }),
    }),

  // ── Assignments ────────────────────────────────────────────────────────────

  /** GET /reseller/network/assignments — all sub-accounts with assignment state */
  listAssignments: () =>
    apiFetch<{ assignments: NetworkAssignment[] }>('/reseller/network/assignments'),

  /** PUT /reseller/network/assignments/{client_id} — assign/unassign one sub-account */
  putAssignment: (clientId: string, data: {
    provider_set_id: string | null;
    route_set_id?: string | null;
  }) =>
    apiFetch<{ client_id: string }>(`/reseller/network/assignments/${clientId}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  /** POST /reseller/network/assignments/bulk — assign/unassign multiple sub-accounts */
  bulkAssign: (data: {
    client_ids: string[];
    provider_set_id: string | null;
    route_set_id?: string | null;
  }) =>
    apiFetch<{ results: NetworkBulkAssignResult[] }>('/reseller/network/assignments/bulk', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  // ── Sub-Account Network (overview + overrides) ─────────────────────────────

  /** GET /sub-accounts/{id}/network/overview */
  getSubAccountNetworkOverview: (subAccountId: string) =>
    apiFetch<NetworkSubAccountOverview>(`/sub-accounts/${subAccountId}/network/overview`),

  /** POST /sub-accounts/{id}/network/provider-overrides — add provider override */
  addProviderOverride: (subAccountId: string, data: {
    provider_id: string;
    priority: number;
    expose_cost: boolean;
    expose_provider_name: boolean;
  }) =>
    apiFetch<{ client_id: string; provider_id: string }>(
      `/sub-accounts/${subAccountId}/network/provider-overrides`,
      { method: 'POST', body: JSON.stringify(data) },
    ),

  /** DELETE /sub-accounts/{id}/network/provider-overrides/{provider_id} */
  deleteProviderOverride: (subAccountId: string, providerId: string) =>
    apiFetch<void>(
      `/sub-accounts/${subAccountId}/network/provider-overrides/${providerId}`,
      { method: 'DELETE' },
    ),

  // ── Route Sets ─────────────────────────────────────────────────────────────

  listRouteSets: () =>
    apiFetch<{ route_sets: NetworkRouteSet[] }>('/reseller/network/route-sets'),
  createRouteSet: (data: { name: string; is_default?: boolean }) =>
    apiFetch<{ id: string; name: string; is_default: boolean }>(
      '/reseller/network/route-sets',
      { method: 'POST', body: JSON.stringify(data) },
    ),
  updateRouteSet: (id: string, data: { name: string; is_default: boolean }) =>
    apiFetch<{ id: string }>(`/reseller/network/route-sets/${id}`, {
      method: 'PUT', body: JSON.stringify(data),
    }),
  deleteRouteSet: (id: string) =>
    apiFetch<void>(`/reseller/network/route-sets/${id}`, { method: 'DELETE' }),

  // ── Route Set Items ────────────────────────────────────────────────────────

  listRouteSetItems: (setId: string) =>
    apiFetch<{ items: NetworkRouteSetItem[] }>(
      `/reseller/network/route-sets/${setId}/items`,
    ),
  createRouteSetItem: (setId: string, data: Omit<NetworkRouteSetItem, 'id' | 'provider_name'>) =>
    apiFetch<{ id: string }>(`/reseller/network/route-sets/${setId}/items`, {
      method: 'POST', body: JSON.stringify(data),
    }),
  updateRouteSetItem: (setId: string, itemId: string, data: Omit<NetworkRouteSetItem, 'id' | 'provider_name'>) =>
    apiFetch<{ id: string }>(`/reseller/network/route-sets/${setId}/items/${itemId}`, {
      method: 'PUT', body: JSON.stringify(data),
    }),
  deleteRouteSetItem: (setId: string, itemId: string) =>
    apiFetch<void>(`/reseller/network/route-sets/${setId}/items/${itemId}`, {
      method: 'DELETE',
    }),
  duplicateRouteSetItem: (setId: string, itemId: string) =>
    apiFetch<{ id: string }>(`/reseller/network/route-sets/${setId}/items/${itemId}/duplicate`, {
      method: 'POST',
    }),
  reorderRouteSetItems: (setId: string, items: Array<{ item_id: string; priority: number }>) =>
    apiFetch<{ set_id: string }>(`/reseller/network/route-sets/${setId}/items/reorder`, {
      method: 'PUT', body: JSON.stringify({ items }),
    }),

  // ── Preview ────────────────────────────────────────────────────────────────

  previewRouteSet: (setId: string, data: { phone: string; sender_id: string; traffic_type: string }) =>
    apiFetch<{ matches: RoutePreviewMatch[] }>(
      `/reseller/network/route-sets/${setId}/preview`,
      { method: 'POST', body: JSON.stringify(data) },
    ),

  // ── Bulk Dry Run ───────────────────────────────────────────────────────────

  bulkAssignDryRun: (data: {
    client_ids: string[];
    provider_set_id: string | null;
    route_set_id?: string | null;
  }) =>
    apiFetch<{ results: NetworkBulkAssignResult[] }>(
      '/reseller/network/assignments/bulk/dry-run',
      { method: 'POST', body: JSON.stringify(data) },
    ),

  // ── Cleanup ────────────────────────────────────────────────────────────────

  routeCleanup: (data: { provider_id: string }) =>
    apiFetch<{ removed_route_set_items: number; removed_overrides: number }>(
      '/reseller/network/route-cleanup',
      { method: 'POST', body: JSON.stringify(data) },
    ),

  // ── Sub-account route overrides ────────────────────────────────────────────

  addRouteOverride: (subAccountId: string, data: Omit<NetworkRouteSetItem, 'id' | 'provider_name'>) =>
    apiFetch<{ id: string }>(
      `/sub-accounts/${subAccountId}/network/route-overrides`,
      { method: 'POST', body: JSON.stringify(data) },
    ),
  updateRouteOverride: (subAccountId: string, routeId: string, data: Omit<NetworkRouteSetItem, 'id' | 'provider_name'>) =>
    apiFetch<{ id: string }>(
      `/sub-accounts/${subAccountId}/network/route-overrides/${routeId}`,
      { method: 'PUT', body: JSON.stringify(data) },
    ),
  deleteRouteOverride: (subAccountId: string, routeId: string) =>
    apiFetch<void>(
      `/sub-accounts/${subAccountId}/network/route-overrides/${routeId}`,
      { method: 'DELETE' },
    ),
};
