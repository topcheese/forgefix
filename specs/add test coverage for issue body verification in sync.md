---
spec_id: "SPEC-1783612711"
status: review
repo_issue: ""
type: feature
version: "v0.8.0"
root_cause: ""
resolution: ""
---
# Add Test Coverage For Issue Body Verification In Sync

## Objective

No existing test verifies that `ff sync` posts the correct body content to the
remote issue. The `TestSpecLifecycle` integration test uses a mock server that
returns fixed responses and never inspects the request body. Add tests that
assert issue body content, title formatting, and body update behavior.

## Requirements

1. Update `TestSpecLifecycle` mock server to capture and verify request bodies.
2. Add unit tests for `SyncSpecs` body comparison logic.
3. Add unit tests for spec title sanitization (especially for specs with `--`).
4. Ensure test coverage for the body quality validation (#2 above).

## Implementation

- In the `createMockRepo` handler, capture `POST /issues` request bodies and
  assert they contain the expected spec body content.
- Add a unit test in `sync_test.go` that calls `syncSpec` and verifies the
  issue body received by the mock matches the spec file body.
- Add a unit test in `spec_test.go` for title sanitization.

## Acceptance Criteria

- `TestSpecLifecycle` fails if the issue body doesn't match the spec content.
- A new `TestSyncIssueBodyMatchesSpec` unit test exists.
- A new `TestSpecTitleSanitization` unit test exists.
- All existing tests still pass after adding tests.

## Verification

- `go test ./engine -run TestSyncIssueBodyMatchesSpec -v` passes.
- `go test ./tests -run TestSpecLifecycle -v` passes.
- `go test ./... -count=1` — all 4 modules green.

<!--
Available types: feature, bug, chore, docs, refactor, ops
Available versions: v0.8.0 (current), v0.9.0
-->