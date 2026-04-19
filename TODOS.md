# CVT TODOs

## Closed: issue #107 (plugin system v1 follow-ups) — see git log 2026-04-18..2026-04-19

Reference plugins extracted to `github.com/sahina/cvt-plugin-rest` and
`github.com/sahina/cvt-plugin-slack` (v0.1.0 each). `validator_service.go`
split, three-of-four hooks wired, `register-schema --check-compatibility`
now does real server-side work. Follow `docs/plugins/reference-plugins.md`
for install.

## P2: Multi-plugin write pipelines (v1.1)

**What:** Extend plugin pipeline executor to support multiple plugins per stage on write RPCs (currently restricted to single-plugin in v1). Requires two-phase commit or compensating-transaction semantics so partial-write failures are not silently swallowed.

**Why:** v1 of the plugin system restricts `register_consumer_usage` (and any future write RPC) to at most one plugin per stage. Teams that want to fan out consumer-usage writes to two registries (e.g., primary Central API Registry + mirror for DR) cannot do so in v1. This unblocks that use case without sacrificing rigor.

**Options to explore:**
- Two-phase commit with prepare/commit/abort phases per plugin.
- Compensating transactions: if plugin B fails after plugin A succeeds, core invokes a delete RPC on plugin A.
- Accept-at-least-once with explicit audit reconciliation workflow (the weakest semantics, documented clearly).

**Effort:** L (human: ~1 week) → with CC: M (~1 hour)
**Priority:** P2
**Depends on:** Plugin system v1 shipped.
**Source:** Outside-voice review 2026-04-17 (plugin-system.md)

## P2: Windows support for plugin framework

**What:** Add Windows support to the subprocess plugin framework. Requires loopback socket + cookie fallback path (HashiCorp go-plugin supports this natively), plus provisioning a Windows CI runner.

**Why:** CVT is built with goreleaser for Windows (amd64), but v1 of the plugin framework ships Linux + macOS only because there's no Windows CI runner today. Windows users must use WSL2 in v1.

**Effort:** M (human: ~3 days; CI runner provisioning is the bulk) → with CC: S (~30 min for code; runner provisioning stays human)
**Priority:** P2
**Depends on:** Plugin system v1 shipped + GitHub Actions Windows runner provisioned.
**Source:** Outside-voice review 2026-04-17 (plugin-system.md)

## P2: Plugin template repo (cvt-plugin-template)

**What:** Publish `github.com/sahina/cvt-plugin-template` with goreleaser config, GitHub Actions CI, `cvt plugins verify` CI step, README template, example RegistryProvider and EventHandler implementations.

**Why:** Three reference plugins (`cvt-plugin-rest`, `cvt-plugin-registry-github`, `cvt-plugin-slack`) and any future third-party plugins all need the same build + release + verify scaffolding. Without a template, each plugin author reinvents.

**Effort:** S (human: ~1 day) → with CC: S (~15 min)
**Priority:** P2
**Depends on:** Plugin proto v1 frozen (end of Plugin System Lane 1).
**Source:** Eng Review 2026-04-17 (plugin-system.md)

## P3: Pipeline `merge` strategy decision (v1.1)

**What:** Plugin system v1 mentions `strategy: merge` in pipeline config but does not implement it (no v1 extension point produces mergeable output). Either implement alongside Validator extension point in v1.1 or remove the mention entirely.

**Why:** Leaving unimplemented strategy names in the config schema is a footgun. Users may set `strategy: merge` expecting it to work and get a silent fallback or unclear error.

**Effort:** S (remove) or M (implement alongside Validator) → with CC: S (~15 min) or M (~1 hour)
**Priority:** P3
**Depends on:** Validator extension point (category A) scope decision for v1.1.
**Source:** Eng Review 2026-04-17 (plugin-system.md)

## P3: Plugin config YAML JSON-schema

**What:** Publish `cvt-config.schema.json` so editors (VSCode, JetBrains, neovim-lsp) can autocomplete and validate `.cvt/config.yaml` + `~/.cvt/config.yaml`.

**Why:** Plugin config is the primary UX surface for the plugin system. Misspelled keys (`on_error` vs `onError`, `strategy: first-success` vs `first_success`) silently ignored or mis-merged is painful. Schema gives squiggles and fix suggestions in editors.

