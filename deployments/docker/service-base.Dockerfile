# Базовый Dockerfile для всех микросервисов
# Используется через build args: SERVICE_NAME и SERVICE_PATH
ARG SERVICE_NAME
ARG SERVICE_PATH

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

# Generate proto files - генерируем все proto файлы для совместимости
RUN mkdir -p api/proto/authv1 api/proto/messagingv1 api/proto/routingv1 \
    api/proto/providerv1 api/proto/clientv1 api/proto/analyticsv1 api/proto/billingv1

RUN protoc \
    --go_out=api/proto/authv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/authv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/auth/auth.proto || true

RUN protoc \
    --go_out=api/proto/messagingv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/messagingv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/messaging/messaging.proto || true

RUN protoc \
    --go_out=api/proto/routingv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/routingv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/routing/routing.proto || true

RUN protoc \
    --go_out=api/proto/providerv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/providerv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/provider/provider.proto || true

RUN protoc \
    --go_out=api/proto/clientv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/clientv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/client/client.proto || true

RUN protoc \
    --go_out=api/proto/analyticsv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/analyticsv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/analytics/analytics.proto || true

RUN protoc \
    --go_out=api/proto/billingv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/billingv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/billing/billing.proto || true

# Update dependencies
RUN go mod tidy

# Build the application
ARG SERVICE_NAME
ARG SERVICE_PATH
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ${SERVICE_NAME} ./${SERVICE_PATH}

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /root/

# Copy the binary from builder
ARG SERVICE_NAME
COPY --from=builder /app/${SERVICE_NAME} .

# Run the binary
CMD ["./${SERVICE_NAME}"]
