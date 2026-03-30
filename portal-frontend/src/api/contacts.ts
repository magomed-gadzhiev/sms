import { apiFetch } from './client';

export interface ContactList {
  id: string;
  name: string;
  description: string;
  contacts_count: number;
  created_at: any; // string or protobuf Timestamp {seconds, nanos}
}

export interface ContactAttribute {
  id?: string;
  name: string;
  display_name: string;
  type: 'string' | 'number' | 'date' | 'boolean';
  required: boolean;
  position: number;
}

export interface Contact {
  id: string;
  phone: string;
  attributes: Record<string, unknown>;
  tags: string[];
  created_at: string;
}

export interface ImportJob {
  id: string;
  file_name: string;
  status: string;
  total_rows: number;
  imported_count: number;
  updated_count: number;
  error_count: number;
  created_at: string;
}

export const contactListsApi = {
  list: (page = 1, perPage = 20) =>
    apiFetch<{ items: ContactList[]; total: number }>(
      `/contact-lists?page=${page}&per_page=${perPage}`,
    ),

  create: (data: { name: string; description?: string }) =>
    apiFetch<ContactList>('/contact-lists', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  get: (id: string) => apiFetch<ContactList>(`/contact-lists/${id}`),

  update: (id: string, data: { name: string; description?: string }) =>
    apiFetch<ContactList>(`/contact-lists/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  remove: (id: string) =>
    apiFetch<void>(`/contact-lists/${id}`, { method: 'DELETE' }),

  getAttributes: (id: string) =>
    apiFetch<{ attributes: ContactAttribute[] }>(
      `/contact-lists/${id}/attributes`,
    ),

  setAttributes: (id: string, attrs: ContactAttribute[]) =>
    apiFetch<{ attributes: ContactAttribute[] }>(
      `/contact-lists/${id}/attributes`,
      { method: 'PUT', body: JSON.stringify({ attributes: attrs }) },
    ),

  listContacts: (id: string, page = 1, perPage = 20, search = '') => {
    const qs = new URLSearchParams({
      page: String(page),
      per_page: String(perPage),
      ...(search ? { search } : {}),
    }).toString();
    return apiFetch<{ contacts: Contact[]; total: number }>(
      `/contact-lists/${id}/contacts?${qs}`,
    );
  },

  createContact: (
    id: string,
    data: {
      phone: string;
      attributes?: Record<string, unknown>;
      tags?: string[];
    },
  ) =>
    apiFetch<Contact>(`/contact-lists/${id}/contacts`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  deleteContact: (listId: string, contactId: string) =>
    apiFetch<void>(`/contact-lists/${listId}/contacts/${contactId}`, {
      method: 'DELETE',
    }),

  listTags: (id: string) =>
    apiFetch<{ tags: string[] }>(`/contact-lists/${id}/tags`),

  uploadImport: (id: string, file: File) => {
    const fd = new FormData();
    fd.append('file', file);
    return apiFetch<{ import_id: string; preview: string[][] }>(
      `/contact-lists/${id}/imports/upload`,
      { method: 'POST', body: fd },
    );
  },

  startImport: (listId: string, importId: string, mapping: unknown) =>
    apiFetch<ImportJob>(
      `/contact-lists/${listId}/imports/${importId}/start`,
      { method: 'POST', body: JSON.stringify({ column_mapping: mapping }) },
    ),

  getImportStatus: (listId: string, importId: string) =>
    apiFetch<ImportJob>(`/contact-lists/${listId}/imports/${importId}`),

  listImports: (listId: string) =>
    apiFetch<{ imports: ImportJob[]; total: number }>(
      `/contact-lists/${listId}/imports`,
    ),

  previewSegment: (listId: string, rules: unknown, tags?: string[]) =>
    apiFetch<{ count: number }>(
      `/contact-lists/${listId}/segment/preview`,
      { method: 'POST', body: JSON.stringify({ rules, tags }) },
    ),
};
