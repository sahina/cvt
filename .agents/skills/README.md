# CVT Agent Skills

Agent skills for CVT (Contract Validator Toolkit) SDK consumers. These skills guide AI agents through common CVT workflows.

Compatible with Claude Code, Codex CLI, Gemini CLI, Cursor, and other SKILL.md-compatible agents.

## Consumer Journey

| Phase            | Skill                   | Description                                      |
| ---------------- | ----------------------- | ------------------------------------------------ |
| Setup            | `/cvt-setup`            | Install SDK, configure server, create first test |
| Write Tests      | `/cvt-write-contract`   | Write consumer contract tests                    |
| Producer Testing | `/cvt-producer-test`    | Validate API responses against schema            |
| CI/CD            | `/cvt-ci`               | Integrate CVT into CI/CD pipelines               |
| Breaking Changes | `/cvt-breaking-changes` | Detect breaking schema changes                   |
| Troubleshoot     | `/cvt-troubleshoot`     | Diagnose common CVT issues                       |

## Install

Run this from your project root:

```bash
curl -sL https://github.com/sahina/cvt/archive/main.tar.gz \
  | tar xz --strip-components=1 'cvt-main/.agents'
```

This copies `.agents/skills/` into your project. Commit it to version control so your whole team gets the skills.

### Update

Re-run the same command to pull the latest skills:

```bash
curl -sL https://github.com/sahina/cvt/archive/main.tar.gz \
  | tar xz --strip-components=1 'cvt-main/.agents'
```

Check if you're up to date by comparing your local version:

```bash
cat .agents/skills/VERSION 2>/dev/null || echo "no version file"
```

## Quick Start

1. Install skills (see above) or clone the CVT repo
2. Open your project in an agent-supported editor (Claude Code, Codex, Gemini CLI, Cursor, etc.)
3. Ask the agent to run `/cvt-setup` to get started

## SDK Support

All skills support Node.js, Python, Go, and Java SDKs. Skills auto-detect your project language.
