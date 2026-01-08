# ContractValidator SDK Examples

Example code demonstrating the ContractValidator SDK. For full SDK documentation, see [../README.md](../README.md).

## Quick Start

1. **Start the server** (from repository root):

   ```bash
   make up
   ```

2. **Run an example** (from `sdks/node` directory):

   ```bash
   npx ts-node examples/basic-usage.ts
   ```

## Examples

### basic-usage.ts

Introduction to the SDK covering:

- Schema registration from local files
- Basic request/response validation
- TypeScript type safety
- Error handling

```bash
npx ts-node examples/basic-usage.ts
```

### advanced-usage.ts

Advanced scenarios including:

- Nested object validation
- Batch validation (multiple requests)
- Enum validation
- Using helper functions from `shared.ts`
- Swagger 2.0 support notes

```bash
npx ts-node examples/advanced-usage.ts
```

### breaking-changes.ts

Breaking change detection between schema versions:

- Register multiple schema versions
- Compare versions for breaking changes
- Detect removed endpoints, added required fields, type changes
- CI/CD integration patterns

```bash
npx ts-node examples/breaking-changes.ts
```

## Troubleshooting

**Server connection errors?** Ensure the server is running: `make up` from repository root.

**TypeScript errors?** Rebuild the SDK: `npm run build`
