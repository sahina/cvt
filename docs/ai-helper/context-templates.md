---
title: Context Templates
sidebar_label: Context Templates
sidebar_position: 2
description: Copy-paste context file templates for AI coding agents
---

# Context Templates

Context files help AI coding agents understand your project and provide accurate assistance. These templates configure your AI tool to use CVT's documentation effectively.

## Why Context Files Matter

AI coding agents work best when they have:

- **Project context**: What your project does and how it's structured
- **Domain knowledge**: CVT's APIs, patterns, and best practices
- **Code conventions**: Your team's coding standards

Context files provide this information upfront, so you don't need to explain it in every conversation.

---

## Claude Code (CLAUDE.md)

Create a `CLAUDE.md` file in your project root:

```markdown
# Project Context

## CVT Integration

This project uses CVT (Contract Validator Toolkit) for API contract testing.

### Documentation References

- CVT Summary: https://raw.githubusercontent.com/sahina/cvt/main/llms.txt
- CVT Full Reference: https://raw.githubusercontent.com/sahina/cvt/main/llms-full.txt
- Consumer Testing Guide: https://sahina.github.io/cvt/docs/guides/consumer-testing

### CVT Server

- Address: localhost:9550
- Start with: `make up` or `docker run -d -p 9550:9550 ghcr.io/cvt/cvt-server:latest`

### SDK Usage

We use the [Node.js/Python/Go/Java] SDK. Key patterns:

1. Register schemas before validation
2. Use `validate()` for request/response validation
3. Register as consumer for deployment safety checks

### Contract Test Location

- Schema files: `./contracts/`
- Contract tests: `./tests/contract/`
```

---

## Cursor (.cursorrules)

Create a `.cursorrules` file in your project root:

```text
# CVT Contract Testing Context

This project uses CVT for API contract testing against OpenAPI schemas.

## Documentation

Fetch CVT documentation from:
- https://raw.githubusercontent.com/sahina/cvt/main/llms.txt (summary)
- https://raw.githubusercontent.com/sahina/cvt/main/llms-full.txt (full reference)

## Key CVT Patterns

1. Always register the OpenAPI schema before validation
2. Use the SDK's validate() method - don't write validation logic manually
3. Run the real CVT server (localhost:9550) - don't mock it
4. Use generateFixture() for test data instead of hand-crafting JSON

## Project Structure

- OpenAPI schemas: ./contracts/
- Contract tests: ./tests/contract/
- CVT server: localhost:9550

## SDK

Using [Node.js/Python/Go/Java] SDK. Import from:
- Node.js: @sahina/cvt-sdk
- Python: cvt_sdk
- Go: github.com/sahina/cvt/sdks/go/cvt
- Java: com.cvt.sdk
```

---

## GitHub Copilot

Create a `.github/copilot-instructions.md` file:

```markdown
# CVT Contract Testing

This project uses CVT for API contract testing.

## Documentation

- Summary: https://raw.githubusercontent.com/sahina/cvt/main/llms.txt
- Full reference: https://raw.githubusercontent.com/sahina/cvt/main/llms-full.txt

## Patterns

When writing contract tests:

1. Register schema first: `validator.registerSchema('api-name', schemaContent)`
2. Validate interactions: `validator.validate(request, response)`
3. Check result.valid and result.errors

## Don't

- Don't write custom validation logic - use CVT's validate()
- Don't mock the CVT server - run the real server
- Don't hand-craft test fixtures - use generateFixture()
```

---

## Generic Template

For other AI tools or as a conversation starter:

```text
I'm working on a project that uses CVT (Contract Validator Toolkit) for API contract testing.

CVT validates HTTP request/response interactions against OpenAPI schemas via a gRPC server.

Documentation:
- Summary: https://raw.githubusercontent.com/sahina/cvt/main/llms.txt
- Full reference: https://raw.githubusercontent.com/sahina/cvt/main/llms-full.txt
- Consumer testing guide: https://sahina.github.io/cvt/docs/guides/consumer-testing

Key APIs:
- registerSchema(schemaId, content) - Register an OpenAPI schema
- validate(request, response) - Validate an interaction
- registerConsumer(...) - Register as a consumer for deployment checks
- canIDeploy(...) - Check if schema changes are safe to deploy

Server: localhost:9550 (gRPC)
```

---

## Offline / Restricted Environments

If your environment cannot access external URLs, copy the llms.txt content locally.

### Option 1: Download Files

```bash
# Download to your project
curl -o ./docs/llms.txt https://raw.githubusercontent.com/sahina/cvt/main/llms.txt
curl -o ./docs/llms-full.txt https://raw.githubusercontent.com/sahina/cvt/main/llms-full.txt
```

Then reference the local paths in your context file:

```markdown
## CVT Documentation

See local files:
- ./docs/llms.txt - CVT summary
- ./docs/llms-full.txt - Full API reference
```

### Option 2: Embed Key Information

For highly restricted environments, embed the essential information directly:

````markdown
## CVT Quick Reference

### Server
- gRPC port: 9550
- Metrics port: 9551
- Start: `docker run -d -p 9550:9550 ghcr.io/cvt/cvt-server:latest`

### Node.js SDK

```typescript
import { ContractValidator } from '@sahina/cvt-sdk';

const validator = new ContractValidator('localhost:9550');
await validator.registerSchema('api-name', schemaContent);

const result = await validator.validate(
  { method: 'GET', path: '/users/123' },
  { statusCode: 200, body: '{"id": "123"}' }
);
```

### Python SDK

```python
from cvt_sdk import ContractValidator

validator = ContractValidator('localhost:9550')
validator.register_schema('api-name', schema_content)

result = validator.validate(
    request={'method': 'GET', 'path': '/users/123'},
    response={'status_code': 200, 'body': '{"id": "123"}'}
)
```
````

---

## Customizing Templates

Adapt these templates for your project:

1. **Specify your SDK**: Replace `[Node.js/Python/Go/Java]` with your actual SDK
2. **Add schema locations**: Point to where your OpenAPI schemas live
3. **Include server address**: Update if not using default localhost:9550
4. **Add project conventions**: Include your team's testing patterns

---

## Next Steps

- **[Common Mistakes](./common-mistakes.md)** - Pitfalls to avoid
- **[Advanced Patterns](./advanced-patterns.md)** - Schema evolution and CI/CD workflows
- **[Consumer Testing Guide](https://sahina.github.io/cvt/docs/guides/consumer-testing)** - Full testing workflow
