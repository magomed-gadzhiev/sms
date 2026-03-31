import { apiFetch } from './client';

// ─── Types ────────────────────────────────────────────────────────────────────

export interface DeliveryChannel {
  channel_id: string;
  channel_type: string;
  name: string;
  description: string;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface StrategyStep {
  channel_id: string;
  channel_type: string;
  channel_name: string;
  step_order: number;
  timeout_s: number;
  billable: boolean;
}

export interface DeliveryStrategy {
  strategy_id: string;
  name: string;
  description: string;
  mode: 'sequential' | 'parallel';
  active: boolean;
  steps: StrategyStep[];
  created_at: string;
  updated_at: string;
}

export interface OCSEntry {
  operator_id: string;
  operator_name: string;
  channel_type: string;
  supported: boolean;
  notes: string;
}

export interface AttemptInfo {
  id: string;
  channel_type: string;
  step_order: number;
  status: string;
  cost: string;
  currency: string;
  error_message: string;
  sent_at?: string;
  result_at?: string;
}

export interface Delivery {
  id: string;
  client_id: string;
  strategy_id: string;
  recipient: string;
  status: string;
  delivered_via: string;
  total_cost: string;
  currency: string;
  attempts: AttemptInfo[];
  created_at: string;
  updated_at: string;
}

export interface DeliveryStats {
  total_deliveries: number;
  delivered_count: number;
  failed_count: number;
  delivery_rate: string;
  channel_stats: {
    channel_type: string;
    attempt_count: number;
    delivered_count: number;
    delivery_rate: string;
    avg_latency_ms: string;
    avg_cost: string;
  }[];
  avg_cost: string;
  total_cost: string;
  currency: string;
}

// ─── Admin: Channels ─────────────────────────────────────────────────────────

export const cascadeChannelsApi = {
  list: () =>
    apiFetch<{ channels: DeliveryChannel[] }>('/admin/channels'),

  get: (id: string) =>
    apiFetch<DeliveryChannel>(`/admin/channels/${id}`),

  create: (data: { channel_type: string; name: string; description?: string; config_json?: string }) =>
    apiFetch<DeliveryChannel>('/admin/channels', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (id: string, data: { name: string; description?: string; config_json?: string }) =>
    apiFetch<DeliveryChannel>(`/admin/channels/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  toggle: (id: string, active: boolean) =>
    apiFetch<DeliveryChannel>(`/admin/channels/${id}/toggle`, {
      method: 'PUT',
      body: JSON.stringify({ active }),
    }),
};

// ─── Admin: Strategies ───────────────────────────────────────────────────────

export interface CreateStrategyInput {
  name: string;
  description?: string;
  mode: string;
  steps: { channel_id: string; step_order: number; timeout_s: number; billable: boolean }[];
}

export const cascadeStrategiesApi = {
  list: (activeOnly = false) =>
    apiFetch<{ strategies: DeliveryStrategy[] }>(`/admin/delivery-strategies?active_only=${activeOnly}`),

  get: (id: string) =>
    apiFetch<DeliveryStrategy>(`/admin/delivery-strategies/${id}`),

  create: (data: CreateStrategyInput) =>
    apiFetch<DeliveryStrategy>('/admin/delivery-strategies', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (id: string, data: CreateStrategyInput) =>
    apiFetch<DeliveryStrategy>(`/admin/delivery-strategies/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  delete: (id: string) =>
    apiFetch<{ success: boolean }>(`/admin/delivery-strategies/${id}`, { method: 'DELETE' }),
};

// ─── Admin: Operator Channel Support ─────────────────────────────────────────

export const cascadeOCSApi = {
  list: (operatorId?: string) => {
    const qs = operatorId ? `?operator_id=${operatorId}` : '';
    return apiFetch<{ entries: OCSEntry[] }>(`/admin/operator-channel-support${qs}`);
  },

  update: (data: { operator_id: string; channel_type: string; supported: boolean; notes?: string }) =>
    apiFetch<OCSEntry>('/admin/operator-channel-support', {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
};

// ─── Client: Deliveries ───────────────────────────────────────────────────────

export interface ListDeliveriesFilter {
  strategy_id?: string;
  status?: string;
  date_from?: string;
  date_to?: string;
  page?: number;
  page_size?: number;
}

export const cascadeDeliveriesApi = {
  list: (filter: ListDeliveriesFilter = {}) => {
    const qs = new URLSearchParams();
    if (filter.strategy_id) qs.set('strategy_id', filter.strategy_id);
    if (filter.status) qs.set('status', filter.status);
    if (filter.date_from) qs.set('date_from', filter.date_from);
    if (filter.date_to) qs.set('date_to', filter.date_to);
    if (filter.page) qs.set('page', String(filter.page));
    if (filter.page_size) qs.set('page_size', String(filter.page_size));
    return apiFetch<{ deliveries: Delivery[]; total: number; page: number }>(
      `/cascade/deliveries?${qs.toString()}`
    );
  },

  get: (id: string) =>
    apiFetch<Delivery>(`/cascade/deliveries/${id}`),

  getStats: (filter: { strategy_id?: string; date_from?: string; date_to?: string } = {}) => {
    const qs = new URLSearchParams();
    if (filter.strategy_id) qs.set('strategy_id', filter.strategy_id);
    if (filter.date_from) qs.set('date_from', filter.date_from);
    if (filter.date_to) qs.set('date_to', filter.date_to);
    return apiFetch<DeliveryStats>(`/cascade/stats?${qs.toString()}`);
  },
};
