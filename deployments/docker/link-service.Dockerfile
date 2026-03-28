# deployments/docker/link-service.Dockerfile
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git protobuf protobuf-dev

WORKDIR /app

# Install protoc plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Generate proto files for link service
RUN mkdir -p api/proto/linkv1 && \
    protoc \
    --go_out=api/proto/linkv1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/linkv1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/link/link.proto && \
    mv api/proto/linkv1/link/* api/proto/linkv1/ 2>/dev/null || true && \
    rm -rf api/proto/linkv1/link || true

RUN CGO_ENABLED=0 go build -o /link-service ./cmd/services/link-service

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata wget
COPY --from=builder /link-service /link-service
EXPOSE 9102 8085
CMD ["/link-service"]
