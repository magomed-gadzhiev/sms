# Pipeline Bottleneck Logging — Grafana Loki Design

**Date:** 2026-04-11
**Status:** Approved
**Approach:** Grafana Loki + Promtail + trace_id propagation + timing fields

---

## Goal

Make pipeline bottlenecks visible in Grafana: find where messages spend time waiting (Kafka lag per stage), trace a single message end-to-end by `trace_id`, and see throughput divergence between pipeline stages.

**Trigger:** Load test run 6 showed pipeline-sender lag of ~79,000 messages/partition × 16 partitions = ~1.26M total backlog on `sms.routed`, with 100% pipeline timeout errors. Logs had no timing data to diagnose this.

---

## Architecture

```
Docker containers (zerolog JSON → stderr)
        │
   Promtail (reads Docker socket)
        │  labels: container, service
        │  pipeline: json parse → extract fields
        ▼
      Loki  :3100
        │
   Grafana :3001  (existing)
        └── datasource: Loki
        └── dashboard: "Pipeline Bottleneck"
```

All pipeline stages already log structured JSON events with `trace_id`, `message_id`, `stage`, `event`, `component`. After this spec is implemented, they will also log `kafka_wait_ms` and `processing_ms`.

---

## Components

### 1. Loki

**File:** `deployments/configs/loki/loki.yml`

```yaml
auth_enabled: false

server:
  http_listen_port: 3100

common:
  path_prefix: /loki
  storage:
    filesystem:
      chunks_directory: /loki/chunks
      rules_directory: /loki/rules
  replication_factor: 1
  ring:
    instance_addr: 127.0.0.1
    kvstore:
      store: inmemory

schema_config:
  configs:
    - from: 2024-01-01
      store: tsdb
      object_store: filesystem
      schema: v13
      index:
        prefix: index_
        period: 24h

limits_config:
  retention_period: 168h   # 7 days
  ingestion_rate_mb: 16
  ingestion_burst_size_mb: 32
```

Added to `docker-compose.yml`:
```yaml
loki:
  image: grafana/loki:3.0.0
  container_name: loki
  ports:
    - "3100:3100"
  volumes:
    - ./configs/loki/loki.yml:/etc/loki/local-config.yaml
    - loki_data:/loki
  command: -config.file=/etc/loki/local-config.yaml
  restart: unless-stopped
```

### 2. Promtail

**File:** `deployments/configs/promtail/promtail.yml`

Reads all container logs via Docker socket. Parses JSON and extracts structured fields as Loki labels and structured metadata.

```yaml
server:
  http_listen_port: 9080
  grpc_listen_port: 0

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://loki:3100/loki/api/v1/push

scrape_configs:
  - job_name: docker
    docker_sd_configs:
      - host: unix:///var/run/docker.sock
        refresh_interval: 5s
    relabel_configs:
      # Use container name as service label, stripping "deployments-" prefix and "-N" suffix
      - source_labels: [__meta_docker_container_name]
        regex: '/?(?:deployments-)?(.*?)(?:-\d+)?$'
        target_label: service
      - source_labels: [__meta_docker_container_name]
        target_label: container
        regex: '/?(.+)'
    pipeline_stages:
      - json:
          expressions:
            level: level
            component: component
            trace_id: trace_id
            message_id: message_id
            stage: stage
            event: event
            kafka_wait_ms: kafka_wait_ms
            processing_ms: processing_ms
      - labels:
          level:
          component:
```

Added to `docker-compose.yml`:
```yaml
promtail:
  image: grafana/promtail:3.0.0
  container_name: promtail
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock
    - ./configs/promtail/promtail.yml:/etc/promtail/config.yml
  command: -config.file=/etc/promtail/config.yml
  restart: unless-stopped
  depends_on:
    - loki
```

### 3. Grafana Loki Datasource

**File:** `deployments/configs/grafana/provisioning/datasources/loki.yml`

```yaml
apiVersion: 1
datasources:
  - name: Loki
    type: loki
    uid: loki
    url: http://loki:3100
    access: proxy
    isDefault: false
    jsonData:
      maxLines: 1000
      derivedFields:
        - name: trace_id
          matcherRegex: '"trace_id":"([^"]+)"'
          url: '/explore?left={"datasource":"loki","queries":[{"expr":"{service=~\"pipeline-.*\"} | json | trace_id=\"${__value.raw}\""}]}'
          urlDisplayLabel: "Trace in pipeline"
```

The `derivedFields` config makes `trace_id` a clickable link in log lines that opens a filtered view of all pipeline stages for that message.

---

## Code Changes

### 4. Trace ID Propagation

**File:** `internal/api/http/handlers.go`

Two identical one-line additions after `queue.FromMessage(msg)` calls — one in the single send handler, one in the batch handler:

```go
// Single send (around existing line ~111):
kafkaMsg := queue.FromMessage(msg)
kafkaMsg.TraceID = shared.GetRequestID(ctx)   // ← add this

// Batch send (around existing line ~209, inside the per-message loop):
kafkaMsg := queue.FromMessage(msg)
kafkaMsg.TraceID = shared.GetRequestID(ctx)   // ← add this
```

`shared.GetRequestID(ctx)` is already implemented in `internal/shared/context.go`. The request ID is set by `LoggingMiddleware` from `X-Request-ID` header or generated as a UUID. No other files need changes — Router and Sender stages already copy `TraceID` through correctly.

### 5. Router Stage — kafka_wait_ms

**File:** `internal/pipeline/router/stage.go`

In the existing `route_matched` log event, add `kafka_wait_ms`:

