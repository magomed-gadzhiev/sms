INSERT INTO network_stats_hourly (
    partner_id, hour, provider_id, operator, country, channel,
    login, sender_name, traffic_type, method,
    total, sent, delivered, failed, pending, timeout, error,
    revenue, cost,
    dlr_latency_sum, dlr_latency_cnt, dlr_latency_p50, dlr_latency_p95, throughput_max
)
SELECT
    0::bigint AS partner_id,
    date_trunc('hour', m.created_at) AS hour,
    0::bigint AS provider_id,
    COALESCE(op.name, '') AS operator,
    COALESCE(co.iso_code, '') AS country,
    COALESCE(NULLIF(m.channel, ''), 'sms') AS channel,
    COALESCE(c.name, c.email, '') AS login,
    COALESCE(m.source, '') AS sender_name,
    COALESCE(m.service_type, '') AS traffic_type,
    COALESCE(m.send_method, '') AS method,
    COUNT(*)::bigint AS total,
    COUNT(*) FILTER (WHERE m.status = 'sent')::bigint AS sent,
    COUNT(*) FILTER (WHERE m.status = 'delivered')::bigint AS delivered,
    COUNT(*) FILTER (WHERE m.status = 'failed')::bigint AS failed,
    COUNT(*) FILTER (WHERE m.status = 'pending')::bigint AS pending,
    COUNT(*) FILTER (WHERE m.status = 'expired')::bigint AS timeout,
    COUNT(*) FILTER (WHERE m.status IN ('failed','rejected'))::bigint AS error,
    0, 0, 0, 0, 0, 0, 0
FROM messages m
LEFT JOIN clients   c  ON c.id  = m.client_id
LEFT JOIN operators op ON op.id = m.operator_id
LEFT JOIN countries co ON co.id = m.country_id
WHERE m.created_at >= NOW() - INTERVAL '90 days'
GROUP BY 1, 2, 3, 4, 5, 6, 7, 8, 9, 10
ON CONFLICT (partner_id, hour, provider_id, operator, country, channel, login, sender_name, traffic_type, method) DO NOTHING;

SELECT COUNT(*) AS rows, SUM(total) AS total,
       COUNT(DISTINCT operator) AS dops,
       COUNT(DISTINCT channel) AS dchan,
       COUNT(DISTINCT login) AS dlog
FROM network_stats_hourly;
