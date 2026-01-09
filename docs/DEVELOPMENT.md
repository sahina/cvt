# Local Development Guide

This guide covers local development for CVT (Contract Validator Toolkit), including server development, SDK development, local testing without publishing packages, and SDK publishing.

## Table of Contents

1. [Prerequisites](#1-prerequisites)
2. [Repository Setup](#2-repository-setup)
3. [Server Development](#3-server-development)
4. [SDK Development](#4-sdk-development)
5. [Testing SDKs Locally Without Publishing](#5-testing-sdks-locally-without-publishing)
6. [Integration Testing](#6-integration-testing)
7. [Producer Testing Development](#7-producer-testing-development)
8. [Protobuf Code Generation](#8-protobuf-code-generation)
9. [TLS/mTLS Development Setup](#9-tlsmtls-development-setup)
10. [SDK Publishing Guide](#10-sdk-publishing-guide)
11. [Common Development Tasks](#11-common-development-tasks)

---

## 1. Prerequisites

Install the following tools before developing CVT:

| Tool    | Version      | Installation                                                                  |
| ------- | ------------ | ----------------------------------------------------------------------------- |
| Go      | 1.25+        | [golang.org/dl](https://golang.org/dl/)                                       |
| Node.js | 20+          | [nodejs.org](https://nodejs.org/)                                             |
| pnpm    | 10+          | `npm install -g pnpm`                                                         |
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

---

## 2. Repository Setup

Clone and set up the repository:

```bash
# Clone the repository
git clone https://github.com/your-org/cvt.git
cd cvt

# Build all components (server + Node.js + Python SDKs)
make build

# Verify the setup works
make test
```

This runs the full test suite including server unit tests and all SDK tests.

---

## 3. Server Development

The gRPC server is located in the `server/` directory.

### Directory Structure

```shell
server/
├── main.go                 # Entry point
├── validator_service.go    # gRPC service (validation + producer testing RPCs)
├── cache.go               # Ristretto-based schema caching
├── compatibility_engine.go # Breaking change detection
├── auth.go                # API key/TLS authentication
├── health.go              # gRPC health checks
├── metrics.go             # Prometheus metrics
├── storage/               # Consumer registry persistence
│   ├── storage.go         # Storage interface + ConsumerRecord
│   ├── memory.go          # In-memory implementation
│   ├── sqlite/            # SQLite implementation + migrations
│   └── postgres/          # PostgreSQL implementation + migrations
├── pb/                    # Generated protobuf code
└── *_test.go              # Unit tests
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

# Restart server
make restart
```

### Observability

```bash
# Open Grafana dashboard (admin/admin)
make grafana

# View Prometheus metrics
make metrics

# Open Prometheus UI
make prometheus
```

### Environment Variables

| Variable              | Default           | Description                               |
| --------------------- | ----------------- | ----------------------------------------- |
| `CVT_PORT`            | 9550             | gRPC server port                          |
| `CVT_METRICS_PORT`    | 9551              | Prometheus metrics port                   |
| `LOG_LEVEL`           | info              | Logging level (debug, info, warn, error)  |
| `CVT_TLS_ENABLED`     | false             | Enable TLS                                |
| `CVT_TLS_CERT_FILE`   | /certs/server.crt | Server certificate path                   |
| `CVT_TLS_KEY_FILE`    | /certs/server.key | Server private key path                   |
| `CVT_TLS_CA_FILE`     |                   | CA certificate for mTLS                   |
| `CVT_TLS_CLIENT_AUTH` | none              | Client auth mode (none, request, require) |
| `CVT_API_KEY_ENABLED` | false             | Enable API key authentication             |
| `CVT_API_KEYS`        |                   | Comma-separated valid API keys            |
| `CVT_API_KEYS_FILE`   |                   | Path to API keys JSON file                |

---

## 4. SDK Development

### 4.1 Node.js SDK

Location: `sdks/node/`

```bash
cd sdks/node

# Install dependencies
pnpm install

# Build TypeScript
pnpm build

# Run tests
pnpm test

# Run linting
pnpm lint

# Check formatting
pnpm format:check

# Run example
pnpm example
```

**Key files:**

- `package.json` - Dependencies and scripts
- `src/` - TypeScript source code
- `src/producer/` - Producer middleware and testing
- `src/producer/testing.ts` - ProducerTestKit for schema compliance tests
- `tests/` - Jest test files
- `examples/` - Usage examples

### 4.2 Python SDK

Location: `sdks/python/`

```bash
cd sdks/python

# Install dependencies (creates virtual environment)
uv sync

# Run tests with coverage
uv run pytest --cov

# Run linting
uv run ruff check .

# Check formatting
uv run ruff format --check .

# Run example
uv run python examples/basic_usage.py
```

**Key files:**

- `pyproject.toml` - Project configuration
- `cvt_sdk/` - Python package source
- `cvt_sdk/producer/` - Producer middleware and testing
- `cvt_sdk/producer/testing.py` - ProducerTestKit for schema compliance tests
- `tests/` - pytest test files
- `examples/` - Usage examples

### 4.3 Go SDK

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

# Format code
go fmt ./...

# Run linting
go vet ./...
```

**Key files:**

- `go.mod` - Module definition
- `cvt/` - Main package
- `cvt/adapters/` - HTTP adapters
- `cvt/producer/` - Producer middleware and testing
- `cvt/producer/testing.go` - ProducerTestKit for schema compliance tests
- `examples/` - Usage examples

### 4.4 Java SDK

Location: `sdks/java/`

```bash
cd sdks/java

# Build the SDK
./gradlew build

# Run tests
./gradlew test

# Generate coverage report
./gradlew jacocoTestReport
# Report at: build/reports/jacoco/test/html/index.html

# Run checks (lint + style)
./gradlew check
```

**Key files:**

- `build.gradle` - Gradle build configuration
- `src/main/java/com/cvt/` - Java source
- `src/main/java/com/cvt/sdk/producer/` - Producer middleware and testing
- `src/main/java/com/cvt/sdk/producer/ProducerTestKit.java` - Schema compliance tests
- `src/test/java/com/cvt/` - JUnit tests
- `examples/` - Usage examples

---

## 5. Testing SDKs Locally Without Publishing

Test SDK changes in consumer projects without publishing to a registry.

### 5.1 Node.js SDK - pnpm link

```bash
# In the SDK directory
cd sdks/node
pnpm build
pnpm link --global

# In your consumer project
cd /path/to/your/project
pnpm link --global @cvt/cvt-sdk
```

#### Alternative: File protocol in package.json

```json
{
  "dependencies": {
    "@cvt/cvt-sdk": "file:../path/to/cvt/sdks/node"
  }
}
```

Then run `pnpm install`.

### 5.2 Python SDK - Editable Install

```bash
# Using uv (recommended)
cd /path/to/your/project
uv pip install -e /path/to/cvt/sdks/python

# Using pip
pip install -e /path/to/cvt/sdks/python
```

#### Alternative: Path dependency in pyproject.toml

```toml
[project]
dependencies = [
    "cvt-sdk @ file:///path/to/cvt/sdks/python"
]
```

### 5.3 Go SDK - replace Directive

Add to your project's `go.mod`:

```go
replace github.com/cvt/cvt-sdk/go => /path/to/cvt/sdks/go
```

Then run `go mod tidy`.

**Remove the replace directive before committing** - it's only for local development.

### 5.4 Java SDK - publishToMavenLocal

```bash
# Publish to local Maven repository
cd sdks/java
./gradlew publishToMavenLocal
```

The SDK is now available at `~/.m2/repository/com/cvt/cvt-sdk/`.

**In your consumer project (Gradle):**

```gradle
repositories {
    mavenLocal()
    mavenCentral()
}

dependencies {
    implementation 'com.cvt:cvt-sdk:1.0.0'
}
```

**In your consumer project (Maven):**

```xml
<dependency>
    <groupId>com.cvt</groupId>
    <artifactId>cvt-sdk</artifactId>
    <version>1.0.0</version>
</dependency>
```

---

## 6. Integration Testing

Integration tests validate SDKs against a running CVT server.

### Running Integration Tests

```bash
# Start the server
make up

# Wait for health check to pass
make health

# Run all SDK tests (they connect to the running server)
make test

# Or run individual SDK tests
make test-node-sdk
make test-python-sdk
make test-go-sdk
make test-java-sdk

# Keep the stack running after tests (for debugging)
KEEP_DOCKER_UP=1 make test

# Stop when done
make down
```

### Manual Testing

```bash
# Start server in one terminal
make up
make logs

# In another terminal, run SDK examples
cd sdks/node && pnpm example
cd sdks/python && uv run python examples/basic_usage.py
cd sdks/go && go run examples/basic/main.go
cd sdks/java && ./gradlew run
```

---

## 7. Producer Testing Development

Producer testing validates that your API implementation matches your OpenAPI specification. This section covers the non-middleware approach using ProducerTestKit.

### ProducerTestKit

The ProducerTestKit validates handler responses against schemas without requiring middleware integration. Use it in your unit/integration tests.

**Running producer tests locally:**

```bash
# Start the CVT server (required for ProducerTestKit)
make run-server

# Run SDK tests that include producer testing
make test-node-sdk
make test-go-sdk
make test-python-sdk
make test-java-sdk
```

**Example usage in tests:**

```typescript
// Node.js
import { ProducerTestKit } from "@cvt/cvt-sdk/producer";

const testKit = new ProducerTestKit({
  schemaId: "my-api",
  serverAddress: "localhost:9550",
});

const result = await testKit.validateResponse({
  method: "GET",
  path: "/users/123",
  statusCode: 200,
  body: { id: "123", name: "Alice" },
});

expect(result.valid).toBe(true);
await testKit.close();
```

### Consumer Registry

Track which services depend on your API. The registry stores consumer dependencies for deployment safety checks.

```bash
# Consumer registry uses in-memory storage by default
# For persistent storage, configure SQLite or PostgreSQL:

# SQLite
CVT_STORAGE_TYPE=sqlite CVT_SQLITE_PATH=./cvt.db make run-server

# PostgreSQL
CVT_STORAGE_TYPE=postgres CVT_POSTGRES_URL=postgres://user:pass@localhost/cvt make run-server
```

### Deployment Safety (can-i-deploy)

Check if schema changes are safe to deploy before releasing:

```bash
# Build the CLI
go build -o cvt ./cmd/cvt

# Check deployment safety
cvt can-i-deploy --schema my-api --version 2.0.0 --env prod

# With custom server address
cvt can-i-deploy --schema my-api --version 2.0.0 --env prod --server localhost:9550
```

### Key Producer Testing Files

| Component  | Files                                                                            |
| ---------- | -------------------------------------------------------------------------------- |
| Server RPC | `server/validator_service.go` (ValidateProducerResponse, RegisterConsumer, etc.) |
| Storage    | `server/storage/` (memory, sqlite, postgres implementations)                     |
| Node.js    | `sdks/node/src/producer/testing.ts`                                              |
| Go         | `sdks/go/cvt/producer/testing.go`                                                |
| Python     | `sdks/python/cvt_sdk/producer/testing.py`                                        |
| Java       | `sdks/java/src/main/java/com/cvt/sdk/producer/ProducerTestKit.java`              |

---

## 8. Protobuf Code Generation

Regenerate protobuf code after modifying `api/protos/cvt.proto`.

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

**Note:** The Node.js SDK uses dynamic proto loading via `@grpc/proto-loader` and doesn't require code generation.

### Prerequisites for Code Generation

```bash
# Install protoc plugins for Go
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Python plugins are installed via uv sync in the Python SDK
# Java plugins are handled by the Gradle protobuf plugin
```

---

## 9. TLS/mTLS Development Setup

Generate self-signed certificates for local TLS development.

### Generate Certificates

```bash
# Generate certificates in ./certs directory
./tools/gen-certs.sh

# Or specify custom directory and domain
./tools/gen-certs.sh ./my-certs myapp.local
```

This creates:

- `ca.crt`, `ca.key` - Certificate Authority
- `server.crt`, `server.key` - Server certificate
- `client.crt`, `client.key` - Client certificate (for mTLS)

### Run Server with TLS

```bash
# TLS only (server authenticates to clients)
CVT_TLS_ENABLED=true \
CVT_TLS_CERT_FILE=./certs/server.crt \
CVT_TLS_KEY_FILE=./certs/server.key \
make run-server
```

### Run Server with mTLS

```bash
# mTLS (mutual authentication)
CVT_TLS_ENABLED=true \
CVT_TLS_CERT_FILE=./certs/server.crt \
CVT_TLS_KEY_FILE=./certs/server.key \
CVT_TLS_CA_FILE=./certs/ca.crt \
CVT_TLS_CLIENT_AUTH=require \
make run-server
```

### Docker with TLS

Uncomment TLS settings in `docker-compose.yml`:

```yaml
services:
  cvt-server:
    environment:
      - CVT_TLS_ENABLED=true
      - CVT_TLS_CERT_FILE=/certs/server.crt
      - CVT_TLS_KEY_FILE=/certs/server.key
    volumes:
      - ./certs:/certs:ro
```

### Testing TLS Connections

**Node.js SDK:**

```typescript
import { ContractValidator } from "@cvt/cvt-sdk";
import * as fs from "fs";

const validator = new ContractValidator({
  serverUrl: "localhost:9550",
  tls: {
    rootCerts: fs.readFileSync("./certs/ca.crt"),
    // For mTLS:
    privateKey: fs.readFileSync("./certs/client.key"),
    certChain: fs.readFileSync("./certs/client.crt"),
  },
});
```

**Python SDK:**

```python
from cvt_sdk import ContractValidator

validator = ContractValidator(
    server_url="localhost:9550",
    tls_root_certs=open("./certs/ca.crt", "rb").read(),
    # For mTLS:
    tls_private_key=open("./certs/client.key", "rb").read(),
    tls_cert_chain=open("./certs/client.crt", "rb").read(),
)
```

**Go SDK:**

```go
import (
    "crypto/tls"
    "crypto/x509"
    "os"
    "google.golang.org/grpc/credentials"
)

caCert, _ := os.ReadFile("./certs/ca.crt")
certPool := x509.NewCertPool()
certPool.AppendCertsFromPEM(caCert)

// For mTLS, also load client cert
clientCert, _ := tls.LoadX509KeyPair("./certs/client.crt", "./certs/client.key")

creds := credentials.NewTLS(&tls.Config{
    RootCAs:      certPool,
    Certificates: []tls.Certificate{clientCert}, // mTLS only
})

validator, _ := cvt.NewValidator(
    cvt.WithAddress("localhost:9550"),
    cvt.WithTransportCredentials(creds),
)
```

---

## 10. SDK Publishing Guide

### 10.1 Pre-publish Checklist

Before publishing any SDK:

- [ ] All tests passing (`make test`)
- [ ] Coverage meets 70% threshold
- [ ] Version bumped in configuration file
- [ ] CHANGELOG updated with release notes
- [ ] README documentation up to date
- [ ] Breaking changes documented
- [ ] License file present

### 10.2 Node.js SDK - Publishing to npm

**Update version:**

```bash
cd sdks/node
# Edit version in package.json
pnpm build
pnpm test
```

**Public Registry (npmjs.com):**

```bash
# Login to npm
npm login

# Publish scoped package
npm publish --access public
```

**Private Registry:**

Create or edit `.npmrc`:

```ini
@cvt:registry=https://npm.internal.example.com/
//npm.internal.example.com/:_authToken=${NPM_TOKEN}
```

```bash
npm publish
```

### 10.3 Python SDK - Publishing to PyPI

**Update version:**

```bash
cd sdks/python
# Edit version in pyproject.toml
uv sync
uv run pytest
```

**Public Registry (pypi.org):**

```bash
# Build distribution
uv build
# or: python -m build

# Upload to PyPI
twine upload dist/*
# or: uv publish
```

Configure `~/.pypirc` for credentials:

```ini
[pypi]
username = __token__
password = pypi-xxxx
```

**Private Registry:**

```bash
twine upload --repository-url https://pypi.internal.example.com/ dist/*
```

Or configure in `pyproject.toml`:

```toml
[[tool.uv.index]]
url = "https://pypi.internal.example.com/simple"
```

### 10.4 Go SDK - Publishing

**Update version and tag:**

```bash
cd sdks/go
# Ensure go.mod has correct module path

# Create version tag (use sdks/go/ prefix for subdirectory module)
git tag sdks/go/v1.2.0
git push --tags
```

**Public (GitHub + proxy.golang.org):**

The module is automatically indexed by the Go module proxy after pushing the tag. Users install with:

```bash
go get github.com/cvt/cvt-sdk/go@v1.2.0
```

**Private Repository:**

Users need to configure their environment:

```bash
# Set GOPRIVATE for private modules
export GOPRIVATE=github.com/your-org/*

# Configure Git credentials
git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"
```

### 10.5 Java SDK - Publishing to Maven

**Update version:**

```bash
cd sdks/java
# Edit version in build.gradle
./gradlew build
./gradlew test
```

**Local Maven (for testing):**

```bash
./gradlew publishToMavenLocal
```

**Maven Central:**

1. Configure Sonatype credentials in `~/.gradle/gradle.properties`:

```properties
sonatypeUsername=your-username
sonatypePassword=your-password
signing.keyId=your-key-id
signing.password=your-key-password
signing.secretKeyRingFile=/path/to/secring.gpg
```

1. Ensure `build.gradle` has publishing configuration:

```gradle
publishing {
    publications {
        mavenJava(MavenPublication) {
            from components.java
            // Add POM metadata
        }
    }
    repositories {
        maven {
            url = "https://oss.sonatype.org/service/local/staging/deploy/maven2/"
            credentials {
                username = sonatypeUsername
                password = sonatypePassword
            }
        }
    }
}
```

1. Publish:

```bash
./gradlew publish
```

**Private Maven Repository (Nexus/Artifactory):**

Configure in `build.gradle`:

```gradle
publishing {
    repositories {
        maven {
            url = "https://nexus.internal.example.com/repository/maven-releases/"
            credentials {
                username = System.getenv("NEXUS_USER")
                password = System.getenv("NEXUS_PASSWORD")
            }
        }
    }
}
```

```bash
./gradlew publish
```

---

## 11. Common Development Tasks

### Linting

```bash
# Lint all components
make lint

# Lint individual components
cd server && go fmt ./... && go vet ./...
cd sdks/node && pnpm lint
cd sdks/python && uv run ruff check .
cd sdks/java && ./gradlew check
```

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
make update-server          # Go dependencies
cd sdks/node && pnpm update
cd sdks/python && uv lock --upgrade
cd sdks/go && go get -u ./...
cd sdks/java && ./gradlew dependencyUpdates
```

### Viewing Logs

```bash
# Docker logs
make logs

# Follow logs
docker compose logs -f cvt-server
```

### Checking Status

```bash
# Container and health status
make status

# Health check
make health
```

### Cleaning Build Artifacts

```bash
# Clean all build outputs
make clean
```

---

## Troubleshooting

### Server won't start on port 9550

The port may be in use. The server auto-increments to find an available port, or specify a different one:

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

### Python SDK "module not found"

Ensure dependencies are installed:

```bash
cd sdks/python && uv sync
```

### Java SDK "Could not resolve dependencies"

For local development, ensure Maven local has the SDK:

```bash
cd sdks/java && ./gradlew publishToMavenLocal
```

### TLS handshake failures

Verify certificates are valid and paths are correct:

```bash
openssl x509 -in ./certs/server.crt -text -noout
```
