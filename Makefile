.PHONY: gen-proto-docs build-client-gateway test

gen-proto-docs:
	bash scripts/gen-proto-docs.sh

build-client-gateway:
	go build ./cmd/client-gateway/...

test:
	go test ./... 2>&1
