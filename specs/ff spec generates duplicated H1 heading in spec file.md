---
spec_id: "SPEC-1784679765"
status: draft
repo_issue: ""
type: bug
version: "0.9.7"
root_cause: ""
resolution: ""
linked_commits: []
---
# Ff Spec Generates Duplicated H1 Heading In Spec File
# Ff Spec Generates Duplicated H1 Heading In Spec File

When ff spec creates a new spec file, the H1 heading is written twice. The template at cmd_spec.go:232 generates "# <title>" and the title itself contains the heading prefix, resulting in two consecutive H1 lines (e.g. "# Fix Foo\n# Fix Foo"). This was observed in multiple newly-created spec files. Fix: ensure writeSpecFromTemplate emits the title exactly once, or strip any existing H1 from the body before prepending.
