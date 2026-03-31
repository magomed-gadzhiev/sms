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

function getCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp(`(^| )${name}=([^;]+)`));
  return match ? match[2] : null;
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
  send: (data: { destination: string; text: string; source?: string }) =>
    apiFetch<{ message_id: string; status: string }>('/messages', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  get: (id: string) => apiFetch<unknown>(`/messages/${id}`),
  exportCsv: (params: Record<string, string> = {}) => {
    const qs = new URLSearchParams(params).toString();
    const csrfToken = getCookie('csrf_token');
    return fetch(`${API_BASE}/messages/export?${qs}`, {
      credentials: 'include',
      headers: csrfToken ? { 'X-CSRF-Token': csrfToken } : {},
    });
  },
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
  get: (params: Record<string, string>) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<unknown>(`/analytics?${qs}`);
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
  created_at: string;
  updated_at: string;
}

export const templatesApi = {
  list: (params: Record<string, string> = {}) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<{ templates: TemplateInfo[]; total: number; page: number; per_page: number; total_pages: number }>(`/templates?${qs}`);
  },
  get: (id: string) => apiFetch<TemplateInfo>(`/templates/${id}`),
  create: (data: { name: string; body: string; sender_name_id?: string }) =>
    apiFetch<TemplateInfo>('/templates', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: { name?: string; body?: string; sender_name_id?: string }) =>
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
  create: (name: string) =>
    apiFetch<SenderNameInfo>('/sender-names', { method: 'POST', body: JSON.stringify({ name }) }),
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

// Billing API
export const billingApi = {
  getBalance: () =>
    apiFetch<{ client_id: string; balance: string; currency: string; updated_at?: string }>('/billing/balance'),
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
