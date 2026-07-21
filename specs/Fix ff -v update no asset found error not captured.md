---spec_id: "SPEC-1784103956"
status: review
repo_issue: 537
type: bug
version: "0.9.6"
root_cause: "runUpdate (engine/command_dispatcher.go:160) handles update failures by printing to stderr and returning — e.g. the 'no asset found' branch (line 187) does fmt.Fprintf(stderr, ...) then returns. It never calls EmitAIError (execute.go:536), which exists specifically to emit structured errors, nor does it enqueue the failure to the background queue for later attention. So update failures (no asset, download error, release fetch error) are silently dropped from any persistent/attention path and only flash on the console."
linked_commits: ["a1ddbff"]
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
