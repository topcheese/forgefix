---
spec_id: "SPEC-1784078880"
status: review
repo_issue: ""
type: bug
version: "0.9.5"
root_cause: "Spec file frontmatter fields and body content are silently discarded when crossing layers: DB INSERT hardcodes '' for body/root_cause/resolution/version; SpecEntry struct lacks Body field; SyncFromSpecsDir parses body then discards it; ff commit --ai generates single-line messages only. No mechanism exists to capture type/version/root_cause at spec creation time through the --ai command path, so the template defaults (type:feature, version:v0.8.0) persist unless manually edited."
resolution: ""
linked_commits: ["63f2e4d", "e849122"]
---
# Fix Spec Body Persistence Gap Between Local Files, SQLite DB, And Commit Messages

## Objective

Close the data-fragmentation gap between layers that each hold different fragments of a spec's content — and prevent agents from silently publishing incomplete spec metadata through the `--ai` command path.

The core problem: a spec file on disk has rich structured data (frontmatter fields + body prose), but every time it crosses a layer boundary (file → ledger → DB → commit → remote issue), fields are silently dropped or replaced with template defaults.

| Layer | Holds body? | Holds frontmatter fields? | Holds commit detail? |
|---|---|---|---|
| Local `.md` file | ✓ (if filled in) | ✓ (spec_id/type/version/root_cause/resolution) | ✗ |
| SQLite `specs` table | ✗ (always `''`) | ✗ (version/root_cause/resolution always `''`) | ✗ |
| `SpecEntry` (ledger) | ✗ (no Body field) | Partial (missing root_cause/resolution/version) | ✗ |
| Remote Gitea issue | ✓ (re-read from file at sync) | Partial (frontmatter → generated body) | ✗ |
| Commit message | ✗ (single-line only) | ✗ | ✗ |
| Resolution field | ✗ | ✗ | ✗ |

At the end of this work, every field in a spec file is faithfully reflected in the DB, commit messages carry a body describing the work, and the resolution field captures the actual diff so there is a permanent record of what changed.

## Requirements

### FR-1 — Populate ALL DB columns from spec file content, not hardcoded empties

The following DB columns in `specs` are **always written as `''`** regardless of what the spec file contains:

| Column | Current value | Source in spec file |
|---|---|---|
| `body` | `''` | Everything after the `---` frontmatter delimiter |
| `root_cause` | `''` | `root_cause:` YAML field in frontmatter |
| `resolution` | `''` | `resolution:` YAML field in frontmatter |
| `version` | `''` | `version:` YAML field in frontmatter |

All four must be populated from their actual spec file values during INSERT and kept in sync on UPDATE.

### FR-2 — Add Body, RootCause, Resolution, Version fields to SpecEntry struct

The `SpecEntry` struct in `engine/ledger.go` currently carries only: `SpecID`, `RepoIssueID`, `Status`, `LinkedCommits`, `Type`.

Add: `Body string`, `RootCause string`, `Resolution string`, `Version string`.

### FR-3 — SyncFromSpecsDir must propagate all fields

`SyncFromSpecsDir` parses each spec file and discards everything except `specID`, `status`, `specType`, and `title`. It must now also capture:
- The full body text (`strings.TrimSpace(parts[2])`)
- `root_cause` from frontmatter
- `resolution` from frontmatter
- `version` from frontmatter
- `repo_issue` from frontmatter (it currently defaults to 0)

### FR-4 — Add commit message body support to ff commit --ai

`ff commit --ai` generates `feat: [SPEC-X] <cleaned_msg>` — single line, no body. Implementation detail lives only in the diff and is lost to history search.

**Required mechanism:** support a `--body <text>` flag. When provided, the commit message becomes:
```
feat: [SPEC-X] <subject>

<body>
```

The `--body` flag passes through `ExtractMessageFromArgs` without being skipped, then gets appended after a blank line when `--ai` mode constructs the final message.

### FR-5 — Capture the git diff in the resolution field on commit

