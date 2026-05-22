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

# Generate proto files for auth service (needed for SMPP Gateway)
RUN mkdir -p api/proto/authv1
RUN protoc \
    --go_out=api/proto/authv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/authv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/auth/auth.proto && \
    mv api/proto/authv1/auth/* api/proto/authv1/ 2>/dev/null || true && \
    rm -rf api/proto/authv1/auth

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o smpp-gateway ./cmd/smpp-gateway

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/smpp-gateway .

# Expose ports (SMPP: 2775, Metrics: 2112)
EXPOSE 2775 2112

# Run the binary
CMD ["./smpp-gateway"]
