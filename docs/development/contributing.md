---
title: Development Guide
sidebar_label: Development
sidebar_position: 1
description: Local development setup for CVT server and SDKs
---

# Development Guide

## What is the Development Guide?

This guide provides detailed instructions for setting up a local CVT development environment, building and testing the server and SDKs, and working with the codebase.

For contribution guidelines (code style, PR process, code review), see **[CONTRIBUTING.md](https://github.com/sahina/cvt/blob/main/CONTRIBUTING.md)**.

## Prerequisites

Install the following tools before developing CVT:

| Tool    | Version      | Installation                                                                  |
| ------- | ------------ | ----------------------------------------------------------------------------- |
| Go      | 1.25+        | [golang.org/dl](https://golang.org/dl/)                                       |
| Node.js | 20+          | [nodejs.org](https://nodejs.org/)                                             |
| Python  | 3.12+        | [python.org](https://python.org/)                                             |
| uv      | latest       | `curl -LsSf https://astral.sh/uv/install.sh \| sh`                            |
| Java    | 21 (Temurin) | [adoptium.net](https://adoptium.net/)                                         |
| Docker  | latest       | [docker.com](https://docker.com/)                                             |
| protoc  | 3.x+         | [grpc.io/docs/protoc-installation](https://grpc.io/docs/protoc-installation/) |

Optional but recommended:

```bash
# Install grpc-health-probe for health checks
make install-health-probe
```

## Repository Setup

```bash
# Clone the repository
git clone https://github.com/your-org/cvt.git
cd cvt

# Build all components (server + Node.js + Python SDKs)
make build

# Verify the setup works
make test
```

## Server Development

The gRPC server is located in the `server/` directory.

### Directory Structure

```shell
server/
├── main.go                 # Entry point
├── cvtservice/             # Core service implementation
│   ├── service.go          # Main service logic (RPC handlers)
│   ├── cache.go            # Ristretto-based schema caching
│   ├── interceptors.go     # gRPC interceptors for logging, metrics, auth
│   ├── metrics.go          # Prometheus metrics collection
│   ├── audit_logger.go     # Audit logging for compliance
│   └── schema_metadata.go  # Schema metadata management
├── storage/                # Consumer registry persistence
│   ├── storage.go          # Storage interface + ConsumerRecord
│   ├── memory.go           # In-memory implementation
│   ├── sqlite/             # SQLite implementation + migrations
│   └── postgres/           # PostgreSQL implementation + migrations
├── pb/                     # Generated protobuf code
└── *_test.go               # Unit tests
```

### Building the Server

```bash
# Build the server binary
make build

# Or build directly with Go
cd server && go build -o cvt-server .
```

### Running Locally

```bash
# Run server on port 9550 (default)
make run-server

# Or with custom port
CVT_PORT=50053 make run-server

# Run directly
cd server && go run .
```

### Running Tests

```bash
# Run server unit tests
make test-server

# Run with coverage report (opens HTML)
make test-coverage

# Run a specific test
cd server && go test -v -run TestValidateInteraction ./...

# Run with race detection
cd server && go test -v -race ./...
```

### Docker Development

```bash
# Start server + observability stack
make up

# View logs
make logs

# Check health
make health

# Stop everything
make down
```

### Storage Configuration

CVT supports three storage backends. Configure with environment variables:

#### In-Memory (Default)

```bash
# No configuration needed - used by default
make run-server
```

#### SQLite

```bash
CVT_STORAGE_ENABLED=true \
CVT_STORAGE_TYPE=sqlite \
CVT_STORAGE_DSN=./cvt.db \
make run-server
```

#### PostgreSQL

```bash
CVT_STORAGE_ENABLED=true \
CVT_STORAGE_TYPE=postgres \
CVT_POSTGRES_HOST=localhost \
CVT_POSTGRES_PORT=5432 \
CVT_POSTGRES_USER=cvt \
CVT_POSTGRES_PASSWORD=secret \
CVT_POSTGRES_DB=cvt \
make run-server
```

See [Configuration Reference](../reference/configuration.md) for all environment variables.

## SDK Development

### Node.js SDK

Location: `sdks/node/`

```bash
cd sdks/node

# Install dependencies
npm install

# Build TypeScript
npm run build

# Run tests
npm test

# Run linting
npm run lint

# Run example
npm run example
```

### Python SDK

Location: `sdks/python/`

```bash
cd sdks/python

# Install dependencies (creates virtual environment)
uv sync

# Run tests with coverage
uv run pytest --cov

# Run linting
uv run ruff check .

# Run example
uv run python examples/basic_usage.py
```

### Go SDK

Location: `sdks/go/`

```bash
cd sdks/go

# Download dependencies
go mod download

# Run tests
go test -v ./...

# Run tests with coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Java SDK

Location: `sdks/java/`

```bash
cd sdks/java

# Build the SDK
./gradlew build

# Run tests
./gradlew test

# Generate coverage report
./gradlew jacocoTestReport
```

## Protobuf Code Generation

Regenerate protobuf code after modifying `api/protos/cvt.proto`:

```bash
# Generate Go server code (server/pb/)
make generate

# Generate Go SDK code (sdks/go/cvt/proto/)
make generate-go-sdk

# Generate Python SDK code (sdks/python/cvt_sdk/proto/)
make generate-python

# Generate Java SDK code (sdks/java/build/generated/)
make generate-java-sdk
```

The Node.js SDK uses dynamic proto loading and doesn't require code generation.

## TLS/mTLS Development Setup

### Generate Certificates

```bash
# Generate certificates in ./certs directory
./tools/gen-certs.sh
```

This creates:
- `ca.crt`, `ca.key` - Certificate Authority
- `server.crt`, `server.key` - Server certificate
- `client.crt`, `client.key` - Client certificate (for mTLS)

### Run Server with TLS

```bash
CVT_TLS_ENABLED=true \
CVT_TLS_CERT_FILE=./certs/server.crt \
CVT_TLS_KEY_FILE=./certs/server.key \
make run-server
```

### Run Server with mTLS

```bash
CVT_TLS_ENABLED=true \
CVT_TLS_CERT_FILE=./certs/server.crt \
CVT_TLS_KEY_FILE=./certs/server.key \
CVT_TLS_CA_FILE=./certs/ca.crt \
CVT_TLS_CLIENT_AUTH=require \
make run-server
```

## Testing SDKs Locally

Test SDK changes in consumer projects without publishing to a registry.

### Node.js - npm link

```bash
# In the SDK directory
cd sdks/node
npm run build
npm link

# In your consumer project
npm link @cvt/cvt-sdk
```

### Python - Editable Install

```bash
# Using uv
uv pip install -e /path/to/cvt/sdks/python

# Using pip
pip install -e /path/to/cvt/sdks/python
```

### Go - replace Directive

Add to your project's `go.mod`:

```go
replace github.com/sahina/cvt/sdks/go => /path/to/cvt/sdks/go
```

### Java - publishToMavenLocal

```bash
cd sdks/java
./gradlew publishToMavenLocal
```

## Common Development Tasks

### Running CI Checks Locally

```bash
# Run full CI pipeline (lint + format + tests)
make ci
```

### Updating Dependencies

```bash
# Update all SDKs
make update

# Update individual components
make update-server
cd sdks/node && npm update
cd sdks/python && uv lock --upgrade
cd sdks/go && go get -u ./...
cd sdks/java && ./gradlew dependencyUpdates
```

### Cleaning Build Artifacts

```bash
make clean
```

## Troubleshooting

### Server won't start on port 9550

The port may be in use. Specify a different one:

```bash
CVT_PORT=50053 make run-server
```

### Tests fail with "connection refused"

Ensure the CVT server is running:

```bash
make up
make health
```

### Protobuf generation fails

Ensure `protoc` and plugins are installed:

```bash
which protoc
which protoc-gen-go
which protoc-gen-go-grpc
```

---

## Related Documentation

- **[CONTRIBUTING.md](https://github.com/sahina/cvt/blob/main/CONTRIBUTING.md)** - Contribution guidelines and code style
- **[Configuration Reference](../reference/configuration.md)** - Environment variables
- **[API Reference](../reference/api.mdx)** - gRPC API details
- **[Observability Guide](../operations/observability.md)** - Metrics and monitoring
