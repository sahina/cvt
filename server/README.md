# CVT Server (Go Implementation)

The CVT server is a Go-based gRPC service that validates HTTP interactions against OpenAPI v2 and v3 specifications.

## Architecture

- **Language**: Go 1.25
- **Framework**: gRPC
- **Validation Engine**: kin-openapi
- **Caching**: Ristretto (max 1000 schemas, 24h TTL, LRU eviction)
- **Logging**: Zap (structured logging)
- **Health Checks**: gRPC Health Check Protocol

## Key Advantages over Java Implementation

- **Image Size**: ~30-40MB (vs ~200MB+ for Java)
- **Startup Time**: <1 second (vs 3-5 seconds for Java)
- **Memory Usage**: ~50-100MB (vs ~512MB+ for Java)
- **Binary Size**: ~19MB (single static binary)
- **No Runtime Dependencies**: Doesn't require JVM

## Project Structure

```shell
server/
├── main.go                    # Server entry point (thin wrapper)
├── cvtservice/                # Core service implementation (importable package)
│   ├── validator_service.go   # Core validation service (RegisterSchema, ValidateInteraction, CanIDeploy)
│   ├── compatibility_engine.go # Breaking change detection between schema versions
│   ├── validation_utils.go    # Input validation utilities
│   ├── cache.go               # Ristretto cache for schemas and consumers
│   ├── health.go              # gRPC health check service
│   ├── logger.go              # Structured logging with Zap
│   ├── metrics.go             # Prometheus metrics
│   ├── auth.go                # API key authentication
│   ├── tls.go                 # TLS/mTLS configuration
│   ├── audit_logger.go        # Audit logging for compliance
│   └── *_test.go              # Comprehensive test suite
├── storage/                   # Persistent storage layer
│   ├── storage.go             # Storage interface
│   ├── config.go              # Storage configuration
│   ├── sqlite/                # SQLite backend
│   └── postgres/              # PostgreSQL backend
├── pb/                        # Generated protobuf code
│   ├── cvt.pb.go
│   └── cvt_grpc.pb.go
├── testdata/                  # Test fixtures
├── go.mod                     # Go module dependencies
├── go.sum                     # Dependency checksums
├── Dockerfile                 # Multi-stage Docker build
└── README.md                  # This file
```

### Package Structure

The server is organized as an importable library (`cvtservice`) with a thin main wrapper:

- **`server/main.go`**: Entry point that imports and configures the service
- **`server/cvtservice/`**: Core service logic (can be imported by CLI and other tools)
- **`server/storage/`**: Pluggable persistence backends (SQLite, PostgreSQL, in-memory)

## Building

### Local Build

```bash
# Build the server binary
go build -o cvt-server .

# Or use Make
make build
```

### Docker Build

```bash
# Build Docker image
docker build -t cvt-server .

# Or use Docker Compose
make up
```

## Running

### Run Locally

```bash
# Run directly
go run .

# Or run the built binary
./cvt-server

# Or use Make
make run-server
```

The server will start on port `9550` by default. You can change it by setting the `CVT_PORT` environment variable:

```bash
CVT_PORT=9090 go run .
```

### Run in Docker

```bash
# Start with Docker Compose
make up

# Check health
make check-health

# View logs
make logs

# Stop
make down
```

## Testing

### Unit Tests

```bash
# Run all unit tests
go test -v ./server/cvtservice/...

# Or use Make
make test-server

# Run specific test
go test -v -run TestValidateSchemaID ./server/cvtservice/...

# Run tests with coverage
go test -v -coverprofile=coverage.out ./server/cvtservice/...
go tool cover -html=coverage.out

# Run storage tests
go test -v ./server/storage/...
```

### Integration Tests

```bash
# Run integration tests (requires Docker)
go test -v -tags=integration ./...

# Or use Make
make test-integration
```

### Cache Tests

```bash
# Run cache-specific tests
go test -v -run TestCache ./...

# Or use Make
make test-cache
```

## Dependencies

Core dependencies (see `go.mod` for full list):

- **kin-openapi** (v0.133.0): OpenAPI 3.0/2.0 validation
- **gRPC** (v1.77.0): gRPC server implementation
- **Ristretto** (v0.2.0): High-performance caching
- **Zap** (v1.27.1): Structured logging

## Validation Features

### Input Validation (validation_utils.go)

- **Schema ID**: Max 255 characters, not empty
- **Schema Content**: Max 10MB, not empty
- **HTTP Method**: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS (case-insensitive)
- **HTTP Path**: Must start with '/', not empty
- **Status Code**: Range 100-599

### OpenAPI Validation (kin-openapi)

- Supports OpenAPI 2.0 (Swagger) and OpenAPI 3.0/3.1
- Request method validation
- Request path matching
- Request body schema validation
- Request header validation
- Response status code validation
- Response body schema validation
- Response header validation
- Complex schemas (allOf, refs, nested objects, arrays)

## Caching

The server uses Ristretto for high-performance schema caching:

- **Capacity**: 1000 schemas max
- **TTL**: 24 hours
- **Eviction**: LRU (Least Recently Used)
- **Thread-Safe**: Concurrent access supported
- **Metrics**: Cache hits, misses, and evictions

## Logging

Structured logging with Zap:

