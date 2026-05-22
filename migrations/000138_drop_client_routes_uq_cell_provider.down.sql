-- Восстановить sandbox-only constraint (best-effort; в чистом dev этой строки
-- никогда не было). UNIQUE-выражение точно совпадает со sandbox-формой,
-- зафиксированной в \d client_routes на 2026-05-05.
ALTER TABLE client_routes ADD CONSTRAINT uq_cell_provider UNIQUE
  (owner_type, COALESCE(owner_id, '00000000-0000-0000-0000-000000000000'::uuid),
   route_type, COALESCE(operator_id, '00000000-0000-0000-0000-000000000000'::uuid),
   COALESCE(country_code, ''::bpchar), COALESCE(traffic_type, ''::text),
   COALESCE(number_from, '-1'::integer::bigint), COALESCE(number_to, '-1'::integer::bigint),
   provider_id);
