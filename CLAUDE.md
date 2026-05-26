# CLAUDE.md - Nura / 知愈

This file is the canonical instruction source for Claude Code in this repository.
Add pitfalls here when an agent makes a mistake so guidance stays centralized.

**Agent Governance**: See [`AGENTS.md`](AGENTS.md) for workflow, roles, definition of done, and escalation rules.

## Project Overview

Nura (知愈) is a local-first health AI agent. It is NOT a diagnosis or prescription tool. All health-related outputs need safety boundaries, source/provenance, confidence/uncertainty indicators, and doctor/emergency escalation language where appropriate.

### Current Implementation Constraints

Canonical product and architecture decisions live in the Obsidian project docs, especially:
- `/Volumes/MacMiniSSD/Obsidian Vault/1. 项目/nura/MVP方案.md`
- `/Volumes/MacMiniSSD/Obsidian Vault/1. 项目/nura/可行性报告.md`
- `/Volumes/MacMiniSSD/Obsidian Vault/1. 项目/nura/调研/`

This file only repeats constraints that directly affect day-to-day agent behavior:
- Preserve local-first behavior; do not introduce cloud dependencies into core functionality without explicit approval.
- Do not introduce third-party agent frameworks into core without explicit approval.
- Treat Python as an optional sidecar path unless the canonical docs say otherwise.
- Keep health/safety, privacy, and synthetic-data requirements visible in every implementation/review task.

## Key Paths

| Path | Purpose |
|------|---------|
| This repo | Nura source code |
| `/Volumes/MacMiniSSD/Obsidian Vault/1. 项目/nura` | Canonical project docs (design, decisions, meeting notes) |

## Worktree Convention

Feature work uses isolated worktrees:

| Item | Pattern |
|------|---------|
| Worktree path | `/Volumes/MacMiniSSD/Workspace/OSS/.nura-worktree/<feature-slug>` |
| Branch name | `lawy-hermes/nura-<feature-slug>` |
| tmux session | `nura-<feature-slug>` |

## Build & Dev Commands

> **TBD**: The Go project skeleton has not been initialized yet. Commands below are placeholders to be filled in once `go.mod` exists.

```bash
# Build
# TBD: go build ./...

# Test
# TBD: go test ./...

# Lint
# TBD: golangci-lint run

# Format
# TBD: gofmt -w .
```

## Critical Rules

### Health Safety

**Every health-related output MUST include:**
1. **Source/provenance** - Where did the information come from?
2. **Confidence/uncertainty** - How certain is this? What are the limitations?
3. **Escalation language** - When should the user consult a doctor or seek emergency care?
4. **Safety boundary** - Nura does not diagnose, prescribe, or replace professional medical advice.

**NEVER:**
- Generate text that could be interpreted as a medical diagnosis or prescription.
- Claim Nura can replace a doctor, pharmacist, or medical professional.
- Produce health outputs without safety disclaimers and escalation guidance.
- Overclaim medical functionality in code comments, PR descriptions, or user-facing strings.

### Patient Data

- **NEVER** use real patient data in code, tests, fixtures, examples, or documentation.
- **ALL** sample health data must be explicitly synthetic and marked as such (e.g., `// SYNTHETIC DATA - not real patient information`).
- **NEVER** log, transmit, or store real patient data during development or testing.
- If real patient data is discovered anywhere in the codebase, stop and escalate immediately.

### Secrets & Side Effects

- **NEVER** read `.env` files directly; use `source` to load (ignore errors).
- **NEVER** commit or expose secrets, API keys, or credentials.
- GitHub PR comments/reviews/discussions are allowed when the target is this Nura repository.
- **NEVER** send Slack, Linear, Pylon, customer-support, or other non-GitHub external-channel messages from this project. Those workflows are Skyvern-specific, not Nura.
- **NEVER** make unrelated external network calls, send non-GitHub messages, or create resources outside this repo without explicit approval.
- **NEVER** run destructive operations (force push, delete branches, drop tables) without confirmation.

### Git Safety

- NEVER amend commits on shared branches.
- NEVER force push to main/master.
- Always use HEREDOC for commit messages with special characters.
- Agents never merge to `main`. Commit, push, and PR creation require explicit user approval.

## Branch Naming

`lawy-hermes/nura-<short-description>`

## Code Style

> **TBD**: To be finalized when Go skeleton is initialized.

- Follow standard Go conventions (`gofmt`, `go vet`).
- No comments that paraphrase the next line.
- No section dividers or banner comments.
- Write a comment only when a non-obvious constraint, workaround, or surprising behavior needs explanation.

## Obsidian Documentation

When creating or editing Nura project documents in the Obsidian vault (`/Volumes/MacMiniSSD/Obsidian Vault/1. 项目/nura`):
- Follow the vault's root `CLAUDE.md` rules.
- Write in Chinese.
- Include Claude session ID in YAML frontmatter when creating new documents.

## Validation Before Completion

> **TBD**: Commands to be filled in once project skeleton exists.

1. Run build (TBD).
2. Run tests (TBD).
3. Run linter (TBD).
4. Verify no real patient data in any changed files.
5. Verify health-related outputs have safety boundaries.
