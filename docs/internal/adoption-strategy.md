---
title: Adoption Strategy
sidebar_label: Adoption Strategy
sidebar_position: 1
description: Challenges and strategies for driving internal CVT adoption
---

# CVT Adoption Strategy

This document outlines the challenges and strategies for driving internal adoption of CVT as the standard contract testing tool.

## Context

- CVT is a greenfield internal tool (not replacing Pact)
- Goal: Become the standard for consumer and producer contract testing
- Current state: Consumer/schema validation implemented, producer testing planned

---

## Adoption Challenges

### 1. Contract Testing is a New Practice

Most teams don't do contract testing today. Introducing CVT means introducing a new practice, not just a new tool.

**Symptoms:**

- Teams don't understand the value proposition
- "We have integration tests" pushback
- Contract testing seen as extra work with unclear ROI

**Why it matters:** Tool adoption fails when the underlying practice isn't understood or valued.

### 2. Unclear Ownership Model

Contract testing requires answering: who owns the contract?

| Model           | Description                     | Tradeoff                             |
| --------------- | ------------------------------- | ------------------------------------ |
| Producer-owned  | API team publishes OpenAPI spec | Consumers must adapt to changes      |
| Consumer-driven | Consumers define expectations   | Producers must satisfy all consumers |
| Collaborative   | Both sides contribute           | Requires coordination overhead       |

**Without a clear model:** Teams argue about responsibility, specs drift from reality, nobody maintains contracts.

### 3. Integration Friction

Every new tool competes for attention in CI/CD pipelines.

**Common friction points:**

- Another service to run/maintain
- New SDK to learn and integrate
- Test failures block deployments (seen as obstacle, not value)
- Schema registration step feels like bureaucracy

### 4. No Immediate Pain

Contract testing prevents future problems. Teams prioritize current fires.

**The timing problem:**

- Value is realized when a breaking change is caught
- If nothing breaks, the tool feels unnecessary
- If something breaks in production, it's too late to prove CVT would have helped

### 5. Decentralized Decision Making

In microservices organizations, each team chooses their own tooling.

**Result:**

- Adoption requires convincing N teams, not one decision
- Different teams have different priorities
- Inconsistent adoption creates gaps in coverage

---

## Practical Strategies

### Strategy 1: Embed in Service Scaffolding

**Approach:** Pre-configure CVT in your service template/generator so new services have contract testing by default.

**Implementation:**

- Add CVT SDK as a default dependency
- Include sample contract test in template
- Pre-configure CI pipeline step (disabled by default, easy to enable)
- Add schema directory structure to template

**Why it works:**

- Default behavior beats optional adoption
- New services start with the practice built-in
- Removes the "getting started" friction entirely

**Metrics to track:**

- % of new services with CVT enabled
- Time from service creation to first contract test

### Strategy 2: Target High-Pain Integrations First

**Approach:** Find the integrations that break most often and prove value there.

**How to identify targets:**

- Review incident history for integration failures
- Ask teams: "Which upstream service breaks you most often?"
- Look for services with frequent deployments and many consumers

**Implementation:**

1. Identify 2-3 high-pain integration points
2. Work directly with those teams to implement CVT
3. Document the before/after experience
4. Publicize wins when CVT catches breaking changes

**Why it works:**

- Solves a real, felt problem (not theoretical value)
- Creates internal advocates who experienced the benefit
- Generates case studies for broader rollout

### Strategy 3: Make CI Integration Trivial

**Approach:** Reduce integration to a one-liner that teams can copy-paste.

**Current friction:**

```yaml
# Too many steps = adoption friction
- name: Start CVT server
  run: docker run -d cvt-server
- name: Install SDK
  run: npm install @cvt/cvt-sdk
- name: Register schema
  run: npx cvt register ./openapi.json
- name: Run tests
  run: npm test
```

**Target state:**

```yaml
# One step = easy adoption
- name: Contract tests
  uses: internal/cvt-action@v1
  with:
    schema: ./openapi.json
```

**Implementation:**

- Create GitHub Action / GitLab CI template / Jenkins shared library
- Handle server lifecycle, SDK installation, schema registration internally
- Provide clear error messages with fix instructions
- Support common patterns out of the box (monorepo, multi-schema, etc.)

### Strategy 4: Establish Clear Ownership Model

**Approach:** Make an organizational decision about contract ownership and document it.

**Recommended model for OpenAPI-first:**

```text
Producer owns the OpenAPI spec (source of truth)
     ↓
Spec published to central registry (CVT server)
     ↓
Consumers validate their calls against registered spec
     ↓
Breaking changes detected before deployment
```

**Document and communicate:**

- Who creates/maintains OpenAPI specs (producer team)
- Where specs are stored (repo location, registry)
- When specs are registered (CI pipeline, on merge to main)
- How breaking changes are handled (versioning policy)

**Enforcement:**

- PR checks that validate spec changes
- Required schema registration before deployment
- Breaking change detection in CI

### Strategy 5: Create Visible Wins

**Approach:** When CVT catches a breaking change, make it visible to the organization.

**Tactics:**

