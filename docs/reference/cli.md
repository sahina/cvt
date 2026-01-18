---
title: CLI Reference
sidebar_label: CLI Reference
sidebar_position: 2
description: CVT command-line interface reference
---

# CLI Reference

The CVT CLI provides local validation capabilities without requiring Docker or a running server. It also includes commands for server management and deployment safety checks.

## Installation

```bash
# Build from source
go build -o cvt ./cmd/cvt

# Or install globally
go install github.com/sahina/cvt/cmd/cvt@latest
```

## Commands Overview

| Command | Description |
|---------|-------------|
| `validate` | Validate an interaction against a schema |
| `compare` | Compare schemas for breaking changes |
| `generate` | Generate test fixtures from schemas |
| `can-i-deploy` | Check deployment safety against registered consumers |
| `serve` | Start the gRPC server |
| `version` | Show version information |

---

## validate

Validate an HTTP request/response interaction against an OpenAPI schema.

### Usage

```bash
cvt validate --schema <path> [options]
```

### Options

| Flag | Description | Required |
|------|-------------|----------|
| `--schema` | Path to OpenAPI schema file (JSON or YAML) | Yes |
| `--request` | Path to request JSON file | Yes (unless `--interaction`) |
| `--response` | Path to response JSON file | Yes (unless `--interaction`) |
| `--interaction` | Path to combined interaction JSON file | No |
| `--json` | Output results as JSON | No |

### Examples

```bash
# Validate with separate request/response files
cvt validate --schema ./openapi.json --request req.json --response resp.json

# Validate with combined interaction file
cvt validate --schema ./openapi.json --interaction interaction.json

# Output as JSON (useful for CI/CD)
cvt validate --schema ./openapi.json --request req.json --response resp.json --json
```

### Request File Format

```json
{
  "method": "GET",
  "path": "/users/123",
  "headers": {
    "Accept": "application/json"
  },
  "body": null
}
```

### Response File Format

```json
{
  "status_code": 200,
  "headers": {
    "Content-Type": "application/json"
  },
  "body": "{\"id\": \"123\", \"name\": \"John Doe\"}"
}
```

### Interaction File Format

```json
{
  "request": {
    "method": "GET",
    "path": "/users/123",
    "headers": {"Accept": "application/json"}
  },
  "response": {
    "status_code": 200,
    "headers": {"Content-Type": "application/json"},
    "body": "{\"id\": \"123\", \"name\": \"John Doe\"}"
  }
}
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Validation passed |
| 1 | Validation failed (errors printed to stderr) |
| 2 | Invalid arguments or file not found |

---

## compare

Compare two OpenAPI schema versions to detect breaking changes.

### Usage

```bash
cvt compare --old <path> --new <path> [options]
```

### Options

| Flag | Description | Required |
|------|-------------|----------|
| `--old` | Path to old/previous schema file | Yes |
| `--new` | Path to new schema file | Yes |
| `--json` | Output results as JSON | No |

### Examples

```bash
# Compare schemas
cvt compare --old ./v1/openapi.json --new ./v2/openapi.json

# JSON output for CI/CD integration
cvt compare --old ./v1/openapi.json --new ./v2/openapi.json --json
```

### Output

```
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

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Schemas are compatible (no breaking changes) |
| 1 | Breaking changes detected |
| 2 | Invalid arguments or file not found |

---

## generate

Generate test fixtures from an OpenAPI schema.

### Usage

```bash
cvt generate --schema <path> [options]
```

### Options

| Flag | Description | Required |
|------|-------------|----------|
| `--schema` | Path to OpenAPI schema file | Yes |
| `--endpoint` | Endpoint to generate fixture for (e.g., "GET /users/{id}") | No |
| `--list` | List all available endpoints | No |
| `--output-type` | What to generate: `fixture`, `request`, or `response` | No (default: `fixture`) |
| `--status-code` | Response status code to generate | No (default: auto-select) |
| `--use-examples` | Use schema examples when available | No |
| `--json` | Output as JSON | No |

### Examples

```bash
# List all endpoints in the schema
cvt generate --schema ./openapi.json --list

# Generate complete request/response fixture
cvt generate --schema ./openapi.json --endpoint "GET /users/{id}"

# Generate only request body
cvt generate --schema ./openapi.json --endpoint "POST /users" --output-type request

# Generate response with specific status code
cvt generate --schema ./openapi.json --endpoint "GET /users" --output-type response --status-code 200

# Use schema examples instead of generated data
cvt generate --schema ./openapi.json --endpoint "GET /users" --use-examples
```

### Output (Fixture)

```json
{
  "request": {
    "method": "GET",
    "path": "/users/abc123",
    "headers": {
      "Accept": "application/json"
    }
  },
  "response": {
    "status_code": 200,
    "headers": {
      "Content-Type": "application/json"
    },
    "body": "{\"id\":\"abc123\",\"name\":\"string\",\"email\":\"user@example.com\"}"
  }
}
```

### Endpoint List Output

```
Available Endpoints
===================

GET     /users              List all users
GET     /users/{id}         Get user by ID
POST    /users              Create a new user
PUT     /users/{id}         Update user
DELETE  /users/{id}         Delete user
GET     /users/{id}/orders  Get user's orders

Total: 6 endpoints
```

