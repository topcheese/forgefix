---
name: forgefix-git-workflow
description: "Use when making code changes, managing specs, or binding commits to issues in a ForgeFix-enabled project. The AGENT workflow is exactly three steps: ff spec --ai → ff test --ai → ff commit --ai (only after tests pass). ff sync and ff ship are HUMAN-ONLY commands and must never be run by the agent. All git operations go through ff git passthrough."
---

# ForgeFix Spec-Driven Git Workflow

## Overview

ForgeFix (`ff`) replaces ad-hoc git workflow with a **spec-driven development lifecycle**. Every change is traceable to a spec, every spec links to a remote issue, and no code ships without a paper trail.

```
AGENT WORKFLOW (exactly five steps):
  1. ff specs --search "<title>"   → Check for duplicates before creating (SPEC-1783931981)
  2. ff spec --ai <title>          → Create a spec (issue contract)
  3. [OUTPUT PRE-COMMIT SUMMARY]   → Print structured summary BEFORE calling ff commit
  4. ff test --ai                  → Write the spec's tests, implement against the spec, run tests
  5. ff commit --ai <msg>          → Only after tests pass + summary output: auto-detect spec, commit with binding, set "review"

CRITICAL RULES:
  - One spec at a time: commit the current spec before starting work on another
  - Pre-commit summary MUST be output BEFORE ff commit --ai, not after
  - ff commit --ai shows acceptance criteria from the spec — review them before confirming

HUMAN-ONLY COMMANDS (never run by the agent):
  ff sync                → Talks to the remote issue tracker; requires explicit confirmation
  ff ship                → Pushes commits/tags to a remote and creates a release; requires explicit confirmation
  ff archive             → Archives closed specs (human housekeeping)

The agent MUST NOT run `ff sync`, `ff ship`, or `ff archive`. If a workflow
seems to need them, stop and ask the human. As of the remote-target safeguard,
`ff sync` and `ff ship` themselves refuse to run without explicit confirmation
and are refused by default (and always in --ai mode).
```

**Never use raw `git` commands.** Every git operation uses `ff`'s transparent git passthrough — `ff log`, `ff diff`, `ff branch`, `ff rebase`, `ff stash`, `ff tag`, `ff worktree`, `ff reset`, `ff bisect`, `ff blame`. All work as drop-in replacements.

## When to Use

Every code change in a ForgeFix-enabled project. The `ff` binary must be in PATH and a `<project>_ff.yaml` config must exist in the project root (auto-generated on first `ff spec` or `ff commit`).

For git operations that `ff` doesn't handle natively (`log`, `diff`, `branch`, `rebase`, `stash`, etc.), `ff` passes them through to `git` transparently — just type `ff log --oneline` instead of `git log --oneline`.

## Core Principles

### Trunk-Based Development (Recommended)

Keep `main` always deployable. Work in short-lived feature branches that merge back within 1-3 days.

```
main ──●──●──●──●──●──●──●──●──●──  (always deployable)
        ╲      ╱  ╲    ╱
         ●──●─╱    ●──╱    ← short-lived feature branches (1-3 days)
```

- **Dev branches are costs.** Every day a branch lives, it accumulates merge risk.
- **Release branches are acceptable.** When you need to stabilize a release while main moves forward.
- **Feature flags > long branches.** Prefer deploying incomplete work behind flags rather than keeping it on a branch for weeks.
- **Spec-driven workflow adapts to any branching model.** Whether you use trunk-based, gitflow, or release branches, each feature gets its own spec and commits are bound to it.

### 1. Spec First, Code Second

Before writing code, create a spec:

```
ff spec --ai "my-feature"
# → Creates specs/my-feature.md with a unique SPEC-<timestamp> ID
# → Registers it in the ledger as "draft"
# → --ai creates silently; without --ai, opens in $EDITOR
```

The spec file is Markdown with YAML frontmatter:

```markdown
---
spec_id: "SPEC-1741712345"
status: draft
type: feature
version: "1.0"
repo_issue: ""
---
# My Feature

## Objective
What this feature accomplishes.

## Requirements
- Requirement one

## Implementation
- Implementation detail

## Acceptance Criteria
- [ ] Criterion one
```

**Workflow pattern:**

