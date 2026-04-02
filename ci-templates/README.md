# CVT CI/CD Integration Templates

This directory contains ready-to-use CI/CD templates for integrating CVT contract validation into your build pipelines.

## Quick Start

Choose your CI/CD platform and follow the instructions below.

---

## GitHub Actions

### Demo Workflows

This directory contains three demo workflows for GitHub Actions:

- **`demo-consumer.yml`** - Demonstrates how an API consumer runs contract tests and registers API usage with the CVT server
- **`demo-producer.yml`** - Demonstrates how an API producer validates schema changes and checks consumer compatibility
- **`demo-can-i-deploy.yml`** - Demonstrates the deployment safety check that verifies schema changes won't break consumers

Copy any of these to your repository's `.github/workflows/` directory and customize for your project.

### Use the Composite Action

Reference the CVT action directly in your workflow:

```yaml
name: Contract Validation

on: [push, pull_request]

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: sahina/cvt/.github/actions/cvt-validate@main
        with:
          schema-path: "./openapi.json"
          validate-fixtures: "tests/fixtures/*.json"
          compare-with: "" # Set to previous schema for breaking change detection
```

### Action Inputs

| Input                      | Required | Default      | Description                                    |
| -------------------------- | -------- | ------------ | ---------------------------------------------- |
| `schema-path`              | Yes      | -            | Path to your OpenAPI schema                    |
| `schema-id`                | No       | `api-schema` | Unique identifier for the schema               |
| `cvt-server`               | No       | -            | CVT server address (if using server mode)      |
| `validate-fixtures`        | No       | -            | Glob pattern for fixture files to validate     |
| `compare-with`             | No       | -            | Path to previous schema version                |
| `fail-on-breaking-changes` | No       | `true`       | Fail build on breaking changes                 |
| `go-version`               | No       | `1.25`       | Go version to use for building CVT CLI         |

---

## GitLab CI

You can adapt the GitHub Actions demo workflows for GitLab CI. The CVT CLI commands are the same across all platforms. Here is an example `.gitlab-ci.yml` configuration:

```yaml
variables:
  CVT_SCHEMA_PATH: "api/openapi.json"

cvt:validate-schema:
  image: ghcr.io/sahina/cvt:latest
  script:
    - cvt generate --schema "$CVT_SCHEMA_PATH" --list

cvt:breaking-changes:
  image: ghcr.io/sahina/cvt:latest
  script:
    - git fetch origin $CI_MERGE_REQUEST_TARGET_BRANCH_NAME
    - git show origin/$CI_MERGE_REQUEST_TARGET_BRANCH_NAME:$CVT_SCHEMA_PATH > /tmp/old.json || true
    - cvt compare --old /tmp/old.json --new "$CVT_SCHEMA_PATH"
  rules:
    - if: $CI_MERGE_REQUEST_IID

cvt:validate-fixtures:
  image: ghcr.io/sahina/cvt:latest
  script:
    - |
      for f in tests/fixtures/*.json; do
        cvt validate --schema "$CVT_SCHEMA_PATH" --interaction "$f"
      done
```

---

## Jenkins

### Option 1: Copy the Jenkinsfile

Copy `Jenkinsfile` to your repository root and customize:

```groovy
pipeline {
    // ... see Jenkinsfile for full example
}
```

### Option 2: Use as Shared Library

1. Add CVT as a shared library in Jenkins configuration
2. Call from your Jenkinsfile:

```groovy
@Library('cvt') _

cvtValidation(
    schemaPath: 'openapi.json',
    fixturesPath: 'tests/fixtures',
    failOnBreakingChanges: true
)
```

### Parameters

| Parameter                  | Default          | Description                    |
| -------------------------- | ---------------- | ------------------------------ |
| `SCHEMA_PATH`              | `openapi.json`   | Path to your OpenAPI schema    |
| `FIXTURES_PATH`            | `tests/fixtures` | Path to fixture files          |
| `FAIL_ON_BREAKING_CHANGES` | `true`           | Fail build on breaking changes |

---

## Common Use Cases

### 1. Basic Schema Validation

