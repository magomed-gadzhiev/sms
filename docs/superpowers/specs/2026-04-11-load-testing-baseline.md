# Load Testing Baseline Strategy

**Date:** 2026-04-11  
**Status:** Approved  
**Approach:** k6 + Prometheus remote write + Grafana

---

## Goal

Establish performance baselines for the SMS gateway pipeline and detect regressions on manual pre-release runs. Not CI/CD automated — triggered by developer before deploy.

**Success criteria:**
- A single command runs the full baseline test suite
- Results appear in Grafana with run tagging
- Subsequent runs can be compared against a pinned baseline run
- Regression = any threshold breach vs established baseline values

---

## Architecture

```
[k6 on sms-server]
    │
    ├── HTTP → Client Gateway :8080 (via HAProxy)
    │         POST /api/v1/sms/send
    │         POST /api/v1/sms/batch
    │         GET  /api/v1/sms/status/{id}   ← polling until DLR
    │
    └── --out experimental-prometheus-rw
              → Prometheus :9091
                    └── Grafana :3001
                          └── load-test-baseline dashboard
                                ├── current run (by run_id)
                                └── baseline run (overlay)
```

Every run is tagged:
- `run_id` — `YYYY-MM-DD-HHmm` (unique per run)
- `run_type` — `baseline` or `regression`
- `git_sha` — short commit hash at time of run

---

## Test Scenarios

Three scenarios executed by one script in sequence. Smoke must pass before pipeline-baseline starts.

### 1. Smoke (always runs first)

| Parameter | Value |
|-----------|-------|
| Duration | 2 min |
| VUs | 10 |
| Purpose | Verify system is alive before load run |
| On failure | Abort — do not proceed to pipeline-baseline |

Mix: 80% `sendSMS`, 20% `healthCheck`.

### 2. Pipeline Baseline (main scenario)

| Phase | Duration | VUs |
|-------|----------|-----|
| Ramp up | 2 min | 0 → 200 |
| Ramp up | 3 min | 200 → 500 |
| Plateau | 5 min | 500 |
| Ramp down | 2 min | 500 → 0 |
| **Total** | **12 min** | |

Request mix:
- **70%** `sendSMS` — POST /api/v1/sms/send, records `message_id` for polling
- **15%** `sendBatch` — POST /api/v1/sms/batch (5–20 messages)
- **15%** `pollStatus` — GET /api/v1/sms/status/{id} on previously sent messages, measures e2e latency until `delivered` or 30s timeout

The `pollStatus` function is the key new behaviour vs old scripts: it tracks specific `message_id` values returned from `sendSMS` and measures wall-clock time from send to DLR, giving true end-to-end pipeline latency.

### 3. Soak (optional)

| Parameter | Value |
|-----------|-------|
| Duration | 30 min |
| VUs | 200 |
| Trigger | `RUN_SOAK=1` env var |
| Purpose | Detect memory leaks and gradual degradation |

---

## Metrics

### Custom k6 metrics

| Metric | Type | Description |
|--------|------|-------------|
| `sms_e2e_latency` | Trend | Time from send to DLR (ms) |
| `sms_send_latency` | Trend | HTTP POST /sms/send duration |
| `sms_pipeline_errors` | Rate | Messages without DLR within 30s timeout |
| `sms_sent_total` | Counter | Total SMS sent |
| `sms_batch_total` | Counter | Total SMS sent via batch |

All metrics carry `run_id`, `run_type`, `git_sha` tags via Prometheus labels.

### Thresholds

Thresholds are **not fixed** until after the first baseline run. Workflow:

1. Run first baseline with no thresholds (just collect data)
2. Read p95 values from Grafana
3. Set thresholds = p95 + 20% headroom
4. Commit thresholds to script

**Expected thresholds (to be confirmed after first run):**

```
http_req_duration{name="SendSMS"}:  p95 < 500ms, p99 < 1000ms
sms_e2e_latency:                    p95 < 5000ms
http_req_failed:                    rate < 1%
sms_pipeline_errors:                rate < 2%
```

---

## Grafana Dashboard

New dashboard: `configs/grafana/dashboards/load-test-baseline.json`

Panels:
1. **Throughput** — req/s, two lines: current run vs selected baseline `run_id`
2. **Send Latency** — p50/p95/p99 with baseline overlay
3. **E2E Pipeline Latency** — p50/p95/p99 (send → DLR)
4. **Error Rate** — `http_req_failed` + `sms_pipeline_errors`
5. **VU Count** — virtual users over time
6. **Regression Table** — metric / baseline value / current value / delta % / pass/fail

Dashboard variables: `$run_id` (current), `$baseline_run_id` (reference). Both are dropdown selectors populated from `run_id` label values in Prometheus.

---

## Infrastructure Changes

### Prometheus — enable remote write receiver

In `docker-compose.yml`, add flag to Prometheus command:

```yaml
command:
  - '--web.enable-remote-write-receiver'
  # ... existing flags
```

No changes to `configs/prometheus.yml` needed — remote write receiver uses the existing Prometheus storage.

### k6 — Prometheus remote write output

Run with:
```bash
k6 run \
  --out experimental-prometheus-rw \
  -e K6_PROMETHEUS_RW_SERVER_URL=http://localhost:9091/api/v1/write \
  -e K6_PROMETHEUS_RW_TREND_STATS=p50,p95,p99 \
  scripts/k6_baseline_test.js
```

k6 version must be >= 0.42.0 (supports `experimental-prometheus-rw`).

---

## Files

### Created

| File | Purpose |
|------|---------|
| `scripts/k6_baseline_test.js` | Main test script (all 3 scenarios) |
| `scripts/run-baseline.sh` | Runner: validates env, tags run, executes k6 |
| `configs/grafana/dashboards/load-test-baseline.json` | Grafana baseline dashboard |

### Modified

| File | Change |
|------|--------|
| `docker-compose.yml` | Add `--web.enable-remote-write-receiver` to Prometheus |

### Deleted (replaced by new scripts)

| File | Replaced by |
|------|-------------|
| `scripts/k6_load_test.js` | `scripts/k6_baseline_test.js` |
| `scripts/k6_10k_load_test.js` | `scripts/k6_baseline_test.js` |
| `scripts/k6_quick_test.js` | smoke scenario in `k6_baseline_test.js` |
| `scripts/load-test.sh` | `scripts/run-baseline.sh` |

`scripts/k6_dev_test.js` is kept unchanged.

---

## Usage

```bash
# First baseline run (establishes reference)
RUN_TYPE=baseline ./scripts/run-baseline.sh

# Regression check before deploy
RUN_TYPE=regression ./scripts/run-baseline.sh

# With soak test
RUN_TYPE=baseline RUN_SOAK=1 ./scripts/run-baseline.sh
```

After each run, open Grafana at `http://72.56.232.202:3001`, select the new dashboard, pick `$run_id` and `$baseline_run_id` to compare.

---

## Out of Scope

- Automated CI/CD triggers
- Admin Gateway / Portal Gateway load testing
- gRPC load testing
- Database-level load (pgbench)
- Provider/SMPP layer stress testing