**Effort:** S (human: ~4 hours) → with CC: S (~15 min)
**Priority:** P3
**Depends on:** Plugin config schema frozen (end of Plugin System Lane 1).
**Source:** Eng Review 2026-04-17 (plugin-system.md)

## P2: Wire consumer registry operations to storage

**What:** Wire RegisterConsumer, ListConsumers, DeregisterConsumer to use persistent storage with read-through cache.

**Why:** Consumer registrations currently only live in cache. The Store interface supports full consumer persistence but it's not wired. CanIDeploy results are meaningless across server restarts.

**Effort:** M (human: ~2 days) → with CC: S (~15 min)
**Priority:** P2
**Depends on:** Storage wiring (this PR)
**Source:** Eng Review 2026-03-19

## P3: Add storage env var validation in serve.go

**What:** Add bounds checking for storage-related environment variables (CVT_POSTGRES_MAX_CONNS, CVT_POSTGRES_PORT, etc.).

**Why:** A typo like CVT_POSTGRES_MAX_CONNS=0 or CVT_POSTGRES_PORT=-1 causes cryptic runtime failures. Fail-fast with clear error messages.

**Effort:** S (human: ~1 hour) → with CC: S (~5 min)
**Priority:** P3
**Depends on:** None
**Source:** Eng Review 2026-03-19

## P2: Contributor-facing agent skills

**What:** Create Claude Code skills for CVT contributors: /cvt-add-feature (add server features), /cvt-add-rpc (add gRPC methods), /cvt-add-sdk-feature (add SDK features across languages), /cvt-generate (protobuf codegen), /cvt-review (code review against CVT conventions), /cvt-audit (security/quality audit).

**Why:** Consumer skills encode the SDK user journey; contributor skills encode institutional knowledge for working ON CVT itself. Together they complete the agent skill suite. Reduces onboarding time for contributors and ensures consistent patterns.

**Pros:** Full skill coverage for both audiences. Contributors follow consistent patterns encoded in skills.
**Cons:** ~6 more skills to create and maintain. Requires deep understanding of CVT internals.
**Context:** Consumer skills landed first (chore/skills branch) as the pattern to follow. Contributor skills should mirror the same SKILL.md format with frontmatter, steps, SDK-specific instructions, common errors, and success criteria.

**Effort:** L (human: ~2 weeks) → with CC: M (~1 hour)
**Priority:** P2
**Depends on:** Consumer skills (chore/skills PR) landing first.
**Source:** CEO Review 2026-03-24

## P2: DRY schema URL fetching (3 duplicate implementations) — SUPERSEDED

**Status:** Superseded by plugin system (see `docs/design/plugin-system.md`). Registry-based schema fetching moves into `cvt-plugin-rest` and `cvt-plugin-registry-github`. Core keeps one HTTP fetch path in `pkg/cvt/validator.go:RegisterSchemaFromURL()` for direct file/URL schema args; `cmd/cvt/register_schema.go:fetchSchemaFromURL()` remains the CLI entry point. The 3-way duplication with a hypothetical `HTTPProvider.Fetch()` no longer lands because `HTTPProvider` ships as a plugin.

_Original description preserved below for context:_

**What:** Consolidate schema URL fetching from 3 locations into the SchemaProvider interface: `pkg/cvt/validator.go:RegisterSchemaFromURL()`, `cmd/cvt/register_schema.go:fetchSchemaFromURL()`, and the planned `HTTPProvider.Fetch()`.

**Why:** Three independent HTTP fetch implementations with inconsistent timeout handling (one has no timeout, one hardcodes 30s, one is TBD). DRY into a single provider-based path.

**Effort:** S (human: ~4 hours) → with CC: S (~15 min)
**Priority:** P2
**Depends on:** SchemaProvider interface (Phase 1a of Enterprise Deployment)
**Source:** Eng Review 2026-04-09

## P2: Document offline/server can-i-deploy coupling

**What:** Add inline documentation in both `pkg/contracts/can_i_deploy.go` and `server/cvtservice/validator_service.go` documenting the coupling: offline can-i-deploy must mirror the server CanIDeploy flow (schema fetch, compatibility check, consumer filtering).

**Why:** Two implementations of the same logic. Without documented coupling, they will diverge silently over time. Shared test fixtures help but documentation is the first line of defense.

**Effort:** S (human: ~1 hour) → with CC: S (~5 min)
**Priority:** P2
**Depends on:** Offline can-i-deploy implementation (Phase 1a)
**Source:** Eng Review 2026-04-09

