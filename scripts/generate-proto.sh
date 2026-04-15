#!/bin/bash
# Скрипт для генерации gRPC кода из proto файлов

set -e

# Проверка наличия protoc
if ! command -v protoc &> /dev/null; then
    echo "Ошибка: protoc не установлен"
    echo "Установите protoc: https://grpc.io/docs/protoc-installation/"
    exit 1
fi

# Проверка наличия плагинов
if ! command -v protoc-gen-go &> /dev/null; then
    echo "Установка protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "Установка protoc-gen-go-grpc..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

# Список всех proto файлов для генерации
declare -a proto_files=(
    "api/proto/sms.proto:api/proto/smsv1"
    "api/proto/auth/auth.proto:api/proto/authv1"
    "api/proto/messaging/messaging.proto:api/proto/messagingv1"
    "api/proto/routing/routing.proto:api/proto/routingv1"
    "api/proto/provider/provider.proto:api/proto/providerv1"
    "api/proto/client/client.proto:api/proto/clientv1"
    "api/proto/analytics/analytics.proto:api/proto/analyticsv1"
    "api/proto/billing/billing.proto:api/proto/billingv1"
    "api/proto/contact/contact.proto:api/proto/contactv1"
    "api/proto/campaign/campaign.proto:api/proto/campaignv1"
    "api/proto/smpp/smpp.proto:api/proto/smppv1"
)

errors=()

for entry in "${proto_files[@]}"; do
    IFS=':' read -r proto_path output_dir <<< "$entry"
    
    # Создание директории для сгенерированного кода
    mkdir -p "$output_dir"
    
    # Проверка существования proto файла
    if [ ! -f "$proto_path" ]; then
        echo "Предупреждение: файл $proto_path не найден, пропускаем..."
        continue
    fi
    
    echo "Генерация кода из $proto_path..."
    
    # Генерация кода
    if protoc \
        --go_out="$output_dir" \
        --go_opt=paths=source_relative \
        --go-grpc_out="$output_dir" \
        --go-grpc_opt=paths=source_relative \
        --proto_path=api/proto \
        "$proto_path"; then
        echo "  ✓ $proto_path - успешно"
    else
        echo "  ✗ $proto_path - ошибка"
        errors+=("$proto_path")
    fi
done

if [ ${#errors[@]} -eq 0 ]; then
    echo ""
    echo "Генерация всех proto файлов завершена успешно!"
    exit 0
else
    echo ""
    echo "Ошибка при генерации следующих файлов:"
    for err in "${errors[@]}"; do
        echo "  - $err"
    done
    exit 1
fi
