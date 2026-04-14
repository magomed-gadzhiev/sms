# Pipeline Bottleneck Logging — Grafana Loki Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Grafana Loki log aggregation + trace_id propagation + per-stage timing fields so pipeline bottlenecks are visible in Grafana.

**Architecture:** Loki + Promtail collect Docker container logs (already JSON via zerolog). Two one-line fixes propagate trace_id from HTTP through Kafka. Three pipeline stages get `kafka_wait_ms` / `processing_ms` fields added to existing log events. A new Grafana dashboard queries Loki for bottleneck analysis.

**Tech Stack:** Grafana Loki 3.0.0, Promtail 3.0.0, Go 1.24 (zerolog), Docker Compose, Grafana (existing, port 3001)

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `deployments/configs/loki/loki.yml` | Loki server config (storage, retention) |
| Create | `deployments/configs/promtail/promtail.yml` | Log scraping from Docker socket + JSON parsing |
| Create | `deployments/configs/grafana/provisioning/datasources/loki.yml` | Grafana datasource for Loki |
| Create | `deployments/configs/grafana/dashboards/pipeline-bottleneck.json` | Grafana dashboard (6 panels) |
| Modify | `deployments/docker-compose.yml` | Add loki + promtail services + loki_data volume |
| Modify | `internal/api/http/handlers.go` | Set kafkaMsg.TraceID from HTTP request_id (2 places) |
| Modify | `internal/pipeline/router/stage.go` | Add kafka_wait_ms to route_matched + no_route logs |
| Modify | `internal/pipeline/sender/stage.go` | Add kafka_wait_ms + processing_ms to send completed log |
| Modify | `internal/pipeline/status/stage.go` | Add kafka_wait_ms to upserted log |

---

## Task 1: Loki config file

**Files:**
- Create: `deployments/configs/loki/loki.yml`

- [ ] **Step 1: Create loki config directory and file**

```bash
mkdir -p deployments/configs/loki
```

Create `deployments/configs/loki/loki.yml`:

```yaml
auth_enabled: false

server:
  http_listen_port: 3100
  grpc_listen_port: 9096

common:
  instance_addr: 127.0.0.1
  path_prefix: /loki
  storage:
    filesystem:
      chunks_directory: /loki/chunks
      rules_directory: /loki/rules
  replication_factor: 1
  ring:
    kvstore:
      store: inmemory

query_range:
  results_cache:
    cache:
      embedded_cache:
        enabled: true
        max_size_mb: 100

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
  retention_period: 168h
  ingestion_rate_mb: 16
  ingestion_burst_size_mb: 32
  max_query_series: 5000
  allow_structured_metadata: true

compactor:
  working_directory: /loki/compactor
  retention_enabled: true
  delete_request_store: filesystem
```

- [ ] **Step 2: Commit**

```bash
git add deployments/configs/loki/loki.yml
git commit -m "feat(logging): add Loki config"
```

---

## Task 2: Promtail config file

**Files:**
- Create: `deployments/configs/promtail/promtail.yml`

- [ ] **Step 1: Create promtail config directory and file**

```bash
mkdir -p deployments/configs/promtail
```

Create `deployments/configs/promtail/promtail.yml`:

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
        filters:
          - name: status
            values: ["running"]
    relabel_configs:
      # Strip leading "/" from container name
      - source_labels: [__meta_docker_container_name]
        regex: '/?(.+)'
        target_label: container
      # Derive service name: strip "deployments-" prefix and trailing "-N" replica suffix
      - source_labels: [__meta_docker_container_name]
        regex: '/?(?:deployments-)?(.*?)(?:-\d+)?$'
        target_label: service
        replacement: '${1}'
    pipeline_stages:
      # Parse the JSON log line
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
      # Promote level and component to Loki stream labels for filtering
      - labels:
          level:
          component:
      # Drop debug logs to reduce storage (pipeline stages log at info for key events)
      - match:
          selector: '{level="debug"}'
          action: drop
