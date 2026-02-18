---
title: Common Mistakes
sidebar_label: Common Mistakes
sidebar_position: 3
description: Pitfalls to avoid when using AI agents with CVT
---

# Common Mistakes

When using AI coding agents with CVT, watch out for these common pitfalls. Share this page with your AI agent to help it avoid these mistakes.

---

## Schema Management

- **Don't ask AI to generate OpenAPI schemas from scratch** - Use your real API specification. AI-generated schemas won't match your actual API behavior and defeat the purpose of contract testing.

- **Don't validate against outdated schemas** - Always use the current version of your OpenAPI spec. Stale schemas lead to false positives or missed contract violations.

- **Don't modify schemas to make tests pass** - If validation fails, fix your code or update the real API spec through proper channels. Never tweak schemas just to silence errors.

- **Don't skip schema registration** - Every validation requires a registered schema. Calling `validate()` without first calling `registerSchema()` will fail.

---

## Server Setup

- **Don't mock the CVT server** - Run the real CVT server. Mocking bypasses actual validation logic and gives false confidence. The server is lightweight and easy to run:

  ```bash
  docker run -d -p 9550:9550 ghcr.io/sahina/cvt-server:latest
  ```

- **Don't hardcode server addresses in tests** - Use environment variables or configuration files so tests work in different environments (local, CI, staging).

- **Don't forget to wait for server startup** - In CI/CD, ensure the CVT server is healthy before running tests:

  ```bash
  grpc-health-probe -addr=localhost:9550
  ```

---

## Testing Patterns

- **Don't write validation logic yourself** - Use CVT's `validate()` method. Hand-written validation logic will drift from the actual OpenAPI spec and miss edge cases.

  ```typescript
  // Wrong: manual validation
  if (!response.body.id || typeof response.body.id !== 'string') {
    throw new Error('Invalid response');
  }

  // Right: use CVT
  const result = await validator.validate(request, response);
  expect(result.valid).toBe(true);
  ```

- **Don't validate only happy paths** - Test error responses (4xx, 5xx) too. Your OpenAPI spec should define error response schemas, and CVT validates those as well.

- **Don't ignore validation errors** - Validation errors indicate contract violations. Log them, fail tests, or alert—never silently swallow them:

  ```typescript
  if (!result.valid) {
    console.error('Contract violation:', result.errors);
    throw new Error(`Validation failed: ${result.errors.join(', ')}`);
  }
  ```

- **Don't test implementation details** - Contract tests verify the interface, not internal behavior. Focus on request/response shapes, not business logic.

---

## Fixture Generation

- **Don't hand-craft test data** - Use CVT's fixture generation to create schema-compliant test data. Hand-crafted fixtures drift from the schema over time:

  ```typescript
  // Generate a valid response from the schema
  const response = await validator.generateResponse('GET', '/users/{id}');

  // Or generate both request and response
  const fixture = await validator.generateFixture('POST', '/users');
  ```

- **Don't assume generated fixtures are complete** - `generateFixture()` creates structurally valid data. You may need to customize specific values for your test scenarios.

- **Don't generate fixtures without a registered schema** - The schema must be registered first for fixture generation to work.

---

## Dependencies

- **Don't mix SDK versions** - If you have multiple services using CVT, ensure compatible SDK and server versions across your organization.

- **Don't ignore gRPC version conflicts** - The SDKs depend on specific gRPC library versions. Check for conflicts with other gRPC-using dependencies in your project.

- **Don't forget to close connections** - SDKs maintain gRPC connections. Close them when done to avoid resource leaks:

  ```typescript
  afterAll(() => {
    validator.close();
  });
  ```

---

## Error Handling

- **Don't treat validation errors as bugs** - A validation error means the interaction doesn't match the contract. This is CVT working correctly—the error is in your code or expectations.

- **Don't retry on validation failure** - Validation is deterministic. If it fails once, it will fail again. Fix the underlying issue instead of retrying.

- **Don't catch and ignore gRPC errors** - Connection errors, timeouts, and server errors need proper handling. Distinguish between "validation failed" and "couldn't reach server."

---

## Consumer Registration

- **Don't skip consumer registration** - Registering your service as a consumer enables the `canIDeploy` safety check. Without registration, upstream teams can't know they might break you.

- **Don't register with incorrect endpoints** - The `usedEndpoints` must match what your service actually calls. Incorrect registrations lead to false safety signals.

- **Don't forget to deregister deprecated consumers** - When you stop using an API, deregister to avoid blocking upstream deployments unnecessarily.

---

## CI/CD Integration

- **Don't run contract tests against production** - Use dedicated test environments. Contract tests may create test data or hit endpoints in ways that shouldn't happen in production.

- **Don't skip contract tests in PRs** - Contract tests should run on every pull request to catch violations early.

- **Don't deploy without checking `canIDeploy`** - If you're a producer, always verify deployment safety before releasing schema changes.

---

## Quick Reference for AI Agents

Include this in your AI context to help avoid mistakes:

```text
CVT Anti-Patterns:
- Use real OpenAPI schemas, don't generate them
- Run real CVT server, don't mock it
- Use validate(), don't write manual validation
- Use generateFixture(), don't hand-craft test data
- Handle validation errors, don't ignore them
- Register as consumer, don't skip it
- Check canIDeploy before deploying schema changes
```

---

## Next Steps

- **[Advanced Patterns](./advanced-patterns.md)** - Schema evolution and CI/CD workflows
- **[Context Templates](./context-templates.md)** - Set up your AI tool with CVT context
- **[Consumer Testing Guide](https://sahina.github.io/cvt/docs/guides/consumer-testing)** - Full testing workflow
