# CVT CI/CD Integration Templates

This directory contains ready-to-use CI/CD templates for integrating CVT contract validation into your build pipelines.

## Quick Start

Choose your CI/CD platform and follow the instructions below.

---

## GitHub Actions

### Option 1: Copy the Workflow (Recommended)

Copy `github-workflow.yml` to your repository:

```bash
mkdir -p .github/workflows
cp github-workflow.yml .github/workflows/contract-validation.yml
```

Edit the file to match your project structure:

- Update `openapi.json` path to your schema location
- Update `tests/fixtures/*.json` to your fixture location
- Update the CVT repository reference

### Option 2: Use the Composite Action

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

| Input                      | Required | Default      | Description                                |
| -------------------------- | -------- | ------------ | ------------------------------------------ |
| `schema-path`              | Yes      | -            | Path to your OpenAPI schema                |
| `schema-id`                | No       | `api-schema` | Unique identifier for the schema           |
| `validate-fixtures`        | No       | -            | Glob pattern for fixture files to validate |
| `compare-with`             | No       | -            | Path to previous schema version            |
| `fail-on-breaking-changes` | No       | `true`       | Fail build on breaking changes             |

---

## GitLab CI

### Option 1: Include the Template

Add to your `.gitlab-ci.yml`:

```yaml
include:
  - project: "sahina/cvt"
    file: "/ci-templates/gitlab-ci.yml"

variables:
  CVT_SCHEMA_PATH: "api/openapi.json"
```

This gives you these jobs:

- `cvt:validate-schema` - Validates the schema is correct
- `cvt:breaking-changes` - Checks for breaking changes in MRs
- `cvt:validate-fixtures` - Validates fixture files
- `cvt:generate-fixtures` - Generates fixtures (manual trigger)

### Option 2: Copy Specific Jobs

Copy the jobs you need from `gitlab-ci.yml` into your own `.gitlab-ci.yml`.

### Variables

| Variable          | Default        | Description                 |
| ----------------- | -------------- | --------------------------- |
| `CVT_SCHEMA_PATH` | `openapi.json` | Path to your OpenAPI schema |
| `CVT_GO_VERSION`  | `1.25`         | Go version for building CVT |

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
from cvt_sdk import ContractValidator

@pytest.fixture
def validator():
    v = ContractValidator()
    v.register_schema('my-api', './openapi.json')
    yield v
    v.close()

def test_post_users_contract(validator):
    response = validator.generate_response('POST', '/users', status_code=201)
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
    "body": "{\"id\": 123, \"name\": \"John\"}"
  }
}
```

---

## Support

- **Issues**: Report problems at [CVT Issues](https://github.com/sahina/cvt/issues)
- **Slack**: #platform-cvt
- **Documentation**: See main [README](../README.md)
