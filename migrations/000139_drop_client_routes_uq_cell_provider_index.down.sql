-- Восстановить sandbox-only UNIQUE INDEX (best-effort; в чистом dev этого
-- индекса никогда не было). Выражение совпадает с тем, что показывал
-- \d client_routes на 2026-05-05.
CREATE UNIQUE INDEX IF NOT EXISTS uq_cell_provider ON client_routes
  (owner_type, COALESCE(owner_id, '00000000-0000-0000-0000-000000000000'::uuid),
   route_type, COALESCE(operator_id, '00000000-0000-0000-0000-000000000000'::uuid),
   COALESCE(country_code, ''::bpchar), COALESCE(traffic_type, ''::text),
   COALESCE(number_from, '-1'::integer::bigint), COALESCE(number_to, '-1'::integer::bigint),
   provider_id);
