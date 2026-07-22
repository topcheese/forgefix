---
spec_id: "SPEC-1784743523"
status: draft
repo_issue: ""
type: bug
version: "0.9.8"
root_cause: ""



linked_commits: ["558901b", "fdee604", "8dead67"]
resolution: fixed in f995178
---
# Fix Isvalidissuetitle Helper Regex For Old Format
## Objective
TestIsValidIssueTitle_Helper is failing because the updated regex does not correctly match the old 'type/component: Title' format when the component name is long.

## Root Cause
The old format part of the regex was . The issue_validator_test.go has a test case:  which is expected to return false but returns true because the component name 'engine' is short, and the title part is long, and the maxTitleLength check was moved/changed.

Actually, the test failure message is:


This means the regex matched, and the length check passed (or failed to fail).

## Requirements
1. Fix the regex and validation logic to correctly handle both new and old formats.
2. Ensure the maxTitleLength check (120 for new, likely 60 for old or consistent 120) is enforced correctly.
3. Fix the failing test case.
