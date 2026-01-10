FROM golang:alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make protobuf protobuf-dev

# Install protoc plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.35.2
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Generate proto files
RUN mkdir -p api/proto/smsv1
RUN protoc \
    --go_out=api/proto/smsv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/smsv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/sms.proto

# Update dependencies
RUN go mod tidy

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o smpp-server ./cmd/smpp-server

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/smpp-server .

# Expose ports
EXPOSE 2775

# Run the binary
CMD ["./smpp-server"]
