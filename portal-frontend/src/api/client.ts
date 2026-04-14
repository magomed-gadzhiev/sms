import { getCookie } from '../utils/cookies';

const API_BASE = '/portal/v1';

export async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const csrfToken = getCookie('csrf_token');
  const isFormData = options?.body instanceof FormData;
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: 'include',
    ...options,
    headers: {
      ...(isFormData ? {} : { 'Content-Type': 'application/json' }),
      ...(csrfToken ? { 'X-CSRF-Token': csrfToken } : {}),
      ...options?.headers,
    },
  });
  if (!res.ok) {
    if (res.status === 401 && !path.startsWith('/auth/')) {
      if (window.location.pathname !== '/login') {
        window.location.href = '/login';
      }
      throw new ApiError(401, 'Unauthorized');
    }
    const err = await res.json().catch(() => ({ error: { message: res.statusText } }));
    let msg = err.error?.message || res.statusText;
    if (typeof msg === 'string') {
      msg = msg.replace(' не найден', '').replace('parent client not found', 'Parent client not found');
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

export const apiKeysApi = {
  list: () => apiFetch<{ keys: APIKeyInfo[] }>('/api-keys'),
  create: (data: CreateAPIKeyRequest) =>
    apiFetch<CreateAPIKeyResponse>('/api-keys', { method: 'POST', body: JSON.stringify(data) }),
  revoke: (id: string) => apiFetch<void>(`/api-keys/${id}`, { method: 'DELETE' }),
  get: (id: string) => apiFetch<APIKeyInfo>(`/api-keys/${id}`),
  update: (id: string, data: { name: string; scopes: string[]; allowed_ips: string[]; expires_at?: string }) =>
    apiFetch<APIKeyInfo>(`/api-keys/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
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
  operator_id: string;
  operator_name: string;
  type: string;
  status: string;
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
