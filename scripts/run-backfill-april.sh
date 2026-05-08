#!/usr/bin/env bash
# Backfill network_stats_hourly from 2026-04-01 to now.
# Usage (from sandbox shell): DATABASE_URL=... scripts/run-backfill-april.sh
set -euo pipefail
exec /opt/sms/bin/network-stats-backfill \
  --from 2026-04-01T00:00:00Z \
  --to now \
  --truncate \
  --yes
