FROM golang:1.25-alpine

# Pinned tool versions, чтобы regen был воспроизводим.
# Эти версии должны совпадать с теми, что в шапках api/proto/{auth,client}v1/*.pb.go:
#   protoc-gen-go v1.36.11
#   protoc        v4.25.1   (release tag "v25.1", major bumped после v3.x)
#   protoc-gen-go-grpc v1.6.1

ARG PROTOC_VERSION=25.1

RUN apk add --no-cache git make bash curl unzip \
 && curl -fsSL -o /tmp/protoc.zip \
      "https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip" \
 && unzip -q /tmp/protoc.zip -d /usr/local \
 && rm /tmp/protoc.zip \
 && chmod +x /usr/local/bin/protoc

ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$PATH

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 \
 && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.1

WORKDIR /src
