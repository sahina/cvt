# ContractValidator SDK Examples

Example code demonstrating the ContractValidator SDK. For full SDK documentation, see [../README.md](../README.md).

## Quick Start

1. **Start the server** (from repository root):

   ```bash
   make up
   ```

2. **Run an example** (from `sdks/python` directory):

   ```bash
   uv run python examples/basic_usage.py
   ```

## Examples

### basic_usage.py

Introduction to the SDK covering:

- Schema registration from local files
- Basic request/response validation
- Type hints for better IDE support
- Error handling

```bash
uv run python examples/basic_usage.py
```

### advanced_usage.py

Advanced scenarios including:

- Nested object validation
- Batch validation (multiple requests)
- Enum validation
- Using helper functions from `shared.py`
- Swagger 2.0 support notes

```bash
uv run python examples/advanced_usage.py
```

### breaking_changes.py

Breaking change detection between schema versions:

- Register multiple schema versions
- Compare versions for breaking changes
- Detect removed endpoints, added required fields, type changes
- CI/CD integration patterns

```bash
uv run python examples/breaking_changes.py
```

### shared.py

Common utilities shared across examples:

- Type definitions for Petstore API entities (Pet, User, Order, etc.)
- Helper functions for creating sample data
- Schema path constants
- Validation result logging utilities

## Troubleshooting

**Server connection errors?** Ensure the server is running: `make up` from repository root.

**Import errors?** Install dependencies: `uv sync` from the `sdks/python` directory.
