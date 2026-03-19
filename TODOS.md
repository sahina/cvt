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
