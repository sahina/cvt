---
title: Configuration Reference
sidebar_label: Configuration
sidebar_position: 3
description: CVT environment variables and configuration options
---

# Configuration Reference

CVT is configured primarily through environment variables. This document provides a comprehensive reference for all configuration options.

## Quick Reference

| Category | Key Variables |
|----------|---------------|
| [Core](#core-settings) | `CVT_PORT`, `CVT_METRICS_PORT`, `LOG_LEVEL` |
| [TLS](#tls-configuration) | `CVT_TLS_ENABLED`, `CVT_TLS_CERT_FILE`, `CVT_TLS_KEY_FILE` |
| [Authentication](#api-key-authentication) | `CVT_API_KEY_ENABLED`, `CVT_API_KEYS` |
| [Storage](#persistent-storage) | `CVT_STORAGE_ENABLED`, `CVT_STORAGE_TYPE` |
| [PostgreSQL](#postgresql-settings) | `CVT_POSTGRES_HOST`, `CVT_POSTGRES_USER`, `CVT_POSTGRES_PASSWORD` |
| [Cache](#cache-settings) | `CVT_STORAGE_CACHE_ENABLED`, `CVT_STORAGE_CACHE_MAX_SCHEMAS` |

---

## Core Settings

Basic server configuration.

| Variable | Default | Description |
|----------|---------|-------------|
| `CVT_PORT` | `9550` | gRPC server port |
| `CVT_METRICS_PORT` | `9551` | Prometheus metrics endpoint port |
| `LOG_LEVEL` | `info` | Logging level: `debug`, `info`, `warn`, `error` |

### Example

```bash
CVT_PORT=9550 CVT_METRICS_PORT=9551 LOG_LEVEL=debug cvt serve
```

---

## TLS Configuration

Secure communication settings.

| Variable | Default | Description |
|----------|---------|-------------|
| `CVT_TLS_ENABLED` | `false` | Enable TLS encryption |
| `CVT_TLS_CERT_FILE` | `/certs/server.crt` | Path to server TLS certificate |
| `CVT_TLS_KEY_FILE` | `/certs/server.key` | Path to server TLS private key |
| `CVT_TLS_CA_FILE` | - | Path to CA certificate for mTLS client verification |
| `CVT_TLS_CLIENT_AUTH` | `none` | Client authentication mode: `none`, `request`, `require` |

### Client Authentication Modes

| Mode | Behavior |
|------|----------|
| `none` | No client certificate required |
| `request` | Request client certificate but don't require it |
| `require` | Require valid client certificate (mTLS) |

### Example: TLS Only

```bash
CVT_TLS_ENABLED=true \
CVT_TLS_CERT_FILE=./certs/server.crt \
CVT_TLS_KEY_FILE=./certs/server.key \
cvt serve
```

### Example: Mutual TLS (mTLS)

```bash
CVT_TLS_ENABLED=true \
CVT_TLS_CERT_FILE=./certs/server.crt \
CVT_TLS_KEY_FILE=./certs/server.key \
CVT_TLS_CA_FILE=./certs/ca.crt \
CVT_TLS_CLIENT_AUTH=require \
cvt serve
```

---

## API Key Authentication

Optional API key-based authentication for gRPC requests.

| Variable | Default | Description |
|----------|---------|-------------|
| `CVT_API_KEY_ENABLED` | `false` | Enable API key authentication |
| `CVT_API_KEYS` | - | Comma-separated list of valid API keys |
| `CVT_API_KEYS_FILE` | - | Path to JSON file with API key configuration |

### Example: Inline Keys

```bash
CVT_API_KEY_ENABLED=true \
CVT_API_KEYS="key1,key2,key3" \
cvt serve
```

### Example: Keys File

```bash
CVT_API_KEY_ENABLED=true \
CVT_API_KEYS_FILE=./config/api-keys.json \
cvt serve
```

### API Keys File Format

```json
{
  "keys": [
    {
      "key": "sk_live_abc123",
      "name": "Production Key",
      "permissions": ["read", "write"]
    },
    {
      "key": "sk_test_xyz789",
      "name": "Test Key",
      "permissions": ["read"]
    }
  ]
}
```

### Sending API Keys

Clients must include the API key in the `x-api-key` metadata header:

```typescript
// Node.js SDK
const client = new CVTClient('localhost:9550', {
  metadata: { 'x-api-key': 'your-api-key-here' }
});
```

```python
# Python SDK
client = CVTClient('localhost:9550', api_key='your-api-key-here')
```

---

## Persistent Storage

Configure backend storage for schemas, consumers, and validation records.

| Variable | Default | Description |
|----------|---------|-------------|
| `CVT_STORAGE_ENABLED` | `false` | Enable persistent storage (default uses in-memory) |
| `CVT_STORAGE_TYPE` | `sqlite` | Storage backend: `memory`, `sqlite`, `postgres` |
| `CVT_STORAGE_DSN` | `cvt.db` | Data source name for SQLite |
| `CVT_VALIDATION_RETENTION_DAYS` | `90` | Days to retain validation records |

### Storage Types

| Type | Use Case | Persistence |
|------|----------|-------------|
| `memory` | Development, testing | None (lost on restart) |
| `sqlite` | Single-instance deployments | Local file |
| `postgres` | Production, multi-instance | External database |

### Example: SQLite

```bash
CVT_STORAGE_ENABLED=true \
CVT_STORAGE_TYPE=sqlite \
CVT_STORAGE_DSN=./data/cvt.db \
cvt serve
```

### Example: PostgreSQL

```bash
CVT_STORAGE_ENABLED=true \
CVT_STORAGE_TYPE=postgres \
CVT_POSTGRES_HOST=db.example.com \
CVT_POSTGRES_USER=cvt \
CVT_POSTGRES_PASSWORD=secret \
CVT_POSTGRES_DB=cvt \
cvt serve
```

---

## PostgreSQL Settings

Configuration for PostgreSQL storage backend.

| Variable | Default | Description |
|----------|---------|-------------|
| `CVT_POSTGRES_HOST` | `localhost` | PostgreSQL host |
| `CVT_POSTGRES_PORT` | `5432` | PostgreSQL port |
| `CVT_POSTGRES_USER` | `cvt` | PostgreSQL user |
| `CVT_POSTGRES_PASSWORD` | - | PostgreSQL password |
| `CVT_POSTGRES_DB` | `cvt` | PostgreSQL database name |
| `CVT_POSTGRES_SSLMODE` | `disable` | SSL mode: `disable`, `require`, `verify-ca`, `verify-full` |
| `CVT_POSTGRES_DSN` | - | Full PostgreSQL DSN (alternative to individual settings) |
| `CVT_POSTGRES_MAX_CONNS` | `25` | Maximum database connections |

### Individual Settings vs DSN

You can configure PostgreSQL either with individual variables or a single DSN:

**Individual Variables:**
```bash
CVT_POSTGRES_HOST=db.example.com \
CVT_POSTGRES_PORT=5432 \
CVT_POSTGRES_USER=cvt \
CVT_POSTGRES_PASSWORD=secret \
CVT_POSTGRES_DB=cvt \
CVT_POSTGRES_SSLMODE=require \
cvt serve
```

**Single DSN:**
```bash
CVT_POSTGRES_DSN="postgres://cvt:secret@db.example.com:5432/cvt?sslmode=require" \
cvt serve
```

If both are provided, `CVT_POSTGRES_DSN` takes precedence.

### SSL Modes

| Mode | Description |
|------|-------------|
| `disable` | No SSL (not recommended for production) |
| `require` | Use SSL but don't verify certificate |
| `verify-ca` | Verify server certificate is signed by trusted CA |
| `verify-full` | Verify CA and that server hostname matches certificate |

---

## Cache Settings

In-memory cache configuration (used with persistent storage).

| Variable | Default | Description |
|----------|---------|-------------|
| `CVT_STORAGE_CACHE_ENABLED` | `true` | Enable in-memory cache with persistent storage |
| `CVT_STORAGE_CACHE_MAX_SCHEMAS` | `1000` | Maximum number of schemas in cache |

The cache uses [Ristretto](https://github.com/dgraph-io/ristretto) with a 24-hour TTL by default.

### Example

```bash
CVT_STORAGE_ENABLED=true \
CVT_STORAGE_TYPE=postgres \
CVT_STORAGE_CACHE_ENABLED=true \
CVT_STORAGE_CACHE_MAX_SCHEMAS=5000 \
cvt serve
```

---

## Port Configuration Summary

| Port | Purpose |
|------|---------|
| `9550` | gRPC server (configurable via `CVT_PORT`) |
| `9551` | Prometheus metrics (configurable via `CVT_METRICS_PORT`) |
| `9091` | Prometheus UI (when using Docker observability stack) |
| `3000` | Grafana UI (when using Docker observability stack) |

---

## Docker Configuration

### docker-compose.yml Example

```yaml
services:
  cvt-server:
    image: cvt-server:latest
    ports:
      - "9550:9550"
      - "9551:9551"
    environment:
      - CVT_PORT=9550
      - CVT_METRICS_PORT=9551
      - LOG_LEVEL=info
      - CVT_STORAGE_ENABLED=true
      - CVT_STORAGE_TYPE=postgres
      - CVT_POSTGRES_HOST=postgres
      - CVT_POSTGRES_USER=cvt
      - CVT_POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
      - CVT_POSTGRES_DB=cvt
      # TLS (optional)
      # - CVT_TLS_ENABLED=true
      # - CVT_TLS_CERT_FILE=/certs/server.crt
      # - CVT_TLS_KEY_FILE=/certs/server.key
    volumes:
      - ./certs:/certs:ro
    depends_on:
      - postgres

  postgres:
    image: postgres:15
    environment:
      - POSTGRES_USER=cvt
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
      - POSTGRES_DB=cvt
    volumes:
      - postgres-data:/var/lib/postgresql/data

volumes:
  postgres-data:
```

### Environment File (.env)

```bash
# Core
CVT_PORT=9550
CVT_METRICS_PORT=9551
LOG_LEVEL=info

# Storage
CVT_STORAGE_ENABLED=true
CVT_STORAGE_TYPE=postgres

# PostgreSQL
CVT_POSTGRES_HOST=localhost
CVT_POSTGRES_PORT=5432
CVT_POSTGRES_USER=cvt
CVT_POSTGRES_PASSWORD=your-secret-password
CVT_POSTGRES_DB=cvt
CVT_POSTGRES_SSLMODE=require

# Security
CVT_API_KEY_ENABLED=true
CVT_API_KEYS_FILE=/etc/cvt/api-keys.json

# TLS
CVT_TLS_ENABLED=true
CVT_TLS_CERT_FILE=/etc/cvt/certs/server.crt
CVT_TLS_KEY_FILE=/etc/cvt/certs/server.key
```

---

## Production Recommendations

### Security

1. **Enable TLS** in production environments
2. **Use mTLS** for service-to-service communication
3. **Enable API key authentication** to control access
4. **Use PostgreSQL SSL mode** `verify-full` for database connections
5. **Rotate API keys** regularly

### Storage

1. **Use PostgreSQL** for production deployments
2. **Configure connection pooling** appropriately (`CVT_POSTGRES_MAX_CONNS`)
3. **Set up database backups** for disaster recovery
4. **Monitor disk usage** for validation record retention

### Observability

1. **Expose metrics endpoint** to your monitoring system
2. **Configure log aggregation** for centralized logging
3. **Set appropriate log level** (`info` for production, `debug` for troubleshooting)

---

## Related Documentation

- **[CLI Reference](./cli.mdx)** - Command-line options
- **[Observability Guide](../operations/observability.md)** - Metrics and monitoring
- **[Development Guide](../development/contributing.md)** - Local development setup
