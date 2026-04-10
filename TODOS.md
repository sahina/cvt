# CVT TODOs

## P2: Split validator_service.go into focused files

**What:** Extract validator_service.go (1,956 LOC, 33 methods) into focused files along Phase boundaries.

**Why:** Single-author project risk. A god-file makes onboarding contributors harder and increases merge conflict surface.

**Proposed split:**
- `validator_service.go` — core validation (RegisterSchema, ValidateInteraction, helpers)
- `fixture_generator.go` — GenerateFixture + all generate* helpers (~300 LOC)
- `consumer_registry.go` — RegisterConsumer, ListConsumers, DeregisterConsumer (~200 LOC)
- `deployment_safety.go` — CanIDeploy (~150 LOC)
- `producer_validation.go` — ValidateProducerResponse (~250 LOC)

**Effort:** M (human: ~3 days) → with CC: S (~15 min)
**Priority:** P2
**Depends on:** Ideally after storage wiring lands to avoid double-touching the same code.
**Source:** CEO Review 2026-03-19

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

## P2: DRY schema URL fetching (3 duplicate implementations)

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

## P2: Notification system design document

**What:** Before implementing notifications in Phase 2, produce a design doc covering: notification triggers (breaking change detected, schema registered, consumer deregistered), delivery channels (GitHub issues, webhooks, email), auth model (GITHUB_TOKEN vs CVT_NOTIFY_TOKEN), rate limiting, and failure handling.

**Why:** Notifications touch external systems with real blast radius. GitHub issue creation could spam repos. Webhooks could leak contract data to third parties. Needs deliberate design, not just implementation.

**Effort:** M (human: ~2 days) → with CC: S (~30 min)
**Priority:** P2
**Depends on:** Phase 1a + 1b complete
**Source:** CEO Review 2026-04-09

## P2: GitHubProvider dependency investigation

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

**Why:** Currently creates a new router on every Validate call (O(P) per call where P = number of paths). Server caches at registration time. Accepted in eng review but not blocking Phase 1a since offline can-i-deploy doesn't use routers.

**Effort:** S (human: ~2 hours) → with CC: S (~10 min)
**Priority:** P3
**Depends on:** None (can be done anytime)
**Source:** Eng Review 2026-04-09

## P3: Ristretto cache cost model fix

**What:** Change Ristretto cache cost from cost=1 per schema to `len(schemaBytes)`. Update `CacheMaxCost` from 1000 to a byte budget (e.g., 100MB = 104857600).

**Why:** Cost=1 means 1000 schemas at 10MB each = 10GB with no backpressure. Using actual byte size makes CacheMaxCost a real memory budget.

**Effort:** S (human: ~30 min) → with CC: S (~5 min)
**Priority:** P3
**Depends on:** None (can be done anytime, but ideally during CompatibilityEngine consolidation)
**Source:** Eng Review 2026-04-09
