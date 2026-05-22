#!/usr/bin/env bash
# Backfill network_stats_hourly from 2026-04-01 to now. Run on sandbox after
# Slice 1 deploy. Truncates the window first; values will be repopulated by
# the aggregator with revenue from tarification_log.
#
# Usage (from sandbox shell):
#   DATABASE_URL=postgres://smpp:smpp@postgres:5432/smpp_db?sslmode=disable \
#     scripts/run-backfill-april.sh
set -euo pipefail
exec /opt/sms/bin/network-stats-backfill \
  --from 2026-04-01T00:00:00Z \
  --to now \
  --truncate \
  --yes