When `ff commit --ai` runs, the git diff contains the most complete and accurate record of what was changed. After the commit is made, capture the diff (via `git diff HEAD~1..HEAD` or the staged diff) and write it into the spec file's `resolution:` frontmatter field.

This means:
- `resolution` is no longer a manually-entered summary — it's auto-populated from the real diff
- The resolution always matches what was actually shipped
- The DB stores the diff text so queries against `specs` show the actual changes

### FR-6 — ff spec --ai must enforce structured frontmatter fields at creation time (not optional)

**The problem:** `ff spec --ai <title> <body>` creates a spec with the template defaults (`type: feature, version: v0.8.0`). Agents have no way to set these at creation time, so every AI-created spec silently carries wrong metadata unless manually edited after the fact. A human may have written a detailed spec with `type: bug, version: 0.9.5` in the frontmatter, but when the agent runs `ff commit --ai`, it never reads or validates those fields — it bypasses them entirely.

**The fix — two enforcement layers:**

#### Layer 1: `--type` must be required (not optional) in --ai mode

When `ff spec --ai` is invoked without `--type`, the command MUST fail with an error:

```
$ ff spec --ai "Fix crash on null input" "Root cause: nil pointer..."
Error: --type is required in --ai mode. Valid values: feature, bug, refactor
```

Valid types enforced server-side: `feature`, `bug`, `refactor`. Anything else is rejected.

This means every AI-created spec has an explicit, validated type from birth — no agent can skip it.

#### Layer 2: ff commit --ai must read and verify spec metadata before committing

Before `ff commit --ai` constructs the commit message, it reads the spec file, parses its frontmatter, and validates that critical fields are present and sensible:

| Field | Check | Action on failure |
|---|---|---|
| `type` | Must be one of: feature, bug, refactor | Warn + require `--force` to proceed |
| `version` | Must match the project's current version | Warn on mismatch (e.g., spec says 0.8.0 but project is 0.9.5) |
| `root_cause` | Must be non-empty if type=bug | Warn (a bug without a documented root cause is incomplete) |

These are soft gates — they warn but don't block — because the user may have intentionally left a field empty. The point is to surface the mismatch so the agent/human can fix it, not to let it pass silently.

#### Layer 3: Suggested flags at creation time

```
ff spec --ai "Fix crash" "Nil pointer..." --type bug --version 0.9.5 --root-cause "nil pointer dereference in parser"
```

Supported flags: `--type` (required), `--version` (optional, defaults to project version), `--root-cause` (optional, recommended for bugs).

### SC-1 — No regressions in existing tests

All existing tests must pass: `go test ./... -count=1`. No breaking changes to the parse-spec-file or sync-spec flows for existing specs (even if they have empty frontmatter fields, they should still round-trip safely).

## Implementation Approach

This is broken into 7 ordered steps. Steps 1-3 fix the DB/ledger data loss (FR-1 through FR-3). Step 4 fixes the commit message body (FR-4). Step 5 captures diff in resolution (FR-5). Step 6 enforces type at creation time (FR-6). Step 7 is a cleanup audit.

Steps 1-3 must be done before Step 5 (which needs Resolution stored in SpecEntry). Step 4 is independent. Step 6 is independent.

### Step 1 — Add missing fields to SpecEntry struct (FR-2)

**File:** `engine/ledger.go` ~line 210

**What changes:**
```go
type SpecEntry struct {
    SpecID        string   `json:"spec_id"`
    RepoIssueID   int      `json:"repo_issue_id"`
    Status        string   `json:"status"`
    LinkedCommits []string `json:"linked_commits"`
    Type          string   `json:"type,omitempty"`
    Body          string   `json:"body,omitempty"`         // NEW — full body text after frontmatter
    RootCause     string   `json:"root_cause,omitempty"`   // NEW — from frontmatter root_cause field
    Resolution    string   `json:"resolution,omitempty"`   // NEW — from frontmatter resolution field (will later be auto-populated from diff)
    Version       string   `json:"version,omitempty"`      // NEW — from frontmatter version field
}
```

