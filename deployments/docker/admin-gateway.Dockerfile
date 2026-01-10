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

# Generate proto files for all services
RUN mkdir -p api/proto/authv1 api/proto/clientv1 api/proto/providerv1 api/proto/routingv1 api/proto/analyticsv1 api/proto/billingv1
RUN protoc \
    --go_out=api/proto/authv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/authv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/auth/auth.proto

RUN protoc \
    --go_out=api/proto/clientv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/clientv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/client/client.proto

RUN protoc \
    --go_out=api/proto/providerv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/providerv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/provider/provider.proto

RUN protoc \
    --go_out=api/proto/routingv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/routingv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/routing/routing.proto

RUN protoc \
    --go_out=api/proto/analyticsv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/analyticsv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/analytics/analytics.proto

RUN protoc \
    --go_out=api/proto/billingv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/billingv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/billing/billing.proto

# Update dependencies
RUN go mod tidy

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o admin-gateway ./cmd/admin-gateway

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/admin-gateway .

# Expose ports (HTTP: 8081, gRPC: 9091)
EXPOSE 8081 9091

# Run the binary
CMD ["./admin-gateway"]
