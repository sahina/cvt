# Enterprise Deployment Models for CVT

**Date:** 2026-04-02
**Status:** Draft
**Author:** Design brainstorm session

## Problem Statement

When a large enterprise adopts CVT, the question of how to deploy it has no clear answer today. The current architecture assumes a single gRPC server backed by a database — a model that introduces blast radius concerns, maintenance burden, performance/scale challenges, and doesn't integrate with existing enterprise schema registries.

This design introduces multiple deployment topologies, a plugin system for schema and contract sources, file-based storage for git-native workflows, and a notification system for coordinating breaking changes across teams.

### Goals

- Support enterprises with hundreds of teams and APIs without requiring centralized infrastructure
- Eliminate the database as a hard dependency for most use cases
- Integrate with existing schema registries (homegrown, GitHub Enterprise, etc.)
- Enable cross-team contract validation without a shared server
- Maintain full backward compatibility with the existing server-based model

### Non-Goals

- Multi-tenancy within a single CVT server instance (solved by topology isolation instead)
- Replacing existing schema registries (CVT consumes schemas, it doesn't own them)
- Building a notification routing service (fire-and-forget, not delivery guarantees)

---

## Section 1: Deployment Topologies

CVT supports three deployment models. Teams choose based on their org structure and infrastructure preferences.

### Model A: CLI-Only (Zero Infrastructure)

Teams run `cvt validate`, `cvt compare`, `cvt generate` in CI pipelines. Schemas are pulled from external sources (registry, GitHub, URL). No server, no database, no shared state.

**Best for:** Teams that just want schema validation and breaking change detection.

**Flow:**
```
CI Pipeline → cvt validate --schema <url-or-file> → pass/fail
```

### Model B: Centralized Contract Repo (Git-as-Database)

A shared git repo holds schemas, consumer contracts, and validation state as files. CI on the contract repo runs `can-i-deploy` and gates merges. Consumer contracts are auto-generated from SDK test runs and committed via PR.

**Best for:** Organizations that want cross-team visibility with minimal infrastructure.

**Repository structure:**
```
contracts-repo/
├── schemas/
│   └── payments-api/
│       ├── spec.yaml              # OpenAPI spec (or pointer to external source)
│       └── metadata.yaml          # version, owner, team
├── consumers/
│   └── checkout-service/
│       └── payments-api.yaml      # auto-generated contract: endpoints + fields used
└── .github/workflows/
    └── cvt-validate.yml           # CI: runs can-i-deploy on every PR
```

### Model C: Distributed Contracts (Contracts in Consumer Repos)

Consumer contracts live in each consumer's own repo under `.cvt/`. Producer repos list their known consumers or discover them via GitHub API. `can-i-deploy` fetches contracts from consumer repos at validation time.

**Best for:** Organizations where teams want full ownership of their contract definitions.

**Repository structure:**
```
checkout-service/                 (consumer repo)
└── .cvt/
    └── payments-api.yaml         # "I depend on these endpoints"

payments-api/                     (producer repo)
├── openapi.yaml
└── .cvt/
    └── consumers.yaml            # lists consumer repos (or uses discovery)
```

### Models B and C Can Coexist

Some teams use a contract repo, others keep contracts in their own repos. `can-i-deploy` queries both through the contract source plugin interface (see Section 3).

### Existing Server Model (Unchanged)

Teams using `cvt serve` with PostgreSQL/SQLite continue as before. This design adds new options — it doesn't replace the existing model.

---

## Section 2: Schema Provider Plugins

CVT currently requires schemas to be registered via gRPC or passed as local files. In the enterprise model, schemas already live elsewhere. Schema providers let CVT pull from any source.

### Provider Interface

```
cvt validate --schema <provider>://<reference> ...
```

### Built-in Providers

| Provider | Syntax | Description |
|----------|--------|-------------|
| `file` | `file://./openapi.yaml` or just `./openapi.yaml` | Local file (current behavior) |
| `url` | `https://api.example.com/openapi.json` | HTTP/HTTPS URL (current behavior) |
| `github` | `github://org/repo/path/openapi.yaml@ref` | GitHub Enterprise or github.com, fetches via GitHub API. `@ref` is optional (branch, tag, SHA) |
| `registry` | `registry://payments-api@1.0.0` | Pulls from an external schema registry via a configurable adapter |

### Provider URI Clarification

The `github://` scheme is **convenience sugar**, not a new capability. Anything it does can be accomplished with a raw HTTPS URL:

| Provider URI | Equivalent HTTPS URL |
|---|---|
| `github://org/repo/openapi.yaml@v1.0.0` | `https://raw.githubusercontent.com/org/repo/v1.0.0/openapi.yaml` |
| `github://org/repo/openapi.yaml@main` | `https://raw.githubusercontent.com/org/repo/main/openapi.yaml` |

**Why offer it anyway:**
- Shorter, more readable in config files and CI scripts
- Abstracts away GitHub Enterprise vs github.com host differences
- Auto-resolves auth from configured `GITHUB_TOKEN` without manual headers
- Consistent syntax with `registry://` — one pattern for all providers

**Where custom URIs are essential:** `registry://` is the only scheme that enables something you *can't* do with a plain URL. A homegrown schema registry has a proprietary API, authentication, and response format. `registry://payments-api@1.0.0` delegates to a configured adapter that knows how to fetch and extract the schema from that registry's specific API shape.

**The rule:** if a schema is reachable via a public or authenticated HTTPS URL, you can always use that URL directly. Provider URIs are an optional layer of convenience and consistency on top.

### Registry Adapter

The `registry://` provider delegates to a configurable adapter. CVT ships with none — this is where enterprises plug in their homegrown registry.

Configuration (`.cvt/config.yaml` or env vars):

```yaml
schema_providers:
  registry:
    adapter: http           # built-in: http, command
    http:
      base_url: https://schema-registry.internal/api/v1
      schema_path: /schemas/{name}/versions/{version}/content
      auth:
        type: bearer
        token_env: SCHEMA_REGISTRY_TOKEN
    # OR: shell out to a custom command
    command:
      exec: my-registry-cli fetch {name} --version {version}
```

Two adapter types cover most cases:

- **`http`**: Configurable URL template + auth. Works with any REST-based registry.
- **`command`**: Shells out to a CLI tool. Escape hatch for registries with proprietary CLIs or complex auth.

### How Providers Compose with Topologies

- **CLI-only (Model A):** `cvt validate --schema github://org/repo/openapi.yaml`
- **Contract repo (Model B):** `spec.yaml` can either contain the schema inline OR a provider reference (`source: github://org/payments-api/openapi.yaml@v1.0.0`)
- **Distributed (Model C):** Same — consumer contracts reference schemas by provider URI

---

## Section 3: Contract Source Interface

Just like schemas come from pluggable providers, `can-i-deploy` needs to find consumer contracts from pluggable sources.

### Contract Source Interface

```
cvt can-i-deploy --schema payments-api --version 2.0.0 \
  --contracts <source>
```

### Built-in Sources

| Source | Syntax | Description |
|--------|--------|-------------|
| `dir` | `dir://./consumers/` or just `./consumers/` | Local directory of contract files (default) |
| `repo` | `repo://org/contracts-repo/consumers/` | Reads contracts from a git repo (centralized topology) |
| `discover` | `discover://org?topic=cvt-consumer-payments-api` | Discovers consumer repos via GitHub API topics/tags (distributed topology) |
| `server` | `server://localhost:9550` | Queries a running CVT server (current behavior, still supported) |
| `multi` | Configured in `.cvt/config.yaml` | Queries multiple sources and merges results |

### Multi-Source Configuration

For orgs that mix topologies:

```yaml
contract_sources:
  - type: repo
    repo: org/contracts-repo
    path: consumers/
  - type: discover
    org: org
    topic_prefix: cvt-consumer-
```

`can-i-deploy` queries all configured sources, deduplicates by consumer ID, and validates against all of them.

### Consumer Contract File Format

Whether in a contract repo or a consumer's own `.cvt/` directory, the contract file is the same:

```yaml
# checkout-service's contract with payments-api
consumer_id: checkout-service
consumer_version: 3.1.0
schema_id: payments-api
schema_version: ">=1.0.0"     # version constraint, not pinned
environment: prod
used_endpoints:
  - method: GET
    path: /payments/{id}
    response_fields:
      - id
      - amount
      - currency
      - status
  - method: POST
    path: /payments
    request_fields:
      - amount
      - currency
```

### Auto-Generation from Tests

SDKs already capture interactions during test runs. The new piece is a **contract export** step:

```typescript
// Node.js example
const validator = new ContractValidator({ schema: 'github://org/payments-api/openapi.yaml' });
const adapter = createHttpAdapter(validator);
// ... run tests ...
await validator.exportContract('.cvt/payments-api.yaml');
```

This writes the contract file based on the endpoints and fields actually exercised in tests. Developers review the file and commit it.

---

## Section 4: Notification System

Two tiers: GitHub-native for immediate value, webhooks for extensibility. Both follow the gate-then-notify behavioral model.

### Behavioral Model

1. Producer CI runs `cvt can-i-deploy` — this **gates** the merge by default
2. If consumers would break, the build fails with a clear report
3. If the producer overrides the gate (`--breaking-change-acknowledged`), notifications fire to affected consumers
4. Notifications are **opt-in per producer** via configuration

### Tier 1: GitHub-Native Notifications

```yaml
# .cvt/config.yaml in the producer repo
notifications:
  github:
    enabled: true
    mode: issue          # issue or pr
    labels: ["contract-breaking-change"]
    # For centralized topology: open issue on contract repo
    # For distributed topology: open issue on each consumer repo
```

When `can-i-deploy` fails and the producer overrides:

```bash
cvt can-i-deploy --schema payments-api --version 2.0.0 \
  --contracts discover://org \
  --breaking-change-acknowledged \
  --notify
```

CVT opens a GitHub issue on each affected consumer repo:

```
Title: Breaking change: payments-api v2.0.0

Body:
  Schema: payments-api
  Version: 2.0.0 (was 1.5.0)
  Breaking changes:
    - REMOVED: GET /payments/{id} response field `currency`
    - CHANGED: POST /payments field `amount` type string -> number

  Your contract (.cvt/payments-api.yaml) uses affected endpoints.
  Please update your contract and tests.

  Detected by CVT in org/payments-api CI run #1234.
```

### Tier 2: Webhook Notifications

```yaml
notifications:
  webhooks:
    - url: https://ci.internal/repository-dispatch
      events: [breaking_change]
      headers:
        Authorization: "Bearer ${WEBHOOK_TOKEN}"
    - url: ${SLACK_WEBHOOK_URL}
      events: [breaking_change, new_version]
      template: slack    # built-in templates: slack, generic
```

CVT posts a JSON payload:

```json
{
  "event": "breaking_change",
  "schema_id": "payments-api",
  "old_version": "1.5.0",
  "new_version": "2.0.0",
  "breaking_changes": [
    {
      "type": "removed",
      "location": "GET /payments/{id} response",
      "field": "currency"
    },
    {
      "type": "changed",
      "location": "POST /payments request",
      "field": "amount",
      "from": "string",
      "to": "number"
    }
  ],
  "affected_consumers": [
    {
      "consumer_id": "checkout-service",
      "repo": "org/checkout-service"
    }
  ],
  "source_run": "https://github.com/org/payments-api/actions/runs/1234"
}
```

Templates format the payload for specific targets (Slack block kit, generic JSON, etc.). The `generic` template is the raw JSON above.

### What CVT Does NOT Become

- **Not a notification routing service** — it fires and forgets. Retry logic, delivery guarantees, and fan-out are the webhook consumer's problem.
- **Not a Slack bot or PagerDuty integration** — templates format the payload, but CVT doesn't maintain connections to these services.

---

## Section 5: File-Based Storage Backend

Today CVT has memory, SQLite, and PostgreSQL backends. For the git-native model, we add a **file-based backend** that stores state as human-readable YAML files — no database process, no binary files, fully git-diffable.

### What Gets Stored as Files

| Data | File | Purpose |
|------|------|---------|
| Schema metadata | `schemas/<id>/metadata.yaml` | Version, owner, team, endpoint count |
| Schema content | `schemas/<id>/spec.yaml` | The OpenAPI spec itself (or a provider reference) |
| Consumer contracts | `consumers/<consumer>/<schema>.yaml` | Endpoints and fields used |
| Validation results | `validations/<date>/<id>.yaml` | Optional audit trail, CI writes these |

### Provider References vs Inline Content

Schema files can either contain the spec inline or point to an external source:

```yaml
# Option A: inline (schema content lives in the contract repo)
schema_id: payments-api
version: 2.0.0
content_type: inline

# Option B: reference (schema lives elsewhere, CVT fetches it)
schema_id: payments-api
version: 2.0.0
content_type: reference
source: github://org/payments-api/openapi.yaml@v2.0.0
```

### How It Interacts with the CLI

The file backend is just another storage option. The CLI detects it automatically:

```bash
# If current directory (or --contracts-dir) has the file structure, use it
cvt can-i-deploy --schema payments-api --version 2.0.0

# Explicit
cvt can-i-deploy --schema payments-api --version 2.0.0 \
  --storage file --storage-path ./contracts/
```

### What This Does NOT Replace

- **PostgreSQL/SQLite** remain for teams running the CVT server with consumer registry, high-throughput validation, and query-heavy workloads
- **File-based storage** is for the git-native topologies (Models B and C) where git is the persistence layer and CI is the compute layer

### No Migration Between Backends

File-based and database backends serve different deployment models. There's no need to migrate between them — a team either uses the git-native model or the server model.

---

## Section 6: Configuration Model

All the new features (providers, contract sources, notifications) need a unified configuration surface. One file, hierarchical, with env var overrides.

### `.cvt/config.yaml`

Lives in the repo root (producer repo, consumer repo, or contract repo). This is the single configuration file for all CVT behavior.

```yaml
# .cvt/config.yaml

# Where schemas come from
schema_providers:
  github:
    default_org: my-org
    token_env: GITHUB_TOKEN
  registry:
    adapter: http
    http:
      base_url: https://schema-registry.internal/api/v1
      schema_path: /schemas/{name}/versions/{version}/content
      auth:
        type: bearer
        token_env: SCHEMA_REGISTRY_TOKEN

# Where consumer contracts come from (for can-i-deploy)
contract_sources:
  - type: dir
    path: ./consumers/
  - type: discover
    org: my-org
    topic_prefix: cvt-consumer-

# Notification configuration
notifications:
  github:
    enabled: true
    mode: issue
    labels: ["contract-breaking-change"]
  webhooks:
    - url: ${SLACK_WEBHOOK_URL}
      events: [breaking_change]
      template: slack

# Storage mode (file for git-native, or server connection)
storage:
  type: file                    # file | memory | sqlite | postgres | server
  path: ./                      # for file type: root of the contract structure
```

### Precedence

1. CLI flags (highest)
2. Environment variables (`CVT_SCHEMA_PROVIDER_REGISTRY_BASE_URL`, etc.)
3. `.cvt/config.yaml` in current repo
4. Defaults (lowest)

### Minimal Configs for Each Topology

**Model A (CLI-only):** No config file needed. Everything via CLI flags.

```bash
cvt validate --schema ./openapi.yaml --interaction interaction.json
```

**Model B (Contract repo):**

```yaml
# contracts-repo/.cvt/config.yaml
contract_sources:
  - type: dir
    path: ./consumers/
storage:
  type: file
```

**Model C (Distributed):**

```yaml
# producer-repo/.cvt/config.yaml
contract_sources:
  - type: discover
    org: my-org
    topic_prefix: cvt-consumer-
notifications:
  github:
    enabled: true
    mode: issue
```

### Topology Support Verification

| Capability | Model A (CLI-only) | Model B (Contract Repo) | Model C (Distributed) | Existing Server |
|---|---|---|---|---|
| Schema providers | CLI flags | Config + inline/reference | Config + CLI flags | gRPC registration |
| Contract sources | N/A | `dir://` | `discover://` | `server://` |
| Storage | None | `file` | `file` | `postgres`, `sqlite`, `memory` |
| Notifications | N/A | GitHub issues, webhooks | GitHub issues, webhooks | N/A |
| `can-i-deploy` | N/A | Reads local files | Discovers across repos | Queries server |
| Config required | No | Minimal | Minimal | Env vars (existing) |

### No Config Needed for Existing Users

Everything is backward compatible. Teams using `cvt serve` with PostgreSQL change nothing. The config file is only needed when using the new features.

---

## Section 7: CLI Changes

The new features surface through the existing CLI commands with additional flags. No new top-level commands except `export-contract` and `init` — just extensions to `validate`, `can-i-deploy`, `compare`, and `generate`.

### Extended Commands

**`cvt validate`** — now accepts provider URIs:

```bash
# Existing (unchanged)
cvt validate --schema ./openapi.yaml --interaction interaction.json

# New: pull schema from GitHub (convenience sugar)
cvt validate --schema github://org/payments-api/openapi.yaml@v1.0.0 \
  --interaction interaction.json

# New: pull schema from registry (requires configured adapter)
cvt validate --schema registry://payments-api@1.0.0 \
  --interaction interaction.json

# Equivalent to github:// using plain HTTPS URL (existing capability)
cvt validate --schema https://raw.githubusercontent.com/org/payments-api/v1.0.0/openapi.yaml \
  --interaction interaction.json
```

**`cvt compare`** — same provider URI support:

```bash
cvt compare --old registry://payments-api@1.0.0 \
  --new github://org/payments-api/openapi.yaml@main
```

**`cvt can-i-deploy`** — new contract source and notification flags:

```bash
# Existing (unchanged, queries server)
cvt can-i-deploy --schema payments-api --version 2.0.0 --server localhost:9550

# New: file-based, reads from contract repo
cvt can-i-deploy --schema payments-api --version 2.0.0 \
  --contracts dir://./consumers/

# New: distributed, discovers consumer repos
cvt can-i-deploy --schema payments-api --version 2.0.0 \
  --contracts discover://my-org

# New: override gate and notify
cvt can-i-deploy --schema payments-api --version 2.0.0 \
  --contracts discover://my-org \
  --breaking-change-acknowledged \
  --notify
```

**`cvt generate`** — provider URIs work here too:

```bash
cvt generate --schema github://org/payments-api/openapi.yaml \
  --method GET --path /payments/{id}
```

### New Command: `cvt export-contract`

Exports a consumer contract file from SDK test results:

```bash
# Run after tests — reads captured interactions and writes contract YAML
cvt export-contract \
  --consumer checkout-service \
  --consumer-version 3.1.0 \
  --schema payments-api \
  --interactions ./test-results/interactions.json \
  --output .cvt/payments-api.yaml
```

The SDK test run captures interactions to a file. `export-contract` distills that into the minimal contract format (endpoints + fields used). This is the auto-generation path — developers review and commit the output.

### New Command: `cvt init`

Scaffolds the `.cvt/` directory structure for a chosen topology:

```bash
# Initialize a contract repo (Model B)
cvt init --mode contract-repo
# Creates: schemas/, consumers/, .cvt/config.yaml, .github/workflows/cvt-validate.yml

# Initialize a producer repo (Model C)
cvt init --mode producer
# Creates: .cvt/config.yaml with discover contract source

# Initialize a consumer repo (Model C)
cvt init --mode consumer
# Creates: .cvt/ directory, sample contract file
```

### No Changes to `cvt serve`

The server command is unchanged. Teams using the server model continue as before. The server may eventually support the provider interface for schema registration, but that's a future enhancement, not part of this design.

---

## Section 8: SDK Changes

The SDKs need two additions: support for provider URIs when creating validators, and a contract export capability. The core validation flow is unchanged.

### Provider URI Support

Today SDKs take a server address or local file. They now also accept provider URIs:

```typescript
// Node.js — existing (unchanged)
const validator = new ContractValidator({ address: 'localhost:9550' });
await validator.registerSchema('payments-api', './openapi.yaml');

// Node.js — HTTPS URL (existing, unchanged)
await validator.registerSchema('payments-api',
  'https://raw.githubusercontent.com/org/repo/v1.0.0/openapi.yaml');

// Node.js — new: provider URI sugar, no server needed
const validator = new ContractValidator({
  schema: 'github://org/payments-api/openapi.yaml@v1.0.0'
});

// Node.js — new: registry provider (requires configured adapter)
const validator = new ContractValidator({
  schema: 'registry://payments-api@1.0.0'
});

// Python
validator = ContractValidator(schema="github://org/payments-api/openapi.yaml@v1.0.0")
validator = ContractValidator(schema="registry://payments-api@1.0.0")

// Go
validator, _ := cvt.NewValidator(cvt.WithSchema("github://org/payments-api/openapi.yaml@v1.0.0"))
validator, _ := cvt.NewValidator(cvt.WithSchema("registry://payments-api@1.0.0"))

// Java
ContractValidator validator = ContractValidator.builder()
    .schema("github://org/payments-api/openapi.yaml@v1.0.0")
    .build();
ContractValidator validator = ContractValidator.builder()
    .schema("registry://payments-api@1.0.0")
    .build();
```

When using a provider URI, the SDK fetches and parses the schema locally. No server involved. Validation, mock adapters, and fixture generation all work the same way — they just need a parsed schema.

### Provider URI Clarification

The `github://` scheme is **convenience sugar**, not a new capability. Anything it does can be accomplished with a raw HTTPS URL:

| Provider URI | Equivalent HTTPS URL |
|---|---|
| `github://org/repo/openapi.yaml@v1.0.0` | `https://raw.githubusercontent.com/org/repo/v1.0.0/openapi.yaml` |
| `github://org/repo/openapi.yaml@main` | `https://raw.githubusercontent.com/org/repo/main/openapi.yaml` |

**Why offer it anyway:**
- Shorter, more readable in config files and CI scripts
- Abstracts away GitHub Enterprise vs github.com host differences
- Auto-resolves auth from configured `GITHUB_TOKEN` without manual headers
- Consistent syntax with `registry://` — one pattern for all providers

**Where custom URIs are essential:** `registry://` is the only scheme that enables something you *can't* do with a plain URL. A homegrown schema registry has a proprietary API, authentication, and response format. `registry://payments-api@1.0.0` delegates to a configured adapter that knows how to fetch and extract the schema from that registry's specific API shape.

**The rule:** if a schema is reachable via a public or authenticated HTTPS URL, you can always use that URL directly. Provider URIs are an optional layer of convenience and consistency on top.

### Contract Export

SDKs already capture interactions during test runs. The new method writes them as a contract file:

```typescript
// Node.js
const adapter = createHttpAdapter(validator);
// ... run tests ...
await validator.exportContract({
  consumerId: 'checkout-service',
  consumerVersion: '3.1.0',
  outputPath: '.cvt/payments-api.yaml'
});
```

```python
# Python
adapter = create_http_adapter(validator)
# ... run tests ...
validator.export_contract(
    consumer_id="checkout-service",
    consumer_version="3.1.0",
    output_path=".cvt/payments-api.yaml"
)
```

```go
// Go
adapter := cvt.NewHTTPAdapter(validator)
// ... run tests ...
validator.ExportContract(cvt.ExportOptions{
    ConsumerID: "checkout-service",
    ConsumerVersion: "3.1.0",
    OutputPath: ".cvt/payments-api.yaml",
})
```

```java
// Java
HttpAdapter adapter = HttpAdapter.create(validator);
// ... run tests ...
validator.exportContract(ExportOptions.builder()
    .consumerId("checkout-service")
    .consumerVersion("3.1.0")
    .outputPath(".cvt/payments-api.yaml")
    .build());
```

### What Stays the Same

- **Mock adapters** — unchanged, they already work with a parsed schema
- **HTTP adapters** — unchanged
- **Producer middleware/test kit** — unchanged
- **`registerConsumer()` via gRPC** — still works for server-based deployments

The SDK changes are additive. No breaking changes to existing APIs.

---

## Summary: Architecture Diagram

```
                          ┌─────────────────────────┐
                          │   Schema Sources         │
                          │  (GitHub, Registry, URL) │
                          └────────────┬────────────┘
                                       │
                          ┌────────────▼────────────┐
                          │   Schema Provider        │
                          │   Plugin Interface       │
                          │  file | url | github |   │
                          │  registry               │
                          └────────────┬────────────┘
                                       │
               ┌───────────────────────┼───────────────────────┐
               │                       │                       │
    ┌──────────▼──────────┐ ┌─────────▼─────────┐ ┌──────────▼──────────┐
    │  Model A: CLI-Only  │ │  Model B: Contract │ │  Model C: Distributed│
    │                     │ │  Repo              │ │                      │
    │  cvt validate       │ │  Git-as-database   │ │  .cvt/ in consumer   │
    │  cvt compare        │ │  File-based storage│ │  repos               │
    │  cvt generate       │ │  Centralized CI    │ │  Discovery via       │
    │                     │ │                    │ │  GitHub API          │
    │  No state needed    │ │  can-i-deploy via  │ │  can-i-deploy via    │
    │                     │ │  local file reads  │ │  cross-repo fetch    │
    └─────────────────────┘ └────────┬──────────┘ └──────────┬───────────┘
                                     │                       │
                          ┌──────────▼───────────────────────▼──┐
                          │   Contract Source Interface          │
                          │  dir | repo | discover | server |   │
                          │  multi                              │
                          └──────────────────┬──────────────────┘
                                             │
                          ┌──────────────────▼──────────────────┐
                          │   Notification System               │
                          │  Tier 1: GitHub issues/PRs          │
                          │  Tier 2: Webhooks (Slack, CI, etc.) │
                          │  Behavior: gate first, notify on    │
                          │  acknowledged override              │
                          └─────────────────────────────────────┘
```

### Existing Server Model (Unchanged)

```
    ┌───────────────────┐
    │  cvt serve        │
    │  gRPC server      │
    │  PostgreSQL/SQLite │
    │  Consumer registry │
    │  Prometheus metrics│
    └───────────────────┘
```

The server model continues to work exactly as it does today. It is an additional deployment option alongside Models A, B, and C.
