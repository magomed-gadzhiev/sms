import { apiFetch } from './client';

export interface Campaign {
  id: string;
  name: string;
  status: string;
  contact_list_id: string;
  template_id: string;
  source: string;
  total_recipients: number;
  sent_count: number;
  delivered_count: number;
  failed_count: number;
  created_at: string;
  started_at?: string;
  completed_at?: string;
  variants?: Variant[];
  ab_config?: ABConfig;
}

export interface Variant {
  id: string;
  name: string;
  template_id: string;
  percentage: number;
  is_winner: boolean;
  is_control: boolean;
  sent_count: number;
  delivered_count: number;
  failed_count: number;
}

export interface ABConfig {
  metric: string;
  test_duration_hours: number;
  auto_select_winner: boolean;
  winner_variant_id?: string;
}

export interface CampaignStats {
  total_recipients: number;
  sent: number;
  delivered: number;
  failed: number;
  pending: number;
  delivery_rate: number;
  total_cost: number;
  per_variant: {
    variant_id: string;
    variant_name: string;
    sent: number;
    delivered: number;
    delivery_rate: number;
  }[];
}

export const campaignsApi = {
  list: (page = 1, perPage = 20, status = '') => {
    const qs = new URLSearchParams({
      page: String(page),
      per_page: String(perPage),
      ...(status ? { status } : {}),
    }).toString();
    return apiFetch<{ campaigns: Campaign[]; total: number }>(
      `/campaigns?${qs}`,
    );
  },

  create: (data: {
    name: string;
    contact_list_id: string;
    template_id?: string;
    source?: string;
    send_rate?: number;
  }) =>
    apiFetch<Campaign>('/campaigns', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  get: (id: string) => apiFetch<Campaign>(`/campaigns/${id}`),

  update: (id: string, data: Record<string, unknown>) =>
    apiFetch<Campaign>(`/campaigns/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  remove: (id: string) =>
    apiFetch<void>(`/campaigns/${id}`, { method: 'DELETE' }),

  launch: (id: string) =>
    apiFetch<Campaign>(`/campaigns/${id}/launch`, { method: 'POST' }),

  pause: (id: string) =>
    apiFetch<Campaign>(`/campaigns/${id}/pause`, { method: 'POST' }),

  resume: (id: string) =>
    apiFetch<Campaign>(`/campaigns/${id}/resume`, { method: 'POST' }),

  cancel: (id: string) =>
    apiFetch<Campaign>(`/campaigns/${id}/cancel`, { method: 'POST' }),

  setVariants: (
    id: string,
    variants: {
      name: string;
      template_id: string;
      percentage: number;
      is_control?: boolean;
    }[],
  ) =>
    apiFetch<{ variants: Variant[] }>(`/campaigns/${id}/variants`, {
      method: 'PUT',
      body: JSON.stringify({ variants }),
    }),

  setABConfig: (
    id: string,
    cfg: {
      metric: string;
      test_duration_hours: number;
      auto_select_winner: boolean;
    },
  ) =>
    apiFetch<ABConfig>(`/campaigns/${id}/ab-config`, {
      method: 'PUT',
      body: JSON.stringify(cfg),
    }),

  selectWinner: (id: string, variantId: string) =>
    apiFetch<Campaign>(`/campaigns/${id}/select-winner`, {
      method: 'POST',
      body: JSON.stringify({ variant_id: variantId }),
    }),

  setRetryConfig: (
    id: string,
    cfg: { enabled: boolean; delay_hours: number; max_retries: number },
  ) =>
    apiFetch<unknown>(`/campaigns/${id}/retry-config`, {
      method: 'PUT',
      body: JSON.stringify(cfg),
    }),

  retryFailed: (id: string) =>
    apiFetch<Campaign>(`/campaigns/${id}/retry`, { method: 'POST' }),

  getStats: (id: string) =>
    apiFetch<CampaignStats>(`/campaigns/${id}/stats`),

  getTimeline: (id: string, interval = '5min', metric = 'delivered') =>
    apiFetch<{ points: { timestamp: string; value: number }[] }>(
      `/campaigns/${id}/timeline?interval=${interval}&metric=${metric}`,
    ),

  getVariantComparison: (id: string) =>
    apiFetch<{
      rows: {
        variant_id: string;
        variant_name: string;
        sent: number;
        delivered: number;
        delivery_rate: number;
      }[];
      winner_variant_id: string;
    }>(`/campaigns/${id}/variants/compare`),

  getHeatmap: (id: string) =>
    apiFetch<{
      cells: {
        day_of_week: number;
        hour: number;
        delivered_count: number;
        delivery_rate: number;
      }[];
    }>(`/campaigns/${id}/heatmap`),
};