- **Production Mode**: JSON-formatted logs with timestamps
- **Development Mode**: Console-formatted logs with colors
- **Log Levels**: DEBUG, INFO, WARN, ERROR, FATAL

Example log output:

```json
{
  "level": "info",
  "timestamp": "2025-11-26T21:51:00Z",
  "msg": "Schema registered successfully",
  "schemaId": "my-api-v1"
}
```

## Health Checks

The server implements the [gRPC Health Checking Protocol](https://github.com/grpc/grpc/blob/master/doc/health-checking.md):

### Available Services

| Service Name            | Description               |
| ----------------------- | ------------------------- |
| `` (empty)              | Overall server health     |
| `cvt.ContractValidator` | Validation service health |

### Health Check Commands

```bash
# Install grpc-health-probe
make install-health-probe

# Quick health check
make health

# Detailed health check
make check-health

# Continuous monitoring
make watch-health
```

### Manual Health Check

```bash
grpc-health-probe -addr=localhost:9550
grpc-health-probe -addr=localhost:9550 -service=cvt.ContractValidator
```

## Docker

### Image Details

- **Base Image**: Alpine 3.21 (~5MB)
- **Final Image Size**: ~30-40MB
- **Build Time**: ~2-3 minutes
- **Multi-Stage Build**: Yes (builder + runtime)
- **Non-Root User**: Yes (uid/gid 1000)
- **Health Check**: Built-in with grpc-health-probe

### Build Optimizations

- Static binary (CGO_ENABLED=0)
- Debug symbols stripped (-ldflags="-w -s")
- Minimal runtime dependencies
- Layer caching for dependencies

### Docker Commands

```bash
# Build image
docker build -t cvt-server .

# Run container
docker run -p 9550:9550 cvt-server

# Run with Docker Compose
docker compose up -d

# Check health
docker inspect cvt-server --format='{{.State.Health.Status}}'
```

## Performance

### Design Targets

- **Validation Throughput**: 5000+ validations/second (Baseline target)
- **Schema Registration**: <10ms per schema
- **Memory Usage**: ~50-100MB base, +~5MB per 100 cached schemas
- **CPU Usage**: Low (async I/O, efficient caching)
- **Startup Time**: <1 second

> **Note**: Official benchmarking suite is currently under development (see `docs/poc_status.md`).

### Optimization Tips

1. **Reuse Schemas**: Register once, validate many times
2. **Monitor Cache**: Check logs for cache hit/miss ratios
3. **Resource Allocation**: 256MB RAM minimum, 512MB recommended
4. **Concurrent Requests**: gRPC handles concurrency efficiently

## Security

### Current Implementation

- ✅ Input validation (schema size, format, paths)
- ✅ Non-root container user (uid/gid 1000)
- ✅ Structured logging (no sensitive data)
- ✅ Bounded caching (DoS protection)
- ⚠️ **Insecure gRPC** (no TLS/authentication)

### Production Recommendations

1. **Enable TLS**: Use grpc.Creds() with SSL/TLS certificates
2. **Add Authentication**: Implement API keys or JWT tokens via interceptors
3. **Rate Limiting**: Add per-client rate limits
4. **Network Security**: Use firewall rules and VPC
5. **Monitoring**: Enable metrics and alerting

## Troubleshooting

### Server Won't Start

```bash
# Check if port is in use
lsof -i :9550

# Check logs
go run . 2>&1 | grep -i error

# Run with debug logging
# (Edit logger.go: InitLogger(true) for development mode)
```

### Build Errors

```bash
# Update dependencies
go mod tidy
go mod download

# Clear module cache
go clean -modcache

# Rebuild
go build -v .
```

### Tests Failing

```bash
# Run with verbose output
go test -v ./...

# Run specific test
go test -v -run TestValidateSchemaID

# Check test coverage
go test -v -coverprofile=coverage.out ./...
```

### Docker Issues

```bash
# Rebuild without cache
docker build --no-cache -t cvt-server .

# Check Docker logs
docker logs cvt-server

# Verify health
docker exec cvt-server /bin/grpc-health-probe -addr=:9550
```

## Migrating from Java

### Breaking Changes

**None!** The Go server maintains 100% compatibility with the existing SDKs and protobuf contracts:

- ✅ Same gRPC service definition
- ✅ Same request/response messages
- ✅ Same validation behavior
- ✅ Same error messages
- ✅ Same health check protocol

### Deployment Strategy

1. **Test in Staging**: Deploy Go server to staging environment
2. **Validate SDKs**: Run SDK tests against Go server
3. **Performance Test**: Compare throughput and latency
4. **Blue-Green Deploy**: Run both servers temporarily
5. **Monitor**: Watch metrics and logs closely
6. **Rollback Plan**: Keep Java deployment ready if needed

## Contributing

When adding new features:

1. Write tests first (TDD approach)
2. Maintain test coverage >70% (Enforced in CI/CD)
3. Run `go fmt ./...` before committing
4. Run `go vet ./...` to catch issues
5. Update documentation

## Resources

- [gRPC Go Documentation](https://grpc.io/docs/languages/go/)
- [kin-openapi GitHub](https://github.com/getkin/kin-openapi)
- [Ristretto Cache](https://github.com/dgraph-io/ristretto)
- [Zap Logging](https://github.com/uber-go/zap)
- [Go Testing](https://golang.org/pkg/testing/)
