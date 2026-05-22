import { apiFetch } from './client';

export interface OptOutEntry {
  id: string;
  client_id: string;
  phone: string;
  keyword: string;
  opted_out_at: string;
}

export interface OptOutListResponse {
  items: OptOutEntry[];
  total: number;
  page: number;
}

export interface ImportOptOutResult {
  imported: number;
  skipped: number;
}

export const optOutApi = {
  list: (params?: { page?: number; per_page?: number; search?: string }) => {
    const q = new URLSearchParams();
    if (params?.page) q.set('page', String(params.page));
    if (params?.per_page) q.set('per_page', String(params.per_page));
    if (params?.search) q.set('search', params.search);
    return apiFetch<OptOutListResponse>(`/opt-out?${q.toString()}`);
  },

  add: (phone: string, keyword?: string) =>
    apiFetch<OptOutEntry>('/opt-out', {
      method: 'POST',
      body: JSON.stringify({ phone, keyword: keyword ?? '' }),
    }),

  remove: (id: string) =>
    apiFetch<void>(`/opt-out/${id}`, { method: 'DELETE' }),

  import: (phones: string[]) =>
    apiFetch<ImportOptOutResult>('/opt-out/import', {
      method: 'POST',
      body: JSON.stringify({ phones }),
    }),
};
