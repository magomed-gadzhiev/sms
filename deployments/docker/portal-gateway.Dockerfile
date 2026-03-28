FROM golang:1.25-alpine AS builder

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

# Generate proto files
RUN mkdir -p api/proto/authv1 api/proto/messagingv1 api/proto/routingv1 \
    api/proto/providerv1 api/proto/clientv1 api/proto/analyticsv1 api/proto/billingv1 \
    api/proto/webhookv1 api/proto/templatev1

RUN protoc \
    --go_out=api/proto/authv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/authv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/auth/auth.proto && \
    mv api/proto/authv1/auth/* api/proto/authv1/ 2>/dev/null || true && \
    rm -rf api/proto/authv1/auth || true

RUN protoc \
    --go_out=api/proto/messagingv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/messagingv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/messaging/messaging.proto && \
    mv api/proto/messagingv1/messaging/* api/proto/messagingv1/ 2>/dev/null || true && \
    rm -rf api/proto/messagingv1/messaging || true

RUN protoc \
    --go_out=api/proto/billingv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/billingv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/billing/billing.proto && \
    mv api/proto/billingv1/billing/* api/proto/billingv1/ 2>/dev/null || true && \
    rm -rf api/proto/billingv1/billing || true

RUN protoc \
    --go_out=api/proto/analyticsv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/analyticsv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/analytics/analytics.proto && \
    mv api/proto/analyticsv1/analytics/* api/proto/analyticsv1/ 2>/dev/null || true && \
    rm -rf api/proto/analyticsv1/analytics || true

RUN protoc \
    --go_out=api/proto/clientv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/clientv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/client/client.proto && \
    mv api/proto/clientv1/client/* api/proto/clientv1/ 2>/dev/null || true && \
    rm -rf api/proto/clientv1/client || true

RUN protoc \
    --go_out=api/proto/webhookv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/webhookv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/webhook/webhook.proto && \
    mv api/proto/webhookv1/webhook/* api/proto/webhookv1/ 2>/dev/null || true && \
    rm -rf api/proto/webhookv1/webhook || true

RUN protoc \
    --go_out=api/proto/templatev1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/templatev1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/template/template.proto && \
    mv api/proto/templatev1/template/* api/proto/templatev1/ 2>/dev/null || true && \
    rm -rf api/proto/templatev1/template || true

# Update package cache after proto generation
RUN go list -e ./api/proto/... > /dev/null 2>&1 || true

# Build portal-gateway
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o portal-gateway ./cmd/portal-gateway

# Final stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/portal-gateway .

EXPOSE 8082

CMD ["./portal-gateway"]
