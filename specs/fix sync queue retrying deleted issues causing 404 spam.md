---
spec_id: "SPEC-1783151970"
status: in-progress
repo_issue: 453
type: bug
version: 0.8.1
root_cause: "syncSingleSpec in engine/sync.go treats a 404 from GetIssueByNumber as a hard error, returning it from the function. This causes every sync cycle to repeatedly try fetching the deleted issue, log the 404, and record a sync failure — creating infinite 404 spam."
resolution: ""
---
# Fix Sync Queue Retrying Deleted Issues Causing 404 Spam

## Objective

Stop the sync queue from repeatedly retrying remote operations on deleted issues, which produces 404 log spam and consumes API rate limits unnecessarily.

## Requirements

- When syncSingleSpec encounters a 404 (ErrResourceNotFound) fetching a spec's remote issue, it should clear the RepoIssue reference and continue gracefully instead of erroring out.
- The spec file's repo_issue field should be reset to 0 so the next sync creates a replacement issue.
- A warning message should be logged to stderr.
- The processSyncQueue already handles 404s correctly for queued operations — only syncSingleSpec needs the fix.
- All existing tests continue to pass.

## Implementation

In engine/sync.go, syncSingleSpec:
- After GetIssueByNumber returns an error, check if it wraps ErrResourceNotFound using errors.Is.
- If so, log a warning to stderr, call updateSpecFileRepoIssue(filePath, 0) to clear the field, and continue to the next spec (or fall through to the ledger save).
- All other errors still return as before.

## Acceptance Criteria

- [ ] syncSingleSpec handles 404 gracefully instead of returning error
- [ ] Spec file's repo_issue is reset to 0 when remote issue is deleted
- [ ] Warning is logged to stderr
- [ ] Existing tests still pass
- [ ] New test covers the deleted-issue scenario

## Verification

- go test ./engine/ -count=1 -v -run TestSyncSpecs_DeletedIssue
- go test ./... -count=1
