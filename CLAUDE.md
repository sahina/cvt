# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CVT (Contract Validator Toolkit) is a consumer-based contract validation platform for OpenAPI v2/v3 specifications. It consists of a Go gRPC server that validates HTTP request/response interactions against registered OpenAPI schemas, with SDKs for Node.js, Python, Go, and Java.

## Project Structure

```shell
cvt/
├── api/protos/         # gRPC proto definitions
├── cmd/cvt/            # CLI entry point
├── server/             # gRPC server implementation
│   ├── cvtservice/     # Core service logic
│   └── storage/        # Persistence backends (sqlite, postgres, memory)
├── pkg/cvt/            # Embeddable Go library
├── sdks/               # Client SDKs (node, python, go, java, shared)
├── docs/               # Detailed documentation
├── examples/           # Example code and schemas
├── observability/      # Prometheus/Grafana configuration
├── config/             # Configuration files
├── ci-templates/       # CI/CD templates
├── certs/              # TLS certificates
└── tools/              # Build and test scripts
```

## Common Commands

### Building and Running

```bash
make build                 # Build Go server, Node.js SDK, and Python SDK
make up                    # Start server + observability stack (Docker)
make down                  # Stop all Docker services
make run-server            # Run Go server locally on port 9550
make run-example           # Run Node.js SDK example
```

### Testing

```bash
make test                  # Fast tests with direct server (no Docker)
make test-docker           # Full tests with Docker + PostgreSQL
make test-server           # Go server unit tests only
make test-coverage         # Server tests with HTML coverage report
make test-node-sdk         # Node.js SDK tests (npm test)
make test-python-sdk       # Python SDK tests with coverage
make test-go-sdk           # Go SDK tests with coverage
make test-java-sdk         # Java SDK tests with coverage

# Run single Go test
cd server && go test -v -run TestFunctionName ./...
```

### Code Generation (after modifying api/protos/cvt.proto)

```bash
make generate              # Generate Go protobuf for server
make generate-go-sdk       # Generate Go protobuf for Go SDK
make generate-python       # Generate Python protobuf
make generate-java-sdk     # Generate Java protobuf
```

### Health & Observability

```bash
make health                # Check server health via grpc-health-probe
make grafana               # Open Grafana dashboard (localhost:3000)
make metrics               # View Prometheus metrics
```

## Architecture

### Core Components

**gRPC Server** (`server/`): Go 1.25-based service with RPC methods organized by phase:

_Phase 1 - Schema & Validation:_

- `RegisterSchema`: Registers OpenAPI v2/v3 schemas (auto-converts v2 to v3)
- `ValidateInteraction`: Validates HTTP request/response pairs against registered schemas
- `GetSchema`: Get metadata and content for a registered schema
- `ListSchemas`: List all registered schemas with optional filtering
- `ValidateProducerResponse`: Producer-side response validation
- `CompareSchemas`: Compare two schema versions for breaking changes
- `GenerateFixture`: Generate test fixtures from schemas
- `ListEndpoints`: List all endpoints in a registered schema

_Phase 2 - Consumer Registry:_

- `RegisterConsumer`: Register a consumer with expected interactions
- `ListConsumers`: List all registered consumers for a schema
- `DeregisterConsumer`: Remove a consumer registration

_Phase 3 - Deployment Safety:_

- `CanIDeploy`: Check if schema changes will break registered consumers

**Service Library** (`server/cvtservice/`): Core service implementation as an importable package. Key components:

- `service.go`: Main service implementation
- `cache.go`: Ristretto-based schema cache (1000 schemas, 24-hour TTL)
- `interceptors.go`: gRPC interceptors for logging, metrics, auth
- `metrics.go`: Prometheus metrics collection
- `audit_logger.go`: Audit logging for compliance
- `schema_metadata.go`: Schema metadata management

**Persistent Storage** (`server/storage/`): Pluggable storage backends:

- `memory.go`: In-memory storage (default, no persistence)
- `sqlite/`: SQLite storage backend
- `postgres/`: PostgreSQL storage backend

**Key Libraries**:

- `kin-openapi`: OpenAPI parsing, validation, and v2-to-v3 conversion
- `gorillamux`: Route matching for OpenAPI validation
- `ristretto`: High-performance caching
- `zap`: Structured logging

### Validation Flow

1. Client registers schema via `RegisterSchema` (parsed, validated, cached)
2. Client calls `ValidateInteraction` with request/response data
3. Server retrieves schema from cache, matches route via gorillamux
4. Request validated (path params, query, headers, body) via openapi3filter
5. Response validated (status code, headers, body) via openapi3filter

### SDKs (`sdks/`)

- **Node.js** (`sdks/node/`): TypeScript, npm, Jest - production-ready
- **Python** (`sdks/python/`): uv package manager, pytest
- **Go** (`sdks/go/`): Standard Go modules
- **Java** (`sdks/java/`): Gradle build
- **Shared** (`sdks/shared/`): Test schemas (openapi.json, swagger.json)

