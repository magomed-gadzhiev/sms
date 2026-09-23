# SMS Platform

[![Release](https://img.shields.io/github/v/release/magomed-gadzhiev/sms)](https://github.com/magomed-gadzhiev/sms/releases)
[![CI](https://github.com/magomed-gadzhiev/sms/actions/workflows/ci.yml/badge.svg)](https://github.com/magomed-gadzhiev/sms/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Open-source SMS and messaging platform built in Go and React. The project includes SMPP ingress, HTTP/gRPC APIs, routing, billing, delivery status handling, campaign tooling, a self-service portal, and operational Docker Compose manifests for local development.

## Features

- SMPP gateway for client connections.
- HTTP and gRPC gateways for client, admin, and portal APIs.
- Multi-service Go backend with PostgreSQL, Redis, and Kafka.
- Routing, provider management, billing, templates, webhooks, contacts, campaigns, and delivery reports.
- React portal frontend for operators, clients, and reseller-style workflows.
- OpenAPI and protobuf API definitions under `api/`.
- Database migrations under `migrations/`.
- Docker Compose deployment files under `deployments/`.

## Repository Layout

```text
api/              API definitions: OpenAPI, protobuf, generated Go stubs
cmd/              service entry points
configs/          example application configuration
deployments/      Dockerfiles, Compose files, monitoring configs
docs/             public architecture, API, development, and deployment docs
internal/         Go application and domain packages
migrations/       PostgreSQL migrations
portal-frontend/  React/Vite frontend
scripts/          local development and maintenance scripts
test/             functional, integration, and load test assets
tests/            additional integration tests
```

## Quick Start

Requirements:

- Go 1.25+
- Node.js 20+
- Docker and Docker Compose

Create a local environment file:

```bash
cp deployments/.env.example deployments/.env
```

Set strong local values in `deployments/.env`, then start the stack:

```bash
docker compose --env-file deployments/.env -f deployments/docker-compose.yml up -d --build
```

Run backend checks:

```bash
go test ./...
go vet ./...
```

Run frontend checks:

```bash
cd portal-frontend
npm ci
npm run typecheck
npm test
npm run build
```

## Configuration

Do not commit filled environment files. Use:

- `deployments/.env.example` for local Compose defaults.
- `deployments/.env.prod.example` as a production template.
- `configs/config.example.yaml` as an application configuration reference.

All real credentials, tokens, domains, and provider settings must be supplied by environment variables or a secret manager.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Security issues should be reported privately according to [SECURITY.md](SECURITY.md).

## License

MIT. See [LICENSE](LICENSE).
