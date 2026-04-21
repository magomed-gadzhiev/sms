# Routing/sandbox behavioural contract change + weighted-pick race fix

Date: 2026-04-22
Branch: fix/network-stats-pipeline (follow-up to QA bugs #10/#11/#12)

## Behavioural contract change — sandbox clients

**Before:** messages from clients with `is_sandbox = TRUE` short-circuited
inside the messaging/routing layer and the HTTP/gRPC response carried
`status = "delivered"` synchronously. No provider interaction, no pipeline
entry, no delivery receipt lifecycle.

**After:** sandbox clients now traverse the normal pipeline. The initial
response is `status = "queued"`. Delivery is reported later via the usual
DLR / webhook channels, driven by the configured route (which in the current
fixture data points to `LoadTest-Simulator` for every sandbox client — see
audit below).

**Impact on SDK consumers:**
- Clients that asserted on `"delivered"` in the synchronous response for
  sandbox traffic must switch to awaiting the DLR / polling `/messages/{id}`.
- Synchronous latency for sandbox calls is now pipeline-normal (a few ms of
  Kafka round-trip) instead of ~0.

## Sandbox route-coverage audit (pre-deploy)

Query run against prod DB (72.56.232.202, `smpp_db`):

| client | account_type | own routes | parent routes | platform fallback |
|---|---|---|---|---|
| LoadTest-LowVolume | direct | 0 | 0 | 2 |
| LoadTest-Default | direct | 0 | 0 | 2 |
| LoadTest-MedVolume | direct | 0 | 0 | 2 |
| Test Client | direct | 0 | 0 | 2 |
| Lanser Test | direct | 0 | 0 | 2 |
| Test Heavy (C) | sub_account | 3 | 2 | 2 |
| Test Problem (B) | sub_account | 2 | 2 | 2 |
| Test Clean (A) | sub_account | 1 | 2 | 2 |

Total sandbox clients: 8. All covered by at least one active route
(own/parent/platform). Platform fallbacks point at provider
`LoadTest-Simulator` (safe — no real network egress). **No mitigation SQL
required.**

## Race fix — `internal/services/routing/application/weighted_pick.go`

`math/rand.Rand` is not goroutine-safe. The previous implementation captured
`pickRNG` under `pickRNGMu`, released the mutex, and called `r.Intn` outside
it. Two concurrent pipeline-router workers could corrupt RNG state or panic
under `-race`.

**Fix:** refactored `PickWeightedRoute` and `PickWeightedRouteWithRand` to
share a private `pickWeightedRouteWithRoller(routes, roller func(int) int)`.
The public `PickWeightedRoute` injects a closure that holds `pickRNGMu` for
the duration of `pickRNG.Intn`. `PickWeightedRouteWithRand` passes
`r.Intn` directly (caller owns synchronization).

`internal/router/unified.go` `pickWeightedSharedRoute` was already correct —
it rolls Intn inline under the mutex. No change needed there.

Regression test added: `TestPickWeightedRoute_Concurrent_NoRace` — 100
goroutines × 1000 iterations, passes under `go test -race`.

## Deploy steps

Services that import the patched packages (routing picker + matcher seed):
- `routing-service`
- `pipeline-router` (if a separate container) — otherwise covered by the
  routing-service rebuild

Order:
1. `./scripts/server.sh deploy routing-service`
2. Verify with a sandbox-client send: response is `status=queued`, DLR
   arrives from `LoadTest-Simulator` within a few seconds.
