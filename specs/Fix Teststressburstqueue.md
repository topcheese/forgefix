---
spec_id: "SPEC-1784743251"
status: draft
repo_issue: ""
type: bug
version: "0.9.8"
root_cause: ""
resolution: ""
linked_commits: []
---
# Fix Teststressburstqueue
## Objective
Automatically created from failing test TestStressBurstQueue during ff --ai run.

## Root Cause
Test failed - see failure details below.

## Failure Details
- Test: TestStressBurstQueue
- File: sync_stress_test.go
- Line: 0
- Error: === RUN   TestStressBurstQueue
[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
[DEBUG] GET no open issues with matching title: feat/test: burst-0
[DEBUG] GET no open issues with matching title: feat/test: burst-1
[DEBUG] GET no open issues with matching title: feat/test: burst-2
[DEBUG] GET no open issues with matching title: feat/test: burst-3
[DEBUG] GET no open issues with matching title: feat/test: burst-4
[DEBUG] GET no open issues with matching title: feat/test: burst-5
[DEBUG] GET no open issues with matching title: feat/test: burst-6
[DEBUG] GET no open issues with matching title: feat/test: burst-7
[DEBUG] GET no open issues with matching title: feat/test: burst-8
[DEBUG] GET no open issues with matching title: feat/test: burst-9
[DEBUG] GET no open issues with matching title: feat/test: burst-10
[DEBUG] GET no open issues with matching title: feat/test: burst-11
[DEBUG] GET no open issues with matching title: feat/test: burst-12
[DEBUG] GET no open issues with matching title: feat/test: burst-13
[DEBUG] GET no open issues with matching title: feat/test: burst-14
[DEBUG] GET no open issues with matching title: feat/test: burst-15
[DEBUG] GET no open issues with matching title: feat/test: burst-16
[DEBUG] GET no open issues with matching title: feat/test: burst-17
[DEBUG] GET no open issues with matching title: feat/test: burst-18
[DEBUG] GET no open issues with matching title: feat/test: burst-19
    sync_stress_test.go:449: processSyncQueue: issue title validation failed: regex mismatch for "feat/test: burst-19": invalid issue title format
--- FAIL: TestStressBurstQueue (0.12s)