```
Agent starts work
    │
    ├── ff spec --ai "my-feature"     ← Define what you're doing
    ├── Implement slice               ← Write code
    │   ├── Test passes?
    │   │   ├── Yes → ff commit --ai "message" → Continue
    │       │   └── No  → ff reset --hard HEAD → Investigate
    │
    └── Feature complete → [human handles ff sync/ff ship/ff archive]
```

### 2. Spec Lifecycle States

With `--ai`, the lifecycle is fully automatic. Without `--ai`, edit the `status` field in the YAML frontmatter:

| Status | Auto Set By | Manual Edit | Description |
|--------|-------------|-------------|-------------|
| `draft` | `ff spec --ai` | initial creation | Newly created, not yet actionable |
| `review` | `ff commit --ai` | edit status | Committed, ready for feedback |
| `ship` | `ff sync --ai` | edit status | Approved — passes the Shipping Gate |
| `closed` | `ff ship --ai` (via housekeeping drain) | edit status | Shipped, remote issue closed |
| archived | `ff archive --ai` | — | Moved to archive document |

The auto-promote (`ff sync --ai` promotes review→ship) only runs with `--ai` flag. Without `--ai`, the function checks if stdin is a terminal and prompts interactively.

### 3. Atomic Specs, Atomic Commits

Each spec describes one logical unit of work. Each commit binds to exactly one spec.

```
ff spec --ai "add-task-validation"
ff commit --ai "add Zod schema validation to POST endpoint"
ff commit --ai "add validation error UI to form component"
```

A spec can have multiple commits (one logical feature). A commit must belong to at least one spec.

### 4. Commit with Spec Binding

`ff commit` is the **only** way to commit. Raw `git commit` breaks traceability and is forbidden.

