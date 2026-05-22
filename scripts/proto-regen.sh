#!/usr/bin/env bash
# Regenerate Go gRPC stubs from .proto files inside a pinned Docker image.
#
# Usage:
#   ./scripts/proto-regen.sh                   # regen targets из TARGETS массива ниже
#   ./scripts/proto-regen.sh auth client       # regen только auth и client
#   ./scripts/proto-regen.sh --build           # пересобрать Docker-образ (после изменения Dockerfile)
#
# Targeted regen: только файлы из TARGETS массива. Все 21 .proto зарегистрированы
# (proto-нормализация 2026-05-02, см. docs/audit-followups-progress.md).
#
# Каждый target — это запись `name|proto_relative_file|proto_path_arg|out_dir`:
#   name              — короткое имя для CLI-аргумента и логов
#   proto_relative_file — путь к .proto относительно proto_path_arg (передаётся
#                        protoc'у как файл; обычно basename типа `auth.proto`)
#   proto_path_arg    — значение для --proto_path (определяет header `source:` в .pb.go)
#   out_dir           — куда писать .pb.go (paths=source_relative)
#
# ВАЖНО: proto_path_arg для каждого target подобран так, чтобы получить
# идемпотентный regen без подкаталогов в output. См. секцию open observations
# в docs/audit-followups-progress.md о том, почему текущие headers .pb.go
# не воспроизводятся стандартным protoc.

set -euo pipefail

readonly IMAGE_NAME="sms-proto-gen:latest"
readonly DOCKERFILE="deployments/docker/proto-gen.Dockerfile"

# Реестр regen targets. Расширять по мере необходимости.
# Поля разделены `|`. Не использовать пробелы внутри полей.
# Стратегия: proto_path указывает на директорию .proto, proto_relative_file — basename.
# Это даёт output `<out>/<basename>.pb.go` без подкаталогов и idempotent regen.
#
# Жертва: header `source:` в .pb.go станет "<basename>.proto", тогда как существующие
# файлы имеют разнобой ("api/proto/auth/auth.proto", "client/client.proto" и т.д.) —
# реверс этих headers через стандартный protoc невозможен (они генерировались разными
# командами/инструментами в разное время). Первый regen каждого target'а даст
# одно-двухстрочный cosmetic diff в header, далее regen идемпотентен.
readonly -a TARGETS=(
  "analytics|analytics.proto|api/proto/analytics|api/proto/analyticsv1"
  "audit|audit.proto|api/proto/audit|api/proto/auditv1"
  "auth|auth.proto|api/proto/auth|api/proto/authv1"
  "billing|billing.proto|api/proto/billing|api/proto/billingv1"
  "campaign|campaign.proto|api/proto/campaign|api/proto/campaignv1"
  "cascade|cascade.proto|api/proto/cascade|api/proto/cascadev1"
  "client|client.proto|api/proto/client|api/proto/clientv1"
  "client-provider|client_provider.proto|api/proto/client-provider|api/proto/clientproviderv1"
  "company|company.proto|api/proto/company|api/proto/companyv1"
  "contact|contact.proto|api/proto/contact|api/proto/contactv1"
  "link|link.proto|api/proto/link|api/proto/linkv1"
  "messaging|messaging.proto|api/proto/messaging|api/proto/messagingv1"
  "network_analytics|network_analytics.proto|api/proto/network_analytics|api/proto/networkanalyticsv1"
  "provider|provider.proto|api/proto/provider|api/proto/providerv1"
  "routing|routing.proto|api/proto/routing|api/proto/routingv1"
  "sender_name|sender_name.proto|api/proto/sender-name|api/proto/sendernamev1"
  "smpp|smpp.proto|api/proto/smpp|api/proto/smppv1"
  "sms|sms.proto|api/proto|api/proto/smsv1"
  "tarification|tarification.proto|api/proto/tarification|api/proto/tarificationv1"
  "template|template.proto|api/proto/template|api/proto/templatev1"
  "webhook|webhook.proto|api/proto/webhook|api/proto/webhookv1"
)

build_image() {
  echo "[proto-regen] Building image $IMAGE_NAME from $DOCKERFILE..."
  docker build -t "$IMAGE_NAME" -f "$DOCKERFILE" .
}

ensure_image() {
  if ! docker image inspect "$IMAGE_NAME" >/dev/null 2>&1; then
    build_image
  fi
}

regen_one() {
  local name="$1" proto="$2" path_arg="$3" out="$4"
  echo "[proto-regen] $name: protoc --proto_path=$path_arg $proto → $out"
  mkdir -p "$out"
  # На Windows/MSYS bash автоматически конвертирует POSIX-пути в Windows-пути,
  # ломая `-w /src` и volume-mount. MSYS_NO_PATHCONV=1 отключает конвертацию.
  # `pwd` под MSYS возвращает /c/projects/... — Docker требует C:/projects/...,
  # для этого используем `pwd -W` (Windows-form), доступную в MSYS.
  local host_pwd
  if [[ "$(uname -s)" == MINGW* || "$(uname -s)" == MSYS* ]]; then
    host_pwd="$(pwd -W)"
  else
    host_pwd="$(pwd)"
  fi
  MSYS_NO_PATHCONV=1 docker run --rm -v "$host_pwd:/src" -w /src "$IMAGE_NAME" \
    protoc \
      --proto_path="$path_arg" \
      --go_out="$out" --go_opt=paths=source_relative \
      --go-grpc_out="$out" --go-grpc_opt=paths=source_relative \
      "$proto"
}

main() {
  if [[ "${1:-}" == "--build" ]]; then
    build_image
    exit 0
  fi

  ensure_image

  local -a wanted=()
  if [[ $# -eq 0 ]]; then
    for entry in "${TARGETS[@]}"; do
      wanted+=("${entry%%|*}")
    done
  else
    wanted=("$@")
  fi

  local matched=0
  for want in "${wanted[@]}"; do
    for entry in "${TARGETS[@]}"; do
      IFS='|' read -r name proto path_arg out <<<"$entry"
      if [[ "$name" == "$want" ]]; then
        regen_one "$name" "$proto" "$path_arg" "$out"
        matched=$((matched + 1))
        break
      fi
    done
  done

  if [[ $matched -eq 0 ]]; then
    echo "[proto-regen] ERROR: no matching targets for: ${wanted[*]}"
    echo "[proto-regen] Available targets:"
    for entry in "${TARGETS[@]}"; do
      echo "  - ${entry%%|*}"
    done
    exit 1
  fi

  echo "[proto-regen] Done: $matched target(s) regenerated."
}

main "$@"
