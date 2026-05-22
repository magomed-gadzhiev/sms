import { apiFetch } from './client';
import { getCookie } from '../utils/cookies';

// --- Types ---

export interface SharedFilter {
  period_preset?: string;
  date_from?: string;
  date_to?: string;
  group_by?: string;
  login?: string;
  service_type?: string;
  operator?: string;
  channel?: string;
  sender_name?: string;
  traffic_type?: string;
  status?: string;
  provider?: string;
  country?: string;
  manager?: string;
  error_code?: string;
  page?: number;
  page_size?: number;
  sort_by?: string;
  sort_dir?: string;
}

export type KPIFormat = 'count' | 'percent' | 'currency';

export interface KPI {
  name: string;
  value?: number;
  status?: 'ok' | 'warning' | 'danger' | 'unknown';
  delta?: number;
  format?: KPIFormat;
  currency?: string;
}

export interface StatRow {
  slice: string;
  total: number;
  sent: number;
  delivered: number;
  failed: number;
  pending: number;
  timeout: number;
  error: number;
  dlr_rate: number;
  revenue: number;
  cost: number;
  profit: number;
  margin: number;
  health: string;
  alerts: Record<string, string>;
}

export interface MonitorRow {
  slice: string;
  throughput: number;
  sent: number;
  delivered: number;
  pending: number;
  timeout: number;
  error: number;
  dlr_latency_p50: number;
  dlr_latency_p95: number;
  dlr_rate: number;
  top_error: string;
  health: string;
}

export interface MetricPoint {
  timestamp: number;
  value: number;
}

export interface Signal {
  severity: 'danger' | 'warning' | 'success';
  text: string;
  link_type: string;
  link_value: string;
}

export interface Pagination {
  page: number;
  page_size: number;
  total_rows: number;
  total_pages: number;
}

export interface StatisticsResponse {
  kpis: KPI[];
  rows: StatRow[];
  pagination: Pagination;
}

export interface AnalyticsResponse {
  kpis: KPI[];
  previous_kpis: KPI[];
  trends: { metric: string; points: MetricPoint[] }[];
  signals: Signal[];
  rows: StatRow[];
  pagination: Pagination;
}

export interface MonitoringResponse {
  kpis: KPI[];
  rows: MonitorRow[];
  chart: MetricPoint[];
  pagination: Pagination;
}

export interface DrillDownResponse {
  summary: KPI[];
  rows: StatRow[];
  trends: { metric: string; points: MetricPoint[] }[];
  health: string;
}

export interface SavedView {
  id: number;
  name: string;
  mode: string;
  filters_json: string;
  group_by: string;
  sort_by: string;
  sort_dir: string;
  columns: string[];
  is_default: boolean;
  is_template: boolean;
}

// --- Helpers ---

function qs(params: Record<string, string | number | boolean | undefined>): string {
  const p = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '') p.set(k, String(v));
  }
  const s = p.toString();
  return s ? `?${s}` : '';
}

function filterToParams(f: SharedFilter): Record<string, string | number | boolean | undefined> {
  const params: Record<string, string | number | boolean | undefined> = {};
  for (const [k, v] of Object.entries(f)) {
    if (v !== '' && v !== undefined && v !== null) {
      params[k] = v as string | number | boolean;
    }
  }
  // Convert ISO date strings to Unix timestamps for backend
  if (typeof params.date_from === 'string') {
    const ts = Math.floor(new Date(params.date_from).getTime() / 1000);
    if (!isNaN(ts)) params.date_from = ts;
  }
  if (typeof params.date_to === 'string') {
    const ts = Math.floor(new Date(params.date_to).getTime() / 1000);
    if (!isNaN(ts)) params.date_to = ts;
  }
  return params;
}

// --- API ---

export const networkStatsApi = {
  getStatistics: (filter: SharedFilter) =>
    apiFetch<StatisticsResponse>(`/reseller/statistics${qs(filterToParams(filter))}`),

  getAnalytics: (filter: SharedFilter) =>
    apiFetch<AnalyticsResponse>(`/reseller/analytics-summary${qs(filterToParams(filter))}`),

  getMonitoring: (filter: SharedFilter, hideHealthy = false) =>
    apiFetch<MonitoringResponse>(
      `/reseller/monitoring${qs({ ...filterToParams(filter), hide_healthy: hideHealthy })}`,
    ),

  getDrillDown: (
    filter: SharedFilter,
    sliceType: string,
    sliceValue: string,
    detailView: string,
    parentType?: string,
    parentValue?: string,
  ) =>
    apiFetch<DrillDownResponse>(
      `/reseller/drilldown${qs({
        ...filterToParams(filter),
        slice_type: sliceType,
        slice_value: sliceValue,
        detail_view: detailView,
        parent_type: parentType,
        parent_value: parentValue,
      })}`,
    ),

  startExport: (filter: SharedFilter, mode: string, format: 'csv' | 'xlsx') =>
    apiFetch<{ job_id: string }>('/reseller/export', {
      method: 'POST',
      body: JSON.stringify({ filter: filterToParams(filter), mode, format }),
    }),

  getExportStatus: (jobId: string) =>
    apiFetch<{ status: string; row_count: number; error: string; download_url: string }>(
      `/reseller/export/${jobId}/status`,
    ),

  downloadExport: (jobId: string) => {
    const csrfToken = getCookie('csrf_token');
    return fetch(`/portal/v1/reseller/export/${jobId}/download`, {
      credentials: 'include',
      headers: csrfToken ? { 'X-CSRF-Token': csrfToken } : {},
    });
  },

  listViews: () => apiFetch<{ views: SavedView[] }>('/reseller/views'),

  saveView: (view: Omit<SavedView, 'id' | 'is_template'>) =>
    apiFetch<{ view: SavedView }>('/reseller/views', {
      method: 'POST',
      body: JSON.stringify({ view }),
    }),

  deleteView: (id: number) => apiFetch<void>(`/reseller/views/${id}`, { method: 'DELETE' }),
};
