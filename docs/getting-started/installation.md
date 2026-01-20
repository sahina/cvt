---
title: Installation
sidebar_label: Installation
sidebar_position: 1
description: Install CVT server and SDKs
---

# Installation

This guide covers installing the CVT server and client SDKs.

## Server Installation

### Docker (Recommended)

The easiest way to run CVT is with Docker:

```bash
# Using Docker Compose (includes observability stack)
make up

# Or Docker directly
docker run -d -p 9550:9550 -p 9551:9551 ghcr.io/cvt/cvt-server:latest
```

### From Source

Build and run the server locally:

```bash
# Clone the repository
git clone https://github.com/sahina/cvt.git
cd cvt

# Build the server
make build

# Run locally
make run-server
```

### Binary Installation

Download a pre-built binary:

```bash
# Linux
curl -L https://github.com/sahina/cvt/releases/latest/download/cvt-linux-amd64 -o cvt
chmod +x cvt

# macOS (Intel)
curl -L https://github.com/sahina/cvt/releases/latest/download/cvt-darwin-amd64 -o cvt
chmod +x cvt

# macOS (Apple Silicon)
curl -L https://github.com/sahina/cvt/releases/latest/download/cvt-darwin-arm64 -o cvt
chmod +x cvt

# Windows
curl -L https://github.com/sahina/cvt/releases/latest/download/cvt-windows-amd64.exe -o cvt.exe
```

### Go Install

```bash
go install github.com/sahina/cvt/cmd/cvt@latest
```

---

## SDK Installation

Install the SDK for your language:

### Node.js

The Node.js SDK is not published to npm. Install from a local clone:

```bash
# Clone the repository (if you haven't already)
git clone https://github.com/sahina/cvt.git

# Install from local path
cd your-project
npm install ../cvt/sdks/node

# Or with pnpm
pnpm add ../cvt/sdks/node
```

### Python

The Python SDK is not published to PyPI. Install from a local clone:

```bash
# Clone the repository (if you haven't already)
git clone https://github.com/sahina/cvt.git

# Install from local path
pip install ./cvt/sdks/python

# Or with uv
uv pip install ./cvt/sdks/python
```

### Go

```bash
go get github.com/cvt/cvt-sdk/go
```

### Java

The Java SDK is not published to Maven Central. Build and publish to your local Maven repository:

```bash
# Clone the repository (if you haven't already)
git clone https://github.com/sahina/cvt.git
cd cvt/sdks/java

# Build and publish to local Maven repository
./gradlew publishToMavenLocal
```

Then add to your `build.gradle`:

```gradle
repositories {
    mavenLocal()
    mavenCentral()
}

dependencies {
    implementation 'com.cvt:cvt-sdk:1.0.0'
}
```

Or for Maven, add to your `pom.xml`:

```xml
<dependency>
    <groupId>com.cvt</groupId>
    <artifactId>cvt-sdk</artifactId>
    <version>1.0.0</version>
</dependency>
```

---

## Verify Installation

### Check Server

```bash
# Health check
make health

# Or use grpc-health-probe
grpc-health-probe -addr=localhost:9550

# Check metrics endpoint
curl http://localhost:9551/metrics
```

### Check SDK

```typescript
// Node.js
import { ContractValidator } from "@cvt/cvt-sdk";
const validator = new ContractValidator("localhost:9550");
console.log("Connected!");
```

```python
# Python
from cvt_sdk import ContractValidator
validator = ContractValidator('localhost:9550')
print('Connected!')
```

```go
// Go
import "github.com/cvt/cvt-sdk/go/cvt"
client, _ := cvt.NewValidator("localhost:9550")
fmt.Println("Connected!")
```

---

## Port Configuration

| Port | Service                                  |
| ---- | ---------------------------------------- |
| 9550 | gRPC server                              |
| 9551 | Prometheus metrics                       |
| 9091 | Prometheus UI (with observability stack) |
| 3000 | Grafana UI (with observability stack)    |

---

## Next Steps

- **[Quick Start](./quick-start.mdx)** - Your first contract test
- **[Consumer Testing Guide](../guides/consumer-testing.md)** - Test API integrations
- **[Producer Testing Guide](../guides/producer-testing.md)** - Validate your APIs
- **[Configuration Reference](../reference/configuration.md)** - Server settings
