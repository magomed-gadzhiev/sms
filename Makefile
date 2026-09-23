.PHONY: gen-proto-docs build-client-gateway version test test-functional test-integration test-e2e demo-seed demo-clean

# Release version stamped into binaries via ldflags (internal/version).
# Falls back to the short commit hash between tags, with a -dirty suffix.
VERSION ?= $(shell git describe --tags --always --dirty)
VERSION_LDFLAGS := -X github.com/smpp-server/smpp-server/internal/version.Version=$(VERSION)

# ─── Test-launch seam (architecture review, candidate 5) ────────────────────
# The make targets own provisioning: build tags, env, and the host ports of
# the local deployments compose stand. Preconditions: the stand is up
# (docker compose -f deployments/docker-compose.yml up -d) and
# deployments/.env exists.

DEPLOYMENTS_ENV := deployments/.env
POSTGRES_PASSWORD ?= $(shell grep -E '^POSTGRES_PASSWORD=' $(DEPLOYMENTS_ENV) | cut -d= -f2)
REDIS_PASSWORD   ?= $(shell grep -E '^REDIS_PASSWORD=' $(DEPLOYMENTS_ENV) | cut -d= -f2)

# One DSN convention across integration tests (postgres:// URL).
TEST_DATABASE_URL ?= postgres://smpp:$(POSTGRES_PASSWORD)@127.0.0.1:5432/smpp_db?sslmode=disable&default_query_exec_mode=simple_protocol
TEST_PORTAL_URL   ?= http://127.0.0.1:8080
TEST_REDIS_URL    ?= redis://:$(REDIS_PASSWORD)@127.0.0.1:6379/0

gen-proto-docs:
	bash scripts/gen-proto-docs.sh

build-client-gateway:
	go build -ldflags "$(VERSION_LDFLAGS)" ./cmd/client-gateway/...

version:
	@echo $(VERSION)

test:
	go test ./... 2>&1

test-functional:
	TEST_DB_HOST=127.0.0.1 TEST_DB_PORT=5432 TEST_DB_USER=smpp \
	TEST_DB_PASSWORD=$(POSTGRES_PASSWORD) TEST_DB_NAME=smpp_db \
	TEST_REDIS_ADDR=127.0.0.1:6379 \
	go test -tags functional ./test/functional/...

test-integration:
	TEST_DATABASE_URL='$(TEST_DATABASE_URL)' \
	TEST_REDIS_URL='$(TEST_REDIS_URL)' \
	TEST_PORTAL_URL='$(TEST_PORTAL_URL)' \
	go test -tags integration ./test/integration/...

test-e2e:
	cd e2e && npm ci && npx playwright test

# Демо-данные на локальном стенде (deployments, контейнер postgres).
# Полный цикл с пре/пост-условиями: очистка -> demo_seed.sql -> demo_data.sql
# -> flush Redis. ВНИМАНИЕ: demo_seed перезаписывает пароль portal-admin
# (см. test/load/README.md).
demo-seed:
	bash scripts/demo-seed.sh
	@docker exec redis redis-cli --no-auth-warning \
		-a "$(REDIS_PASSWORD)" FLUSHDB >/dev/null \
		&& echo "Redis flushed"

demo-clean:
	bash scripts/demo-seed.sh --clean
