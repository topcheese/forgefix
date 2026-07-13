---
spec_id: "SPEC-1783933843"
status: review
repo_issue: ""
type: feature
version: "0.9.0"
root_cause: ""
resolution: ""
linked_commits: ["146acde"]
---
Harden the agent workflow by enforcing pre-commit discipline, one-spec-at-a-time rules, acceptance criteria review at commit, and optional remote issue creation at spec creation time. Updates the forgefix-git-workflow skill file and adds CLI enforcement where feasible.

## Objective

The 4-step workflow (spec search → spec → test → commit) is solid but still allows agents to:
- Commit without a structured summary of what changed
- Start work on a second spec before committing the first
- Commit without reviewing whether acceptance criteria were met
- Create specs with no link to a remote issue (repo_issue stays "" indefinitely)

This spec closes all four gaps through a combination of skill enforcement and optional CLI features.

## Requirements

### FR-001: Pre-commit change summary requirement
The agent **must** output a structured change summary *before* calling `ff commit --ai`, not after. This is enforced in the skill file as a mandatory pre-commit step. The summary template:

```
SPEC: SPEC-XXXXXXXXXX

CHANGES MADE:
- <file>: <what changed>

THINGS I DIDN'T TOUCH (intentionally):
- <file scope>: <why>

VERIFICATION:
- Tests: PASS (X/Y)
```

`ff commit --ai` itself should not enforce this (it's a git tool, not a QA gate) — the skill file is the source of truth here.

### FR-002: One-spec-at-a-time rule
The skill file adds a hard rule: the agent commits the current spec's work before starting work related to another spec. Concretely:

- After `ff spec --ai "X"` → implement → `ff commit --ai "X"` is complete
- Only then: `ff spec --ai "Y"` → new cycle
- If the agent discovers work for spec Y while implementing spec X, it must either:
  a) Commit X first, then create Y, then implement Y
  b) Include the Y work as part of X if it's a true dependency (but note it in the commit message)

The `ff commit --ai` auto-detect heuristic already picks the most recently modified spec, so this rule prevents the auto-detect from targeting the wrong spec.

### FR-003: Spec acceptance criteria review at commit time (see also)  
`ff commit --ai` should, when run with `--ai`, print the spec's acceptance criteria checklist before prompting for the commit message. This is an FYI display — the agent reads the spec's `## Acceptance Criteria` section, sees what's unchecked, and can note partial progress or confirm completion in the message.

This is NOT a gate — it's a reminder. The agent can proceed regardless.

### FR-004: Optional remote issue creation at spec creation time
`ff spec --ai` optionally creates a draft remote issue when GitHub/Gitea credentials are configured in `_ff.yaml`. Adds a `--create-issue` flag (or makes it default when `github.*` is configured):

- Calls the GitHub Issues API to create a draft issue with the spec title and objective
- Sets `repo_issue` in the spec frontmatter immediately
- The issue is created in "open" state with a label like `type/<spec-type>` and `status/draft`
- `ff sync` later updates it during promotion (review→ship→closed)
- If `auto_issue_management` is false in config, the flag is ignored with a warning

This eliminates the "unlinked specs" problem that happens when `auto_issue_management` is off and `ff sync` is human-only.

### FR-005: Skill file hardening
Update `skills/forgefix-git-workflow.md`:

**Agent Workflow (5 steps):**
```
1. ff specs --search "<title>"   → Check for duplicates (from SPEC-1783931981)
2. ff spec --ai <title>          → Create spec (optionally creates remote issue)
3. [Pre-commit summary]          → Output structured change summary
4. ff test --ai                  → Write tests, implement, run tests
5. ff commit --ai <msg>          → Auto-detects spec; shows acceptance criteria
```

**Agent Verification Checklist additions:**
- [ ] Pre-spec duplicate check run (`ff specs --search` showed no match)
- [ ] Pre-commit change summary was output *before* the `ff commit --ai` call
- [ ] Only one spec was active during this work cycle (no cross-spec leakage)
- [ ] Acceptance criteria were reviewed at commit time

**Red Flags additions:**
- Agent commits without outputting a pre-commit change summary first
- Agent starts work on spec Y while spec X has uncommitted changes
- Spec created without a remote issue link (when GitHub is configured)
- Acceptance criteria not referenced in commit message or summary

## Implementation

### Step 1: Skill file update (immediate, no code)
Edit `skills/forgefix-git-workflow.md` with the workflow, checklist, and red flags changes above. This is a documentation change only.

### Step 2: ff commit --ai acceptance criteria display (code)
In `cmd_commit.go` or `runCommit`, before the commit message prompt:
1. Load the spec file for the auto-detected spec
2. Parse the markdown body for a `## Acceptance Criteria` section
3. Print any `- [ ]` checklist items found
4. Print a reminder: "Review the unchecked items above before committing"

This only runs with `--ai` flag. No structural enforcement — just visibility.

### Step 3: ff spec --ai remote issue creation (code)
In `cmd_spec.go` or `runSpec`:
1. If `--create-issue` flag set OR github config is present, load the IssueCoordinator
2. Call API to create a draft issue with the spec title and body
3. Write the returned issue number to the spec file frontmatter's `repo_issue` field
4. Silently skip if API call fails (non-fatal — ff sync will catch it later)
5. Respect `auto_issue_management: false` by skipping

## Acceptance Criteria

- [ ] Skill file updated with 5-step agent workflow
- [ ] Pre-commit summary requirement enforced in skill file (output before ff commit)
- [ ] One-spec-at-a-time rule documented in skill file
- [ ] Red flags for cross-spec work and missing pre-commit summary added
- [ ] `ff commit --ai` displays acceptance criteria from the spec before prompting
- [ ] `ff spec --ai --create-issue` creates a draft GitHub/Gitea issue and sets repo_issue
- [ ] All engine tests pass
- [ ] `go build ./...` compiles
