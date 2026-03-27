const API_BASE = '/portal/v1';

export async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
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
    throw new ApiError(res.status, err.error?.message || res.statusText, err.error);
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
  setupTOTP: () =>
    apiFetch<{ secret: string; qr_code_url: string }>('/profile/2fa/setup', { method: 'POST' }),
  verifyTOTP: (code: string) =>
    apiFetch<unknown>('/profile/2fa/verify', {
      method: 'POST',
      body: JSON.stringify({ totp_code: code }),
    }),
  disableTOTP: (password: string) =>
    apiFetch<void>('/profile/2fa', { method: 'DELETE', body: JSON.stringify({ password }) }),
};

export interface ProfileData {
  id: string;
  email: string;
  company_name: string;
  contact_person: string;
  phone: string;
  totp_enabled: boolean;
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