---

## can-i-deploy

Check if a schema version can be safely deployed without breaking registered consumers.

### Usage

```bash
cvt can-i-deploy --schema <path> [options]
```

### Options

| Flag | Description | Required |
|------|-------------|----------|
| `--schema` | Path to schema file or schema ID | Yes |
| `--version` | Version to check for deployment | No |
| `--env` | Target environment (dev, staging, prod) | No (default: `prod`) |
| `--server` | CVT server address | No (default: `localhost:9550`) |
| `--json` | Output results as JSON | No |
| `--timeout` | Request timeout | No (default: `30s`) |

### Examples

```bash
# Check deployment safety against local server
cvt can-i-deploy --schema ./openapi.json --server localhost:9550

# Check specific version for production
cvt can-i-deploy --schema user-api --version 2.0.0 --env prod

# JSON output for CI/CD
cvt can-i-deploy --schema user-api --version 2.0.0 --env prod --json

# Custom timeout for slow connections
cvt can-i-deploy --schema user-api --version 2.0.0 --server cvt.internal:9550 --timeout 60s
```

### Output (Safe)

```
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

### Output (Unsafe)

```
Deployment Safety Check
=======================
Schema:      user-api
Version:     2.0.0
Environment: prod
Server:      localhost:9550

Result: UNSAFE TO DEPLOY

Breaking changes in v2.0.0:
  - FIELD_REMOVED: GET /users/{id} response removed 'email'
  - ENDPOINT_REMOVED: DELETE /users/{id}

Affected consumers in production:

  order-service v2.1.0
    Schema version: 1.0.0
    Impact: BREAKING
    Uses: GET /users/{id} (fields: id, email, name)

  billing-service v1.0.0
    Schema version: 1.0.0
    Impact: None

Summary:
  Safe consumers:     1/2
  Affected consumers: 1/2

Recommendation: Coordinate with order-service team before deploying.
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Safe to deploy |
| 1 | Unsafe to deploy (breaking changes affect consumers) |
| 2 | Invalid arguments or connection error |

---

## serve

Start the CVT gRPC server.

### Usage

```bash
cvt serve [options]
```

### Options

| Flag | Description | Default |
|------|-------------|---------|
| `--port` | gRPC server port | `9550` |
| `--metrics-port` | Prometheus metrics port | `9551` |
| `--tls` | Enable TLS | `false` |
| `--cert` | Path to TLS certificate | - |
| `--key` | Path to TLS private key | - |
| `--ca` | Path to CA certificate (for mTLS) | - |
| `--client-auth` | Client auth mode: none, request, require | `none` |
| `--api-key-auth` | Enable API key authentication | `false` |

### Examples

```bash
# Start with defaults
cvt serve

# Custom ports
cvt serve --port 50051 --metrics-port 50052

# Enable TLS
cvt serve --tls --cert ./certs/server.crt --key ./certs/server.key

# Enable mTLS (mutual TLS)
cvt serve --tls --cert ./certs/server.crt --key ./certs/server.key \
  --ca ./certs/ca.crt --client-auth require

# Enable API key authentication
cvt serve --api-key-auth
```

### Environment Variables

The serve command also respects environment variables. See [Configuration Reference](./configuration.md) for the complete list.

---

## version

Display version information.

### Usage

```bash
cvt version
```

### Output

```
CVT - Contract Validator Toolkit
Version: 1.0.0
Git Commit: abc1234
Build Date: 2024-01-15T10:30:00Z
Go Version: go1.21.5
```

---

## Global Options

These options work with all commands:

| Flag | Description |
|------|-------------|
| `--help`, `-h` | Show help for command |
| `--verbose`, `-v` | Enable verbose output |

---

## CI/CD Integration

### GitHub Actions

```yaml
- name: Check for breaking changes
  run: |
    cvt compare --old ./main-schema.json --new ./pr-schema.json
    if [ $? -ne 0 ]; then
      echo "Breaking changes detected!"
      exit 1
    fi

- name: Check deployment safety
  run: |
    cvt can-i-deploy \
      --schema my-api \
      --version ${{ github.sha }} \
      --env prod \
      --server ${{ secrets.CVT_SERVER }} \
      --json > deploy-check.json

    if [ $(jq '.safe_to_deploy' deploy-check.json) != "true" ]; then
      echo "Unsafe to deploy!"
      jq '.affected_consumers' deploy-check.json
      exit 1
    fi
```

### GitLab CI

```yaml
check-breaking-changes:
  script:
    - cvt compare --old $CI_MERGE_REQUEST_TARGET_BRANCH_SHA:openapi.json --new ./openapi.json --json
  allow_failure: false
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
      changes:
        - openapi.json

deployment-safety:
  script:
    - cvt can-i-deploy --schema $SCHEMA_ID --version $CI_COMMIT_SHA --env prod --json
  allow_failure: false
```

---

## Related Documentation

- **[API Reference](./api.md)** - gRPC API details
- **[Configuration Reference](./configuration.md)** - Environment variables
- **[Breaking Changes Guide](../guides/breaking-changes.md)** - Understanding schema compatibility
