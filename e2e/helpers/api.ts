import { type APIRequestContext } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

const API_BASE = process.env.API_URL || 'http://localhost:8083/portal/v1';

function extractCsrfToken(): string {
  try {
    const statePath = path.resolve(__dirname, '..', 'auth-state.json');
    const state = JSON.parse(fs.readFileSync(statePath, 'utf-8'));
    const csrfCookie = state.cookies?.find((c: { name: string; value: string }) => c.name === 'csrf_token');
    return csrfCookie?.value || '';
  } catch {
    return '';
  }
}

export class ApiHelper {
  private csrfToken: string;

  constructor(private request: APIRequestContext) {
    this.csrfToken = extractCsrfToken();
  }

  private async fetch(path: string, options?: {
    method?: string;
    data?: unknown;
  }) {
    const url = `${API_BASE}${path}`;
    const method = options?.method || 'GET';
    const headers: Record<string, string> = {};

    // Add CSRF token for non-GET requests
    if (method !== 'GET' && this.csrfToken) {
      headers['X-CSRF-Token'] = this.csrfToken;
    }

    if (method === 'GET') {
      return this.request.get(url);
    }
    return this.request.fetch(url, {
      method,
      data: options?.data,
      headers,
    });
  }

  // --- Auth ---
  async login(email: string, password: string) {
    return this.fetch('/auth/login', { method: 'POST', data: { email, password } });
  }

  // --- Billing ---
  async getBalance() {
    const res = await this.fetch('/billing/balance');
    return res.json();
  }

  async getTransactions(params: Record<string, string> = {}) {
    const qs = new URLSearchParams(params).toString();
    const res = await this.fetch(`/billing/transactions?${qs}`);
    return res.json();
  }

  // --- Messages ---
  async sendMessage(destination: string, text: string, source: string) {
    const res = await this.fetch('/messages', {
      method: 'POST',
      data: { destination, text, source },
    });
    return res.json();
  }

  async getMessage(id: string) {
    const res = await this.fetch(`/detalization/${id}`);
    return res.json();
  }

  async listMessages(params: Record<string, string> = {}) {
    const qs = new URLSearchParams(params).toString();
    const res = await this.fetch(`/detalization?${qs}`);
    return res.json();
  }

  // --- Campaigns ---
  async createCampaign(data: {
    name: string;
    contact_list_id: string;
    template_id?: string;
    source?: string;
    send_rate?: number;
    scheduled_at?: string;
    use_subscriber_timezone?: boolean;
  }) {
    const res = await this.fetch('/campaigns', { method: 'POST', data });
    return res.json();
  }

  async getCampaign(id: string) {
    const res = await this.fetch(`/campaigns/${id}`);
    return res.json();
  }

  async launchCampaign(id: string) {
    const res = await this.fetch(`/campaigns/${id}/launch`, { method: 'POST' });
    return res.json();
  }

  async getCampaignStats(id: string) {
    const res = await this.fetch(`/campaigns/${id}/stats`);
    return res.json();
  }

  async deleteCampaign(id: string) {
    return this.fetch(`/campaigns/${id}`, { method: 'DELETE' });
  }

  async estimateCost(data: {
    contact_list_id: string;
    text: string;
    source: string;
  }) {
    const res = await this.fetch('/campaigns/estimate-cost', { method: 'POST', data });
    return res.json();
  }

  // --- Tariffs ---
  async getCurrentTariff() {
    const res = await this.fetch('/tariffs/current');
    return res.json();
  }

  async listTariffPlans() {
    const res = await this.fetch('/tariffs/plans');
    return res.json();
  }

  // --- Sender Names ---
  async listApprovedSenderNames() {
    const res = await this.fetch('/sender-names?status=approved&per_page=100');
    return res.json();
  }

  // --- Contact Lists ---
  async listContactLists() {
    const res = await this.fetch('/contact-lists?page=1&per_page=100');
    return res.json();
  }

  // --- Templates ---
  async listTemplates(params: Record<string, string> = {}) {
    const qs = new URLSearchParams({ per_page: '100', ...params }).toString();
    const res = await this.fetch(`/templates?${qs}`);
    return res.json();
  }

  // --- Routes ---
  async listRoutes(params: Record<string, string> = {}) {
    const qs = new URLSearchParams(params).toString();
    const res = await this.fetch(`/routes?${qs}`);
    return res.json();
  }

  async getRoute(id: string) {
    const res = await this.fetch(`/routes/${id}`);
    return res.json();
  }

  async createRoute(data: {
    name: string;
    route_type: string;
    provider_id: string;
    priority: number;
    share: number;
    status: string;
    comment?: string;
    condition_groups?: unknown[];
    schedules?: unknown[];
  }) {
    const res = await this.fetch('/routes', { method: 'POST', data });
    return res.json();
  }

  async deleteRoute(id: string) {
    return this.fetch(`/routes/${id}`, { method: 'DELETE' });
  }

  async listRouteProviders() {
    const res = await this.fetch('/routes/providers');
    return res.json();
  }
}

// Admin API helper (uses /admin/v1 base)
const ADMIN_API_BASE = process.env.ADMIN_API_URL || `${process.env.BASE_URL || 'http://localhost:8083'}/admin/v1`;

export class AdminApiHelper {
  private csrfToken: string;

  constructor(private request: import('@playwright/test').APIRequestContext) {
    this.csrfToken = extractCsrfToken();
  }

  private async fetch(path: string, options?: {
    method?: string;
    data?: unknown;
  }) {
    const url = `${ADMIN_API_BASE}${path}`;
    const method = options?.method || 'GET';
    const headers: Record<string, string> = {};

    if (method !== 'GET' && this.csrfToken) {
      headers['X-CSRF-Token'] = this.csrfToken;
    }

    if (method === 'GET') {
      return this.request.get(url);
    }
    return this.request.fetch(url, {
      method,
      data: options?.data,
      headers,
    });
  }

  // --- Tarification ---
  async listTariffPlans() {
    const res = await this.fetch('/tarification/tariff-plans');
    return res.json();
  }

  async createTariffPlan(data: { operator_id: string; sender_category: string; strategy: string }) {
    const res = await this.fetch('/tarification/tariff-plans', { method: 'POST', data });
    return res.json();
  }

  async updateTariffPlan(id: string, data: { active?: boolean }) {
    const res = await this.fetch(`/tarification/tariff-plans/${id}`, { method: 'PUT', data });
    return res.json();
  }

  async listPeriods(params: Record<string, string> = {}) {
    const qs = new URLSearchParams(params).toString();
    const res = await this.fetch(`/tarification/periods?${qs}`);
    return res.json();
  }

  async createPeriod(data: {
    country_id?: string | null;
    operator_id?: string | null;
    sender_category?: string | null;
    traffic_type?: string | null;
    client_id?: string | null;
    strategy: string;
    start_date: string;
  }) {
    const res = await this.fetch('/tarification/periods', { method: 'POST', data });
    return res.json();
  }

  async deletePeriod(id: string) {
    return this.fetch(`/tarification/periods/${id}`, { method: 'DELETE' });
  }

  async listPeriodTiers(periodId: string) {
    const res = await this.fetch(`/tarification/periods/${periodId}/tiers`);
    return res.json();
  }

  async createPeriodTier(periodId: string, data: { from_count: number; price_per_segment: string }) {
    const res = await this.fetch(`/tarification/periods/${periodId}/tiers`, { method: 'POST', data });
    return res.json();
  }

  async listOperators(params: Record<string, string> = {}) {
    const qs = new URLSearchParams(params).toString();
    const res = await this.fetch(`/operators?${qs}`);
    return res.json();
  }
}
