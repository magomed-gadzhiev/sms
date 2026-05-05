-- Plan 5 Task 9 (D8+D9): SQL функции для operator/country resolution в network_route_preview.

CREATE OR REPLACE FUNCTION resolve_operator_id_by_phone(p_phone TEXT)
RETURNS UUID
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
    result_id UUID;
    clean_phone TEXT;
BEGIN
    IF p_phone IS NULL OR p_phone = '' THEN RETURN NULL; END IF;
    clean_phone := regexp_replace(p_phone, '[^0-9]', '', 'g');
    IF clean_phone = '' THEN RETURN NULL; END IF;
    SELECT operator_id INTO result_id
      FROM operator_prefixes
     WHERE clean_phone LIKE prefix || '%'
       AND active = true
     ORDER BY length(prefix) DESC, priority DESC
     LIMIT 1;
    RETURN result_id;
END;
$$;

COMMENT ON FUNCTION resolve_operator_id_by_phone(TEXT) IS
    'Plan 5: longest-prefix-match operator lookup for preview-routes operator-condition matching';

CREATE OR REPLACE FUNCTION resolve_country_iso_by_phone(p_phone TEXT)
RETURNS VARCHAR(2)
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
    result_iso VARCHAR(2);
    clean_phone TEXT;
BEGIN
    IF p_phone IS NULL OR p_phone = '' THEN RETURN ''; END IF;
    clean_phone := regexp_replace(p_phone, '[^0-9]', '', 'g');
    IF clean_phone = '' THEN RETURN ''; END IF;
    SELECT iso_code INTO result_iso
      FROM countries
     WHERE clean_phone LIKE phone_code || '%'
     ORDER BY length(phone_code) DESC
     LIMIT 1;
    RETURN COALESCE(result_iso, '');
END;
$$;

COMMENT ON FUNCTION resolve_country_iso_by_phone(TEXT) IS
    'Plan 5: longest-phone-code-match country lookup, replaces hardcoded SNG map in network_route_preview.go';
