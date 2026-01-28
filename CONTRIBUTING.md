# Contributing to CVT

CVT (Contract Validator Toolkit) is an internal tool for consumer-based contract validation. We welcome contributions from all teams.

## Quick Start

For detailed development setup instructions, see the **[Development Guide](docs/development/contributing.md)**.

```bash
# Clone and setup
git clone <repo-url>
cd cvt
make build
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
gofmt -w .
golangci-lint run
```

### TypeScript (Node SDK)

- Use TypeScript strict mode
- Run ESLint and Prettier before committing

```bash
cd sdks/node
npm run lint
npm run format:check
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

```bash
cd sdks/java
./gradlew checkstyleMain
```

## Making Changes

### Modifying the Protocol (cvt.proto)

If you change `api/protos/cvt.proto`, regenerate code for all SDKs:

```bash
make generate           # Go server
make generate-go-sdk    # Go SDK
make generate-python    # Python SDK
make generate-java-sdk  # Java SDK
```

The Node SDK uses dynamic proto loading and doesn't require code generation.

### Adding a New Feature

1. **Server changes**: Update `server/` with the new functionality
2. **Proto changes**: If needed, update `api/protos/cvt.proto` and regenerate
3. **SDK changes**: Update all relevant SDKs to expose the feature
4. **Tests**: Add tests at all levels (server, SDK, integration)
5. **Documentation**: Update docs and examples

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

### Writing Good Tests

- Test both success and failure cases
- Use table-driven tests in Go
- Mock external dependencies
- Keep tests focused and readable

## Documentation

Update documentation when:

- Adding a new feature
- Changing behavior
- Making breaking changes

See the [docs/](docs/) directory for all documentation.

## Getting Help

- **Questions**: Reach out to @platform-team on Slack
- **Bugs**: Open an issue with reproduction steps
- **Feature requests**: Discuss with @platform-team before implementing

## Code Review

All changes require review from code owners. Reviews focus on:

- Correctness and test coverage
- Code style and consistency
- Performance implications
- Security considerations
- Documentation completeness

## Release Process

Releases are managed by @platform-team:

1. Changes merged to `main`
2. Version bumped in relevant files
3. Changelog updated
4. Tag created and pushed
5. CI builds and publishes artifacts
