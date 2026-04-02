# Sources of Truth by Scope

This reference maps each audit scope to the implementation files that serve as the "source of truth" and the documentation files to audit.

## sdk

### Truth sources

- **Node.js SDK main**: `sdks/node/src/index.ts` — ContractValidator client class with all methods: registerSchema, validate, compareSchemas, generateFixture, generateResponse, generateRequestBody, listEndpoints, registerConsumer, listConsumers, deregisterConsumer, canIDeploy, close. Also: buildConsumerFromInteractions, registerConsumerFromInteractions
- **Node.js SDK types**: `sdks/node/src/index.ts` — Interfaces: TLSOptions, ContractValidatorOptions, ValidationRequest, ValidationResponse, ValidationResult, BreakingChange, CompareResult, GenerateOptions, GeneratedRequest, GeneratedResponse, GeneratedFixture, EndpointInfo, EndpointUsage, ConsumerInfo, RegisterConsumerOptions, ConsumerImpact, CanIDeployResult
- **Node.js SDK adapters**: `sdks/node/src/adapters/` — axios.ts, fetch.ts, mock.ts, types.ts, index.ts
- **Node.js SDK producer**: `sdks/node/src/producer/` — producer.ts, types.ts, testing.ts, index.ts; adapters: express.ts, fastify.ts
- **Node.js SDK auto-register**: `sdks/node/src/auto-register.ts`
- **Node.js package**: `sdks/node/package.json` — Name: @sahina/cvt-sdk, exports for main, adapters, producer

- **Python SDK main**: `sdks/python/cvt_sdk/__init__.py` — ContractValidator class with all methods: register_schema, validate, compare_schemas, generate_fixture, generate_response, generate_request_body, list_endpoints, register_consumer, list_consumers, deregister_consumer, can_i_deploy, close
- **Python SDK types**: `sdks/python/cvt_sdk/__init__.py` — Dataclasses: TLSOptions, ContractValidatorOptions, GenerateOptions, EndpointUsage, RegisterConsumerOptions. TypedDicts: ValidationRequest, ValidationResponse, ValidationResult, BreakingChange, CompareResult, GeneratedRequest, GeneratedResponse, GeneratedFixture, EndpointInfo, ConsumerInfo, ConsumerImpact, CanIDeployResult
- **Python SDK adapters**: `sdks/python/cvt_sdk/adapters/` — requests_adapter.py, mock_adapter.py, types.py
- **Python SDK producer**: `sdks/python/cvt_sdk/producer/` — producer.py, config.py, testing.py; adapters: fastapi.py, flask.py
- **Python SDK auto-register**: `sdks/python/cvt_sdk/auto_register.py`
- **Python package**: `sdks/python/pyproject.toml` — Name: cvt-sdk, Python >=3.11, dependencies: grpcio>=1.76.0, protobuf>=6.33.2

- **Go SDK main**: `sdks/go/cvt/validator.go` — ContractValidator client with exported types and methods: RegisterSchema, Validate, CompareSchemas, GenerateFixture, GenerateResponse, GenerateRequestBody, ListEndpoints, RegisterConsumer, ListConsumers, DeregisterConsumer, CanIDeploy, Close
- **Go SDK adapters**: `sdks/go/cvt/adapters/` — middleware.go, roundtripper.go, mocking.go, types.go
- **Go SDK producer**: `sdks/go/cvt/producer/` — producer.go, config.go, testing.go, errors.go, metrics.go; adapters: chi.go, gin.go, nethttp.go
- **Go SDK auto-register**: `sdks/go/cvt/auto_register.go`
- **Go package**: `sdks/go/go.mod`

- **Java SDK main**: `sdks/java/src/main/java/io/github/sahina/sdk/ContractValidator.java` — Builder pattern client
- **Java SDK types**: `sdks/java/src/main/java/io/github/sahina/sdk/` — ValidationRequest.java, ValidationResponse.java, ValidationResult.java, BreakingChange.java, CompareResult.java, GenerateOptions.java, GeneratedFixture.java, GeneratedRequest.java, GeneratedResponse.java, EndpointInfo.java, EndpointUsage.java, ConsumerInfo.java, RegisterConsumerOptions.java, ConsumerImpact.java, CanIDeployResult.java, AutoRegisterConfig.java, AutoRegisterUtils.java
- **Java SDK adapters**: `sdks/java/src/main/java/io/github/sahina/sdk/adapters/` — OkHttpContractAdapter.java, MockInterceptor.java, AdapterConfig.java, CapturedInteraction.java, MockConfig.java
- **Java SDK producer**: `sdks/java/src/main/java/io/github/sahina/sdk/producer/` — Producer.java, ProducerConfig.java, ProducerTestKit.java, ProducerValidationResult.java, TestRequestContext.java, TestResponseData.java, TestValidationResult.java, ValidationMode.java, Validator.java; adapters: ServletFilter.java, SpringInterceptor.java
- **Java package**: `sdks/java/pom.xml` — GroupId: io.github.sahina, ArtifactId: cvt-sdk, Version: 0.1.0

