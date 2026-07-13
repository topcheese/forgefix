---
spec_id: "SPEC-1783945938"
status: review
repo_issue: ""
type: feature
version: "0.9.5"
root_cause: "Version had three disconnected sources: (1) CurrentVersion() read from ledger which has no version field, so it always returned 0.0.0. (2) createSpec() hardcoded const Version (0.9.0) instead of reading from DB. (3) WriteVersion and createSpec both used a hardcoded 'v0.8.0' string replacement to update the template instead of matching any version: line."
resolution: "CurrentVersion() now reads from DB first, falls back to ledger, then 0.0.0. createSpec() now reads the DB version via VersionManager.CurrentVersion() instead of const Version. Both WriteVersion and createSpec now match any 'version:' line in the template with a flexible prefix match instead of the hardcoded v0.8.0 string."
linked_commits: []
---
The version has three bugs: (1) CurrentVersion() reads from ledger but the ledger has no version field — should read DB first, fall back to ledger, then '0.0.0'. (2) createSpec() hardcodes const Version (0.9.0) instead of reading from DB — new specs get the wrong version. (3) WriteVersion and createSpec both use hardcoded 'v0.8.0' string replacement to update the template instead of matching any version: line.
