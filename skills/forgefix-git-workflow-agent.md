---
name: forgefix-git-workflow
description: "Use when making code changes, managing specs, binding commits to issues, or shipping releases in a ForgeFix-enabled project. Replaces all raw git operations with ff spec → ff commit → ff sync → ff ship → ff archive lifecycle. All git commands go through ff git passthrough."
---

# ForgeFix Spec-Driven Git Workflow

## Overview

ForgeFix (`ff`) replaces ad-hoc git workflow with a **spec-driven development lifecycle**. Every change is traceable to a spec, every spec links to a remote issue, and no code ships without a paper trail.

```
Full lifecycle:
  ff spec --ai <title>             → Create a spec (issue contract)
  ff spec --ai --type bug <title>  → Create bug spec with --type enforcement
  ff spec --ai --root-cause "..."  → Document root cause at creation time
  Implement code                   → Write against the spec
  ff commit --ai <msg>             → Auto-detect spec, commit with binding, set "review"
  ff commit --ai --body "..."      → Commit with multi-line message body
  ff sync --ai                     → Sync to remote issue tracker, auto-promote to "ship"
  ff ship --ai                     → Push, close issues, set "closed"
  ff archive --ai                  → Archive closed specs into timestamped document
```

**Never use raw `git` commands.** Every git operation goes through `ff`'s git passthrough: `ff log`, `ff diff`, `ff rebase`, `ff reset`, `ff stash`, `ff tag`, `ff worktree`, `ff bisect`, `ff blame`, etc. Raw `git commit` is forbidden; always use `ff commit`.

## When to Use

Every code change in a ForgeFix-enabled project.

## Core Principles

### 1. Spec First, Code Second

```
ff spec --ai "my feature" → creates spec with SPEC-ID, registers as "draft"
```

### 2. Auto Lifecycle (No Manual Status Editing)

| Status | Set By | Description |
|--------|--------|-------------|
| `draft` | `ff spec --ai` | Newly created |
| `review` | `ff commit --ai` | Committed, ready for review |
| `ship` | `ff sync --ai` | Auto-promoted from review |
| `closed` | `ff ship --ai` | Shipped, issue closed |
| archived | `ff archive --ai` | Moved to archive document |

### 3. Commit with Auto-Detect

`ff commit --ai` auto-detects the most-recently-modified active spec:

```
ff commit --ai "add user authentication"              → auto-detect spec
ff commit --ai --spec SPEC-X "fix bug"                → explicit spec
ff commit --ai --type bug "fix null pointer"          → with metadata
ff commit --ai --body "multi-line\nbody" "msg body"   → with body for multi-line messages
ff commit --ai captures git diff in the spec's resolution: field
ff commit --ai warns if bug spec missing root_cause or spec missing version
```

Never use `git commit` — even infrastructure changes.

### 4. The Full Cycle

```
ff spec --ai <title>  → creates spec at "draft"
implement code
ff commit --ai <msg>  → auto-detects, commits bound, sets "review"
ff sync --ai          → syncs issues, promotes review→"ship"
ff ship --ai          → pushes, drains housekeeping, sets "closed"
ff archive --ai       → archives closed specs
```

## Git Operations Through ff Passthrough

Never run `git` directly. Every command works through `ff`:

```
ff log --oneline -5     ff diff --cached         ff status --short
ff blame src/file.ts    ff rebase main           ff reset HEAD~1
ff stash list           ff bisect start/end      ff tag -a v1.0
ff push origin main     ff worktree add ../path  ff checkout -b feature/x
ff merge feature-x      ff branch                ff git log --oneline
```

Exit codes are preserved for CI.

## Verification Checklist

1. [ ] `ff spec --ai <title>` creates a spec
2. [ ] `ff commit --ai <msg>` auto-detects and commits with `[SPEC-XXX]`
3. [ ] `ff sync --ai` promotes `review`→`ship`
4. [ ] `ff ship --ai` ships and sets to `closed`
5. [ ] `ff archive --ai` archives closed specs
6. [ ] Every `git` command replaced with `ff` passthrough
7. [ ] `ff status --short` shows clean tree after each ff command
8. [ ] `ff commit --ai --body "..."` creates multi-line commit with body
9. [ ] `ff spec --ai --type bug "title"` rejects if --type is missing

## Red Flags

- Any raw `git` command (all go through `ff`)
- `ff commit` without `--ai` or `--spec` (prompts fail in agent mode)
- Specs stuck in `review` (run `ff sync --ai`)
- Manual editing of `.ff/forgefix_ledger.json`
- Accumulating unarchived closed specs (run `ff archive --ai`)
- Skipping `--type` in `ff spec --ai` will produce an error in agent mode
