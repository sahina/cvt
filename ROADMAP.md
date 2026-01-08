# CVT Roadmap

This document outlines the planned development phases for CVT (Contract Validator Toolkit).

## Vision

CVT aims to be the standard internal tool for API contract testing, covering both consumer and producer validation with OpenAPI specifications as the source of truth.

> **Related document:** See [Adoption Strategy](docs/adoption-strategy.md) for organizational rollout plans, team onboarding, and adoption metrics. This roadmap focuses on technical delivery; the adoption strategy covers organizational change management.

---

## Phase Overview

| Phase   | Name                 | Status                 |
| ------- | -------------------- | ---------------------- |
| Phase 1 | Consumer Validation  | ✅ Complete            |
| Phase 2 | Developer Experience | ✅ Complete            |
| Phase 3 | Producer Validation  | ✅ Complete            |
| Phase 4 | Platform Features    | ⏳ Under Consideration |

---

## ✅ Phase 1 - Consumer Validation

### Status: ✅ Complete

Consumer-based contract testing is fully implemented and production-ready.

### Delivered Features

- ✅ **gRPC Server**
  - ✅ Schema registration and caching (Ristretto, 1000 schemas, 24h TTL)
  - ✅ Request/response validation against OpenAPI v2/v3 specs
  - ✅ Automatic Swagger 2.0 to OpenAPI 3.0 conversion
  - ✅ Breaking change detection between schema versions
  - ✅ TLS/mTLS support
  - ✅ API key authentication
  - ✅ Prometheus metrics and structured logging

- ✅ **SDKs** (Node.js, Python, Go, Java)
  - ✅ Schema registration and validation APIs
  - ✅ HTTP adapters for automatic request/response capture
  - ✅ Breaking change detection APIs
  - ✅ Comprehensive examples and documentation

- ✅ **CLI**
  - ✅ Local validation without server (`cvt validate`)
  - ✅ Schema comparison (`cvt compare`)
  - ✅ Local server mode (`cvt serve`)

- ✅ **Infrastructure**
  - ✅ Docker Compose setup with observability stack
  - ✅ GitHub Actions CI/CD pipeline
  - ✅ Grafana dashboards and Prometheus metrics

### Success Criteria

- ✅ All 4 SDKs production-ready with documentation
- ✅ Breaking change detection working
- ✅ Observability stack operational

---

## ✅ Phase 2 - Developer Experience

### Status: ✅ Complete

Improved developer experience for consumer contract testing, especially for teams without immediate API access.

### Delivered Features

- ✅ **Test fixture generator**
  - ✅ Generate sample request/response JSON from OpenAPI schemas
  - ✅ Eliminates manual construction of test data
  - ✅ Solves the "Testing Without API Access" problem (README Use Case 4)

  **CLI commands:**

  ```bash
  # List all endpoints in a schema
  cvt generate --schema openapi.json --list

  # Generate complete fixture for an endpoint
  cvt generate --schema openapi.json --method GET --path /users/123

  # Generate response only
  cvt generate --schema openapi.json --method GET --path /users/123 --output-type response

  # Generate request body only
  cvt generate --schema openapi.json --method POST --path /users --output-type request
  ```

  **SDK methods (all 4 SDKs):**

  ```typescript
  // Generate complete fixture (request + response)
  const fixture = await validator.generateFixture("POST", "/users", {
    statusCode: 201,
    useExamples: true,
  });

  // Generate response only
  const response = await validator.generateResponse("GET", "/users/123");

  // Generate request body only
  const body = await validator.generateRequestBody("POST", "/users");

  // List all endpoints
  const endpoints = await validator.listEndpoints();
  ```

  **Use case:** Enables contract testing without manually writing JSON:

  ```typescript
  it("handles user response", async () => {
    const response = await validator.generateResponse("GET", "/users/{id}");
    const result = await validator.validate(
      { method: "GET", path: "/users/123" },
      { statusCode: response.statusCode, body: JSON.stringify(response.body) },
    );
    expect(result.valid).toBe(true);
  });
  ```

- ✅ **CI/CD integration templates**
  - ✅ GitHub Action for one-line integration
  - ✅ GitLab CI template
  - ✅ Jenkins pipeline example
  - ✅ Comprehensive documentation in `ci-templates/README.md`

### Success Criteria

- ✅ Test fixture generator available in CLI and Node.js/Python SDKs
- ✅ CI/CD templates published and documented
- ✅ "Testing Without API Access" workflow fully supported

### SDK Feature Parity

- ✅ Go SDK: Fixture generation methods
- ✅ Java SDK: HTTP adapters (OkHttp interceptor)
- ✅ Java SDK: Fixture generation methods

---

## ✅ Phase 3 - Producer Validation

### Status: ✅ Complete

Producer-based validation ensures API implementations conform to their published contracts. This capability targets API owners (producers), enabling them to validate incoming requests and outgoing responses against their OpenAPI schemas.

### Delivered Features

- ✅ **Server-side request validation**
  - ✅ Validate incoming requests match the OpenAPI spec
  - ✅ Reject malformed requests before they reach business logic
  - ✅ Return standardized error responses for contract violations

- ✅ **Middleware/interceptors for each SDK**
  - ✅ Go: HTTP middleware for net/http, Gin, and Chi
  - ✅ Node.js: Express middleware and Fastify plugin
  - ✅ Python: ASGI middleware (FastAPI/Starlette) and WSGI middleware (Flask)
  - ✅ Java: Servlet filter and Spring HandlerInterceptor

- ✅ **Validation modes**
  - ✅ **Strict**: Reject non-conforming requests (production enforcement)
  - ✅ **Warn**: Log violations but allow requests (gradual rollout)
  - ✅ **Shadow**: Validate asynchronously, record metrics only (monitoring)

