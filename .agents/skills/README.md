# CVT Agent Skills

Agent skills for CVT (Contract Validator Toolkit) SDK consumers. These skills guide AI agents through common CVT workflows.

Compatible with Claude Code, Codex CLI, Gemini CLI, Cursor, and other SKILL.md-compatible agents.

## Consumer Journey

| Phase | Skill | Description |
|-------|-------|-------------|
| Setup | `/cvt-setup` | Install SDK, configure server, create first test |
| Write Tests | `/cvt-write-contract` | Write consumer contract tests |
| Producer Testing | `/cvt-producer-test` | Validate API responses against schema |
| CI/CD | `/cvt-ci` | Integrate CVT into CI/CD pipelines |
| Breaking Changes | `/cvt-breaking-changes` | Detect breaking schema changes |
| Troubleshoot | `/cvt-troubleshoot` | Diagnose common CVT issues |

## Quick Start

1. Clone the CVT repo or ensure `.agents/skills/` is present
2. Open your project in an agent-supported editor
3. Ask the agent to run `/cvt-setup` to get started

## SDK Support

All skills support Node.js, Python, Go, and Java SDKs. Skills auto-detect your project language.