### Proto Definition (`api/protos/cvt.proto`)

Defines the gRPC service contract with messages: `RegisterSchemaRequest`, `InteractionRequest`, `ValidationResult`, etc.

## Environment Variables

### Core Settings

- `CVT_PORT`: gRPC server port (default: 9550)
- `CVT_METRICS_PORT`: Prometheus metrics port (default: 9551)
- `LOG_LEVEL`: Set to "debug" for development mode logging

### TLS Configuration

- `CVT_TLS_ENABLED`: Enable TLS (default: false)
- `CVT_TLS_CERT_FILE`: Path to server certificate (default: /certs/server.crt)
- `CVT_TLS_KEY_FILE`: Path to server private key (default: /certs/server.key)
- `CVT_TLS_CA_FILE`: Path to CA certificate for mTLS (optional)
- `CVT_TLS_CLIENT_AUTH`: Client auth mode: none, request, require (default: none)

### API Key Authentication

- `CVT_API_KEY_ENABLED`: Enable API key authentication (default: false)
- `CVT_API_KEYS`: Comma-separated list of valid API keys
- `CVT_API_KEYS_FILE`: Path to JSON file with API key configuration

### Persistent Storage

- `CVT_STORAGE_ENABLED`: Enable persistent storage (default: false, uses in-memory)
- `CVT_STORAGE_TYPE`: Storage backend: sqlite, postgres, or memory (default: sqlite when enabled)
- `CVT_STORAGE_DSN`: Data source name for SQLite (default: cvt.db)
- `CVT_POSTGRES_HOST`: PostgreSQL host (default: localhost)
- `CVT_POSTGRES_PORT`: PostgreSQL port (default: 5432)
- `CVT_POSTGRES_USER`: PostgreSQL user (default: cvt)
- `CVT_POSTGRES_PASSWORD`: PostgreSQL password
- `CVT_POSTGRES_DB`: PostgreSQL database name (default: cvt)
- `CVT_POSTGRES_SSLMODE`: PostgreSQL SSL mode (default: disable)
- `CVT_STORAGE_CACHE_ENABLED`: Enable in-memory cache with persistent storage (default: true)
- `CVT_STORAGE_CACHE_MAX_SCHEMAS`: Maximum schemas in cache (default: 1000)
- `CVT_VALIDATION_RETENTION_DAYS`: Days to retain validation records (default: 90)
- `CVT_POSTGRES_DSN`: Full PostgreSQL DSN (alternative to individual host/port/user/password/db)
- `CVT_POSTGRES_MAX_CONNS`: Maximum PostgreSQL connections (default: 25)

## CLI (Local Lite Mode)

The CVT CLI allows local validation without Docker:

```bash
# Build the CLI
go build -o cvt ./cmd/cvt
```

### CLI Commands

**validate** - Validate an interaction against a schema:

```bash
cvt validate --schema ./openapi.json --request req.json --response resp.json
cvt validate --schema ./openapi.json --interaction interaction.json  # Combined file
cvt validate --schema ./openapi.json --request req.json --response resp.json --json  # JSON output
```

**compare** - Compare schemas for breaking changes:

```bash
cvt compare --old ./v1/openapi.json --new ./v2/openapi.json
cvt compare --old ./v1/openapi.json --new ./v2/openapi.json --json  # JSON output
```

**generate** - Generate test fixtures from schemas:

```bash
cvt generate --schema ./openapi.json --endpoint "GET /users/{id}"
cvt generate --schema ./openapi.json --endpoint "POST /users" --output-type request
cvt generate --schema ./openapi.json --list  # List all endpoints
cvt generate --schema ./openapi.json --endpoint "GET /users" --use-examples  # Use schema examples
```

**can-i-deploy** - Check deployment safety against registered consumers:

```bash
cvt can-i-deploy --schema ./openapi.json --server localhost:9550
cvt can-i-deploy --schema ./openapi.json --server localhost:9550 --json --timeout 30s
```

**serve** - Start the gRPC server:

```bash
cvt serve --port 9550
cvt serve --port 9550 --metrics-port 9551
cvt serve --port 9550 --tls --cert server.crt --key server.key
cvt serve --port 9550 --api-key-auth  # Enable API key authentication
```

**version** - Show version information:

```bash
cvt version
```

The CLI uses the embedded library (`pkg/cvt/`) which can also be used directly in Go code.

## Port Configuration

- **9550**: gRPC server (both local and Docker)
- **9551**: Prometheus metrics endpoint
- **9091**: Prometheus UI (when running observability stack)
- **3000**: Grafana UI (admin/admin)

## Testing Patterns

Server tests use testify/assert and table-driven tests. Integration tests require Docker (`-tags=integration`). Coverage target is 70%+.

```bash
# Run all server tests
go test ./server/cvtservice/... -v

# Verbose single test
go test ./server/cvtservice/... -v -run TestValidateInteraction

# Test with race detection
go test ./server/cvtservice/... -v -race
```
