# Schema Inference System — Design Document

**Status:** DRAFT v1  
**Date:** January 29, 2025  
**Author:** [Your Name]  
**Project:** CVT (Contract Validator Toolkit)

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Problem Statement](#problem-statement)
3. [Goals and Non-Goals](#goals-and-non-goals)
4. [Design Decisions](#design-decisions)
5. [Architecture Overview](#architecture-overview)
6. [Integration with CVT](#integration-with-cvt)
7. [Component Design](#component-design)
8. [Configuration](#configuration)
9. [Local Development Workflow](#local-development-workflow)
10. [Kubernetes Deployment](#kubernetes-deployment)
11. [Data Flow](#data-flow)
12. [Next Steps](#next-steps)
13. [Open Questions](#open-questions)
14. [Appendix](#appendix)

---

## Executive Summary

This document describes the design of a schema inference system that automatically generates OpenAPI specifications by observing production API traffic. The system integrates into the existing CVT (Contract Validator Toolkit) ecosystem, enabling organizations to bootstrap contract testing for APIs that lack formal schema definitions.

**Key value proposition:** APIs without schemas → Inferred schemas → Full contract testing capability.

---

## Problem Statement

### The Challenge

Many organizations have APIs in production that:

- Were built without OpenAPI specifications
- Have specifications that have drifted from actual implementation
- Have incomplete or outdated documentation

To adopt contract testing (which CVT provides), these APIs need schemas. Manually creating schemas is:

- Time-consuming and error-prone
- Difficult to keep synchronized with actual behavior
- Often deprioritized against feature work

### Industry Context

Current approaches to bootstrapping schemas:

| Approach                | How it works                              | Limitation                                                                   |
| ----------------------- | ----------------------------------------- | ---------------------------------------------------------------------------- |
| Traffic-based inference | Capture production traffic, infer schemas | Tools like Optic, APIClarity exist but don't integrate with contract testing |
| Code-first generation   | Generate from source code types           | Types may not reflect runtime behavior                                       |
| Manual documentation    | Engineers write OpenAPI by hand           | Time-consuming, drift-prone                                                  |

**Gap:** No existing solution captures traffic AND feeds directly into a contract testing workflow.

---

## Goals and Non-Goals

### Goals

1. **Capture API traffic** with minimal application changes
2. **Infer JSON Schemas** from observed request/response patterns
3. **Export OpenAPI 2.x/3.x** specifications from inferred schemas
4. **Integrate with CVT** so inferred schemas become immediately usable for contract validation
5. **Support Kubernetes** deployments via sidecar pattern
6. **Enable local development** without Kubernetes dependencies

### Non-Goals

1. **Replacing design-first API development** — This is for catching up, not replacing good practices
2. **Real-time validation** — Inference happens asynchronously, not inline with requests
3. **Full APM/observability** — We capture for schema inference, not general observability
4. **Multi-language SDK implementation** — Sidecar approach eliminates need for per-language SDKs

---

## Design Decisions

| Decision                 | Choice                                 | Rationale                                                                                                            |
| ------------------------ | -------------------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| **Capture approach**     | Go sidecar (explicit proxy)            | Single implementation, language-agnostic. Eliminates need to maintain Node, Python, Java, Go SDKs for capture.       |
| **Traffic interception** | Explicit proxy (HTTP_PROXY)            | Simpler than iptables-based transparent proxy. Easier debugging. Can add transparent mode later.                     |
| **Sampling strategy**    | Adapter pattern, head-based default    | Pluggable strategies without complexity. Head-based is simplest and sufficient for most cases.                       |
| **Inference location**   | Server-side only                       | Keep sidecar lightweight and fast. Heavy processing happens centrally.                                               |
| **PII handling**         | Server-side redaction                  | Middleware captures raw traffic; redaction happens before inference to centralize policy.                            |
| **Buffering**            | In-memory + periodic flush             | Start simple. Transport adapters allow future options (Kafka, SQS) without sidecar changes.                          |
| **Type conflicts**       | Union types                            | When a field appears as both string and integer, represent as `type: [string, integer]`. Industry standard approach. |
| **Required threshold**   | 95% (configurable)                     | Field present in 95%+ of observations = required. Balances accuracy with handling of optional fields.                |
| **Schema format**        | JSON Schema internally, OpenAPI export | JSON Schema is flexible for inference; OpenAPI is the contract testing standard.                                     |
| **K8s injection**        | Manual first, webhook later            | Start simple with manual sidecar injection. Add mutating webhook as adoption grows.                                  |

---

## Architecture Overview

### High-Level Architecture

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                    RUNTIME ENVIRONMENT (K8s Pod or Docker)                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────────────┐         ┌─────────────────────────────────────┐   │
│   │                     │         │                                     │   │
│   │   Application       │         │   CVT Capture Sidecar (Go)          │   │
│   │                     │◀───────▶│                                     │   │
│   │   • Node            │  proxy  │   • Reverse proxy                   │   │
│   │   • Python          │         │   • Sampling (pluggable)            │   │
│   │   • Java            │         │   • Capture middleware              │   │
│   │   • Go              │         │   • Buffer + flush                  │   │
│   │                     │         │                                     │   │
│   └─────────────────────┘         └───────────────┬─────────────────────┘   │
│             │                                     │                         │
│             │ HTTP_PROXY=localhost:9080           │                         │
│             │                                     │                         │
└─────────────┼─────────────────────────────────────┼─────────────────────────┘
              │                                     │
              │ (outbound to external services)     │ (captured traffic via gRPC)
              ▼                                     ▼
        ┌───────────┐                 ┌─────────────────────────────────────┐
        │ External  │                 │         CVT Server (Extended)       │
        │ Services  │                 │                                     │
        └───────────┘                 │  EXISTING          NEW              │
                                      │  ┌──────────────┐ ┌───────────────┐ │
                                      │  │ Validation   │ │ Inference     │ │
                                      │  │ Service      │ │ Service       │ │
                                      │  │              │ │               │ │
                                      │  │ • Schema     │ │ • Redaction   │ │
                                      │  │   validation │ │ • Inference   │ │
                                      │  │ • Consumer   │ │ • Export      │ │
                                      │  │   registry   │ │ • Promote     │ │
                                      │  │ • can-i-     │ │               │ │
                                      │  │   deploy     │ │               │ │
                                      │  └──────────────┘ └───────────────┘ │
                                      │                                     │
                                      └─────────────────────────────────────┘
```

### Sidecar Components

```text
┌────────────────────────────────────────────────────────────────┐
│                  CVT CAPTURE SIDECAR (Go)                      │
├────────────────────────────────────────────────────────────────┤
│                                                                │
│   ┌───────────────────────────────────────────────────────┐    │
│   │                  Reverse Proxy Core                   │    │
│   │            (net/http/httputil.ReverseProxy)           │    │
│   └───────────────────────┬───────────────────────────────┘    │
│                           │                                    │
│           ┌───────────────┼───────────────┐                    │
│           ▼               ▼               ▼                    │
│   ┌───────────┐   ┌───────────┐   ┌───────────────┐            │
│   │ Sampling  │   │  Capture  │   │    Buffer     │            │
│   │ Strategy  │   │ Middleware│   │   + Flush     │            │
│   │ (adapter) │   │           │   │   (adapter)   │            │
│   └───────────┘   └───────────┘   └───────────────┘            │
│                                                                │
│   Sampling Strategies:          Transport Adapters:            │
│   • HeadBased (default)         • gRPC (default)               │
│   • TailBased                   • HTTP                         │
│   • Adaptive                    • File (debugging)             │
│   • EndpointAware                                              │
│                                                                │
└────────────────────────────────────────────────────────────────┘
```

### Server Components

```text
┌───────────────────────────────────────────────────────────────┐
│                    CVT SERVER (Extended)                      │
├───────────────────────────────────────────────────────────────┤
│                                                               │
│   ┌────────────────────────────────────────────────────────   │
│   │                  INFERENCE SERVICE (NEW)               │  │
│   ├────────────────────────────────────────────────────────┤  │
│   │                                                        │  │
│   │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐ │  │
│   │  │    PII      │  │   Schema    │  │  Inference      │ │  │
│   │  │  Redaction  │─▶│  Inference  │─▶│  Store          │ │  │
│   │  └─────────────┘  └─────────────┘  └────────┬────────┘ │  │
│   │                                             │          │  │
│   │  ┌─────────────┐  ┌─────────────────────────▼────────┐ │  │
│   │  │   Export    │  │  Conflict Resolution             │ │  │
│   │  │  Adapters   │◀─│  • Union types for type conflicts│ │  │
│   │  │             │  │  • 95% threshold for required    │ │  │
│   │  │ • OpenAPI 3 │  └──────────────────────────────────┘ │  │
│   │  │ • OpenAPI 2 │                                       │  │
│   │  └──────┬──────┘                                       │  │
│   │         │                                              │  │
│   └─────────┼──────────────────────────────────────────────┘  │
│             │                                                 │
│             │ Promote                                         │
│             ▼                                                 │
│   ┌─────────────────────────────────────────────────────────┐ │
│   │              EXISTING CVT SERVICE                       │ │
│   │                                                         │ │
│   │  • Schema validation    • Consumer registry             │ │
│   │  • Breaking changes     • can-i-deploy                  │ │
│   │                                                         │ │
│   └─────────────────────────────────────────────────────────┘ │
│                                                               │
└───────────────────────────────────────────────────────────────┘
```

---

## Integration with CVT

### Directory Structure

```text
cvt/
├── api/
│   └── protos/
│       ├── cvt.proto              # Existing
│       └── inference.proto        # NEW: Schema inference service definitions
│
├── cmd/
│   ├── cvt/                       # Existing CLI
│   │   └── main.go
│   └── cvt-capture/               # NEW: Capture sidecar binary
│       └── main.go
│
├── pkg/
│   ├── cvt/                       # Existing validation library
│   └── inference/                 # NEW: Inference library (shared logic)
│       ├── schema.go              # JSON Schema generation
│       ├── merge.go               # Schema merging & conflict resolution
│       ├── redaction.go           # PII redaction
│       └── export/                # Export adapters
│           ├── openapi3.go
│           └── openapi2.go
│
├── server/
│   ├── cvtservice/                # Existing validation service
│   └── inferenceservice/          # NEW: Inference service
│       ├── service.go             # gRPC service implementation
│       ├── ingest.go              # Traffic ingestion handler
│       └── store.go               # Inferred schema storage
│
├── capture/                       # NEW: Sidecar components
│   ├── proxy/                     # Reverse proxy core
│   │   └── proxy.go
│   ├── sampling/                  # Sampling strategies (adapters)
│   │   ├── strategy.go            # Interface
│   │   ├── head_based.go
│   │   ├── tail_based.go
│   │   ├── adaptive.go
│   │   └── endpoint_aware.go
│   ├── buffer/                    # Buffering + flush
│   │   └── buffer.go
│   └── transport/                 # Transport adapters
│       ├── transport.go           # Interface
│       ├── grpc.go                # gRPC transport (default)
│       ├── http.go                # HTTP transport
│       └── file.go                # File transport (debugging)
│
├── sdks/                          # Existing - NO CHANGES NEEDED
│   ├── node/
│   ├── python/
│   ├── go/
│   └── java/
│
├── deploy/                        # NEW: Kubernetes deployment
│   ├── kubernetes/
│   │   ├── capture-sidecar/       # Sidecar injection examples
│   │   │   ├── configmap.yaml
│   │   │   └── example-deployment.yaml
│   │   └── inference-server/      # If separate from main server
│   └── helm/
│       └── cvt/                   # Extended Helm chart
│
├── examples/
│   ├── ...                        # Existing examples
│   └── schema-inference/          # NEW: Inference examples
│       ├── docker-compose.yaml    # Local dev setup
│       ├── docker-compose.k8s-like.yaml
│       ├── sample-app/            # Test application
│       └── README.md
│
├── docker-compose.yml             # Extended to include capture components
└── Makefile                       # Extended with inference targets
```

### CLI Extensions

```bash
# Existing CVT commands (unchanged)
cvt validate --schema ./openapi.json --request req.json --response resp.json
cvt compare --old ./v1/openapi.json --new ./v2/openapi.json
cvt can-i-deploy --schema user-api --version 2.0.0 --env prod
cvt serve --port 9550

# NEW: Schema inference commands
cvt infer status --service orders-api           # Check inference status
cvt infer export --service orders-api --format openapi3 --output ./inferred.yaml
cvt infer promote --service orders-api --schema-id orders-api-v1
cvt infer list                                   # List all services with inferred schemas
cvt infer clear --service orders-api            # Clear inferred data
```

### The Promotion Flow

Once schemas are inferred, they can be **promoted** to registered schemas, making them immediately usable for all CVT capabilities:

```text
┌────────────────────────────────────────────────────────────────────────────┐
│                    SCHEMA INFERENCE → CONTRACT VALIDATION                  │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│   1. CAPTURE                    2. INFER                  3. PROMOTE       │
│   ┌─────────────────┐          ┌─────────────────┐       ┌───────────────┐ │
│   │                 │          │                 │       │               │ │
│   │  Sidecar proxy  │─────────▶│  CVT Server     │──────▶│  Register as  │ │
│   │  captures       │ traffic  │  infers JSON    │ export│  official     │ │
│   │  production     │          │  Schema, merges │       │  schema       │ │
│   │  traffic        │          │  conflicts      │       │               │ │
│   │                 │          │                 │       └───────┬───────┘ │
│   └─────────────────┘          └─────────────────┘               │         │
│                                                                  │         │
│                                                                  ▼         │
│   ┌─────────────────────────────────────────────────────────────────────┐  │
│   │                     EXISTING CVT CAPABILITIES                       │  │
│   │                                                                     │  │
│   │  ✓ Contract validation      ✓ Breaking change detection             │  │
│   │  ✓ Consumer registry        ✓ can-i-deploy                          │  │
│   │  ✓ Producer middleware      ✓ SDK validation adapters               │  │
│   │                                                                     │  │
│   └─────────────────────────────────────────────────────────────────────┘  │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
```

---

## Component Design

### Sampling Strategy Interface

```go
// capture/sampling/strategy.go
package sampling

type RequestContext struct {
    Method      string
    Path        string
    StatusCode  int
    LatencyMs   int64
    Error       error
    Headers     map[string]string
}

type Strategy interface {
    // ShouldSample decides whether to capture this request
    ShouldSample(ctx RequestContext) bool

    // Name returns the strategy identifier
    Name() string
}

// Implementations:
// - HeadBasedStrategy: Random sampling at configured rate
// - TailBasedStrategy: Sample based on response status/latency
// - AdaptiveStrategy: Adjusts rate based on traffic volume
// - EndpointAwareStrategy: Different rates per endpoint pattern
```

### Transport Adapter Interface

```go
// capture/transport/transport.go
package transport

type CapturedTraffic struct {
    ServiceName string
    Endpoint    Endpoint
    Request     HTTPRequest
    Response    HTTPResponse
    Metadata    Metadata
}

type Transport interface {
    // Send transmits a batch of captured traffic
    Send(ctx context.Context, batch []CapturedTraffic) error

    // Name returns the transport identifier
    Name() string

    // Close cleanly shuts down the transport
    Close() error
}

// Implementations:
// - GRPCTransport: Sends to CVT server via gRPC (default)
// - HTTPTransport: Sends to CVT server via HTTP/REST
// - FileTransport: Writes to local files (debugging/testing)
```

### Inference Engine Interface

```go
// pkg/inference/inferencer.go
package inference

type InferenceConfig struct {
    RequiredThreshold  float64  // Default: 0.95
    ConflictStrategy   string   // "union" | "majority" | "flag"
}

type Inferencer interface {
    // Process incorporates new traffic observation
    Process(traffic *CapturedTraffic) error

    // GetSchema returns current inferred schema for a service
    GetSchema(serviceName string) (*JSONSchema, error)

    // GetEndpoints lists all inferred endpoints for a service
    GetEndpoints(serviceName string) ([]Endpoint, error)

    // ExportOpenAPI3 generates OpenAPI 3.x specification
    ExportOpenAPI3(serviceName string) ([]byte, error)

    // ExportOpenAPI2 generates OpenAPI 2.x (Swagger) specification
    ExportOpenAPI2(serviceName string) ([]byte, error)
}
```

### Captured Traffic Structure

```json
{
  "serviceName": "orders-api",
  "endpoint": {
    "method": "POST",
    "path": "/api/users/{id}/orders",
    "rawPath": "/api/users/123/orders"
  },
  "request": {
    "headers": {
      "content-type": "application/json",
      "accept": "application/json"
    },
    "body": "{ \"item\": \"widget\", \"qty\": 5 }",
    "contentType": "application/json",
    "contentLength": 42
  },
  "response": {
    "status": 201,
    "headers": {
      "content-type": "application/json"
    },
    "body": "{ \"orderId\": \"abc123\", \"status\": \"created\" }",
    "contentType": "application/json"
  },
  "metadata": {
    "timestamp": "2025-01-29T15:30:00Z",
    "latencyMs": 45,
    "podName": "orders-api-7d8f9-abc",
    "namespace": "production"
  }
}
```

---

## Configuration

### CVT Server Configuration (Extended)

```yaml
# config/cvt.yaml

# Existing CVT configuration
server:
  grpcPort: 9550
  metricsPort: 9551

storage:
  type: postgres
  postgres:
    host: postgres
    port: 5432
    database: cvt
    user: cvt
    password: ${CVT_DB_PASSWORD}

cache:
  maxSchemas: 1000
  ttlHours: 24

# NEW: Schema inference configuration
inference:
  enabled: true

  redaction:
    enabled: true
    patterns:
      - name: email
        regex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
      - name: credit-card
        regex: "\\b\\d{4}[- ]?\\d{4}[- ]?\\d{4}[- ]?\\d{4}\\b"
      - name: ssn
        regex: "\\b\\d{3}-\\d{2}-\\d{4}\\b"
      - name: phone
        regex: "\\b\\d{3}[-.]?\\d{3}[-.]?\\d{4}\\b"
    blockedFields:
      - password
      - passwd
      - token
      - secret
      - authorization
      - api_key
      - apikey
      - access_token
      - refresh_token
      - private_key

  schema:
    requiredThreshold: 0.95 # 95% presence = required field
    conflictStrategy: union # union | majority | flag
    maxDepth: 10 # Max nesting depth to infer
    maxArraySamples: 100 # Max array items to analyze

  storage:
    retentionDays: 30 # How long to keep inferred schemas
    maxServicesCount: 1000 # Max services to track
```

### Capture Sidecar Configuration

```yaml
# config/capture.yaml

proxy:
  listenPort: 9080
  upstreamUrl: "${UPSTREAM_URL}"
  readTimeoutSeconds: 30
  writeTimeoutSeconds: 30

sampling:
  strategy: head-based # head-based | tail-based | adaptive | endpoint-aware
  rate: 0.10 # 10% sampling rate

  strategies:
    head-based:
      rate: 0.10

    tail-based:
      sampleAllErrors: true # Always capture 4xx/5xx responses
      latencyThresholdMs: 1000 # Capture requests > 1s
      errorRate: 1.0 # 100% of errors
      successRate: 0.05 # 5% of successful requests

    adaptive:
      minRate: 0.01 # Never go below 1%
      maxRate: 0.50 # Never exceed 50%
      targetRequestsPerMinute: 100

    endpoint-aware:
      defaultRate: 0.10
      rules:
        - pattern: "/api/checkout/*"
          rate: 0.50 # Higher rate for critical paths
        - pattern: "/api/payments/*"
          rate: 0.50
        - pattern: "/health"
          rate: 0.0 # Never capture health checks
        - pattern: "/ready"
          rate: 0.0
        - pattern: "/metrics"
          rate: 0.0

capture:
  includeRequestHeaders: true
  includeResponseHeaders: true
  maxBodySize: 1048576 # 1MB - skip larger payloads
  excludePaths:
    - /health
    - /ready
    - /metrics
    - /favicon.ico
  excludeContentTypes:
    - image/*
    - video/*
    - audio/*
    - application/octet-stream

buffer:
  maxSize: 1000 # Max items before force flush
  flushIntervalSeconds: 30 # Periodic flush interval

transport:
  type: grpc # grpc | http | file

  grpc:
    address: "${CVT_SERVER}:9550"
    timeoutSeconds: 5
    retries: 3
    retryBackoffMs: 100

  http:
    endpoint: "${CVT_SERVER}:9551/api/v1/ingest"
    timeoutSeconds: 5
    batchSize: 100

  file:
    directory: /var/log/cvt-capture
    rotateSize: 100MB
    maxFiles: 10

observability:
  metrics:
    enabled: true
    port: 9091
  logging:
    level: info # debug | info | warn | error
    format: json # json | text
```

---

## Local Development Workflow

### Development Lifecycle

```text
┌────────────────────────────────────────────────────────────────────────────┐
│                         DEVELOPMENT LIFECYCLE                              │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│   ┌─────────────┐     ┌─────────────┐     ┌─────────────────────────┐      │
│   │   Local     │     │   Docker    │     │      Kubernetes         │      │
│   │  Processes  │────▶│   Compose   │────▶│    (staging/prod)       │      │
│   └─────────────┘     └─────────────┘     └─────────────────────────┘      │
│                                                                            │
│   • Fast iteration    • Integration      • Manual injection first          │
│   • Debugger attach     testing          • Webhook injection later         │
│   • Unit tests        • CI/CD pipeline   • Helm chart                      │
│                       • E2E tests                                          │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
```

### Option 1: Local Processes (No Containers)

Best for rapid iteration and debugging.

```bash
# Terminal 1: CVT Server
cd server && go run ./cmd/server --config ../config/cvt.yaml

# Terminal 2: Capture Sidecar
cd capture && go run ./cmd/cvt-capture \
    --upstream=http://localhost:8080 \
    --listen=:9080 \
    --cvt-server=localhost:9550 \
    --sampling-rate=1.0

# Terminal 3: Sample Application
HTTP_PROXY=http://localhost:9080 go run ./examples/schema-inference/sample-app/main.go
```

### Option 2: Docker Compose (Integration Testing)

```yaml
# examples/schema-inference/docker-compose.yaml
version: "3.8"

services:
  cvt-server:
    build:
      context: ../..
      dockerfile: server/Dockerfile
    ports:
      - "9550:9550"
      - "9551:9551"
    environment:
      - CVT_INFERENCE_ENABLED=true
    depends_on:
      - postgres

  postgres:
    image: postgres:15-alpine
    environment:
      - POSTGRES_DB=cvt
      - POSTGRES_USER=cvt
      - POSTGRES_PASSWORD=cvt
    volumes:
      - pgdata:/var/lib/postgresql/data

  capture-sidecar:
    build:
      context: ../..
      dockerfile: capture/Dockerfile
    ports:
      - "9080:9080"
      - "9091:9091"
    environment:
      - UPSTREAM_URL=http://localhost:8080
      - CVT_SERVER=cvt-server:9550
      - SAMPLING_RATE=1.0
      - FLUSH_INTERVAL_SECONDS=5

  sample-app:
    build: ./sample-app
    environment:
      - HTTP_PROXY=http://localhost:9080
      - PORT=8080
    network_mode: "service:capture-sidecar"
    depends_on:
      - capture-sidecar

volumes:
  pgdata:
```

### Option 3: Docker Compose with K8s-like Networking

Simulates Kubernetes pod networking where containers share localhost.

```yaml
# examples/schema-inference/docker-compose.k8s-like.yaml
version: "3.8"

services:
  capture-sidecar:
    build:
      context: ../..
      dockerfile: capture/Dockerfile
    environment:
      - UPSTREAM_URL=http://localhost:8080
      - CVT_SERVER=cvt-server:9550
      - LISTEN_PORT=9080
    ports:
      - "9080:9080"
      - "8080:8080"
    networks:
      - infra

  sample-app:
    build: ./sample-app
    network_mode: "service:capture-sidecar" # Shares network namespace
    environment:
      - HTTP_PROXY=http://localhost:9080
      - PORT=8080
    depends_on:
      - capture-sidecar

  cvt-server:
    build:
      context: ../..
      dockerfile: server/Dockerfile
    ports:
      - "9550:9550"
    networks:
      - infra

networks:
  infra:
    driver: bridge
```

### Makefile Targets

```makefile
# Makefile (additions)

.PHONY: dev-inference up-inference up-inference-k8s test-inference

# Run inference components as local processes
dev-inference:
 @echo "Starting CVT server..."
 @(cd server && go run ./cmd/server) &
 @sleep 2
 @echo "Starting capture sidecar..."
 @(cd capture && go run ./cmd/cvt-capture \
  --upstream=http://localhost:8080 \
  --listen=:9080 \
  --cvt-server=localhost:9550 \
  --sampling-rate=1.0) &
 @sleep 1
 @echo "Starting sample app..."
 @HTTP_PROXY=http://localhost:9080 go run ./examples/schema-inference/sample-app/main.go

# Docker Compose (standard)
up-inference:
 docker-compose -f examples/schema-inference/docker-compose.yaml up --build

# Docker Compose (K8s-like networking)
up-inference-k8s:
 docker-compose -f examples/schema-inference/docker-compose.k8s-like.yaml up --build

# Run inference tests
test-inference:
 ./scripts/test-inference.sh

# Build capture sidecar
build-capture:
 go build -o bin/cvt-capture ./cmd/cvt-capture

# Build Docker images
docker-build-capture:
 docker build -t cvt-capture:local -f capture/Dockerfile .
```

### Test Script

```bash
#!/bin/bash
# scripts/test-inference.sh

set -e

BASE_URL="http://localhost:8080"
CVT_URL="http://localhost:9550"

echo "=== Generating traffic ==="

for i in {1..20}; do
    # Consistent schema endpoints
    curl -s "$BASE_URL/api/users" > /dev/null
    curl -s -X POST "$BASE_URL/api/users" \
        -H "Content-Type: application/json" \
        -d '{"name":"Test User","email":"test@example.com"}' > /dev/null

    # Variable type endpoint (tests union types)
    curl -s "$BASE_URL/api/items" > /dev/null
    curl -s "$BASE_URL/api/items?format=string" > /dev/null

    # Optional field endpoint (tests required threshold)
    curl -s "$BASE_URL/api/orders" > /dev/null
    curl -s "$BASE_URL/api/orders?verbose=true" > /dev/null
done

echo "=== Waiting for flush ==="
sleep 10

echo "=== Checking inferred schemas ==="
cvt infer list

echo "=== Exporting OpenAPI 3 ==="
cvt infer export --service sample-app --format openapi3 --output /tmp/inferred.yaml
cat /tmp/inferred.yaml

echo "=== Promoting to registered schema ==="
cvt infer promote --service sample-app --schema-id sample-app-inferred-v1

echo "=== Validating against inferred schema ==="
cvt validate --schema sample-app-inferred-v1 \
    --request '{"method":"GET","path":"/api/users"}' \
    --response '{"statusCode":200,"body":"[{\"id\":1,\"name\":\"Test\"}]"}'

echo "=== Done ==="
```

---

## Kubernetes Deployment

### Manual Sidecar Injection

```yaml
# deploy/kubernetes/capture-sidecar/example-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: orders-api
  labels:
    app: orders-api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: orders-api
  template:
    metadata:
      labels:
        app: orders-api
    spec:
      containers:
        # Application container
        - name: app
          image: my-registry/orders-api:latest
          ports:
            - containerPort: 8080
          env:
            - name: HTTP_PROXY
              value: "http://localhost:9080"
            - name: HTTPS_PROXY
              value: "http://localhost:9080"
            - name: NO_PROXY
              value: "localhost,127.0.0.1,.cluster.local"
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "512Mi"
              cpu: "500m"

        # CVT Capture Sidecar
        - name: cvt-capture
          image: my-registry/cvt-capture:latest
          ports:
            - containerPort: 9080
              name: proxy
            - containerPort: 9091
              name: metrics
          env:
            - name: UPSTREAM_URL
              value: "http://localhost:8080"
            - name: CVT_SERVER
              value: "cvt-server.observability.svc.cluster.local:9550"
            - name: SERVICE_NAME
              value: "orders-api"
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
          volumeMounts:
            - name: capture-config
              mountPath: /etc/cvt-capture
          resources:
            requests:
              memory: "64Mi"
              cpu: "50m"
            limits:
              memory: "128Mi"
              cpu: "100m"
          livenessProbe:
            httpGet:
              path: /health
              port: 9091
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /ready
              port: 9091
            initialDelaySeconds: 5
            periodSeconds: 5

      volumes:
        - name: capture-config
          configMap:
            name: cvt-capture-config
```

### ConfigMap

```yaml
# deploy/kubernetes/capture-sidecar/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cvt-capture-config
  namespace: default
data:
  config.yaml: |
    proxy:
      listenPort: 9080
      upstreamUrl: "${UPSTREAM_URL}"

    sampling:
      strategy: head-based
      rate: 0.10

    capture:
      maxBodySize: 1048576
      excludePaths:
        - /health
        - /ready
        - /metrics

    buffer:
      maxSize: 1000
      flushIntervalSeconds: 30

    transport:
      type: grpc
      grpc:
        address: "${CVT_SERVER}"
```

### Future: Mutating Webhook (Automatic Injection)

```yaml
# Label namespace or pod for automatic sidecar injection
apiVersion: v1
kind: Namespace
metadata:
  name: my-app
  labels:
    cvt-capture.io/inject: "true"
```

**Note:** Webhook implementation is deferred. Start with manual injection to validate the approach before automating.

---

## Data Flow

### End-to-End Flow

```text
┌──────────────────────────────────────────────────────────────────────────────┐
│                              DATA FLOW                                       │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  1. REQUEST ARRIVES                                                          │
│     ┌─────────┐      ┌─────────────────┐      ┌─────────────┐                │
│     │ Client  │─────▶│ Capture Sidecar │─────▶│ Application │                │
│     └─────────┘      └────────┬────────┘      └──────┬──────┘                │
│                               │                      │                       │
│  2. RESPONSE RETURNS          │                      │                       │
│     ┌─────────┐      ┌────────┴────────┐      ┌──────┴──────┐                │
│     │ Client  │◀─────│ Capture Sidecar │◀─────│ Application │                │
│     └─────────┘      └────────┬────────┘      └─────────────┘                │
│                               │                                              │
│  3. SAMPLING DECISION         │                                              │
│                               ▼                                              │
│                      ┌─────────────────┐                                     │
│                      │ Should sample?  │                                     │
│                      │ (strategy)      │                                     │
│                      └────────┬────────┘                                     │
│                               │                                              │
│                        Yes ───┴─── No                                        │
│                         │          │                                         │
│                         ▼          ▼                                         │
│  4. BUFFER             Add to    Discard                                     │
│                       buffer                                                 │
│                         │                                                    │
│                         ▼                                                    │
│  5. FLUSH (periodic or full)                                                 │
│                      ┌─────────────────┐                                     │
│                      │ Send batch to   │                                     │
│                      │ CVT Server      │                                     │
│                      └────────┬────────┘                                     │
│                               │                                              │
│                               ▼                                              │
│  6. SERVER PROCESSING                                                        │
│     ┌─────────────────────────────────────────────────────────────────┐      │
│     │  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────────┐  │      │
│     │  │ Redact   │──▶│ Infer    │──▶│ Merge    │──▶│ Store        │  │      │
│     │  │ PII      │   │ Schema   │   │ Schemas  │   │ Schema       │  │      │
│     │  └──────────┘   └──────────┘   └──────────┘   └──────────────┘  │      │
│     └─────────────────────────────────────────────────────────────────┘      │
│                                                                              │
│  7. EXPORT & PROMOTE                                                         │
│     ┌──────────────────┐                                                     │
│     │ cvt infer export │──▶ OpenAPI 3.x / 2.x                                │
│     │ cvt infer promote│──▶ Registered schema (usable for validation)        │
│     └──────────────────┘                                                     │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Schema Inference Logic

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                         SCHEMA INFERENCE LOGIC                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  For each observed traffic:                                                 │
│                                                                             │
│  1. PARSE BODY                                                              │
│     JSON string ──▶ Parsed structure                                        │
│                                                                             │
│  2. INFER TYPES                                                             │
│     {"id": 123, "name": "foo"} ──▶ {id: integer, name: string}              │
│                                                                             │
│  3. MERGE WITH EXISTING                                                     │
│     ┌────────────────────────────────────────────────────────────┐          │
│     │ Existing schema       New observation       Result         │          │
│     ├────────────────────────────────────────────────────────────┤          │
│     │ id: integer           id: integer           id: integer    │          │
│     │ name: string          name: string          name: string   │          │
│     │ count: integer        count: string         count: [int,   │          │
│     │                                               string]      │          │
│     │ email: required       email: missing        email: optional│          │
│     │ (100% presence)       (now 95%)             (< threshold)  │          │
│     └────────────────────────────────────────────────────────────┘          │
│                                                                             │
│  4. TRACK STATISTICS                                                        │
│     - Total observations per endpoint                                       │
│     - Field presence counts (for required/optional)                         │
│     - Type occurrence counts (for conflict detection)                       │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Next Steps

The following areas need detailed design and implementation:

### 1. Protobuf Definitions

- [ ] Define `inference.proto` with all messages and service methods
- [ ] Define error codes and status responses
- [ ] Plan for backward compatibility

### 2. Capture Sidecar Implementation

- [ ] Implement reverse proxy core (`capture/proxy/`)
- [ ] Implement sampling strategy interface and adapters
- [ ] Implement buffer with flush logic
- [ ] Implement transport adapters (gRPC, HTTP, file)
- [ ] Add metrics and health endpoints

### 3. Inference Engine

- [ ] Implement JSON Schema inference from JSON payloads
- [ ] Implement schema merging with conflict resolution
- [ ] Implement union type handling
- [ ] Implement required/optional threshold logic
- [ ] Add support for nested objects and arrays

### 4. Path Parameterization

- [ ] Detect `/users/123` vs `/users/456` as `/users/{id}`
- [ ] Handle multiple path parameters
- [ ] Handle edge cases (UUIDs, dates, etc.)

### 5. PII Redaction

- [ ] Implement regex-based pattern matching
- [ ] Implement field-name blocklist
- [ ] Add support for custom patterns
- [ ] Consider NER-based detection (future)

### 6. Server Integration

- [ ] Implement `inferenceservice` package
- [ ] Integrate with existing storage layer
- [ ] Implement promotion to registered schemas
- [ ] Add CLI commands

### 7. OpenAPI Export

- [ ] Implement JSON Schema to OpenAPI 3.x conversion
- [ ] Implement JSON Schema to OpenAPI 2.x conversion
- [ ] Handle edge cases (refs, allOf, etc.)

### 8. Testing

- [ ] Unit tests for each component
- [ ] Integration tests with sample app
- [ ] Performance benchmarks

### 9. Documentation

- [ ] User guide for schema inference
- [ ] Configuration reference
- [ ] Kubernetes deployment guide
- [ ] Troubleshooting guide

---

## Open Questions

1. **Path parameterization heuristics:** What algorithm should we use to detect path parameters? Options include:
   - Cardinality-based (high cardinality segments are likely params)
   - Pattern-based (UUIDs, integers, etc.)
   - User-provided hints

2. **Schema versioning:** How do we handle schema evolution over time? Should we:
   - Keep historical versions
   - Track when schemas change
   - Alert on significant changes

3. **Multi-service aggregation:** If the sidecar captures traffic to multiple upstream services, how do we:
   - Separate schemas by service
   - Handle service discovery

4. **Webhook injection scope:** When we add the mutating webhook:
   - Namespace-level opt-in?
   - Pod-level annotations?
   - Global with opt-out?

5. **Storage limits:** How do we handle:
   - Maximum schema size
   - Maximum number of services
   - Retention and cleanup policies

---

## Appendix

### A. Sampling Strategy Comparison

| Strategy       | Best For                          | Overhead   | Coverage              |
| -------------- | --------------------------------- | ---------- | --------------------- |
| Head-based     | General purpose, predictable load | Lowest     | Uniform               |
| Tail-based     | Error analysis, latency debugging | Medium     | Biased to interesting |
| Adaptive       | Variable traffic patterns         | Medium     | Dynamic               |
| Endpoint-aware | Critical path monitoring          | Low-Medium | Targeted              |

### B. Industry Precedents

| Tool            | Approach                        | Difference from CVT                       |
| --------------- | ------------------------------- | ----------------------------------------- |
| Optic           | Proxy-based capture + inference | Not integrated with contract testing      |
| APIClarity      | Service mesh integration        | Requires Istio/Envoy, focused on security |
| Akita (Postman) | Traffic capture                 | Commercial, not open source               |

### C. Related CVT Documentation

- [Consumer Testing Guide](docs/guides/consumer-testing.mdx)
- [Producer Testing Guide](docs/guides/producer-testing.mdx)
- [Breaking Changes Guide](docs/guides/breaking-changes.mdx)
- [Server README](server/README.md)

---

## Document History

| Version  | Date       | Author   | Changes       |
| -------- | ---------- | -------- | ------------- |
| Draft v1 | 2025-01-29 | [Author] | Initial draft |

---

_This is a living document. Updates will be made as design decisions are refined and implementation progresses._
