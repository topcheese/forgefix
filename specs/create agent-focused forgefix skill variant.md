---
spec_id: "SPEC-1783612199"
status: review
repo_issue: ""
type: feature
version: "v0.8.0"
root_cause: ""
resolution: ""
---
# Create Agent Focused Forgefix Skill Variant

## Objective

Create a short, agent-focused variant of the forgefix-git-workflow skill that
contains only the lifecycle and essential commands, without branching strategy,
worktree details, or comprehensive reference sections. Agents load this variant
through the skill system for quick reference.

## Requirements

1. Agent variant lives at `skills/forgefix-git-workflow-agent.md` (~100 lines).
2. Contains only: lifecycle, auto-detect, auto-promote, ff passthrough mapping,
   verification checklist, and red flags.
3. Omitted from this variant: branching strategy, worktrees, changelog,
   configuration reference, rationalizations, internal architecture.
4. Both variants coexist in the repo for different use cases.

## Implementation

- Created `skills/forgefix-git-workflow-agent.md` alongside the full version.
- Full version (`-workflow.md`) stays at 623 lines with all detail preserved.
- Agent version (`-agent.md`) focuses only on what an agent needs to know.
- Both copied to `~/.agents/skills/forgefix-git-workflow/SKILL.md` (full version)
  — the agent variant is repo-only for optional use.

## Acceptance Criteria

- Agent variant fits in ~100 lines.
- Full version retains all original content.
- No raw git commands in either file.
- All ff commands use `--ai` flag.
- Verification checklist covers the essential flow.

## Verification

- Both files present in `skills/` directory.
- `grep -c '`git ' skills/forgefix-git-workflow-agent.md` is 0.
- Full test suite passes.

<!--
Available types: feature, bug, chore, docs, refactor, ops
Available versions: v0.8.0 (current), v0.9.0
-->