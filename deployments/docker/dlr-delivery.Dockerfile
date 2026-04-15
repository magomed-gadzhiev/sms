FROM golang:alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o dlr-delivery ./cmd/dlr-delivery

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/dlr-delivery .

# Expose metrics port
EXPOSE 2112

# Run the binary
CMD ["./dlr-delivery"]
