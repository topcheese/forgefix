---
spec_id: "SPEC-1783224084"
status: ship
repo_issue: "466"
type: bug
version: "v0.8.1"
root_cause: "GitHub API returns 410 Gone (not 404) for deleted issues, causing the sync queue to retry operations infinitely on deleted issues because the GitHub client only checks for StatusNotFound"
resolution: "Added HTTP 410 Gone handling to GitHub client methods alongside existing 404 handling. The isNotFoundOrGone helper function is used in all issue-operating methods (GetIssueByNumber, CloseIssueByNumber, PostComment, GetIssueComments, UpdateIssueTitle, UpdateIssueBody) to return ErrResourceNotFound for both status codes. Also fixed TestPreferLocalBinaryFindsLocalFF to use os.SameFile instead of path-based comparison. Removed duplicate version manager functions from execute.go that were extracted to version_manager.go but not cleaned up."
---

# Fix Sync Queue Retrying Deleted Issues With 410 Gone Handling

## Objective

Fix the sync queue from endlessly retrying operations on deleted GitHub issues. GitHub's REST API returns HTTP 410 (Gone) for deleted issues, but the ForgeFix GitHub client only handles HTTP 404 (Not Found) as `ErrResourceNotFound`. This causes operations on deleted issues to be retried up to 3 times and then recorded as permanent sync failures, when they should be immediately dropped and cleaned up.

Additionally, fix a failing unit test `TestPreferLocalBinaryFindsLocalFF` that fails on macOS due to `/var` → `/private/var` symlink resolution differences.

## Requirements

- GitHub client methods that operate on individual issues must treat HTTP 410 (Gone) the same as HTTP 404 (Not Found), returning `ErrResourceNotFound`
- Methods affected: `GetIssueByNumber`, `CloseIssueByNumber`, `PostComment`, `GetIssueComments`, `UpdateIssueTitle`, `UpdateIssueBody`
- When the sync queue encounters a 410 on a delete issue operation, it should drop the operation and clean up the spec's `repo_issue` field (same as 404 behavior)
- Fix `TestPreferLocalBinaryFindsLocalFF` to use `os.SameFile` for comparison instead of `filepath.EvalSymlinks`
- Add test coverage for 410 handling in the GitHub client and sync queue
- All existing tests must continue to pass

## Implementation

### Changes to `engine/github_client.go`

1. Add a helper function `isNotFound(err error) bool` or inline `http.StatusGone` checks:
   - `GetIssueByNumber`: Add `resp.StatusCode == http.StatusGone` alongside `StatusNotFound` check
   - `CloseIssueByNumber`: Add `resp.StatusCode == http.StatusGone` alongside `StatusNotFound` check  
   - `PostComment`: Add `resp.StatusCode == http.StatusGone` alongside `StatusNotFound` check
   - `GetIssueComments`: Add `resp.StatusCode == http.StatusGone` alongside `StatusNotFound` check
   - `UpdateIssueTitle`: Add `StatusNotFound` and `StatusGone` handling (currently missing any 404 handling)
   - `UpdateIssueBody`: Add `StatusNotFound` and `StatusGone` handling (currently missing any 404 handling)

### Changes to `engine/shortcut_test.go`

1. `TestPreferLocalBinaryFindsLocalFF`: Replace path string comparison with `os.SameFile` to compare file identity instead of resolved path strings, which is robust against macOS `/var` → `/private/var` symlink differences

### New/Updated Tests

1. Update `TestProcessSyncQueue_DropsDeletedIssueOpAndCleansUpSpec` in `sync_test.go` to also test 410 response
2. Add test helper to mock 410 responses in `github_client_test.go`

## Acceptance Criteria

- [ ] GitHub client returns `ErrResourceNotFound` for both 404 and 410 responses on all issue operations
- [ ] Sync queue drops operations and cleans up spec references when a deleted issue returns 410
- [ ] `TestPreferLocalBinaryFindsLocalFF` passes on macOS
- [ ] All existing tests pass when run with `go test ./...`
- [ ] No regression in sync queue behavior

## Verification

```bash
# Run specific tests
go test ./engine/ -run TestPreferLocalBinaryFindsLocalFF -v
go test ./engine/ -run TestProcessSyncQueue -v
go test ./engine/ -run TestGone -v

# Run full suite
go test ./... -v

# Run with ForgeFix itself
ff --ai
```
