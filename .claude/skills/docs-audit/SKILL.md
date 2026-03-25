---
name: docs-audit
description: Audit and update project documentation against current implementation. Use whenever the user wants to review, update, verify, or fix documentation accuracy — SDK docs, READMEs, CLI reference, tutorials, how-to guides, explanation docs, internal specs, roadmap, CLAUDE.md, or any project documentation. Triggers on "update docs", "audit documentation", "docs outdated", "sync docs", "review docs", "fix documentation", "update roadmap", "docs drift", "check docs accuracy", or any request about keeping documentation current and correct.
user-invocable: true
argument-hint: "[scope] — run /docs-audit to see options"
allowed-tools:
  - Read
  - Edit
  - Write
  - Grep
  - Bash
  - Agent
---

# Documentation Audit & Fix

Systematically audit project documentation against the current implementation, fix discrepancies directly, and report what changed.

## Step 0: Self-Update — Keep This Skill Current

The codebase evolves — new CLI commands appear in `cmd/cvt/`, new SDK files show up, new doc files get created. Before auditing docs, check whether this skill's own `references/sources-of-truth.md` is still accurate.

### Discovery scan

Run these checks and update `references/sources-of-truth.md` if anything is missing:

1. **New CLI commands**: Glob `cmd/cvt/*.go` and compare against the `cli` scope's truth sources. If new command files exist, add them.
2. **New SDK files**: Glob `sdks/node/src/**/*.ts`, `sdks/python/cvt_sdk/**/*.py`, `sdks/go/cvt/**/*.go`, `sdks/java/src/main/java/**/*.java` and compare against the `sdk` scope. If new source files exist, add them.
3. **New doc files**: Glob `docs/**/*.md`, `docs/**/*.mdx` and compare against all scope documentation lists. If new doc files exist, add them to the appropriate scope.
4. **New proto files**: Glob `api/protos/*.proto` and compare against the `reference` scope.
5. **New server files**: Glob `server/cvtservice/*.go` and `server/storage/**/*.go` and compare against truth sources.
6. **New example files**: Glob `examples/**/*` and `sdks/*/examples/**/*` and compare against the `examples` scope.

If any updates are needed, apply them to `references/sources-of-truth.md` using the Edit tool before proceeding to Step 1. Briefly note what was added in the final report under a "Skill self-update" section.

If nothing is new, skip straight to Step 1.

## Step 1: Scope Selection

Present these scope options and **wait for the user to pick one explicitly**. Do not proceed without a selection.

| Scope            | Covers                                                     | Key truth sources                                      |
| ---------------- | ---------------------------------------------------------- | ------------------------------------------------------ |
| `sdk`            | SDK READMEs + SDK reference docs (all 4 SDKs)              | SDK source exports, types, function signatures         |
| `cli`            | CLI reference + CLI usage across docs                      | Cobra command definitions in `cmd/cvt/`                |
| `reference`      | All `docs/reference/*` (API, config, architecture, SDKs)   | Proto files, server handlers, config parsing           |
| `guides`         | All `docs/guides/*`                                        | Current SDK APIs, CLI commands, server behavior        |
| `getting-started`| `docs/getting-started/*`                                   | SDK APIs, CLI commands, server defaults                |
| `ai-helper`      | All `docs/ai-helper/*`                                     | Current SDK APIs, CLI commands, validation patterns    |
| `operations`     | `docs/operations/*`                                        | Observability config, Makefile targets, Docker setup   |
| `development`    | `docs/development/*`                                       | Build system, test commands, release process           |
| `examples`       | All example READMEs + example source code                  | Current SDK APIs, CLI commands                         |
| `internal`       | `docs/internal/*` + `docs/design/*`                        | Implementation state, shipped features                 |
| `project`        | `README.md`, `CLAUDE.md`, `CONTRIBUTING.md`, `TODOS.md`    | Everything — features, patterns, commands              |
| `ci-templates`   | `ci-templates/*`                                           | Current CLI commands, SDK APIs, workflow patterns      |
| `full`           | All of the above                                           | Everything                                             |

