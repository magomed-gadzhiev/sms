#!/usr/bin/env bash
# Quality gates: run all static checks locally and in CI.
# Pre-commit hook calls this without --with-tests (fast mode).
# CI calls this with --with-tests (full mode).

set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

WITH_TESTS=0
for arg in "$@"; do
    [ "$arg" = "--with-tests" ] && WITH_TESTS=1
done

fail=0
step() {
    local name="$1"
    shift
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "[CHECK] $name"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    if "$@"; then
        echo "[OK] $name"
    else
        echo "[FAIL] $name" >&2
        fail=1
    fi
}

# === Go ===

# Check if go is actually executable — some Windows environments (Device Guard / WDAC
# running under Claude Code CLI) may block go.exe. In that case, skip Go locally
# and rely on CI (GitHub Actions runs on Linux, no Device Guard restrictions).
if command -v go >/dev/null 2>&1 && go version >/dev/null 2>&1; then
    step "go vet"   go vet ./...
    step "go build" go build ./...

    if [ "$WITH_TESTS" = "1" ]; then
        step "go test" go test -count=1 -short ./...
    fi
else
    echo ""
    echo "[SKIP] Go checks — 'go' not runnable in this environment."
    echo "       This is common on Windows with Device Guard/WDAC under restricted processes."
    echo "       Run from a regular terminal for local Go validation, or rely on CI."
fi

# === Frontend (portal-frontend) ===

if [ -d portal-frontend ]; then
    cd portal-frontend

    if [ ! -d node_modules ]; then
        echo "[WARN] portal-frontend/node_modules missing; running npm install"
        npm install --silent
    fi

    step "tsc --noEmit" npx --no-install tsc --noEmit
    # max-warnings=56 is the frozen baseline from 2026-04-22 — ratchet down as warnings are fixed.
    step "eslint"       npx --no-install eslint . --max-warnings=56

    if [ "$WITH_TESTS" = "1" ] && npm run | grep -q '^  test$'; then
        step "frontend tests" npm test -- --run
    fi

    cd "$PROJECT_ROOT"
fi

echo ""
if [ "$fail" = "1" ]; then
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "QUALITY GATES: FAILED"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    exit 1
else
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "QUALITY GATES: PASSED"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
fi
