---
title: Advanced Patterns
sidebar_label: Advanced Patterns
sidebar_position: 4
description: Schema evolution, CI/CD, and fixture generation workflows with AI assistance
---

# Advanced Patterns

This guide covers advanced CVT workflows where AI coding agents can provide significant help.

---

## Schema Evolution Workflows

Managing API changes safely requires understanding breaking changes and coordinating with consumers.

### Comparing Schemas for Breaking Changes

Use `compareSchemas` to understand the impact of API changes:

```typescript
// Compare old and new schema versions
const comparison = await validator.compareSchemas(oldSchema, newSchema);

console.log('Breaking changes:', comparison.breakingChanges);
console.log('Non-breaking changes:', comparison.nonBreakingChanges);
```

AI agents can help interpret the results:

```text
User: I'm getting breaking changes when comparing my schemas. Here's the output:
[paste comparison result]

AI: Looking at the comparison, I see two breaking changes:
1. The `email` field was removed from the User response - this will break consumers expecting that field
2. The `status` field changed from optional to required - consumers not sending this field will fail

Recommended actions:
- For the email field: Add it back or coordinate deprecation with consumers
- For the status field: Make it optional, or provide a default value
```

### Consumer Registry Setup

Help AI understand your consumer registration needs:

```text
User: Set up our service as a consumer of the user-api. We call GET /users/{id}
and POST /users, and we use the id, name, and email fields.

AI: I'll register your service as a consumer:

await validator.registerConsumer({
  consumerId: 'your-service',
  consumerVersion: process.env.APP_VERSION || '1.0.0',
  schemaId: 'user-api',
  schemaVersion: '1.0.0',
  environment: process.env.NODE_ENV || 'dev',
  usedEndpoints: [
    {
      method: 'GET',
      path: '/users/{id}',
      usedFields: ['id', 'name', 'email']
    },
    {
      method: 'POST',
      path: '/users',
      usedFields: ['id', 'name', 'email']
    }
  ]
});
```

### Deployment Safety Checks

Before deploying schema changes, verify safety:

```typescript
const result = await validator.canIDeploy('user-api', '2.0.0', 'prod');

if (!result.safeToDeploy) {
  console.error('UNSAFE TO DEPLOY');
  console.error('Summary:', result.summary);

  for (const consumer of result.affectedConsumers) {
    if (consumer.willBreak) {
      console.error(`- ${consumer.consumerId} v${consumer.consumerVersion} will break`);
      console.error(`  Reason: ${consumer.breakageReason}`);
    }
  }
  process.exit(1);
}

console.log('Safe to deploy!');
```

---

## CI/CD Integration

AI agents can help set up contract testing in your CI/CD pipelines.

### GitHub Actions

:::note
The health check in the example below uses `grpc-health-probe`. Ensure the CVT server image includes this binary, or use an alternative health check method like checking the metrics endpoint.
:::

```yaml
name: Contract Tests

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  contract-tests:
    runs-on: ubuntu-latest

    services:
      cvt:
        image: ghcr.io/cvt/cvt-server:latest
        ports:
          - 9550:9550
          - 9551:9551
        options: >-
          --health-cmd "grpc-health-probe -addr=localhost:9550 || exit 1"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v4

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'

      - name: Install dependencies
        run: npm ci

      - name: Run contract tests
        run: npm run test:contract
        env:
          CVT_SERVER: localhost:9550

      - name: Register consumer (main branch only)
        if: github.ref == 'refs/heads/main'
        run: |
          npm run register-consumer -- \
            --consumer-id ${{ github.repository }} \
            --consumer-version ${{ github.sha }} \
            --environment staging
```

### GitLab CI

```yaml
stages:
  - test
  - register
  - deploy

variables:
  CVT_SERVER: cvt-server:9550

contract-tests:
  stage: test
  services:
    - name: ghcr.io/cvt/cvt-server:latest
      alias: cvt-server
  script:
    - npm ci
    - npm run test:contract
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH

register-consumer:
  stage: register
  services:
    - name: ghcr.io/cvt/cvt-server:latest
      alias: cvt-server
  script:
    - npm run register-consumer -- \
        --consumer-id $CI_PROJECT_PATH \
        --consumer-version $CI_COMMIT_SHA \
        --environment staging
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH

check-deploy-safety:
  stage: deploy
  script:
    - |
      cvt can-i-deploy \
        --schema user-api \
        --version $NEW_SCHEMA_VERSION \
        --env prod \
        --server $CVT_SERVER
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
      when: manual
```

