# AGENTS.md - Agent Governance

This file defines how AI agents work in this repository: what they can do, when they must stop, and what "done" means. It is tool-agnostic (Claude Code, Codex, Copilot, or any future agent).

For tool-specific configuration, see `CLAUDE.md`.

---

## Workflow

All non-trivial agent work follows this flow:

```
Planner -> Implementer -> Reviewer -> QA -> Human Merge
```

1. **Planner** produces a scoped plan with file paths, acceptance criteria, and verification steps.
2. **Implementer** executes the plan using TDD (RED-GREEN-REFACTOR).
3. **Reviewer** checks the diff against the plan and coding standards.
4. **QA** runs verification commands and confirms output matches expectations.
5. **Human** reviews the PR and merges.

Agents never merge to `main`. A human always performs the final merge.

Agents open PRs as ready for review by default. Do not create draft PRs unless the human explicitly asks for a draft, or a documented blocker makes the PR not ready for human review.

---

## Roles

### Planner
- **Responsibilities**: Analyze the task, research the codebase, produce a step-by-step plan with exact file paths and verification commands.
- **Restrictions**: Must not modify code. Output is a plan document only.

### Implementer
- **Responsibilities**: Execute the plan. Write failing tests first, then minimal code to pass, then refactor. Commit atomically per logical change.
- **Restrictions**: Must stay within the plan's scope. Must not skip tests for production code changes. Must not modify files outside the plan without re-planning.

### Reviewer
- **Responsibilities**: Review the diff against the plan. Check for correctness, security, scope creep, health/safety language, and adherence to project conventions.
- **Restrictions**: Must not modify code directly. Reports issues for the Implementer to fix.

### QA
- **Responsibilities**: Run all verification commands (tests, type checks, linting). Confirm all pass with evidence.
- **Restrictions**: Must not modify code. Reports failures with reproduction steps.

---

## Definition of Done

A change is PR-ready when ALL of the following are true:

1. **Scope match** - The diff addresses exactly what the task specifies. No unrelated changes, no missing requirements.
2. **Tests pass** - All relevant test suites pass (see `CLAUDE.md` for canonical commands). New behavior has corresponding tests.
3. **Verification evidence** - Actual command output is captured, not assumed. No "should work" or "looks correct."
4. **No security regressions** - No new command injection, XSS, SQL injection, or secrets in code.
5. **Risk note + rollback** - If the change touches health/safety logic, data storage, policy enforcement, or LLM prompts, the PR description includes: what could go wrong and how to roll back.
6. **Safety & privacy review** - Health-related outputs include appropriate safety boundaries (source/provenance, confidence/uncertainty, escalation language). No real patient data anywhere.
7. **Source evidence retention** - User-uploaded health artifacts (screenshots, reports, PDFs/images, OCR source files) are archived as original local source evidence with path/hash/metadata before parsing. Do not keep only OCR text or derived structured records.
8. **Synthetic data only** - All sample/test health data is explicitly synthetic and marked as such. No real patient names, conditions, or records.

---

## Stop Conditions

Agents MUST stop and escalate when any of these conditions are met:

### Medical Safety
- The change could produce health advice, diagnosis-like output, or treatment suggestions without appropriate safety boundaries.
- Health-related outputs lack source/provenance, confidence/uncertainty indicators, or doctor/emergency escalation language.
- Any real patient data is encountered or would be needed to proceed.

### Scope Ambiguity
- The task is unclear and multiple valid interpretations exist.
- Requirements conflict with existing behavior and the intended resolution is not specified.

### Size Limits
- The change touches **more than 20 files**.
- The diff exceeds **500 lines of code** (excluding generated files and test fixtures).
- The task requires changes across **more than 3 subsystems**.

### Protected Paths
Agents must NOT **commit, stage, or push** changes to these paths without explicit human approval:
- `.github/workflows/` - CI/CD pipeline definitions
- Database migration files
- `Dockerfile*`, `docker-compose*` - Container configurations
- `*.yaml` / `*.yml` in repo root - Infrastructure configs
- `CLAUDE.md`, `AGENTS.md` - Governance files (unless the task's sole purpose is governance documentation)

Never put real secret values (API keys, tokens, credentials) into commits, staged changes, remote branches, PRs, logs, or chat.

Agents may use GitHub PR comments/reviews/discussions when the target is this Nura repository. Agents must not send Slack, Linear, Pylon, customer-support, or other non-GitHub external-channel messages from this project. Those workflows are Skyvern-specific. If a non-GitHub message is needed, draft it for human approval instead of sending it.

### Dependency Changes
- Adding, removing, or upgrading dependencies in `go.mod`, `go.sum`, `pyproject.toml`, `package.json`, or lock files requires human approval.

### Real Patient Data
- If real patient data is discovered in the codebase, test fixtures, or environment, stop immediately and escalate. Do not process, log, or transmit it.

### Repeated Failures
- If a test or build fails **3 times in a row** after attempted fixes, stop and escalate.
- If a pre-commit hook fails **2 times in a row** on the same issue, stop and investigate root cause before retrying.

### Destructive Operations
- Force-pushing, deleting branches, dropping database tables, or modifying shared infrastructure always requires human confirmation.

---

## Escalation Protocol

When a stop condition is triggered:

1. **Document** what was attempted and why it failed or is blocked.
2. **Write findings** to the PR description, a ticket comment, or a plan document - never rely on chat history alone.
3. **State clearly** which stop condition was hit and what decision is needed from a human.
4. **Do not guess** past the ambiguity. Partial progress with clear notes is better than a wrong solution.

Format for escalation notes:

```
## Escalation: [Stop Condition Name]

**What I tried**: [Brief description]
**What blocked me**: [Specific issue]
**Decision needed**: [What the human needs to decide]
**My recommendation**: [Optional - what the agent would do if allowed]
```

---

## Coordination

- **Shared state lives in repo artifacts** (plans, PRs, code comments), not in chat history.
- Agents in parallel workflows must not modify the same files without coordination.
- When multiple agents work on related changes, use feature branches and merge sequentially.
- Plans must be committed to a version-controlled location so other agents and humans can reference them.
