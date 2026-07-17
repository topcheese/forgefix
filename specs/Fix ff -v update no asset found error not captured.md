---spec_id: "SPEC-1784103956"
status: review
repo_issue: 537
type: bug
version: "0.9.6"
root_cause: "runUpdate (engine/command_dispatcher.go:160) handles update failures by printing to stderr and returning — e.g. the 'no asset found' branch (line 187) does fmt.Fprintf(stderr, ...) then returns. It never calls EmitAIError (execute.go:536), which exists specifically to emit structured errors, nor does it enqueue the failure to the background queue for later attention. So update failures (no asset, download error, release fetch error) are silently dropped from any persistent/attention path and only flash on the console."
 ""
linked_commits: ["a1ddbff"]
resolution: |
  diff --git a/CHANGELOG.md b/CHANGELOG.md
  index 44e4d6f..56040c2 100644
  --- a/CHANGELOG.md
  +++ b/CHANGELOG.md
  @@ -6,6 +6,7 @@
   - feat: Remove test-specs directory from commit (SPEC-1784101811)
   - feat: Update spec SPEC-1784100187 to review status with linked commit (SPEC-1784100187)
   - feat: Fix detonation/defused/timeout issue-handling integration tests failing on GitHub API 404 (SPEC-1784101189)
  +- feat: Fix ff -v update no asset found error not captured (SPEC-1784103956)
   
   ## [Unreleased] - 2026-07-14
   
  diff --git a/engine/command_dispatcher.go b/engine/command_dispatcher.go
  index 6d10746..77d2510 100644
  --- a/engine/command_dispatcher.go
  +++ b/engine/command_dispatcher.go
  @@ -162,6 +162,8 @@ func runUpdate(configDir string, coord *IssueCoordinator, version string, stdout
   	release, err := coord.LatestRelease()
   	if err != nil {
   		fmt.Fprintf(stderr, "Update failed: %v\n", err)
  +		EmitAIError("UPDATE_RELEASE_FETCH_FAILED", err.Error())
  +		_ = QueueUpdateFailure(configDir, "UPDATE_RELEASE_FETCH_FAILED", err.Error())
   		return
   	}
   	// Find asset matching current platform. Assets are named "ff", "ff-linux-amd64",
  @@ -184,23 +186,32 @@ func runUpdate(configDir string, coord *IssueCoordinator, version string, stdout
   		}
   	}
   	if asset == nil {
  -		fmt.Fprintf(stderr, "Update failed: no asset found for %s/%s\n", runtime.GOOS, runtime.GOARCH)
  +		errMsg := fmt.Sprintf("no asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
  +		fmt.Fprintf(stderr, "Update failed: %s\n", errMsg)
  +		EmitAIError("UPDATE_NO_ASSET", errMsg)
  +		_ = QueueUpdateFailure(configDir, "UPDATE_NO_ASSET", errMsg)
   		return
   	}
   	data, err := coord.DownloadReleaseAsset(asset.ID)
   	if err != nil {
   		fmt.Fprintf(stderr, "Update failed: %v\n", err)
  +		EmitAIError("UPDATE_DOWNLOAD_FAILED", err.Error())
  +		_ = QueueUpdateFailure(configDir, "UPDATE_DOWNLOAD_FAILED", err.Error())
   		return
   	}
   	// Write to a temp file, make executable, replace current binary
   	tmpPath := filepath.Join(os.TempDir(), "ff-update")
   	if err := os.WriteFile(tmpPath, data, 0755); err != nil {
   		fmt.Fprintf(stderr, "Update failed: writing temp file: %v\n", err)
  +		EmitAIError("UPDATE_WRITE_TEMP_FAILED", err.Error())
  +		_ = QueueUpdateFailure(configDir, "UPDATE_WRITE_TEMP_FAILED", err.Error())
   		return
   	}
   	exe, err := os.Executable()
   	if err != nil {
   		fmt.Fprintf(stderr, "Update failed: finding executable: %v\n", err)
  +		EmitAIError("UPDATE_FIND_EXECUTABLE_FAILED", err.Error())
  +		_ = QueueUpdateFailure(configDir, "UPDATE_FIND_EXECUTABLE_FAILED", err.Error())
   		return
   	}
   	backupPath := exe + ".bak"
  @@ -208,6 +219,8 @@ func runUpdate(configDir string, coord *IssueCoordinator, version string, stdout
   	if err := os.Rename(tmpPath, exe); err != nil {
   		os.Rename(backupPath, exe) // restore backup
   		fmt.Fprintf(stderr, "Update failed: replacing binary: %v\n", err)
  +		EmitAIError("UPDATE_REPLACE_BINARY_FAILED", err.Error())
  +		_ = QueueUpdateFailure(configDir, "UPDATE_REPLACE_BINARY_FAILED", err.Error())
   		return
   	}
   	fmt.Fprintf(stdout, "Updated to version %s.\n", version)
  diff --git a/engine/sync.go b/engine/sync.go
  index 0b83b66..9878468 100644
  --- a/engine/sync.go
  +++ b/engine/sync.go
  @@ -39,6 +39,7 @@ const (
   	SyncOpUpdateIssueBody SyncOpType = "update_issue_body"
   	SyncOpSyncSpec        SyncOpType = "sync_spec"
   	SyncOpDeleteIssue     SyncOpType = "delete_issue"
  +	SyncOpUpdateFailure   SyncOpType = "update_failure"
   )
   
   type SyncOperation struct {
  @@ -184,6 +185,16 @@ func QueueDeleteIssue(configDir, specID string, issueNumber int) error {
   	})
   }
   
  +// QueueUpdateFailure enqueues an update failure to the sync queue for attention.
  +// This ensures update failures (no asset found, download error, etc.) are not silently dropped.
  +func QueueUpdateFailure(configDir, errorCode, detail string) error {
  +	return EnqueueSyncOp(configDir, SyncOperation{
  +		Type:   SyncOpUpdateFailure,
  +		Title:  errorCode,
  +		Body:   detail,
  +	})
  +}
  +
   func SyncFailureLogPath(configDir string) string {
   	return filepath.Join(FFDir(configDir), syncFailureLogName)
   }
  diff --git a/specs/Fix ff -v update no asset found error not captured.md b/specs/Fix ff -v update no asset found error not captured.md
  index 09e4770..1356885 100644
  --- a/specs/Fix ff -v update no asset found error not captured.md	
  +++ b/specs/Fix ff -v update no asset found error not captured.md	
  @@ -1,6 +1,6 @@
   ---
   spec_id: "SPEC-1784103956"
  -status: draft
  +status: review
   repo_issue: ""
   type: bug
   version: "0.9.6"
---

# Fix `ff -v` Update "No Asset Found" Error Not Captured Or Logged

## Objective

When `ff -v` (or the update flow) cannot find a release asset for the current
platform, it prints "Update failed: no asset found for <os>/<arch>" to stderr
and returns. That error is never captured for attention (no background queue)
and not logged persistently. This spec routes update failures through the
existing error-capture path (`EmitAIError`) and, at minimum, logs them to
console so they are not silently lost.

## Root Cause (from code reading)

- `runUpdate` (command_dispatcher.go:160) has three failure exits:
  - release fetch error (line 164)
  - `asset == nil` → "no asset found" (line 187)
  - asset download error (line 192)
  Each does `fmt.Fprintf(stderr, "Update failed: %v\n", err)` and `return`.
- `EmitAIError` (execute.go:536) already exists to emit a structured
  `AIResponsePayload` error (status="error", with code + detail). It is used
  by other paths (CONFIG_LOAD_FAILURE, LEDGER_CORRUPTION, ship controller) but
  **not** by `runUpdate`.
- There is prior intent for such errors to be captured and re-queued for
  attention (background queue), but `runUpdate` bypasses it entirely.

## Requirements

### 1. Route update failures through EmitAIError
- Replace the bare `fmt.Fprintf(stderr, "Update failed: ...")` calls in
  `runUpdate` with `EmitAIError` using a distinct error code
  (e.g. `UPDATE_NO_ASSET`, `UPDATE_DOWNLOAD_FAILED`, `UPDATE_RELEASE_FETCH_FAILED`)
  so the failure is emitted in the structured error format.

### 2. Enqueue to background queue for attention
- Where a background-queue capture mechanism exists, enqueue the update
  failure so it surfaces for later attention (matching the prior intent).
- If no such queue is reachable from `runUpdate`, at minimum ensure the error
  is logged to console via `EmitAIError` (which prints the structured payload)
  so it is not silently dropped.

### 3. Console logging guarantee
- Regardless of queue availability, the error MUST be visible on console
  (EmitAIError prints to stdout; ensure stderr path is also preserved or the
  structured output is sufficient). No update failure should exit silently.

## Acceptance Criteria

- The "no asset found" path emits a structured error via `EmitAIError` (not a
  bare stderr print-and-return).
- Update failures are enqueued to the background queue for attention where
  such a mechanism exists; otherwise they are at minimum logged to console.
- No update failure exits silently — every failure path is captured/logged.
- No regression to the 13 pre-existing failing integration tests (additive
  change; those remain tracked separately).

