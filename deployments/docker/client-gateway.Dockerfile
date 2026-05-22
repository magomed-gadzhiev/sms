FROM golang:alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make protobuf protobuf-dev

# Install protoc plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Generate proto files и перемещаем их в правильную структуру
RUN mkdir -p api/proto/messagingv1 api/proto/authv1 api/proto/billingv1 api/proto/analyticsv1

RUN protoc \
    --go_out=api/proto/messagingv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/messagingv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/messaging/messaging.proto && \
    mv api/proto/messagingv1/messaging/* api/proto/messagingv1/ 2>/dev/null || true && \
    rm -rf api/proto/messagingv1/messaging

RUN protoc \
    --go_out=api/proto/authv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/authv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/auth/auth.proto && \
    mv api/proto/authv1/auth/* api/proto/authv1/ 2>/dev/null || true && \
    rm -rf api/proto/authv1/auth

RUN protoc \
    --go_out=api/proto/billingv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/billingv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/billing/billing.proto && \
    mv api/proto/billingv1/billing/* api/proto/billingv1/ 2>/dev/null || true && \
    rm -rf api/proto/billingv1/billing

RUN protoc \
    --go_out=api/proto/analyticsv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/analyticsv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/analytics/analytics.proto && \
    mv api/proto/analyticsv1/analytics/* api/proto/analyticsv1/ 2>/dev/null || true && \
    rm -rf api/proto/analyticsv1/analytics

# Обновление кеша пакетов после генерации proto файлов
# Go должен автоматически видеть сгенерированные файлы как часть модуля,
# но нужно обновить кеш пакетов
RUN go list -e ./api/proto/messagingv1/... ./api/proto/authv1/... ./api/proto/billingv1/... ./api/proto/analyticsv1/... > /dev/null 2>&1 || true

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o client-gateway ./cmd/client-gateway

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/client-gateway .

# Expose ports (HTTP: 8080, gRPC: 9090)
EXPOSE 8080 9090

# Run the binary
CMD ["./client-gateway"]