### Documentation to audit

- `sdks/node/README.md` — Node.js SDK documentation
- `sdks/python/README.md` — Python SDK documentation
- `sdks/go/README.md` — Go SDK documentation
- `sdks/java/README.md` — Java SDK documentation
- `docs/reference/sdk/index.mdx` — SDK overview doc
- `docs/reference/sdk/nodejs.md` — Node.js SDK reference
- `docs/reference/sdk/python.md` — Python SDK reference
- `docs/reference/sdk/go.md` — Go SDK reference
- `docs/reference/sdk/java.md` — Java SDK reference

### Key checks

- Verify all exported functions/types listed in docs actually exist in source
- Check function parameter types and shapes match across all 4 SDKs
- Verify import paths are correct (`@sahina/cvt-sdk`, `cvt_sdk`, `github.com/sahina/cvt/sdks/go/cvt`, `io.github.sahina.sdk`)
- Verify adapter patterns: Node (axios, fetch, mock), Python (requests, mock), Go (middleware, roundtripper, mock), Java (OkHttp, mock)
- Verify producer testing patterns: Node (express, fastify), Python (producer), Go (chi, gin, net/http), Java (servlet, spring)
- Check auto-registration and consumer registry documentation accuracy

---

## cli

### Truth sources

- **Command definitions**: `cmd/cvt/*.go` — each file defines a Cobra command with flags, args, and descriptions
- **Main/root command**: `cmd/cvt/main.go` — root command registration and version info
- **Commands**: validate.go, compare.go, generate.go, serve.go, can_i_deploy.go, wait.go, register_schema.go
- **Shared types**: `cmd/cvt/types.go` — shared CLI types and helpers
- **Flag defaults**: Check `cmd.Flags().StringVar()`, `BoolVar()`, etc. calls for default values

### Documentation to audit

- `docs/reference/cli.mdx` — Primary CLI reference
- `docs/getting-started/quick-start.mdx` — CLI usage in getting started
- `docs/getting-started/installation.mdx` — Build/install commands
- `docs/guides/ci-cd-integration.mdx` — CLI usage in CI/CD
- `docs/guides/breaking-changes.mdx` — `cvt compare` usage
- `CLAUDE.md` — CLI Commands section
- `cmd/cvt/examples/README.md` — CLI examples documentation
- Any guide that shows CLI commands (search for `cvt ` in `docs/`)

### Key checks

- Every documented command/subcommand exists in Cobra definitions
- Every Cobra command is documented (no missing commands)
- Flag names, types, and defaults match
- Command descriptions match
- Output format examples are accurate
- The 7 commands documented: validate, compare, generate, serve, can-i-deploy, wait, register-schema, version

---

## reference

### Truth sources

- **gRPC API**: `api/protos/cvt.proto` — ContractValidator service with 11 RPC methods, all message types, BreakingChangeType enum
- **Server implementation**: `server/cvtservice/validator_service.go` — RegisterSchema, ValidateInteraction, CompareSchemas, GenerateFixture, ListEndpoints, RegisterConsumer, ListConsumers, DeregisterConsumer, CanIDeploy, GetSchema, ListSchemas, ValidateProducerResponse
- **Configuration**: `server/storage/config.go` (storage env vars), `server/cvtservice/auth.go` (API key env vars), `server/cvtservice/tls.go` (TLS env vars), `cmd/cvt/serve.go` (core env vars)
- **Storage backends**: `server/storage/storage.go` (interface), `server/storage/memory.go`, `server/storage/sqlite/sqlite.go`, `server/storage/postgres/postgres.go`, `server/storage/factory.go` (backend factory)
- **Cache**: `server/cvtservice/cache.go` — Ristretto cache with 1000 max schemas, 24h TTL
- **Metrics**: `server/cvtservice/metrics.go` — Prometheus metrics
- **Interceptors**: `server/cvtservice/interceptors.go` — gRPC interceptors
- **Health**: `server/cvtservice/health.go` — gRPC health check
- **Embedded library**: `pkg/cvt/validator.go`, `pkg/cvt/compatibility.go`, `pkg/cvt/generator.go`
- **Architecture source files**: `server/cvtservice/` (all *.go), `server/storage/` (all *.go)