### Integrating with Existing Test Pipelines

If you already have tests, add contract validation:

```typescript
// jest.setup.ts
import fs from 'fs';
import { ContractValidator } from '@cvt/cvt-sdk';

let validator: ContractValidator;

beforeAll(async () => {
  validator = new ContractValidator(process.env.CVT_SERVER || 'localhost:9550');

  // Register all schemas your tests need
  const userApiSchema = fs.readFileSync('./contracts/user-api.json', 'utf-8');
  await validator.registerSchema('user-api', userApiSchema);

  // Make validator available globally
  (global as any).cvtValidator = validator;
});

afterAll(async () => {
  validator.close();
});
```

```typescript
// user.contract.test.ts
describe('User API Contract', () => {
  const validator = (global as any).cvtValidator;

  it('GET /users/{id} returns valid response', async () => {
    // Your actual API call
    const response = await fetch('http://user-service/users/123');
    const body = await response.text();

    // Validate against contract
    const result = await validator.validate(
      { method: 'GET', path: '/users/123' },
      { statusCode: response.status, body }
    );

    expect(result.valid).toBe(true);
  });
});
```

---

## Fixture Generation Power Pattern

CVT can generate schema-compliant test data. This is powerful when combined with AI customization.

### The Pattern: Generate → Customize → Validate

Instead of hand-crafting test data (error-prone and drifts from schema), use this workflow:

1. **Generate** a valid fixture from the schema
2. **Customize** specific fields for your test scenario
3. **Validate** to ensure customizations didn't break compliance

```typescript
// 1. Generate base request body
const baseRequestBody = await validator.generateRequestBody('POST', '/users');

// 2. Customize for your scenario
const testUser = {
  ...baseRequestBody,
  name: 'Test User for Edge Case',
  email: 'edge-case@test.com'
};

// 3. Validate customized fixture still complies
const preValidation = await validator.validate(
  { method: 'POST', path: '/users', body: JSON.stringify(testUser) },
  { statusCode: 201, body: '{"id": "generated-id"}' }
);

expect(preValidation.valid).toBe(true);
```

### Generating Response Fixtures

Use generated responses for mock services:

```typescript
import nock from 'nock';

// Generate a valid response
const response = await validator.generateResponse('GET', '/users/{id}', {
  statusCode: 200
});

// Use in mock
nock('http://user-service')
  .get('/users/123')
  .reply(200, response.body);
```

### Using Schema Examples

If your OpenAPI schema has examples, prefer those:

```typescript
const response = await validator.generateResponse('GET', '/users/{id}', {
  useExamples: true  // Use examples from schema if available
});
```

### Listing Available Endpoints

Ask AI to help explore what fixtures can be generated:

```typescript
const endpoints = await validator.listEndpoints();

for (const endpoint of endpoints) {
  console.log(`${endpoint.method} ${endpoint.path}`);
  console.log(`  Summary: ${endpoint.summary}`);
}
```

---

## Working with AI on Advanced Tasks

When asking AI for help with advanced patterns, provide context:

```text
User: Help me set up consumer registration that runs after our contract tests.
We're using Jest and the Node.js SDK. Our service is "order-service" and we
depend on "user-api" and "inventory-api".

AI: I'll help you set up post-test consumer registration. Here's the approach:

1. Capture interactions during tests using the HTTP adapter
2. After all tests pass, register with captured endpoints

[AI provides implementation...]
```

```text
User: Our canIDeploy check is failing. Here's the output:
[paste canIDeploy result]

AI: Looking at the results, I see order-service v2.1.0 will break because:
- It uses GET /users/{id} which is being removed in the new schema

Options:
1. Coordinate with order-service team to update before deploying
2. Keep the endpoint but mark it deprecated
3. Version the API (v2) rather than breaking v1
```

---

## Next Steps

- **[Common Mistakes](./common-mistakes.md)** - Pitfalls to avoid
- **[Context Templates](./context-templates.md)** - Set up your AI tool with CVT context
- **[Breaking Changes Guide](https://sahina.github.io/cvt/docs/guides/breaking-changes)** - Full breaking changes documentation
- **[API Reference](https://sahina.github.io/cvt/docs/reference/api)** - Complete API documentation