If the user already specified a scope as an argument or in their message, confirm the scope and proceed to Step 2.

For `full` scope, process each sub-scope sequentially in the order listed above, producing a combined report at the end.

## Step 2: Gather Source of Truth

Before examining any documentation, gather the current implementation state relevant to the selected scope. Read `references/sources-of-truth.md` in this skill's directory for detailed mappings of which source files to examine per scope.

The general principle: **code is truth, docs are claims**. For every claim in a doc file, there should be a verifiable source in the implementation.

### What to gather per scope

**sdk**: Read SDK source files to extract current exports, type definitions, function signatures, constructor parameters, and public API surface. Check `package.json`, `pyproject.toml`, `pom.xml`, `go.mod` for package names and versions. CVT has four SDKs: Node.js (`@sahina/cvt-sdk`), Python (`cvt-sdk`), Go (`github.com/sahina/cvt/sdks/go`), and Java (`io.github.sahina:cvt-sdk`).

**cli**: Read Cobra command files in `cmd/cvt/` to extract command names, subcommands, flags, flag types, defaults, and descriptions. Cross-reference with the CLI reference doc and CLAUDE.md.

**reference**: Read proto file (`api/protos/cvt.proto`), server handlers (`server/cvtservice/`), and config parsing (`server/storage/config.go`, `server/cvtservice/auth.go`, `server/cvtservice/tls.go`) to verify API endpoints, request/response shapes, and configuration options.

**guides**: For each guide (consumer testing, producer testing, breaking changes, validation modes, CI/CD), verify the feature exists, the API shown is current, and described behavior matches implementation.

**getting-started**: Verify that installation steps, quick-start flows, and FAQ content work with current APIs. Check default ports, URLs, command syntax, and SDK installation instructions.

**ai-helper**: Verify AI helper content reflects current SDK APIs, CLI commands, and validation patterns.

**operations**: Verify observability setup, Docker compose configuration, Prometheus/Grafana integration, and monitoring instructions match current implementation.

**development**: Verify contributing guide and release process match current Makefile targets and tooling.

**examples**: Compare example README instructions and code snippets against actual example source code and current SDK APIs.

**internal**: Check PRD and adoption strategy against what was actually implemented.

**project**: Verify top-level claims about features, architecture, build commands, and patterns in README.md, CLAUDE.md, and CONTRIBUTING.md.

**ci-templates**: Verify CI template YAML and Jenkinsfile use current CLI commands, SDK APIs, and workflow patterns.

### Parallelization strategy

Use subagents to gather truth in parallel when auditing broad scopes. For example, when auditing `sdk` scope, dispatch parallel agents to read all four SDK sources and the doc files simultaneously. When auditing `full`, process each sub-scope sequentially but parallelize within each sub-scope.

## Step 3: Audit

For each documentation file in the selected scope, check the following (all that apply):

### Code snippet accuracy

- **Import paths**: Do import statements reference exports that actually exist in the packages? (e.g., `import { ContractValidator } from '@sahina/cvt-sdk'`, `from cvt_sdk import ContractValidator`, `import "github.com/sahina/cvt/sdks/go/cvt"`, `import io.github.sahina.sdk.ContractValidator`)
- **Function signatures**: Do `registerSchema()`, `validate()`, `compareSchemas()`, `generateFixture()`, `listEndpoints()`, `registerConsumer()`, `canIDeploy()`, etc. use the correct parameter shapes?
- **Return types**: Do documented return values match actual implementations?
- **Configuration objects**: Do option objects (e.g., `ContractValidatorOptions`, `TLSOptions`, `GenerateOptions`) show the correct property names and types?
- **Adapter usage**: Do documented adapter patterns (axios, fetch, express, fastify, requests, OkHttp, chi, gin, servlet) match actual implementations?