### Documentation to audit

- `docs/reference/api.mdx` — API reference
- `docs/reference/cli.mdx` — CLI reference
- `docs/reference/configuration.mdx` — Configuration reference
- `docs/reference/openapi-support.md` — OpenAPI version support matrix
- `docs/reference/architecture/index.md` — Architecture overview
- `docs/reference/architecture/validation-engine.md` — Validation engine design
- `docs/reference/architecture/storage-layer.md` — Storage layer architecture
- `docs/reference/architecture/consumer-registry.md` — Consumer registry design
- `docs/reference/architecture/sdk-architecture.md` — SDK architecture
- `docs/reference/sdk/index.mdx` — SDK overview
- `docs/reference/sdk/nodejs.md` — Node.js SDK reference
- `docs/reference/sdk/python.md` — Python SDK reference
- `docs/reference/sdk/go.md` — Go SDK reference
- `docs/reference/sdk/java.md` — Java SDK reference

### Key checks

- Every documented gRPC method exists in proto definition and server implementation
- Request/response shapes match proto message definitions
- BreakingChangeType enum values are complete and accurate
- Environment variables and defaults match config parsing code
- Storage backend configuration matches actual parsing
- Port numbers are correct (9550 gRPC, 9551 metrics, 9091 Prometheus, 3000 Grafana)
- OpenAPI v2/v3 support claims match kin-openapi capabilities

---

## guides

### Truth sources

- Feature implementations in `server/cvtservice/` corresponding to each guide topic
- SDK implementations for each guide's code examples
- Proto definitions for any API examples
- `pkg/cvt/` for CLI-based validation patterns

### Documentation to audit

- `docs/guides/consumer-testing.mdx` — Consumer-side testing guide
- `docs/guides/producer-testing.mdx` — Producer-side testing guide
- `docs/guides/breaking-changes.mdx` — Breaking changes detection guide
- `docs/guides/validation-modes.mdx` — Validation modes guide
- `docs/guides/ci-cd-integration.mdx` — CI/CD integration guide

### Key checks

- Code snippets use current SDK APIs for all shown languages
- Feature behavior descriptions match implementation
- Configuration examples use current env vars and defaults
- Consumer registry workflow is accurate (register consumer → validate → can-i-deploy)
- Breaking change detection aligns with compatibility_engine.go behavior

---

## getting-started

### Truth sources

- All SDK sources (see `sdk` scope)
- CLI commands (see `cli` scope)
- Server defaults: `cmd/cvt/serve.go`, `server/storage/config.go`
- Docker: `docker-compose.yml`, `Dockerfile`

### Documentation to audit

- `docs/intro.md` — Docusaurus landing page
- `docs/getting-started/installation.mdx` — Installation instructions
- `docs/getting-started/quick-start.mdx` — Quick start guide
- `docs/getting-started/faq.md` — FAQ page

### Key checks

- Install commands are current (package names, versions, registry URLs)
- Code examples use current API signatures
- Server URLs and ports match defaults (localhost:9550)
- Docker commands match docker-compose.yml
- Build commands (`make build`, `make up`, etc.) are accurate
- FAQ answers are technically correct and up to date

---

## ai-helper

### Truth sources

- SDK source files for API patterns
- CLI commands for command references
- `api/protos/cvt.proto` for API contract
- `.agents/skills/` for agent skill definitions

### Documentation to audit

- `docs/ai-helper/overview.md` — AI Helper overview
- `docs/ai-helper/context-templates.md` — Context templates for AI
- `docs/ai-helper/common-mistakes.md` — Common mistakes guide
- `docs/ai-helper/advanced-patterns.md` — Advanced patterns
- `docs/ai-helper/openapi-schema-generator.md` — OpenAPI schema generator

### Key checks

- API patterns and code examples use current SDK signatures
- Context templates reference correct file paths
- Common mistakes are still relevant
- Advanced patterns work with current implementation

