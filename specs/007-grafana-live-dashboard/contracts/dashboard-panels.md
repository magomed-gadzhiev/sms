# Dashboard Panel Layout Contract

**Dashboard**: Live Load Test | **UID**: `load-test-live` | **Refresh**: 5s

## Layout (Grid: 24 columns)

### Row 1: Finance (y=1, collapsed=false)

| Panel | Type | Width | Datasource | Query |
|-------|------|-------|------------|-------|
| Current Balance | stat | 8 | PostgreSQL | `SELECT balance FROM accounts WHERE client_id = '$client_id'` |
| Accumulated Cost | stat | 8 | PostgreSQL | `SELECT SUM(total_amount) FROM tarification_log WHERE client_id='$client_id' AND created_at >= NOW() - INTERVAL '$time_range'` |
| Balance Over Time | timeseries | 8 | PostgreSQL | Time-series по `transactions.balance_after` |

### Row 2: Messages (y=10, collapsed=false)

| Panel | Type | Width | Datasource | Query |
|-------|------|-------|------------|-------|
| Messages Sent | stat | 6 | Prometheus | `sum(smpp_messages_sent_total)` |
| Messages Delivered | stat | 6 | PostgreSQL | `SELECT COUNT(*) FROM messages WHERE status='delivered' AND ...` |
| Messages Failed | stat | 6 | Prometheus | `sum(smpp_messages_failed_total)` |
| Messages Queued | stat | 6 | Prometheus | `sum(sms_messages_queued_total)` |

### Row 3: Performance (y=19, collapsed=false)

| Panel | Type | Width | Datasource | Query |
|-------|------|-------|------------|-------|
| Throughput (msg/s) | stat | 8 | Prometheus | `sum(rate(smpp_messages_sent_total[1m]))` |
| Delivery Rate % | gauge | 8 | PostgreSQL | delivered / (delivered+failed) * 100 |
| Throughput Over Time | timeseries | 8 | Prometheus | `sum(rate(smpp_messages_sent_total[1m]))` over time |

### Row 4: Providers (y=28, collapsed=false)

| Panel | Type | Width | Datasource | Query |
|-------|------|-------|------------|-------|
| Traffic by Provider | bargauge | 12 | Prometheus | `sum by (provider_name)(smpp_messages_sent_total)` |
| Provider Latency (p95) | timeseries | 12 | Prometheus | `histogram_quantile(0.95, sum by (le, provider_name)(...))` |

### Row 5: Operators (y=37, collapsed=false)

| Panel | Type | Width | Datasource | Query |
|-------|------|-------|------------|-------|
| Traffic by Operator | bargauge | 12 | PostgreSQL | Messages grouped by operator (destination prefix) |
| Delivery Rate by Provider | timeseries | 12 | PostgreSQL | `aggregated_metrics` grouped by provider_id |

### Row 6: Message Feed (y=46, collapsed=false)

| Panel | Type | Width | Datasource | Query |
|-------|------|-------|------------|-------|
| Recent Messages | table | 24 | PostgreSQL | Last 20 messages with masked phone, status, provider, time |

### Row 7: Historical Graphs (y=55, collapsed=false)

| Panel | Type | Width | Datasource | Query |
|-------|------|-------|------------|-------|
| msg/s History | timeseries | 12 | Prometheus | `sum(rate(smpp_messages_sent_total[1m]))` |
| Delivery Rate & Latency | timeseries | 12 | Prometheus + PG | Overlay: delivery rate + p95 latency |

## Template Variables

| Variable | Label | Type | Query | Default |
|----------|-------|------|-------|---------|
| `client_id` | Client | query | `SELECT id AS __value, name AS __text FROM clients WHERE active=true` | First |
| `time_range` | Session Duration | interval | `5m,15m,30m,1h,2h` | `30m` |

## Annotations

| Name | Datasource | Query | Color |
|------|------------|-------|-------|
| Balance Zero | PostgreSQL | `SELECT created_at FROM transactions WHERE balance_after <= 0` | red |
