# Contributing to CVT

CVT (Contract Validator Toolkit) is an internal tool for consumer-based contract validation. We welcome contributions from all teams.

## Getting Started

### Prerequisites

- Go 1.25+
- Docker and Docker Compose
- Node.js 18+ and pnpm (for Node SDK)
- Python 3.11+ and uv (for Python SDK)
- Java 17+ and Gradle (for Java SDK)

### Setup

```bash
# Clone the repository
git clone <repo-url>
cd cvt

# Start the server and dependencies
make up

# Verify everything is working
make health
make test
```

## Development Workflow

### 1. Create a Branch

```bash
git checkout -b feat/your-feature-name
# or
git checkout -b fix/your-bug-fix
```

### 2. Make Your Changes

Follow the code style guidelines below. Run tests locally before pushing.

### 3. Run Tests

```bash
# Run all tests (fast, direct server - no Docker required)
make test

# Run all tests with Docker (includes PostgreSQL integration)
make test-docker

# Run specific test suites
make test-server      # Go server tests
make test-node-sdk    # Node.js SDK tests
make test-python-sdk  # Python SDK tests
make test-go-sdk      # Go SDK tests
make test-java-sdk    # Java SDK tests

# Run with coverage
make test-coverage
```

### 4. Submit a Pull Request

- Provide a clear description of the change
- Reference any related issues
- Ensure CI passes

## Code Style

### Go (Server, CLI, Go SDK)

- Run `gofmt` before committing
- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Use meaningful variable names
- Add tests for new functionality

```bash
# Format Go code
gofmt -w .

# Run linter
golangci-lint run
```

### TypeScript (Node SDK)

- Use TypeScript strict mode
- Run ESLint and Prettier before committing
- Follow existing patterns in the codebase

```bash
cd sdks/node
pnpm lint
pnpm format
```

### Python (Python SDK)

- Use type hints
- Run Ruff for linting and formatting
- Follow PEP 8 conventions

```bash
cd sdks/python
uv run ruff check .
uv run ruff format .
```

### Java (Java SDK)

- Follow Google Java Style Guide
- Run Checkstyle before committing
- Use builder pattern for public APIs

```bash
cd sdks/java
./gradlew checkstyleMain
```

## Project Structure

```shell
cvt/
├── api/protos/          # gRPC protocol definitions
├── assets/              # Static assets (images, diagrams)
├── certs/               # TLS certificates
├── ci-templates/        # CI/CD pipeline templates
├── cmd/cvt/             # CLI application
├── config/              # Configuration files
├── docs/                # Documentation
├── examples/            # Example code and schemas
├── internal/            # Internal packages (not exported)
├── observability/       # Prometheus/Grafana configuration
├── pkg/cvt/             # Embedded Go library
├── server/              # gRPC server implementation
│   ├── cvtservice/      # Core service logic
│   └── storage/         # Persistence backends
├── sdks/
│   ├── go/              # Go SDK
│   ├── java/            # Java SDK
│   ├── node/            # Node.js SDK
│   ├── python/          # Python SDK
│   └── shared/          # Shared test schemas
└── tools/               # Build and test scripts
```

## Making Changes

### Modifying the Protocol (cvt.proto)

If you change `api/protos/cvt.proto`, you must regenerate code for all SDKs:

```bash
make generate           # Go server
make generate-go-sdk    # Go SDK
make generate-python    # Python SDK
make generate-java-sdk  # Java SDK
```

The Node SDK uses `ts-proto` and regenerates automatically during build.

### Adding a New Feature

1. **Server changes**: Update `server/` with the new functionality
2. **Proto changes**: If needed, update `api/protos/cvt.proto` and regenerate
3. **SDK changes**: Update all relevant SDKs to expose the feature
4. **Tests**: Add tests at all levels (server, SDK, integration)
5. **Documentation**: Update READMEs and examples

### Fixing a Bug

1. **Write a failing test** that reproduces the bug
2. **Fix the bug** in the relevant code
3. **Verify the test passes**
4. **Check for similar issues** in other SDKs if applicable

## Testing Guidelines

### Coverage Requirements

- Server: 70% minimum coverage
- SDKs: 70% minimum coverage
- All new code should include tests

### Test Categories

| Type              | Location                           | Command                 |
| ----------------- | ---------------------------------- | ----------------------- |
| Unit tests        | `*_test.go`, `*.test.ts`, etc.     | `make test-<component>` |
| Integration tests | `server/` with `-tags=integration` | `make test-integration` |
| End-to-end        | Via Docker Compose                 | `make up && make test`  |

### Writing Good Tests

- Test both success and failure cases
- Use table-driven tests in Go
- Mock external dependencies
- Keep tests focused and readable

## Documentation

### When to Update Docs

- Adding a new feature: Update relevant SDK README and add examples
- Changing behavior: Update affected documentation
- Breaking changes: Document in PR and update migration guide

### Documentation Locations

| Content           | Location                         |
| ----------------- | -------------------------------- |
| Getting started   | `README.md`                      |
| SDK usage         | `sdks/<sdk>/README.md`           |
| Architecture      | `docs/prd.md`                    |
| Development setup | `docs/DEVELOPMENT.md`            |
| Observability     | `docs/OBSERVABILITY.md`          |
| Adoption strategy | `docs/adoption-strategy.md`      |
| Producer testing  | `docs/producer-testing.md`       |
| Consumer testing  | `docs/consumer-testing-guide.md` |
| Use cases         | `docs/use-cases.md`              |
| Sequence diagrams | `docs/sequence-diagrams.md`      |
| Operating modes   | `docs/modes.md`                  |

## Getting Help

- **Questions**: Reach out to @platform-team on Slack
- **Bugs**: Open an issue with reproduction steps
- **Feature requests**: Discuss with @platform-team before implementing

## Code Review

All changes require review from code owners (see `CODEOWNERS`). Reviews focus on:

- Correctness and test coverage
- Code style and consistency
- Performance implications
- Security considerations
- Documentation completeness

## Release Process

Releases are managed by @platform-team. The process:

1. Changes merged to `main`
2. Version bumped in relevant files
3. Changelog updated
4. Tag created and pushed
5. CI builds and publishes artifacts
