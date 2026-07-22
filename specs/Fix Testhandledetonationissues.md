---
spec_id: "SPEC-1784143268"
status: ship
repo_issue: ""
type: bug
version: "0.9.6"
root_cause: "handleDetonationIssues was called with an empty configDir string, causing writeSpecFromTemplate to fail silently (template file not found). The spec creation failed, so no issue was queued."
resolution: "Fixed in commit f84bb9c (SPEC-1784101189). Added template directory creation with spec_template.md in test setup, changed handleDetonationIssues(d, \"\") to handleDetonationIssues(d, tmpDir) to pass the correct config directory."
linked_commits: ["f84bb9c"]
---

# Fix TestHandleDetonationIssues

## Objective

Automatically created from failing test TestHandleDetonationIssues during ff --ai run. The test verifies that handleDetonationIssues properly queues issue operations.
