---
spec_id: "SPEC-1783149788"
status: in-progress
repo_issue: ""
type: bug
root_cause: "Three interacting bugs in ExecuteSuite cause the test runner to report 'timeout' even when all tests successfully completed: (1) The done goroutine sleeps 300ms before closing the done channel, creating a race window where ctx.Done() can fire after tests finish; (2) ToAIPayload checks d.TestCommandCompleted and assigns status='timeout' — backwards logic that conflates completion with timeout; (3) The timeout path in execute.go never triggers detonation, leaving BombActive, which causes renderFinal to fall through to the success footer path."
resolution: ""
---
# Fix Test Runner Timeout Reporting False Positives

## Objective
When the global timeout fires after tests have already completed, the runner must report the actual result (pass/fail), not a contradictory timeout-with-success message. When the timeout fires before completion, it must report failure (detonation), not a success footer.

## Requirements
1. Remove the 300ms race window in the done goroutine — close the channel immediately after wg and parseWG complete.
2. Fix ToAIPayload logic so `TestCommandCompleted` yields `pass` (when no failures), and timeout-firing yields `timeout` only when tests did NOT complete.
3. In the `ctx.Done()` path, trigger detonation so the UI/JSON output correctly shows failure state instead of success.

## Implementation
- `engine/execute.go`: Remove `time.Sleep(300 * time.Millisecond)` from done goroutine; add `dashboard.TriggerDetonation()` in `<-ctx.Done()` branch.
- `engine/aipayload.go`: Fix `TestCommandCompleted` branch to set `status="pass"` and only set `status="timeout"` when `d.GetTimeoutFired() && !d.TestCommandCompleted`.

## Acceptance Criteria
- [ ] Running `ff --ai` on a project where tests complete within timeout reports `"status": "pass"`, not `"timeout"`.
- [ ] Running `ff` on a project where the global timeout fires before completion reports detonation, not success.
- [ ] No "TESTS STILL RUNNING AT TIMEOUT" message appears when all tests actually finished.

## Verification