### CLI accuracy

- **Command names**: Do documented commands match actual Cobra commands? (`validate`, `compare`, `generate`, `serve`, `can-i-deploy`, `wait`, `register-schema`, `version`)
- **Flags**: Are all documented flags real? Are types and defaults correct?
- **Missing commands**: Are there commands in the implementation not documented?
- **Output format**: Do documented output examples match actual CLI output?

### API accuracy

- **gRPC methods**: Do documented RPC methods exist in the proto definition?
- **Request/response shapes**: Do documented payloads match proto message definitions?
- **Breaking change types**: Are documented `BreakingChangeType` enum values accurate?
- **Consumer registry**: Are consumer registration and `CanIDeploy` flows documented correctly?

### Configuration accuracy

- **Environment variables**: Do documented env vars (`CVT_PORT`, `CVT_STORAGE_*`, `CVT_TLS_*`, `CVT_API_KEY_*`, `CVT_POSTGRES_*`, etc.) match what the server actually reads?
- **Default values**: Are documented defaults correct?
- **Port numbers**: Are ports (9550 gRPC, 9551 metrics, 9091 Prometheus, 3000 Grafana) documented correctly?

### Feature accuracy

- **Feature claims**: Does the doc claim a feature exists that is actually implemented?
- **Feature descriptions**: Does the described behavior match actual behavior?
- **SDK feature parity**: Do all four SDKs support the features documented for them?

### Cross-document consistency

- **Terminology**: Is the same feature described consistently across docs?
- **API examples**: Do different docs showing the same API use consistent patterns?
- **Feature lists**: Do feature lists in README, reference docs, and SDK docs agree?
- **Version numbers**: Are SDK versions consistent across docs?

### Link integrity

For every markdown link in each audited file, verify the target exists:

- **Relative file links**: `[text](./path/to/file.md)` or `[text](../other/file.md)` — resolve relative to the file's directory and verify the target file exists on disk using Glob or Read. Include anchor fragments (`#section-name`) — verify the target file has a heading that would produce that anchor (lowercase, hyphens for spaces, strip punctuation).
- **Cross-doc links**: `[text](../../guides/consumer-testing.mdx)` — resolve the full path and verify.
- **Image links**: `![alt](../images/foo.png)` — verify image file exists.
- **GitHub links**: Verify they use the correct repo URL (`github.com/sahina/cvt`).
- **Package registry links**: Verify npm, PyPI, Maven Central, pkg.go.dev, and GHCR links are correct.
- **Missing links**: If a doc references a feature that has its own dedicated doc page but doesn't link to it, flag it as a candidate for a new link.

Common broken link patterns to watch for:

- Files that were renamed or moved (e.g., `.md` to `.mdx` or vice versa)
- Anchor links to headings that were renamed
- Relative paths that are wrong after a doc was moved to a different directory depth
- Links to files that were deleted

### Markdown quality

Each audited markdown file should be clean, valid markdown. Check for:

- **Heading hierarchy**: No skipped heading levels (e.g., `##` followed directly by `####` without `###`). Headings should descend in order.
- **Unclosed or mismatched formatting**: Unmatched `**`, `_`, `` ` ``, or `~~` markers.
- **Broken code fences**: Code blocks with mismatched opening/closing ` ``` ` markers, or missing language specifiers on fenced code blocks that contain code.
- **Trailing whitespace and blank lines**: No excessive trailing blank lines at end of file. No trailing whitespace on lines (single trailing newline at EOF is fine).
- **List formatting**: Consistent list markers within the same list (don't mix `-` and `*`). Proper indentation for nested lists (2 or 4 spaces, consistent within a file).
- **Table formatting**: Tables should have proper header separator rows (`| --- | --- |`). Column counts should be consistent across rows.
- **Duplicate headings at same level**: Avoid identical heading text at the same level within a file (causes ambiguous anchor links).
- **HTML in markdown**: Avoid raw HTML when markdown syntax suffices. HTML comments are fine.
- **Line length**: Not a hard rule, but flag extremely long lines (>500 chars) that aren't in code blocks or tables — they likely indicate a formatting issue.
- **Frontmatter**: If a file uses YAML frontmatter (`---`), it should be valid YAML with properly quoted strings.
- **MDX compatibility**: For `.mdx` files, verify JSX components are valid and imports exist.

## Step 4: Fix

For each issue found, fix it directly using the Edit tool. Apply these principles:

- **Match the existing style** of the document being edited. Don't rewrite sections unnecessarily.
- **Fix only what's wrong**. Don't refactor, reformat, or "improve" content that is accurate.
- **Preserve document structure**. Don't reorganize sections, change headings, or alter the flow.
- **Update code snippets** to match current API signatures, imports, and patterns. Use the actual source code as the reference, not what you think the API should be.
- **Add missing items** only when there's a clear gap (e.g., a shipped CLI command not in the CLI reference). Don't add speculative content.
- **Remove stale content** only when the referenced feature, API, or behavior no longer exists.
- **Fix broken links** by updating the path to the correct target. If the target file was deleted and no replacement exists, remove the link and leave the descriptive text. If the target was moved, update the relative path.
- **Fix markdown issues** that violate the checks above. For heading hierarchy violations, add the missing intermediate heading level or adjust the violating heading. For broken code fences, close them properly. For inconsistent list markers, normalize to match the file's dominant style.

### What NOT to fix

- Don't add docstrings, comments, or annotations that weren't there before
- Don't change the writing tone or voice
- Don't add sections for features that are "coming soon" unless they're already shipped
- Don't fix typos or grammar unless they cause confusion about technical content
- Don't change formatting preferences (e.g., tabs vs spaces in code snippets)
- Don't reformat markdown that is valid but stylistically different from your preference (e.g., don't change `*` list markers to `-` if the file consistently uses `*`)

## Step 5: Report

After completing all fixes, produce a structured report with this format:

```md
## Documentation Audit Report — [scope]

### Summary

- **Files audited**: N
- **Files with issues**: N
- **Total fixes applied**: N

### Changes by file

#### `path/to/file.md`

- **[code snippet]** Updated `registerSchema()` signature — added missing `version` parameter (line XX)
- **[feature claim]** Removed reference to "Priority Queues" — not yet implemented (line XX)
- **[cli]** Added `--json` flag to `cvt compare` documentation (line XX)

#### `path/to/other-file.md`

- **[cross-doc]** Updated feature list to match SDK capabilities (line XX)

### Files with no issues

- `path/to/clean-file.md`
- `path/to/another-clean-file.md`

### Notes

Any observations about systemic issues, patterns of drift, or areas that need attention beyond this audit.
```

Tag each fix with one of: `[code snippet]`, `[cli]`, `[api]`, `[config]`, `[feature claim]`, `[cross-doc]`, `[stale content]`, `[missing content]`, `[broken link]`, `[markdown]`.

## Handling large scopes

For scopes with many files (e.g., `full`, `reference`), break the work into batches:

1. List all files in the scope
2. Process files in groups of 5-8, using subagents where possible
3. Track progress and report incrementally if the user asks
4. Produce the combined report at the end

For `full` scope specifically, process sub-scopes in this order (dependencies flow downward):

1. `project` — top-level claims inform everything else
2. `sdk` — SDK truth informs all code snippets
3. `cli` — CLI truth informs command references
4. `reference` — canonical reference docs
5. `getting-started` — onboarding materials
6. `guides` — task guides
7. `ai-helper` — AI-specific docs
8. `operations` — observability and deployment
9. `development` — contributing and release
10. `examples` — example code
11. `ci-templates` — CI/CD templates
12. `internal` — internal specs and plans