- Slack/Teams notifications: "CVT prevented breaking change in payment-service"
- Monthly report: "X breaking changes caught, Y hours of incident response avoided"
- Incident postmortems: "This would have been caught by CVT" (for incidents in non-CVT services)

**Track and report:**

- Breaking changes caught per month
- Services protected by CVT
- Estimated incidents prevented (based on historical rate)

**Why it works:**

- Demonstrates concrete value
- Creates FOMO for teams not using CVT
- Builds organizational support for broader mandate

### Strategy 6: Provide Migration Path for Existing Services

**Approach:** Make it easy for existing services to adopt CVT incrementally.

**Phased adoption for existing services:**

| Phase                  | Scope                                          | Effort |
| ---------------------- | ---------------------------------------------- | ------ |
| 1. Schema only         | Register existing OpenAPI spec                 | Low    |
| 2. Consumer tests      | Add tests for one critical upstream dependency | Medium |
| 3. Full coverage       | Test all external integrations                 | High   |
| 4. Producer validation | Validate incoming requests (when available)    | Medium |

**Provide tooling:**

- Script to generate initial schema from existing code
- Example PR showing CVT integration
- Office hours / pairing sessions for teams adopting

### Strategy 7: Executive Sponsorship for Mandate

**Approach:** If adoption is critical, get organizational mandate.

**When to use:** After proving value with early adopters, expand via policy.

**Mandate options (escalating):**

1. **Encouraged:** "Teams should consider CVT for new integrations"
2. **Expected:** "New services must have CVT enabled within 30 days"
3. **Required:** "Services cannot deploy without passing contract tests"

**Prerequisites for mandate:**

- Proven value with early adopters
- Mature tooling (low friction to adopt)
- Clear documentation and support
- Escape hatch for exceptions

**Caution:** Mandates without maturity breed resentment. Earn adoption before requiring it.

---

## Rollout Phases

### Phase 1: Foundation (Current)

_Corresponds to: Roadmap Phase 1 (Consumer Validation) - Complete_

**Goals:**

- Complete consumer validation implementation
- Document ownership model and getting started guide
- Identify early adopter teams

**Success criteria:**

- 2-3 teams using CVT in production
- CI integration takes < 30 minutes
- Clear documentation for common scenarios

**Known gap:** Developers must manually construct test responses without API access. Addressed in Phase 2.

### Phase 2: Early Adoption

_Corresponds to: Roadmap Phase 2 (Developer Experience)_

**Goals:**

- Deploy test fixture generator (eliminates manual JSON construction)
- Create CI/CD integration templates
- Target high-pain integrations
- Build internal case studies

**Success criteria:**

- Test fixture generator available and adopted
- 10+ services using CVT
- At least 3 documented "saves" (breaking changes caught)
- "Testing Without API Access" workflow fully supported

### Phase 3: Broad Adoption

_Corresponds to: Roadmap Phase 3 (Producer Validation)_

**Goals:**

- Organizational awareness
- Optional but encouraged for all services
- Producer validation available for API owners
- Embed in service scaffolding

**Success criteria:**

- 50%+ of services with external integrations using CVT
- Producer middleware adopted by API teams
- Monthly metrics published
- Self-service adoption (teams adopt without hand-holding)

### Phase 4: Standard Practice

_Corresponds to: Roadmap Phase 4 (Platform Features)_

**Goals:**

- Contract testing is the norm
- Mandate for critical services
- Full consumer + producer coverage
- Platform tooling (persistence, dashboard) available

**Success criteria:**

- 90%+ coverage for critical integration points
- Contract test failures block deployments
- Incident rate for contract violations near zero

---

## Metrics to Track

| Metric                        | Purpose              |
| ----------------------------- | -------------------- |
| Services with CVT enabled     | Adoption breadth     |
| Contract tests per service    | Adoption depth       |
| Breaking changes caught       | Value delivered      |
| Time to integrate CVT         | Friction measurement |
| Schema registration frequency | Active usage         |
| CI pipeline pass rate         | Quality indicator    |

---

## Common Objections and Responses

### "We already have integration tests"

**Response:** Integration tests verify behavior, contract tests verify compatibility. Integration tests require running both services. Contract tests run independently and catch breaking changes before integration.

**When contract tests add value:**

- Upstream service deploys before you test against it
- Integration test environment is flaky/unavailable
- You want fast feedback without spinning up dependencies

### "It's extra work"

**Response:** Initial setup takes ~30 minutes. Ongoing cost is maintaining OpenAPI spec (which you should have anyway) and running tests in CI (automated).

**Reframe:** It's less work than debugging production integration failures.

### "Our APIs don't change often"

**Response:** Contract tests are insurance. The cost is low, the protection is valuable. When APIs do change, contract tests catch incompatibilities before production.

### "Who has time for this?"

**Response:** Start with one critical integration. Add more coverage over time. Partial coverage is better than none.

---

## Next Steps

1. **Finalize CI integration templates** - GitHub Actions, GitLab CI, etc.
2. **Identify 2-3 pilot teams** - High-pain integrations, willing early adopters
3. **Create getting started guide** - 15-minute path to first contract test
4. **Establish schema ownership policy** - Organizational decision, documented
5. **Set up metrics tracking** - Dashboard for adoption and value metrics
