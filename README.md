# Contract Validator Toolkit

<p align="center">
  <img src="assets/cvt-infographic.jpg" alt="CVT - Contract Validator Toolkit" style="border-radius: 12px;">
</p>

<p align="center">
  <a href="https://github.com/sahina/cvt/actions/workflows/ci.yml"><img src="https://github.com/sahina/cvt/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/sahina/cvt/actions/workflows/release.yml"><img src="https://github.com/sahina/cvt/actions/workflows/release.yml/badge.svg" alt="Release"></a>
  <a href="https://github.com/sahina/cvt/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT"></a>
  <a href="https://www.npmjs.com/package/@sahina/cvt-sdk"><img src="https://img.shields.io/npm/v/@sahina/cvt-sdk.svg" alt="npm"></a>
  <a href="https://pypi.org/project/cvt-sdk/"><img src="https://img.shields.io/pypi/v/cvt-sdk.svg" alt="PyPI"></a>
  <a href="https://pkg.go.dev/github.com/sahina/cvt/sdks/go"><img src="https://pkg.go.dev/badge/github.com/sahina/cvt/sdks/go.svg" alt="Go Reference"></a>
  <a href="https://central.sonatype.com/artifact/io.github.sahina/cvt-sdk"><img src="https://img.shields.io/maven-central/v/io.github.sahina/cvt-sdk.svg" alt="Maven Central"></a>
  <a href="https://ghcr.io/sahina/cvt-server"><img src="https://img.shields.io/badge/docker-ghcr.io-blue?logo=docker" alt="Docker"></a>
</p>

A contract validation platform for OpenAPI v2/v3 specifications that validates API requests and responses against API contracts. Supports both consumer-side (client) and producer-side (server) validation.