```go
// Existing log (around line 204):
log.Info().
    Str("component", "pipeline_router").
    Str("trace_id", kafkaMsg.TraceID).
    Str("message_id", kafkaMsg.MessageID.String()).
    Str("stage", "router").
    Str("event", "route_matched").
    Str("provider_id", route.ProviderID.String()).
    Str("route_name", route.Name).
    Int("priority", route.Priority).
    Bool("used_default", result.UsedDefault).
    Int("total_matched", len(result.Matched)).
    Int64("kafka_wait_ms", time.Since(kafkaMsg.CreatedAt).Milliseconds()).  // ← add
    Msg("route selected")
```

Also add to `no_route` event for diagnosing unrouted messages:
```go
Int64("kafka_wait_ms", time.Since(kafkaMsg.CreatedAt).Milliseconds()).
```

### 6. Sender Stage — kafka_wait_ms + processing_ms

**File:** `internal/pipeline/sender/stage.go`

In the existing `sender.send` / `completed` event, add both fields. The sender already has `pickupTime` or can record it at message pickup:

```go
// At the start of message processing, record pickup time:
pickupAt := time.Now()

// In the existing "completed" log event (around line 537):
log.Info().
    Str("component", "pipeline_sender").
    Str("trace_id", traceID).
    Str("message_id", routedMsg.MessageID.String()).
    Str("stage", "sender.send").
    Str("event", "completed").
    Str("provider_id", usedProviderID.String()).
    Str("smpp_message_id", result.SMPPMessageID).
    Str("status", string(result.Status)).
    Int64("kafka_wait_ms", pickupAt.Sub(routedMsg.RoutedAt).Milliseconds()).   // ← add
    Int64("processing_ms", time.Since(pickupAt).Milliseconds()).                // ← add
    Msg("message sent")
```

### 7. Status Stage — kafka_wait_ms

**File:** `internal/pipeline/status/stage.go`

In the existing `upserted` event (around line 167):

```go
log.Info().
    Str("component", "pipeline_status").
    Str("trace_id", sentMsg.TraceID).
    Str("message_id", sentMsg.MessageID.String()).
    Str("stage", "status").
    Str("event", "upserted").
    Str("status", status).
    Int64("kafka_wait_ms", time.Since(sentMsg.SentAt).Milliseconds()).  // ← add
    Msg("status upserted")
```

---

## Grafana Dashboard

**File:** `deployments/configs/grafana/dashboards/pipeline-bottleneck.json`

Dashboard provisioned automatically via existing Grafana provisioning. 6 panels:

### Panel 1 — Log Explorer (Logs panel)
```logql
{service=~"pipeline-.*"} | json | trace_id=~"$trace_id"
```
Variable `$trace_id` (text input). Shows chronological log stream for one message across all stages.

### Panel 2 — Kafka Wait by Stage (Time series)
```logql
# Router wait
avg_over_time({service="pipeline-router"} | json | kafka_wait_ms != "" | unwrap kafka_wait_ms [1m])

# Sender wait  
avg_over_time({service=~"pipeline-sender.*"} | json | kafka_wait_ms != "" | unwrap kafka_wait_ms [1m])

# Status wait
avg_over_time({service=~"pipeline-status.*"} | json | kafka_wait_ms != "" | unwrap kafka_wait_ms [1m])
```
Three lines. When sender line diverges upward → Kafka backlog on `sms.routed`.

### Panel 3 — Sender Processing Time (Time series)
```logql
avg_over_time({service=~"pipeline-sender.*"} | json | processing_ms != "" | unwrap processing_ms [1m])
```
Distinguishes "waiting in Kafka queue" from "actual SMPP call time".

### Panel 4 — Stage Throughput (Time series)
```logql
sum by (service) (
  rate({service=~"pipeline-.*"} | json | event="completed" [1m])
)
```
If router throughput >> sender throughput → sender is the bottleneck.

### Panel 5 — Errors by Stage (Bar gauge)
```logql
sum by (component) (
  count_over_time({service=~"pipeline-.*"} | json | level="error" [5m])
)
```

### Panel 6 — Slow Messages Table (Table)
```logql
{service=~"pipeline-sender.*"} | json | kafka_wait_ms > 5000
  | line_format "{{.trace_id}}\t{{.kafka_wait_ms}}ms\t{{.processing_ms}}ms"
```
Top messages with highest Kafka wait time, with clickable trace_id.

---

## Files Summary

| Action | File |
|--------|------|
| Create | `deployments/configs/loki/loki.yml` |
| Create | `deployments/configs/promtail/promtail.yml` |
| Create | `deployments/configs/grafana/provisioning/datasources/loki.yml` |
| Create | `deployments/configs/grafana/dashboards/pipeline-bottleneck.json` |
| Modify | `deployments/docker-compose.yml` — add loki + promtail services + loki_data volume |
| Modify | `internal/api/http/handlers.go` — 2 lines: set kafkaMsg.TraceID |
| Modify | `internal/pipeline/router/stage.go` — add kafka_wait_ms to 2 log events |
| Modify | `internal/pipeline/sender/stage.go` — add kafka_wait_ms + processing_ms to send event |
| Modify | `internal/pipeline/status/stage.go` — add kafka_wait_ms to upserted event |

---

## Out of Scope

- OpenTelemetry distributed tracing (spans, waterfall view)
- Log-based alerting (Grafana alerts on Loki queries)
- Admin Gateway / Portal Gateway log enrichment
- gRPC request tracing
- Log retention beyond 7 days