Ensure your OpenAPI schema is valid on every commit:

```yaml
# GitHub Actions
- run: cvt generate --schema ./openapi.json --list
```

### 2. Breaking Change Detection on PRs

Prevent breaking changes from being merged:

```yaml
# GitHub Actions
- name: Check Breaking Changes
  run: |
    git show origin/${{ github.base_ref }}:openapi.json > /tmp/old.json
    cvt compare --old /tmp/old.json --new ./openapi.json
```

### 3. Validate Test Fixtures

Ensure your test fixtures match the contract:

```yaml
# GitHub Actions
- run: |
    for f in tests/fixtures/*.json; do
      cvt validate --schema ./openapi.json --interaction "$f"
    done
```

### 4. Generate Fixtures in CI

Auto-generate fixtures for new endpoints:

```yaml
# GitHub Actions
- run: |
    cvt generate --schema ./openapi.json --method POST --path /users -o fixtures/post_users.json
```

---

## Integration with Test Frameworks

### Jest (Node.js)

```typescript
import { ContractValidator } from "@sahina/cvt-sdk";

describe("API Contract Tests", () => {
  let validator: ContractValidator;

  beforeAll(async () => {
    validator = new ContractValidator();
    await validator.registerSchema("my-api", "./openapi.json");
  });

  it("POST /users matches contract", async () => {
    const response = await validator.generateResponse("POST", "/users", {
      statusCode: 201,
    });
    const result = await validator.validate(
      { method: "POST", path: "/users", body: { name: "Test" } },
      { statusCode: 201, body: response.body },
    );
    expect(result.valid).toBe(true);
  });
});
```

### Pytest (Python)

```python
import pytest
from cvt_sdk import ContractValidator, GenerateOptions

@pytest.fixture
def validator():
    v = ContractValidator()
    v.register_schema('my-api', './openapi.json')
    yield v
    v.close()

def test_post_users_contract(validator):
    response = validator.generate_response('POST', '/users', GenerateOptions(status_code=201))
    result = validator.validate(
        {'method': 'POST', 'path': '/users', 'body': {'name': 'Test'}},
        {'status_code': 201, 'body': response['body']}
    )
    assert result['valid']
```

---

## Local Testing

Before pushing to CI, you can test the workflow commands locally.

### Prerequisites

Build the CVT CLI:

```bash
go build -o cvt ./cmd/cvt
```

### Simulate the CI Workflow

```bash
# 1. Validate schema (list all endpoints)
./cvt generate --schema ./openapi.json --list

# 2. Check for breaking changes between versions
./cvt compare --old ./openapi-v1.json --new ./openapi-v2.json

# 3. Generate a test fixture
./cvt generate --schema ./openapi.json --method GET --path "/users/{id}" -o fixture.json

# 4. Validate the fixture against the schema
./cvt validate --schema ./openapi.json --interaction fixture.json
```

### Using act for GitHub Actions

You can use [act](https://github.com/nektos/act) to run GitHub Actions locally:

```bash
# Install act
brew install act

# Run the workflow
act push
```

---

## Troubleshooting

### Schema Not Found

Ensure the schema path is correct relative to your repository root:

```yaml
# Wrong
schema-path: 'openapi.json'  # If schema is in api/ directory

# Correct
schema-path: 'api/openapi.json'
```

### Breaking Changes False Positives

If you're seeing false positives, ensure you're comparing the correct branches:

```yaml
# GitHub Actions - compare against base branch
git show origin/${{ github.base_ref }}:openapi.json > /tmp/old.json
```

### Fixture Validation Fails

Check that your fixture files have the correct structure:

```json
{
  "request": {
    "method": "GET",
    "path": "/users/123",
    "headers": {}
  },
  "response": {
    "statusCode": 200,
    "headers": { "Content-Type": "application/json" },
    "body": { "id": 123, "name": "John" }
  }
}
```

---

## Support

- **Issues**: Report problems at [CVT Issues](https://github.com/sahina/cvt/issues)
- **Slack**: #platform-cvt
- **Documentation**: See main [README](../README.md)