```

- [ ] **Step 2: Verify regex produces correct service names**

Mental check on the regex `(?:deployments-)?(.*?)(?:-\d+)?$`:
- `deployments-pipeline-sender-1` → `pipeline-sender` ✅
- `deployments-pipeline-router-2` → `pipeline-router` ✅
- `postgres` → `postgres` ✅
- `grafana` → `grafana` ✅

- [ ] **Step 3: Commit**

```bash
git add deployments/configs/promtail/promtail.yml
git commit -m "feat(logging): add Promtail config for Docker log scraping"
```

---

## Task 3: Add Loki + Promtail to docker-compose.yml

**Files:**
- Modify: `deployments/docker-compose.yml`

The Grafana service is around line 1222. The volumes section is around line 90. Add loki and promtail services after prometheus (line 1220), and add `loki_data` to the volumes section.

- [ ] **Step 1: Add loki service after the prometheus service block (after line 1220)**

Find this block in `deployments/docker-compose.yml`:
```yaml
    networks:
      - smpp-network
    restart: unless-stopped

  # Grafana
```

Replace with:
```yaml
    networks:
      - smpp-network
    restart: unless-stopped

  loki:
    image: grafana/loki:3.0.0
    container_name: loki
    ports:
      - "3100:3100"
    volumes:
      - ./configs/loki/loki.yml:/etc/loki/local-config.yaml
      - loki_data:/loki
    command: -config.file=/etc/loki/local-config.yaml
    networks:
      - smpp-network
    restart: unless-stopped

  promtail:
    image: grafana/promtail:3.0.0
    container_name: promtail
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./configs/promtail/promtail.yml:/etc/promtail/config.yml
    command: -config.file=/etc/promtail/config.yml
    networks:
      - smpp-network
    depends_on:
      - loki
    restart: unless-stopped

  # Grafana
```

- [ ] **Step 2: Add loki_data to the volumes section**

Find the volumes section (around line 90):
```yaml
volumes:
  postgres-data:
  redis-data:
  kafka-data:
  zookeeper-data:
  zookeeper-logs:
  prometheus-data:
  grafana-data:
  contact-uploads:
  directus-uploads:
  directus-extensions:
```

Replace with:
```yaml
volumes:
  postgres-data:
  redis-data:
  kafka-data:
  zookeeper-data:
  zookeeper-logs:
  prometheus-data:
  grafana-data:
  loki_data:
  contact-uploads:
  directus-uploads:
  directus-extensions:
```

- [ ] **Step 3: Add loki to grafana depends_on**

Find the grafana `depends_on` block:
```yaml
    depends_on:
      - prometheus
      - postgres
```

Replace with:
```yaml
    depends_on:
      - prometheus
      - postgres
      - loki
```

- [ ] **Step 4: Validate docker-compose syntax**

```bash
docker compose -f deployments/docker-compose.yml config --quiet
```

Expected: no output (silent success). If errors appear, fix YAML indentation.

- [ ] **Step 5: Commit**

```bash
git add deployments/docker-compose.yml
git commit -m "feat(logging): add Loki and Promtail to docker-compose"
```

---

## Task 4: Grafana Loki datasource

**Files:**
- Create: `deployments/configs/grafana/provisioning/datasources/loki.yml`

The datasources directory already exists and Grafana loads all YAML files from it. Add a new file for Loki without touching the existing datasources.yml.

- [ ] **Step 1: Create the datasource file**

Create `deployments/configs/grafana/provisioning/datasources/loki.yml`:

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
          url: '/explore?orgId=1&left={"datasource":"loki","queries":[{"expr":"{service=~\"pipeline-.*\"} | json | trace_id=\"${__value.raw}\"","refId":"A"}],"range":{"from":"now-1h","to":"now"}}'
          urlDisplayLabel: "Trace pipeline"
```

The `derivedFields` entry makes `trace_id` a clickable link in log lines — clicking it opens an Explore view filtered to that trace_id across all pipeline stages.

