FROM golang:1.24-alpine AS dev

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make bash

# Install development tools
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Default command
CMD ["/bin/sh"]
