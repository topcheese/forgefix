---
spec_id: "SPEC-1784660285"
status: review
repo_issue: ""
type: bug
version: "0.9.7"
root_cause: ""
resolution: ""
linked_commits: ["effd4a1", "02a4e96", "fbecfb4", "515655c"]
---
ff commit --ai corrupts the bound spec file's frontmatter with duplicate linked_commits keys and writes a stale pre-amend commit hash. Two defects: (1) UpdateSpecFileLinkedCommits can leave duplicate linked_commits keys when the spec already has one, producing invalid YAML. (2) ff commit writes the pre-amend SHA to linked_commits then amends the commit, producing a new SHA that the spec file still references. Fixes: de-duplicate linked_commits on write, record final SHA after amend, advance spec status from draft to review on commit, validate frontmatter on write.