- ✅ **Response validation (producer side)**
  - ✅ Validate outgoing responses match the spec
  - ✅ Catch implementation drift before consumers notice

### Usage Examples

**Go (net/http):**

```go
import "github.com/cvt/cvt/sdks/go/cvt/producer"
import "github.com/cvt/cvt/sdks/go/cvt/producer/adapters"

config := producer.Config{
    SchemaID:  "my-api",
    Validator: myValidator,
    Mode:      producer.ModeStrict,
}
http.Handle("/", adapters.NetHTTPMiddleware(config)(myHandler))
```

**Node.js (Express):**

```typescript
import { createExpressMiddleware } from "@cvt/cvt-sdk/producer";

app.use(
  createExpressMiddleware({
    schemaId: "my-api",
    validator,
    mode: "strict",
  }),
);
```

**Python (FastAPI):**

```python
from cvt_sdk.producer import ProducerConfig, ValidationMode
from cvt_sdk.producer.adapters import ASGIMiddleware

config = ProducerConfig(
    schema_id="my-api",
    validator=validator,
    mode=ValidationMode.STRICT,
)
app.add_middleware(ASGIMiddleware, config=config)
```

**Java (Spring):**

```java
ProducerConfig config = ProducerConfig.builder()
    .schemaId("my-api")
    .validator(myValidator)
    .mode(ValidationMode.STRICT)
    .build();

registry.addInterceptor(new SpringInterceptor(config))
    .addPathPatterns("/api/**");
```

### Success Criteria

- ✅ Producer middleware available for all 4 SDKs
- ✅ Clear migration guide from consumer-only to full contract testing

---

## ⏳ Phase 4 - Platform Features

### Status: ⏳ Under Consideration

Platform capabilities to support broader adoption and operational needs.

### Planned Features

Features are ordered by dependency (each builds on the previous):

- ⏳ **Persistence layer** (prerequisite for UI)
  - ⏳ Schema versioning and history
  - ⏳ Validation run storage
  - ⏳ Environment tagging (dev, staging, prod)
  - ⏳ "Can I deploy" compatibility checks

- ⏳ **REST API** (required for browser access)
  - ⏳ HTTP/JSON wrapper around gRPC methods
  - ⏳ OpenAPI spec for the CVT API itself
  - ⏳ Endpoints: `/api/schemas`, `/api/validate`, `/api/compare`, `/api/generate`

- ⏳ **Admin dashboard** (React SPA embedded in Go server)
  - ⏳ Schema registry browser with upload, versioning, and search
  - ⏳ Interactive validation tester (request/response editor)
  - ⏳ Fixture generator with endpoint explorer
  - ⏳ Breaking change analyzer with visual diff
  - ⏳ Validation history and analytics dashboard

### Architecture Decisions

| Decision           | Choice                          | Rationale                          |
| ------------------ | ------------------------------- | ---------------------------------- |
| Deployment         | Embedded React SPA in Go server | Single binary, simple deployment   |
| Persistence        | Required for UI                 | Enables history, analytics         |
| Validation trigger | Button click                    | Predictable UX, fewer server calls |

### Decision Criteria

These features will be prioritized based on:

- Adoption metrics from Phase 1, 2, and 3
- Feedback from early adopter teams
- Operational pain points encountered

---

## ❌ Out of Scope (For Now)

The following are explicitly not planned for near-term development:

| Feature                 | Reason                                                             |
| ----------------------- | ------------------------------------------------------------------ |
| gRPC contract testing   | Focus on REST/HTTP APIs first; may revisit based on demand         |
| GraphQL support         | Requires different validation approach; plugin architecture needed |
| AsyncAPI support        | Event-driven contracts are a separate problem space                |
| Public/external release | Internal tool; no plans for open-source release                    |
| AI-powered features     | Nice-to-have but not core to contract testing value                |

---

## Timeline

We do not commit to specific dates. Phases progress based on:

- Completion of current phase deliverables
- Adoption and feedback from internal teams
- Available engineering capacity

### Phase Progression Criteria

| Phase   | Entry Criteria                                                     |
| ------- | ------------------------------------------------------------------ |
| Phase 2 | ✅ Phase 1 adopted by 2-3 teams; feedback incorporated             |
| Phase 3 | ✅ Phase 2 complete; demand for producer validation identified     |
| Phase 4 | ⏳ Phase 3 adopted by 5+ teams; clear demand for platform features |

---

## Feedback

This roadmap evolves based on user needs. To provide input:

- **Feature requests**: Discuss with @platform-team
- **Priority feedback**: Share which features would unblock your team
- **Pain points**: Report friction in current implementation

---

## Changelog

| Date    | Change                                                                          |
| ------- | ------------------------------------------------------------------------------- |
| 2025-01 | Initial roadmap created                                                         |
| 2025-01 | Phase 1 marked complete                                                         |
| 2025-01 | Added test fixture generator                                                    |
| 2025-01 | Added cross-reference to adoption strategy                                      |
| 2025-01 | Expanded test fixture generator with SDK methods and CLI                        |
| 2025-01 | Reorganized phases: DX (Phase 2), Producer (Phase 3), Platform (Phase 4)        |
| 2025-01 | Phase 2 test fixture generator implemented and marked complete                  |
| 2025-01 | Added CI/CD templates for GitHub Actions, GitLab CI, and Jenkins                |
| 2026-01 | SDK Feature Parity complete: Go/Java fixture generation, Java OkHttp adapter    |
| 2026-01 | Phase 3 Producer Validation complete: middleware for all 4 SDKs                 |
| 2026-01 | Removed Phase 4 UI implementation; platform features remain under consideration |