`ff commit`:
1. **Auto-detects the spec** from the working tree (most recently modified active spec file)
2. **Stages all changes** automatically via `GitHelper.AddAll()`
3. **Binds the commit** to a spec ID in the ledger
4. **Auto-formats the message** as `feat: [SPEC-XXX] <description>`
5. **Advances the spec** to "review"
6. **Folds metadata** into the commit (spec status, ledger binding) via pre-commit write + amend — no untracked side effects
7. **Preserves references** to other specs in the message (dedup strips only the current spec's tag)
8. **Queues a background sync** to the remote issue tracker
9. **Drains housekeeping queue** to process any pending metadata tasks

```
# Auto-detect spec (recommended for agents):
ff commit --ai "add user authentication"

# Explicit spec binding:
ff commit --ai --spec SPEC-1741712345 "fix login timeout"

# With metadata flags:
ff commit --ai --type bug "fix login timeout"
```

**If you omit both `--ai` and `--spec`**, ForgeFix shows an interactive categorized menu (Feature / Bug / Refactor / All) to pick the spec. In agent mode always use `--ai`.

#### Auto-Detect Heuristic

When `--ai` is set and no `--spec` is provided, `autoDetectSpecFromWorkingTree` scans `specs/` and picks the most recently modified spec file that is NOT at `ship` or `closed` status. This ensures that `ff spec --ai "foo"` → `ff commit --ai "msg"` works automatically — the newly created spec (draft) is the most recent active file.

#### Commit Message Dedup

If the message already contains `[SPEC-XXX]`, only the matching spec's tag is stripped before prepending `feat: [SPEC-XXX]`. References to other specs are preserved:

```
# Message: "[SPEC-111] integrate with [SPEC-456]"
# ff commit strips only [SPEC-111], preserves [SPEC-456]
# → "feat: [SPEC-111] integrate with [SPEC-456]"
```

### 5. Descriptive Messages

`ff commit` auto-formats: `feat: [SPEC-XXX] <your message>`. When using raw git (disallowed), follow conventional commits:

```
<type>: <short description>

<optional body explaining why, not what>
```

**Types:** `feat`, `fix`, `refactor`, `test`, `docs`, `chore`

### 6. Keep Concerns Separate

Don't combine formatting changes with behavior changes. Don't combine refactors with features.

```
# Good: Separate concerns (two specs, two commits)
ff spec --ai "extract validation utility"
ff commit --ai "extract validation logic to shared utility"

ff spec --ai "add phone number validation"
ff commit --ai "add phone number validation to registration"

# Bad: Mixed concerns in one spec
ff spec --ai "big-feature"    # Too broad, can't ship incrementally
```

### 7. Size Your Changes

Target ~100 lines per commit/PR. Changes over ~1000 lines should be split.

```
~100 lines  → Easy to review, easy to revert
~300 lines  → Acceptable for a single logical change
~1000 lines → Split into smaller specs and commits
```

### 8. The Ledger Is the Source of Truth

ForgeFix stores spec state in `.ff/forgefix_ledger.json` — a JSON file that maps spec IDs to their status, linked commits, and remote issue IDs.

```
ff specs              # Active specs only
ff specs --all        # Include closed specs
```

**Check the ledger into version control.** It's the portable history of your project's spec state.

### 9. Spec Status Lifecycle

```
draft → review → ship → closed → archived
```

With `--ai`: `ff commit --ai` sets review, `ff sync --ai` promotes to ship, `ff ship --ai` sets closed, `ff archive --ai` archives.

Without `--ai`: edit the `status` field in the spec file's YAML frontmatter manually.

**Do not skip states.** The Shipping Gate (`ff ship`) rejects any spec not in `ship` status. The promote function (`promoteReviewSpecs` in `sync.go`) only promotes `review` to `ship`, not draft or backlog.

### 10. Sync to Remote Issue Tracker

When credentials are configured in `<project>_ff.yaml`, `ff sync` creates or updates remote issues (GitHub/Gitea) to match your local specs:

```
ff sync --ai           # Sync all specs, auto-promote review→ship
ff sync /path/to/project   # Sync a specific project
```

With `--ai`, the `promoteReviewSpecs` function auto-confirms promotion instead of checking `term.IsTerminal()`. Without `--ai`, it prompts interactively.

**Order matters:** `promoteReviewSpecs` runs BEFORE `SyncSpecs` so that review specs are promoted to ship before the sync sees a previously-closed remote issue and prematurely closes them.

**Configuring Remote Sync:**

```yaml
# <project>_ff.yaml
github:
  owner: "my-org"
  repo: "myapp"
  token: "ghp_..."
  base_url: "https://api.github.com"  # or Gitea: http://host:3000/api/v1
auto_issue_management: true
```

Supported backends: GitHub, Gitea, and any GitHub API-compatible host.

## Git Passthrough

Every git command that `ff` doesn't have a native subcommand for passes through transparently:

```
# Repository inspection
ff log --oneline -5         ff diff --cached            ff status --short
ff blame src/file.ts        ff log --grep="bug"         ff diff HEAD~5..HEAD

# Branching
ff branch                   ff checkout -b feature/x    ff merge feature-x

# History manipulation
ff rebase main              ff reset HEAD~1             ff stash list

# Debugging
ff bisect start             ff bisect bad HEAD          ff bisect good <commit>

# Tags and releases
ff tag -a v1.0 -m "Release"  ff push origin v1.0        ff push origin main

# Worktrees
ff worktree add ../path branch    ff worktree remove ../path

# Explicit form
ff git log --oneline
```

**Exit codes are preserved.** If `ff log -999` returns exit code 128, it matches `git log -999`.

**Known ForgeFix commands take priority:** `commit`, `spec`, `sync`, `ship`, `specs`, `archive`, `help`, `version`, `backlog`, `status`, `config`, `export`, `import` always route to their native handlers.

**Configurable opt-out:** Set `git_passthrough: false` in `<project>_ff.yaml` to restore the original behavior (unknown commands fall through to the test runner).

## Branching Strategy

### Feature Branches

```
main (always deployable)
  │
  ├── feature/task-creation    ← One feature per branch
  ├── feature/user-settings    ← Parallel work
  └── fix/duplicate-tasks      ← Bug fixes
```

- Branch from `main`, keep short-lived (1-3 days)
- Delete branches after merge
- Prefer feature flags over long-lived branches

### Branch Naming

```
feature/<short-description>   → ff checkout -b feature/task-creation
fix/<short-description>       → ff checkout -b fix/duplicate-tasks
chore/<short-description>     → ff checkout -b chore/update-deps
refactor/<short-description>  → ff checkout -b refactor/auth-module
```

## Working with Worktrees

```
ff worktree add ../project-feature-a feature/task-creation
ff worktree add ../project-feature-b feature/user-settings

# Each worktree has its own spec and commit lifecycle
cd ../project-feature-a
ff spec --ai "feature-a"
ff commit --ai "implement feature A"

cd ../project-feature-b
ff spec --ai "feature-b"
ff commit --ai "implement feature B"

# Clean up
ff worktree remove ../project-feature-a
```

Benefits: parallel agent work, no branch switching, isolated experiments.

## The Save Point Pattern

```
Agent starts work
    │
    ├── Makes a change
    │   ├── Test passes? → ff commit --ai "msg" → Continue
    │   └── Test fails?  → ff reset --hard HEAD → Investigate
    │
    └── Feature complete → [human handles ff sync/ff ship/ff archive]
```

Never lose more than one increment of work. Each save point has a paper trail via spec binding.

When the feature is complete and all specs are in review, the human operator runs `ff sync --ai`, `ff ship --ai`, and `ff archive --ai` to close the loop.

## Pre-Commit Summary (MANDATORY)

**Before** calling `ff commit --ai`, the agent MUST output a structured change summary. This is not optional — the summary is the evidence that the work is deliberate, scoped, and tested. Output it as plain text in the conversation BEFORE the commit command:

```
SPEC: SPEC-XXXXXXXXXX

CHANGES MADE:
- <file>: <what changed and why>

THINGS I DIDN'T TOUCH (intentionally):
- <file or scope>: <why it was out of scope>

POTENTIAL CONCERNS:
- <dependency changes, risk areas, edge cases>

VERIFICATION:
- Tests: PASS (X/Y)
```

Include the **spec ID** at the top. This summary is the agent's declaration that the work matches the spec. If it doesn't match, don't commit — re-read the spec first.

## Pre-Commit Hygiene

```
# 1. Check what you're about to commit
ff diff --staged

# 2. Ensure no secrets
ff diff --staged | grep -i "password\|secret\|api_key\|token"

# 3. Run tests
ff --ai      # Headless JSON output

# 4. Build
go build ./...
```

## Handling Generated Files

- **Commit generated files** only if the project expects them (`package-lock.json`, Prisma migrations)
- **Don't commit** build output, `.env`, IDE config
- **Have a `.gitignore`** covering standard exclusions
- **ForgeFix's `.ff/` directory** is listed in `.gitignore` by default — un-ignore to share ledger history

## Spec Lifecycle in Practice

### New Feature (Agent Workflow)

```
1. ff spec --ai "user-authentication"           → draft
2. Implement auth logic
3. ff commit --ai "add JWT middleware"          → review
4. ff commit --ai "add login endpoint"          → review
                                                 ─────────────────
                                                 Agent stops here.
                                                 HUMAN-ONLY:
5. ff sync --ai                                 → ship (auto-promote)
6. ff ship --ai                                 → closed (push + housekeeping)
7. ff archive --ai                              → archived
```

### New Feature (Manual Workflow)

```
1. ff spec user-authentication                  → draft
2. Edit specs/user-authentication.md            → fill in requirements
3. Edit status to "backlog"                     → backlog
4. Implement auth logic
5. ff commit --spec SPEC-X "add JWT middleware"  → review
6. ff commit --spec SPEC-X "add login endpoint"  → review
7. ff sync                                      → create/update remote issue
8. Edit status to "ship"                        → ship (approved)
9. ff ship                                      → closed + release
```

### Bug Fix

```
# Agent workflow (--ai):
1. ff spec --ai "fix login null pointer"        → draft
2. Implement fix
3. ff commit --ai "fix null check"              → review
                                                 ─────────────────
                                                 Agent stops here.
                                                 HUMAN-ONLY:
4. ff sync --ai                                 → ship
5. ff ship --ai                                 → closed
6. ff archive --ai                              → archived

# Interactive:
1. ff commit                          → interactive mode
2. Select "Bug" category → [0] New Bug → enter title
3. Implement fix
4. ff commit --spec SPEC-X "fix null pointer"   → review
5. ff sync
6. Edit status to "ship"
7. ff ship
```

### Refactoring

```
1. ff spec --ai "extract-validation-module"     → draft
2. Set type: refactor in spec frontmatter
3. ff commit --ai "extract email validation"     → review
4. ff commit --ai "extract phone validation"     → review
                                                 ─────────────────
                                                 Agent stops here.
                                                 HUMAN-ONLY:
5. ff sync --ai                                 → ship
6. ff ship --ai                                 → closed
7. ff archive --ai                              → archived
```

## Release & Versioning

### Semantic Versioning

```
MAJOR  breaking change — consumers must change their code
MINOR  new functionality, backward-compatible
PATCH  bug fix, backward-compatible
```

Record version numbers in spec frontmatter.

### Tag the Release

Use `ff`'s git passthrough:

```
ff tag -a v1.4.0 -m "Release 1.4.0"
ff push origin v1.4.0
```

The ledger doesn't replace git tags — tags remain the source of truth for releases.

### Keep a Changelog

```
## [1.4.0] - 2025-06-12
### Added
- Bulk task import via CSV ([SPEC-1741712345])
```

## Shipping Gate

`ff ship` is the **Strict Shipping Gate**. It checks every active spec:

- **Rejects** if any spec is `backlog` or `in-progress`
- **Passes** if all active specs are `ship` or `closed`
- With `--ai`, version prompts are auto-confirmed
- On success: pushes commits, runs housekeeping immediately (close issues, sync metadata → `closed`)

**Before `ff ship`:**

```
1. ff specs          → Verify all specs show "ship" or "closed"
2. ff sync --ai      → Promote any review specs, sync remote issues
3. ff ship --ai      → Gate opens, pushes, closes
```

## Internal Architecture

### GitHelper Abstraction (`engine/git_helper.go`)

All raw `os/exec.Command("git", ...)` calls are encapsulated in the `GitHelper` struct:

```
GitHelper.AddAll()      → git add --all (handles .gitignore correctly)
GitHelper.Add(files...) → git add -- <files>
GitHelper.Commit(msg)   → git commit -m, returns short hash
GitHelper.Amend()       → git commit --amend --no-edit
```

The `--all` flag (not `.`) ensures `.gitignore` un-exclusion patterns like `.ff/*` → `!.ff/forgefix_ledger.json` are correctly handled.

### Side-Effect Folding

`ff commit --ai` writes metadata (spec status → review, ledger binding) to disk **before** the commit (pre-commit write in `runCommit`), then `AutoStageAndCommit` includes it. After `UpdateLedgerAfterCommit` adds the linked commit hash, `amendLastCommit` (using `git add --all`) folds that into the commit too. Result: zero untracked side effects after `ff commit`.

## Config Reference

```yaml
# <project>_ff.yaml
global_timeout_seconds: 120
failure_decay_seconds: 30
auto_issue_management: true
git_passthrough: true

github:
  owner: "my-org"
  repo: "myapp"
  token: "ghp_..."
  base_url: "https://api.github.com"

pipelines:
  - id: myapp
    name: "[myapp]"
    type: go_mod
    panel_color: blue
    timeout_seconds: 300

languages:
  go_mod:
    root_anchor: go.mod
    test_command: go test -json ./...
```

ForgeFix auto-detects from anchor files: `go.mod`, `pubspec.yaml`, `package.json`, `Cargo.toml`, `Gemfile`, `pyproject.toml`, `pom.xml`, `build.gradle`, `mix.exs`, `composer.json`, `CMakeLists.txt`, `Makefile`, `Rakefile`, `Package.swift`, `deno.json`, `bun.lock`, and more.

## Integration with Agent Skills

| Skill | Integration |
|-------|-------------|
| `api-and-interface-design` | Spec's Implementation section documents API contracts |
| `test-driven-development` | Write tests first, then `ff commit --ai` binds them to the spec |
| `code-review-and-quality` | `ff sync --ai` promotes review→ship after approval |
| `shipping-and-launch` | `ff ship --ai` is the final gate |
| `deprecation-and-migration` | Spec type `refactor` for migration specs |
| `documentation-and-adrs` | Archived specs serve as ADR-style documentation |
| `planning-and-task-breakdown` | Break work into specs with acceptance criteria |

## Common Rationalizations

| Rationalization | Reality |
| --- | --- |
| "I'll use `git commit`, it's faster" | `ff commit --ai` auto-detects spec, formats message, binds to ledger, folds metadata. Raw `git commit` breaks traceability. |
| "Raw `git` is fine for simple commands" | Every `git log`/`diff`/`status` works identically through `ff` passthrough. No reason to use raw git. |
| "I'll write the spec when the feature is done" | A spec written after the fact is documentation, not planning. Write it first. |
| "I'll commit when the feature is done" | One giant commit is impossible to review, debug, or revert. Commit each slice with `ff commit --ai`. |
| "The spec doesn't need a status update" | With `--ai` the status is automatic. Without `--ai`, the Shipping Gate enforces it. |
| "I'll sync later" | Stale remote issues defeat the purpose. Sync after every commit. |
| "I don't need `ff`, git works fine" | `ff` adds spec → commit → issue → ship traceability that raw git can't. |
| "The message doesn't matter" | `ff commit` auto-formats, so there's no excuse for bad messages. |
| "I'll squash it all later" | Squashing destroys the spec-to-commit binding in the ledger. Prefer clean incremental commits. |
| "I need to edit the ledger manually" | Never edit `.ff/forgefix_ledger.json`. Use `ff spec --delete`, `ff sync`, `ff ship`. |
| "Branches add overhead" | Short-lived branches paired with specs provide full traceability for free. |
| "I'll split this change later" | Large changes are harder to review. Split into multiple specs before implementing. |
| "I don't need a `.gitignore`" | Until `.env` with production secrets gets committed. Set it up immediately. |
| "It's just a small fix, bump the patch" | Check what consumers can observe. A behavior change they relied on is a major, whatever the diff size. |
| "The changelog is just the commit log" | Commits are for you; the changelog is for consumers. |
| "We'll write the changelog at release time" | By then the impact is reconstructed from memory. Write it with the change. |

## Red Flags

### Agent (AI Mode) Red Flags
- Agent runs `ff sync`, `ff ship`, or `ff archive` — these are HUMAN-ONLY
- Agent creates a spec without running `ff specs --search` for duplicates first
- Agent commits without outputting a pre-commit change summary first
- Agent starts work on spec Y while spec X has uncommitted changes (violates one-spec-at-a-time)
- Agent creates a spec with a title closely matching an existing or archived spec
- Agent uses raw `git` commands
- Any use of raw `git` commands (all go through `ff` passthrough)
- `ff commit` without `--ai` or `--spec` (interactive prompts fail in agent mode)
- Specs stuck in `review` (run `ff sync --ai` to auto-promote)
- `ff sync --ai` not run before `ff ship --ai` (misses the auto-promote)
- Untracked side effects after `ff commit --ai` (pre-commit write + amend should fold all metadata)
- Manual editing of `.ff/forgefix_ledger.json`
- `ff archive` never run — accumulating resolved spec files
- The ledger has specs that don't exist as files (orphans — use `ff spec --delete <id>`)
- Multiple specs with similar titles (duplicate detection is on by default)
- Ship gate rejection because specs were never promoted to `ship`
- Large uncommitted changes accumulating
- Formatting changes mixed with behavior changes
- No `.gitignore` in the project
- Long-lived branches that diverge significantly from main
- Force-pushing to shared branches
- A breaking change shipped under a minor or patch version bump
- A user-facing release with no changelog entry

## Agent Verification Checklist

For every agent change cycle (exactly 5 steps):

1. [ ] Pre-spec duplicate check run (`ff specs --search` showed no match)
2. [ ] A spec exists (`ff specs` shows it) before code is written
3. [ ] Only one spec was active during this work cycle (no cross-spec leakage)
4. [ ] Pre-commit change summary was output *before* the `ff commit --ai` call
5. [ ] Acceptance criteria from the spec were reviewed at commit time
6. [ ] `ff commit --ai <msg>` auto-detects and commits with `[SPEC-XXX]`
7. [ ] Each commit is bound to a spec (`ff specs` shows linked commits)
8. [ ] Every `git` command replaced with `ff` passthrough
9. [ ] `ff status --short` shows clean tree after each ff command
10. [ ] All tests pass (`ff --ai` produces JSON pass)
11. [ ] Build compiles (`go build ./...` or equivalent)
12. [ ] No secrets in the diff
13. [ ] `.gitignore` covers standard exclusions
14. [ ] No formatting-only changes mixed with behavior changes

## Human Verification Checklist

For human-operated sync/ship/archive operations:

1. [ ] `ff sync --ai` promotes `review`→`ship` without prompts
2. [ ] `ff ship --ai` ships, pushes, and sets to `closed`
3. [ ] `ff archive --ai` archives closed specs
4. [ ] `ff sync` completes without errors
5. [ ] Archived specs are removed from active view (`ff archive --ai`)
6. [ ] Spec file's `resolution` field is filled in for closed specs
