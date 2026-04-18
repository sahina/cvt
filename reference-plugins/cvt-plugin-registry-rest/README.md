# cvt-plugin-registry-rest

REST schema-registry backend for CVT. Implements the `RegistryProvider`
plugin contract.

## Endpoints

- `GET {base_url}/schemas/{schemaId}/versions/{version}/spec` — fetch
  raw OpenAPI spec bytes. `version = "latest"` supported. Response
  header `X-Schema-Version` (optional) carries the resolved version
  for "latest" requests.
- `POST {base_url}/schemas/{schemaId}/consumers` — record consumer
  usage. JSON body:
  ```json
  {
    "consumerId": "order-service",
    "schemaVersion": "2.1.0",
    "environment": "ci",
    "endpoints": [{"method": "GET", "path": "/pets/{id}"}]
  }
  ```
  MUST be idempotent upsert on the server side.

## Build

```sh
go build -o cvt-plugin-registry-rest .
cvt plugins install ./cvt-plugin-registry-rest
```

## Config

```yaml
plugins:
  registry:
    binary: ~/.cvt/plugins/cvt-plugin-registry-rest
    timeout: 5s
    on_error: fail_closed
    secrets: [token]
    config:
      base_url: https://registry.example.com/api/v1
      token: ${CVT_REGISTRY_TOKEN}
hooks:
  fetch_schema: registry
  register_consumer_usage: registry
```

## gRPC status codes

| Code | When |
|---|---|
| `NotFound` | registry returned 404 |
| `InvalidArgument` | registry returned 4xx other than 404; or request missing required fields |
| `Unavailable` | registry unreachable or 5xx |
| `FailedPrecondition` | `base_url` not configured |