**How it works:** These are simple `string` fields on the in-memory struct. All existing code that accesses `SpecEntry` continues to compile because the new fields are optional (zero-value `""`). The `omitempty` JSON tag means serialization doesn't change when fields are empty.

**Edge case — existing in-memory entries:** Code that creates `SpecEntry{}` literals (e.g. in `SyncFromSpecsDir`'s new-entry path) will get zero-value `""` for the new fields. This is safe — empty is the correct value when no body/root_cause/etc. has been parsed yet (Step 2 fixes the parsing, so after Step 2 the new fields will have real values).

### Step 2 — SyncFromSpecsDir propagates all fields (FR-3)

**File:** `engine/ledger.go` ~lines 553-632 (the `SyncFromSpecsDir` function)

**What changes — the frontmatter parsing loop:** Currently the loop extracts `specID`, `status`, and `specType` from frontmatter lines. Add extraction for:

```
version:     → entry.Version = value (trim quotes)
root_cause:  → entry.RootCause = value (trim quotes)
resolution:  → entry.Resolution = value (trim quotes)
repo_issue:  → entry.RepoIssueID = parsed int (0 if unset)
```

**What changes — the body text:** The function already does `body := strings.TrimSpace(parts[2])` at ~line 584 but only uses it to extract the title heading. After the heading extraction, pass the full body into the entry:

- **Update path** (~line 610): add `existing.Body = body` alongside the existing status/title/type updates
- **Create path** (~line 622): add `Body: body` to the `SpecEntry{...}` literal

**How the body is used today vs after:** Currently `body` is parsed to find the `# Title` heading and then thrown away. After Step 2, `body` is stored in the entry so it survives into the DB INSERT (Step 3) and any future ledger queries.

**Edge case — spec has frontmatter but no body text (template-only):** `strings.TrimSpace(parts[2])` returns `""`. The entry gets `Body: ""`. This is correct — an empty body means no prose content, and downstream consumers (`effectiveSpecBody` in `spec_manager.go`) already handle this via `isTemplateBody()`.

### Step 3 — Fix DB INSERT to use real values (FR-1)

**File:** `engine/ledger.go` ~lines 174-178 (within `SaveToDB`)

**Current (broken):**
```go
INSERT INTO specs (spec_id, title, status, type, version, repo_issue_id, root_cause, resolution, body, updated_at)
VALUES (?, ?, ?, ?, '', ?, '', '', '', datetime('now'))
//     spec_id ^  ^title  ^status ^type  ^^^^  ^repo   ^^^^  ^^^^  ^^^^  ^timestamp
//     version ─────┘              repo_issue ─┘  root_cause ─┘  resolution ─┘  body ─┘
//     ALL FOUR hardcoded to ''
```

**After fix:**
```go
INSERT INTO specs (spec_id, title, status, type, version, repo_issue_id, root_cause, resolution, body, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
```

**Parameter order becomes:** `specID, entry.SpecID, entry.Status, entry.Type, entry.Version, entry.RepoIssueID, entry.RootCause, entry.Resolution, entry.Body`

**Why this is safe after Step 2:** Before Step 2, `entry.Version`/`entry.RootCause`/`entry.Resolution`/`entry.Body` would be `""` for ledger entries that were synced before Step 2 existed. But Step 2 ensures `SyncFromSpecsDir` populates them, and `RunBackgroundSync` calls `SyncFromSpecsDir` before any DB write (see `sync.go` line ~322). So by the time `SaveToDB` runs, the entries have real values.

**Edge case — existing DB rows with '' in body/root_cause/resolution/version:** This change only affects NEW INSERT and future UPDATE operations. Existing rows remain as-is. The Step 2 reconciliation will fill in values on the next sync cycle, but only for specs whose files still exist with frontmatter intact.

### Step 4 — Add --body flag to ff commit --ai (FR-4)

**Files:** `engine/cmd_commit.go`, `engine/command_dispatcher.go` (ParseFlags)

**How ParseFlags learns about --body:**

`ParseFlags` currently recognizes a fixed set of flags. Add `--body` to the recognized set. Unlike `-m`/`--message` (which are consumed by `ExtractMessageFromArgs` and stripped from the message), `--body` is preserved as a separate field in the parsed flags so the commit handler can access it.

**How the commit handler uses it (~line 126 of cmd_commit.go):**

After the existing message construction:
```go
commitMsg = strings.TrimSpace(fmt.Sprintf("feat: [%s] %s", specID, cleaned))

// NEW: append body if provided
if flags.Body != "" {
    commitMsg = fmt.Sprintf("%s\n\n%s", commitMsg, flags.Body)
}
```

**How git handles it:** `git commit -m <msg>` natively handles embedded `\n\n`. No change needed in `git_helper.go`.

**Edge case — body contains markdown or newlines:** The body is a plain string. If it contains `\n` escapes, they should be preserved. If it contains actual newlines from a multi-line shell argument, git handles them correctly. If the body is excessively long (>64KB), git may truncate — document this as a known limit but don't enforce in code (git itself enforces it).

**Why not use multiple `-m` flags (matching git convention)?:** `ExtractMessageFromArgs` currently skips ALL `-m` values (they're consumed by flag detection). Changing this would break existing callers that pass `-m` as the message. A separate `--body` flag is cleaner and doesn't risk breaking `ff commit` messages that happen to contain the string `-m`.

### Step 5 — Capture git diff in resolution field on commit (FR-5)

**File:** `engine/cmd_commit.go` ~lines 168-171 (after `AutoStageAndCommit` succeeds)

**The mechanism:**

1. After `commitHash, err := AutoStageAndCommit(gitRoot, commitMsg)` succeeds:
2. Run `exec.Command("git", "-C", gitRoot, "diff", "HEAD~1..HEAD")` to capture the diff
3. Truncate if > 10KB (the resolution field should be descriptive, not a full dump for massive changes)
4. Write the diff text into the spec file's `resolution:` frontmatter using `updateSpecFileField` or direct YAML manipulation
5. Write it into the ledger entry's `Resolution` field
6. The next DB INSERT/UPDATE via `SaveToDB` will persist it (because Step 3 already fixed the hardcoded `''`)

**What the diff looks like stored:**
```yaml
resolution: |
  diff --git a/engine/cmd_commit.go b/engine/cmd_commit.go
  index abc123..def456 100644
  --- a/engine/cmd_commit.go
  +++ b/engine/cmd_commit.go
  @@ -123,6 +123,9 @@ func runCommit(...) {
  +    // new code here
```

**Why the resolution field specifically:** The `resolution:` field in frontmatter is the natural place to record what was actually done. Today it's manually entered (and often left empty). Auto-populating it from the diff means:
- It's always accurate (it's the actual diff, not a summary)
- It's verifiable post-hoc
- It carries over to the remote issue when `ff sync` runs `effectiveSpecBody()`

**Edge case — first commit on a branch with no parent:** `git diff HEAD~1..HEAD` fails when `HEAD~1` doesn't exist. Fall back to `git diff --cached` (staged vs HEAD) in that case.

**Edge case — resolution truncation:** If the diff is very large (>10KB), truncate with a notice: `"... [truncated at 10KB]"`. The full diff is always available via `git log -p`.

### Step 6 — Enforce --type in ff spec --ai and add supporting flags (FR-6)

**Files:** `engine/cmd_spec.go`, `engine/command_dispatcher.go` (ParseFlags)

**The enforcement — three-tier approach:**

**Tier 1 — --type is required in --ai mode (HARD GATE):**

In `handleSpec`, when `flags.AIMode` is true, check `flags.SpecType`. If empty:
```go
if flags.AIMode && flags.SpecType == "" {
    return fmt.Errorf("--type is required in --ai mode: valid values are feature, bug, refactor")
}
if flags.AIMode && !isValidSpecType(flags.SpecType) {
    return fmt.Errorf("invalid --type %q: valid values are feature, bug, refactor", flags.SpecType)
}
```

`isValidSpecType` is a simple set check against `{"feature", "bug", "refactor"}`.

**Tier 2 — ff commit --ai reads and validates spec metadata (SOFT GATE):**

In `runCommit`, after the spec file is parsed (which already happens), add validation:
```go
spec, _ := parseSpecFile(specFile)  // already exists in the flow
if spec.Type == "" || spec.Type == "feature" {
    // Blind default — warn if no explicit type
    fmt.Fprintf(os.Stderr, "warning: spec %s has no explicit type (defaults to 'feature')\n", specID)
}
if spec.Version != "" && spec.Version != runtimeVersion {
    fmt.Fprintf(os.Stderr, "warning: spec %s has version %q but project is %q\n", specID, spec.Version, runtimeVersion)
}
if spec.Type == "bug" && spec.RootCause == "" {
    fmt.Fprintf(os.Stderr, "warning: bug spec %s has no root_cause documented\n", specID)
}
```

These are stderr warnings — they don't block the commit — but they surface silently-mismatched metadata that would otherwise go unnoticed.

**Tier 3 — template uses actual values:**

When `--ai` mode writes the spec template, instead of hardcoding `type: feature` and `version: "v0.8.0"` from the template file, it uses the flag values:
```yaml
---
type: bug                  # from --type flag
version: "0.9.5"           # from --version flag (or auto-detect project version)
root_cause: "nil pointer"  # from --root-cause flag
---
```

**How ParseFlags learns about these flags:**

Add `--type`, `--version`, `--root-cause` to the recognized flag set in `command_dispatcher.go`'s `ParseFlags` function. Unlike `--body` (which is consumed by `ff commit`), these are consumed by `ff spec`.

**Edge case — --type is provided but invalid:** Rejected immediately with a clear error listing valid types.

**Edge case — --version is provided but doesn't match project:** Allowed (not blocked) but warned about in Tier 2.

**Edge case -- non-AI (interactive) mode:** `--type` is optional in interactive mode because the user can see and edit the template directly. The template's `type: feature` default remains.

### Step 7 — Audit all DB column writes for remaining hardcoded empties

**Files:** `engine/ledger.go`, `engine/db.go` (search for all INSERT/UPDATE on `specs` table)

**Procedure:**
1. Search for all SQL statements that reference the `specs` table: `grep -n "specs.*INSERT\|specs.*UPDATE\|specs.*VALUES" engine/*.go`
2. For each one, verify that every column's value expression is a parameter (`?`) bound from a real field — not a literal `''` or `0`.
3. If any hardcoded placeholders remain, fix them to use the corresponding `SpecEntry` field.

**Known INSERT locations:**
- `engine/ledger.go` ~line 176 — the `SaveToDB` INSERT (fixed in Step 3)
- Any other INSERT/UPDATE in `db.go` or `ledger.go` that writes to the `specs` table

**Verification that Step 7 passed:** A grep for `'',` in VALUES clauses against `specs` should return zero matches.

## Acceptance Criteria

Each criterion maps directly to a requirement. They are ordered so that earlier criteria can be verified independently of later ones.

| # | Criterion | Verifies | How to test |
|---|---|---|---|
| **C1** | `SpecEntry` struct has `Body`, `RootCause`, `Resolution`, `Version` fields | FR-2 | `grep -A10 'type SpecEntry struct' engine/ledger.go` — fields exist |
| **C2** | `SyncFromSpecsDir` populates Body/RootCause/Resolution/Version/RepoIssueID from spec file | FR-3 | Unit test: create a spec file with all fields → call `SyncFromSpecsDir` → verify `SpecEntry` has matching values |
| **C3** | `sqlite3 .ff/forgefix.db "SELECT body FROM specs WHERE body != ''"` returns at least one row | FR-1 | After `ff sync --ai`, the body text from the spec file exists in the DB |
| **C4** | DB `root_cause`, `resolution`, `version` contain spec file frontmatter values, not `''` | FR-1 | `sqlite3 ... "SELECT root_cause, resolution, version FROM specs WHERE ..."` |
| **C5** | `ff commit --ai "subject" --body "detailed body text"` produces a commit with subject+body | FR-4 | `git log -1 --format='%B'` shows both lines separated by blank line |
| **C6** | After `ff commit --ai`, the spec file's `resolution:` frontmatter field contains the git diff | FR-5 | `grep "^resolution:" specs/MySpec.md` starts with `diff --git` |
| **C7** | The DB `resolution` column also contains the diff (not empty) | FR-5 | `sqlite3 ... "SELECT resolution FROM specs WHERE ..."` begins with `diff --git` |
| **C8** | `ff spec --ai "title" "body" --type bug --version 0.9.5` creates spec with `type: bug`, `version: "0.9.5"` | FR-6 | `grep "^type:" specs/Spec.md` → `bug`; `grep "^version:"` → `0.9.5` |
| **C9** | `ff spec --ai "title" "body"` WITHOUT `--type` FAILS with a clear error message | FR-6 | Exit code != 0, stderr contains `--type is required` |
| **C10** | `ff spec --ai "title" "body" --type invalid_value` FAILS with valid-types listing | FR-6 | Stderr includes `valid values are feature, bug, refactor` |
| **C11** | `ff commit --ai` with a bug-type spec that has no `root_cause` prints a stderr warning | FR-6 | Stderr includes `warning:.*bug.*no root_cause` |
| **C12** | `go test ./... -count=1` passes with no regressions | SC-1 | `go test ./run ./...` exits 0 |

## Verification

### Automated (run after each implementation step)

```
go test ./... -count=1
```

### C1 — SpecEntry fields exist
```
grep -A12 'type SpecEntry struct' engine/ledger.go | grep -E 'Body|RootCause|Resolution|Version'
```

### C3/C4 — DB has real content after sync
```
sqlite3 .ff/forgefix.db "SELECT spec_id, type, version, substr(root_cause,1,40), substr(resolution,1,40), substr(body,1,60) FROM specs WHERE body != '' LIMIT 5;"
```

Expected: each column has actual spec content, not `''`.

### C5 — Commit message body visible
```
ff commit --ai "test subject" --body "this is the body content"
git log -1 --format='%B'
```

Expected:
```
feat: [SPEC-XXXXXX] test subject

this is the body content
```

### C6/C7 — Resolution captures git diff
```
head -5 specs/MySpec.md | grep resolution
sqlite3 .ff/forgefix.db "SELECT substr(resolution,1,80) FROM specs WHERE resolution != '' LIMIT 1;"
```

Expected: output starts with `resolution: |` followed by `diff --git a/...`

### C8-C10 — --type enforcement on ff spec --ai
```
# Should pass: explicit type
ff spec --ai "valid spec" "body text" --type bug --version 0.9.5 && echo "PASS"

# Should fail: no --type
ff spec --ai "bad spec" "body text" 2>&1 && echo "SHOULD HAVE FAILED" || echo "PASS (expected failure)"

# Should fail: invalid --type
ff spec --ai "bad spec" "body text" --type invalid 2>&1 && echo "SHOULD HAVE FAILED" || echo "PASS (expected failure)"
```

### C11 — Bug spec without root_cause warns on commit
```
ff spec --ai "bug without cause" "body" --type bug
ff commit --ai "fix" 2>&1 | grep -i "warning.*root_cause"
```

### Data integrity round-trip (full lifecycle)
```
ff spec --ai "Full round-trip test" "This spec tests the complete data flow" --type refactor --version 0.9.5
# Edit the spec file to add Objective, Requirements sections
ff commit --ai "implement round-trip" --body "All layers updated to propagate body, root_cause, resolution, version"
ff sync --ai  # (if remote available; skip if no credentials)
sqlite3 .ff/forgefix.db "SELECT type, version, substr(body,1,40) FROM specs WHERE title LIKE '%round-trip%';"
```

Expected: `type=refactor`, `version=0.9.5`, `body=non-empty`.
