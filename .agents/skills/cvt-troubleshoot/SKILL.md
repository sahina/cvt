---
name: cvt-troubleshoot
description: Diagnose and fix common CVT contract testing issues
sdk_version: "0.1.0"
---

# CVT Troubleshoot

## Prerequisites

- CVT SDK installed in the project
- A failing contract test or CVT error to diagnose

## Goal

Systematically diagnose and resolve common CVT issues by following a structured diagnostic sequence from connectivity through schema validation to runtime errors.

## Steps

Follow this diagnostic sequence in order. Stop at the first failure and apply the fix before proceeding.

### Step 1: Server Connectivity

Verify the CVT server is reachable:

```bash
cvt wait --server localhost:9550 --timeout 10
```

If this fails:
- **Server not running**: Start with `cvt serve --port 9550` or `docker run -p 9550:9550 ghcr.io/sahina/cvt:latest`
- **Wrong address**: Check the server address in your test configuration. Default port is 9550.
- **Firewall/network**: Ensure the port is open. In Docker, verify port mapping with `docker ps`.
- **DNS resolution**: If using a hostname, verify it resolves with `nslookup <hostname>`.

### Step 2: Authentication

If the server requires API keys:

```bash
# Check if auth is enabled
curl -s http://localhost:9551/metrics | grep cvt_auth
```

- Ensure `CVT_API_KEY` environment variable is set in your test environment.
- Verify the key is in the server's allowed list (`CVT_API_KEYS` or `CVT_API_KEYS_FILE`).
- Check for typos or trailing whitespace in the key value.

### Step 3: TLS Configuration

If using TLS (`CVT_TLS_ENABLED=true`):

- Verify certificate files exist at the configured paths.
- Check certificate expiry: `openssl x509 -enddate -noout -in server.crt`
- For mTLS (`CVT_TLS_CLIENT_AUTH=require`), ensure the client has a valid certificate and the CA cert is configured.
- Confirm the SDK is configured to use TLS (not plain gRPC).

### Step 4: Schema Registration

Verify the schema is registered and parseable:

```bash
# List registered schemas
cvt generate --schema ./openapi.json --list
```

Common schema issues:

| Symptom | Cause | Fix |
|---|---|---|
| `file not found` | Wrong schema path | Use absolute path or verify relative path from test working directory |
| `parse error` | Invalid JSON/YAML | Validate with `python -m json.tool openapi.json` or an online validator |
| `unsupported version` | Schema version not recognized | CVT supports OpenAPI v2 (Swagger) and v3; check the `openapi` or `swagger` field |
| `$ref resolution failed` | Broken JSON references | Ensure all `$ref` targets exist; external refs need to be reachable |
| `duplicate schema ID` | Re-registration with different content | Use a unique schema ID or a new version |

### Step 5: Route Matching

If validation returns `no route matched`:

- **Check the exact path**: The path in your test must match the schema path with actual values substituted for parameters (e.g., `/users/123` not `/users/{id}`).
- **Check path prefix**: Some schemas define a `basePath` (v2) or `servers[].url` path prefix (v3). Your test path must include this prefix.
- **Check HTTP method**: The method must match (GET, POST, PUT, DELETE, PATCH). Methods are case-insensitive.
- **List available routes**: Use `cvt generate --schema ./openapi.json --list` to see all defined endpoints.

### Step 6: Request Validation Errors

If the request fails validation:

| Error | Cause | Fix |
|---|---|---|
| `request body has an error: value is required` | Missing required field in request body | Add all fields marked `required` in the schema |
| `request body has an error: doesn't match schema` | Wrong field type | Check that numbers are numbers, strings are strings, etc. |
| `header Content-Type has unexpected value` | Missing or wrong Content-Type | Set `Content-Type: application/json` for JSON bodies |
| `parameter ... is required` | Missing required query/header parameter | Add the required parameter to headers or query string |
| `path parameter ... not found` | Path param mismatch | Ensure path params in the URL match the schema (e.g., `{id}` vs `{userId}`) |

### Step 7: Response Validation Errors

If the response fails validation:

| Error | Cause | Fix |
|---|---|---|
| `response status code not defined` | Status code not in schema | Add the status code to your schema or use a defined one |
| `response body has an error: value is required` | Missing required field in response | Include all required fields in the response body |
| `response body has an error: additionalProperties` | Extra fields in response | Either remove extra fields or set `additionalProperties: true` in schema |
| `response body has an error: type mismatch` | Wrong field type in response | Match the exact types: `"id": 1` not `"id": "1"` |
| `response header ... is required` | Missing required response header | Add the required header to the response |

### Step 8: Body Serialization

A frequent issue is passing objects instead of serialized strings:

```javascript
// WRONG: passing an object
body: { name: "Alice" }

// CORRECT: passing a JSON string
body: JSON.stringify({ name: "Alice" })
// or
body: '{"name":"Alice"}'
```

This applies to all SDKs. The `body` field expects a string, not a native object.

### Step 9: Version Mismatch

If tests passed before but now fail:

- **Schema changed**: The provider may have updated the schema. Re-register with the latest version.
- **SDK version**: Ensure all team members use the same SDK version (currently 0.1.0).
- **Server version**: If the CVT server was upgraded, check the changelog for breaking changes.
- **Compare schemas**: Use `cvt compare --old ./old-openapi.json --new ./new-openapi.json` to see what changed.

### Step 10: Performance and Timeouts

If tests are slow or timing out:

- **Server cold start**: First request may be slow due to schema parsing. Use `cvt wait` before tests.
- **Large schemas**: Schemas with hundreds of endpoints take longer to parse. Consider splitting by domain.
- **Connection pooling**: Reuse the validator instance across tests instead of creating one per test.
- **gRPC deadline**: Increase the timeout if the server is remote or under load.

## SDK-Specific Instructions

### Node.js

```bash
# Enable debug logging
LOG_LEVEL=debug npm test -- --testPathPattern=contract
```

### Python

```bash
# Enable debug logging
LOG_LEVEL=debug pytest tests/contract/ -v -s
```

### Go

```bash
# Enable debug logging
LOG_LEVEL=debug go test ./contract/... -v
```

### Java

```bash
# Enable debug logging
LOG_LEVEL=debug mvn test -Dtest="*Contract*"
```

## Common Errors

| Error | Cause | Fix |
|---|---|---|
| `connection refused on :9550` | Server not running | Start server or check address |
| `no route matched for path` | Path not in schema or wrong prefix | Verify path against schema endpoints |
| `request body has an error` | Request body does not match schema | Check required fields and types |
| `response body has an error` | Response body does not match schema | Check required fields, types, and additionalProperties |
| `authentication required` | API key missing or invalid | Set `CVT_API_KEY` environment variable |
| `TLS handshake failed` | Certificate issue | Check cert paths, expiry, and CA trust |
| `deadline exceeded` | Server overloaded or unreachable | Increase timeout or check server health |
| `schema not found` | Schema ID not registered | Register with `registerSchema()` before validating |

## Success Criteria

- The root cause of the failure is identified from the diagnostic sequence
- The fix is applied and the contract test now passes
- If the issue was a schema mismatch, the provider team is notified
