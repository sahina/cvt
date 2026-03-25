---
title: AI Helper Overview
sidebar_label: Overview
sidebar_position: 1
description: Use AI coding agents to accelerate CVT integration
---

# AI Helper Overview

This guide helps you leverage AI coding agents (Claude Code, Cursor, GitHub Copilot, Windsurf, etc.) to accelerate your CVT integration.

## What AI Agents Can Help With

AI coding agents excel at CVT-related tasks:

- **Setup & Configuration**: Installing SDKs, configuring the server, setting up Docker
- **Writing Contract Tests**: Creating validation tests for your API consumers
- **Debugging Validation Errors**: Understanding why requests/responses fail validation
- **Schema Comparison**: Interpreting breaking change reports
- **CI/CD Integration**: Setting up contract testing in your pipelines
- **Fixture Generation**: Creating schema-compliant test data

---

## The llms.txt Approach

CVT provides LLM-friendly documentation through standardized `llms.txt` files. These files are optimized for AI agents to quickly understand the project and provide accurate assistance.

### Available Files

| File | Purpose | Best For |
|------|---------|----------|
| [llms.txt](https://raw.githubusercontent.com/sahina/cvt/main/llms.txt) | Summary with links | Quick orientation, finding specific docs |
| [llms-full.txt](https://raw.githubusercontent.com/sahina/cvt/main/llms-full.txt) | Complete API reference | Detailed implementation help |

### How to Use

Point your AI agent to the llms.txt files when starting a CVT-related task:

```text
I'm integrating with CVT for contract testing.
Reference: https://raw.githubusercontent.com/sahina/cvt/main/llms.txt
```

For detailed implementation questions:

```text
I need help with CVT's consumer registration API.
Reference: https://raw.githubusercontent.com/sahina/cvt/main/llms-full.txt
```

---

## Supported AI Tools

These tools work well with CVT's documentation:

### Claude Code

Add the llms.txt reference to your `CLAUDE.md` file. See [Context Templates](./context-templates.md) for the full template.

### Cursor

Add the llms.txt reference to your `.cursorrules` file. See [Context Templates](./context-templates.md) for the full template.

### GitHub Copilot

Reference llms.txt in your copilot instructions file or conversation context.

### Windsurf

Add llms.txt to your project context configuration.

### Other Tools

Any AI coding agent that supports project context files can use the llms.txt approach.

---

## Agent Skills (Recommended)

If your AI tool supports the [SKILL.md format](https://github.com/anthropics/skills) (Claude Code, Codex CLI, Gemini CLI, Cursor, and others), CVT provides **6 agent skills** in `.agents/skills/` that guide workflows step-by-step with auto-language-detection and SDK-specific code:

| Skill | What It Does |
|-------|-------------|
| `/cvt-setup` | Install SDK, configure server, create first contract test |
| `/cvt-write-contract` | Write consumer contract tests |
| `/cvt-producer-test` | Validate API responses against your schema |
| `/cvt-ci` | Integrate CVT into CI/CD pipelines |
| `/cvt-breaking-changes` | Detect breaking schema changes |
| `/cvt-troubleshoot` | Diagnose common CVT issues |

To use skills, clone the CVT repo and open your project. The agent discovers skills automatically from `.agents/skills/`.

---

## llms.txt Approach (Universal)

For agents that don't support SKILL.md, or for quick reference, CVT provides LLM-friendly documentation:

1. **Add context to your project**: Create a context file for your AI tool (see [Context Templates](./context-templates.md))

2. **Point to llms.txt**: Include the URL in your context file:
   ```
   https://raw.githubusercontent.com/sahina/cvt/main/llms.txt
   ```

3. **Start asking questions**: Your AI agent now has CVT knowledge

### Example Conversation

```text
User: Set up CVT for my Node.js project

AI: I'll help you set up CVT. Based on the CVT documentation, here's what we need:

1. Start the CVT server:
   docker run -d -p 9550:9550 ghcr.io/sahina/cvt:latest

2. Install the Node.js SDK (see installation guide for details):
   npm install @sahina/cvt-sdk

3. Create your first contract test...
```

---

## What's in This Guide

| Page | Content |
|------|---------|
| [Context Templates](./context-templates.md) | Copy-paste templates for CLAUDE.md, .cursorrules, and other tools |
| [Common Mistakes](./common-mistakes.md) | Pitfalls to avoid when using AI with CVT |
| [Advanced Patterns](./advanced-patterns.md) | Schema evolution, CI/CD, and fixture generation workflows |
| [OpenAPI Schema Generator](./openapi-schema-generator.md) | Adaptive prompt for generating or improving OpenAPI schemas from code |

---

## Next Steps

- **[Agent Skills](./../../../.agents/skills/README.md)** - Use agent skills for guided CVT workflows (recommended)
- **[Context Templates](./context-templates.md)** - Set up your AI tool with CVT context (for llms.txt approach)
- **[Consumer Testing Guide](https://sahina.github.io/cvt/docs/guides/consumer-testing)** - Learn the full consumer testing workflow
- **[API Reference](https://sahina.github.io/cvt/docs/reference/api)** - Complete API documentation
