#!/usr/bin/env bash
# Message Status vocabulary guard (architecture review, candidate 4).
#
# Raw Message-status string literals are banned outside the vocabulary module
# (internal/shared/messagestatus) in Message-domain code. Use
# shared.MessageStatus* aliases or the messagestatus package instead, so the
# enum, terminality and the DLR/SMPP mappers have exactly one owner.
#
# Exit 1 with offending file:line listings when a raw literal is found.
set -euo pipefail

cd "$(dirname "$0")/.."

# Message-domain scope (Go files; tests exempt — they may quote wire values).
SCOPE=(
  internal/pipeline/sender
  internal/pipeline/status
  internal/pipeline/persist
  internal/services/dlr
  internal/services/messaging
  internal/gateway/client/handlers/sms.go
)
OWNER=internal/shared/messagestatus
PATTERN='(pending|queued|sent|delivered|failed|expired|rejected|scheduled|cancelled)'

status=0
for path in "${SCOPE[@]}"; do
  [ -e "$path" ] || continue
  # Collect candidate Go files (skip tests and the vocabulary owner).
  files=$(find "$path" -name '*.go' ! -name '*_test.go' 2>/dev/null || true)
  for f in $files; do
    case "$f" in "$OWNER"/*) continue ;; esac
    # Strip line comments, then look for raw status literals in code.
    # Case-sensitive: uppercase SMPP wire codes (DELIVRD & co) are the DLR
    # vocabulary, not Message statuses. JSON object keys ("<status>":) are
    # API field names, not status values.
    offenders=$(awk '{ sub(/\/\/.*/, "");
      line = $0;
      gsub(/"[^"]*":/, "", line);
      if (line ~ /"'"$PATTERN"'"/) print FILENAME ":" FNR ": " $0 }' "$f")
    if [ -n "$offenders" ]; then
      echo "ERROR: raw Message-status literal(s) outside $OWNER:" >&2
      echo "$offenders" >&2
      status=1
    fi
  done
done

if [ $status -ne 0 ]; then
  echo "" >&2
  echo "Use github.com/smpp-server/smpp-server/internal/shared/messagestatus" >&2
  echo "(or the shared.MessageStatus* aliases) instead of raw status strings." >&2
fi
exit $status