---

## operations

### Truth sources

- `docker-compose.yml` — Docker services configuration
- `observability/` — Prometheus and Grafana configuration
- `server/cvtservice/metrics.go` — Prometheus metrics definitions
- `server/cvtservice/health.go` — Health check implementation
- `Makefile` — observability-related targets

### Documentation to audit

- `docs/operations/observability.md` — Observability setup guide

### Key checks

- Docker compose service names and ports are accurate
- Prometheus metrics names match server implementation
- Grafana dashboard setup instructions work
- Health check endpoints and commands are correct
- Makefile targets referenced are accurate (health, grafana, metrics, prometheus)

---

## development

### Truth sources

- `Makefile` — All build, test, lint, and release targets
- `.github/workflows/` — CI/CD workflows
- Root `go.mod`, SDK package files
- `CONTRIBUTING.md` — Contribution guidelines

### Documentation to audit

- `docs/development/contributing.md` — Detailed development guide
- `docs/development/releasing.md` — Release process guide
- `CONTRIBUTING.md` — Root contribution guidelines

### Key checks

- Build commands match Makefile targets
- Test commands are accurate (make test, make test-docker, per-SDK tests)
- Lint commands match (make lint, make ci)
- Release process matches actual workflow
- CI/CD workflow descriptions match .github/workflows/

---

## examples

### Truth sources

- Example source code: `examples/`, `cmd/cvt/examples/`, `sdks/*/examples/`
- Current SDK APIs (see `sdk` scope)
- Current CLI commands (see `cli` scope)

### Documentation to audit

- `examples/README.md` — Examples overview
- `cmd/cvt/examples/README.md` — CLI examples documentation
- `sdks/node/examples/` — Node.js SDK examples
- `sdks/python/examples/` — Python SDK examples
- `sdks/go/examples/` — Go SDK examples
- `sdks/java/src/main/java/io/github/sahina/examples/` — Java SDK examples

### Key checks

- README code snippets match actual example source code
- Dependency versions in READMEs match package manifests
- Build/run instructions work with current project state
- Feature claims match what the example actually demonstrates
- Example JSON fixtures are valid and match expected schema format

---

## internal

### Truth sources

- Implementation state across entire codebase
- Shipped features (check code existence)

### Documentation to audit

- `docs/internal/prd.md` — Product requirements document
- `docs/internal/adoption-strategy.md` — Internal adoption strategy
- `docs/design/schema-inference.md` — Schema inference design

### Key checks

- PRD items match what was actually built vs planned
- Adoption strategy reflects current distribution channels (npm, PyPI, Maven Central, pkg.go.dev, GHCR)
- Design documents reflect implemented behavior

---

## project

### Truth sources

- All of the above (broad scope)
- `Makefile` — build commands and targets
- Root `go.mod` — project metadata
- Feature implementations across entire codebase
- `docs-site/docusaurus.config.ts` — docs site configuration

### Documentation to audit

- `README.md` — Main project README
- `CLAUDE.md` — AI assistant guide
- `CONTRIBUTING.md` — Contribution guidelines
- `TODOS.md` — Project TODOs
- `server/README.md` — Server implementation docs

### Key checks

- README feature list matches implemented features
- Build/run commands match Makefile targets
- Architecture overview matches actual package structure
- CLAUDE.md patterns, key files, and commands match current implementation
- CLAUDE.md build/test/lint commands are current
- CLAUDE.md environment variables are complete and accurate
- CLAUDE.md CLI commands section is accurate
- CONTRIBUTING.md references are current

---

## ci-templates

### Truth sources

- `ci-templates/` — All CI/CD template files
- Current CLI commands (see `cli` scope)
- Current SDK APIs (see `sdk` scope)

### Documentation to audit

- `ci-templates/README.md` — CI/CD templates documentation
- `ci-templates/Jenkinsfile` — Jenkins pipeline definition
- `ci-templates/demo-consumer.yml` — Consumer demo workflow
- `ci-templates/demo-producer.yml` — Producer demo workflow
- `ci-templates/demo-can-i-deploy.yml` — Deployment safety demo

### Key checks

- CLI commands in templates match current cvt CLI
- SDK usage patterns in templates match current APIs
- Docker image references are correct (ghcr.io/sahina/cvt)
- Environment variables in templates match current server config
- Workflow steps are accurate and work with current implementation
