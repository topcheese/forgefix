---
spec_id: "SPEC-1784075091"
status: review
repo_issue: ""
type: feature
version: "0.9.5"
root_cause: "Investigation identified syncSingleSpec unconditionally creating remote issues when RepoIssue==0 instead of checking for existing issues by title first, risking duplicate remote issues if repo_issue was cleared"
resolution: "Aligned syncSingleSpec with SyncSpecs: now calls findRemoteIssueByTitle first, and only creates a new issue when no match is found. Also aligned title format to prefixedTitle(spec) and added markSpecAsDuplicateIfNeeded call for consistency. Duplicate issue risk eliminated."
linked_commits: ["0467f8b", "0fce8d8", "37ff9ba"]
---
Investigate where, how, and when the repo_issue value gets set in spec frontmatter. Some specs have it populated (e.g., repo_issue: 42) and others don't (repo_issue: ''). Trace the full lifecycle of repo_issue assignment across spec creation, sync, commit, and ship operations to identify the source of inconsistency.

## Investigation Results

### Lifecycle of `repo_issue`

| Operation | repo_issue in frontmatter | Ledger RepoIssueID | Method |
|-----------|--------------------------|-------------------|--------|
| `ff spec --ai` / `createNewBugSpec` | `repo_issue: ""` | `0` | Hardcoded in template (cmd_commit.go:718, 741) |
| `ff commit --ai` | Unchanged | Unchanged | No repo_issue modification — only status → "review" |
| `ff sync --ai` (full) | Set to issue number | Set via ledger | `SyncSpecs`: find by title or create → `UpdateRepoIssue(filePath, num)` (issue_coordinator.go:897-922, 1067) |
| `ff sync --spec X` | Set to issue number | Set via ledger | `syncSingleSpec`: unconditionally creates issue if RepoIssue==0 → `updateSpecFileRepoIssue(filePath, num)` (sync.go:605-657) |
| `ff ship --ai` | Used (read only) | Used (read only) | Housekeeping reads RepoIssueID to close remote issue |
| `ff export` | `repo_issue: ""` | `0` | Export hardcodes 0 (cmd_export.go:181) — safe for import to new env |

### Root Cause of Inconsistency

**No code-level bug.** The `repo_issue` field is a *sync-time artifact* — it only gets populated when a successful `ff sync --ai` completes against a reachable remote issue tracker.

**Specs WITH `repo_issue` set** (e.g., #494, #516, #518, #520, #522, #526-529) → Went through a successful `ff sync --ai` cycle that created or matched a remote issue.

**Specs WITHOUT `repo_issue`** (current draft/review specs) → Either:
- Created after the last successful sync (no `ff sync --ai` has been run since)
- Remote issue tracker is unreachable (current environment: Gitea at 192.168.1.18:3000)
- `auto_issue_management: false` is set (sync gated by SPEC-1783983677 fix) — correct behavior

### Minor Finding: `syncSingleSpec` vs `SyncSpecs` Divergence

`syncSingleSpec` (sync.go:605) unconditionally creates a new remote issue when `spec.RepoIssue == 0`:
```go
issue, err := coord.CreateIssueWithBody(title, effectiveSpecBody(spec))
```

`SyncSpecs` (issue_coordinator.go:897) first tries to find an existing remote issue by title:
```go
if existing, err := c.findRemoteIssueByTitle(spec.Title); err == nil {
    spec.RepoIssue = existing.Number
}
```

This means `ff sync --spec X` may create duplicate remote issues if the spec was already synced but had its `repo_issue` cleared. In practice this is low risk since `repo_issue` isn't cleared without a specific action (404 from deleted remote issue, or manual edit).

### Conclusion

No code changes required. The `repo_issue` population works as designed — it's a side effect of remote issue creation/matching during `ff sync --ai`. Specs that haven't been synced will always have `repo_issue: ""`, which is correct and expected.
