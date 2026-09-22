#!/usr/bin/env bash
# Demo stand seeding (architecture review, candidate 5).
#
# Applies test/load/fixtures/demo_*.sql in order — cleanup → seed → data —
# with preconditions (DB reachable, migrations applied) and postconditions
# (no running campaigns, balance chain converges). Replaces the raw psql
# one-liners that used to live directly in the Makefile.
#
# WARNING: demo_seed.sql OVERWRITES the portal admin password to Admin123!
# Never run against a stand with real users. (Documented in test/load/README.md.)
#
# Usage: scripts/demo-seed.sh [--clean]
#   --clean  apply only demo_cleanup.sql
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FIXTURES="$REPO_ROOT/test/load/fixtures"
CLEAN_ONLY=0
[ "${1:-}" = "--clean" ] && CLEAN_ONLY=1

# Local stand defaults (deployments/docker-compose.yml). Override via env.
PSQL_CONTAINER="${DEMO_PSQL_CONTAINER:-postgres}"
PSQL_USER="${DEMO_PSQL_USER:-smpp}"
PSQL_DB="${DEMO_PSQL_DB:-smpp_db}"

psql_exec() {
  docker exec -i "$PSQL_CONTAINER" psql -U "$PSQL_USER" -d "$PSQL_DB" -v ON_ERROR_STOP=1 "$@"
}

psql_scalar() {
  docker exec -i "$PSQL_CONTAINER" psql -U "$PSQL_USER" -d "$PSQL_DB" -tAc "$1"
}

echo "==> demo-seed: container=$PSQL_CONTAINER db=$PSQL_DB"

# ── Preconditions ────────────────────────────────────────────────────────────
echo "==> precondition: database reachable"
psql_scalar "SELECT 1" > /dev/null

echo "==> precondition: migrations applied (schema_migrations non-empty)"
MIGRATIONS=$(psql_scalar "SELECT count(*) FROM schema_migrations")
if [ "$MIGRATIONS" -lt 1 ] 2>/dev/null; then
  echo "ERROR: schema_migrations is empty — apply migrations before seeding." >&2
  exit 1
fi

for f in demo_cleanup.sql demo_seed.sql demo_data.sql; do
  [ -f "$FIXTURES/$f" ] || { echo "ERROR: fixture $FIXTURES/$f not found" >&2; exit 1; }
done

# ── Apply fixtures ───────────────────────────────────────────────────────────
apply() {
  echo "==> applying $1"
  psql_exec -f - < "$FIXTURES/$1"
}

apply demo_cleanup.sql
if [ "$CLEAN_ONLY" -eq 1 ]; then
  echo "==> demo-seed: clean-only done"
  exit 0
fi
apply demo_seed.sql
apply demo_data.sql

# ── Postconditions ───────────────────────────────────────────────────────────
echo "==> postcondition: no campaign left in 'running' (the campaign worker would really send)"
RUNNING=$(psql_scalar "SELECT count(*) FROM campaigns WHERE status = 'running'")
if [ "$RUNNING" != "0" ]; then
  echo "ERROR: $RUNNING campaign(s) in status 'running' after seed — expected 'paused' demo data." >&2
  exit 1
fi

echo "==> postcondition: balance chain converges (account.balance == last transaction balance_after)"
DIVERGED=$(psql_scalar "
  SELECT count(*) FROM accounts a
  WHERE EXISTS (SELECT 1 FROM transactions t WHERE t.client_id = a.client_id)
    AND a.balance::numeric <> (
      SELECT t.balance_after::numeric FROM transactions t
      WHERE t.client_id = a.client_id
      ORDER BY t.created_at DESC, t.id DESC LIMIT 1
    )")
if [ "$DIVERGED" != "0" ]; then
  echo "ERROR: $DIVERGED account(s) diverge from their transaction chain." >&2
  exit 1
fi

echo "==> postcondition: demo admin present"
ADMIN=$(psql_scalar "SELECT count(*) FROM users WHERE username = 'admin'")
if [ "$ADMIN" != "1" ]; then
  echo "ERROR: demo admin user not found after seed." >&2
  exit 1
fi

echo ""
echo "demo-seed: OK (admin password is now Admin123! — see test/load/README.md)"
