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
