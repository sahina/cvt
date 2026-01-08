# ContractValidator SDK Examples

Example code demonstrating the ContractValidator Go SDK. For full SDK documentation, see [../README.md](../README.md).

## Quick Start

1. **Start the server** (from repository root):

   ```bash
   make up
   ```

2. **Run an example** (from `sdks/go` directory):

   ```bash
   go run examples/basic/main.go
   ```

## Examples

### basic/main.go

Introduction to the SDK covering:

- Schema registration from local files
- Basic request/response validation
- Type safety with Go structs
- Error handling

```bash
go run examples/basic/main.go
```

### advanced/main.go

Advanced scenarios including:

- Nested object validation
- Batch validation (multiple requests)
- Enum validation
- Using helper functions from `shared.go`
- Swagger 2.0 support notes

```bash
go run examples/advanced/main.go
```

### breaking/main.go

Breaking change detection between schema versions:

- Register multiple schema versions
- Compare versions for breaking changes
- Detect removed endpoints, added required fields, type changes
- CI/CD integration patterns

```bash
go run examples/breaking/main.go
```

### shared.go

Common utilities shared across examples:

- Type definitions for Petstore API entities (Pet, User, Order, etc.)
- Helper functions for creating sample data
- Schema path constants
- Validation result logging utilities

## Troubleshooting

**Server connection errors?** Ensure the server is running: `make up` from repository root.

**Import errors?** Install dependencies: `go mod tidy` from the `sdks/go` directory.