## P2: Notification system design document — SUPERSEDED

**Status:** Superseded by plugin system. The `EventHandler` plugin contract (`api/protos/plugin/events/v1/events.proto`) is the notification surface; the reference plugin `cvt-plugin-slack` is the first implementation. Per-plugin rate limiting and dedup live in the plugin itself (each plugin decides its own policy), which is the right placement per the original TODO's blast-radius concern. Additional channels (Jira, GitHub issues, email) become additional plugins rather than core code.

_Original description preserved below for context:_

**What:** Before implementing notifications in Phase 2, produce a design doc covering: notification triggers (breaking change detected, schema registered, consumer deregistered), delivery channels (GitHub issues, webhooks, email), auth model (GITHUB_TOKEN vs CVT_NOTIFY_TOKEN), rate limiting, and failure handling.

**Why:** Notifications touch external systems with real blast radius. GitHub issue creation could spam repos. Webhooks could leak contract data to third parties. Needs deliberate design, not just implementation.

**Effort:** M (human: ~2 days) → with CC: S (~30 min)
**Priority:** P2
**Depends on:** Phase 1a + 1b complete
**Source:** CEO Review 2026-04-09

## P2: GitHubProvider dependency investigation — SUPERSEDED

**Status:** Superseded by plugin system. GitHub-backed schema registry ships as a separate plugin (`cvt-plugin-registry-github`, not yet implemented as a v1 deliverable but tracked as a separate repo). The `go-github` vs `gh` CLI vs raw HTTP decision is made inside the plugin's own module; CVT core gains zero dependency weight regardless of which direction the plugin takes.

_Original description preserved below for context:_

**What:** Investigate whether GitHubProvider should depend on `go-github`, `gh` CLI, or raw HTTP + GraphQL. Consider: auth token handling (GITHUB_TOKEN, GH_TOKEN, gh auth), API rate limits, large repo schema fetching, and release asset downloads.

**Why:** go-github is a heavy dependency for a CLI tool. gh CLI requires GitHub-specific setup. Raw HTTP is more work but fewer deps. This decision affects the entire provider ecosystem design.

**Effort:** S (human: ~4 hours) → with CC: S (~15 min)
**Priority:** P2
**Depends on:** Phase 1a complete (provider interface finalized)
**Source:** CEO Review 2026-04-09

## P3: Field-level consumer filtering

**What:** Extend `filterChangesForConsumer` to match breaking changes against specific response/request fields, not just path+method. Requires structured `FieldName` in breaking changes (currently flat description strings) and a 5th EndpointUsage representation.

**Why:** Current filtering is endpoint-level only (path+method matching). Consumers that only use a subset of response fields get false positives when unused fields change. Deferred from Phase 1a to avoid scope explosion.

**Effort:** M (human: ~3 days) → with CC: M (~1 hour)
**Priority:** P3
**Depends on:** CompatibilityEngine consolidation complete
**Source:** Eng Review 2026-04-09

## P3: Router caching in pkg/cvt/

**What:** Cache gorillamux routers at schema registration time in `pkg/cvt/validator.go`, matching the server pattern in `validator_service.go:199-211`.

**Why:** Currently creates a new router on every Validate call (O(P) per call where P = number of paths). Server caches at registration time. Accepted in eng review but not blocking Phase 1a since offline can-i-deploy doesn't use routers. Also impacts `cvt mock` performance: `findOperation()` in `generator.go` creates a new router per GenerateResponse call, meaning every mock request builds a router.

**Effort:** S (human: ~2 hours) → with CC: S (~10 min)
**Priority:** P3
**Depends on:** None (can be done anytime)
**Source:** Eng Review 2026-04-09, CEO Review 2026-04-14 (mock server outside voice)

## P3: Ristretto cache cost model fix

**What:** Change Ristretto cache cost from cost=1 per schema to `len(schemaBytes)`. Update `CacheMaxCost` from 1000 to a byte budget (e.g., 100MB = 104857600).

**Why:** Cost=1 means 1000 schemas at 10MB each = 10GB with no backpressure. Using actual byte size makes CacheMaxCost a real memory budget.

**Effort:** S (human: ~30 min) → with CC: S (~5 min)
**Priority:** P3
**Depends on:** None (can be done anytime, but ideally during CompatibilityEngine consolidation)
**Source:** Eng Review 2026-04-09