- [ ] **Step 2: Commit**

```bash
git add deployments/configs/grafana/provisioning/datasources/loki.yml
git commit -m "feat(logging): add Loki datasource to Grafana"
```

---

## Task 5: Trace ID propagation in HTTP handlers

**Files:**
- Modify: `internal/api/http/handlers.go`

The handlers call `queue.FromMessage(msg)` but never populate `TraceID`. The request_id is already in context (set by `LoggingMiddleware`). Two identical one-line additions needed.

- [ ] **Step 1: Find the exact context variable name in handlers**

Read `internal/api/http/handlers.go` around lines 50-130 to confirm whether the handler uses `ctx := r.Context()` or calls `r.Context()` directly inline.

If the handler has `ctx := r.Context()` declared, use `shared.GetRequestID(ctx)`.
If not, use `shared.GetRequestID(r.Context())`.

- [ ] **Step 2: Add TraceID to single send handler (around line 111)**

Find:
```go
kafkaMsg := queue.FromMessage(msg)
```
(in the single send handler, the first occurrence)

Replace with:
```go
kafkaMsg := queue.FromMessage(msg)
kafkaMsg.TraceID = shared.GetRequestID(r.Context())
```

- [ ] **Step 3: Add TraceID to batch send handler (around line 209)**

Find the second occurrence of `kafkaMsg := queue.FromMessage(msg)` (inside the batch handler's per-message loop).

Replace with:
```go
kafkaMsg := queue.FromMessage(msg)
kafkaMsg.TraceID = shared.GetRequestID(r.Context())
```

- [ ] **Step 4: Verify shared package is already imported**

Check that `"github.com/smpp-server/smpp-server/internal/shared"` is already in the import block of `handlers.go`. If it is, no import change needed. If not, add it to imports.

- [ ] **Step 5: Build to verify no compilation errors**

```bash
go build ./internal/api/http/...
```

Expected: no output (success).

- [ ] **Step 6: Commit**

```bash
git add internal/api/http/handlers.go
git commit -m "feat(logging): propagate HTTP request_id as trace_id into Kafka messages"
```

---

## Task 6: Router stage — add kafka_wait_ms

**Files:**
- Modify: `internal/pipeline/router/stage.go`

Two existing log events need `kafka_wait_ms` added: `route_matched` (line 204) and `no_route` (line 188). `kafkaMsg.CreatedAt` (type `time.Time`) is available at both locations.

- [ ] **Step 1: Add kafka_wait_ms to route_matched event (around line 204)**

Find:
```go
trace.Log(s.logger, kafkaMsg.TraceID, kafkaMsg.MessageID.String(), "router", "route_matched").
    Str("route_id", route.ID.String()).
    Str("route_name", route.Name).
    Str("provider_id", providerID.String()).
    Int("priority", route.Priority).
    Bool("used_default", result.UsedDefault).
    Int("total_matched", len(result.Matched)).
    Msg("route selected")
```

Replace with:
```go
trace.Log(s.logger, kafkaMsg.TraceID, kafkaMsg.MessageID.String(), "router", "route_matched").
    Str("route_id", route.ID.String()).
    Str("route_name", route.Name).
    Str("provider_id", providerID.String()).
    Int("priority", route.Priority).
    Bool("used_default", result.UsedDefault).
    Int("total_matched", len(result.Matched)).
    Int64("kafka_wait_ms", time.Since(kafkaMsg.CreatedAt).Milliseconds()).
    Msg("route selected")
```

- [ ] **Step 2: Add kafka_wait_ms to no_route event (around line 188)**

Find:
```go
trace.Warn(s.logger, kafkaMsg.TraceID, kafkaMsg.MessageID.String(), "router", "no_route").
    Str("operator_id", operatorID.String()).
    Str("traffic_type", string(trafficType)).
    Str("sender_name", kafkaMsg.Source).
    Int("client_routes_checked", result.ClientRoutes).
    Int("default_routes_checked", result.DefaultRoutes).
    Msg("no matching route found")
```

Replace with:
```go
trace.Warn(s.logger, kafkaMsg.TraceID, kafkaMsg.MessageID.String(), "router", "no_route").
    Str("operator_id", operatorID.String()).
    Str("traffic_type", string(trafficType)).
    Str("sender_name", kafkaMsg.Source).
    Int("client_routes_checked", result.ClientRoutes).
    Int("default_routes_checked", result.DefaultRoutes).
    Int64("kafka_wait_ms", time.Since(kafkaMsg.CreatedAt).Milliseconds()).
    Msg("no matching route found")
```

- [ ] **Step 3: Verify `time` package is already imported**

Check imports in `router/stage.go` — `"time"` should already be present (it's used for `time.Now()` in `RoutedAt` assignment). If not, add it.

- [ ] **Step 4: Build**

```bash
go build ./internal/pipeline/router/...
```

Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add internal/pipeline/router/stage.go
git commit -m "feat(logging): add kafka_wait_ms to router stage log events"
```

---

## Task 7: Sender stage — add kafka_wait_ms + processing_ms

**Files:**
- Modify: `internal/pipeline/sender/stage.go`

The "completed" log event is at line 538. `routedMsg.RoutedAt` (type `time.Time`) is available. Need to record `pickupAt` at the start of per-message processing to compute `processing_ms`.

- [ ] **Step 1: Read the processMessage (or equivalent) function signature**

Read `internal/pipeline/sender/stage.go` lines 260-290 to find where the per-message processing function starts (the function that contains the billing check at line ~298). Identify the first line of the function body.

- [ ] **Step 2: Add pickupAt at the start of message processing**

At the first line of the function body that processes a single `routedMsg` (before billing check, before backpressure), add:

```go
pickupAt := time.Now()
```

This records when the sender picked up the message from Kafka for processing.

- [ ] **Step 3: Add kafka_wait_ms + processing_ms to completed event (around line 538)**

Find:
```go
trace.Log(s.logger, traceID, routedMsg.MessageID.String(), "sender.send", "completed").
    Str("provider_id", routedMsg.ProviderID.String()).
    Str("smpp_message_id", sentMsg.SMPPMessageID).
    Str("status", sentMsg.Status).
    Str("connection_id", usedConnID).
    Str("sender_type", senderType).
    Int("segments", sentMsg.SegmentsCount).
    Msg("message processed by sender")
```

Replace with:
```go
trace.Log(s.logger, traceID, routedMsg.MessageID.String(), "sender.send", "completed").
    Str("provider_id", routedMsg.ProviderID.String()).
    Str("smpp_message_id", sentMsg.SMPPMessageID).
    Str("status", sentMsg.Status).
    Str("connection_id", usedConnID).
    Str("sender_type", senderType).
    Int("segments", sentMsg.SegmentsCount).
    Int64("kafka_wait_ms", pickupAt.Sub(routedMsg.RoutedAt).Milliseconds()).
    Int64("processing_ms", time.Since(pickupAt).Milliseconds()).
    Msg("message processed by sender")
```

- [ ] **Step 4: Build**

```bash
go build ./internal/pipeline/sender/...
```

Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add internal/pipeline/sender/stage.go
git commit -m "feat(logging): add kafka_wait_ms and processing_ms to sender stage"
```

---

## Task 8: Status stage — add kafka_wait_ms

**Files:**
- Modify: `internal/pipeline/status/stage.go`

The "upserted" event is at lines 160-163, inside a loop over `records`. Each `r` is a `statusRecord`. Need to check if `statusRecord` has `SentAt time.Time` — it may not since it aggregates data from both `sms.sent` and `sms.dlr` topics.

- [ ] **Step 1: Read statusRecord struct**

Read `internal/pipeline/status/stage.go` lines 1-100 (or search for `type statusRecord`) to find the struct definition.

- [ ] **Step 2a: If statusRecord has SentAt field** — add kafka_wait_ms

Find (lines 160-163):
```go
trace.Log(s.logger, r.TraceID, r.MessageID.String(), "status", "upserted").
    Str("status", r.Status).
    Str("smpp_message_id", r.SMPPMessageID).
    Msg("status written to DB")
```

Replace with:
```go
trace.Log(s.logger, r.TraceID, r.MessageID.String(), "status", "upserted").
    Str("status", r.Status).
    Str("smpp_message_id", r.SMPPMessageID).
    Int64("kafka_wait_ms", time.Since(r.SentAt).Milliseconds()).
    Msg("status written to DB")
```

- [ ] **Step 2b: If statusRecord does NOT have SentAt** — add it from SentMessage deserialization

First, add field to statusRecord struct:
```go
SentAt time.Time `json:"sent_at,omitempty"`
```

Then in `deserializeMessage`, when processing from `sms.sent` topic, populate it:
```go
// After deserializing sentMsg from sms.sent topic:
rec.SentAt = sentMsg.SentAt
```

Then apply the log change from Step 2a.

- [ ] **Step 3: Build**

```bash
go build ./internal/pipeline/status/...
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add internal/pipeline/status/stage.go
git commit -m "feat(logging): add kafka_wait_ms to status stage upserted event"
```

---

## Task 9: Grafana Pipeline Bottleneck dashboard

**Files:**
- Create: `deployments/configs/grafana/dashboards/pipeline-bottleneck.json`

The dashboard uses Loki as datasource (uid: `loki`). It has 6 panels. The JSON below is a complete, valid Grafana dashboard definition.

- [ ] **Step 1: Create the dashboard JSON**

Create `deployments/configs/grafana/dashboards/pipeline-bottleneck.json`:

```json
{
  "title": "Pipeline Bottleneck",
  "uid": "pipeline-bottleneck",
  "schemaVersion": 39,
  "version": 1,
  "refresh": "30s",
  "time": { "from": "now-1h", "to": "now" },
  "templating": {
    "list": [
      {
        "name": "trace_id",
        "type": "textbox",
        "label": "Trace ID",
        "current": { "value": "" },
        "hide": 0
      }
    ]
  },
  "panels": [
    {
      "id": 1,
      "title": "Log Explorer (filter by Trace ID)",
      "type": "logs",
      "gridPos": { "h": 8, "w": 24, "x": 0, "y": 0 },
      "datasource": { "type": "loki", "uid": "loki" },
      "targets": [
        {
          "expr": "{service=~\"pipeline-.*\"} | json | trace_id=~\"${trace_id:pipe}\"",
          "refId": "A"
        }
      ],
      "options": {
        "showTime": true,
        "sortOrder": "Ascending",
        "wrapLogMessage": false,
        "dedupStrategy": "none"
      }
    },
    {
      "id": 2,
      "title": "Kafka Wait by Stage (avg ms, 1m)",
      "type": "timeseries",
      "gridPos": { "h": 8, "w": 12, "x": 0, "y": 8 },
      "datasource": { "type": "loki", "uid": "loki" },
      "targets": [
        {
          "expr": "avg_over_time({service=\"pipeline-router\"} | json | kafka_wait_ms != \"\" | unwrap kafka_wait_ms [1m])",
          "refId": "A",
          "legendFormat": "router"
        },
        {
          "expr": "avg_over_time({service=\"pipeline-sender\"} | json | kafka_wait_ms != \"\" | unwrap kafka_wait_ms [1m])",
          "refId": "B",
          "legendFormat": "sender"
        },
        {
          "expr": "avg_over_time({service=\"pipeline-status\"} | json | kafka_wait_ms != \"\" | unwrap kafka_wait_ms [1m])",
          "refId": "C",
          "legendFormat": "status"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "ms",
          "custom": { "lineWidth": 2 }
        }
      }
    },
    {
      "id": 3,
      "title": "Sender: Wait vs Processing Time (avg ms, 1m)",
      "type": "timeseries",
      "gridPos": { "h": 8, "w": 12, "x": 12, "y": 8 },
      "datasource": { "type": "loki", "uid": "loki" },
      "targets": [
        {
          "expr": "avg_over_time({service=\"pipeline-sender\"} | json | kafka_wait_ms != \"\" | unwrap kafka_wait_ms [1m])",
          "refId": "A",
          "legendFormat": "kafka_wait (queue backlog)"
        },
        {
          "expr": "avg_over_time({service=\"pipeline-sender\"} | json | processing_ms != \"\" | unwrap processing_ms [1m])",
          "refId": "B",
          "legendFormat": "processing (SMPP call)"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "ms",
          "custom": { "lineWidth": 2 }
        }
      }
    },
    {
      "id": 4,
      "title": "Stage Throughput (completed events/s)",
      "type": "timeseries",
      "gridPos": { "h": 8, "w": 12, "x": 0, "y": 16 },
      "datasource": { "type": "loki", "uid": "loki" },
      "targets": [
        {
          "expr": "sum by (service) (rate({service=~\"pipeline-.*\"} | json | event=\"route_matched\" [1m]))",
          "refId": "A",
          "legendFormat": "router ({{service}})"
        },
        {
          "expr": "sum by (service) (rate({service=~\"pipeline-sender.*\"} | json | event=\"completed\" [1m]))",
          "refId": "B",
          "legendFormat": "sender ({{service}})"
        },
        {
          "expr": "sum by (service) (rate({service=~\"pipeline-status.*\"} | json | event=\"upserted\" [1m]))",
          "refId": "C",
          "legendFormat": "status ({{service}})"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "reqps",
          "custom": { "lineWidth": 2 }
        }
      }
    },
    {
      "id": 5,
      "title": "Errors by Stage (5m window)",
      "type": "bargauge",
      "gridPos": { "h": 8, "w": 12, "x": 12, "y": 16 },
      "datasource": { "type": "loki", "uid": "loki" },
      "targets": [
        {
          "expr": "sum by (component) (count_over_time({service=~\"pipeline-.*\"} | json | level=\"error\" [5m]))",
          "refId": "A",
          "legendFormat": "{{component}}"
        }
      ],
      "options": {
        "orientation": "horizontal",
        "reduceOptions": { "calcs": ["lastNotNull"] }
      },
      "fieldConfig": {
        "defaults": { "unit": "short", "color": { "mode": "fixed", "fixedColor": "red" } }
      }
    },
    {
      "id": 6,
      "title": "Slow Messages — sender kafka_wait > 5s",
      "type": "logs",
      "gridPos": { "h": 8, "w": 24, "x": 0, "y": 24 },
      "datasource": { "type": "loki", "uid": "loki" },
      "targets": [
        {
          "expr": "{service=\"pipeline-sender\"} | json | kafka_wait_ms > 5000 | line_format \"trace={{.trace_id}} wait={{.kafka_wait_ms}}ms proc={{.processing_ms}}ms msg={{.message_id}}\"",
          "refId": "A"
        }
      ],
      "options": {
        "showTime": true,
        "sortOrder": "Descending",
        "wrapLogMessage": false
      }
    }
  ]
}
```

- [ ] **Step 2: Verify dashboard JSON is valid**

```bash
python3 -c "import json; json.load(open('deployments/configs/grafana/dashboards/pipeline-bottleneck.json')); print('valid JSON')"
```

Expected: `valid JSON`

- [ ] **Step 3: Commit**

```bash
git add deployments/configs/grafana/dashboards/pipeline-bottleneck.json
git commit -m "feat(logging): add Pipeline Bottleneck Grafana dashboard"
```

---

## Task 10: Deploy and verify on server

- [ ] **Step 1: Push to GitHub**

```bash
git push origin master
```

- [ ] **Step 2: Pull and start Loki + Promtail on server**

```bash
bash scripts/server.sh exec "cd /opt/sms && git pull && docker compose -f deployments/docker-compose.yml up -d loki promtail"
```

Expected output includes:
```
Container loki  Started
Container promtail  Started
```

- [ ] **Step 3: Verify Loki is ready**

```bash
bash scripts/server.sh exec "curl -s http://localhost:3100/ready"
```

Expected: `ready`

- [ ] **Step 4: Verify Promtail is scraping logs**

```bash
bash scripts/server.sh exec "curl -s http://localhost:9080/metrics | grep promtail_files_active_total"
```

Expected: a line like `promtail_files_active_total 42` (nonzero number).

- [ ] **Step 5: Restart Grafana to pick up new datasource and dashboard**

```bash
bash scripts/server.sh exec "docker restart grafana"
```

- [ ] **Step 6: Verify Loki datasource in Grafana**

```bash
bash scripts/server.sh exec "curl -s -u admin:admin http://localhost:3001/api/datasources | python3 -c \"import json,sys; ds=json.load(sys.stdin); print([d['name'] for d in ds])\""
```

Expected: list includes `'Loki'`.

- [ ] **Step 7: Send a test SMS and verify trace_id appears in Loki**

```bash
# Send a message and capture its trace from request ID header
bash scripts/server.sh exec "curl -s -X POST http://localhost:8080/api/v1/sms/send \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: load-test-probe' \
  -H 'X-Request-ID: test-trace-001' \
  -d '{\"source\":\"Test\",\"destination\":\"79001234567\",\"text\":\"trace test\",\"registered_delivery\":true}'"
```

Expected: `{"status":"queued","message_id":"...","...": ...}`

Wait 5 seconds, then query Loki:

```bash
bash scripts/server.sh exec "curl -s 'http://localhost:3100/loki/api/v1/query_range?query=%7Bservice%3D~%22pipeline-.*%22%7D%20%7C%20json%20%7C%20trace_id%3D%22test-trace-001%22&limit=20&start=$(date -d '1 minute ago' +%s)000000000&end=$(date +%s)000000000' | python3 -c \"import json,sys; r=json.load(sys.stdin); results=r.get('data',{}).get('result',[]); print(f'Found {len(results)} streams'); [print(v[1][:120]) for s in results for v in s.get('values',[])]\"" 2>/dev/null
```

Expected: 2–4 log lines containing `trace_id=test-trace-001`, one per pipeline stage (router route_matched, sender completed, status upserted). Also verify `kafka_wait_ms` field appears in the sender log line.

- [ ] **Step 8: Open Grafana Pipeline Bottleneck dashboard**

Open in browser: `http://72.56.232.202:3001/d/pipeline-bottleneck`

Verify:
- Dashboard loads without errors
- Log Explorer panel shows logs
- Enter `test-trace-001` in the Trace ID variable and verify filtered logs appear
- "Slow Messages" panel shows any sender messages with wait > 5s

- [ ] **Step 9: Commit deploy notes (optional)**

No code changes at this step — deploy is complete.

---

## Self-Review

**Spec coverage:**
- ✅ Loki config (Task 1)
- ✅ Promtail config (Task 2)
- ✅ docker-compose (Task 3)
- ✅ Grafana datasource (Task 4)
- ✅ Trace ID propagation (Task 5)
- ✅ Router kafka_wait_ms (Task 6)
- ✅ Sender kafka_wait_ms + processing_ms (Task 7)
- ✅ Status kafka_wait_ms (Task 8 — conditional on statusRecord having SentAt)
- ✅ Dashboard with 6 panels (Task 9)
- ✅ Deploy verification (Task 10)

**Placeholder scan:** All steps have exact code. No TBD. Task 8 Step 2a/2b handle the conditional case explicitly. ✅

**Type consistency:** `kafka_wait_ms` is `Int64` (zerolog) in all three stages. `time.Since(...).Milliseconds()` returns `int64`. ✅ `pickupAt.Sub(routedMsg.RoutedAt).Milliseconds()` also `int64`. ✅