> **Examples**: For complete working examples with real-world usage patterns, see the [CVT Demo Repository](https://github.com/sahina/cvt-demo).

## Features

- **OpenAPI v2/v3 Support** - Validate against Swagger 2.0 and OpenAPI 3.0 specifications
- **Consumer Validation** - Validate outgoing API calls match the contract (client-side)
- **Producer Validation** - Validate incoming requests match the contract (server-side middleware)
- **Consumer Registry** - Track which consumers depend on which schemas
- **Deployment Safety** - `can-i-deploy` checks prevent breaking changes from reaching production
- **High Performance** - gRPC-based server with schema caching (1000 schemas, 24h TTL)
- **Multiple SDKs** - Node.js, Python, Go, and Java client libraries
- **Security** - TLS/mTLS support and API key authentication
- **Observability** - Prometheus metrics and Grafana dashboards built-in

## Contract Testing Overview

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Contract Testing with CVT                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  CONSUMER TESTING                        PRODUCER TESTING                   │
│  "Am I calling the API correctly?"       "Does my API match my spec?"       │
│                                                                             │
│  ┌─────────────┐      HTTP       ┌─────────────┐                            │
│  │ Your Service│ ──────────────► │ Upstream API│                            │
│  │ (Consumer)  │                 │ (Producer)  │                            │
│  └──────┬──────┘                 └─────────────┘                            │
│         │                                                                   │
│         │ Validate request/response                                         │
│         ▼                                                                   │
│  ┌─────────────┐                 ┌─────────────┐      HTTP       ┌────────┐ │
│  │ CVT Server  │                 │  Your API   │◄────────────────│ Client │ │
│  │ + Schema    │                 │ + Middleware│                 │        │ │
│  └─────────────┘                 └──────┬──────┘                 └────────┘ │
│                                         │                                   │
│                                         │ Validate request/response         │
│                                         ▼                                   │
│                                  ┌─────────────┐                            │
│                                  │ CVT Server  │                            │
│                                  │ + Schema    │                            │
│                                  └─────────────┘                            │
│                                                                             │
│  Use Cases:                              Use Cases:                         │
│  • Test your API integrations            • Runtime request validation       │
│  • Mock upstream APIs                    • Detect implementation drift      │
│  • Register as consumer                  • Deployment safety checks         │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Quick Start

```bash
# 1. Start CVT server + PostgreSQL + Prometheus + Grafana
make up

# 2. Verify gRPC server is accepting connections
make health

# 3. Run Node.js SDK example to see validation in action
make run-example

# 4. Stop all containers
make down
```

### Basic Usage

```typescript
import { ContractValidator } from "@sahina/cvt-sdk";

const validator = new ContractValidator("localhost:9550");
await validator.registerSchema("my-api", "./openapi.json");

const result = await validator.validate(
  { method: "GET", path: "/users/123" },
  { statusCode: 200, body: { id: 123, name: "Alice" } },
);

console.log(result.valid ? "Valid" : "Invalid");
```

## When to Use Each Approach

| Approach                | You Are...                 | You Want To...                                | Guide                                                |
| ----------------------- | -------------------------- | --------------------------------------------- | ---------------------------------------------------- |
| **Consumer Validation** | Calling another team's API | Validate your HTTP calls match their contract | [Consumer Testing](docs/guides/consumer-testing.mdx) |
| **Producer Middleware** | Exposing an API            | Reject invalid requests at runtime            | [Producer Testing](docs/guides/producer-testing.mdx) |
| **Deployment Safety**   | Changing your API          | Ensure changes won't break consumers          | [Breaking Changes](docs/guides/breaking-changes.mdx) |

## Key Commands

```bash
# Docker
make up                    # Start server + observability
make down                  # Stop all services
make health                # Check server health

# Testing
make test                  # Run all tests
make test-server           # Server unit tests only

# Development
make build                 # Build server and SDKs
make run-example           # Run Node.js example
```

Run `make help` for all available commands.

## CLI (Local Lite Mode)

Validate schemas locally without Docker.

### Install

Download a pre-built binary from [GitHub Releases](https://github.com/sahina/cvt/releases):

```bash
# macOS (Apple Silicon)
curl -L https://github.com/sahina/cvt/releases/latest/download/cvt-darwin-arm64 -o cvt

# macOS (Intel)
curl -L https://github.com/sahina/cvt/releases/latest/download/cvt-darwin-amd64 -o cvt

# Linux (x86_64)
curl -L https://github.com/sahina/cvt/releases/latest/download/cvt-linux-amd64 -o cvt

# Linux (ARM64)
curl -L https://github.com/sahina/cvt/releases/latest/download/cvt-linux-arm64 -o cvt

chmod +x cvt
sudo mv cvt /usr/local/bin/
```

For Windows, download `cvt-windows-amd64.exe` or `cvt-windows-arm64.exe` from the releases page.

Verify the installation:

```bash
cvt version
```

**Build from source** (requires Go 1.25+):

```bash
go build -o cvt ./cmd/cvt
```

### Usage

```bash
# Validate request/response against schema
cvt validate --schema ./openapi.json --request req.json --response resp.json

# Detect breaking changes between schema versions
cvt compare --old ./v1/openapi.json --new ./v2/openapi.json

# Check deployment safety
cvt can-i-deploy --schema my-api --version 2.0.0 --env prod
```

See [CLI Reference](docs/reference/cli.mdx) for all commands.

## Project Structure

```text
/
├── api/protos/       # gRPC proto definitions
├── cmd/cvt/          # CLI entry point
├── server/           # Go gRPC server
├── sdks/             # Client SDKs (Node.js, Python, Go, Java)
├── docs/             # Documentation
└── observability/    # Prometheus & Grafana configs
```

## Prerequisites

- **Docker & Docker Compose** (required)
- **Node.js 20+** (for Node.js SDK)
- **Optional**: Go 1.25+, Python 3.12+, Java 21+

## Documentation

### Getting Started

| Document                                              | Description              |
| ----------------------------------------------------- | ------------------------ |
| [Installation](docs/getting-started/installation.mdx) | Server and SDK setup     |
| [Quick Start](docs/getting-started/quick-start.mdx)   | Your first contract test |

### Guides

| Guide                                                | Description                      |
| ---------------------------------------------------- | -------------------------------- |
| [Consumer Testing](docs/guides/consumer-testing.mdx) | Validate your API integrations   |
| [Producer Testing](docs/guides/producer-testing.mdx) | Validate your API implementation |
| [Breaking Changes](docs/guides/breaking-changes.mdx) | Detect schema incompatibilities  |
| [Validation Modes](docs/guides/validation-modes.mdx) | Strict, warn, and shadow modes   |

### Reference

| Reference                                            | Description                  |
| ---------------------------------------------------- | ---------------------------- |
| [Architecture](docs/reference/architecture/index.md) | System design and components |
| [API Reference](docs/reference/api.mdx)              | gRPC API documentation       |
| [CLI Reference](docs/reference/cli.mdx)              | Command-line interface       |
| [Configuration](docs/reference/configuration.mdx)    | Environment variables        |
| [SDK Documentation](docs/reference/sdk/index.mdx)    | Language-specific SDK guides |

### Operations

| Document                                          | Description             |
| ------------------------------------------------- | ----------------------- |
| [Observability](docs/operations/observability.md) | Metrics and monitoring  |
| [Development](docs/development/contributing.md)   | Local development setup |

### For AI Agents / LLMs

- [llms.txt](llms.txt) - Index file with documentation links
- [llms-full.txt](llms-full.txt) - Complete API reference with examples

## Comparison with Alternatives

| Approach                      | Examples            | Problem                                  | How CVT Differs                              |
| ----------------------------- | ------------------- | ---------------------------------------- | -------------------------------------------- |
| **In-Process Libraries**      | Ajv, swagger-parser | Inconsistent validation across languages | Unified Go-based validator for all languages |
| **Consumer-Driven Contracts** | Pact                | Requires separate pact files and broker  | Uses existing OpenAPI specs directly         |
| **Runtime Gateways**          | Kong, Apigee        | Validation happens too late              | Shift-left: validates during development     |

## Troubleshooting

| Issue                      | Solution                                              |
| -------------------------- | ----------------------------------------------------- |
| "Connection refused"       | Ensure CVT server is running: `make up`               |
| "Schema not found"         | Check schema ID matches between register and validate |
| "Path not found in schema" | Verify the path exists in your OpenAPI spec           |

See [Development Guide](docs/development/contributing.md) for more troubleshooting tips.

## License

MIT License - see LICENSE file for details

---

**Built with**: [gRPC](https://grpc.io/), [kin-openapi](https://github.com/getkin/kin-openapi), [Ristretto](https://github.com/dgraph-io/ristretto), [Zap](https://github.com/uber-go/zap)
