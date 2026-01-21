---
title: Breaking Changes Guide
sidebar_label: Breaking Changes
sidebar_position: 4
description: Understanding and detecting breaking changes in API schemas
---

# Breaking Changes Guide

**CONTENT TO BE VALIDATED**

This guide covers how CVT detects breaking changes between API schema versions and how to use the `can-i-deploy` safety check.

## What Are Breaking Changes?

Breaking changes are modifications to an API that can cause existing consumers to fail. CVT detects these automatically when comparing schema versions.

### Types of Breaking Changes

| Type                       | Description                          | Example                                   |
| -------------------------- | ------------------------------------ | ----------------------------------------- |
| `ENDPOINT_REMOVED`         | An endpoint was removed              | `DELETE /users/{id}` no longer exists     |
| `REQUIRED_FIELD_ADDED`     | A new required field in request      | Request now requires `phone` field        |
| `REQUIRED_PARAMETER_ADDED` | New required query/path/header param | `?apiVersion` now required                |
| `TYPE_CHANGED`             | Field type changed incompatibly      | `id` changed from `integer` to `string`   |
| `RESPONSE_SCHEMA_CHANGED`  | Response structure changed           | Response no longer includes `email` field |
| `ENUM_VALUE_REMOVED`       | Enum value was removed               | `status` no longer accepts `"pending"`    |

### Non-Breaking Changes

These changes are safe for existing consumers:

| Change                     | Why It's Safe                          |
| -------------------------- | -------------------------------------- |
| Adding optional fields     | Consumers can ignore them              |
| Adding new endpoints       | Existing calls aren't affected         |
| Adding optional parameters | Existing calls work without them       |
| Adding enum values         | Existing values still work             |
| Relaxing validation        | Previously valid requests remain valid |
| Improving descriptions     | Documentation-only                     |

---

## Schema Comparison

### Using the CLI

```bash
# Compare two schema files
cvt compare --old ./v1/openapi.json --new ./v2/openapi.json

# JSON output for scripting
cvt compare --old ./v1/openapi.json --new ./v2/openapi.json --json
```

### Output Example

```text
Schema Comparison
=================
Old: ./v1/openapi.json
New: ./v2/openapi.json

Breaking changes detected: 2

  1. [ENDPOINT_REMOVED] DELETE /users/{id}
     The endpoint has been removed from the API

  2. [REQUIRED_FIELD_ADDED] POST /users
     Required field 'phone' added to request body

Compatible changes: 3

  1. [ENDPOINT_ADDED] GET /users/{id}/profile
  2. [OPTIONAL_FIELD_ADDED] GET /users response: 'avatar_url'
  3. [DESCRIPTION_CHANGED] PUT /users/{id}

Result: INCOMPATIBLE (2 breaking changes)
```

### Using the SDK

```typescript
import { ContractValidator } from "@cvt/cvt-sdk";

const validator = new ContractValidator("localhost:9550");

// Register both versions
await validator.registerSchemaWithVersion("my-api", oldSchema, "1.0.0");
await validator.registerSchemaWithVersion("my-api", newSchema, "2.0.0");

// Compare
const result = await validator.compareSchemas("my-api", "1.0.0", "2.0.0");

if (!result.compatible) {
  console.error("Breaking changes detected:");
  result.breakingChanges.forEach((change) => {
    console.error(`- [${change.type}] ${change.path} ${change.method}`);
    console.error(`  ${change.description}`);
  });
}
```

---

## Deployment Safety (can-i-deploy)

The `can-i-deploy` check combines breaking change detection with the consumer registry to determine if a new schema version is safe to deploy.

### How It Works

```text
┌─────────────────────┐
│  New Schema v2.0.0  │
└─────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────┐
│            CVT Server                       │
│                                             │
│  1. Detect breaking changes from v1.0.0     │
│  2. Look up registered consumers            │
│  3. Check which consumers use affected      │
│     endpoints/fields                        │
│  4. Return safety assessment                │
└─────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────┐
│   Safe / Unsafe     │
│   + Affected list   │
└─────────────────────┘
```

### CLI Usage

```bash
# Basic usage
cvt can-i-deploy --schema user-api --version 2.0.0 --env prod

# With server address
cvt can-i-deploy --schema user-api --version 2.0.0 --env prod --server cvt.internal:9550

# JSON output for CI/CD
cvt can-i-deploy --schema user-api --version 2.0.0 --env prod --json
```

### SDK Usage

```typescript
const result = await validator.canIDeploy({
  schemaId: "user-api",
  newVersion: "2.0.0",
  environment: "prod",
});

if (result.safeToDeploy) {
  console.log("Safe to deploy!");
} else {
  console.error("UNSAFE:", result.summary);

  // Show breaking changes
  for (const change of result.breakingChanges) {
    console.error(`- [${change.type}] ${change.description}`);
  }

  // Show affected consumers
  for (const consumer of result.affectedConsumers) {
    if (consumer.willBreak) {
      console.error(`Consumer ${consumer.consumerId} will break!`);
      console.error(
        `  Uses endpoints: ${consumer.relevantChanges.map((c) => c.path).join(", ")}`,
      );
    }
  }
}
```

