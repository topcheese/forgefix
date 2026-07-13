---
spec_id: "SPEC-1783931981"
status: review
repo_issue: ""
type: feature
version: "0.9.5"
root_cause: ""
resolution: ""
linked_commits: ["4f1ab65"]
---
Add an `ff specs --search <query>` command that queries the DB for similar spec titles/IDs to detect duplicates before creating new specs. Also update the forgefix-git-workflow skill to add a mandatory pre-spec search step and other missing workflow gates.

## Objective

The 3-step agent workflow (`ff spec --ai` → `ff test --ai` → `ff commit --ai`) is clean but has several missing gates that allow duplicate specs, overlapping scope, and process drift. This spec adds:

1. A `--search` flag on `ff specs` that queries forgefix.db for similar specs
2. A mandatory pre-spec duplicate check step in the agent workflow

## Requirements

### FR-001: ff specs --search flag
- `ff specs --search "query text"` searches forgefix.db `specs` table WHERE title or spec_id LIKE '%query%'
- Includes both active and archived specs in results
- Returns spec_id, title, status, and linked repo issue number
- Limits to top 10 results by default (configurable via --limit)
- If no DB connection, falls back to grepping spec file names in specs/ directory
- Searches ALL statuses — during cleanup we need to find everything

### FR-002: Pre-spec duplicate check (skill workflow)

## Implementation

### Step 1: DB query helper in db.go

Add a `SearchSpecs(query string, limit int)` method on `*DB` that:

```sql
SELECT spec_id, title, status, repo_issue_id
FROM specs
WHERE title LIKE '%' || ? || '%' OR spec_id LIKE '%' || ? || '%'
ORDER BY updated_at DESC
LIMIT ?
```

### Step 2: CLI flag in cmd_list.go

Add `--search` flag to the specs command (`cmd_list.go` / `cmd_specs.go`). The handler:
1. Strips non-alphanumeric from query for safety
2. Opens DB and calls `SearchSpecs()`
3. Falls back to `grep -ril` on `specs/*.md` if DB is unavailable
4. Prints results table with columns: Spec ID, Title, Status, Linked Issue

### Step 3: Skill file update

In `skills/forgefix-git-workflow.md`:

**Agent Workflow (4 steps instead of 3):**
```
AGENT WORKFLOW (exactly four steps):
  1. ff specs --search "<title>"   → Check for duplicates, verify gap
  2. ff spec --ai <title>          → Create a spec (issue contract)
  3. ff test --ai                  → Write tests, implement, run tests
  4. ff commit --ai <msg>          → Only after tests pass; verify changelog
```

**Agent Verification Checklist update:**
- Add: "[ ] Pre-spec duplicate check run (`ff specs --search` showed no match)"

**Red Flags update:**
- Add: "Agent creates a spec without running `ff specs --search` first"
- Add: "Spec created with a title that closely matches an existing spec"
- Add: "Multiple specs with nearly identical descriptions or objectives"

## Acceptance Criteria

- [ ] `ff specs --search "query"` returns matching results from DB
- [ ] `ff specs --search "query"` falls back to file grep when DB unavailable
- [ ] Skill file agent workflow updated to 4 steps with pre-spec search
- [ ] Pre-spec search step added to Agent Verification Checklist
- [ ] Agent red flags updated for duplicate-creation violations
- [ ] All engine tests pass
- [ ] `go build ./...` compiles
