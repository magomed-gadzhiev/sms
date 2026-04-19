import type { ApiHelper } from './api';

export async function pickProviderId(api: ApiHelper): Promise<string | null> {
  const resp = await api.listRouteProviders();
  return resp.providers?.[0]?.id ?? null;
}

export interface SeedRouteOverrides {
  name?: string;
  route_type?: string;
  priority?: number;
  share?: number;
  status?: string;
}

export async function seedRoute(
  api: ApiHelper,
  providerId: string,
  overrides: SeedRouteOverrides = {},
  comment = 'seeded by routing e2e spec',
): Promise<{ id: string; name: string }> {
  const name =
    overrides.name ??
    `E2E Seed ${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
  const created = await api.createRoute({
    name,
    route_type: overrides.route_type ?? 'sms',
    provider_id: providerId,
    priority: overrides.priority ?? 50,
    share: overrides.share ?? 100,
    status: overrides.status ?? 'active',
    comment,
    condition_groups: [{ logic_op: 'IF', conditions: [{ type: 'operator', value: '' }] }],
  });
  const id = created.id || created.route_id;
  if (!id) throw new Error(`seedRoute failed: ${JSON.stringify(created)}`);
  return { id, name };
}

export async function cleanupRoutesByPrefix(api: ApiHelper, prefix: string): Promise<void> {
  if (!prefix) return;
  const resp = await api.listRoutes();
  const matches = (resp.routes ?? []).filter((r: { name: string }) => r.name.startsWith(prefix));
  for (const r of matches as Array<{ id: string }>) {
    await api.deleteRoute(r.id).catch(() => undefined);
  }
}
