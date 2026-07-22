---
spec_id: "SPEC-1784143269"
status: review
repo_issue: ""
type: bug
version: "0.9.6"
root_cause: "handleDetonationIssues was called with an empty configDir string in some tests, causing writeSpecFromTemplate to fail silently (template file not found). Spec creation failed, so no issue was queued and IssueRefs stayed empty."
resolution: "Fixed in commit f84bb9c (SPEC-1784101189). Added template directory creation with spec_template.md in test setup, changed handleDetonationIssues(d, \"\") to handleDetonationIssues(d, tmpDir) to pass the correct config directory."
linked_commits: ["f84bb9c"]
---

# Fix TestIntegration_MultiplePipelinesFailures

## Objective

Automatically created from failing test TestIntegration_MultiplePipelinesFailures during ff --ai run. The test verifies that multiple pipelines produce the expected issue refs.
