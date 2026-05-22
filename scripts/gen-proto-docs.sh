#!/bin/bash
# Regenerates internal/docs/grpc.html from all .proto files in api/proto/.
# Requires: protoc, protoc-gen-doc
# Install: go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@latest
set -e

PROTO_DIR="api/proto"
OUT_FILE="internal/docs/grpc.html"

echo "Generating gRPC docs from $PROTO_DIR → $OUT_FILE"

protoc \
  --doc_out="$(dirname "$OUT_FILE")" \
  --doc_opt=html,"$(basename "$OUT_FILE")" \
  --proto_path="$PROTO_DIR" \
  $(find "$PROTO_DIR" -name "*.proto" | sort)

echo "Done. Commit $OUT_FILE to persist the changes."