### Output: Safe to Deploy

```text
Deployment Safety Check
=======================
Schema:      user-api
Version:     2.0.0
Environment: prod
Server:      localhost:9550

Result: SAFE TO DEPLOY

No breaking changes detected that would affect registered consumers.
Consumers checked: 3
```

### Output: Unsafe to Deploy

```text
Deployment Safety Check
=======================
Schema:      user-api
Version:     2.0.0
Environment: prod

Result: UNSAFE TO DEPLOY

Breaking changes in v2.0.0:
  - FIELD_REMOVED: GET /users/{id} response removed 'email'
  - ENDPOINT_REMOVED: DELETE /users/{id}

Affected consumers in production:

  order-service v2.1.0
    Schema version: 1.0.0
    Impact: BREAKING
    Uses: GET /users/{id} (fields: id, email, name)
    Affected by:
      - FIELD_REMOVED: email field

  notification-service v1.5.0
    Schema version: 1.0.0
    Impact: BREAKING
    Uses: DELETE /users/{id}
    Affected by:
      - ENDPOINT_REMOVED: DELETE /users/{id}

  billing-service v1.0.0
    Schema version: 1.0.0
    Impact: None
    Uses: GET /users/{id} (fields: id, name)

Summary:
  Safe consumers:     1/3
  Affected consumers: 2/3

Recommendation: Coordinate with order-service and notification-service teams before deploying.
```

---

## CI/CD Integration

### PR Check for Schema Changes

```yaml
name: API Compatibility Check

on:
  pull_request:
    paths:
      - "api/openapi.json"

jobs:
  check-breaking-changes:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Get previous schema
        run: git show origin/main:api/openapi.json > /tmp/old-schema.json

      - name: Check for breaking changes
        run: |
          cvt compare --old /tmp/old-schema.json --new ./api/openapi.json
```

### Pre-Deploy Safety Gate

```yaml
deploy:
  runs-on: ubuntu-latest
  steps:
    - name: Check deployment safety
      run: |
        result=$(cvt can-i-deploy \
          --schema ${{ env.SCHEMA_ID }} \
          --version ${{ github.sha }} \
          --env prod \
          --server ${{ secrets.CVT_SERVER }} \
          --json)

        if [ $(echo $result | jq '.safeToDeploy') != "true" ]; then
          echo "DEPLOYMENT BLOCKED: Breaking changes would affect consumers"
          echo $result | jq '.affectedConsumers[] | select(.willBreak) | .consumerId'
          exit 1
        fi

    - name: Deploy
      if: success()
      run: ./deploy.sh
```

---

## Handling Breaking Changes

When you need to make a breaking change, you have several options:

### 1. Coordinate with Consumers

1. Run `can-i-deploy` to identify affected consumers
2. Contact consumer teams with timeline
3. Wait for consumers to update their code
4. Deploy once all consumers are ready

### 2. Version Your API

Keep both versions available during transition:

```yaml
# Old version still available
/api/v1/users/{id}

# New version with breaking changes
/api/v2/users/{id}
```

### 3. Feature Flags

Gradually roll out changes:

```typescript
app.get("/users/:id", (req, res) => {
  const user = getUser(req.params.id);

  if (req.headers["x-api-version"] === "2") {
    // New format (breaking)
    res.json({ id: user.id, profile: { name: user.name } });
  } else {
    // Old format (compatible)
    res.json({ id: user.id, name: user.name, email: user.email });
  }
});
```

### 4. Deprecation Period

1. Add deprecation headers to old endpoints
2. Set a sunset date
3. Monitor usage metrics
4. Remove after deprecation period

---

## Best Practices

### For API Producers

1. **Run comparisons in CI** — Catch breaking changes before they're merged
2. **Use can-i-deploy before production** — Make it a required gate
3. **Version your API** — Major versions for breaking changes
4. **Communicate deprecations** — Give consumers time to adapt

### For API Consumers

1. **Register your dependencies** — Enable can-i-deploy checks
2. **Specify used fields** — Help producers understand impact
3. **Test against schema** — Catch issues before producers deploy
4. **Monitor deprecation warnings** — Plan updates proactively

---

## Related Documentation

- **[Consumer Testing Guide](./consumer-testing.mdx)** - Register as a consumer
- **[Producer Testing Guide](./producer-testing.md)** - Validate your API
- **[CLI Reference](../reference/cli.md)** - Command-line options
- **[API Reference](../reference/api.md)** - Message types
