---
spec_id: "SPEC-1783944987"
status: draft
repo_issue: ""
type: feature
version: "0.9.0"
root_cause: "ff --version read const Version from source (0.9.0), but the canonical project version (0.9.5) lives in the DB meta table. These diverged because ff ship updates the DB but not the source const."
resolution: "Changed handleVersion to read from the DB via VersionManager.CurrentVersion(), with fallback to the const when no DB exists. Also redirected cmd_run.go's flags.Version path to handleVersion() instead of PrintVersion()."
linked_commits: ["2e0dd5c"]
---
ff --version prints a hardcoded constant (const Version = "0.9.0") from engine/flags.go, but the actual project version is stored in the DB meta table (project_version = 0.9.5). These diverge because ff ship updates the DB but not the source const. Fix: handleVersion should read from the DB via VersionManager, falling back to the const when no DB exists. Also fix cmd_run.go:14 which also calls PrintVersion before running tests.
