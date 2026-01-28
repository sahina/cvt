---
title: OpenAPI Support
sidebar_label: OpenAPI Support
sidebar_position: 5
description: OpenAPI v2 and v3 support, gaps, and limitations in CVT
---

# OpenAPI Support

CVT supports both **OpenAPI v2 (Swagger 2.0)** and **OpenAPI v3.x** specifications. This document covers the support level, conversion behavior, and known limitations.

## Supported Versions

| Version | Support Level | Notes |
|---------|---------------|-------|
| OpenAPI 3.0.x | Full | Native support |
| OpenAPI 3.1.x | Full | Native support |
| Swagger 2.0 | Full | Auto-converted to v3 |
| Swagger 1.x | Not supported | Will return an error |

## How v2 to v3 Conversion Works

When you register a Swagger 2.0 schema, CVT automatically converts it to OpenAPI v3 for unified validation. This conversion is handled by the [kin-openapi](https://github.com/getkin/kin-openapi) library.

```
Swagger 2.0 Schema
       ↓
   Detection (checks for "swagger": "2.x")
       ↓
   Parse as OpenAPI v2
       ↓
   Convert to OpenAPI v3 (openapi2conv.ToV3)
       ↓
   Validate using v3 engine
```

### What Gets Converted

| v2 Feature | v3 Equivalent | Status |
|------------|---------------|--------|
| `basePath` | `servers[].url` | ✅ Converted |
| `host` | `servers[].url` | ✅ Converted |
| `schemes` | `servers[].url` | ✅ Converted |
| `consumes` | `requestBody.content` | ✅ Converted |
| `produces` | `responses.content` | ✅ Converted |
| `definitions` | `components/schemas` | ✅ Converted |
| `parameters` (body) | `requestBody` | ✅ Converted |
| `parameters` (formData) | `requestBody` with encoding | ✅ Converted |
| `securityDefinitions` | `components/securitySchemes` | ✅ Converted |
| `x-*` extensions | Preserved | ✅ Converted |
| `externalDocs` | Preserved | ✅ Converted |
| `type: file` | `format: binary` | ✅ Converted |

---

## Known Gaps and Limitations

### 1. `collectionFormat` Not Converted

**Severity: Medium**

OpenAPI v2's `collectionFormat` parameter is **not converted** to the equivalent v3 `style`/`explode` parameters.

#### What This Means

In Swagger 2.0, array query parameters use `collectionFormat` to specify serialization:

```json
{
  "name": "status",
  "in": "query",
  "type": "array",
  "collectionFormat": "pipes"
}
```

The expected v3 mapping:

| v2 `collectionFormat` | v3 `style` | v3 `explode` | Example |
|----------------------|------------|--------------|---------|
| `csv` (default) | `form` | `false` | `?status=a,b,c` |
| `ssv` | `spaceDelimited` | `false` | `?status=a%20b%20c` |
| `tsv` | N/A | N/A | No v3 equivalent |
| `pipes` | `pipeDelimited` | `false` | `?status=a\|b\|c` |
| `multi` | `form` | `true` | `?status=a&status=b` |

**Current Behavior:** The `collectionFormat` field is silently dropped during conversion. The v3 parameter uses default serialization (`style: form`, `explode: true`).

**Impact by Format:**

| collectionFormat | Validation Correct? | Notes |
|------------------|---------------------|-------|
| `multi` | ✅ Yes | v3 default matches `multi` |
| `csv` | ⚠️ Partial | May accept incorrect formats |
| `ssv` | ❌ No | Validation will not enforce space-delimited |
| `tsv` | ❌ No | No v3 equivalent exists |
| `pipes` | ❌ No | Validation will not enforce pipe-delimited |

**Workaround:** Migrate schemas using `ssv`, `tsv`, or `pipes` to OpenAPI v3 with explicit `style` and `explode` settings.

---

### 2. Multiple formData Parameters

**Severity: Low**

When a v2 operation has multiple `formData` parameters alongside a `body` parameter, only one is selected for the v3 `requestBody`.

**Reason:** OpenAPI v3 has a single `requestBody` object, while v2 allowed both `body` and `formData` parameters on the same operation.

**Workaround:** Ensure v2 operations use either `body` OR `formData`, not both.

---

### 3. Tab-Separated Values (`tsv`)

**Severity: Low**

The `collectionFormat: tsv` has no direct equivalent in OpenAPI v3. If your v2 schema uses tab-separated array parameters, validation cannot be accurately performed.

**Workaround:** Use a different serialization format or migrate to v3 with custom validation.

---

## v3-Specific Considerations

### `oneOf`, `anyOf`, `allOf` Validation

CVT fully supports JSON Schema composition keywords. When generating mock responses:

- `allOf`: All sub-schemas are merged
- `oneOf`/`anyOf`: The first option is used for generation

### `nullable` vs `type: null`

Both OpenAPI 3.0 (`nullable: true`) and OpenAPI 3.1 (`type: ["string", "null"]`) nullable syntax are supported.

### `discriminator` Support

Discriminator-based polymorphism is supported for validation but not used during mock response generation.

---

## Best Practices

### When to Use v2 vs v3

| Scenario | Recommendation |
|----------|----------------|
| New projects | Use OpenAPI v3.1 |
| Legacy APIs with simple schemas | v2 works fine |
| Complex array serialization | Migrate to v3 |
| Using `collectionFormat` other than `multi` | Migrate to v3 |

### Validating Your Schema

Before registering a schema, you can validate it will work correctly:

```bash
# Check if schema parses correctly
cvt validate --schema ./your-schema.json \
  --request request.json \
  --response response.json
```

### Testing v2 Conversion

To verify your v2 schema converts correctly:

1. Register the schema
2. Use `GetSchema` to retrieve it (returns the converted v3 version internally)
3. Test all endpoints, especially those with array parameters

---

## Reporting Issues

If you encounter validation discrepancies between v2 and v3 schemas:

1. Check if the issue is in this known limitations list
2. Test with both the original v2 and a manually-converted v3 schema
3. Report issues at [CVT GitHub Issues](https://github.com/sahina/cvt/issues)

Include:
- The original v2 schema (or relevant excerpt)
- The request/response that failed validation
- Expected vs actual behavior
