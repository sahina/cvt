# CVT CLI Examples

This directory contains sample files for testing the CVT CLI.

## Files

| File                    | Description                                    |
| ----------------------- | ---------------------------------------------- |
| `schema.json`           | Sample OpenAPI 3.0 schema (User API)           |
| `request.json`          | Valid GET request                              |
| `response.json`         | Valid response for GET request                 |
| `interaction.json`      | Complete request/response pair (POST)          |
| `invalid-response.json` | Invalid response (wrong types, missing fields) |

## Usage Examples

### Validate a request/response pair

```bash
# Using separate request and response files
cvt validate \
  --schema ./cmd/cvt/examples/schema.json \
  --request ./cmd/cvt/examples/request.json \
  --response ./cmd/cvt/examples/response.json

# Using a combined interaction file
cvt validate \
  --schema ./cmd/cvt/examples/schema.json \
  --interaction ./cmd/cvt/examples/interaction.json
```

### Validate with JSON output

```bash
cvt validate \
  --schema ./cmd/cvt/examples/schema.json \
  --request ./cmd/cvt/examples/request.json \
  --response ./cmd/cvt/examples/response.json \
  --json
```

### Test validation failure

```bash
# This should report validation errors (missing required field, wrong type)
cvt validate \
  --schema ./cmd/cvt/examples/schema.json \
  --request ./cmd/cvt/examples/request.json \
  --response ./cmd/cvt/examples/invalid-response.json
```

### Compare schemas for breaking changes

```bash
# Compare two versions of a schema
cvt compare \
  --old ./sdks/shared/openapi-v1.json \
  --new ./sdks/shared/openapi-v2-breaking.json
```

## Expected Output

### Valid interaction

```shell
Validation passed!
```

### Invalid interaction

```shell
Validation failed:
- response.body.id: expected integer, got string
- response.body.email: missing required field
```
