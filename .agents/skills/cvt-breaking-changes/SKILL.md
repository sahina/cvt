---
name: cvt-breaking-changes
description: Detect and report breaking changes between OpenAPI schema versions
sdk_version: "0.4.0"
---

# CVT Breaking Changes Detection

## Prerequisites

- Two versions of the OpenAPI schema: the current (old) and proposed (new)
- CVT CLI installed (`go install github.com/sahina/cvt/cmd/cvt@latest`) or CVT SDK installed in the project
- For SDK-based detection: a running CVT server on port 9550

## Auto-Detection

```bash
if [ -f package.json ]; then echo "LANG=node"
elif [ -f pyproject.toml ] || [ -f setup.py ]; then echo "LANG=python"
elif [ -f go.mod ]; then echo "LANG=go"
elif [ -f pom.xml ] || [ -f build.gradle ]; then echo "LANG=java"
else echo "LANG=unknown"
fi
```

## Goal

Compare two versions of an OpenAPI schema to detect breaking changes that would affect consumers, and produce a clear report of what changed and why it breaks compatibility.

## Steps

1. **Obtain both schema versions**: the old (current/baseline) and new (proposed/updated) schemas. These can be local files or URLs.
2. **Run the comparison** using either the CVT CLI or the SDK `compareSchemas()` method.
3. **Review the results**: each breaking change includes the type, location, and description.
4. **Report findings** to the team with actionable details.

## CLI Usage (Any Language)

The fastest way to detect breaking changes without writing code:

```bash
# Compare two local files
cvt compare --old ./openapi-v1.json --new ./openapi-v2.json

# Compare a local file against a remote URL
cvt compare --old ./openapi.json --new https://api.example.com/openapi.json

# JSON output for scripting
cvt compare --old ./openapi-v1.json --new ./openapi-v2.json --json
```

The CLI exits with code 1 if breaking changes are detected, making it suitable for CI gates.

## SDK-Specific Instructions

### Node.js

```typescript
import { ContractValidator } from "@sahina/cvt-sdk";

async function checkBreakingChanges() {
  const validator = new ContractValidator("localhost:9550");

  // Register both schema versions
  await validator.registerSchema("my-api", "./openapi-v1.json");

  // Compare schemas
  const result = await validator.compareSchemas("my-api", "1.0.0", "2.0.0");

  console.log(`Breaking changes: ${result.breakingChanges.length}`);
  for (const change of result.breakingChanges) {
    console.log(`  [${change.type}] ${change.path}: ${change.description}`);
  }

  await validator.close();
}
```

### Python

```python
from cvt_sdk import ContractValidator

def check_breaking_changes():
    validator = ContractValidator("localhost:9550")

    validator.register_schema("my-api", "./openapi-v1.json")

    result = validator.compare_schemas("my-api", "1.0.0", "2.0.0")

    print(f"Breaking changes: {len(result['breaking_changes'])}")
    for change in result["breaking_changes"]:
        print(f"  [{change['type']}] {change['path']}: {change['description']}")

    validator.close()
```

### Go

```go
package main

import (
    "context"
    "fmt"
    "log"
    "github.com/sahina/cvt/sdks/go/cvt"
)

func main() {
    ctx := context.Background()
    v, err := cvt.NewValidator("localhost:9550")
    if err != nil {
        log.Fatal(err)
    }
    defer v.Close()

    if err := v.RegisterSchema(ctx, "my-api", "./openapi-v1.json"); err != nil {
        log.Fatal(err)
    }

    result, err := v.CompareSchemas(ctx, "my-api", "1.0.0", "2.0.0")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Breaking changes: %d\n", len(result.BreakingChanges))
    for _, change := range result.BreakingChanges {
        fmt.Printf("  [%s] %s: %s\n", change.Type, change.Path, change.Description)
    }
}
```

### Java

```java
import io.github.sahina.sdk.ContractValidator;

public class BreakingChangeCheck {
    public static void main(String[] args) {
        var validator = ContractValidator.builder()
            .address("localhost:9550")
            .build();

        validator.registerSchema("my-api", "./openapi-v1.json");

        var result = validator.compareSchemas("my-api", "1.0.0", "2.0.0");

        System.out.printf("Breaking changes: %d%n", result.getBreakingChanges().size());
        for (var change : result.getBreakingChanges()) {
            System.out.printf("  [%s] %s: %s%n",
                change.getType(), change.getPath(), change.getDescription());
        }

        validator.close();
    }
}
```

## Breaking Change Types

CVT detects the following categories of breaking changes:

| Type | Description | Impact |
|---|---|---|
| **Endpoint removed** | A path+method combination was deleted | Consumers calling this endpoint will get 404 |
| **Required field added** | A new required field added to request body | Consumers not sending this field will get 400 |
| **Field type changed** | A field's type changed (e.g., `string` to `integer`) | Consumers sending the old type will get validation errors |
| **Required parameter added** | A new required query/header/path param added | Consumers not sending this parameter will get 400 |
| **Response schema changed** | Response body structure changed (field removed, type changed) | Consumers parsing the response may break |
| **Enum value removed** | An allowed enum value was removed | Consumers sending the removed value will get 400 |
| **Response status code removed** | A previously defined status code is no longer documented | Consumers handling that status code may have stale logic |
| **Content type removed** | A supported content type was removed (e.g., `application/xml`) | Consumers requesting that content type will fail |

## Non-Breaking Changes (Safe)

These changes are detected but do not count as breaking:

- Adding a new endpoint
- Adding an optional field to request body
- Adding a new optional query parameter
- Adding a new enum value
- Adding a new response status code
- Relaxing a constraint (e.g., removing `required` from a field)

## CI Integration

Add breaking change detection to pull requests. See `ci-templates/` for full examples:

```yaml
# GitHub Actions - detect breaking changes on PRs
- name: Check for breaking changes
  run: |
    git show origin/${{ github.base_ref }}:openapi.json > /tmp/old-schema.json
    cvt compare --old /tmp/old-schema.json --new ./openapi.json --json
```

For automated gating with `register-schema`:

```bash
cvt register-schema my-api ./openapi.json --check-compatibility --fail-on-breaking
```

This registers the new schema only if it has no breaking changes compared to the currently registered version.

## Common Errors

| Error | Cause | Fix |
|---|---|---|
| `schema not found` | Schema ID not registered on the server | Register both versions first, or use CLI with file paths |
| `version not found` | The specified version does not exist | Check registered versions with `cvt generate --schema <id> --list` |
| `parse error on old/new schema` | One of the schema files is invalid | Validate both schemas independently before comparing |
| `no differences found` | Schemas are identical | Verify you are comparing the correct files |
| `exit code 1` from CLI | Breaking changes detected (expected behavior) | Review the output and decide whether to proceed |

## Success Criteria

- Both schema versions are compared successfully
- All breaking changes are listed with type, location, and description
- The report is clear enough for the provider team to understand what needs attention
- In CI: the pipeline fails if breaking changes are detected (when using `--fail-on-breaking`)
